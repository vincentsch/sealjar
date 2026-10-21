package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func withoutReportBrowser(t *testing.T) {
	t.Helper()
	original := findChrome
	findChrome = func(string) (string, error) { return "", fmt.Errorf("Chrome unavailable") }
	t.Cleanup(func() { findChrome = original })
}

func TestCaptureDestinationInPlan(t *testing.T) {
	withoutReportBrowser(t)
	repo := fixture(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.Mkdir(filepath.Join(home, "work"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Chdir(filepath.Join(home, "work"))
	for _, explicit := range []bool{false, true} {
		for _, jsonMode := range []bool{false, true} {
			args := []string{"capture", repo, "http://127.0.0.1:1/", "--dry-run", "--no-timestamp"}
			want := filepath.Join(home, "work")
			if explicit {
				args = append(args, "--out", "../absent/reader")
				want = filepath.Join(home, "absent", "reader")
			}
			if jsonMode {
				args = append(args, "--json")
			}
			out, _ := invoke(t, 0, args...)
			if jsonMode {
				var result struct {
					Output string `json:"output_directory"`
				}
				oneJSON(t, out, &result)
				if result.Output != want {
					t.Fatalf("destination: %s", out)
				}
			} else if !strings.Contains(string(out), "Output directory: "+want) {
				t.Fatalf("plan: %s", out)
			}
		}
	}
	if _, err := os.Stat(filepath.Join(home, "absent")); !os.IsNotExist(err) {
		t.Fatal("dry run created destination")
	}
}

func TestMissingDestinationParentGuidance(t *testing.T) {
	withoutReportBrowser(t)
	pkg := captured(t, fixture(t), t.TempDir())
	for _, command := range []string{"report", "restore"} {
		for _, jsonMode := range []bool{false, true} {
			parent := filepath.Join(t.TempDir(), "new-parent")
			dest := filepath.Join(parent, "result")
			flag := "--out"
			if command == "restore" {
				flag = "--to"
			}
			args := []string{command, pkg, flag, dest}
			if jsonMode {
				args = append(args, "--json")
			}
			invoke(t, 0, append(args, "--dry-run")...)
			if _, err := os.Stat(parent); !os.IsNotExist(err) {
				t.Fatal("dry run created parent")
			}
			out, stderr := invoke(t, 2, args...)
			all := string(out) + string(stderr)
			if !strings.Contains(all, parent) || !strings.Contains(all, "Create it before retrying") {
				t.Fatalf("missing guidance: %s", all)
			}
			if err := os.Mkdir(parent, 0700); err != nil {
				t.Fatal(err)
			}
			invoke(t, 0, args...)
			if _, err := os.Stat(dest); err != nil {
				t.Fatal(err)
			}
			invoke(t, 1, args...)
		}
	}
}

func TestVerifyCommandQuoting(t *testing.T) {
	zip := "client handover/O'Brien $HOME `whoami` $(pwd).sealjar.zip"
	root := "roots/O'Brien $HOME `whoami` $(pwd).pem"
	posix := verifyCommandLine(zip, root, "linux")
	want := "sealjar verify 'client handover/O'\"'\"'Brien $HOME `whoami` $(pwd).sealjar.zip' --tsa-root 'roots/O'\"'\"'Brien $HOME `whoami` $(pwd).pem'"
	if posix != want || verifyCommandLine(zip, root, "darwin") != want {
		t.Fatal(posix)
	}
	windows := verifyCommandLine(zip, root, "windows")
	want = "sealjar verify 'client handover/O''Brien $HOME `whoami` $(pwd).sealjar.zip' --tsa-root 'roots/O''Brien $HOME `whoami` $(pwd).pem'"
	if windows != want {
		t.Fatal(windows)
	}
	for _, platform := range []string{"linux", "windows"} {
		if strings.Contains(verifyCommandLine(zip, "", platform), "--tsa-root") {
			t.Fatal("invented root")
		}
	}
}
