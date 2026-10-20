package tsa

import "strings"

// Explanation keeps technical diagnostics in the token facts, away from the primary text.
func Explanation(t Result, lang string) string {
	en, de := "The timestamp could not be checked.", "Der Zeitstempel konnte nicht geprüft werden."
	switch {
	case t.Trusted:
		en, de = "Valid signature; the certificate chain leads to a trusted root at the timestamp's time.", "Die Signatur ist gültig. Die Zertifikatskette führt zum Zeitpunkt des Zeitstempels zu einem vertrauenswürdigen Stammzertifikat."
	case strings.HasPrefix(t.Reason, "SHA-256 manifest imprint mismatch"):
		en, de = "The timestamp belongs to a different manifest.", "Der Zeitstempel gehört zu einem anderen Manifest."
	case strings.HasPrefix(t.Reason, "Invalid timestamp signature:"):
		en, de = "The timestamp's signature does not match its content.", "Die Signatur des Zeitstempels passt nicht zu seinem Inhalt."
	case strings.HasPrefix(t.Reason, "Exactly one identifiable signer"), strings.HasPrefix(t.Reason, "Missing signer certificate"), strings.HasPrefix(t.Reason, "Signed certificate identifier"), strings.HasPrefix(t.Reason, "Signer purpose"):
		en, de = "The timestamp's signing certificate is missing, does not match, or is not meant for timestamps.", "Das Signaturzertifikat fehlt, passt nicht zum Zeitstempel oder ist nicht für Zeitstempel vorgesehen."
	case strings.HasPrefix(t.Reason, "Untrusted certificate chain at genTime:"):
		en, de = "The certificate chain could not be validated at the timestamp's time.", "Die Zertifikatskette konnte zum Zeitpunkt des Zeitstempels nicht als gültig bestätigt werden."
	case strings.HasPrefix(t.Reason, "Certificate chain is invalid at genTime:"):
		en, de = "The certificate chain was not valid at the timestamp's time.", "Die Zertifikatskette war zum Zeitpunkt des Zeitstempels nicht gültig."
	case t.Invalid, strings.HasPrefix(t.Reason, "Invalid signed timestamp content type"), strings.HasPrefix(t.Reason, "Timestamp requires exactly one signed content type"):
		en, de = "The timestamp file is damaged or is not a timestamp.", "Die Zeitstempeldatei ist beschädigt oder enthält keinen Zeitstempel."
	}
	if lang == "de" {
		return de
	}
	return en
}

func DemoProvenance(t Result, lang string) string {
	if !t.Demo {
		return ""
	}
	if t.Trusted {
		if lang == "de" {
			return "Dieser Zeitstempel verwendet den öffentlichen Testschlüssel von Signetleaf. Damit kann jeder solche Zeitstempel erstellen. Das Vertrauen beruht allein auf dem angegebenen Stammzertifikat. Dieser Zeitstempel liefert keinen unabhängigen Nachweis des Zeitpunkts."
		}
		return "This token uses Signetleaf's public test key. Anyone can create such tokens. Trust comes only from the supplied root; this token provides no independent evidence of time."
	}
	if lang == "de" {
		return "Demo-Zeitstempel, nicht vertrauenswürdig. Er stammt von Signetleafs eingebauter Teststelle und zeigt, wie ein Zeitstempel aussieht."
	}
	return "Demo timestamp, not trusted. It comes from Signetleaf's built-in test authority and shows how a timestamp looks."
}

func TimeLimitation(lang string) string {
	if lang == "de" {
		return "Ein Zeitstempel einer unabhängigen, vertrauenswürdigen Stelle belegt, dass die Dateien spätestens zu diesem Zeitpunkt vorlagen. Das gilt nicht für Zeitstempel mit dem öffentlichen Testschlüssel von Signetleaf."
	}
	return "A timestamp from an independent trusted authority gives an upper bound for when the files existed. This does not apply to tokens made with Signetleaf's public demo key."
}
