package cli

import (
	"crypto/x509"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vincentsch/rungrad"
	packagefmt "github.com/vincentsch/signetleaf/internal/pkg"
	"github.com/vincentsch/signetleaf/internal/report"
	"github.com/vincentsch/signetleaf/internal/tsa"
	"github.com/vincentsch/signetleaf/internal/verify"
)

// Report copies existing reader artifacts; report generation happens at capture.
func reportCommand() *rungrad.Command {
	var root, out, lang string
	var pdf bool
	c := command("report <package>", "Copy stored reports after checking package integrity and timestamp trust")
	c.Mutates = true
	c.Args = cobra.ExactArgs(1)
	c.Examples = []string{"signetleaf report capture.signetleaf.zip --out reader", "signetleaf report capture.signetleaf.zip --lang de --pdf --out reader-de --dry-run"}
	c.Related = []string{"signetleaf verify", "signetleaf restore"}
	c.Configure = func(cmd *cobra.Command) {
		f := cmd.Flags()
		f.StringVar(&root, "tsa-root", "", "Add trusted timestamp roots from a PEM file")
		f.StringVar(&out, "out", "", "Write copies to this new reader folder")
		f.StringVar(&lang, "lang", "both", "Report language: en, de or both")
		f.BoolVar(&pdf, "pdf", false, "Include stored PDFs (no browser required)")
	}
	c.Run = func(f *rungrad.Factory, cmd *cobra.Command, args []string) error {
		if lang != "en" && lang != "de" && lang != "both" {
			return rungrad.NewError(1, "Report language must be en, de or both.")
		}
		roots, e := tsa.Roots(root)
		if e != nil {
			return failure(2, e)
		}
		r, e := packagefmt.Open(args[0])
		if e != nil {
			return inputError(e)
		}
		defer r.Close()
		v, code, e := verify.Check(r, roots)
		if e != nil {
			return inputError(e)
		}
		if code == 10 {
			var b strings.Builder
			verify.Render(&b, v)
			return failure(10, fmt.Errorf("report stopped: %s", b.String()))
		}
		selected := []string{}
		langs := []string{lang}
		if lang == "both" {
			langs = []string{"en", "de"}
		}
		for _, l := range langs {
			names := []string{"report/report." + l + ".html"}
			if pdf {
				names = append(names, "report/report."+l+".pdf")
			}
			for _, name := range names {
				if _, ok := r.Entries[name]; !ok {
					return rungrad.NewError(5, "The package has no stored "+name+".")
				}
				selected = append(selected, name)
			}
		}
		for _, n := range r.Names {
			if n == "VERIFY.txt" || (strings.HasPrefix(n, "web/") && (strings.HasSuffix(n, ".png") || strings.HasSuffix(n, ".jpg"))) {
				selected = append(selected, n)
			}
		}
		if out == "" {
			out = strings.TrimSuffix(args[0], ".zip")
		}
		out = filepath.Clean(out)
		if e = readerDestination(out); e != nil {
			return failure(1, e)
		}
		result := struct {
			Directory    string        `json:"directory"`
			DryRun       bool          `json:"dry_run"`
			Verification verify.Result `json:"verification"`
		}{resultPath(out), f.DryRun(), v}
		if !f.DryRun() {
			seal, e := report.Seal(r.Manifest, r.ManifestBytes, v.Tokens, filepath.Base(strings.TrimSuffix(args[0], ".zip")))
			if e != nil {
				return failure(2, e)
			}
			if e = extractReader(r, out, selected, seal); e != nil {
				return failure(2, e)
			}
		}
		return f.WriteResult(result, func(w io.Writer) {
			if f.DryRun() {
				fmt.Fprintln(w, "DRY RUN: would copy stored reports; no files were written.")
			} else {
				fmt.Fprintf(w, "Copied stored reports to %s. These are copies taken from the package.\n", result.Directory)
			}
			verify.Render(w, v)
		})
	}
	return c
}

func readerDestination(out string) error {
	info, e := os.Lstat(out)
	if os.IsNotExist(e) {
		return nil
	}
	if e != nil {
		return e
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("Reader destination must be an empty folder. Choose another --out folder.")
	}
	entries, e := os.ReadDir(out)
	if e != nil {
		return e
	}
	if len(entries) > 0 {
		return fmt.Errorf("Reader folder is not empty. Choose another --out folder.")
	}
	return nil
}
func extractReader(r *packagefmt.Reader, out string, names []string, seal []byte) error {
	stage, e := os.MkdirTemp(filepath.Dir(out), ".signetleaf-reader-")
	if e != nil {
		return e
	}
	defer os.RemoveAll(stage)
	for _, n := range names {
		dest := filepath.Join(stage, filepath.FromSlash(n))
		if e = os.MkdirAll(filepath.Dir(dest), 0700); e != nil {
			return e
		}
		in, e := r.Entries[n].Open()
		if e != nil {
			return e
		}
		dst, e := os.OpenFile(dest, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			in.Close()
			return e
		}
		_, e = io.Copy(dst, in)
		ce := in.Close()
		de := dst.Close()
		if e != nil {
			return e
		}
		if ce != nil {
			return ce
		}
		if de != nil {
			return de
		}
	}
	if e = os.WriteFile(filepath.Join(stage, "SEAL.html"), seal, 0600); e != nil {
		return e
	}
	if e = readerDestination(out); e != nil {
		return e
	}
	// Remove accepts only an empty destination; it cannot remove another reader tree.
	if _, err := os.Lstat(out); err == nil {
		if e = os.Remove(out); e != nil {
			return e
		}
	}
	return os.Rename(stage, out)
}
func writePackageReader(name, lang string, pdf bool, roots *x509.CertPool) (string, error) {
	r, e := packagefmt.Open(name)
	if e != nil {
		return "", e
	}
	defer r.Close()
	v, code, e := verify.Check(r, roots)
	if e != nil {
		return "", e
	}
	if code == 10 {
		return "", fmt.Errorf("new package failed integrity checks: %s", v.Files.Detail)
	}
	selected := []string{}
	for _, n := range r.Names {
		if n == "VERIFY.txt" || strings.HasPrefix(n, "web/") && (strings.HasSuffix(n, ".png") || strings.HasSuffix(n, ".jpg")) {
			selected = append(selected, n)
		}
		for _, l := range report.Languages(lang) {
			if n == "report/report."+l+".html" || pdf && n == "report/report."+l+".pdf" {
				selected = append(selected, n)
			}
		}
	}
	seal, e := report.Seal(r.Manifest, r.ManifestBytes, v.Tokens, filepath.Base(strings.TrimSuffix(name, ".zip")))
	if e != nil {
		return "", e
	}
	out := strings.TrimSuffix(name, ".zip")
	return out, extractReader(r, out, selected, seal)
}
