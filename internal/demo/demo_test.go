package demo

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	packagefmt "github.com/vincentsch/sealjar/internal/pkg"
	"github.com/vincentsch/sealjar/internal/repo"
	"github.com/vincentsch/sealjar/internal/verify"
)

func TestEmbeddedFixturesMatchTrackedSources(t *testing.T) {
	cmd := exec.Command("git", "ls-files", "-z", "--", "testdata/site", "testdata/example-repo")
	cmd.Dir = "../.."
	tracked, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	expected := map[string]bool{}
	for _, entry := range bytes.Split(tracked, []byte{0}) {
		if len(entry) == 0 {
			continue
		}
		name := string(entry)
		name = "data/" + name[len("testdata/"):]
		expected[name] = true
		src, err := os.ReadFile(filepath.Join("../..", string(entry)))
		if err != nil {
			t.Fatal(err)
		}
		embedded, err := Files.ReadFile(name)
		if err != nil || !bytes.Equal(src, embedded) {
			t.Fatalf("%s differs; run go generate ./internal/demo", name)
		}
	}
	err = fs.WalkDir(Files, "data", func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && !expected[name] {
			t.Errorf("untracked embedded fixture: %s", name)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !expected["data/example-repo/.gitignore"] {
		t.Fatal("dotfile missing")
	}
}
func TestInputsExcludeFakeSecretAndCleanUp(t *testing.T) {
	hooks := t.TempDir()
	if err := os.WriteFile(filepath.Join(hooks, "prepare-commit-msg"), []byte("#!/bin/sh\nexit 77\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "core.hooksPath")
	t.Setenv("GIT_CONFIG_VALUE_0", hooks)
	in, err := Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	plan, err := repo.Inspect(context.Background(), in.Repository, repo.Options{})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range plan.Facts.Excluded {
		if e.Path == ".env" && e.Reason == "may contain secrets" {
			found = true
		}
	}
	if !found || plan.Facts.Status.Modified != 1 {
		t.Fatalf("unexpected fixture facts: %+v", plan.Facts)
	}
	for _, p := range plan.Paths {
		if p == ".env" {
			t.Fatal("secret selected")
		}
	}
	response, err := http.Get(in.SiteURL + "/prices/")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 200 {
		t.Fatal(response.Status)
	}
	repoPath, siteURL, tsaURL := in.Repository, in.SiteURL, in.TimestampURL
	in.Close()
	if _, err := os.Stat(repoPath); !os.IsNotExist(err) {
		t.Fatalf("repository remains: %v", err)
	}
	for _, endpoint := range []string{siteURL, tsaURL} {
		if resp, err := http.Get(endpoint); err == nil {
			resp.Body.Close()
			t.Fatalf("server remains: %s", endpoint)
		}
	}
}
func TestTamperReadableAndNeverOverwrites(t *testing.T) {
	work, out := t.TempDir(), t.TempDir()
	if err := os.MkdirAll(filepath.Join(work, "web/001-home"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(work, "web/001-home/desktop.png"), []byte("screenshot fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	m := packagefmt.Manifest{Type: packagefmt.ManifestType, ToolVersion: "test", CaptureID: "demo-test", Started: time.Now(), Ended: time.Now(), TimeZone: "UTC", Targets: []packagefmt.Target{{Name: "bakery", Kind: "web", State: "complete", Payload: "web/001-home"}}}
	if err := packagefmt.BuildManifest(work, &m); err != nil {
		t.Fatal(err)
	}
	source, err := packagefmt.WriteStem(work, out, "original")
	if err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{"", "-2", "-3"} {
		name, changed, err := Tamper(source, out)
		if err != nil {
			t.Fatal(err)
		}
		if filepath.Base(name) != "demo-tampered"+suffix+".sealjar.zip" {
			t.Fatal(name)
		}
		raw, err := zip.OpenReader(name)
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range raw.File {
			r, e := f.Open()
			if e != nil {
				t.Fatal(e)
			}
			_, e = io.Copy(io.Discard, r)
			r.Close()
			if e != nil {
				t.Fatal(e)
			}
		}
		raw.Close()
		r, err := packagefmt.Open(name)
		if err != nil {
			t.Fatal(err)
		}
		v, code, err := verify.Check(r)
		r.Close()
		if err != nil || code != 10 || len(v.Files.Differences) != 1 || v.Files.Differences[0].Path != changed {
			t.Fatalf("%+v %d %v", v, code, err)
		}
	}
}
