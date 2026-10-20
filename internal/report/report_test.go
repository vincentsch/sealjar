package report

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	packagefmt "github.com/vincentsch/signetleaf/internal/pkg"
	"github.com/vincentsch/signetleaf/internal/repo"
	"github.com/vincentsch/signetleaf/internal/tsa"
)

func TestBilingualReportsAndThumbnails(t *testing.T) {
	dir := t.TempDir()
	write := func(n string, v any) {
		t.Helper()
		raw, e := json.Marshal(v)
		if e != nil {
			t.Fatal(e)
		}
		p := filepath.Join(dir, n)
		if e = os.MkdirAll(filepath.Dir(p), 0700); e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(p, raw, 0600); e != nil {
			t.Fatal(e)
		}
	}
	m := packagefmt.Manifest{Type: packagefmt.ManifestType, ToolVersion: "test", CaptureID: "capture", Started: time.Now(), Ended: time.Now(), TimeZone: "Europe/Berlin", Settings: packagefmt.Settings{BasicAuth: true, ManualLogin: true}, Targets: []packagefmt.Target{{Kind: "web", Name: "Page", URL: "http://localhost:8000", FinalURL: "http://localhost:8000/final", State: "problems", Payload: "web/page", Notes: []string{"asset missing"}, Viewports: []packagefmt.Viewport{{Name: "desktop", Width: 1440, Height: 900, Scale: 1, DocumentStatus: 200}}}, {Kind: "repo", Name: "site", Commit: "abcdef", State: "complete", Payload: "repo/site"}}, Hosts: []packagefmt.Host{{Origin: "http://localhost:8000", OriginID: "one"}, {Origin: "http://localhost:8001", OriginID: "two"}}}
	write("web/page/redirects.json", []any{})
	write("repo/site/git.json", repo.Facts{Name: "site", History: true, Excluded: []repo.Excluded{{Path: ".env", Reason: "may contain secrets"}}})
	write("repo/site/files.json", []repo.File{{Path: "index.html", Size: 5, LinkTarget: "target", PointerOnly: true}})
	for _, h := range m.Hosts {
		write("hosts/"+h.OriginID+"/dns.json", map[string]any{"a": []string{"127.0.0.1"}})
		write("hosts/"+h.OriginID+"/tls.json", map[string]any{"status": "Not applicable"})
	}
	img := image.NewRGBA(image.Rect(0, 0, 1440, 900))
	img.Set(0, 0, color.White)
	f, e := os.Create(filepath.Join(dir, "web/page/desktop.png"))
	if e != nil {
		t.Fatal(e)
	}
	if e = png.Encode(f, img); e != nil {
		t.Fatal(e)
	}
	f.Close()
	if e = Thumbnails(dir); e != nil {
		t.Fatal(e)
	}
	f, e = os.Open(filepath.Join(dir, "web/page/desktop.thumb.jpg"))
	if e != nil {
		t.Fatal(e)
	}
	cfg, e := jpeg.DecodeConfig(f)
	f.Close()
	if e != nil || cfg.Width != 600 || cfg.Height != 375 {
		t.Fatalf("thumbnail: %+v %v", cfg, e)
	}
	if e = packagefmt.CollectFiles(dir, &m); e != nil {
		t.Fatal(e)
	}
	if e = Render(dir, "handover", "both", m); e != nil {
		t.Fatal(e)
	}
	for _, lang := range Languages("both") {
		b, e := os.ReadFile(filepath.Join(dir, "report/report."+lang+".html"))
		if e != nil {
			t.Fatal(e)
		}
		for _, need := range []string{"http://localhost:8000", "http://localhost:8001", "../web/page/desktop.thumb.jpg", "../web/page/desktop.png", "-attime GEN_TIME_UNIX_SECONDS", "openssl ts -reply", "datetime.strptime", ".env", "abcdef", "asset missing"} {
			if !bytes.Contains(b, []byte(need)) {
				t.Errorf("%s lacks %s", lang, need)
			}
		}
		if lang == "de" && !bytes.Contains(b, []byte("kann geheime Daten enthalten")) {
			t.Fatal("exclusion reason not localized")
		}
		if bytes.Contains(b, []byte("<script")) || bytes.Contains(b, []byte("src=\"https://")) {
			t.Fatal("external resource")
		}
		for i := 1; i <= 8; i++ {
			if !strings.Contains(string(b), "§ "+string(rune('0'+i))) {
				t.Errorf("missing section %d", i)
			}
		}
	}
}
func TestSealStatesAndClaims(t *testing.T) {
	for _, tokens := range [][]tsa.Result{nil, {{Authority: "Test authority", Trusted: false}}, {{Authority: "Test authority", Trusted: true}}} {
		b, e := Seal(packagefmt.Manifest{}, []byte("manifest"), tokens, "handover")
		if e != nil {
			t.Fatal(e)
		}
		if !bytes.Contains(b, []byte(packagefmt.Hash([]byte("manifest")))) {
			t.Fatal("missing hash")
		}
		if len(tokens) == 0 && !bytes.Contains(b, []byte("Not timestamped")) {
			t.Fatal("missing state")
		}
	}
	for _, text := range []string{reportHTML, sealHTML, string(packagefmt.Instructions)} {
		for _, word := range []string{"court-proof", "gerichtsfest", "rechtssicher", "beweissicher", "legally binding"} {
			if strings.Contains(strings.ToLower(text), word) {
				t.Fatalf("forbidden claim %s", word)
			}
		}
	}
}

func TestLocalReportURL(t *testing.T) {
	for p, want := range map[string]string{"/tmp/reader copy/report.html": "file:///tmp/reader%20copy/report.html", "C:/reader copy/report.html": "file:///C:/reader%20copy/report.html"} {
		if got := fileURL(p); got != want {
			t.Fatalf("local report URL: %s, wanted %s", got, want)
		}
	}
}
