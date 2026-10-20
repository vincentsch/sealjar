// Package web captures page evidence and host facts.
package web

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

const masked = "<masked>"

func secretName(s string) bool {
	s = strings.ToLower(s)
	switch s {
	case "token", "access_token", "api_key", "key", "secret", "password", "sig", "signature", "session", "sid":
		return true
	}
	return strings.HasPrefix(s, "x-amz-")
}
func MaskURL(raw string) string {
	u, e := url.Parse(raw)
	if e != nil {
		return "<invalid URL>"
	}
	u.User = nil
	q := u.Query()
	for k := range q {
		if secretName(k) {
			q[k] = []string{masked}
		}
	}
	u.RawQuery = q.Encode()
	return u.String()
}

var urlText = regexp.MustCompile(`https?://[^\s<>"'\\]+`)
var querySecretText = regexp.MustCompile(`(?i)([?&](?:token|access_token|api_key|key|secret|password|sig|signature|session|sid|x-amz-[^=&#\s]+)=)[^&#\s<>"'\\]+`)
var secretText = regexp.MustCompile(`(?i)(authorization|proxy-authorization|cookie|set-cookie|password|access_token|api_key|token|secret)\s*[:=]\s*[^\s,;]+`)

func redactText(s, auth string) string {
	if auth != "" {
		s = strings.ReplaceAll(s, auth, masked)
		s = strings.ReplaceAll(s, base64.StdEncoding.EncodeToString([]byte(auth)), masked)
		_, password, _ := strings.Cut(auth, ":")
		if password != "" {
			s = strings.ReplaceAll(s, password, masked)
		}
	}
	s = urlText.ReplaceAllStringFunc(s, MaskURL)
	s = querySecretText.ReplaceAllString(s, "${1}%3Cmasked%3E")
	return secretText.ReplaceAllString(s, "$1=<masked>")
}
func maskHeaders(h map[string]any, auth string) map[string]string {
	out := map[string]string{}
	for k, v := range h {
		switch strings.ToLower(k) {
		case "authorization", "proxy-authorization", "cookie", "set-cookie":
			out[k] = masked
		case "location", "content-location", "referer", "origin":
			out[k] = MaskURL(toString(v))
		default:
			out[k] = redactText(toString(v), auth)
		}
	}
	return out
}
func toString(v any) string { return fmt.Sprint(v) }
