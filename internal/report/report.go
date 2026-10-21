// Package report renders local reading copies from capture facts.
package report

import (
	"bytes"
	"context"
	"crypto/x509"
	_ "embed"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"html/template"
	"image"
	"image/jpeg"
	_ "image/png"
	"math/big"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/vincentsch/sealjar/internal/browser"
	packagefmt "github.com/vincentsch/sealjar/internal/pkg"
	"github.com/vincentsch/sealjar/internal/repo"
	"github.com/vincentsch/sealjar/internal/tsa"
	"golang.org/x/image/draw"
)

//go:embed style.css
var css string

//go:embed report.html
var reportHTML string

//go:embed seal.html
var sealHTML string

func Languages(lang string) []string {
	if lang == "both" {
		return []string{"en", "de"}
	}
	return []string{lang}
}
func ValidLanguage(lang string) bool { return lang == "en" || lang == "de" || lang == "both" }
func execute(source string, data any) ([]byte, error) {
	t, e := template.New("page").Funcs(template.FuncMap{"css": func() template.CSS { return template.CSS(css) }, "pretty": func(v any) string { b, _ := json.MarshalIndent(v, "", "  "); return string(b) }, "thumb": func(s string) string { return strings.TrimSuffix(s, ".png") + ".thumb.jpg" }, "timefmt": formatTime, "tokenTime": func(s, zone, lang string) string {
		t, e := time.Parse(time.RFC3339Nano, s)
		if e != nil {
			return unavailable(lang)
		}
		return formatTime(t, zone)
	}, "groupHash": func(s string) string {
		parts := []string{}
		for len(s) >= 8 {
			parts = append(parts, s[:8])
			s = s[8:]
		}
		return strings.Join(parts, " ")
	}, "status": status, "factLabel": factLabel, "factValue": factValue, "tokenReason": tsa.Explanation, "provenance": tsa.DemoProvenance, "timeLimit": tsa.TimeLimitation, "authority": func(t tsa.Result, lang string) string {
		s := t.DisplayAuthority()
		if s == "Authority unavailable" && lang == "de" {
			return "Zeitstempelstelle nicht verfügbar"
		}
		return s
	}, "serial": func(s, lang string) string {
		if n, ok := new(big.Int).SetString(s, 10); ok {
			return "0x" + strings.ToUpper(n.Text(16))
		}
		return unavailable(lang)
	}, "accuracy": accuracy, "localText": localText}).Parse(source)
	if e != nil {
		return nil, e
	}
	var b bytes.Buffer
	e = t.Execute(&b, data)
	return b.Bytes(), e
}
func formatTime(t time.Time, zone string) string {
	loc, e := time.LoadLocation(zone)
	if e != nil {
		loc = time.UTC
	}
	return t.In(loc).Format("2006-01-02 15:04:05 MST -07:00") + " / " + t.UTC().Format("2006-01-02 15:04:05 UTC")
}
func Thumbnails(dir string) error {
	return filepath.WalkDir(dir, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() || !strings.HasSuffix(p, ".png") {
			return nil
		}
		f, e := os.Open(p)
		if e != nil {
			return e
		}
		src, _, e := image.Decode(f)
		f.Close()
		if e != nil {
			return e
		}
		bounds := src.Bounds()
		w := 600
		h := bounds.Dy() * w / bounds.Dx()
		if h < 1 {
			h = 1
		}
		dst := image.NewRGBA(image.Rect(0, 0, w, h))
		draw.CatmullRom.Scale(dst, dst.Bounds(), src, bounds, draw.Src, nil)
		out, e := os.Create(strings.TrimSuffix(p, ".png") + ".thumb.jpg")
		if e != nil {
			return e
		}
		e = jpeg.Encode(out, dst, &jpeg.Options{Quality: 85})
		ce := out.Close()
		if e != nil {
			return e
		}
		return ce
	})
}

type figure struct {
	Path    string
	Number  int
	Caption string
}

type detail struct {
	Title string
	Facts any
}
type reportData struct {
	Lang, Name   string
	M            packagefmt.Manifest
	Sections     []string
	Details      map[string][]detail
	Images       map[string][]figure
	Instructions string
	Repositories []repository
}

func Render(dir, name, lang string, m packagefmt.Manifest) error {
	for _, l := range Languages(lang) {
		sections := []string{"Capture report", "Summary", "Pages", "Hosts", "Code", "Method", "Checking this package", "File list"}
		if l == "de" {
			sections = []string{"Erfassungsbericht", "Zusammenfassung", "Seiten", "Hosts", "Code", "Methode", "Paket prüfen", "Dateiliste"}
		}
		d := reportData{Lang: l, Name: name, M: m, Sections: sections, Details: map[string][]detail{}, Images: map[string][]figure{}, Instructions: instructions(l)}
		figureNumber := 1
		for _, t := range m.Targets {
			if t.Kind == "repo" {
				r := repository{Target: t}
				raw, e := os.ReadFile(filepath.Join(dir, filepath.FromSlash(t.Payload), "git.json"))
				if e != nil {
					return e
				}
				if e = json.Unmarshal(raw, &r.Facts); e != nil {
					return e
				}
				raw, e = os.ReadFile(filepath.Join(dir, filepath.FromSlash(t.Payload), "files.json"))
				if e != nil {
					return e
				}
				if e = json.Unmarshal(raw, &r.Files); e != nil {
					return e
				}
				for _, f := range r.Files {
					r.Size += f.Size
				}
				d.Repositories = append(d.Repositories, r)
				continue
			}
			files := []string{"redirects.json"}
			for _, n := range files {
				b, e := os.ReadFile(filepath.Join(dir, filepath.FromSlash(t.Payload), n))
				if os.IsNotExist(e) && t.State == "failed" {
					continue
				}
				if e != nil {
					return e
				}
				var v any
				if e = json.Unmarshal(b, &v); e != nil {
					return e
				}
				d.Details[t.Payload] = append(d.Details[t.Payload], detail{n, v})
			}
			for _, v := range t.Viewports {
				n := t.Payload + "/" + v.Name + ".png"
				if _, e := os.Stat(filepath.Join(dir, filepath.FromSlash(n))); e == nil {
					d.Images[t.Payload] = append(d.Images[t.Payload], figure{Path: n, Number: figureNumber, Caption: fmt.Sprintf("%s · %s · %d px", t.URL, v.Name, v.Width)})
					figureNumber++
				}
			}
		}
		for _, h := range m.Hosts {
			if e := packagefmt.SafePath("hosts/" + h.OriginID); e != nil {
				return e
			}
			for _, n := range []string{"dns.json", "tls.json"} {
				b, e := os.ReadFile(filepath.Join(dir, "hosts", h.OriginID, n))
				if e != nil {
					return e
				}
				var v any
				if e = json.Unmarshal(b, &v); e != nil {
					return e
				}
				if n == "tls.json" {
					if raw, e := os.ReadFile(filepath.Join(dir, "hosts", h.OriginID, "tls.pem")); e == nil {
						block, _ := pem.Decode(raw)
						if block != nil {
							if cert, e := x509.ParseCertificate(block.Bytes); e == nil {
								facts := v.(map[string]any)
								facts["issuer"] = cert.Issuer.String()
								facts["subject"] = cert.Subject.String()
							}
						}
					}
				}
				d.Details[h.OriginID] = append(d.Details[h.OriginID], detail{n, v})
			}
		}
		b, e := execute(reportHTML, d)
		if e != nil {
			return e
		}
		if e = os.MkdirAll(filepath.Join(dir, "report"), 0700); e != nil {
			return e
		}
		if e = os.WriteFile(filepath.Join(dir, "report", "report."+l+".html"), b, 0600); e != nil {
			return e
		}
	}
	return nil
}
func PDFs(b *browser.Browser, dir, lang string) error {
	// A fresh tab has none of the capture tab's auth handlers or mobile emulation.
	tc, cancel := chromedp.NewContext(b.Context)
	defer cancel()
	tc, timeout := context.WithTimeout(tc, 60*time.Second)
	defer timeout()
	for _, l := range Languages(lang) {
		p, e := filepath.Abs(filepath.Join(dir, "report", "report."+l+".html"))
		if e != nil {
			return e
		}
		if e = chromedp.Do(tc, chromedp.Navigate(fileURL(p))); e != nil {
			return e
		}
		ready, e := chromedp.Run(tc, chromedp.Evaluate[bool]("Array.from(document.images).every(i => i.complete && i.naturalWidth > 0)"))
		if e != nil {
			return e
		}
		if !ready {
			return fmt.Errorf("report images did not load")
		}
		// Chrome resolves relative PDF annotations to this disposable local folder.
		// Remove links only in the print tab; the stored HTML keeps its local links.
		if _, e = chromedp.Run(tc, chromedp.Evaluate[any](`document.querySelectorAll('a[href]').forEach(a => a.removeAttribute('href'))`)); e != nil {
			return e
		}
		yes := true
		pdf, e := cdp.Call(tc, chromedp.FromContext(tc).Target, page.PrintToPDF, page.PrintToPDFParams{PrintBackground: &yes, PreferCSSPageSize: &yes, PaperWidth: 8.27, PaperHeight: 11.69})
		if e != nil {
			return e
		}
		if e = os.WriteFile(strings.TrimSuffix(p, ".html")+".pdf", pdf.Data, 0600); e != nil {
			return e
		}
	}
	return nil
}
func Seal(m packagefmt.Manifest, manifest []byte, tokens []tsa.Result, name string) ([]byte, error) {
	return execute(sealHTML, struct {
		M          packagefmt.Manifest
		Hash, Name string
		Tokens     []tsa.Result
	}{m, packagefmt.Hash(manifest), name, tokens})
}
func SealDirectory(dir, name string, roots *x509.CertPool) error {
	raw, e := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if e != nil {
		return e
	}
	var m packagefmt.Manifest
	if e = json.Unmarshal(raw, &m); e != nil {
		return e
	}
	tokens := []tsa.Result{}
	entries, e := os.ReadDir(dir)
	if e != nil {
		return e
	}
	for _, entry := range entries {
		n := entry.Name()
		if strings.HasSuffix(n, ".tsr") && packagefmt.Exempt(n) {
			b, e := os.ReadFile(filepath.Join(dir, n))
			if e != nil {
				return e
			}
			t := tsa.Check(b, raw, roots)
			t.Path = n
			tokens = append(tokens, t)
		}
	}
	b, e := Seal(m, raw, tokens, name)
	if e != nil {
		return e
	}
	return os.WriteFile(filepath.Join(dir, "SEAL.html"), b, 0600)
}

type repository struct {
	Target packagefmt.Target
	Facts  repo.Facts
	Files  []repo.File
	Size   int64
}

func instructions(lang string) string {
	text := string(packagefmt.Instructions)
	en, rest, _ := strings.Cut(text, "Deutsch\n")
	de, python, _ := strings.Cut(rest, "Python 3 file check / Dateiprüfung mit Python 3")

	if lang == "de" {
		_, commands, _ := strings.Cut(en, "```sh\n")
		commands, _, _ = strings.Cut(commands, "```")
		de = strings.ReplaceAll(de, "obigen OpenSSL-Befehle", "folgenden OpenSSL-Befehle")
		return de + "\nUTC-Zeit des Tokens anzeigen:\n\n    openssl ts -reply -in token.tsr -text\n\nDie angezeigte Zeit ohne die Bezeichnung Time stamp eintragen und umrechnen:\n\n" + commands + "\nDiese Unix-Sekunden ersetzen GEN_TIME_UNIX_SECONDS:\n\n    openssl ts -verify -data manifest.json -in token.tsr -CAfile roots.pem -attime GEN_TIME_UNIX_SECONDS\n\nDateiprüfung mit Python 3\n" + python
	}
	return en + "\nPython 3 file check\n" + python
}

func status(s, lang string) string {
	if lang == "de" {
		switch s {
		case "complete":
			return "vollständig"
		case "problems":
			return "mit Problemen"
		case "failed":
			return "fehlgeschlagen"
		}
	}
	return s
}

func factLabel(key, lang string) string {
	en := map[string]string{"a": "IPv4 (A)", "aaaa": "IPv6 (AAAA)", "cname": "Canonical name (CNAME)", "resolver": "DNS resolver", "origin": "Origin", "status": "TLS status", "version": "TLS protocol", "cipher": "Cipher", "issuer": "Certificate issuer", "subject": "Certificate subject", "not_before": "Valid from", "not_after": "Valid until", "system_roots_trusted": "Trusted by system roots", "verification_error": "Certificate check", "address_error": "Address lookup", "cname_error": "CNAME lookup", "error": "Problem"}
	de := map[string]string{"a": "IPv4 (A)", "aaaa": "IPv6 (AAAA)", "cname": "Kanonischer Name (CNAME)", "resolver": "DNS-Auflösung", "origin": "Ursprung", "status": "TLS-Status", "version": "TLS-Protokoll", "cipher": "Verschlüsselung", "issuer": "Zertifikatsaussteller", "subject": "Zertifikatsinhaber", "not_before": "Gültig ab", "not_after": "Gültig bis", "system_roots_trusted": "Vertrauen durch Systemzertifikate", "verification_error": "Zertifikatsprüfung", "address_error": "Adressabfrage", "cname_error": "CNAME-Abfrage", "error": "Problem"}
	labels := en
	if lang == "de" {
		labels = de
	}
	if s, ok := labels[key]; ok {
		return s
	}
	return key
}
func factValue(v any, lang string) string {
	switch val := v.(type) {
	case string:
		if lang == "de" {
			switch val {
			case "Not applicable":
				return "nicht zutreffend"
			case "Unavailable":
				return "nicht verfügbar"
			case "Collected":
				return "erfasst"
			case "system":
				return "Betriebssystem"
			}
		}
		return val
	case bool:
		if lang == "de" {
			if val {
				return "ja"
			}
			return "nein"
		}
		if val {
			return "yes"
		}
		return "no"
	case []any:
		parts := []string{}
		for _, item := range val {
			parts = append(parts, fmt.Sprint(item))
		}
		return strings.Join(parts, ", ")
	}
	return fmt.Sprint(v)
}

func fileURL(p string) string {
	p = filepath.ToSlash(p)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return (&url.URL{Scheme: "file", Path: p}).String()
}

func accuracy(s, lang string) string {
	if lang == "de" {
		switch s {
		case "unspecified":
			return "nicht angegeben"
		case "unavailable":
			return "nicht verfügbar"
		}
	}
	return s
}

func localText(s, lang string) string {
	if lang != "de" {
		return s
	}
	return strings.NewReplacer(
		"may contain secrets", "kann geheime Daten enthalten",
		"excluded by pattern ", "ausgeschlossen durch Muster ",
		"nested Git repository; contents not included", "verschachteltes Git-Repository; Inhalt nicht enthalten",
		"parent is a symbolic link or is not a directory", "übergeordneter Pfad ist eine Verknüpfung oder kein Verzeichnis",
		"link points outside the repository", "Verknüpfung verweist nach außerhalb des Repositorys",
		"The viewport could not be captured.", "Diese Ansicht konnte nicht erfasst werden.",
		"Some viewport evidence could not be saved.", "Ein Teil der erfassten Ansicht konnte nicht gespeichert werden.",
		"no target document was captured", "kein Zieldokument erfasst",
		"No target document became available within the navigation budget", "Innerhalb des Zeitlimits wurde kein Zieldokument verfügbar",
		"Navigation failed:", "Seitenaufruf fehlgeschlagen:",
		"Document returned status ", "Dokument lieferte HTTP-Status ",
		"failed requests", "fehlgeschlagene Anfragen",
		"Mixed content", "Gemischte HTTP- und HTTPS-Inhalte",
		"Content exceeds 20,000 CSS px", "Inhalt überschreitet 20.000 CSS-Pixel",
		"Page shows a login form", "Seite zeigt ein Anmeldeformular",
		"Wait limit reached:", "Zeitlimit erreicht:",
		"load and network idle (30s)", "Laden und Netzwerkruhe (30 Sekunden)",
		"post-scroll network idle (10s)", "Netzwerkruhe nach dem Scrollen (10 Sekunden)",
	).Replace(s)
}

func unavailable(lang string) string {
	if lang == "de" {
		return "nicht verfügbar"
	}
	return "unavailable"
}
