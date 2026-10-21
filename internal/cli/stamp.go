package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vincentsch/rungrad"
	packagefmt "github.com/vincentsch/sealjar/internal/pkg"
	"github.com/vincentsch/sealjar/internal/report"
	"github.com/vincentsch/sealjar/internal/tsa"
	"github.com/vincentsch/sealjar/internal/verify"
	"github.com/vincentsch/sealjar/internal/web"
)

func renderAttempts(w io.Writer, attempts []tsa.Attempt) {
	hasToken := false
	for _, a := range attempts {
		if a.Error != "" || a.Token == nil {
			cause := a.Error
			if a.Token != nil {
				cause = tsa.Explanation(*a.Token, "en")
			}
			fmt.Fprintf(w, "  Timestamp authority %s: No timestamp could be obtained. %s. See --json for diagnostic detail.\n", web.MaskURL(a.URL), strings.TrimSuffix(cause, "."))
			continue
		}
		hasToken = true
		t := *a.Token
		fmt.Fprintf(w, "        Timestamp: %s, %s. %s\n", t.DisplayAuthority(), t.DisplayTime(), tsa.Explanation(t, "en"))
		if t.Demo {
			fmt.Fprintln(w, "        "+tsa.DemoProvenance(t, "en"))
		}
	}
	if hasToken {
		fmt.Fprintln(w, "        "+tsa.TimeLimitation("en"))
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
	c.Examples = []string{"sealjar stamp capture.sealjar.zip", "sealjar stamp capture.sealjar.zip --dry-run"}
	c.Related = []string{"sealjar verify", "sealjar capture"}
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
			if f.Flags.JSON {
				for _, t := range v.Tokens {
					fmt.Fprintf(&b, "  %s: %s\n", t.Path, t.Reason)
				}
			}
			return failure(10, fmt.Errorf("stamp stopped: %s", b.String()))
		}
		result := struct {
			ReaderFolder string        `json:"reader_folder,omitempty"`
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
		for i := range result.Attempts {
			result.Attempts[i].URL = web.MaskURL(result.Attempts[i].URL)
		}
		usable := false
		for _, a := range result.Attempts {
			usable = usable || len(a.DER) > 0
		}
		if !usable {
			var b strings.Builder
			if f.Flags.JSON {
				for _, a := range result.Attempts {
					fmt.Fprintf(&b, "  Timestamp authority %s: %s\n", web.MaskURL(a.URL), a.Error)
				}
			} else {
				renderAttempts(&b, result.Attempts)
			}
			return failure(2, fmt.Errorf("no usable timestamp; no package was written. %s", b.String()))
		}
		out := filepath.Dir(args[0])
		work, e := os.MkdirTemp(out, ".sealjar-stamp-")
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
		if e = report.SealDirectory(work, filepath.Base(args[0]), roots); e != nil {
			return failure(2, e)
		}
		stem := strings.TrimSuffix(filepath.Base(args[0]), ".sealjar.zip") + "-stamped"
		output, e := packagefmt.WriteStem(work, out, stem)
		if e != nil {
			return failure(2, e)
		}
		completed, e := packagefmt.Open(output)
		if e != nil {
			os.Remove(output)
			return failure(2, e)
		}
		result.Verification, _, e = verify.Check(completed, roots)
		closeErr := completed.Close()
		if e == nil {
			e = closeErr
		}
		if e != nil {
			os.Remove(output)
			return failure(2, e)
		}
		reader, e := writePackageReader(output, "both", true, roots)
		if e != nil {
			os.Remove(output)
			return failure(2, e)
		}
		result.ReaderFolder = resultPath(reader)
		result.Package = resultPath(output)
		return f.WriteResult(result, func(w io.Writer) { fmt.Fprintf(w, "Stamped  %s\n", result.Package); renderAttempts(w, result.Attempts) })
	}
	return c
}
