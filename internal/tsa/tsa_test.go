package tsa

import (
	"bytes"
	"context"
	"crypto"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/digitorus/timestamp"
	"github.com/vincentsch/sealjar/internal/tsa/testtsa"
)

func authority(t *testing.T, opt testtsa.Options) *testtsa.Authority {
	t.Helper()
	a, e := testtsa.New(opt)
	if e != nil {
		t.Fatal(e)
	}
	return a
}
func token(t *testing.T, a *testtsa.Authority, manifest []byte) []byte {
	t.Helper()
	h := sha256.Sum256(manifest)
	b, e := a.Response(&timestamp.Request{HashAlgorithm: crypto.SHA256, HashedMessage: h[:], Certificates: true, Nonce: big.NewInt(42)})
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestTrustAndIntegrity(t *testing.T) {
	manifest := []byte("exact manifest bytes\n")
	for _, tc := range []struct {
		name                     string
		opt                      testtsa.Options
		invalid, trusted, usable bool
		reason                   string
	}{
		{"valid", testtsa.Options{}, false, true, true, "trusted at genTime"},
		{"dual certificate identifiers", testtsa.Options{DualBinding: true}, false, true, true, "trusted at genTime"},
		{"historical certificate", testtsa.Options{Time: time.Now().AddDate(-2, 0, 0)}, false, true, true, "trusted at genTime"},
		{"missing signer with intermediate", testtsa.Options{MissingSigner: true}, false, false, false, "Missing signer"},
		{"certificate binding", testtsa.Options{WrongBinding: true}, false, false, false, "identifier"},
		{"noncritical purpose", testtsa.Options{NonCritical: true}, false, false, false, "critical timeStamping"},
		{"multiple purposes", testtsa.Options{MultiPurpose: true}, false, false, false, "critical timeStamping"},
		{"wrong imprint", testtsa.Options{WrongImprint: true}, true, false, false, "imprint mismatch"},
		{"invalid signature", testtsa.Options{InvalidSignature: true}, true, false, false, "Invalid timestamp signature"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := authority(t, tc.opt)
			roots := x509.NewCertPool()
			roots.AddCert(a.Root)
			der := token(t, a, manifest)
			r := Check(der, manifest, roots)
			if r.Invalid != tc.invalid || r.Trusted != tc.trusted || r.Usable != tc.usable || !strings.Contains(r.Reason, tc.reason) {
				t.Fatalf("unexpected verdict: %+v", r)
			}
			if tc.trusted {
				untrusted := Check(der, manifest, x509.NewCertPool())
				if untrusted.Invalid || untrusted.Trusted || !untrusted.Usable {
					t.Fatalf("package root was trusted or rejected: %+v", untrusted)
				}
			}
			if tc.opt.Time.IsZero() == false {
				manualOpenSSL(t, a, der, manifest)
			}
		})
	}
	a := authority(t, testtsa.Options{})
	der := token(t, a, manifest)
	if r := Check(der, append(manifest, ' '), x509.NewCertPool()); !r.Invalid {
		t.Fatal(r)
	}
	if r := Check([]byte("broken"), manifest, x509.NewCertPool()); !r.Invalid {
		t.Fatal(r)
	}
}
func manualOpenSSL(t *testing.T, a *testtsa.Authority, der, manifest []byte) {
	t.Helper()
	if _, e := exec.LookPath("openssl"); e != nil {
		t.Fatal("OpenSSL required for manual verification regression")
	}
	dir := t.TempDir()
	for n, b := range map[string][]byte{"token.tsr": der, "manifest.json": manifest, "root.pem": a.RootPEM()} {
		if e := os.WriteFile(filepath.Join(dir, n), b, 0600); e != nil {
			t.Fatal(e)
		}
	}
	// These are the commands packaged in VERIFY.txt, with its genTime converted
	// to Unix seconds. Embedded intermediate certificates are used by OpenSSL.
	display, e := exec.Command("openssl", "ts", "-reply", "-in", filepath.Join(dir, "token.tsr"), "-text").CombinedOutput()
	if e != nil {
		t.Fatalf("%v: %s", e, display)
	}
	// The epoch argument below comes from the token time, not today's clock.
	cmd := exec.Command("openssl", "ts", "-verify", "-data", filepath.Join(dir, "manifest.json"), "-in", filepath.Join(dir, "token.tsr"), "-CAfile", filepath.Join(dir, "root.pem"), "-attime", new(big.Int).SetInt64(a.Options.Time.Unix()).String())
	out, e := cmd.CombinedOutput()
	if e != nil || !bytes.Contains(out, []byte("Verification: OK")) {
		t.Fatalf("manual command: %v\n%s", e, out)
	}
	t.Logf("OpenSSL at historical genTime: %s", out)
}
func TestAcquisition(t *testing.T) {
	good := authority(t, testtsa.Options{})
	server := httptest.NewServer(good)
	defer server.Close()
	roots := x509.NewCertPool()
	roots.AddCert(good.Root)
	for _, tc := range []struct {
		name    string
		handler http.Handler
		timeout time.Duration
		want    string
	}{
		{"valid", good, time.Second, ""},
		{"unknown root", good, time.Second, ""},
		{"nonce", authority(t, testtsa.Options{WrongNonce: true}), time.Second, "nonce mismatch"},
		{"imprint", authority(t, testtsa.Options{WrongImprint: true}), time.Second, "imprint mismatch"},
		{"rejection", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503) }), time.Second, "503"},
		{"response limit", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, strings.Repeat("x", (1<<20)+1)) }), time.Second, "1 MiB"},
		{"deadline", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			select {
			case <-r.Context().Done():
			case <-time.After(300 * time.Millisecond):
			}
		}), 50 * time.Millisecond, "deadline"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := httptest.NewServer(tc.handler)
			defer s.Close()
			pool := roots
			if tc.name == "unknown root" {
				pool = x509.NewCertPool()
			}
			a := request(context.Background(), []byte("manifest"), s.URL, pool, tc.timeout, http.DefaultTransport)
			if tc.want == "" {
				if len(a.DER) == 0 {
					t.Fatalf("%+v", a)
				}
			} else if len(a.DER) > 0 || !strings.Contains(a.Error, tc.want) {
				t.Fatalf("%+v", a)
			}
		})
	}
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503) }))
	defer bad.Close()
	old := Defaults
	Defaults = []string{bad.URL, server.URL}
	defer func() { Defaults = old }()
	got := Acquire(context.Background(), []byte("manifest"), nil, roots)
	if len(got) != 2 || got[0].Error == "" || len(got[1].DER) == 0 {
		t.Fatal(got)
	}
	got = Acquire(context.Background(), []byte("manifest"), []string{server.URL, bad.URL}, roots)
	if len(got) != 2 || len(got[0].DER) == 0 || got[1].Error == "" {
		t.Fatal(got)
	}
	// GrantedWithMods is also a granted response.
	der := token(t, good, []byte("manifest"))
	var resp response
	asn1.Unmarshal(der, &resp)
	resp.Status.Status = 1
	der, _ = asn1.Marshal(resp)
	if !Check(der, []byte("manifest"), roots).Trusted {
		t.Fatal("granted-with-modifications rejected")
	}
}
func TestProxyDeadlines(t *testing.T) {
	// A fresh transport uses the same ProxyFromEnvironment behavior as the
	// production default, with an explicit proxy here to isolate test processes.
	closed, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	addr := closed.Addr().String()
	closed.Close()
	proxy, _ := url.Parse("http://" + addr)
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = http.ProxyURL(proxy)
	defer transport.CloseIdleConnections()
	r := request(context.Background(), []byte("manifest"), "http://timestamp.example.invalid", x509.NewCertPool(), 100*time.Millisecond, transport)
	if !strings.Contains(r.Error, "proxy settings") {
		t.Fatal(r)
	}
	stall := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(300 * time.Millisecond):
		}
	}))
	defer stall.Close()
	proxy, _ = url.Parse(stall.URL)
	transport.Proxy = http.ProxyURL(proxy)
	for _, endpoint := range []string{"http://timestamp.example.invalid", "https://timestamp.example.invalid"} {
		start := time.Now()
		r = request(context.Background(), []byte("manifest"), endpoint, x509.NewCertPool(), 100*time.Millisecond, transport)
		if r.Error == "" || time.Since(start) > time.Second {
			t.Fatalf("deadline: %+v", r)
		}
	}
}

func TestCertificateMustBeValidAtTimestampTime(t *testing.T) {
	a := authority(t, testtsa.Options{})
	a.Options.Time = time.Now().UTC().AddDate(-2, 0, 0)
	roots := x509.NewCertPool()
	roots.AddCert(a.Root)
	der := token(t, a, []byte("manifest"))
	for _, pool := range []*x509.CertPool{roots, x509.NewCertPool()} {
		result := Check(der, []byte("manifest"), pool)
		if result.Invalid || result.Trusted || result.Usable || !strings.Contains(result.Reason, "genTime") {
			t.Fatalf("invalid historical chain accepted: %+v", result)
		}
		for _, lang := range []string{"en", "de"} {
			text := Explanation(result, lang)
			want := "timestamp's time"
			if lang == "de" {
				want = "Zeitpunkt des Zeitstempels"
			}
			if !strings.Contains(text, want) {
				t.Fatal(text)
			}
			if strings.Contains(text, "root") || strings.Contains(text, "Stamm") || strings.Contains(text, "x509") {
				t.Fatal(text)
			}
		}
	}
}
func TestRequestsUseFresh64BitNonces(t *testing.T) {
	a := authority(t, testtsa.Options{})
	nonces := []*big.Int{}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		q, e := timestamp.ParseRequest(b)
		if e != nil {
			t.Error(e)
			w.WriteHeader(400)
			return
		}
		if !q.Certificates || q.HashAlgorithm != crypto.SHA256 || q.Nonce == nil || q.Nonce.BitLen() > 64 {
			t.Error("request fields do not match timestamp contract")
		}
		nonces = append(nonces, q.Nonce)
		der, e := a.Response(q)
		if e != nil {
			t.Error(e)
			w.WriteHeader(500)
			return
		}
		w.Write(der)
	}))
	defer s.Close()
	for range 2 {
		attempts := Acquire(context.Background(), []byte("manifest"), []string{s.URL}, x509.NewCertPool())
		if len(attempts) != 1 || len(attempts[0].DER) == 0 {
			t.Fatal(attempts)
		}
	}
	if len(nonces) != 2 || nonces[0].Cmp(nonces[1]) == 0 {
		t.Fatal("timestamp requests reused a nonce")
	}
}

func TestTimestampAccuracyPresence(t *testing.T) {
	for _, tc := range []struct {
		name string
		omit bool
		want string
	}{
		{"stated", false, "1s"},
		{"omitted", true, "unspecified"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := authority(t, testtsa.Options{OmitAccuracy: tc.omit})
			roots := x509.NewCertPool()
			roots.AddCert(a.Root)
			r := Check(token(t, a, []byte("manifest")), []byte("manifest"), roots)
			if !r.Trusted || r.Accuracy != tc.want {
				t.Fatalf("accuracy verdict: %+v", r)
			}
		})
	}
	r := Check([]byte("malformed"), []byte("manifest"), x509.NewCertPool())
	if !r.Invalid || r.Time != "unavailable" || r.Serial != "unavailable" || r.Accuracy != "unavailable" {
		t.Fatalf("unreadable metadata: %+v", r)
	}
}
