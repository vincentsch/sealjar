// Package demotsa holds deliberately public test material for the offline demo.
// Its private key is public and must never be used for a trusted authority.
package demotsa

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	_ "embed"
	"encoding/pem"
	"math/big"
	"net/http"
	"time"

	"github.com/vincentsch/signetleaf/internal/tsa/testtsa"
)

const Label = "Demo timestamp, not trusted"

//go:embed root.pem
var RootPEM []byte

// Deliberately public test key; never suitable for a trusted authority.
//
//go:embed public-test-key.pem
var publicTestKey []byte

func Root() *x509.Certificate {
	block, _ := pem.Decode(RootPEM)
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		panic(err)
	}
	return cert
}

type Authority struct{ *testtsa.Authority }

func (a *Authority) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	current := *a.Authority
	current.Options.Time = time.Now().UTC()
	current.ServeHTTP(w, r)
}

func New() (*Authority, error) {
	block, _ := pem.Decode(publicTestKey)
	rootKey, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, err
	}
	cert := &x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: "Signetleaf Demo TSA, not trusted"}, NotBefore: now.Add(-time.Hour), NotAfter: now.AddDate(1, 0, 0), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageTimeStamping}, BasicConstraintsValid: true}
	// Timestamp signers require a critical, exclusive extended key usage.
	cert.ExtraExtensions = []pkix.Extension{{Id: []int{2, 5, 29, 37}, Critical: true, Value: []byte{0x30, 0x0a, 0x06, 0x08, 0x2b, 0x06, 0x01, 0x05, 0x05, 0x07, 0x03, 0x08}}}
	root := Root()
	der, err := x509.CreateCertificate(rand.Reader, cert, root, &key.PublicKey, rootKey)
	if err != nil {
		return nil, err
	}
	signer, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, err
	}
	return &Authority{&testtsa.Authority{Root: root, Intermediate: root, Signer: signer, Key: key}}, nil
}
