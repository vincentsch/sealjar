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

func renderAttempts(w io.Writer, attempts []tsa.Attempt) {
	for _, a := range attempts {
		if a.Error != "" {
			fmt.Fprintf(w, "  Timestamp authority %s: %s\n", a.URL, a.Error)
		} else if a.Token != nil {
			t := a.Token
			fmt.Fprintf(w, "  Timestamp authority %s: %s; time %s; serial %s; accuracy %s; trusted %t. %s\n", a.URL, t.Authority, t.Time, t.Serial, t.Accuracy, t.Trusted, t.Reason)
		}
	}
}
func storeTokens(dir string, attempts []tsa.Attempt) (int, error) {
	count := 0
	for _, a := range attempts {
		if len(a.DER) == 0 {
			continue
		}
		for n := 1; ; n++ {
			suffix := ""
			if n > 1 {
				suffix = fmt.Sprintf("-%d", n)
			}
			name := "manifest.json." + tsa.ID(a.URL) + suffix + ".tsr"
			f, e := os.OpenFile(filepath.Join(dir, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
			if os.IsExist(e) {
				continue
			}
			if e != nil {
				return count, e
			}
			_, e = f.Write(a.DER)
			ce := f.Close()
			if e != nil {
				return count, e
			}
			if ce != nil {
				return count, ce
			}
			count++
			break
		}
	}
	return count, nil
}
func stampCommand() *rungrad.Command {
	var authorities []string
	var root string
	c := command("stamp <package>", "Add timestamps without changing captured evidence")
	c.Mutates = true
	c.Args = cobra.ExactArgs(1)
	c.Examples = []string{"signetleaf stamp capture.signetleaf.zip", "signetleaf stamp capture.signetleaf.zip --dry-run"}
	c.Related = []string{"signetleaf verify", "signetleaf capture"}
	c.Configure = func(cmd *cobra.Command) {
		cmd.Flags().StringArrayVar(&authorities, "tsa", nil, "Request this timestamp authority instead of defaults (repeatable)")
		cmd.Flags().StringVar(&root, "tsa-root", "", "Add trusted timestamp roots from a PEM file")
	}
	c.Run = func(f *rungrad.Factory, cmd *cobra.Command, args []string) error {
		if _, e := tsa.Authorities(authorities); e != nil {
			return failure(1, e)
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
			return failure(10, fmt.Errorf("stamp stopped: %s", b.String()))
		}
		result := struct {
			Package      string        `json:"package,omitempty"`
			DryRun       bool          `json:"dry_run"`
			Attempts     []tsa.Attempt `json:"timestamp_attempts"`
			Verification verify.Result `json:"verification"`
		}{DryRun: f.DryRun(), Verification: v}
		if f.DryRun() {
			return f.WriteResult(result, func(w io.Writer) {
				fmt.Fprintln(w, "DRY RUN: would request timestamps and write a new package beside the input. No files were written.")
				verify.Render(w, v)
			})
		}
		result.Attempts = tsa.Acquire(cmd.Context(), r.ManifestBytes, authorities, roots)
		usable := false
		for _, a := range result.Attempts {
			usable = usable || len(a.DER) > 0
		}
		if !usable {
			var b strings.Builder
			renderAttempts(&b, result.Attempts)
			return failure(2, fmt.Errorf("no usable timestamp; no package was written. %s", b.String()))
		}
		out := filepath.Dir(args[0])
		work, e := os.MkdirTemp(out, ".signetleaf-stamp-")
		if e != nil {
			return failure(2, e)
		}
		defer os.RemoveAll(work)
		for _, name := range r.Names {
			dest := filepath.Join(work, filepath.FromSlash(name))
			if e = os.MkdirAll(filepath.Dir(dest), 0700); e != nil {
				return failure(2, e)
			}
			in, e := r.Entries[name].Open()
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
		if _, e = storeTokens(work, result.Attempts); e != nil {
			return failure(2, e)
		}
		stem := strings.TrimSuffix(filepath.Base(args[0]), ".signetleaf.zip") + "-stamped"
		output, e := packagefmt.WriteStem(work, out, stem)
		if e != nil {
			return failure(2, e)
		}
		result.Package = resultPath(output)
		return f.WriteResult(result, func(w io.Writer) { fmt.Fprintf(w, "Stamped  %s\n", result.Package); renderAttempts(w, result.Attempts) })
	}
	return c
}
