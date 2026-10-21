package web

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/chromedp/chromedp"
	"github.com/vincentsch/sealjar/internal/browser"
	packagefmt "github.com/vincentsch/sealjar/internal/pkg"
	"github.com/vincentsch/sealjar/internal/testsite"
	"github.com/vincentsch/sealjar/internal/testutil"
	site "github.com/vincentsch/sealjar/testdata/site"
	"image/png"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func chrome(t *testing.T) string {
	t.Helper()
	p, e := browser.Find("")
	if e != nil {
		testutil.RequireChrome(t, e)
	}
	return p
}
func runCapture(t *testing.T, urls []string, opt Options) (Result, string) {
	t.Helper()
	opt.Chrome = chrome(t)
	work := t.TempDir()
	positions := []int{}
	for i := range urls {
		positions = append(positions, i+1)
	}
	r, e := Capture(context.Background(), work, urls, positions, opt)
	if e != nil {
		t.Fatal(e)
	}
	return r, work
}
func read(t *testing.T, work, name string) string {
	t.Helper()
	b, e := os.ReadFile(filepath.Join(work, filepath.FromSlash(name)))
	if e != nil {
		t.Fatal(e)
	}
	return string(b)
}
func dimensions(t *testing.T, path string, wantWidth, wantMaxHeight int) {
	t.Helper()
	f, e := os.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	c, e := png.DecodeConfig(f)
	if e != nil {
		t.Fatal(e)
	}
	if c.Width != wantWidth || c.Height > wantMaxHeight {
		t.Fatalf("PNG dimensions: %dx%d", c.Width, c.Height)
	}
}
func TestBakeryCapture(t *testing.T) {
	s := httptest.NewServer(testsite.Handler())
	defer s.Close()
	urls := []string{s.URL + "/", s.URL + "/prices/", s.URL + "/qa/late/", s.URL + "/old-page"}
	r, work := runCapture(t, urls, Options{})
	if len(r.Targets) != 4 || len(r.Hosts) != 1 {
		t.Fatalf("results: %+v", r)
	}
	for _, target := range r.Targets {
		if target.Title == "" {
			t.Error("desktop page title missing")
		}
		if target.State != "complete" {
			t.Errorf("target: %+v", target)
		}
		dimensions(t, filepath.Join(work, target.Payload, "desktop.png"), 1440, 20000)
		dimensions(t, filepath.Join(work, target.Payload, "mobile.png"), 780, 40000)
		if !strings.HasPrefix(read(t, work, target.Payload+"/page.pdf"), "%PDF") {
			t.Fatal("PDF missing")
		}
	}
	late := read(t, work, r.Targets[2].Payload+"/dom.html")
	if !strings.Contains(late, "Fresh bread from 7 am.") {
		t.Fatal("late text absent")
	}
	f, e := os.Open(filepath.Join(work, r.Targets[2].Payload, "desktop.png"))
	if e != nil {
		t.Fatal(e)
	}
	img, e := png.Decode(f)
	f.Close()
	if e != nil {
		t.Fatal(e)
	}
	colorful := 0
	for y := img.Bounds().Dy() - 450; y < img.Bounds().Dy()-50; y += 5 {
		for x := 100; x < 700; x += 5 {
			red, green, blue, _ := img.At(x, y).RGBA()
			if red > green+2000 && green > blue+2000 {
				colorful++
			}
		}
	}
	if colorful < 100 {
		t.Fatal("far image was not decoded in the screenshot")
	}
	for _, target := range r.Targets {
		var network struct {
			Requests []request `json:"requests"`
		}
		if err := json.Unmarshal([]byte(read(t, work, target.Payload+"/network.json")), &network); err != nil {
			t.Fatal(err)
		}
		found := map[string]bool{}
		for _, req := range network.Requests {
			u, err := url.Parse(req.URL)
			if err != nil {
				t.Fatal(err)
			}
			if u.Path != "/assets/hero.jpg" && u.Path != "/assets/photo4.jpg" {
				continue
			}
			data, err := site.Files.ReadFile(strings.TrimPrefix(u.Path, "/"))
			if err != nil {
				t.Fatal(err)
			}
			if req.Status == nil || *req.Status != 200 || req.TransferredBytes < float64(len(data)) || req.Failure != "" {
				t.Errorf("incomplete asset evidence: %+v; asset size %d", req, len(data))
			}
			found[req.Viewport+u.Path] = true
		}
		if target.URL == urls[0] || target.URL == urls[3] {
			for _, viewport := range []string{"desktop", "mobile"} {
				for _, asset := range []string{"/assets/hero.jpg", "/assets/photo4.jpg"} {
					if !found[viewport+asset] {
						t.Errorf("missing completed %s request for %s", asset, viewport)
					}
				}
			}
		}
	}
	if !strings.Contains(r.Environment.ChromeVersion, "Chrome/") {
		t.Fatalf("CDP browser version missing: %q", r.Environment.ChromeVersion)
	}
	host := r.Hosts[0]
	var dns struct {
		Origin, Resolver string
		A                []string `json:"a"`
	}
	if e := json.Unmarshal([]byte(read(t, work, "hosts/"+host.OriginID+"/dns.json")), &dns); e != nil {
		t.Fatal(e)
	}
	if dns.Origin != host.Origin || dns.Resolver != "system" || len(dns.A) != 1 || dns.A[0] != "127.0.0.1" {
		t.Fatalf("DNS metadata: %+v", dns)
	}
	var tls struct{ Origin, Status string }
	if e := json.Unmarshal([]byte(read(t, work, "hosts/"+host.OriginID+"/tls.json")), &tls); e != nil {
		t.Fatal(e)
	}
	if tls.Origin != host.Origin || tls.Status != "Not applicable" {
		t.Fatalf("HTTP TLS metadata: %+v", tls)
	}

	redirects := read(t, work, r.Targets[3].Payload+"/redirects.json")
	if !strings.Contains(redirects, `"status": 301`) {
		t.Fatal(redirects)
	}
	headers := read(t, work, r.Targets[0].Payload+"/headers.json")
	if strings.Contains(headers, "bakery_notice=seen") {
		t.Fatal("cookie leaked")
	}
}
func TestWarningsClippingAndFailure(t *testing.T) {
	fixture := testsite.Handler()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/tall":
			fmt.Fprint(w, `<!doctype html><meta name="viewport" content="width=device-width"><title>Long page</title><div style="height:21000px">Tall</div><img src="/missing.png?ToKeN=hidden-value"><input type="password"><script>console.log('capture note',location.href);throw new Error('Fixture exception')</script>`)
		case "/clean":
			fmt.Fprint(w, `<!doctype html><title>Clean</title>Clean`)
		default:
			fixture.ServeHTTP(w, r)
		}
	}))
	defer s.Close()
	r, work := runCapture(t, []string{s.URL + "/tall?X-Amz-Signature=never-store", s.URL + "/clean", "http://127.0.0.1:1/"}, Options{Tablet: true})
	target := r.Targets[0]
	if target.State != "problems" || r.Targets[1].State != "complete" || r.Targets[2].State != "failed" {
		t.Fatalf("outcomes: %+v", r.Targets)
	}
	for _, v := range target.Viewports {
		if !v.Truncated || !v.PasswordForm || v.FailedRequests != 1 {
			t.Errorf("quality: %+v", v)
		}
	}
	dimensions(t, filepath.Join(work, target.Payload, "tablet.png"), 1536, 40000)
	b := read(t, work, target.Payload+"/network.json")
	if strings.Contains(b, "hidden-value") || strings.Contains(b, "never-store") {
		t.Fatal("secret URL leaked")
	}
	messages := read(t, work, target.Payload+"/console.json")
	if !strings.Contains(messages, "capture note") || !strings.Contains(messages, "Fixture exception") || strings.Contains(messages, "never-store") {
		t.Fatalf("console evidence missing or unmasked: %s", messages)
	}

}
func TestAuthenticationAndOriginFacts(t *testing.T) {
	var leaked atomic.Bool
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			leaked.Store(true)
		}
		fmt.Fprint(w, `<!doctype html><title>Other</title>Other`)
	}))
	defer other.Close()
	fixture := testsite.Handler()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/away" {
			http.Redirect(w, r, other.URL, 302)
			return
		}
		fixture.ServeHTTP(w, r)
	})
	s := httptest.NewUnstartedServer(handler)
	s.Config.ErrorLog = log.New(io.Discard, "", 0)
	s.StartTLS()
	defer s.Close()
	second := httptest.NewUnstartedServer(handler)
	second.Config.ErrorLog = log.New(io.Discard, "", 0)
	second.StartTLS()
	defer second.Close()
	r, work := runCapture(t, []string{s.URL + "/private/", s.URL + "/away", second.URL + "/"}, Options{Auth: "client:handover", TestBrowserOptions: []chromedp.ExecAllocatorOption{chromedp.IgnoreCertErrors}})
	if leaked.Load() {
		t.Fatal("basic auth crossed origin")
	}
	if r.Targets[0].State != "complete" {
		t.Fatalf("auth failed: %+v", r.Targets[0])
	}
	if len(r.Hosts) != 3 {
		t.Fatalf("origins: %+v", r.Hosts)
	}
	for _, h := range r.Hosts {
		facts := read(t, work, "hosts/"+h.OriginID+"/tls.json")
		if !strings.Contains(facts, h.Origin) {
			t.Fatal("origin not retained")
		}
		if strings.HasPrefix(h.Origin, "https:") {
			if !strings.Contains(facts, `"system_roots_trusted": false`) {
				t.Fatal(facts)
			}
			_ = read(t, work, "hosts/"+h.OriginID+"/tls.pem")
		}
	}
	now := time.Now().UTC()
	m := packagefmt.Manifest{Type: packagefmt.ManifestType, ToolVersion: "test", CaptureID: "capture", Started: now, Ended: now, TimeZone: "UTC", Targets: r.Targets, Hosts: r.Hosts}
	if e := packagefmt.BuildManifest(work, &m); e != nil {
		t.Fatal(e)
	}
	packed, e := packagefmt.Write(work, t.TempDir(), "hosts", now)
	if e != nil {
		t.Fatal(e)
	}
	reader, e := packagefmt.Open(packed)
	if e != nil {
		t.Fatal(e)
	}
	defer reader.Close()
	if len(reader.Manifest.Hosts) != 3 {
		t.Fatal("origin entries lost during packaging")
	}
	for _, host := range reader.Manifest.Hosts {
		b, e := reader.Read("hosts/" + host.OriginID + "/tls.json")
		if e != nil || !strings.Contains(string(b), host.Origin) {
			t.Fatalf("origin facts lost in package: %v", e)
		}
	}
	for _, target := range r.Targets {
		for _, name := range []string{"network.json", "headers.json", "console.json", "redirects.json"} {
			b := read(t, work, target.Payload+"/"+name)
			if strings.Contains(b, "handover") || strings.Contains(b, "Y2xpZW50OmhhbmRvdmVy") {
				t.Fatal("credential leaked")
			}
		}
	}
	unauth, _ := runCapture(t, []string{other.URL + "/", s.URL + "/private/"}, Options{TestBrowserOptions: []chromedp.ExecAllocatorOption{chromedp.IgnoreCertErrors}})
	if unauth.Targets[1].State != "problems" || unauth.Targets[1].Viewports[0].DocumentStatus != 401 {
		t.Fatalf("401 outcome: %+v", unauth)
	}
}
func TestManualLoginKeepsSession(t *testing.T) {
	s := httptest.NewServer(testsite.Handler())
	defer s.Close()
	r, work := runCapture(t, []string{s.URL + "/account/"}, Options{Login: false, LoginWait: nil})
	if !r.Targets[0].Viewports[0].PasswordForm {
		t.Fatal("login form not detected")
	}
	// Reuse a headless browser through the same supplied-browser path as the CLI.
	b, err := browser.Start(context.Background(), chrome(t), false)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	r, work = runCapture(t, []string{s.URL + "/account/"}, Options{Browser: b, Login: true, LoginWait: func(ctx context.Context) error {
		if e := chromedp.Do(ctx, chromedp.WaitVisible(chromedp.CSS(`input[name=user]`)), chromedp.SetValue(chromedp.CSS(`input[name=user]`), "client"), chromedp.SetValue(chromedp.CSS(`input[name=password]`), "handover"), chromedp.Click(chromedp.CSS(`button[type=submit]`)), chromedp.WaitVisible(chromedp.CSS(`#state`))); e != nil {
			return e
		}
		return nil
	}})
	if !strings.Contains(read(t, work, r.Targets[0].Payload+"/dom.html"), "Logged in. 2 open orders.") {
		t.Fatal("login state lost")
	}
	if r.Targets[0].State != "complete" {
		t.Fatalf("login outcome: %+v", r)
	}
}
func TestStalledNavigationAndResourceStayBounded(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/stall", "/never":
			<-r.Context().Done()
		case "/resource":
			fmt.Fprint(w, `<!doctype html><title>Available</title><p>Available evidence</p><img src="/stall">`)
		default:
			fmt.Fprint(w, `<!doctype html><title>Next</title>Next`)
		}
	}))
	defer s.Close()
	// Two widths each consume one 30-second navigation budget. A stalled asset
	// can also consume its separate ten-second post-scroll idle budget.
	for _, route := range []string{"/resource", "/never"} {
		t.Run(route, func(t *testing.T) {
			start := time.Now()
			r, _ := runCapture(t, []string{s.URL + route, s.URL + "/next"}, Options{})
			if time.Since(start) > 100*time.Second {
				t.Fatal("navigation budgets were not bounded")
			}
			if r.Targets[1].State != "complete" {
				t.Fatal("later target was not captured")
			}
			if route == "/never" {
				if r.Targets[0].State != "failed" {
					t.Fatalf("blank document counted: %+v", r.Targets[0])
				}
			} else {
				v := r.Targets[0].Viewports[0]
				if v.State != "problems" || len(v.Timeouts) == 0 {
					t.Fatalf("timeout evidence lost: %+v", v)
				}
			}
		})
	}
}
func TestOriginAndPrivacy(t *testing.T) {
	for raw, want := range map[string]string{"https://EXAMPLE.com/a": "https://example.com:443", "https://example.com:8443/": "https://example.com:8443", "http://[::1]/": "http://[::1]:80"} {
		o, e := Origin(raw)
		if e != nil || o != want || len(OriginID(o)) != 64 {
			t.Fatalf("origin %q: %q %v", raw, o, e)
		}
	}
	raw := "https://user:pass@example.com/?ToKeN=abc&X-Amz-Credential=xyz&keep=yes"
	got := MaskURL(raw)
	if strings.Contains(got, "abc") || strings.Contains(got, "xyz") || strings.Contains(got, "pass") || !strings.Contains(got, "keep=yes") {
		t.Fatal(got)
	}
	b, _ := json.Marshal(maskHeaders(map[string]any{"Authorization": "Basic value", "Cookie": "session=abc", "Location": raw, "Content-Location": "/next?sid=abc&X-Amz-Signature=xyz"}, ""))
	if strings.Contains(string(b), "abc") || strings.Contains(string(b), "xyz") {
		t.Fatal(string(b))
	}
}

func TestEventStreamDoesNotBlockIdle(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/events" {
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, "data: ready\n\n")
			w.(http.Flusher).Flush()
			<-r.Context().Done()
			return
		}
		fmt.Fprint(w, `<!doctype html><title>Live updates</title><p>Ready</p><script>new EventSource('/events')</script>`)
	}))
	defer s.Close()
	start := time.Now()
	result, _ := runCapture(t, []string{s.URL}, Options{})
	if result.Targets[0].State != "complete" {
		t.Fatalf("stream kept idle busy: %+v", result.Targets[0])
	}
	if time.Since(start) > 15*time.Second {
		t.Fatal("event stream delayed capture")
	}
}
