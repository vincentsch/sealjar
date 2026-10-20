// Package cli provides the Signetleaf terminal commands.
package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/spf13/cobra"
	"github.com/vincentsch/rungrad"
	packagefmt "github.com/vincentsch/signetleaf/internal/pkg"
	"github.com/vincentsch/signetleaf/internal/repo"
	"github.com/vincentsch/signetleaf/internal/tsa"
	"github.com/vincentsch/signetleaf/internal/verify"
)

const Version = "0.1.0"

var slugPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)
var slugReplace = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

func slug(s string) string {
	s = strings.Trim(slugReplace.ReplaceAllString(s, "-"), "-_")
	if s == "" {
		return "repository"
	}
	return s
}
func displayPath(s string) string {
	home, _ := os.UserHomeDir()
	if home != "" {
		s = strings.ReplaceAll(s, home, "~")
	}
	return s
}
func resultPath(s string) string {
	home, _ := os.UserHomeDir()
	if filepath.IsAbs(s) && home != "" && strings.HasPrefix(s, home+string(filepath.Separator)) {
		if cwd, err := os.Getwd(); err == nil {
			if relative, err := filepath.Rel(cwd, s); err == nil {
				return relative
			}
		}
	}
	return s
}
func failure(code int, err error) error { return rungrad.NewError(code, displayPath(err.Error())) }
func inputError(err error) error {
	if packagefmt.IsMalformed(err) {
		return failure(10, err)
	}
	if os.IsNotExist(err) {
		return rungrad.NewError(5, "Package was not found. Check the path and try again.")
	}
	return failure(2, err)
}
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	verdict := 0
	app := rungrad.New(rungrad.AppConfig{Name: "signetleaf", Short: "Capture and check repository snapshots", Long: "Keep the current files from a Git repository in a portable package. Check its hashes and restore the snapshot later.", Version: Version})
	app.AddCommand(captureCommand(&verdict))
	app.AddCommand(verifyCommand(&verdict))
	app.AddCommand(restoreCommand())
	app.AddCommand(stampCommand())
	app.AddCommand(doctorCommand())
	app.AddCommand(reportCommand())
	code := app.RunIO(args, stdin, stdout, stderr)
	// Completed verdicts are data, not invocation failures. Returning nil to
	// rungrad avoids a second generic error document after the result.
	if code == 0 {
		return verdict
	}
	return code
}
func command(use, short string) *rungrad.Command {
	return &rungrad.Command{Use: use, Short: short, OutputModes: []string{rungrad.OutputModeHuman, rungrad.OutputModeJSON}}
}

type captureOptions struct {
	out, name                                  string
	labels, exclude                            []string
	untracked, history, confirmed, noTimestamp bool
	strict                                     bool
	authorities                                []string
	root                                       string
}
type captureResult struct {
	Authorities []string      `json:"timestamp_authorities"`
	NextStep    string        `json:"next_step,omitempty"`
	Package     string        `json:"package,omitempty"`
	Size        int64         `json:"size,omitempty"`
	DryRun      bool          `json:"dry_run"`
	Plan        *repo.Plan    `json:"plan"`
	Timestamp   string        `json:"timestamp"`
	Attempts    []tsa.Attempt `json:"timestamp_attempts"`
	Limitations []string      `json:"limitations"`
}

func renderPlan(w io.Writer, p *repo.Plan) {
	f := p.Facts
	fmt.Fprintf(w, "Signetleaf will capture:\n\n  Code      %s (git, branch %s, %s, %d uncommitted changes)\n            %d files, %d bytes\n", f.Name, f.Branch, f.Commit, f.Status.Modified+f.Status.Added+f.Status.Deleted, len(p.Paths), p.TotalSize)
	for _, e := range f.Excluded {
		fmt.Fprintf(w, "  Left out  %s (%s)\n", e.Path, e.Reason)
	}
	for _, d := range f.DeletedPaths {
		fmt.Fprintf(w, "  Deleted   %s (staged: %t, unstaged: %t)\n", d.Path, d.Staged, d.Unstaged)
	}
	for _, s := range f.Submodules {
		fmt.Fprintf(w, "  Submodule %s (listed only; contents are excluded)\n", s.Path)
	}
	fmt.Fprintln(w, "\nExclusions cannot guarantee that no secret is included.")
	if f.History {
		fmt.Fprintln(w, "History can contain old secrets and unrelated work.")
	}
}
func captureCommand(verdict *int) *rungrad.Command {
	opt := captureOptions{}
	c := command("capture <path>...", "Capture the current files from one Git repository")
	c.Mutates = true
	c.Args = cobra.MinimumNArgs(1)
	c.Examples = []string{"signetleaf capture . --confirm --no-timestamp", "signetleaf capture . --dry-run --no-timestamp --include-untracked"}
	c.Related = []string{"signetleaf verify", "signetleaf restore"}
	c.Configure = func(cmd *cobra.Command) {
		f := cmd.Flags()
		f.StringVar(&opt.out, "out", ".", "Write the package in this folder")
		f.StringVar(&opt.name, "name", "", "Use this package name (letters, numbers, hyphens and underscores)")
		f.StringArrayVar(&opt.labels, "label", nil, "Record a label as key=value (repeatable)")
		f.StringArrayVar(&opt.exclude, "exclude", nil, "Exclude a path.Match pattern; ** is not supported (repeatable)")
		f.BoolVar(&opt.untracked, "include-untracked", false, "Include untracked files that Git does not ignore")
		f.BoolVar(&opt.history, "history", false, "Include all Git history as a separate bundle; it can contain old secrets")
		f.BoolVar(&opt.confirmed, "confirm", false, "Capture without asking to continue")
		f.StringArrayVar(&opt.authorities, "tsa", nil, "Request this timestamp authority instead of defaults (repeatable)")
		f.StringVar(&opt.root, "tsa-root", "", "Add trusted timestamp roots from a PEM file")
		f.BoolVar(&opt.strict, "strict", false, "Exit 10 after writing if requested timestamping fails")
		f.BoolVar(&opt.noTimestamp, "no-timestamp", false, "Create a package without a timestamp")
	}
	c.Run = func(f *rungrad.Factory, cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		if _, err := exec.LookPath("git"); err != nil {
			return rungrad.NewError(2, "Git was not found. Install Git to capture a repository. Verify and restore do not need Git.")
		}
		authorities, err := tsa.Authorities(opt.authorities)
		if err != nil {
			return failure(1, err)
		}
		roots, err := tsa.Roots(opt.root)
		if err != nil {
			return failure(2, err)
		}
		if opt.name != "" && !slugPattern.MatchString(opt.name) {
			return rungrad.NewError(1, "Invalid package name. Use letters, numbers, hyphens and underscores.")
		}
		if err := repo.ValidatePatterns(opt.exclude); err != nil {
			return failure(1, err)
		}
		labels := map[string]string{}
		for _, item := range opt.labels {
			k, v, ok := strings.Cut(item, "=")
			if !ok || strings.TrimSpace(k) == "" {
				return rungrad.NewError(1, "A label must have the form key=value.")
			}
			labels[k] = v
		}
		root := ""
		for _, target := range args {
			r, e := repo.Root(ctx, target)
			if e != nil {
				return failure(1, e)
			}
			if root != "" && r != root {
				return rungrad.NewError(1, "Capture accepts one repository root. Run a separate capture for the second repository.")
			}
			root = r
		}
		p, err := repo.Inspect(ctx, root, repo.Options{IncludeUntracked: opt.untracked, History: opt.history, Exclude: opt.exclude})
		if err != nil {
			return failure(2, err)
		}
		result := captureResult{DryRun: f.DryRun(), Plan: p, Timestamp: "Not timestamped", Attempts: []tsa.Attempt{}, Authorities: []string{}, Limitations: []string{"Exclusions cannot guarantee that no secret is included.", "Hashes alone do not establish the authenticity of an untimestamped manifest.", "A timestamp establishes an upper bound for when the files existed, not an independently checked capture time.", "Offline checks cannot see later certificate revocation.", "Signetleaf does not assess eIDAS qualification."}}
		if !opt.noTimestamp {
			result.Authorities = authorities
		}
		if opt.history {
			result.Limitations = append(result.Limitations, "History can contain old secrets and unrelated work.")
		}
		if f.DryRun() {
			return f.WriteResult(result, func(w io.Writer) {
				fmt.Fprintln(w, "DRY RUN: would capture this repository; no files were written.")
				renderPlan(w, p)
				if !opt.noTimestamp {
					fmt.Fprintf(w, "Timestamp authorities: %s\n", strings.Join(authorities, ", "))
				} else {
					fmt.Fprintln(w, "Timestamp: deliberately skipped")
				}
			})
		}
		if !f.Flags.JSON && !f.Flags.Quiet {
			renderPlan(f.Stdout, p)
			if !opt.noTimestamp {
				fmt.Fprintf(f.Stdout, "Timestamp authorities: %s\n", strings.Join(authorities, ", "))
			} else {
				fmt.Fprintln(f.Stdout, "Timestamp: deliberately skipped")
			}
		}
		if !opt.confirmed {
			if !f.CanPrompt() {
				return rungrad.NewError(1, "Capture requires --confirm without an interactive terminal. Use --dry-run to review the files first.")
			}
			yes, e := f.Chooser().Confirm("Continue?", "Yes, continue", "No, cancel")
			if e != nil || !yes {
				return rungrad.NewError(1, "Capture cancelled. No files were written.")
			}
		}
		started := time.Now().UTC()
		if err = os.MkdirAll(opt.out, 0755); err != nil {
			return failure(2, err)
		}
		work, err := os.MkdirTemp(opt.out, ".signetleaf-capture-")
		if err != nil {
			return failure(2, err)
		}
		defer os.RemoveAll(work)
		payload := "repo/" + slug(p.Facts.Name)
		dir := filepath.Join(work, filepath.FromSlash(payload))
		if err = os.MkdirAll(dir, 0700); err != nil {
			return failure(2, err)
		}
		// Bundle output must be absolute because Git runs with a different -C root.
		dir, err = filepath.Abs(dir)
		if err != nil {
			return failure(2, err)
		}
		if err = repo.Snapshot(ctx, p, dir); err != nil {
			return failure(2, err)
		}
		if err = os.WriteFile(filepath.Join(work, "VERIFY.txt"), packagefmt.Instructions, 0600); err != nil {
			return failure(2, err)
		}
		zone := time.Now().Location().String()
		if zone == "Local" {
			if b, e := os.ReadFile("/etc/timezone"); e == nil {
				zone = strings.TrimSpace(string(b))
			}
			if zone == "Local" {
				if name, e := filepath.EvalSymlinks("/etc/localtime"); e == nil {
					_, z, ok := strings.Cut(name, "zoneinfo/")
					if ok {
						zone = z
					}
				}
			}
		}
		if zone == "Local" {
			zone = "UTC"
		}
		m := packagefmt.Manifest{Type: packagefmt.ManifestType, ToolVersion: Version, CaptureID: ulid.Make().String(), Started: started, Ended: time.Now().UTC(), TimeZone: zone, Labels: labels, Settings: packagefmt.Settings{IncludeUntracked: opt.untracked, History: opt.history, Exclude: append([]string{}, opt.exclude...), NoTimestamp: opt.noTimestamp}, Targets: []packagefmt.Target{{Kind: "repo", Name: p.Facts.Name, State: "complete", Commit: p.Facts.Commit, Changes: p.Facts.Status.Modified + p.Facts.Status.Added + p.Facts.Status.Deleted, Payload: payload}}, Environment: packagefmt.Environment{OS: runtime.GOOS, Arch: runtime.GOARCH, GitVersion: repo.GitVersion(ctx)}, TimestampAuthorities: []string{}}
		if !opt.noTimestamp {
			m.TimestampAuthorities = authorities
		}
		if err = packagefmt.BuildManifest(work, &m); err != nil {
			return failure(2, err)
		}
		if !opt.noTimestamp {
			manifest, e := os.ReadFile(filepath.Join(work, "manifest.json"))
			if e != nil {
				return failure(2, e)
			}
			result.Attempts = tsa.Acquire(ctx, manifest, opt.authorities, roots)
			count, e := storeTokens(work, result.Attempts)
			if e != nil {
				return failure(2, e)
			}
			if count > 0 {
				result.Timestamp = "Timestamped"
			} else {
				result.NextStep = "Add a timestamp later with signetleaf stamp <package>."
				if opt.strict {
					*verdict = 10
				}
			}
		}
		name := opt.name
		if name == "" {
			name = slug(p.Facts.Name)
		}
		output, err := packagefmt.Write(work, opt.out, name, started)
		if err != nil {
			return failure(2, err)
		}
		info, err := os.Stat(output)
		if err != nil {
			return failure(2, err)
		}
		result.Package = resultPath(output)
		result.Size = info.Size()
		return f.WriteResult(result, func(w io.Writer) {
			fmt.Fprintf(w, "\nCaptured  %s (%d bytes)\n          Timestamp: %s\n", result.Package, result.Size, result.Timestamp)
			renderAttempts(w, result.Attempts)
			if !opt.noTimestamp && result.Timestamp == "Not timestamped" {
				fmt.Fprintln(w, "No authority returned a usable timestamp. Add one later with signetleaf stamp <package>.")
			}
			for _, limitation := range result.Limitations[1:] {
				fmt.Fprintln(w, limitation)
			}
		})
	}
	return c
}
func verifyCommand(verdict *int) *rungrad.Command {
	var root string
	c := command("verify <package>", "Check capture results, file hashes and timestamp status")
	c.Args = cobra.ExactArgs(1)
	c.Examples = []string{"signetleaf verify capture.signetleaf.zip", "signetleaf verify capture.signetleaf.zip --json"}
	c.Related = []string{"signetleaf capture", "signetleaf restore"}
	c.Configure = func(cmd *cobra.Command) {
		cmd.Flags().StringVar(&root, "tsa-root", "", "Add trusted timestamp roots from a PEM file")
	}
	c.Run = func(f *rungrad.Factory, cmd *cobra.Command, args []string) error {
		r, err := packagefmt.Open(args[0])
		if err != nil {
			if packagefmt.IsMalformed(err) {
				*verdict = 10
				v := verify.Malformed(err)
				return f.WriteResult(v, func(w io.Writer) { verify.Render(w, v) })
			}
			return inputError(err)
		}
		defer r.Close()
		roots, e := tsa.Roots(root)
		if e != nil {
			return failure(2, e)
		}
		v, code, err := verify.Check(r, roots)
		if err != nil {
			return inputError(err)
		}
		*verdict = code
		return f.WriteResult(v, func(w io.Writer) { verify.Render(w, v) })
	}
	return c
}

type restoreResult struct {
	Destination  string        `json:"destination"`
	History      string        `json:"history,omitempty"`
	CloneCommand string        `json:"clone_command,omitempty"`
	DryRun       bool          `json:"dry_run"`
	Timestamp    string        `json:"timestamp"`
	Verification verify.Result `json:"verification"`
}

func restoreCommand() *rungrad.Command {
	var to, root string
	c := command("restore <package>", "Restore checked repository files into an empty folder")
	c.Mutates = true
	c.Args = cobra.ExactArgs(1)
	c.Examples = []string{"signetleaf restore capture.signetleaf.zip --to recovered", "signetleaf restore capture.signetleaf.zip --to recovered --dry-run"}
	c.Related = []string{"signetleaf capture", "signetleaf verify"}
	c.Configure = func(cmd *cobra.Command) {
		cmd.Flags().StringVar(&root, "tsa-root", "", "Add trusted timestamp roots from a PEM file")
		cmd.Flags().StringVar(&to, "to", "", "Restore into this new or empty folder")
	}
	c.Run = func(f *rungrad.Factory, cmd *cobra.Command, args []string) error {
		if to == "" {
			return rungrad.NewError(1, "Restore needs --to <dir>.")
		}
		r, err := packagefmt.Open(args[0])
		if err != nil {
			return inputError(err)
		}
		defer r.Close()
		roots, e := tsa.Roots(root)
		if e != nil {
			return failure(2, e)
		}
		v, code, err := verify.Check(r, roots)
		if err != nil {
			return inputError(err)
		}
		if code == 10 {
			var b strings.Builder
			verify.Render(&b, v)
			return failure(10, fmt.Errorf("restore stopped: %s", b.String()))
		}
		if len(r.Manifest.Targets) != 1 {
			return rungrad.NewError(10, "Restore needs exactly one repository snapshot.")
		}
		prefix := r.Manifest.Targets[0].Payload
		if err = repo.CheckSnapshot(r, prefix, ""); err != nil {
			return inputError(err)
		}
		to = filepath.Clean(to)
		destination, err := filepath.Abs(to)
		if err != nil {
			return failure(1, err)
		}
		exists := false
		if info, e := os.Lstat(destination); e == nil {
			if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
				return rungrad.NewError(1, "Restore destination must be a new or empty directory.")
			}
			entries, e := os.ReadDir(destination)
			if e != nil {
				return failure(2, e)
			}
			if len(entries) > 0 {
				return rungrad.NewError(1, "Restore destination is not empty. Choose a new directory.")
			}
			exists = true
		} else if !os.IsNotExist(e) {
			return failure(2, e)
		}
		historyEntry, history := r.Entries[prefix+"/history.bundle"]
		sidecar := destination + ".history.bundle"
		if history {
			if _, e := os.Lstat(sidecar); e == nil {
				return rungrad.NewError(1, "History output already exists. Choose another --to directory.")
			} else if !os.IsNotExist(e) {
				return failure(2, e)
			}
		}
		result := restoreResult{Destination: resultPath(to), DryRun: f.DryRun(), Timestamp: v.Timestamp.State + ". " + v.Timestamp.Detail, Verification: v}
		if history {
			result.History = resultPath(to + ".history.bundle")
			result.CloneCommand = "git clone " + shellQuote(result.History) + " " + shellQuote(resultPath(to+"-history"))
		}
		if !f.DryRun() {
			stage, e := os.MkdirTemp(filepath.Dir(destination), ".signetleaf-restore-")
			if e != nil {
				return failure(2, e)
			}
			defer os.RemoveAll(stage)
			if e = repo.CheckSnapshot(r, prefix, stage); e != nil {
				return inputError(e)
			}
			sidecarOwned := false
			committed := false
			defer func() {
				if sidecarOwned && !committed {
					os.Remove(sidecar)
				}
			}()
			if history {
				out, e := os.OpenFile(sidecar, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
				if os.IsExist(e) {
					return rungrad.NewError(1, "History output already exists. Choose another --to directory.")
				}
				if e != nil {
					return failure(2, e)
				}
				sidecarOwned = true
				in, e := historyEntry.Open()
				if e != nil {
					out.Close()
					return failure(10, e)
				}
				_, e = io.Copy(out, in)
				ce := in.Close()
				oe := out.Close()
				if e != nil {
					return failure(2, e)
				}
				if ce != nil {
					return failure(10, ce)
				}
				if oe != nil {
					return failure(2, oe)
				}
			}
			if exists {
				if e = os.Remove(destination); e != nil {
					return failure(1, e)
				}
			} else if _, e = os.Lstat(destination); e == nil {
				return rungrad.NewError(1, "Restore destination appeared during extraction. Choose a new directory.")
			}
			if e = os.Rename(stage, destination); e != nil {
				return failure(2, e)
			}
			committed = true
		}
		return f.WriteResult(result, func(w io.Writer) {
			if result.DryRun {
				fmt.Fprintf(w, "DRY RUN: would restore into %s; no files were written.\n", result.Destination)
			} else {
				fmt.Fprintf(w, "Restored  %s\n", result.Destination)
			}
			verify.Render(w, result.Verification)
			if history {
				fmt.Fprintf(w, "History remains separate: %s\n%s\nHistory can contain old secrets and unrelated work.\n", result.History, result.CloneCommand)
			}
		})
	}
	return c
}
func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }
