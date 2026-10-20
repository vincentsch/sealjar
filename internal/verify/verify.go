// Package verify reports capture, file integrity and timestamp results separately.
package verify

import (
	"fmt"
	"io"
	"strings"

	packagefmt "github.com/vincentsch/signetleaf/internal/pkg"
	"github.com/vincentsch/signetleaf/internal/repo"
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
	Capture     Answer   `json:"capture"`
	Files       Files    `json:"files"`
	Timestamp   Answer   `json:"timestamp"`
	Limitations []string `json:"limitations"`
}

func Check(r *packagefmt.Reader) (Result, int, error) {
	differences, matching, err := r.Check()
	if err != nil {
		return Result{}, 2, err
	}
	targets := []string{}
	for _, t := range r.Manifest.Targets {
		targets = append(targets, fmt.Sprintf("Repository %s at %s with %d uncommitted changes.", t.Name, t.Commit, t.Changes))
	}
	result := Result{
		Capture:     Answer{"Complete", strings.Join(targets, " ")},
		Files:       Files{"Unchanged", fmt.Sprintf("All %d files listed in the manifest match their recorded hashes.", matching), matching, differences},
		Timestamp:   Answer{"Not timestamped", "This package has no timestamp."},
		Limitations: []string{"Hashes alone do not establish the authenticity of an untimestamped manifest.", "A timestamp's time is an upper bound for when the files existed, not an independently checked capture time.", "Offline checks cannot see later certificate revocation."},
	}
	for _, n := range r.Names {
		if n != "manifest.json" && packagefmt.Exempt(n) && strings.HasSuffix(n, ".tsr") {
			result.Timestamp = Answer{"Unchecked", "Timestamp tokens are present. This version cannot validate them; use the manual steps in VERIFY.txt."}
			break
		}
	}
	code := 11
	if len(differences) > 0 {
		result.Files.State = "Changed"
		result.Files.Detail = "Package files differ from the manifest."
		code = 10
	}

	if code != 10 {
		for _, target := range r.Manifest.Targets {
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
	for _, line := range r.Limitations {
		fmt.Fprintln(w, "  "+line)
	}
}
