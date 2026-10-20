// Package cli provides the Signetleaf terminal commands.
package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/spf13/cobra"
	"github.com/vincentsch/rungrad"
	"github.com/vincentsch/signetleaf/internal/browser"
	packagefmt "github.com/vincentsch/signetleaf/internal/pkg"
	"github.com/vincentsch/signetleaf/internal/repo"
	"github.com/vincentsch/signetleaf/internal/report"
	"github.com/vincentsch/signetleaf/internal/tsa"
	"github.com/vincentsch/signetleaf/internal/tsa/demotsa"
	"github.com/vincentsch/signetleaf/internal/verify"
	"github.com/vincentsch/signetleaf/internal/version"
	"github.com/vincentsch/signetleaf/internal/web"
)

var findChrome = browser.Find

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
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer stop()
	restoreSignals := context.AfterFunc(ctx, stop)
	defer restoreSignals()
	verdict := 0
	app := NewApp(&verdict)
	app.Root().SetContext(ctx)
	code := app.RunIO(args, stdin, stdout, stderr)
	// Completed verdicts are data, not invocation failures. Returning nil to
	// rungrad avoids a second generic error document after the result.
	if code == 0 {
		return verdict
	}
	return code
}

// NewApp builds the command tree shared by execution and reference generation.
func NewApp(verdict *int) *rungrad.App {
	app := rungrad.New(rungrad.AppConfig{Name: "signetleaf", Short: "Capture websites and repository snapshots", Long: "Keep web pages and current Git files in a portable package. Check its hashes and timestamps offline.", Version: version.Version + " (" + version.Commit + ")"})
	app.AddCommand(captureCommand(verdict))
	app.AddCommand(verifyCommand(verdict))
	app.AddCommand(restoreCommand())
	app.AddCommand(stampCommand())
	app.AddCommand(doctorCommand())
	app.AddCommand(reportCommand())
	app.AddCommand(demoCommand())
	return app
}

func command(use, short string) *rungrad.Command {
	return &rungrad.Command{Use: use, Short: short, OutputModes: []string{rungrad.OutputModeHuman, rungrad.OutputModeJSON}}
}

type captureOptions struct {
	out, name, lang                            string
	labels, exclude                            []string
	untracked, history, confirmed, noTimestamp bool
	strict                                     bool
	fallback                                   string
	completed                                  func(captureResult) error
	authorities                                []string
	root                                       string
	chrome, auth                               string
	tablet, login                              bool
	wait                                       time.Duration
}
type captureResult struct {
	PDFNote      string              `json:"pdf_note,omitempty"`
	Targets      []packagefmt.Target `json:"targets,omitempty"`
	Pages        []string            `json:"pages,omitempty"`
	Widths       []int               `json:"widths,omitempty"`
	ReaderFolder string              `json:"reader_folder,omitempty"`
	Authorities  []string            `json:"timestamp_authorities"`
	NextStep     string              `json:"next_step,omitempty"`
	Package      string              `json:"package,omitempty"`
	Size         int64               `json:"size,omitempty"`
	DryRun       bool                `json:"dry_run"`
	Plan         *repo.Plan          `json:"plan"`
	Timestamp    string              `json:"timestamp"`
	Attempts     []tsa.Attempt       `json:"timestamp_attempts"`
	Limitations  []string            `json:"limitations"`
}

func renderPlan(w io.Writer, p *repo.Plan) {
	if p == nil {
		return
	}
	f := p.Facts
	changes := f.Status.Modified + f.Status.Added + f.Status.Deleted
	word := "changes"
	if changes == 1 {
		word = "change"
	}
	fmt.Fprintf(w, "Signetleaf will capture:\n\n  Code      %s (git, branch %s, %s, %d uncommitted %s)\n            %d files, %d bytes\n", f.Name, f.Branch, f.Commit, changes, word, len(p.Paths), p.TotalSize)
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
func waitForLoginEnter(ctx context.Context, input io.Reader) error {
	read := make(chan error, 1)
	go func() { _, err := bufio.NewReader(input).ReadString('\n'); read <- err }()
	select {
	case err := <-read:
		if err != nil {
			return fmt.Errorf("Login cancelled before Enter was received.")
		}
		return nil
	case <-ctx.Done():
		return fmt.Errorf("Login cancelled. Chrome was closed.")
	}
}
func captureCommand(verdict *int) *rungrad.Command {
	return captureWithOptions(verdict, captureOptions{})
}
func captureWithOptions(verdict *int, opt captureOptions) *rungrad.Command {
	c := command("capture <target>...", "Capture web pages and current files from one Git repository")
	c.Mutates = true
	c.Args = cobra.MinimumNArgs(1)
	c.Examples = []string{"signetleaf capture . --confirm --no-timestamp --out ../signetleaf-captures", "signetleaf capture . --dry-run --no-timestamp --include-untracked --out ../signetleaf-captures"}
	c.Related = []string{"signetleaf verify", "signetleaf restore"}
	c.Configure = func(cmd *cobra.Command) {
		f := cmd.Flags()
		f.StringVar(&opt.lang, "lang", "both", "Report language: en, de or both")
		f.StringVar(&opt.out, "out", ".", "Write the package in this folder")
		f.StringVar(&opt.name, "name", "", "Use this package name (letters, numbers, hyphens and underscores)")
		f.StringArrayVar(&opt.labels, "label", nil, "Record a label as key=value (repeatable)")
		f.StringArrayVar(&opt.exclude, "exclude", nil, "Exclude a path.Match pattern; ** is not supported (repeatable)")
		f.BoolVar(&opt.untracked, "include-untracked", false, "Include untracked files that Git does not ignore")
		f.BoolVar(&opt.history, "history", false, "Include all Git history as a separate bundle; it can contain old secrets")
		f.BoolVar(&opt.confirmed, "confirm", false, "Capture without asking to continue")
		f.StringArrayVar(&opt.authorities, "tsa", nil, "Request this timestamp authority instead of defaults (repeatable)")
		f.StringVar(&opt.root, "tsa-root", "", "Add trusted timestamp roots from a PEM file")
		f.BoolVar(&opt.strict, "strict", false, "Exit 10 after writing if a target has problems or requested timestamping fails")
		f.StringVar(&opt.chrome, "chrome", "", "Use this Chrome or Chromium executable")
		f.StringVar(&opt.auth, "auth", "", "Send basic auth user:password only to listed target origins (or SIGNETLEAF_AUTH)")
		f.BoolVar(&opt.login, "login", false, "Open Chrome visibly for login, then press Enter in this terminal")
		f.BoolVar(&opt.tablet, "tablet", false, "Also capture a 768×1024 tablet viewport at scale 2")
		f.DurationVar(&opt.wait, "wait", 0, "Wait extra after load (maximum 30s)")
		f.BoolVar(&opt.noTimestamp, "no-timestamp", false, "Create a package without a timestamp")
	}
	c.Run = func(f *rungrad.Factory, cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		if !report.ValidLanguage(opt.lang) {
			return rungrad.NewError(1, "Report language must be en, de or both.")
		}
		if opt.wait < 0 || opt.wait > 30*time.Second {
			return rungrad.NewError(1, "--wait must be between 0s and 30s.")
		}
		if opt.login && !f.CanPrompt() {
			return rungrad.NewError(1, "--login requires an interactive terminal. Run capture from a terminal and press Enter after login.")
		}
		if opt.auth == "" && opt.completed == nil {
			opt.auth = os.Getenv("SIGNETLEAF_AUTH")
		}
		if opt.auth != "" {
			if _, _, ok := strings.Cut(opt.auth, ":"); !ok {
				return rungrad.NewError(1, "Basic auth needs user:password.")
			}
			f.RegisterSecret(opt.auth)
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
		urls := []string{}
		positions := []int{}
		for i, target := range args {
			if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
				u, e := url.Parse(target)
				if e != nil {
					return rungrad.NewError(1, "Invalid target URL. Check its host and port.")
				}
				if u.User != nil {
					return rungrad.NewError(1, "Target URLs cannot contain user info. Use --auth or SIGNETLEAF_AUTH instead.")
				}
				if _, e = web.Origin(target); e != nil {
					return failure(1, e)
				}
				urls = append(urls, target)
				positions = append(positions, i+1)
				continue
			}
			if _, e := exec.LookPath("git"); e != nil {
				return rungrad.NewError(2, "Git was not found. Install Git to capture a repository. Verify and restore do not need Git.")
			}
			r, e := repo.Root(ctx, target)
			if e != nil {
				return failure(1, e)
			}
			if root != "" && r != root {
				return rungrad.NewError(1, "Capture accepts one repository root. Run a separate capture for the second repository.")
			}
			root = r
		}
		if opt.login && len(urls) == 0 {
			return rungrad.NewError(1, "--login needs a web page target.")
		}
		var p *repo.Plan
		if root != "" {
			p, err = repo.Inspect(ctx, root, repo.Options{IncludeUntracked: opt.untracked, History: opt.history, Exclude: opt.exclude})
			if err != nil {
				return failure(2, err)
			}
		}

		chromePath, chromeErr := findChrome(opt.chrome)
		result := captureResult{DryRun: f.DryRun(), Plan: p, Timestamp: "Not timestamped", Attempts: []tsa.Attempt{}, Authorities: []string{}, Limitations: []string{"Exclusions cannot guarantee that no secret is included.", "Hashes alone do not establish the authenticity of an untimestamped manifest.", "A timestamp establishes an upper bound for when the files existed, not an independently checked capture time.", "Offline checks cannot see later certificate revocation.", "Signetleaf does not assess eIDAS qualification."}}
		if chromeErr != nil && len(urls) == 0 {
			result.PDFNote = "Chrome is unavailable. Reports will be HTML only; no report PDFs will be stored."
		}
		if len(urls) > 0 {
			result.Widths = []int{1440, 390}
			if opt.tablet {
				result.Widths = append(result.Widths, 768)
			}
			for _, u := range urls {
				result.Pages = append(result.Pages, web.MaskURL(u))
			}
			result.Limitations = append(result.Limitations, "Captured images, DOM and PDFs may contain private information. Signetleaf masks metadata but does not rewrite captured content.")
		}
		renderCapturePlan := func(w io.Writer) {
			renderPlan(w, p)
			if result.PDFNote != "" {
				fmt.Fprintln(w, result.PDFNote)
			}
			for _, u := range result.Pages {
				fmt.Fprintf(w, "  Page      %s (widths %v)\n", u, result.Widths)
			}
		}
		if !opt.noTimestamp {
			result.Authorities = authorities
		}
		if opt.history {
			result.Limitations = append(result.Limitations, "History can contain old secrets and unrelated work.")
		}
		if f.DryRun() {
			return f.WriteResult(result, func(w io.Writer) {
				fmt.Fprintln(w, "DRY RUN: would capture these targets; no files were written.")
				renderCapturePlan(w)
				if !opt.noTimestamp {
					fmt.Fprintf(w, "Timestamp authorities: %s\n", strings.Join(authorities, ", "))
				} else {
					fmt.Fprintln(w, "Timestamp: deliberately skipped")
				}
			})
		}
		if !f.Flags.JSON && !f.Flags.Quiet {
			renderCapturePlan(f.Stdout)
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
		var captureBrowser *browser.Browser
		if chromeErr == nil {
			captureBrowser, err = browser.Start(ctx, chromePath, opt.login)
			if err != nil {
				return failure(2, err)
			}
			defer captureBrowser.Close()
		}
		targets := []packagefmt.Target{}
		var webResult web.Result
		if p != nil {
			payload := "repo/" + slug(p.Facts.Name)
			dir, e := filepath.Abs(filepath.Join(work, filepath.FromSlash(payload)))
			if e != nil {
				return failure(2, e)
			}
			if e = os.MkdirAll(dir, 0700); e != nil {
				return failure(2, e)
			}
			if e = repo.Snapshot(ctx, p, dir); e != nil {
				return failure(2, e)
			}
			targets = append(targets, packagefmt.Target{Kind: "repo", Name: p.Facts.Name, State: "complete", Commit: p.Facts.Commit, Changes: p.Facts.Status.Modified + p.Facts.Status.Added + p.Facts.Status.Deleted, Payload: payload})
		}
		if len(urls) > 0 {
			webResult, err = web.Capture(ctx, work, urls, positions, web.Options{Browser: captureBrowser, Progress: func(message string) { f.Infof("%s", message) }, Chrome: opt.chrome, Tablet: opt.tablet, Wait: opt.wait, Auth: opt.auth, Login: opt.login, LoginWait: func(ctx context.Context) error {
				fmt.Fprintln(f.Stderr, "Log in in the Chrome window, then press Enter here to capture.")
				return waitForLoginEnter(ctx, f.Stdin)
			}})
			if err != nil {
				return failure(2, err)
			}
			targets = append(targets, webResult.Targets...)
		}
		succeeded := false
		for _, target := range targets {
			if target.State != "failed" {
				succeeded = true
			}
			if opt.strict && target.State != "complete" {
				*verdict = 10
			}
		}
		if !succeeded {
			details := []string{"No target document could be captured."}
			for _, target := range targets {
				note := "No target document was captured"
				if len(target.Notes) > 0 {
					note = target.Notes[0]
				}
				details = append(details, fmt.Sprintf("%s: %s", target.URL, note))
			}
			details = append(details, "Check the target URLs and network, then try again. No package was written.")
			return rungrad.NewError(2, strings.Join(details, "\n"))
		}
		result.Targets = targets

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
		env := webResult.Environment
		env.OS = runtime.GOOS
		env.Arch = runtime.GOARCH
		if env.ChromeVersion == "" && captureBrowser != nil {
			env.ChromeVersion, err = captureBrowser.Version(ctx)
			if err != nil {
				return failure(2, err)
			}
		}
		if p != nil {
			env.GitVersion = repo.GitVersion(ctx)
		}
		m := packagefmt.Manifest{Type: packagefmt.ManifestType, ToolVersion: version.Version, CaptureID: ulid.Make().String(), Started: started, Ended: time.Now().UTC(), TimeZone: zone, Labels: labels, Settings: packagefmt.Settings{IncludeUntracked: opt.untracked, History: opt.history, Exclude: append([]string{}, opt.exclude...), NoTimestamp: opt.noTimestamp, ManualLogin: opt.login, BasicAuth: opt.auth != "" && len(urls) > 0, Wait: opt.wait.String(), Widths: result.Widths}, Targets: targets, Environment: env, Hosts: webResult.Hosts, TimestampAuthorities: []string{}}

		if !opt.noTimestamp {
			m.TimestampAuthorities = authorities
			if opt.fallback != "" {
				m.TimestampAuthorities = append(append([]string{}, authorities...), opt.fallback)
			}
		}
		if err = report.Thumbnails(work); err != nil {
			return failure(2, err)
		}
		if err = os.WriteFile(filepath.Join(work, "VERIFY.txt"), packagefmt.Instructions, 0600); err != nil {
			return failure(2, err)
		}
		// Collect payload hashes before rendering; reports and instructions are excluded from this view.
		if err = packagefmt.CollectFiles(work, &m); err != nil {
			return failure(2, err)
		}
		payload := m.Files[:0]
		for _, rec := range m.Files {
			if rec.Path != "VERIFY.txt" {
				payload = append(payload, rec)
			}
		}
		m.Files = payload
		reportName := opt.name
		if reportName == "" {
			if p != nil {
				reportName = p.Facts.Name
			} else {
				u, _ := url.Parse(urls[0])
				reportName = u.Hostname()
			}
		}
		if err = report.Render(work, reportName, opt.lang, m); err != nil {
			return failure(2, err)
		}
		if captureBrowser != nil {
			if err = report.PDFs(captureBrowser, work, opt.lang); err != nil {
				return failure(2, err)
			}
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
			if opt.fallback != "" {
				usable := false
				for _, a := range result.Attempts {
					usable = usable || len(a.DER) > 0
				}
				if !usable {
					result.Attempts = append(result.Attempts, tsa.Acquire(ctx, manifest, []string{opt.fallback}, roots)...)
				}
			}
			count, e := storeTokens(work, result.Attempts)
			if e != nil {
				return failure(2, e)
			}
			if count > 0 {
				result.Timestamp = "Timestamped"
				for _, a := range result.Attempts {
					if a.Token != nil && a.Token.Demo {
						result.Timestamp = demotsa.Label
					}
				}
				for _, attempt := range result.Attempts {
					if attempt.Token != nil && attempt.Token.Trusted {
						result.Limitations = append(result.Limitations[:1], result.Limitations[2:]...)
						break
					}
				}
			} else {
				result.NextStep = "Add a timestamp later with signetleaf stamp <package>."
				if opt.strict {
					*verdict = 10
				}
			}
		}
		name := opt.name
		if name == "" {
			if p != nil {
				name = slug(p.Facts.Name)
			} else {
				u, _ := url.Parse(urls[0])
				name = slug(u.Hostname())
			}
		}
		if err = report.SealDirectory(work, name, roots); err != nil {
			return failure(2, err)
		}
		output, err := packagefmt.Write(work, opt.out, name, started)
		if err != nil {
			return failure(2, err)
		}
		info, err := os.Stat(output)
		if err != nil {
			return failure(2, err)
		}
		reader, err := writePackageReader(output, "both", true, roots)
		if err != nil {
			os.Remove(output)
			return failure(2, err)
		}
		result.ReaderFolder = resultPath(reader)
		result.Package = resultPath(output)
		result.Size = info.Size()
		if opt.completed != nil {
			return opt.completed(result)
		}
		return f.WriteResult(result, func(w io.Writer) {
			fmt.Fprintf(w, "\nSealed  %s (%s)\n", result.Package, humanSize(result.Size))
			renderAttempts(w, result.Attempts)
			if len(result.Attempts) == 0 {
				fmt.Fprintln(w, "        Timestamp: Not timestamped")
			}
			reportLang := report.Languages(opt.lang)[0]
			fmt.Fprintf(w, "        Report: %s/report/report.%s.html (copy)\n", result.ReaderFolder, reportLang)
			if result.PDFNote != "" {
				fmt.Fprintln(w, result.PDFNote)
			}
			problems := false
			for _, target := range targets {
				if target.State == "complete" {
					continue
				}
				if !problems {
					fmt.Fprintln(w, "\nProblems")
					problems = true
				}
				fmt.Fprintf(w, "  %s: %s\n", target.URL, target.State)
				for _, note := range target.Notes {
					fmt.Fprintln(w, "    "+note)
				}
			}

			if !opt.noTimestamp && result.Timestamp == "Not timestamped" {
				fmt.Fprintln(w, "No authority returned a usable timestamp. Add one later with signetleaf stamp <package>.")
			}

			trusted := false
			for _, attempt := range result.Attempts {
				trusted = trusted || attempt.Token != nil && attempt.Token.Trusted
			}
			if !trusted {
				fmt.Fprintln(w, "Hashes alone do not establish the authenticity of an untimestamped manifest.")
			}
			fmt.Fprintf(w, "Check timestamps with signetleaf verify %s\n", result.Package)
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
		var repositories []packagefmt.Target
		for _, target := range r.Manifest.Targets {
			if target.Kind == "repo" {
				repositories = append(repositories, target)
			}
		}
		if len(repositories) != 1 {
			return rungrad.NewError(10, "Restore needs exactly one repository snapshot.")
		}
		prefix := repositories[0].Payload
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

func humanSize(n int64) string {
	if n < 1000 {
		return fmt.Sprintf("%d bytes", n)
	}
	if n < 1000000 {
		return fmt.Sprintf("%.1f kB", float64(n)/1000)
	}
	return fmt.Sprintf("%.1f MB", float64(n)/1000000)
}
