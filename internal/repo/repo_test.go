package repo

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestExclusionPatterns(t *testing.T) {
	cases := []struct {
		path     string
		patterns []string
		reason   string
	}{
		{".env", nil, "may contain secrets"}, {"nested/.env.production", nil, "may contain secrets"}, {"nested/.env.example", nil, ""}, {".env.dist", nil, ""},
		{"nested/.env.example", []string{".env.*"}, "excluded by pattern .env.*"},
		{"config/server.key", []string{"config/server.key"}, "excluded by pattern config/server.key"},
		{"nested/config/server.key", []string{"config/server.key"}, "may contain secrets"},
		{"config/server.key", []string{"*.key"}, "excluded by pattern *.key"},
		{"build/index.js", []string{"build/"}, "excluded by pattern build/"}, {"a/build/deep/index.js", []string{"build/"}, "excluded by pattern build/"},
		{"a/build/index.js", []string{"a/build/"}, "excluded by pattern a/build/"}, {"other/a/build/index.js", []string{"a/build/"}, ""},
		{"src/build", []string{"build/"}, ""}, {"src/source.go", []string{"*.go"}, "excluded by pattern *.go"},
	}
	for _, tc := range cases {
		t.Run(tc.path+strings.Join(tc.patterns, ","), func(t *testing.T) {
			if e := ValidatePatterns(tc.patterns); e != nil {
				t.Fatal(e)
			}
			if got := reason(tc.path, tc.patterns); got != tc.reason {
				t.Fatalf("reason %q, want %q", got, tc.reason)
			}
		})
	}
	for _, name := range []string{"a.pem", "a.key", "a.p12", "a.pfx", "id_rsa", "id_rsa.pub", "id_ed25519", "credentials-prod.json", "vault.kdbx", ".npmrc", ".pypirc", "auth.json"} {
		if got := reason("nested/"+name, nil); got != "may contain secrets" {
			t.Errorf("%s not excluded", name)
		}
	}
	for _, pattern := range []string{"**", "a/**/b", "[broken", ""} {
		if e := ValidatePatterns([]string{pattern}); e == nil {
			t.Errorf("accepted invalid pattern %q", pattern)
		}
	}
}
func runGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	a := append([]string{"-c", "user.name=Snapshot Fixture", "-c", "user.email=snapshot@example.invalid", "-c", "commit.gpgsign=false", "-C", root}, args...)
	cmd := exec.Command("git", a...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null")
	b, e := cmd.CombinedOutput()
	if e != nil {
		t.Fatalf("git %v: %v %s", args, e, b)
	}
	return strings.TrimSpace(string(b))
}
func makeRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	runGit(t, root, "init", "-b", "main")
	if e := os.WriteFile(filepath.Join(root, "README.md"), []byte("Current source\n"), 0644); e != nil {
		t.Fatal(e)
	}
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-m", "Add source")
	return root
}
func TestSubmodulesAreListedWithoutContents(t *testing.T) {
	sub := makeRepo(t)
	root := makeRepo(t)
	runGit(t, root, "-c", "protocol.file.allow=always", "submodule", "add", sub, "vendor/library")
	runGit(t, root, "commit", "-am", "Add library")
	p, e := Inspect(context.Background(), root, Options{IncludeUntracked: true})
	if e != nil {
		t.Fatal(e)
	}
	if len(p.Facts.Submodules) != 1 || p.Facts.Submodules[0].Path != "vendor/library" {
		t.Fatalf("submodules: %#v", p.Facts.Submodules)
	}
	for _, name := range p.Paths {
		if strings.HasPrefix(name, "vendor/library") {
			t.Fatalf("included submodule content %q", name)
		}
	}
	// A normally ignored fake secret is absent from the candidate/exclusion list.
	if e = os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".env\n"), 0644); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(root, ".env"), []byte("FAKE=test-placeholder\n"), 0644); e != nil {
		t.Fatal(e)
	}
	p, e = Inspect(context.Background(), root, Options{IncludeUntracked: true})
	if e != nil {
		t.Fatal(e)
	}
	for _, excluded := range p.Facts.Excluded {
		if excluded.Path == ".env" {
			t.Fatal("ignored non-candidate appeared in exclusions")
		}
	}
}
func TestSymlinkParentsAreNotFollowed(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires symbolic-link privileges")
	}
	root := makeRepo(t)
	outside := t.TempDir()
	if e := os.Mkdir(filepath.Join(root, "source"), 0755); e != nil {
		t.Fatal(e)
	}
	for _, dir := range []string{outside, filepath.Join(root, "source")} {
		if e := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("bytes"), 0644); e != nil {
			t.Fatal(e)
		}
	}
	runGit(t, root, "add", "source/file.txt")
	runGit(t, root, "commit", "-m", "Add source directory")
	if e := os.RemoveAll(filepath.Join(root, "source")); e != nil {
		t.Fatal(e)
	}
	if e := os.Symlink(outside, filepath.Join(root, "source")); e != nil {
		t.Fatal(e)
	}
	p, e := Inspect(context.Background(), root, Options{IncludeUntracked: true})
	if e != nil {
		t.Fatal(e)
	}
	for _, name := range p.Paths {
		if name == "source/file.txt" {
			t.Fatal("followed symlink ancestor")
		}
	}
	if len(p.Facts.Excluded) != 1 || p.Facts.Excluded[0].Path != "source" {
		t.Fatalf("missing excluded candidate: %#v", p.Facts.Excluded)
	}
}
func TestLinkChainsStayInsideSnapshot(t *testing.T) {
	cases := []struct {
		name, target string
		links        map[string]string
		safe         bool
	}{
		{"link", "src/file", map[string]string{}, true},
		{"src/link", "../README.md", map[string]string{}, true},
		{"link", "missing/file", map[string]string{}, true},
		{"link", "inside/../README.md", map[string]string{"inside": "."}, false},
		{"a", "b", map[string]string{"a": "b", "b": "a"}, false},
		{"link", "chain/file", map[string]string{"chain": "nested"}, true},
	}
	for _, tc := range cases {
		if e := checkLinkChain(tc.name, tc.target, tc.links); (e == nil) != tc.safe {
			t.Errorf("link %s -> %s: %v", tc.name, tc.target, e)
		}
	}
}

func TestDescribeUsesTrackedStatusWithoutRefreshingIndex(t *testing.T) {
	for _, state := range []string{"clean", "metadata only", "untracked only", "modified", "staged"} {
		t.Run(state, func(t *testing.T) {
			root := makeRepo(t)
			want := runGit(t, root, "rev-parse", "--short", "HEAD")
			if state == "untracked only" {
				if err := os.WriteFile(filepath.Join(root, "extra.txt"), []byte("untracked\n"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			if state == "modified" || state == "staged" {
				if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("Edited source\n"), 0644); err != nil {
					t.Fatal(err)
				}
				if state == "staged" {
					runGit(t, root, "add", "README.md")
				}
				want += "-dirty"
			}
			if state == "metadata only" {
				touched := time.Now().Add(2 * time.Second)
				if err := os.Chtimes(filepath.Join(root, "README.md"), touched, touched); err != nil {
					t.Fatal(err)
				}
			}
			index := filepath.Join(root, ".git", "index")
			before, err := os.ReadFile(index)
			if err != nil {
				t.Fatal(err)
			}
			beforeInfo, err := os.Stat(index)
			if err != nil {
				t.Fatal(err)
			}
			plan, err := Inspect(context.Background(), root, Options{})
			if err != nil {
				t.Fatal(err)
			}
			if plan.Facts.Describe != want {
				t.Errorf("description %q, want %q", plan.Facts.Describe, want)
			}
			after, err := os.ReadFile(index)
			if err != nil {
				t.Fatal(err)
			}
			afterInfo, err := os.Stat(index)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, after) || !beforeInfo.ModTime().Equal(afterInfo.ModTime()) {
				t.Fatal("inspection changed the Git index")
			}
		})
	}
}
