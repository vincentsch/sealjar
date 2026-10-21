package browser

import (
	"context"
	"github.com/vincentsch/sealjar/internal/testutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestIsolatedProfileCleanup(t *testing.T) {
	p, e := Find("")
	if e != nil {
		testutil.RequireChrome(t, e)
	}
	b, e := Start(context.Background(), p, false)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(b.Profile); e != nil {
		t.Fatal(e)
	}
	profile := b.Profile
	b.Close()
	if _, e = os.Stat(profile); !os.IsNotExist(e) {
		t.Fatal("temporary profile survived browser shutdown")
	}
}
func TestDiscoveryFailure(t *testing.T) {
	if _, e := Find("/missing/chrome"); e == nil {
		t.Fatal("accepted missing explicit browser")
	}
}

func TestVersionUsesIsolatedBrowser(t *testing.T) {
	path, err := Find("")
	if err != nil {
		testutil.RequireChrome(t, err)
	}
	b, err := Start(context.Background(), path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	version, err := b.Version(context.Background())
	if err != nil || !strings.Contains(version, "Chrome/") {
		t.Fatalf("isolated version: %q, %v", version, err)
	}
	probe, err := Version(context.Background(), path)
	if err != nil || probe != version {
		t.Fatalf("doctor version: %q, %v; expected %q", probe, err, version)
	}
}

func TestVersionNeverLaunchesWithoutProfile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell launcher fixture needs a POSIX shell")
	}
	path, err := Find("")
	if err != nil {
		testutil.RequireChrome(t, err)
	}
	work := t.TempDir()
	launcher := filepath.Join(work, "chrome-probe")
	profileFile := filepath.Join(work, "profile")
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'" }
	script := "#!/bin/sh\nprofile=\nfor arg do\n case \"$arg\" in\n --version) exit 71;;\n --user-data-dir=*) profile=${arg#*=};;\n esac\ndone\n[ -n \"$profile\" ] || exit 72\nprintf '%s' \"$profile\" > " + quote(profileFile) + "\nexec " + quote(path) + " \"$@\"\n"
	if err = os.WriteFile(launcher, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	version, err := Version(context.Background(), launcher)
	if err != nil || version == "" {
		t.Fatalf("version probe: %q, %v", version, err)
	}
	profile, err := os.ReadFile(profileFile)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(string(profile)); !os.IsNotExist(err) {
		t.Fatal("version probe left its isolated profile behind")
	}
}

func TestVisibleStartWithoutDisplay(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("chromedp checks for a display only on Linux")
	}
	t.Setenv("DISPLAY", "")
	t.Setenv("WAYLAND_DISPLAY", "")
	profiles := t.TempDir()
	t.Setenv("TMPDIR", profiles)
	err := failedStart(t, "chrome", true)
	want := "--login opens a visible Chrome window, but this session has no display. Run capture from a desktop session, or capture without --login."
	if err.Error() != want {
		t.Fatalf("no-display explanation: %v", err)
	}
	assertNoProfiles(t, profiles)
}

func TestHeadlessStartMissingExecutable(t *testing.T) {
	profiles := t.TempDir()
	for _, name := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(name, profiles)
	}
	failedStart(t, filepath.Join(t.TempDir(), "missing-chrome"), false)
	assertNoProfiles(t, profiles)
}

func failedStart(t *testing.T, path string, visible bool) error {
	t.Helper()
	done := make(chan error, 1)
	go func() {
		b, err := Start(context.Background(), path, visible)
		if b != nil {
			b.Close()
		}
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("unexpected successful browser startup")
		}
		return err
	case <-time.After(10 * time.Second):
		t.Fatal("failed browser startup did not return within 10 seconds")
		return nil
	}
}

func assertNoProfiles(t *testing.T, dir string) {
	t.Helper()
	profiles, err := filepath.Glob(filepath.Join(dir, "sealjar-chrome-*"))
	if err != nil || len(profiles) != 0 {
		t.Fatalf("temporary profiles survived failed startup: %v (%v)", profiles, err)
	}
}
