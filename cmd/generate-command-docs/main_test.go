package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReferenceLifecycle(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "reference")
	for _, name := range []string{"commands.md", "media/example.svg"} {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("handwritten"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := generate(true, dir); err == nil {
		t.Fatal("missing reference passed")
	}
	if err := generate(false, dir); err != nil {
		t.Fatal(err)
	}
	page := filepath.Join(dir, "sealjar_capture.md")
	first, err := os.ReadFile(page)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(first), "--out ../sealjar-captures") != 2 {
		t.Fatal("unsafe capture examples", string(first))
	}
	if !strings.Contains(string(first), "Defaults to human-readable output. Use `--json` for JSON.") ||
		!strings.Contains(string(first), "`--dry-run` previews changes without writing files.") {
		t.Fatal("reference output and preview guidance missing", string(first))
	}
	if err := generate(true, dir); err != nil {
		t.Fatal("generated reference failed freshness check", err)
	}
	if err := generate(false, dir); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(page)
	if string(first) != string(second) {
		t.Fatal("generation changed on second run")
	}
	oldProse := strings.NewReplacer(
		"Defaults to human-readable output. Use `--json` for JSON.",
		"Shows text you can read in your terminal.\n\nUse --json to get JSON for scripts and other programs.",
		"`--dry-run` previews changes without writing files.",
		"This command makes changes. Use `--dry-run` to see what would change without changing anything.",
	).Replace(string(first))
	if err := os.WriteFile(page, []byte(oldProse), 0644); err != nil {
		t.Fatal(err)
	}
	if err := generate(true, dir); err == nil {
		t.Fatal("old generic prose passed freshness check")
	}
	if err := os.WriteFile(page, []byte("stale"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := generate(true, dir); err == nil {
		t.Fatal("stale reference passed")
	}
	orphan := filepath.Join(dir, "old.md")
	if err := os.WriteFile(orphan, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := generate(false, dir); err != nil {
		t.Fatal(err)
	}
	if err := generate(true, dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(orphan); !os.IsNotExist(err) {
		t.Fatal("obsolete reference survived", err)
	}
	for _, name := range []string{"commands.md", "media/example.svg"} {
		b, err := os.ReadFile(filepath.Join(root, name))
		if err != nil || string(b) != "handwritten" {
			t.Fatal("handwritten content changed", name, err)
		}
	}
}
