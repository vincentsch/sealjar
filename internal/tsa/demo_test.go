package tsa

import (
	"context"
	"crypto/x509"
	"net/http/httptest"
	"testing"

	"github.com/vincentsch/sealjar/internal/tsa/demotsa"
)

func TestDemoProvenanceDoesNotGrantTrust(t *testing.T) {
	authority, err := demotsa.New()
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(authority)
	defer server.Close()
	manifest := []byte("demo manifest")
	attempts := Acquire(context.Background(), manifest, []string{server.URL}, x509.NewCertPool())
	if len(attempts) != 1 || len(attempts[0].DER) == 0 {
		t.Fatalf("%+v", attempts)
	}
	token := Check(attempts[0].DER, manifest, x509.NewCertPool())
	if !token.Demo || token.Trusted || token.Invalid || !token.Usable || token.DisplayAuthority() != "Sealjar Demo TSA" {
		t.Fatalf("%+v", token)
	}
	roots := x509.NewCertPool()
	roots.AppendCertsFromPEM(demotsa.RootPEM)
	token = Check(attempts[0].DER, manifest, roots)
	if !token.Demo || !token.Trusted || token.Invalid {
		t.Fatalf("explicit trust: %+v", token)
	}
	roots, err = Roots("")
	if err != nil {
		t.Fatal(err)
	}
	if Check(attempts[0].DER, manifest, roots).Trusted {
		t.Fatal("demo root leaked into normal roots")
	}
}
