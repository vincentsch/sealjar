package report

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	packagefmt "github.com/vincentsch/sealjar/internal/pkg"
	"github.com/vincentsch/sealjar/internal/repo"
	"github.com/vincentsch/sealjar/internal/tsa"
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
		for i := 1; i <= 7; i++ {
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

func TestReportTitleAndNumbering(t *testing.T) {
	for _, labels := range []map[string]string{nil, {"client": "Bakery"}} {
		dir := t.TempDir()
		m := packagefmt.Manifest{CaptureID: "capture-id", Labels: labels}
		if err := Render(dir, "staging.example.com", "both", m); err != nil {
			t.Fatal(err)
		}
		for _, lang := range []string{"en", "de"} {
			raw, _ := os.ReadFile(filepath.Join(dir, "report/report."+lang+".html"))
			text := string(raw)
			for _, label := range map[string][]string{"en": {"Package", "Capture id", "Started", "Finished"}, "de": {"Paket", "Erfassungs-ID", "Beginn", "Ende"}}[lang] {
				if !strings.Contains(text, "<dt>"+label+"</dt>") {
					t.Fatal(label)
				}
			}
			label := map[string]string{"en": "Labels", "de": "Angaben"}[lang]
			if strings.Contains(text, "<dt>"+label+"</dt>") != (labels != nil) {
				t.Fatal("optional labels")
			}
			if strings.Contains(text, "<header><small>") || strings.Contains(text, "§ 8") || !strings.Contains(text, "<dd>staging.example.com</dd>") {
				t.Fatal("title")
			}
			for i := 1; i <= 7; i++ {
				if strings.Count(text, fmt.Sprintf("§ %d</small>", i)) != 1 {
					t.Fatal(i)
				}
			}
		}
	}
}

func TestSealTokenPresentation(t *testing.T) {
	for _, trusted := range []bool{false, true} {
		demo := tsa.Result{Demo: true, Trusted: trusted, Authority: "CN=raw-subject", Serial: "241", Time: "unavailable", Accuracy: "unavailable", Reason: "Untrusted certificate chain at genTime: raw diagnostic"}
		for _, mixed := range []bool{false, true} {
			tokens := []tsa.Result{demo}
			if mixed {
				tokens = append(tokens, tsa.Result{AuthorityName: "DigiCert Timestamp", Trusted: true})
			}
			raw, err := Seal(packagefmt.Manifest{}, nil, tokens, "test")
			if err != nil {
				t.Fatal(err)
			}
			text := string(raw)
			for _, lang := range []string{"en", "de"} {
				for _, want := range []string{tsa.Explanation(demo, lang), tsa.DemoProvenance(demo, lang), tsa.TimeLimitation(lang)} {
					if !strings.Contains(text, templateEscape(want)) {
						t.Fatal(want)
					}
				}
			}
			for _, want := range []string{"Sealjar Demo TSA", "0xF1", "Certificate details", "Zertifikatsdetails", "nicht verfügbar", "CN=raw-subject"} {
				if !strings.Contains(text, want) {
					t.Fatal(want)
				}
			}
			if trusted && strings.Contains(text, "Demo timestamp, not trusted") {
				t.Fatal("contradictory provenance")
			}
			if !trusted && (!strings.Contains(text, "Technical reason") || !strings.Contains(text, "Technische Angabe")) {
				t.Fatal("missing diagnostics")
			}
		}
	}
	raw, _ := Seal(packagefmt.Manifest{}, nil, []tsa.Result{{Invalid: true, Authority: "Unavailable", Time: "unavailable", Serial: "unavailable", Accuracy: "unavailable", Reason: "decode error"}}, "test")
	for _, want := range []string{"Authority unavailable", "Zeitstempelstelle nicht verfügbar", "nicht verfügbar", "damaged", "beschädigt"} {
		if !strings.Contains(string(raw), want) {
			t.Fatal(want)
		}
	}
}
func templateEscape(s string) string { return template.HTMLEscapeString(s) }

func TestReportExplainsConditionalTimeEvidence(t *testing.T) {
	for _, lang := range []string{"en", "de"} {
		dir := t.TempDir()
		if err := Render(dir, "capture", lang, packagefmt.Manifest{}); err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile(filepath.Join(dir, "report/report."+lang+".html"))
		if err != nil {
			t.Fatal(err)
		}
		text := strings.Join(strings.Fields(string(raw)), " ")
		sections := strings.Split(text, "<section>")
		for _, section := range []string{sections[1], sections[5]} {
			needs := map[string][]string{
				"en": {"independent trusted authority", "public demo key", "even when explicitly trusted", "SEAL.html", "sealjar verify"},
				"de": {"unabhängigen, vertrauenswürdigen Zeitstempelstelle", "öffentlichen Demo-Schlüssel", "auch bei ausdrücklich gewährtem Vertrauen", "SEAL.html", "sealjar verify"},
			}[lang]
			for _, need := range needs {
				if !strings.Contains(section, need) {
					t.Errorf("%s section lacks %q", lang, need)
				}
			}
		}
		for _, old := range []string{"files match the authenticated manifest", "Dateien mit dem authentifizierten Manifest", "A timestamp's time is an upper bound", "Die Zeit eines Zeitstempels ist eine Obergrenze"} {
			if strings.Contains(text, old) {
				t.Errorf("unconditional claim: %s", old)
			}
		}
		method := sections[5]
		for _, need := range map[string][]string{
			"en": {"Matching hashes show agreement with the manifest", "alone do not authenticate", "revocation"},
			"de": {"Passende Prüfsummen zeigen", "allein authentifiziert", "Zertifikatswiderrufe"},
		}[lang] {
			if !strings.Contains(method, need) {
				t.Errorf("%s method lacks %q", lang, need)
			}
		}
	}
}
