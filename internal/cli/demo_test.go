package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vincentsch/signetleaf/internal/browser"
	packagefmt "github.com/vincentsch/signetleaf/internal/pkg"
	"github.com/vincentsch/signetleaf/internal/tsa"
	"github.com/vincentsch/signetleaf/internal/tsa/demotsa"
	"github.com/vincentsch/signetleaf/internal/verify"
)

func TestDemoDryRunNeedsNoPrerequisitesOrFiles(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", filepath.Join(dir, "missing"))
	t.Setenv("TMPDIR", filepath.Join(dir, "missing"))
	out := filepath.Join(dir, "output")
	b, _ := invoke(t, 0, "demo", "--dry-run", "--json", "--out", out)
	var result demoResult
	oneJSON(t, b, &result)
	if !result.DryRun || result.Output != out || result.Package != "" {
		t.Fatal(string(b))
	}
	files, err := os.ReadDir(dir)
	if err != nil || len(files) != 0 {
		t.Fatalf("dry run wrote files: %v %v", files, err)
	}
}
func TestDemoFallbackJSONAndRepeatArtifacts(t *testing.T) {
	if _, err := browser.Find(""); err != nil {
		t.Skip(err)
	}
	old := tsa.Defaults
	tsa.Defaults = []string{"http://127.0.0.1:1", "http://127.0.0.1:2"}
	defer func() { tsa.Defaults = old }()
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	out := t.TempDir()
	var first demoResult
	for n := 0; n < 2; n++ {
		b, stderr := invoke(t, 0, "demo", "--out", out, "--json")
		var result demoResult
		oneJSON(t, b, &result)
		if len(stderr) != 0 || result.VerifyExitCode != 11 || result.Verification == nil || result.Verification.Files.State != "Unchanged" || result.Verification.Capture.State != "Complete" || result.Capture.Timestamp != demotsa.Label || !strings.Contains(result.Verification.Timestamp.Detail, demotsa.Label) {
			t.Fatalf("%s / %s", b, stderr)
		}
		r, err := packagefmt.Open(result.Package)
		if err != nil {
			t.Fatal(err)
		}
		if r.Manifest.Labels["client"] != "Bergblick Bakery (demo)" {
			t.Fatal(r.Manifest.Labels)
		}
		for _, target := range r.Manifest.Targets {
			if target.Kind == "repo" {
				facts, err := r.Read(target.Payload + "/git.json")
				if err != nil || !strings.Contains(string(facts), "may contain secrets") {
					t.Fatalf("%s %v", facts, err)
				}
			}
		}
		r.Close()
		seal, err := os.ReadFile(filepath.Join(result.ReaderFolder, "SEAL.html"))
		if err != nil || !strings.Contains(string(seal), demotsa.Label) {
			t.Fatalf("%s %v", seal, err)
		}
		checked, _ := invoke(t, 11, "verify", result.Package, "--json")
		var v verify.Result
		oneJSON(t, checked, &v)
		checked, _ = invoke(t, 10, "verify", result.TamperedPackage, "--json")
		oneJSON(t, checked, &v)
		if len(v.Files.Differences) != 1 || v.Files.Differences[0].Path != result.ChangedScreenshot {
			t.Fatalf("%+v", v)
		}
		root := filepath.Join(t.TempDir(), "demo-root.pem")
		write(t, root, demotsa.RootPEM)
		checked, _ = invoke(t, 0, "verify", result.Package, "--tsa-root", root, "--json")
		oneJSON(t, checked, &v)
		if !v.Tokens[0].Demo || !v.Tokens[0].Trusted || !strings.Contains(v.Timestamp.Detail, demotsa.Label) {
			t.Fatalf("%+v", v)
		}
		if n == 0 {
			first = result
		} else if result.Package == first.Package || result.ReaderFolder == first.ReaderFolder || result.TamperedPackage == first.TamperedPackage || !strings.HasSuffix(result.TamperedPackage, "-2.signetleaf.zip") {
			t.Fatalf("artifacts overwritten: %+v", result)
		}
		leftovers, err := os.ReadDir(tmp)
		if err != nil || len(leftovers) > 0 {
			t.Fatalf("temporary inputs remain: %v %v", leftovers, err)
		}
	}
}
