// Package testtsa provides an ephemeral timestamp authority for local tests.
package testtsa

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"time"

	"github.com/digitorus/pkcs7"
	"github.com/digitorus/timestamp"
)

type Options struct {
	DualBinding                                                                                        bool
	OmitAccuracy                                                                                       bool
	Time                                                                                               time.Time
	NonCritical, MultiPurpose, MissingSigner, WrongBinding, WrongImprint, WrongNonce, InvalidSignature bool
}
type Authority struct {
	Root, Intermediate, Signer *x509.Certificate
	Key                        *ecdsa.PrivateKey
	Options                    Options
}

func New(opt Options) (*Authority, error) {
	now := time.Now().UTC()
	at := opt.Time
	if at.IsZero() {
		at = now
	}
	opt.Time = at
	makeCert := func(name string, serial int64, ca bool, parent *x509.Certificate, parentKey *ecdsa.PrivateKey) (*x509.Certificate, *ecdsa.PrivateKey, error) {
		key, e := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if e != nil {
			return nil, nil, e
		}
		c := &x509.Certificate{SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: name}, NotBefore: at.Add(-24 * time.Hour), NotAfter: at.Add(24 * time.Hour), BasicConstraintsValid: true, IsCA: ca, KeyUsage: x509.KeyUsageDigitalSignature}
		if ca {
			c.NotAfter = now.AddDate(10, 0, 0)
			c.KeyUsage |= x509.KeyUsageCertSign
		} else {
			usages := []asn1.ObjectIdentifier{{1, 3, 6, 1, 5, 5, 7, 3, 8}}
			if opt.MultiPurpose {
				usages = append(usages, asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 3, 1})
			}
			b, _ := asn1.Marshal(usages)
			c.ExtraExtensions = []pkix.Extension{{Id: asn1.ObjectIdentifier{2, 5, 29, 37}, Critical: !opt.NonCritical, Value: b}}
		}
		if parent == nil {
			parent = c
			parentKey = key
		}
		der, e := x509.CreateCertificate(rand.Reader, c, parent, &key.PublicKey, parentKey)
		if e != nil {
			return nil, nil, e
		}
		parsed, e := x509.ParseCertificate(der)
		return parsed, key, e
	}
	root, rk, e := makeCert("Sealjar Test Root, not trusted", 1, true, nil, nil)
	if e != nil {
		return nil, e
	}
	inter, ik, e := makeCert("Sealjar Test Intermediate", 2, true, root, rk)
	if e != nil {
		return nil, e
	}
	signer, key, e := makeCert("Sealjar Test Timestamp Authority", 3, false, inter, ik)
	if e != nil {
		return nil, e
	}
	return &Authority{root, inter, signer, key, opt}, nil
}
func (a *Authority) RootPEM() []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: a.Root.Raw})
}
func (a *Authority) Response(req *timestamp.Request) ([]byte, error) {
	digest := append([]byte{}, req.HashedMessage...)
	if a.Options.WrongImprint {
		digest[0] ^= 1
	}
	nonce := req.Nonce
	if a.Options.WrongNonce {
		nonce = big.NewInt(-1)
	}
	t := timestamp.Timestamp{HashAlgorithm: req.HashAlgorithm, HashedMessage: digest, Time: a.Options.Time, Accuracy: time.Second, Nonce: nonce, Policy: asn1.ObjectIdentifier{1, 2, 3, 4}, AddTSACertificate: true}
	if a.Options.OmitAccuracy {
		t.Accuracy = 0
	}
	der, e := t.CreateResponseWithOpts(a.Signer, a.Key, crypto.SHA256)
	if e != nil {
		return nil, e
	}
	var response struct {
		Status asn1.RawValue
		Token  asn1.RawValue
	}
	_, e = asn1.Unmarshal(der, &response)
	if e != nil {
		return nil, e
	}
	cms, e := pkcs7.Parse(response.Token.FullBytes)
	if e != nil {
		return nil, e
	}
	sd, e := pkcs7.NewSignedData(cms.Content)
	if e != nil {
		return nil, e
	}
	sd.SetDigestAlgorithm(pkcs7.OIDDigestAlgorithmSHA256)
	sd.SetContentType(asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 16, 1, 4})
	hash := sha256.Sum256(a.Signer.Raw)
	if a.Options.WrongBinding {
		hash[0] ^= 1
	}
	binding := struct{ Certs []struct{ Hash []byte } }{}
	binding.Certs = append(binding.Certs, struct{ Hash []byte }{hash[:]})
	attrs := []pkcs7.Attribute{{Type: asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 16, 2, 47}, Value: binding}}
	if a.Options.DualBinding {
		h := sha1.Sum(a.Signer.Raw)
		legacy := struct{ Certs []struct{ Hash []byte } }{}
		legacy.Certs = append(legacy.Certs, struct{ Hash []byte }{h[:]})
		attrs = append(attrs, pkcs7.Attribute{Type: asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 16, 2, 12}, Value: legacy})
	}
	e = sd.AddSigner(a.Signer, a.Key, pkcs7.SignerInfoConfig{SkipCertificates: a.Options.MissingSigner, SkipSigningTime: true, ExtraSignedAttributes: attrs})
	if e != nil {
		return nil, e
	}
	sd.AddCertificate(a.Intermediate)
	sd.AddCertificate(a.Root)
	token, e := sd.Finish()
	if e != nil {
		return nil, e
	}
	if a.Options.InvalidSignature {
		p, e := pkcs7.Parse(token)
		if e != nil {
			return nil, e
		}
		sig := p.Signers[0].EncryptedDigest
		for i := 0; i+len(sig) <= len(token); i++ {
			if string(token[i:i+len(sig)]) == string(sig) {
				token[i+len(sig)-1] ^= 1
				break
			}
		}
	}
	return asn1.Marshal(struct {
		Status struct{ Status int }
		Token  asn1.RawValue
	}{Token: asn1.RawValue{FullBytes: token}})
}
func (a *Authority) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	b, e := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if e != nil {
		http.Error(w, "read failed", 400)
		return
	}
	req, e := timestamp.ParseRequest(b)
	if e != nil {
		http.Error(w, "invalid request", 400)
		return
	}
	if req.HashAlgorithm != crypto.SHA256 || !req.Certificates || req.Nonce == nil {
		http.Error(w, "required request fields missing", 400)
		return
	}
	b, e = a.Response(req)
	if e != nil {
		http.Error(w, fmt.Sprint(e), 500)
		return
	}
	w.Header().Set("Content-Type", "application/timestamp-reply")
	w.Write(b)
}
