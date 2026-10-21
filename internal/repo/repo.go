// Package repo captures Git working trees without following symbolic links.
package repo

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"

	packagefmt "github.com/vincentsch/sealjar/internal/pkg"
)

type Deleted struct {
	Path     string `json:"path"`
	Staged   bool   `json:"staged"`
	Unstaged bool   `json:"unstaged"`
}
type Excluded struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}
type Submodule struct {
	Path   string `json:"path"`
	Commit string `json:"commit"`
}
type Remote struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}
type Status struct {
	Modified  int `json:"modified"`
	Added     int `json:"added"`
	Deleted   int `json:"deleted"`
	Untracked int `json:"untracked"`
}
type Facts struct {
	Name         string      `json:"name"`
	Commit       string      `json:"commit"`
	Subject      string      `json:"subject"`
	Branch       string      `json:"branch"`
	Describe     string      `json:"describe"`
	Remotes      []Remote    `json:"remotes"`
	Status       Status      `json:"status"`
	DeletedPaths []Deleted   `json:"deleted_paths"`
	Submodules   []Submodule `json:"submodules"`
	Excluded     []Excluded  `json:"excluded"`
	History      bool        `json:"history"`
}
type File struct {
	Path        string `json:"path"`
	Size        int64  `json:"size"`
	SHA256      string `json:"sha256"`
	Mode        string `json:"mode"`
	LinkTarget  string `json:"link_target,omitempty"`
	PointerOnly bool   `json:"pointer_only,omitempty"`
}
type Plan struct {
	Root      string   `json:"-"`
	Facts     Facts    `json:"git"`
	Paths     []string `json:"paths"`
	TotalSize int64    `json:"total_size"`
}
type Options struct {
	IncludeUntracked bool
	History          bool
	Exclude          []string
}

func git(ctx context.Context, root string, args ...string) ([]byte, error) {
	a := append([]string{"--no-optional-locks", "-C", root}, args...)
	cmd := exec.CommandContext(ctx, "git", a...)
	cmd.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0", "GIT_TERMINAL_PROMPT=0")
	b, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s failed; check that Git is installed and the repository is readable", args[0])
	}
	return b, nil
}
func Root(ctx context.Context, target string) (string, error) {
	info, err := os.Stat(target)
	if err != nil {
		return "", fmt.Errorf("repository target is not readable")
	}
	if !info.IsDir() {
		target = filepath.Dir(target)
	}
	b, err := git(ctx, target, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("target must be inside a Git working tree")
	}
	root, err := filepath.EvalSymlinks(strings.TrimSpace(string(b)))
	return root, err
}
func ValidatePatterns(patterns []string) error {
	for _, p := range patterns {
		if p == "" || strings.Contains(p, "**") {
			return fmt.Errorf("invalid exclusion %q: use path.Match patterns; ** is not supported", p)
		}
		if _, err := path.Match(strings.TrimSuffix(p, "/"), ""); err != nil {
			return fmt.Errorf("invalid exclusion %q: %w", p, err)
		}
	}
	return nil
}
func matches(pattern, name string) bool {
	if strings.HasSuffix(pattern, "/") {
		p := strings.TrimSuffix(pattern, "/")
		parts := strings.Split(name, "/")
		for i := 1; i < len(parts); i++ {
			if matches(p, strings.Join(parts[:i], "/")) {
				return true
			}
		}
		return false
	}
	candidate := name
	if !strings.Contains(pattern, "/") {
		candidate = path.Base(name)
	}
	ok, _ := path.Match(pattern, candidate)
	return ok
}

var defaults = []string{".env", ".env.*", "*.pem", "*.key", "*.p12", "*.pfx", "id_rsa*", "id_ed25519*", "credentials*.json", "*.kdbx", ".npmrc", ".pypirc", "auth.json"}

func reason(name string, patterns []string) string {
	for _, p := range patterns {
		if matches(p, name) {
			return "excluded by pattern " + p
		}
	}
	base := path.Base(name)
	if base == ".env.example" || base == ".env.dist" {
		return ""
	}
	for _, p := range defaults {
		if matches(p, name) {
			return "may contain secrets"
		}
	}
	return ""
}
func cleanRemote(raw string) string {
	if strings.Contains(raw, "://") {
		u, err := url.Parse(raw)
		if err != nil {
			return "<unreadable remote>"
		}
		if u.Scheme == "file" {
			return "<local repository>"
		}
		u.User = nil
		u.RawQuery = ""
		u.Fragment = ""
		return u.String()
	}
	if at := strings.Index(raw, "@"); at >= 0 {
		return raw[at+1:]
	}
	// Local remotes can reveal the operator's home directory.
	if filepath.IsAbs(raw) {
		return "<local repository>"
	}
	return raw
}
func Inspect(ctx context.Context, root string, opt Options) (*Plan, error) {
	if err := ValidatePatterns(opt.Exclude); err != nil {
		return nil, err
	}
	p := &Plan{Root: root, Paths: []string{}, Facts: Facts{Name: filepath.Base(root), Remotes: []Remote{}, DeletedPaths: []Deleted{}, Submodules: []Submodule{}, Excluded: []Excluded{}, History: opt.History}}
	read := func(args ...string) (string, error) {
		b, e := git(ctx, root, args...)
		return strings.TrimSpace(string(b)), e
	}
	var err error
	p.Facts.Commit, err = read("rev-parse", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("repository has no readable HEAD commit; commit the initial files before capture")
	}
	if p.Facts.Subject, err = read("log", "-1", "--format=%s"); err != nil {
		return nil, err
	}
	if p.Facts.Branch, err = read("rev-parse", "--abbrev-ref", "HEAD"); err != nil {
		return nil, err
	}
	// --dirty refreshes the index even when optional locks are disabled.
	if p.Facts.Describe, err = read("describe", "--always"); err != nil {
		return nil, err
	}
	remotes, err := git(ctx, root, "remote")
	if err != nil {
		return nil, err
	}
	for _, name := range strings.Fields(string(remotes)) {
		v, e := read("config", "--get-all", "remote."+name+".url")
		if e != nil {
			return nil, e
		}
		for _, u := range strings.Split(v, "\n") {
			p.Facts.Remotes = append(p.Facts.Remotes, Remote{name, cleanRemote(u)})
		}
	}
	status, err := git(ctx, root, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		return nil, err
	}
	entries := strings.Split(string(status), "\x00")
	deleted := map[string]bool{}
	dirty := false
	for i := 0; i < len(entries); i++ {
		e := entries[i]
		if len(e) < 4 {
			continue
		}
		x, y := e[0], e[1]
		name := e[3:]
		if x == 'R' || x == 'C' || y == 'R' || y == 'C' {
			i++
		}
		if x == '?' {
			p.Facts.Status.Untracked++
			continue
		}
		dirty = true
		if x == 'D' || y == 'D' {
			p.Facts.DeletedPaths = append(p.Facts.DeletedPaths, Deleted{name, x == 'D', y == 'D'})
			p.Facts.Status.Deleted++
			deleted[name] = true
		}
		if x == 'A' || y == 'A' {
			p.Facts.Status.Added++
		}
		if strings.ContainsAny(string([]byte{x, y}), "MRTCU") {
			p.Facts.Status.Modified++
		}
	}
	if dirty {
		p.Facts.Describe += "-dirty"
	}
	sort.Slice(p.Facts.DeletedPaths, func(i, j int) bool { return p.Facts.DeletedPaths[i].Path < p.Facts.DeletedPaths[j].Path })
	tracked, err := git(ctx, root, "ls-files", "--stage", "-z")
	if err != nil {
		return nil, err
	}
	candidates := map[string]bool{}
	for _, entry := range strings.Split(string(tracked), "\x00") {
		if entry == "" {
			continue
		}
		parts := strings.SplitN(entry, "\t", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid Git index entry")
		}
		fields := strings.Fields(parts[0])
		if len(fields) != 3 {
			return nil, fmt.Errorf("invalid Git index entry")
		}
		name := parts[1]
		if fields[0] == "160000" {
			p.Facts.Submodules = append(p.Facts.Submodules, Submodule{name, fields[1]})
			continue
		}
		candidates[name] = true
	}
	if opt.IncludeUntracked {
		b, e := git(ctx, root, "ls-files", "-z", "--others", "--exclude-standard")
		if e != nil {
			return nil, e
		}
		for _, name := range strings.Split(string(b), "\x00") {
			if name != "" {
				candidates[name] = true
			}
		}
	}
	names := make([]string, 0, len(candidates))
	for name := range candidates {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if deleted[name] {
			continue
		}
		// Git represents untracked nested repositories and worktrees as folders.
		nested := strings.HasSuffix(name, "/")
		name = strings.TrimSuffix(name, "/")
		if err := packagefmt.SafePath(name); err != nil {
			return nil, err
		}
		if nested {
			p.Facts.Excluded = append(p.Facts.Excluded, Excluded{name, "nested Git repository; contents not included"})
			continue
		}
		why := reason(name, opt.Exclude)
		if why != "" {
			p.Facts.Excluded = append(p.Facts.Excluded, Excluded{name, why})
			continue
		}
		full := filepath.Join(root, filepath.FromSlash(name))
		if !plainParents(root, name) {
			p.Facts.Excluded = append(p.Facts.Excluded, Excluded{name, "parent is a symbolic link or is not a directory"})
			continue
		}
		info, e := os.Lstat(full)
		if e != nil {
			return nil, fmt.Errorf("cannot read candidate %q", name)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			target, e := os.Readlink(full)
			if e != nil {
				return nil, e
			}
			if !safeWorkingLink(root, name, target) {
				p.Facts.Excluded = append(p.Facts.Excluded, Excluded{name, "link points outside the repository"})
				continue
			}
			p.TotalSize += int64(len(target))
		} else if info.Mode().IsRegular() {
			p.TotalSize += info.Size()
		} else {
			return nil, fmt.Errorf("candidate %q is not a regular file or link", name)
		}
		p.Paths = append(p.Paths, name)
	}
	return p, nil
}
func safeWorkingLink(root, name, target string) bool {
	if packagefmt.SafeLink(name, target) != nil {
		return false
	}
	full := root + string(filepath.Separator) + filepath.FromSlash(path.Dir(name)) + string(filepath.Separator) + target
	for {
		resolved, err := filepath.EvalSymlinks(full)
		if err == nil {
			rel, e := filepath.Rel(root, resolved)
			return e == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
		}
		if !os.IsNotExist(err) {
			return false
		}
		parent := filepath.Dir(full)
		if parent == full {
			return false
		}
		full = parent
	}
}
func Snapshot(ctx context.Context, p *Plan, dir string) (err error) {
	out, err := os.Create(filepath.Join(dir, "snapshot.tar.gz"))
	if err != nil {
		return err
	}
	defer func() {
		if e := out.Close(); err == nil {
			err = e
		}
	}()
	gz := gzip.NewWriter(out)
	tw := tar.NewWriter(gz)
	records := []File{}
	for _, name := range p.Paths {
		full := filepath.Join(p.Root, filepath.FromSlash(name))
		if !plainParents(p.Root, name) {
			return fmt.Errorf("parent of %q changed during capture; retry", name)
		}
		info, e := os.Lstat(full)
		if e != nil {
			return e
		}
		rec := File{Path: name, Mode: fmt.Sprintf("%04o", info.Mode().Perm())}
		link := ""
		if info.Mode()&os.ModeSymlink != 0 {
			link, e = os.Readlink(full)
			if e != nil {
				return e
			}
			if !safeWorkingLink(p.Root, name, link) {
				return fmt.Errorf("link %q changed to an unsafe target", name)
			}
			rec.LinkTarget = link
			rec.Size = int64(len(link))
			rec.SHA256 = packagefmt.Hash([]byte(link))
		} else if !info.Mode().IsRegular() {
			return fmt.Errorf("candidate %q changed type", name)
		}
		hdr, e := tar.FileInfoHeader(info, link)
		if e != nil {
			return e
		}
		hdr.Name = name
		hdr.Format = tar.FormatPAX
		if e = tw.WriteHeader(hdr); e != nil {
			return e
		}
		if link == "" {
			f, e := os.Open(full)
			if e != nil {
				return e
			}
			first := make([]byte, 1024)
			n, readErr := f.Read(first)
			if readErr != nil && readErr != io.EOF {
				f.Close()
				return readErr
			}
			rec.PointerOnly = bytes.HasPrefix(first[:n], []byte("version https://git-lfs.github.com/spec/v1\n")) && bytes.Contains(first[:n], []byte("oid sha256:")) && bytes.Contains(first[:n], []byte("\nsize "))
			hash := sha256.New()
			rec.Size, e = io.Copy(io.MultiWriter(tw, hash), io.MultiReader(bytes.NewReader(first[:n]), f))
			closeErr := f.Close()
			if e != nil {
				return fmt.Errorf("capture %q: %w", name, e)
			}
			if closeErr != nil {
				return closeErr
			}
			if rec.Size != info.Size() {
				return fmt.Errorf("file %q changed size during capture; retry", name)
			}
			rec.SHA256 = hex.EncodeToString(hash.Sum(nil))
		}
		records = append(records, rec)
	}
	if err = tw.Close(); err != nil {
		return err
	}
	if err = gz.Close(); err != nil {
		return err
	}
	if err = writeJSON(filepath.Join(dir, "files.json"), records); err != nil {
		return err
	}
	if err = writeJSON(filepath.Join(dir, "git.json"), p.Facts); err != nil {
		return err
	}
	if p.Facts.History {
		_, err = git(ctx, p.Root, "bundle", "create", filepath.Join(dir, "history.bundle"), "--all")
	}
	return err
}
func writeJSON(name string, v any) error {
	b, e := packagefmt.JSON(v)
	if e != nil {
		return e
	}
	return os.WriteFile(name, b, 0600)
}
func GitVersion(ctx context.Context) string {
	b, e := exec.CommandContext(ctx, "git", "--version").Output()
	if e != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(b))
}

func plainParents(root, name string) bool {
	for parent := path.Dir(name); parent != "."; parent = path.Dir(parent) {
		info, err := os.Lstat(filepath.Join(root, filepath.FromSlash(parent)))
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return false
		}
	}
	return true
}
