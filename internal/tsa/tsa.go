// Package tsa acquires RFC 3161 responses and checks their integrity and trust.
package tsa

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	_ "crypto/sha512"
	"crypto/x509"
	"crypto/x509/pkix"
	_ "embed"
	"encoding/asn1"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/digitorus/pkcs7"
	"github.com/digitorus/timestamp"
	"github.com/vincentsch/signetleaf/internal/tsa/demotsa"
)

//go:embed roots/freetsa-cacert.pem
var freeRoot []byte

var Defaults = []string{"http://timestamp.digicert.com", "https://freetsa.org/tsr"}
var sha256OID = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 1}
var ekuOID = asn1.ObjectIdentifier{2, 5, 29, 37}
var timestampOID = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 3, 8}

type Result struct {
	Path          string `json:"path,omitempty"`
	Authority     string `json:"authority"`
	AuthorityName string `json:"authority_name"`
	Time          string `json:"time"`
	Serial        string `json:"serial"`
	Accuracy      string `json:"accuracy"`
	Demo          bool   `json:"demo"`
	Trusted       bool   `json:"trusted"`
	Reason        string `json:"reason"`
	Invalid       bool   `json:"invalid"`
	Usable        bool   `json:"-"`
}
type Attempt struct {
	URL   string  `json:"url"`
	Error string  `json:"error,omitempty"`
	Token *Result `json:"token,omitempty"`
	DER   []byte  `json:"-"`
}

func Roots(file string) (*x509.CertPool, error) {
	roots, err := x509.SystemCertPool()
	if err != nil {
		return nil, fmt.Errorf("load system timestamp roots: %w", err)
	}
	roots.AppendCertsFromPEM(freeRoot)
	if file != "" {
		b, e := os.ReadFile(file)
		if e != nil {
			return nil, e
		}
		if !roots.AppendCertsFromPEM(b) {
			return nil, fmt.Errorf("timestamp root file contains no certificates")
		}
	}
	return roots, nil
}
func Authorities(explicit []string) ([]string, error) {
	list := explicit
	if len(list) == 0 {
		list = Defaults
	}
	for _, s := range list {
		u, e := url.Parse(s)
		if e != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.Fragment != "" {
			return nil, fmt.Errorf("timestamp authority must be an HTTP(S) URL without credentials or a fragment")
		}
	}
	return append([]string{}, list...), nil
}

var unsafeID = regexp.MustCompile(`[^a-zA-Z0-9_.-]+`)

func ID(endpoint string) string {
	if endpoint == Defaults[0] {
		return "digicert"
	}
	if endpoint == Defaults[1] {
		return "freetsa"
	}
	u, _ := url.Parse(endpoint)
	return unsafeID.ReplaceAllString(u.Hostname(), "-")
}

type response struct {
	Status struct {
		Status  int
		Text    []string       `asn1:"optional,utf8"`
		Failure asn1.BitString `asn1:"optional"`
	}
	Token asn1.RawValue `asn1:"optional"`
}
type info struct {
	Version int
	Policy  asn1.ObjectIdentifier
	Imprint struct {
		Algorithm pkix.AlgorithmIdentifier
		Digest    []byte
	}
	Serial   *big.Int
	Time     time.Time `asn1:"generalized"`
	Accuracy struct {
		Raw          asn1.RawContent
		Seconds      int64 `asn1:"optional"`
		Milliseconds int64 `asn1:"optional,tag:0"`
		Microseconds int64 `asn1:"optional,tag:1"`
	} `asn1:"optional"`
	Ordering   bool             `asn1:"optional,default:false"`
	Nonce      *big.Int         `asn1:"optional"`
	TSA        asn1.RawValue    `asn1:"optional,tag:0"`
	Extensions []pkix.Extension `asn1:"optional,tag:1"`
}

func decode(der []byte) (*pkcs7.PKCS7, *info, error) {
	var resp response
	rest, e := asn1.Unmarshal(der, &resp)
	if e != nil || len(rest) > 0 {
		return nil, nil, fmt.Errorf("malformed timestamp response")
	}
	if resp.Status.Status != 0 && resp.Status.Status != 1 {
		return nil, nil, fmt.Errorf("timestamp response was not granted (status %d)", resp.Status.Status)
	}
	p, e := pkcs7.Parse(resp.Token.FullBytes)
	if e != nil {
		return nil, nil, fmt.Errorf("malformed timestamp CMS: %w", e)
	}
	var inf info
	rest, e = asn1.Unmarshal(p.Content, &inf)
	if e != nil || len(rest) > 0 || inf.Version != 1 || inf.Serial == nil || inf.Time.IsZero() {
		return nil, nil, fmt.Errorf("malformed timestamp information")
	}
	// The encapsulated CMS content must be TSTInfo, not arbitrary signed data.
	var ci struct {
		Type    asn1.ObjectIdentifier
		Content asn1.RawValue `asn1:"explicit,tag:0"`
	}
	if _, e = asn1.Unmarshal(resp.Token.FullBytes, &ci); e != nil {
		return nil, nil, e
	}
	var sd struct {
		Version    int
		Algorithms asn1.RawValue
		Content    struct {
			Type asn1.ObjectIdentifier
			Data asn1.RawValue `asn1:"optional,explicit,tag:0"`
		}
		Rest asn1.RawValue `asn1:"optional"`
	}
	// Decode only the leading fields; remaining CMS fields are checked by pkcs7.
	if _, e = asn1.Unmarshal(ci.Content.Bytes, &sd); e != nil || !sd.Content.Type.Equal(asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 16, 1, 4}) {
		return nil, nil, fmt.Errorf("CMS content is not timestamp information")
	}
	return p, &inf, nil
}
func Check(der, manifest []byte, roots *x509.CertPool) Result {
	r := Result{Authority: "Unavailable", Time: "unavailable", Serial: "unavailable", Accuracy: "unavailable", Reason: "Untrusted"}
	p, inf, e := decode(der)
	if e != nil {
		r.Invalid = true
		r.Reason = e.Error()
		return r
	}
	r.Time = inf.Time.UTC().Format(time.RFC3339Nano)
	r.Serial = inf.Serial.String()
	r.Accuracy = "unspecified"
	if len(inf.Accuracy.Raw) > 0 {
		r.Accuracy = (time.Duration(inf.Accuracy.Seconds)*time.Second + time.Duration(inf.Accuracy.Milliseconds)*time.Millisecond + time.Duration(inf.Accuracy.Microseconds)*time.Microsecond).String()
	}
	signer := p.GetOnlySigner()
	if signer != nil {
		r.Authority = signer.Subject.String()
		r.AuthorityName = signer.Subject.CommonName
		if len(signer.Subject.Organization) > 0 {
			r.AuthorityName = strings.Join(signer.Subject.Organization, ", ")
		}
	}
	digest := sha256.Sum256(manifest)
	if !inf.Imprint.Algorithm.Algorithm.Equal(sha256OID) || !bytes.Equal(digest[:], inf.Imprint.Digest) {
		r.Invalid = true
		r.Reason = "SHA-256 manifest imprint mismatch"
		return r
	}
	if len(p.Signers) != 1 {
		r.Reason = "Exactly one identifiable signer is required"
		return r
	}
	if signer == nil {
		r.Reason = "Missing signer certificate; signature could not be checked"
		return r
	}
	// CMS signingTime is not the timestamp's genTime. Check only the CMS
	// signature here, and certificate validity against genTime below.
	signatureCMS := *p
	signatureCMS.Certificates = append([]*x509.Certificate{}, p.Certificates...)
	for i, c := range signatureCMS.Certificates {
		copy := *c
		copy.NotBefore = time.Time{}
		copy.NotAfter = time.Date(9999, 12, 31, 0, 0, 0, 0, time.UTC)
		signatureCMS.Certificates[i] = &copy
	}
	contentTypes := 0
	for _, a := range p.Signers[0].AuthenticatedAttributes {
		if a.Type.Equal(pkcs7.OIDAttributeContentType) {
			contentTypes++
			var typ asn1.ObjectIdentifier
			rest, err := asn1.Unmarshal(a.Value.Bytes, &typ)
			if err != nil || len(rest) > 0 || !typ.Equal(asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 16, 1, 4}) {
				r.Invalid = true
				r.Reason = "Invalid signed timestamp content type"
				return r
			}
		}
	}
	if contentTypes != 1 {
		r.Invalid = true
		r.Reason = "Timestamp requires exactly one signed content type"
		return r
	}
	if e = signatureCMS.Verify(); e != nil {
		r.Invalid = true
		r.Reason = "Invalid timestamp signature: " + e.Error()
		return r
	}
	if !bound(p, signer) {
		r.Reason = "Signed certificate identifier does not match the signer certificate"
		return r
	}
	critical := false
	for _, ext := range signer.Extensions {
		if ext.Id.Equal(ekuOID) {
			var usages []asn1.ObjectIdentifier
			rest, e := asn1.Unmarshal(ext.Value, &usages)
			critical = e == nil && len(rest) == 0 && ext.Critical && len(usages) == 1 && usages[0].Equal(timestampOID)
		}
	}
	if !critical {
		r.Reason = "Signer purpose must be critical timeStamping only"
		return r
	}
	intermediate := x509.NewCertPool()
	for _, c := range p.Certificates {
		if !bytes.Equal(c.Raw, signer.Raw) {
			intermediate.AddCert(c)
		}
	}
	demoRoots := x509.NewCertPool()
	demoRoots.AddCert(demotsa.Root())
	if _, err := signer.Verify(x509.VerifyOptions{Roots: demoRoots, Intermediates: intermediate, CurrentTime: inf.Time, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageTimeStamping}}); err == nil {
		r.Demo = true
	}
	_, e = signer.Verify(x509.VerifyOptions{Roots: roots, Intermediates: intermediate, CurrentTime: inf.Time, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageTimeStamping}})
	if e != nil {
		r.Reason = "Untrusted certificate chain at genTime: " + e.Error()
		var unknown x509.UnknownAuthorityError
		if !errors.As(e, &unknown) {
			return r
		}
		// Unknown anchors are usable only if the available chain is otherwise
		// valid. Temporary anchors here never change the trust verdict.
		temporary := x509.NewCertPool()
		for _, c := range p.Certificates {
			hasIssuer := false
			for _, parent := range p.Certificates {
				if !bytes.Equal(parent.Raw, c.Raw) && bytes.Equal(c.RawIssuer, parent.RawSubject) {
					hasIssuer = true
				}
			}
			if !hasIssuer {
				temporary.AddCert(c)
			}
		}
		if _, err := signer.Verify(x509.VerifyOptions{Roots: temporary, Intermediates: intermediate, CurrentTime: inf.Time, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageTimeStamping}}); err != nil {
			r.Reason = "Certificate chain is invalid at genTime: " + err.Error()
			return r
		}
		r.Usable = true
		return r
	}
	r.Trusted = true
	r.Usable = true
	r.Reason = "Valid signature and certificate chain trusted at genTime"
	return r
}

// ESSCertID identifies the signer through a signed certificate hash. The first
// entry binds the signer; subsequent entries may describe its chain.
func bound(p *pkcs7.PKCS7, c *x509.Certificate) bool {
	v1 := asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 16, 2, 12}
	v2 := asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 16, 2, 47}
	count := 0
	seen := map[string]bool{}
	valid := false
	for _, a := range p.Signers[0].AuthenticatedAttributes {
		if !a.Type.Equal(v1) && !a.Type.Equal(v2) {
			continue
		}
		count++
		if seen[a.Type.String()] {
			return false
		}
		seen[a.Type.String()] = true
		valid = false
		var outer struct {
			Certs    []asn1.RawValue
			Policies asn1.RawValue `asn1:"optional"`
		}
		rest, e := asn1.Unmarshal(a.Value.Bytes, &outer)
		if e != nil || len(rest) > 0 || len(outer.Certs) == 0 {
			return false
		}
		var hash []byte
		var issuer asn1.RawValue
		if a.Type.Equal(v1) {
			var id struct {
				Hash   []byte
				Issuer asn1.RawValue `asn1:"optional"`
			}
			rest, e = asn1.Unmarshal(outer.Certs[0].FullBytes, &id)
			hash = id.Hash
			issuer = id.Issuer
			expected := sha1.Sum(c.Raw)
			valid = e == nil && len(rest) == 0 && bytes.Equal(hash, expected[:])
		} else {
			var id struct {
				Algorithm pkix.AlgorithmIdentifier `asn1:"optional"`
				Hash      []byte
				Issuer    asn1.RawValue `asn1:"optional"`
			}
			rest, e = asn1.Unmarshal(outer.Certs[0].FullBytes, &id)
			hash = id.Hash
			issuer = id.Issuer
			algorithm := id.Algorithm.Algorithm
			h := crypto.SHA256
			switch {
			case len(algorithm) == 0 || algorithm.Equal(sha256OID):
			case algorithm.Equal(asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 2}):
				h = crypto.SHA384
			case algorithm.Equal(asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 3}):
				h = crypto.SHA512
			default:
				return false
			}
			digest := h.New()
			digest.Write(c.Raw)
			valid = e == nil && len(rest) == 0 && bytes.Equal(hash, digest.Sum(nil))
		}
		if valid && len(issuer.FullBytes) > 0 {
			var id struct {
				Names  []asn1.RawValue
				Serial *big.Int
			}
			rest, e = asn1.Unmarshal(issuer.FullBytes, &id)
			valid = e == nil && len(rest) == 0 && id.Serial != nil && id.Serial.Cmp(c.SerialNumber) == 0
			found := false
			for _, n := range id.Names {
				if n.Class == 2 && n.Tag == 4 && bytes.Equal(n.Bytes, c.RawIssuer) {
					found = true
				}
			}
			valid = valid && found
		}
		if !valid {
			return false
		}
	}
	return count > 0 && valid
}

func Acquire(ctx context.Context, manifest []byte, explicit []string, roots *x509.CertPool) []Attempt {
	list, _ := Authorities(explicit)
	attempts := []Attempt{}
	for _, endpoint := range list {
		a := request(ctx, manifest, endpoint, roots, 10*time.Second, http.DefaultTransport)
		attempts = append(attempts, a)
		if len(explicit) == 0 && len(a.DER) > 0 {
			break
		}
	}
	return attempts
}
func request(ctx context.Context, manifest []byte, endpoint string, roots *x509.CertPool, timeout time.Duration, transport http.RoundTripper) Attempt {
	a := Attempt{URL: endpoint}
	fail := func(reason string) Attempt { a.Error = reason; return a }
	nonceBytes := make([]byte, 8)
	if _, e := rand.Read(nonceBytes); e != nil {
		return fail("Could not create timestamp nonce")
	}
	nonce := new(big.Int).SetBytes(nonceBytes)
	der, e := timestamp.CreateRequest(bytes.NewReader(manifest), &timestamp.RequestOptions{Hash: crypto.SHA256, Certificates: true, Nonce: nonce})
	if e != nil {
		return fail("Could not encode timestamp request")
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, e := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(der))
	if e != nil {
		return fail("Invalid timestamp URL")
	}
	req.Header.Set("Content-Type", "application/timestamp-query")
	req.Header.Set("Accept", "application/timestamp-reply")
	client := &http.Client{Timeout: timeout, Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, e := client.Do(req)
	if e != nil { // Avoid proxy URLs and credentials from transport error strings.
		if ctx.Err() != nil {
			return fail("Timestamp request exceeded its deadline or was cancelled")
		}
		return fail("Timestamp connection failed; check endpoint and proxy settings")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fail(fmt.Sprintf("Timestamp HTTP status %d", resp.StatusCode))
	}
	b, e := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
	if e != nil {
		return fail("Could not read timestamp response before its deadline")
	}
	if len(b) > 1<<20 {
		return fail("Timestamp response exceeds 1 MiB")
	}
	_, inf, e := decode(b)
	if e != nil {
		return fail(e.Error())
	}
	if inf.Nonce == nil || inf.Nonce.Cmp(nonce) != 0 {
		return fail("Timestamp response nonce mismatch")
	}
	r := Check(b, manifest, roots)
	a.Token = &r
	if !r.Usable {
		return fail(r.Reason)
	}
	a.DER = b
	return a
}

func (r Result) DisplayAuthority() string {
	if r.Demo {
		return demotsa.Label
	}
	name := r.AuthorityName
	if name == "" {
		return r.Authority
	}
	if strings.HasPrefix(name, "DigiCert") {
		return "DigiCert"
	}
	if name == "Free TSA" || name == "FreeTSA" {
		return "FreeTSA"
	}
	return name
}
func (r Result) DisplayTime() string {
	t, e := time.Parse(time.RFC3339Nano, r.Time)
	if e != nil {
		return r.Time
	}
	return t.UTC().Format("2006-01-02 15:04:05 UTC")
}
