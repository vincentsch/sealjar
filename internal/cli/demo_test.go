package cli

import (
	"bytes"
	"github.com/vincentsch/signetleaf/internal/testutil"
	"github.com/vincentsch/signetleaf/internal/tsa/testtsa"
	"math/big"
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
		testutil.RequireChrome(t, err)
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
		if !v.Tokens[0].Demo || !v.Tokens[0].Trusted || !strings.Contains(v.Timestamp.Detail, "public test key") || !strings.Contains(v.Timestamp.Detail, "supplied root") {
			t.Fatalf("%+v", v)
		}
		if n == 0 {
			serial := result.Capture.Attempts[len(result.Capture.Attempts)-1].Token.Serial
			if _, ok := new(big.Int).SetString(serial, 10); !ok {
				t.Fatalf("serial is not decimal: %s", serial)
			}
			endpoint, independentRoot := localTSA(t, testtsa.Options{})
			mixed := stamp(t, result.Package, "--tsa", endpoint, "--tsa-root", independentRoot)
			mixedResult := assertVerify(t, 0, mixed, "--tsa-root", independentRoot)
			if len(mixedResult.Tokens) != 2 {
				t.Fatal(mixedResult)
			}
			for _, token := range mixedResult.Tokens {
				if token.Demo && token.Trusted {
					t.Fatal("independent root trusted demo")
				}
			}
			var text bytes.Buffer
			verify.Render(&text, mixedResult)
			for _, want := range []string{demotsa.Label, "built-in test authority", "independent trusted authority", "public demo key"} {
				if !strings.Contains(text.String(), want) {
					t.Fatal(text.String())
				}
			}
			for _, limitation := range mixedResult.Limitations {
				if strings.Contains(limitation, "demo") || strings.Contains(limitation, "independent trusted authority") {
					t.Fatal("JSON limitation changed", limitation)
				}
			}
			text.Reset()
			verify.Render(&text, v)
			if strings.Contains(text.String(), demotsa.Label) || !strings.Contains(text.String(), "supplied root") || !strings.Contains(text.String(), "independent trusted authority") {
				t.Fatal(text.String())
			}
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
