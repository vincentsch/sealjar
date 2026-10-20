package tsa

import (
	"strings"
	"testing"
)

func TestPlainTimestampExplanations(t *testing.T) {
	for _, tc := range []struct {
		reason, en, de string
		invalid        bool
	}{
		{"asn1: damaged", "damaged", "beschädigt", true},
		{"Invalid signed timestamp content type", "damaged", "beschädigt", false},
		{"Timestamp requires exactly one signed content type", "damaged", "beschädigt", false},
		{"SHA-256 manifest imprint mismatch", "different manifest", "anderen Manifest", true},
		{"Invalid timestamp signature: raw diagnostic", "does not match", "passt nicht", true},
		{"Exactly one identifiable signer is required", "signing certificate", "Signaturzertifikat", false},
		{"Missing signer certificate; signature could not be checked", "signing certificate", "Signaturzertifikat", false},
		{"Signed certificate identifier does not match the signer certificate", "signing certificate", "Signaturzertifikat", false},
		{"Signer purpose must be critical timeStamping only", "signing certificate", "Signaturzertifikat", false},
		{"Untrusted certificate chain at genTime: raw diagnostic", "could not be validated", "nicht als gültig bestätigt", false},
		{"Certificate chain is invalid at genTime: raw diagnostic", "was not valid", "nicht gültig", false},
		{"unknown raw diagnostic", "could not be checked", "nicht geprüft", false},
	} {
		for lang, want := range map[string]string{"en": tc.en, "de": tc.de} {
			got := Explanation(Result{Reason: tc.reason, Invalid: tc.invalid}, lang)
			if !strings.Contains(got, want) || strings.Contains(got, "raw diagnostic") {
				t.Fatalf("%s: %s", tc.reason, got)
			}
		}
	}
	for _, lang := range []string{"en", "de"} {
		if !strings.Contains(Explanation(Result{Trusted: true}, lang), map[string]string{"en": "Valid signature", "de": "Signatur ist gültig"}[lang]) {
			t.Fatal(lang)
		}
	}
}
