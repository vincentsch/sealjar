package browser

import (
	"context"
	"github.com/vincentsch/signetleaf/internal/testutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
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
