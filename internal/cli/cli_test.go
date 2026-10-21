package cli

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"github.com/vincentsch/sealjar/internal/version"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	packagefmt "github.com/vincentsch/sealjar/internal/pkg"
	"github.com/vincentsch/sealjar/internal/repo"
	"github.com/vincentsch/sealjar/internal/verify"
)

func write(t *testing.T, name string, b []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, b, 0644); err != nil {
		t.Fatal(err)
	}
}
func gitRun(t *testing.T, root string, args ...string) string {
	t.Helper()
	a := []string{"-c", "user.name=Snapshot Fixture", "-c", "user.email=snapshot@example.invalid", "-c", "commit.gpgsign=false", "-C", root}
	a = append(a, args...)
	cmd := exec.Command("git", a...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null")
	b, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, b)
	}
	return strings.TrimSpace(string(b))
}
func fixture(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "example-repo")
	if err := os.Mkdir(root, 0755); err != nil {
		t.Fatal(err)
	}
	source := "../../testdata/example-repo"
	err := filepath.WalkDir(source, func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, e := filepath.Rel(source, name)
		if e != nil {
			return e
		}
		b, e := os.ReadFile(name)
		if e != nil {
			return e
		}
		write(t, filepath.Join(root, rel), b)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(root, "run.sh"), []byte("#!/bin/sh\necho captured\n"))
	if err = os.Chmod(filepath.Join(root, "run.sh"), 0755); err != nil {
		t.Fatal(err)
	}
	mtime := time.Unix(1700000000, 123456789)
	if err = os.Chtimes(filepath.Join(root, "run.sh"), mtime, mtime); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(root, ".env.example"), []byte("EXAMPLE=placeholder\n"))
	write(t, filepath.Join(root, ".env.dist"), []byte("EXAMPLE=placeholder\n"))
	write(t, filepath.Join(root, "large.bin"), []byte("version https://git-lfs.github.com/spec/v1\noid sha256:"+strings.Repeat("a", 64)+"\nsize 1024\n"))
	if runtime.GOOS != "windows" {
		if err = os.Symlink("src/index.html", filepath.Join(root, "inside")); err != nil {
			t.Fatal(err)
		}
		if err = os.Symlink("../outside", filepath.Join(root, "outside")); err != nil {
			t.Fatal(err)
		}
	}
	gitRun(t, root, "init", "-b", "main")
	gitRun(t, root, "add", ".")
	gitRun(t, root, "add", "-f", ".env.example", ".env.dist")
	gitRun(t, root, "commit", "-m", "Add example project")
	gitRun(t, root, "remote", "add", "origin", "https://user:fake-password@example.invalid/project.git?token=fake-query")
	gitRun(t, root, "rm", "src/prices/index.html")
	if err = os.Remove(filepath.Join(root, "src/assets/photo4.jpg")); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(root, "src/index.html"), []byte("<h1>Current uncommitted content</h1>\n"))
	write(t, filepath.Join(root, "untracked.txt"), []byte("Untracked current content\n"))
	write(t, filepath.Join(root, ".env"), []byte("FAKE_SECRET=test-only-placeholder\n"))
	gitRun(t, root, "add", "-f", ".env")
	return root
}
func invoke(t *testing.T, expected int, args ...string) ([]byte, []byte) {
	t.Helper()
	var out, err bytes.Buffer
	code := Run(args, strings.NewReader(""), &out, &err)
	if code != expected {
		t.Fatalf("%v: exit %d, wanted %d\nstdout: %s\nstderr: %s", args, code, expected, out.Bytes(), err.Bytes())
	}
	return out.Bytes(), err.Bytes()
}
func oneJSON(t *testing.T, b []byte, v any) {
	t.Helper()
	d := json.NewDecoder(bytes.NewReader(b))
	if e := d.Decode(v); e != nil {
		t.Fatalf("invalid JSON: %v\n%s", e, b)
	}
	if e := d.Decode(new(any)); e != io.EOF {
		t.Fatalf("expected exactly one JSON document; extra decode: %v\n%s", e, b)
	}
}
func captured(t *testing.T, root, out string, extra ...string) string {
	t.Helper()
	args := []string{"capture", root, "--confirm", "--no-timestamp", "--out", out, "--json"}
	args = append(args, extra...)
	b, stderr := invoke(t, 0, args...)
	if len(stderr) > 0 {
		t.Fatalf("capture stderr: %s", stderr)
	}
	var result captureResult
	oneJSON(t, b, &result)
	return result.Package
}
func entries(t *testing.T, name string) map[string][]byte {
	t.Helper()
	r, e := zip.OpenReader(name)
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	out := map[string][]byte{}
	for _, f := range r.File {
		in, e := f.Open()
		if e != nil {
			t.Fatal(e)
		}
		b, e := io.ReadAll(in)
		in.Close()
		if e != nil {
			t.Fatal(e)
		}
		out[f.Name] = b
	}
	return out
}
func zipEntries(t *testing.T, name string, files map[string][]byte) {
	t.Helper()
	out, e := os.Create(name)
	if e != nil {
		t.Fatal(e)
	}
	z := zip.NewWriter(out)
	names := []string{}
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		w, e := z.Create(n)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = w.Write(files[n]); e != nil {
			t.Fatal(e)
		}
	}
	if e = z.Close(); e != nil {
		t.Fatal(e)
	}
	if e = out.Close(); e != nil {
		t.Fatal(e)
	}
}
func TestCaptureVerifyRestoreWorkingTree(t *testing.T) {
	root := fixture(t)
	output := t.TempDir()
	pkg := captured(t, root, output, "--history", "--label", "client=Example", "--name", "handover")
	files := entries(t, pkg)
	prefix := "repo/example-repo"
	var facts repo.Facts
	if e := json.Unmarshal(files[prefix+"/git.json"], &facts); e != nil {
		t.Fatal(e)
	}
	want := []repo.Deleted{{Path: "src/assets/photo4.jpg", Unstaged: true}, {Path: "src/prices/index.html", Staged: true}}
	if !reflect.DeepEqual(facts.DeletedPaths, want) {
		t.Fatalf("deleted paths: %#v", facts.DeletedPaths)
	}
	if len(facts.Remotes) != 1 || facts.Remotes[0].URL != "https://example.invalid/project.git" {
		t.Fatalf("remote credentials not removed: %#v", facts.Remotes)
	}
	excluded := map[string]string{}
	for _, e := range facts.Excluded {
		excluded[e.Path] = e.Reason
	}
	if excluded[".env"] != "may contain secrets" {
		t.Fatalf("missing secret exclusion: %#v", excluded)
	}
	if runtime.GOOS != "windows" && excluded["outside"] != "link points outside the repository" {
		t.Fatalf("outside link: %#v", excluded)
	}
	var records []repo.File
	if e := json.Unmarshal(files[prefix+"/files.json"], &records); e != nil {
		t.Fatal(e)
	}
	var m packagefmt.Manifest
	if e := json.Unmarshal(files["manifest.json"], &m); e != nil {
		t.Fatal(e)
	}
	if m.Labels["client"] != "Example" || m.TimeZone == "" || m.Type != packagefmt.ManifestType {
		t.Fatalf("manifest metadata: %#v", m)
	}
	previous := ""
	for _, rec := range m.Files {
		if rec.Path <= previous || packagefmt.Exempt(rec.Path) {
			t.Fatalf("manifest order/exemption: %s", rec.Path)
		}
		if rec.Size != int64(len(files[rec.Path])) || rec.SHA256 != packagefmt.Hash(files[rec.Path]) {
			t.Fatalf("manifest hash: %s", rec.Path)
		}
		previous = rec.Path
	}
	if len(m.Files) != len(files)-2 {
		t.Fatal("manifest does not cover payload")
	}
	b, stderr := invoke(t, 11, "verify", pkg, "--json")
	var result verify.Result
	oneJSON(t, b, &result)
	if len(stderr) > 0 {
		t.Fatalf("verdict stderr: %s", stderr)
	}
	again, stderr := invoke(t, 11, "verify", pkg, "--json")
	if !bytes.Equal(b, again) || len(stderr) > 0 {
		t.Fatal("verify JSON is not stable")
	}
	if result.Capture.State != "Complete" || result.Files.State != "Unchanged" || result.Timestamp.State != "Not timestamped" || result.Files.Matching != len(m.Files) {
		t.Fatalf("verdict: %#v", result)
	}
	human, _ := invoke(t, 11, "verify", pkg)
	if !strings.Contains(string(human), fmt.Sprintf("All %d files listed in the manifest match their recorded hashes.", len(m.Files))) {
		t.Fatalf("verify text: %s", human)
	}
	destination := filepath.Join(t.TempDir(), "recovered")
	invoke(t, 0, "restore", pkg, "--to", destination)
	pointerFound := false
	for _, rec := range records {
		full := filepath.Join(destination, filepath.FromSlash(rec.Path))
		if rec.Path == ".env" || rec.Path == "untracked.txt" {
			t.Fatalf("unexpected selected file: %s", rec.Path)
		}
		if rec.LinkTarget != "" {
			got, e := os.Readlink(full)
			if e != nil || got != rec.LinkTarget {
				t.Fatalf("restored link %s: %q %v", rec.Path, got, e)
			}
			continue
		}
		got, e := os.ReadFile(full)
		if e != nil {
			t.Fatal(e)
		}
		want, e := os.ReadFile(filepath.Join(root, filepath.FromSlash(rec.Path)))
		if e != nil || !bytes.Equal(got, want) {
			t.Fatalf("restored bytes differ for %s", rec.Path)
		}
		info, e := os.Stat(full)
		if e != nil {
			t.Fatal(e)
		}
		if runtime.GOOS != "windows" && fmt.Sprintf("%04o", info.Mode().Perm()) != rec.Mode {
			t.Fatalf("restored mode %s: %v", rec.Path, info.Mode())
		}
		if rec.Path == "run.sh" {
			sourceInfo, e := os.Stat(filepath.Join(root, rec.Path))
			if e != nil {
				t.Fatal(e)
			}
			if !info.ModTime().Equal(sourceInfo.ModTime()) {
				t.Fatalf("restored mtime differs: %v != %v", info.ModTime(), sourceInfo.ModTime())
			}
		}
		if rec.Path == "large.bin" {
			pointerFound = rec.PointerOnly
		}
		if rec.Path == ".env.example" || rec.Path == ".env.dist" {
			delete(excluded, rec.Path)
		}
	}
	if !pointerFound {
		t.Fatal("LFS pointer not marked")
	}
	for _, name := range []string{".env", "untracked.txt", "src/prices/index.html", "src/assets/photo4.jpg", "outside"} {
		if _, e := os.Lstat(filepath.Join(destination, name)); !os.IsNotExist(e) {
			t.Fatalf("restored excluded/deleted path: %s", name)
		}
	}
	clone := filepath.Join(t.TempDir(), "history")
	gitRun(t, t.TempDir(), "clone", destination+".history.bundle", clone)
	committed := gitRun(t, clone, "show", "HEAD:src/prices/index.html")
	if committed == "" {
		t.Fatal("history missing deleted committed file")
	}
	if _, e := os.Stat(filepath.Join(destination, ".git")); !os.IsNotExist(e) {
		t.Fatal("history was checked out into snapshot")
	}
	invoke(t, 1, "restore", pkg, "--to", destination)
	withUntracked := captured(t, root, t.TempDir(), "--include-untracked")
	restore2 := filepath.Join(t.TempDir(), "recovered")
	invoke(t, 0, "restore", withUntracked, "--to", restore2)
	b, e := os.ReadFile(filepath.Join(restore2, "untracked.txt"))
	if e != nil || !strings.Contains(string(b), "Untracked") {
		t.Fatal("untracked file missing")
	}
}
func TestConfirmationAndDryRuns(t *testing.T) {
	root := fixture(t)
	parent := t.TempDir()
	out := filepath.Join(parent, "not-created")
	invoke(t, 1, "capture", root, "--no-timestamp", "--out", out)
	invoke(t, 1, "capture", root, "--no-timestamp", "--out", out, "--no-prompt")
	b, _ := invoke(t, 0, "capture", root, filepath.Join(root, "src"), "--no-timestamp", "--out", out, "--dry-run")
	if !strings.Contains(string(b), ".env (may contain secrets)") {
		t.Fatalf("plan does not name excluded .env: %s", b)
	}
	if _, e := os.Stat(out); !os.IsNotExist(e) {
		t.Fatal("dry run or refusal created output")
	}
	b, _ = invoke(t, 0, "capture", root, "--no-timestamp", "--out", out, "--dry-run", "--json")
	var result captureResult
	oneJSON(t, b, &result)
	if !result.DryRun {
		t.Fatal("JSON dry run")
	}
	invoke(t, 1, "capture", root, fixture(t), "--no-timestamp", "--dry-run")
	invoke(t, 1, "capture", root, "--no-timestamp", "--exclude", "**/*.key", "--dry-run")
	invoke(t, 1, "capture", root, "--no-timestamp", "--name", "../unsafe", "--confirm")
	invoke(t, 1, "capture", root, "--no-timestamp", "--label", "invalid", "--confirm")
	pkg := captured(t, root, t.TempDir(), "--history")
	dest := filepath.Join(parent, "restored")
	invoke(t, 0, "restore", pkg, "--to", dest, "--dry-run")
	if _, e := os.Stat(dest); !os.IsNotExist(e) {
		t.Fatal("restore dry run created destination")
	}
	if _, e := os.Stat(dest + ".history.bundle"); !os.IsNotExist(e) {
		t.Fatal("restore dry run created bundle")
	}
	b, _ = invoke(t, 0, "--version")
	if !strings.Contains(string(b), version.Version) || !strings.Contains(string(b), version.Commit) {
		t.Fatal("version missing")
	}
	invoke(t, 5, "verify", filepath.Join(parent, "missing.zip"))
}
func TestRestorePreservesExistingHistorySidecar(t *testing.T) {
	root := fixture(t)
	pkg := captured(t, root, t.TempDir(), "--history")
	dest := filepath.Join(t.TempDir(), "recovered")
	write(t, dest+".history.bundle", []byte("keep existing history"))
	invoke(t, 1, "restore", pkg, "--to", dest)
	b, e := os.ReadFile(dest + ".history.bundle")
	if e != nil || string(b) != "keep existing history" {
		t.Fatal("existing bundle changed")
	}
	if _, e = os.Stat(dest); !os.IsNotExist(e) {
		t.Fatal("snapshot created despite sidecar collision")
	}
	if e = os.Mkdir(dest, 0755); e != nil {
		t.Fatal(e)
	}
	invoke(t, 1, "restore", pkg, "--to", dest)
	if items, e := os.ReadDir(dest); e != nil || len(items) != 0 {
		t.Fatal("empty destination changed")
	}
}
func TestPayloadTamperingVerdicts(t *testing.T) {
	original := captured(t, fixture(t), t.TempDir())
	base := entries(t, original)
	for _, kind := range []string{"changed", "missing", "unexpected"} {
		t.Run(kind, func(t *testing.T) {
			files := map[string][]byte{}
			for n, b := range base {
				files[n] = bytes.Clone(b)
			}
			name := "repo/example-repo/git.json"
			switch kind {
			case "changed":
				files[name][0] ^= 1
			case "missing":
				delete(files, name)
			case "unexpected":
				name = "extra.txt"
				files[name] = []byte("extra")
			}
			pkg := filepath.Join(t.TempDir(), "tampered.zip")
			zipEntries(t, pkg, files)
			b, stderr := invoke(t, 10, "verify", pkg, "--json")
			var result verify.Result
			oneJSON(t, b, &result)
			if len(stderr) > 0 {
				t.Fatalf("extra error document: %s", stderr)
			}
			again, _ := invoke(t, 10, "verify", pkg, "--json")
			if !bytes.Equal(b, again) {
				t.Fatal("unstable tamper verdict")
			}
			if len(result.Files.Differences) != 1 || result.Files.Differences[0].Path != name || result.Files.Differences[0].State != kind {
				t.Fatalf("tamper result: %#v", result)
			}
			dest := filepath.Join(t.TempDir(), "recovered")
			invoke(t, 10, "restore", pkg, "--to", dest)
			if _, e := os.Stat(dest); !os.IsNotExist(e) {
				t.Fatal("corrupt input wrote destination")
			}
		})
	}
	files := entries(t, original)
	var m packagefmt.Manifest
	json.Unmarshal(files["manifest.json"], &m)
	m.Type = "sealjar.manifest/v2"
	files["manifest.json"], _ = packagefmt.JSON(m)
	unsupported := filepath.Join(t.TempDir(), "future.zip")
	zipEntries(t, unsupported, files)
	invoke(t, 2, "verify", unsupported)
	files["manifest.json"] = []byte("{broken")
	malformed := filepath.Join(t.TempDir(), "broken.zip")
	zipEntries(t, malformed, files)
	b, stderr := invoke(t, 10, "verify", malformed, "--json")
	oneJSON(t, b, new(verify.Result))
	if len(stderr) != 0 {
		t.Fatal("malformed verdict produced stderr")
	}
}
func TestMalformedArchivesNeverExtract(t *testing.T) {
	original := captured(t, fixture(t), t.TempDir())
	base := entries(t, original)
	for _, name := range []string{"/absolute", "../escape", "repo/../escape", "C:/drive", "C:relative", `repo\file`, "manifest.json", "link"} {
		t.Run(name, func(t *testing.T) {
			pkg := filepath.Join(t.TempDir(), "bad.zip")
			out, e := os.Create(pkg)
			if e != nil {
				t.Fatal(e)
			}
			z := zip.NewWriter(out)
			for n, b := range base {
				w, e := z.Create(n)
				if e != nil {
					t.Fatal(e)
				}
				if _, e = w.Write(b); e != nil {
					t.Fatal(e)
				}
			}
			header := &zip.FileHeader{Name: name}
			if name == "link" {
				header.SetMode(os.ModeSymlink | 0777)
			}
			w, e := z.CreateHeader(header)
			if e != nil {
				t.Fatal(e)
			}
			w.Write([]byte("unsafe"))
			z.Close()
			out.Close()
			b, stderr := invoke(t, 10, "verify", pkg, "--json")
			oneJSON(t, b, new(verify.Result))
			if len(stderr) != 0 {
				t.Fatal("extra error document")
			}
			dest := filepath.Join(t.TempDir(), "recovered")
			invoke(t, 10, "restore", pkg, "--to", dest)
			if _, e = os.Stat(dest); !os.IsNotExist(e) {
				t.Fatal("malformed archive extracted")
			}
		})
	}
}
func TestSnapshotMemberChecks(t *testing.T) {
	original := captured(t, fixture(t), t.TempDir())
	base := entries(t, original)
	cases := []struct {
		name   string
		header tar.Header
		record repo.File
	}{
		{"absolute", tar.Header{Name: "/escape", Mode: 0644, Size: 1, Typeflag: tar.TypeReg}, repo.File{Path: "safe", Size: 1, Mode: "0644", SHA256: packagefmt.Hash([]byte("x"))}},
		{"mode", tar.Header{Name: "safe", Mode: 0755, Size: 1, Typeflag: tar.TypeReg}, repo.File{Path: "safe", Size: 1, Mode: "0644", SHA256: packagefmt.Hash([]byte("x"))}},
		{"hash", tar.Header{Name: "safe", Mode: 0644, Size: 1, Typeflag: tar.TypeReg}, repo.File{Path: "safe", Size: 1, Mode: "0644", SHA256: packagefmt.Hash([]byte("y"))}},
		{"outside link", tar.Header{Name: "safe", Mode: 0777, Linkname: "../escape", Typeflag: tar.TypeSymlink}, repo.File{Path: "safe", Size: 9, Mode: "0777", LinkTarget: "../escape", SHA256: packagefmt.Hash([]byte("../escape"))}},
		{"link target", tar.Header{Name: "safe", Mode: 0777, Linkname: "another", Typeflag: tar.TypeSymlink}, repo.File{Path: "safe", Size: 6, Mode: "0777", LinkTarget: "inside", SHA256: packagefmt.Hash([]byte("inside"))}},
		{"hard link", tar.Header{Name: "safe", Mode: 0644, Linkname: "inside", Typeflag: tar.TypeLink}, repo.File{Path: "safe", Size: 1, Mode: "0644", SHA256: packagefmt.Hash([]byte("x"))}},
	}
	for _, name := range []string{"../escape", "C:/drive", `folder\file`, "duplicate"} {
		headerName := name
		if name == "duplicate" {
			headerName = "safe"
		}
		cases = append(cases, struct {
			name   string
			header tar.Header
			record repo.File
		}{name, tar.Header{Name: headerName, Mode: 0644, Size: 1, Typeflag: tar.TypeReg}, repo.File{Path: "safe", Size: 1, Mode: "0644", SHA256: packagefmt.Hash([]byte("x"))}})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			files := map[string][]byte{}
			for n, b := range base {
				files[n] = b
			}
			var buf bytes.Buffer
			gz := gzip.NewWriter(&buf)
			tw := tar.NewWriter(gz)
			if e := tw.WriteHeader(&tc.header); e != nil {
				t.Fatal(e)
			}
			if tc.header.Size > 0 {
				tw.Write([]byte("x"))
			}
			if tc.name == "duplicate" {
				if e := tw.WriteHeader(&tc.header); e != nil {
					t.Fatal(e)
				}
				tw.Write([]byte("x"))
			}
			tw.Close()
			gz.Close()
			files["repo/example-repo/snapshot.tar.gz"] = buf.Bytes()
			files["repo/example-repo/files.json"], _ = packagefmt.JSON([]repo.File{tc.record})
			var m packagefmt.Manifest
			json.Unmarshal(files["manifest.json"], &m)
			for i, rec := range m.Files {
				m.Files[i].Size = int64(len(files[rec.Path]))
				m.Files[i].SHA256 = packagefmt.Hash(files[rec.Path])
			}
			files["manifest.json"], _ = packagefmt.JSON(m)
			pkg := filepath.Join(t.TempDir(), "bad-snapshot.zip")
			zipEntries(t, pkg, files)
			invoke(t, 10, "verify", pkg)
			parent := t.TempDir()
			dest := filepath.Join(parent, "recovered")
			invoke(t, 10, "restore", pkg, "--to", dest)
			items, e := os.ReadDir(parent)
			if e != nil || len(items) != 0 {
				t.Fatal("malformed snapshot wrote output")
			}
		})
	}
}
func TestCaptureNameCollisionsPreserveReaderFolders(t *testing.T) {
	root := fixture(t)
	out := t.TempDir()
	first := captured(t, root, out, "--name", "handover")
	reader := strings.TrimSuffix(first, ".zip")
	write(t, filepath.Join(reader, "keep.txt"), []byte("keep reader"))
	moved := filepath.Join(t.TempDir(), "moved.zip")
	if e := os.Rename(first, moved); e != nil {
		t.Fatal(e)
	}
	second := captured(t, root, out, "--name", "handover")
	if !strings.HasSuffix(second, "-2.sealjar.zip") {
		t.Fatalf("orphan reader folder reused: %s", second)
	}
	third := captured(t, root, out, "--name", "handover")
	if !strings.HasSuffix(third, "-3.sealjar.zip") {
		t.Fatalf("ZIP collision: %s", third)
	}
	b, e := os.ReadFile(filepath.Join(reader, "keep.txt"))
	if e != nil || string(b) != "keep reader" {
		t.Fatal("reader folder changed")
	}
}
func TestManualPythonCheck(t *testing.T) {
	if _, e := exec.LookPath("python3"); e != nil {
		t.Skip("Python 3 is not installed")
	}
	pkg := captured(t, fixture(t), t.TempDir())
	files := entries(t, pkg)
	dir := t.TempDir()
	for n, b := range files {
		write(t, filepath.Join(dir, filepath.FromSlash(n)), b)
	}
	parts := strings.Split(string(packagefmt.Instructions), "```python\n")
	if len(parts) != 2 {
		t.Fatal("manual Python block missing")
	}
	script := strings.SplitN(parts[1], "```", 2)[0]
	check := func(want int) {
		t.Helper()
		cmd := exec.Command("python3", "-c", script)
		cmd.Dir = dir
		b, e := cmd.CombinedOutput()
		code := 0
		if e != nil {
			exit, ok := e.(*exec.ExitError)
			if !ok {
				t.Fatal(e)
			}
			code = exit.ExitCode()
		}
		if code != want {
			t.Fatalf("manual check exit %d wanted %d: %s", code, want, b)
		}
		if want == 1 && !strings.Contains(string(b), "changed: repo/example-repo/git.json") {
			t.Fatalf("manual check failed to name file: %s", b)
		}
	}
	check(0)
	write(t, filepath.Join(dir, "repo/example-repo/git.json"), []byte("changed"))
	check(1)
}

func TestRestoreIntoEmptyDirectory(t *testing.T) {
	pkg := captured(t, fixture(t), t.TempDir())
	dest := filepath.Join(t.TempDir(), "empty")
	if e := os.Mkdir(dest, 0755); e != nil {
		t.Fatal(e)
	}
	invoke(t, 0, "restore", pkg, "--to", dest)
	before, e := os.ReadFile(filepath.Join(dest, "src/index.html"))
	if e != nil {
		t.Fatal(e)
	}
	invoke(t, 1, "restore", pkg, "--to", dest)
	after, e := os.ReadFile(filepath.Join(dest, "src/index.html"))
	if e != nil || !bytes.Equal(before, after) {
		t.Fatal("non-empty destination changed")
	}
}

func TestManualTimestampTimeConversion(t *testing.T) {
	if _, e := exec.LookPath("python3"); e != nil {
		t.Skip("Python 3 is not installed")
	}
	parts := strings.Split(string(packagefmt.Instructions), "```sh\n")
	if len(parts) != 2 {
		t.Fatal("manual time conversion block missing")
	}
	script := strings.SplitN(parts[1], "```", 2)[0]
	cmd := exec.Command("sh", "-c", script)
	b, e := cmd.CombinedOutput()
	if e != nil {
		t.Fatalf("manual time conversion: %v %s", e, b)
	}
	want := fmt.Sprintf("%d", time.Date(2026, 10, 20, 18, 45, 0, 0, time.UTC).Unix())
	if strings.TrimSpace(string(b)) != want {
		t.Fatalf("time conversion: %s, wanted %s", b, want)
	}
}

func TestVerifyRestoreWithoutGit(t *testing.T) {
	root := fixture(t)
	pkg := captured(t, root, t.TempDir())
	dest := filepath.Join(t.TempDir(), "recovered")
	t.Setenv("PATH", t.TempDir())
	invoke(t, 11, "verify", pkg, "--json")
	invoke(t, 0, "restore", pkg, "--to", dest)
	_, stderr := invoke(t, 2, "capture", root, "--confirm", "--no-timestamp")
	if !strings.Contains(string(stderr), "Git was not found") {
		t.Fatalf("missing Git error: %s", stderr)
	}
}

func TestCaptureSkipsNestedRepositoriesAndWorktrees(t *testing.T) {
	root := fixture(t)
	gitRun(t, root, "clone", "--local", root, filepath.Join(root, "tools", "helper"))
	gitRun(t, root, "worktree", "add", "--detach", filepath.Join(root, ".worktrees", "feature"), "HEAD")
	out := filepath.Join(t.TempDir(), "packages")
	for _, extra := range [][]string{nil, {"--exclude", "tools/", "--exclude", ".worktrees/"}} {
		args := []string{"capture", root, "--include-untracked", "--no-timestamp", "--dry-run", "--out", out, "--json"}
		args = append(args, extra...)
		b, _ := invoke(t, 0, args...)
		var result captureResult
		oneJSON(t, b, &result)
		excluded := map[string]string{}
		for _, item := range result.Plan.Facts.Excluded {
			excluded[item.Path] = item.Reason
		}
		for _, name := range []string{"tools/helper", ".worktrees/feature"} {
			if excluded[name] != "nested Git repository; contents not included" {
				t.Errorf("nested repository %q missing from plan: %#v", name, excluded)
			}
		}
	}
	human, _ := invoke(t, 0, "capture", root, "--include-untracked", "--no-timestamp", "--dry-run", "--out", out)
	for _, name := range []string{"tools/helper", ".worktrees/feature"} {
		if !strings.Contains(string(human), name+" (nested Git repository; contents not included)") {
			t.Errorf("nested repository %q missing from human plan", name)
		}
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatal("dry run created output")
	}
	pkg := captured(t, root, out, "--include-untracked")
	invoke(t, 11, "verify", pkg)
	destination := filepath.Join(t.TempDir(), "restored")
	invoke(t, 0, "restore", pkg, "--to", destination)
	for _, name := range []string{"tools/helper", ".worktrees/feature"} {
		if _, err := os.Stat(filepath.Join(destination, filepath.FromSlash(name))); !os.IsNotExist(err) {
			t.Fatalf("nested repository %q was restored", name)
		}
	}
	if _, err := os.Stat(filepath.Join(destination, "untracked.txt")); err != nil {
		t.Fatal("ordinary untracked file was not restored")
	}
}

func TestRestoreHistoryPathsWithTrailingSeparator(t *testing.T) {
	pkg := captured(t, fixture(t), t.TempDir(), "--history")
	dest := filepath.Join(t.TempDir(), "recovered")
	if err := os.Mkdir(dest, 0755); err != nil {
		t.Fatal(err)
	}
	trailing := dest + string(filepath.Separator)
	b, _ := invoke(t, 0, "restore", pkg, "--to", trailing, "--dry-run", "--json")
	var preview restoreResult
	oneJSON(t, b, &preview)
	wantHistory := resultPath(dest + ".history.bundle")
	wantCommand := "git clone " + shellQuote(wantHistory) + " " + shellQuote(resultPath(dest+"-history"))
	if preview.Destination != resultPath(dest) || preview.History != wantHistory || preview.CloneCommand != wantCommand {
		t.Fatalf("trailing separator preview: %#v", preview)
	}
	b, _ = invoke(t, 0, "restore", pkg, "--to", trailing, "--json")
	var result restoreResult
	oneJSON(t, b, &result)
	if result.History != preview.History || result.CloneCommand != preview.CloneCommand {
		t.Fatalf("restore differs from preview: %#v", result)
	}
	if _, err := os.Stat(dest + ".history.bundle"); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		cmd := exec.Command("sh", "-c", result.CloneCommand)
		cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("printed clone command failed: %v %s", err, out)
		}
		if _, err := os.Stat(filepath.Join(dest+"-history", ".git")); err != nil {
			t.Fatal("clone was not created beside snapshot")
		}
	}
	if _, err := os.Stat(filepath.Join(dest, ".git")); !os.IsNotExist(err) {
		t.Fatal("clone landed inside snapshot")
	}
	humanDest := filepath.Join(t.TempDir(), "human")
	human, _ := invoke(t, 0, "restore", pkg, "--to", humanDest+string(filepath.Separator))
	humanHistory := resultPath(humanDest + ".history.bundle")
	if !strings.Contains(string(human), "History remains separate: "+humanHistory+"\n") || !strings.Contains(string(human), "git clone "+shellQuote(humanHistory)+" "+shellQuote(resultPath(humanDest+"-history"))) {
		t.Fatalf("human history paths are wrong: %s", human)
	}
}

func TestCaptureLeavesGitIndexUnchanged(t *testing.T) {
	for _, dryRun := range []bool{true, false} {
		t.Run(fmt.Sprintf("dry_run_%t", dryRun), func(t *testing.T) {
			root := fixture(t)
			readme := filepath.Join(root, "README.md")
			touched := time.Now().Add(2 * time.Second)
			if err := os.Chtimes(readme, touched, touched); err != nil {
				t.Fatal(err)
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
			args := []string{"capture", root, "--no-timestamp", "--out", t.TempDir()}
			if dryRun {
				args = append(args, "--dry-run")
			} else {
				args = append(args, "--confirm")
			}
			invoke(t, 0, args...)
			after, err := os.ReadFile(index)
			if err != nil {
				t.Fatal(err)
			}
			afterInfo, err := os.Stat(index)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, after) || !beforeInfo.ModTime().Equal(afterInfo.ModTime()) {
				t.Fatal("capture rewrote the Git index after a metadata-only file change")
			}
		})
	}
}
