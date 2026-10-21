// Package verify reports capture, file integrity and timestamp results separately.
package verify

import (
	"crypto/x509"
	"fmt"
	"io"
	"strings"

	packagefmt "github.com/vincentsch/sealjar/internal/pkg"
	"github.com/vincentsch/sealjar/internal/repo"
	"github.com/vincentsch/sealjar/internal/tsa"
)

type Answer struct {
	State  string `json:"state"`
	Detail string `json:"detail"`
}
type Files struct {
	State       string                  `json:"state"`
	Detail      string                  `json:"detail"`
	Matching    int                     `json:"matching"`
	Differences []packagefmt.Difference `json:"differences"`
}
type Result struct {
	Capture     Answer       `json:"capture"`
	Files       Files        `json:"files"`
	Timestamp   Answer       `json:"timestamp"`
	Tokens      []tsa.Result `json:"tokens"`
	Limitations []string     `json:"limitations"`
}

func Check(r *packagefmt.Reader, additions ...*x509.CertPool) (Result, int, error) {
	var roots *x509.CertPool
	if len(additions) > 0 {
		roots = additions[0]
	} else {
		var err error
		roots, err = tsa.Roots("")
		if err != nil {
			return Result{}, 2, err
		}
	}
	differences, matching, err := r.Check()
	if err != nil {
		return Result{}, 2, err
	}
	targets := []string{}
	for _, t := range r.Manifest.Targets {
		if t.Kind == "web" {
			targets = append(targets, fmt.Sprintf("Page %s: %s. %s", t.URL, t.State, strings.Join(t.Notes, "; ")))
			continue
		}
		word := "changes"
		if t.Changes == 1 {
			word = "change"
		}
		targets = append(targets, fmt.Sprintf("Repository %s at %s with %d uncommitted %s.", t.Name, t.Commit, t.Changes, word))
	}
	result := Result{
		Capture:     Answer{"Complete", strings.Join(targets, " ")},
		Files:       Files{"Unchanged", fmt.Sprintf("All %d files listed in the manifest match their recorded hashes.", matching), matching, differences},
		Timestamp:   Answer{"Not timestamped", "This package has no timestamp."},
		Limitations: []string{"Hashes alone do not establish the authenticity of an untimestamped manifest.", "A timestamp's time is an upper bound for when the files existed, not an independently checked capture time.", "Offline checks cannot see later certificate revocation."},
	}
	for _, target := range r.Manifest.Targets {
		if target.State != "complete" {
			result.Capture.State = "Problems"
		}
	}
	result.Tokens = []tsa.Result{}
	code := 11
	trusted := false
	for _, n := range r.Names {
		if n != "manifest.json" && packagefmt.Exempt(n) && strings.HasSuffix(n, ".tsr") {
			der, e := r.Read(n)
			token := tsa.Result{Path: n, Authority: "Unavailable", Time: "unavailable", Serial: "unavailable", Accuracy: "unavailable", Invalid: true}
			if e != nil {
				token.Reason = e.Error()
			} else {
				token = tsa.Check(der, r.ManifestBytes, roots)
				token.Path = n
			}
			result.Tokens = append(result.Tokens, token)
			if token.Invalid {
				code = 10
			}
			trusted = trusted || token.Trusted
		}
	}
	if len(result.Tokens) > 0 {
		result.Timestamp = Answer{"Untrusted", "No timestamp is trusted."}
	}
	if trusted {
		result.Limitations = result.Limitations[1:]
		result.Timestamp = Answer{"Trusted", "At least one timestamp has a valid signature and trusted certificate chain."}
		if code != 10 {
			code = 0
		}
	}
	if code == 10 {
		result.Timestamp = Answer{"Invalid", "A timestamp has a structural, imprint or signature failure."}
	}
	for _, token := range result.Tokens {
		if token.Demo {
			result.Timestamp.Detail += " " + tsa.DemoProvenance(token, "en")
		}
	}
	result.Limitations = append(result.Limitations, "Sealjar does not assess eIDAS qualification.")
	if len(differences) > 0 {
		result.Files.State = "Changed"
		result.Files.Detail = "Package files differ from the manifest."
		code = 10
	}

	if code != 10 {
		for _, target := range r.Manifest.Targets {
			if target.Kind != "repo" {
				continue
			}
			if err := repo.CheckSnapshot(r, target.Payload, ""); err != nil {
				if !packagefmt.IsMalformed(err) {
					return result, 2, err
				}
				result.Files.State = "Malformed"
				result.Files.Detail = err.Error()
				code = 10
			}
		}
	}
	return result, code, nil
}
func Malformed(err error) Result {
	return Result{Capture: Answer{"Unknown", "The capture record could not be read."}, Files: Files{State: "Malformed", Detail: err.Error(), Differences: []packagefmt.Difference{}}, Timestamp: Answer{"Unknown", "A malformed package cannot be checked for timestamps."}, Limitations: []string{"Hashes alone do not establish the authenticity of an untimestamped manifest."}}
}
func Render(w io.Writer, r Result) {
	fmt.Fprintf(w, "Capture\n  %s. %s\n\nFiles\n  %s. %s\n", r.Capture.State, r.Capture.Detail, r.Files.State, r.Files.Detail)
	for _, d := range r.Files.Differences {
		fmt.Fprintf(w, "  %s: %s\n", d.State, d.Path)
	}
	fmt.Fprintf(w, "\nTimestamp\n  %s. %s\n", r.Timestamp.State, r.Timestamp.Detail)
	for _, t := range r.Tokens {
		fmt.Fprintf(w, "  %s at %s: %s\n", t.DisplayAuthority(), t.DisplayTime(), tsa.Explanation(t, "en"))
	}
	for _, line := range r.Limitations {
		if strings.HasPrefix(line, "A timestamp's time is an upper bound") {
			line = tsa.TimeLimitation("en")
		}
		fmt.Fprintln(w, "  "+line)
	}
}
