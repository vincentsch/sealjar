package packagefmt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWritePreservesCollisionsAtFixedTime(t *testing.T) {
	source, out := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "example.txt"), []byte("capture"), 0600); err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 10, 23, 12, 34, 0, 0, time.UTC)
	first, err := Write(source, out, "handover", at)
	if err != nil {
		t.Fatal(err)
	}
	reader := strings.TrimSuffix(first, ".zip")
	if err := os.Mkdir(reader, 0700); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(reader, "keep.txt")
	if err := os.WriteFile(sentinel, []byte("keep reader"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(first, filepath.Join(t.TempDir(), "moved.zip")); err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{"-2.sealjar.zip", "-3.sealjar.zip"} {
		result, err := Write(source, out, "handover", at)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasSuffix(result, suffix) {
			t.Fatalf("collision path %s, wanted suffix %s", result, suffix)
		}
	}
	if data, err := os.ReadFile(sentinel); err != nil || string(data) != "keep reader" {
		t.Fatalf("reader changed: %q, %v", data, err)
	}
}
