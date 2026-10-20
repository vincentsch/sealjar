package cli

import (
	"fmt"
	"io"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/vincentsch/rungrad"
	"github.com/vincentsch/signetleaf/internal/demo"
	packagefmt "github.com/vincentsch/signetleaf/internal/pkg"
	"github.com/vincentsch/signetleaf/internal/verify"
)

type demoResult struct {
	DryRun            bool           `json:"dry_run"`
	Output            string         `json:"output"`
	Capture           *captureResult `json:"capture,omitempty"`
	Verification      *verify.Result `json:"verification,omitempty"`
	VerifyExitCode    int            `json:"verify_exit_code"`
	Package           string         `json:"package,omitempty"`
	ReaderFolder      string         `json:"reader_folder,omitempty"`
	TamperedPackage   string         `json:"tampered_package,omitempty"`
	ChangedScreenshot string         `json:"changed_screenshot,omitempty"`
	VerifyCommand     string         `json:"verify_command,omitempty"`
}

func demoCommand() *rungrad.Command {
	out := "signetleaf-demo"
	c := command("demo", "Capture the bundled bakery and try detecting a changed screenshot")
	c.Mutates = true
	c.Args = cobra.NoArgs
	c.Examples = []string{"signetleaf demo", "signetleaf demo --out ./bakery-demo --json", "signetleaf demo --dry-run"}
	c.Related = []string{"signetleaf capture", "signetleaf verify", "signetleaf report"}
	c.Configure = func(cmd *cobra.Command) {
		cmd.Flags().StringVar(&out, "out", out, "Write demo packages and reader folders here")
	}
	c.Run = func(f *rungrad.Factory, cmd *cobra.Command, args []string) error {
		result := demoResult{DryRun: f.DryRun(), Output: resultPath(out)}
		if f.DryRun() {
			return f.WriteResult(result, func(w io.Writer) {
				fmt.Fprintf(w, "DRY RUN: would capture the bundled bakery's / and /prices/ pages and repository into %s, verify, and write a changed screenshot example. Chrome and Git are required. Default timestamp authorities are tried first, then an untrusted demo timestamp. No files were written or services started.\n", displayPath(out))
			})
		}
		if _, err := findChrome(""); err != nil {
			return failure(2, err)
		}
		if _, err := exec.LookPath("git"); err != nil {
			return rungrad.NewError(2, "Git was not found. Install Git to run the demo.")
		}
		in, err := demo.Start(cmd.Context())
		if err != nil {
			return failure(2, err)
		}
		defer in.Close()
		verdict := 0
		opt := captureOptions{out: out, name: "bakery-demo", lang: "both", labels: []string{"client=Bergblick Bakery (demo)"}, confirmed: true, fallback: in.TimestampURL}
		opt.completed = func(captured captureResult) error {
			result.Capture = &captured
			result.Package = captured.Package
			result.ReaderFolder = captured.ReaderFolder
			r, err := packagefmt.Open(captured.Package)
			if err != nil {
				return inputError(err)
			}
			v, code, err := verify.Check(r)
			r.Close()
			if err != nil {
				return inputError(err)
			}
			if code != 0 && code != 11 {
				return failure(2, fmt.Errorf("demo package failed its integrity check"))
			}
			if len(v.Tokens) == 0 {
				return failure(2, fmt.Errorf("demo could not obtain a timestamp; check loopback networking"))
			}
			result.Verification = &v
			result.VerifyExitCode = code
			tampered, screenshot, err := demo.Tamper(captured.Package, out)
			if err != nil {
				return failure(2, err)
			}
			changed, err := packagefmt.Open(tampered)
			if err != nil {
				return inputError(err)
			}
			changedVerdict, changedCode, err := verify.Check(changed)
			changed.Close()
			if err != nil {
				return inputError(err)
			}
			if changedCode != 10 || len(changedVerdict.Files.Differences) != 1 || changedVerdict.Files.Differences[0].Path != screenshot {
				return failure(2, fmt.Errorf("demo changed-file example did not produce the expected verification result"))
			}
			result.TamperedPackage = resultPath(tampered)
			result.ChangedScreenshot = screenshot
			result.VerifyCommand = "signetleaf verify " + shellQuote(result.TamperedPackage)
			return nil
		}
		capture := captureWithOptions(&verdict, opt)
		if err = capture.Run(f, cmd, []string{in.SiteURL + "/", in.SiteURL + "/prices/", in.Repository}); err != nil {
			return err
		}
		return f.WriteResult(result, func(w io.Writer) {
			fmt.Fprintf(w, "\nPackage: %s\nReader folder: %s\nReport: %s\n", result.Package, result.ReaderFolder, filepath.Join(result.ReaderFolder, "report", "report.en.html"))
			renderAttempts(w, result.Capture.Attempts)
			verify.Render(w, *result.Verification)
			fmt.Fprintf(w, "\nTry the changed screenshot:\n  %s\nExpected: exit 10, changed %s\n", result.VerifyCommand, result.ChangedScreenshot)
		})
	}
	return c
}
