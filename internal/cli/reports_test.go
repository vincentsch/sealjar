package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	packagefmt "github.com/vincentsch/sealjar/internal/pkg"
	"github.com/vincentsch/sealjar/internal/tsa/testtsa"
)

func readerFiles(t *testing.T, dir string) []string {
	t.Helper()
	names := []string{}
	e := filepath.WalkDir(dir, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if !d.IsDir() {
			rel, e := filepath.Rel(dir, p)
			if e != nil {
				return e
			}
			names = append(names, filepath.ToSlash(rel))
		}
		return nil
	})
	if e != nil {
		t.Fatal(e)
	}
	sort.Strings(names)
	return names
}
func TestHTMLOnlyReaderAndAvailability(t *testing.T) {
	original := findChrome
	findChrome = func(string) (string, error) { return "", fmt.Errorf("Chrome unavailable") }
	t.Cleanup(func() { findChrome = original })
	root := fixture(t)
	out := t.TempDir()
	b, _ := invoke(t, 0, "capture", root, "--dry-run", "--json", "--out", out)
	var plan captureResult
	oneJSON(t, b, &plan)
	if !strings.Contains(plan.PDFNote, "HTML only") {
		t.Fatal(string(b))
	}
	b, _ = invoke(t, 0, "capture", root, "--confirm", "--no-timestamp", "--out", out, "--json")
	var capture captureResult
	oneJSON(t, b, &capture)
	if capture.PDFNote == "" {
		t.Fatal("missing PDF explanation")
	}
	want := []string{"SEAL.html", "VERIFY.txt", "report/report.de.html", "report/report.en.html"}
	if got := readerFiles(t, capture.ReaderFolder); !reflect.DeepEqual(got, want) {
		t.Fatalf("reader files: %v", got)
	}
	dest := filepath.Join(t.TempDir(), "reader")
	invoke(t, 5, "report", capture.Package, "--out", dest, "--pdf")
	if _, e := os.Stat(dest); !os.IsNotExist(e) {
		t.Fatal("missing PDFs wrote output")
	}
	if e := os.Mkdir(dest, 0700); e != nil {
		t.Fatal(e)
	}
	invoke(t, 0, "report", capture.Package, "--out", dest, "--lang", "de")
	want = []string{"SEAL.html", "VERIFY.txt", "report/report.de.html"}
	if got := readerFiles(t, dest); !reflect.DeepEqual(got, want) {
		t.Fatal(got)
	}
	invoke(t, 1, "report", capture.Package, "--out", dest)
	invoke(t, 0, "report", capture.Package, "--out", filepath.Join(t.TempDir(), "dry"), "--dry-run")
	// An absent requested language is different from a changed authenticated report.
	b, _ = invoke(t, 0, "capture", root, "--confirm", "--no-timestamp", "--lang", "en", "--out", out, "--json")
	var english captureResult
	oneJSON(t, b, &english)
	invoke(t, 5, "report", english.Package, "--out", filepath.Join(t.TempDir(), "missing"), "--lang", "de")
	files := entries(t, capture.Package)
	files["report/report.en.html"] = append(files["report/report.en.html"], []byte("changed")...)
	bad := filepath.Join(t.TempDir(), "bad.zip")
	zipEntries(t, bad, files)
	v := assertVerify(t, 10, bad)
	if v.Files.Differences[0].Path != "report/report.en.html" {
		t.Fatal(v.Files)
	}
	invoke(t, 10, "report", bad, "--out", filepath.Join(t.TempDir(), "bad-reader"))
}
func TestReportSealTrustAndStampPreservation(t *testing.T) {
	original := findChrome
	findChrome = func(string) (string, error) { return "", fmt.Errorf("Chrome unavailable") }
	t.Cleanup(func() { findChrome = original })
	unsigned := captured(t, fixture(t), t.TempDir())
	url, root := localTSA(t, testtsa.Options{})
	signed := stamp(t, unsigned, "--tsa", url, "--tsa-root", root)
	before := entries(t, signed)
	for _, trusted := range []bool{false, true} {
		out := filepath.Join(t.TempDir(), "reader")
		args := []string{"report", signed, "--out", out, "--lang", "en"}
		if trusted {
			args = append(args, "--tsa-root", root)
		}
		invoke(t, 0, args...)
		seal, e := os.ReadFile(filepath.Join(out, "SEAL.html"))
		if e != nil {
			t.Fatal(e)
		}
		needle := "Untrusted"
		if trusted {
			needle = "Trusted"
		}
		if !bytes.Contains(seal, []byte(needle)) {
			t.Fatalf("seal lacks %s", needle)
		}
	}
	second := stamp(t, signed, "--tsa", url, "--tsa-root", root)
	after := entries(t, second)
	for n, b := range before {
		if n != "SEAL.html" && !bytes.Equal(b, after[n]) {
			t.Fatalf("stamp changed %s", n)
		}
	}
	if bytes.Equal(before["SEAL.html"], after["SEAL.html"]) {
		t.Fatal("stamp did not refresh seal")
	}
	if len(readerFiles(t, strings.TrimSuffix(second, ".zip"))) != 4 {
		t.Fatal("stamp reader incomplete")
	}
}
func TestPairedOutputCollisions(t *testing.T) {
	original := findChrome
	findChrome = func(string) (string, error) { return "", fmt.Errorf("Chrome unavailable") }
	t.Cleanup(func() { findChrome = original })
	out := t.TempDir()
	root := fixture(t)
	first := captured(t, root, out)
	files := entries(t, first)
	if e := os.Remove(first); e != nil {
		t.Fatal(e)
	}
	second := captured(t, root, out)
	if !strings.HasSuffix(second, "-2.sealjar.zip") {
		t.Fatal(second)
	}
	url, pem := localTSA(t, testtsa.Options{})
	input := filepath.Join(t.TempDir(), "capture.sealjar.zip")
	zipEntries(t, input, files)
	orphan := filepath.Join(filepath.Dir(input), "capture-stamped.sealjar")
	if e := os.Mkdir(orphan, 0700); e != nil {
		t.Fatal(e)
	}
	stamped := stamp(t, input, "--tsa", url, "--tsa-root", pem)
	if !strings.HasSuffix(stamped, "-stamped-2.sealjar.zip") {
		t.Fatal(stamped)
	}
	r, e := packagefmt.Open(stamped)
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	for _, f := range r.Manifest.Files {
		if packagefmt.Exempt(f.Path) {
			t.Fatal(f.Path)
		}
	}
}

func TestReaderDestinationWithTrailingSeparator(t *testing.T) {
	original := findChrome
	findChrome = func(string) (string, error) { return "", fmt.Errorf("Chrome unavailable") }
	t.Cleanup(func() { findChrome = original })
	pkg := captured(t, fixture(t), t.TempDir())
	for _, existing := range []bool{false, true} {
		t.Run(fmt.Sprintf("existing_%t", existing), func(t *testing.T) {
			out := filepath.Join(t.TempDir(), "reader")
			if existing {
				if err := os.Mkdir(out, 0700); err != nil {
					t.Fatal(err)
				}
			}
			withSeparator := out + string(filepath.Separator)
			invoke(t, 0, "report", pkg, "--out", withSeparator, "--lang", "en", "--dry-run")
			if existing {
				if names := readerFiles(t, out); len(names) != 0 {
					t.Fatal("dry run changed the empty destination")
				}
			} else if _, err := os.Stat(out); !os.IsNotExist(err) {
				t.Fatal("dry run created the destination")
			}
			invoke(t, 0, "report", pkg, "--out", withSeparator, "--lang", "en")
			want := []string{"SEAL.html", "VERIFY.txt", "report/report.en.html"}
			if got := readerFiles(t, out); !reflect.DeepEqual(got, want) {
				t.Fatalf("reader files: %v", got)
			}
			before, err := os.ReadFile(filepath.Join(out, "report/report.en.html"))
			if err != nil {
				t.Fatal(err)
			}
			invoke(t, 1, "report", pkg, "--out", withSeparator, "--lang", "en")
			after, err := os.ReadFile(filepath.Join(out, "report/report.en.html"))
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("refused extraction changed the report")
			}
			if got := readerFiles(t, out); !reflect.DeepEqual(got, want) {
				t.Fatalf("refused extraction changed files: %v", got)
			}
		})
	}
}

func TestCaptureAuthenticityNotice(t *testing.T) {
	original := findChrome
	findChrome = func(string) (string, error) { return "", fmt.Errorf("Chrome unavailable") }
	t.Cleanup(func() { findChrome = original })
	repo := fixture(t)
	authority, root := localTSA(t, testtsa.Options{})
	otherAuthority, _ := localTSA(t, testtsa.Options{})
	for _, tc := range []struct {
		name   string
		flags  []string
		notice bool
	}{
		{"missing", []string{"--no-timestamp"}, true},
		{"failed", []string{"--tsa", "http://127.0.0.1:1/"}, true},
		{"untrusted", []string{"--tsa", authority}, true},
		{"trusted", []string{"--tsa", authority, "--tsa-root", root}, false},
		{"mixed_trust", []string{"--tsa", otherAuthority, "--tsa", authority, "--tsa-root", root}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{"capture", repo, "--confirm", "--out", t.TempDir()}, tc.flags...)
			out, _ := invoke(t, 0, args...)
			want := 0
			if tc.notice {
				want = 1
			}
			if got := bytes.Count(out, []byte("Hashes alone do not establish the authenticity of an untimestamped manifest.")); got != want {
				t.Fatalf("notice count %d, wanted %d: %s", got, want, out)
			}
		})
	}
}
