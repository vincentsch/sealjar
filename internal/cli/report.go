package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vincentsch/rungrad"
	packagefmt "github.com/vincentsch/signetleaf/internal/pkg"
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
	c.Examples = []string{"signetleaf report capture.signetleaf.zip --out reader", "signetleaf report capture.signetleaf.zip --lang de --pdf --dry-run"}
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
					return rungrad.NewError(5, "The package has no stored "+name+". Report generation is not yet available during capture.")
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
		if _, e = os.Lstat(out); e == nil {
			return rungrad.NewError(1, "Reader folder already exists. Choose another --out folder.")
		} else if !os.IsNotExist(e) {
			return failure(2, e)
		}
		result := struct {
			Directory    string        `json:"directory"`
			DryRun       bool          `json:"dry_run"`
			Verification verify.Result `json:"verification"`
		}{resultPath(out), f.DryRun(), v}
		if !f.DryRun() {
			stage, e := os.MkdirTemp(filepath.Dir(out), ".signetleaf-reader-")
			if e != nil {
				return failure(2, e)
			}
			defer os.RemoveAll(stage)
			for _, n := range selected {
				dest := filepath.Join(stage, filepath.FromSlash(n))
				if e = os.MkdirAll(filepath.Dir(dest), 0700); e != nil {
					return failure(2, e)
				}
				in, e := r.Entries[n].Open()
				if e != nil {
					return inputError(e)
				}
				dst, e := os.OpenFile(dest, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
				if e != nil {
					in.Close()
					return failure(2, e)
				}
				_, e = io.Copy(dst, in)
				ce := in.Close()
				de := dst.Close()
				if e != nil {
					return failure(2, e)
				}
				if ce != nil {
					return inputError(ce)
				}
				if de != nil {
					return failure(2, de)
				}
			}
			// Mkdir reserves the destination without replacing another writer's folder.
			if e = os.Mkdir(out, 0700); e != nil {
				return failure(2, e)
			}
			committed := false
			defer func() {
				if !committed {
					os.RemoveAll(out)
				}
			}()
			children, e := os.ReadDir(stage)
			if e != nil {
				return failure(2, e)
			}
			for _, child := range children {
				if e = os.Rename(filepath.Join(stage, child.Name()), filepath.Join(out, child.Name())); e != nil {
					return failure(2, e)
				}
			}
			committed = true
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
