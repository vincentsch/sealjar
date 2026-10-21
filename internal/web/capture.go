package web

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/fetch"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	js "github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"github.com/vincentsch/sealjar/internal/browser"
	packagefmt "github.com/vincentsch/sealjar/internal/pkg"
)

type Options struct {
	Browser  *browser.Browser
	Progress func(string)
	Chrome   string
	Tablet   bool
	Wait     time.Duration
	Auth     string
	Login    bool
	// LoginWait also lets tests drive the login form in the capture tab.
	LoginWait          func(context.Context) error
	TestBrowserOptions []chromedp.ExecAllocatorOption
}
type Result struct {
	Targets     []packagefmt.Target
	Hosts       []packagefmt.Host
	Environment packagefmt.Environment
}
type request struct {
	Viewport         string  `json:"viewport"`
	Method           string  `json:"method"`
	URL              string  `json:"url"`
	Status           *int64  `json:"status"`
	ResourceType     string  `json:"resource_type"`
	MIMEType         string  `json:"mime_type"`
	TransferredBytes float64 `json:"transferred_bytes"`
	Failure          string  `json:"failure,omitempty"`
}
type headerRecord struct {
	Viewport string            `json:"viewport"`
	URL      string            `json:"url"`
	Request  map[string]string `json:"request"`
	Response map[string]string `json:"response"`
}
type redirectRecord struct {
	Viewport string `json:"viewport"`
	From     string `json:"from"`
	To       string `json:"to"`
	Status   int64  `json:"status"`
}
type consoleRecord struct {
	Viewport string `json:"viewport"`
	Kind     string `json:"kind"`
	Text     string `json:"text"`
}
type completion struct {
	at      cdp.MonotonicTime
	bytes   float64
	failure string
}
type recorder struct {
	title     string
	ended     map[network.RequestID]completion
	responses map[network.RequestID]*network.Response
	ignored   map[network.RequestID]bool

	sync.Mutex
	viewport                 string
	auth                     string
	main                     cdp.FrameID
	requests                 []*request
	active                   map[network.RequestID]bool
	indices                  map[network.RequestID]int
	headers                  []headerRecord
	redirects                []redirectRecord
	console                  []consoleRecord
	origins                  map[string]bool
	available, loaded, mixed bool
	status                   int64
	last                     time.Time
	final                    string
	stops                    []func()
	wg                       sync.WaitGroup
}

func subscribe[E any](ctx context.Context, t *chromedp.Target, r *recorder, method string, handle func(E)) {
	ch, stop := t.Subscribe(method)
	r.stops = append(r.stops, stop)
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case raw, ok := <-ch:
				if !ok {
					return
				}
				var e E
				if err := json.Unmarshal(raw, &e); err == nil {
					r.Lock()
					handle(e)
					r.Unlock()
				}
			}
		}
	}()
}
func (r *recorder) stop() {
	for _, s := range r.stops {
		s()
	}
	r.wg.Wait()
}
func record(ctx context.Context, t *chromedp.Target, viewport, auth string, main cdp.FrameID) *recorder {
	r := &recorder{viewport: viewport, auth: auth, main: main, active: map[network.RequestID]bool{}, indices: map[network.RequestID]int{}, ended: map[network.RequestID]completion{}, responses: map[network.RequestID]*network.Response{}, ignored: map[network.RequestID]bool{}, origins: map[string]bool{}, last: time.Now()}
	subscribe(ctx, t, r, "Network.requestWillBeSent", func(e network.EventRequestWillBeSent) {
		if e.RedirectResponse != nil {
			if i, ok := r.indices[e.RequestID]; ok {
				s := e.RedirectResponse.Status
				r.requests[i].Status = &s
				r.requests[i].TransferredBytes = e.RedirectResponse.EncodedDataLength
			}
			if e.FrameID == main && e.Type == network.ResourceTypeDocument {
				r.redirects = append(r.redirects, redirectRecord{viewport, MaskURL(e.RedirectResponse.URL), MaskURL(e.Request.URL), e.RedirectResponse.Status})
				r.headers = append(r.headers, headerRecord{viewport, MaskURL(e.RedirectResponse.URL), maskHeaders(e.RedirectResponse.RequestHeaders, auth), maskHeaders(e.RedirectResponse.Headers, auth)})
			}
		}
		r.indices[e.RequestID] = len(r.requests)
		r.requests = append(r.requests, &request{Viewport: viewport, Method: e.Request.Method, URL: MaskURL(e.Request.URL), ResourceType: string(e.Type)})
		q := r.requests[len(r.requests)-1]
		if response := r.responses[e.RequestID]; response != nil && response.URL == e.Request.URL {
			s := response.Status
			q.Status = &s
			q.MIMEType = response.MimeType
		}
		if done, ok := r.ended[e.RequestID]; ok && done.at >= e.Timestamp {
			q.TransferredBytes = done.bytes
			q.Failure = done.failure
		}
		done := r.ended[e.RequestID]
		if !r.ignored[e.RequestID] && done.at < e.Timestamp && e.Type != network.ResourceTypeWebSocket && e.Type != network.ResourceTypeEventSource {
			r.active[e.RequestID] = true
			r.last = time.Now()
		}
		if e.Request.MixedContentType != "" && string(e.Request.MixedContentType) != "none" {
			r.mixed = true
		}
		if e.FrameID == main && e.Type == network.ResourceTypeDocument {
			r.headers = append(r.headers, headerRecord{Viewport: viewport, URL: MaskURL(e.Request.URL), Request: maskHeaders(e.Request.Headers, auth), Response: map[string]string{}})
			if o, err := Origin(e.Request.URL); err == nil {
				r.origins[o] = true
			}
			if e.RedirectResponse != nil {
				if o, err := Origin(e.RedirectResponse.URL); err == nil {
					r.origins[o] = true
				}
			}
		}
	})
	subscribe(ctx, t, r, "Network.responseReceived", func(e network.EventResponseReceived) {
		r.responses[e.RequestID] = e.Response
		if e.Type == network.ResourceTypeEventSource || e.Type == network.ResourceTypeWebSocket {
			r.ignored[e.RequestID] = true
			delete(r.active, e.RequestID)
		}
		if i, ok := r.indices[e.RequestID]; ok {
			s := e.Response.Status
			q := r.requests[i]
			q.Status = &s
			q.MIMEType = e.Response.MimeType
			if _, done := r.ended[e.RequestID]; !done {
				q.TransferredBytes = e.Response.EncodedDataLength
			}
		}
		if e.FrameID == main && e.Type == network.ResourceTypeDocument {
			r.status = e.Response.Status
			r.headers = append(r.headers, headerRecord{viewport, MaskURL(e.Response.URL), maskHeaders(e.Response.RequestHeaders, auth), maskHeaders(e.Response.Headers, auth)})
		}
	})
	subscribe(ctx, t, r, "Network.loadingFinished", func(e network.EventLoadingFinished) {
		r.ended[e.RequestID] = completion{at: e.Timestamp, bytes: e.EncodedDataLength}
		delete(r.active, e.RequestID)
		r.last = time.Now()
		if i, ok := r.indices[e.RequestID]; ok {
			r.requests[i].TransferredBytes = e.EncodedDataLength
		}
	})
	subscribe(ctx, t, r, "Network.loadingFailed", func(e network.EventLoadingFailed) {
		r.ended[e.RequestID] = completion{at: e.Timestamp, failure: redactText(e.ErrorText, auth)}
		delete(r.active, e.RequestID)
		r.last = time.Now()
		if i, ok := r.indices[e.RequestID]; ok {
			r.requests[i].Failure = redactText(e.ErrorText, auth)
		}
		if strings.Contains(strings.ToLower(e.ErrorText), "mixed") || strings.Contains(string(e.BlockedReason), "mixed") {
			r.mixed = true
		}
	})
	subscribe(ctx, t, r, "Page.frameNavigated", func(e page.EventFrameNavigated) {
		if e.Frame.ParentID == "" && strings.HasPrefix(e.Frame.URL, "http") && e.Frame.UnreachableURL == "" {
			r.available = true
			r.final = MaskURL(e.Frame.URL)
		}
	})
	subscribe(ctx, t, r, "Page.loadEventFired", func(e page.EventLoadEventFired) {
		r.loaded = true
	})
	subscribe(ctx, t, r, "Runtime.consoleAPICalled", func(e js.EventConsoleAPICalled) {
		parts := []string{}
		for _, a := range e.Args {
			if len(a.Value) > 0 {
				parts = append(parts, string(a.Value))
			} else if a.Description != "" {
				parts = append(parts, a.Description)
			}
		}
		r.console = append(r.console, consoleRecord{viewport, string(e.Type), redactText(strings.Join(parts, " "), auth)})
	})
	subscribe(ctx, t, r, "Runtime.exceptionThrown", func(e js.EventExceptionThrown) {
		text := e.ExceptionDetails.Text
		if e.ExceptionDetails.Exception != nil {
			text += " " + e.ExceptionDetails.Exception.Description
		}
		r.console = append(r.console, consoleRecord{viewport, "exception", redactText(text, auth)})
	})
	return r
}
func waitIdle(ctx context.Context, r *recorder, budget time.Duration, load bool) bool {
	timer := time.NewTimer(budget)
	defer timer.Stop()
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()
	for {
		r.Lock()
		ready := (!load || (r.loaded && r.available)) && len(r.active) == 0 && time.Since(r.last) >= time.Second
		r.Unlock()
		if ready {
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case <-timer.C:
			return false
		case <-tick.C:
		}
	}
}
func pause(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func writeJSON(path string, value any) error {
	b, e := packagefmt.JSON(value)
	if e != nil {
		return e
	}
	return os.WriteFile(path, b, 0600)
}
func boolPtr(v bool) *bool { return &v }

func Capture(ctx context.Context, work string, urls []string, positions []int, opt Options) (Result, error) {
	result := Result{}
	b := opt.Browser
	var e error
	if b == nil {
		path, err := browser.Find(opt.Chrome)
		if err != nil {
			return result, err
		}
		b, e = browser.Start(ctx, path, opt.Login, opt.TestBrowserOptions...)
		if e != nil {
			return result, e
		}
		defer b.Close()
	}
	ctx = b.Context
	t := chromedp.FromContext(ctx).Target
	if _, e = cdp.Call(ctx, t, network.Enable, network.EnableParams{}); e != nil {
		return result, e
	}
	if _, e = cdp.Call(ctx, t, page.Enable, page.EnableParams{}); e != nil {
		return result, e
	}
	if _, e = cdp.Call(ctx, t, js.Enable, cdp.Empty{}); e != nil {
		return result, e
	}
	_, e = cdp.Call(ctx, t, network.SetCacheDisabled, network.SetCacheDisabledParams{CacheDisabled: true})
	if e != nil {
		return result, e
	}
	{
		allowed := map[string]bool{}
		for _, u := range urls {
			o, _ := Origin(u)
			allowed[o] = true
		}
		ch, stop := t.Subscribe("Fetch.requestPaused")
		authEvents, stopAuth := t.Subscribe("Fetch.authRequired")
		defer stopAuth()
		defer stop()
		authCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		var wg sync.WaitGroup
		defer wg.Wait()
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-authCtx.Done():
					return
				case raw, ok := <-authEvents:
					if !ok {
						return
					}
					var ev fetch.EventAuthRequired
					if json.Unmarshal(raw, &ev) != nil {
						continue
					}
					call, c := context.WithTimeout(authCtx, 5*time.Second)
					_, _ = cdp.Call(call, t, fetch.ContinueWithAuth, fetch.ContinueWithAuthParams{RequestID: ev.RequestID, AuthChallengeResponse: &fetch.AuthChallengeResponse{Response: fetch.AuthChallengeResponseResponseCancelAuth}})
					c()
				case raw, ok := <-ch:
					if !ok {
						return
					}
					var ev fetch.EventRequestPaused
					if json.Unmarshal(raw, &ev) != nil {
						continue
					}
					headers := []*fetch.HeaderEntry{}
					for k, v := range ev.Request.Headers {
						if !strings.EqualFold(k, "Authorization") {
							headers = append(headers, &fetch.HeaderEntry{Name: k, Value: toString(v)})
						}
					}
					origin, _ := Origin(ev.Request.URL)
					if opt.Auth != "" && allowed[origin] {
						headers = append(headers, &fetch.HeaderEntry{Name: "Authorization", Value: "Basic " + base64.StdEncoding.EncodeToString([]byte(opt.Auth))})
					}
					call, c := context.WithTimeout(authCtx, 5*time.Second)
					_, _ = cdp.Call(call, t, fetch.ContinueRequest, fetch.ContinueRequestParams{RequestID: ev.RequestID, Headers: headers})
					c()
				}
			}
		}()
		if _, e = cdp.Call(ctx, t, fetch.Enable, fetch.EnableParams{HandleAuthRequests: boolPtr(true)}); e != nil {
			cancel()
			return result, e
		}
		// Cancellation precedes the join, including when capture returns normally.
		defer cancel()
	}
	result.Environment.ChromeVersion, _ = b.Version(ctx)
	result.Environment.ChromeVersion = strings.TrimSpace(result.Environment.ChromeVersion)
	result.Environment.UserAgent, _ = chromedp.Run(ctx, chromedp.Evaluate[string]("navigator.userAgent"))
	result.Environment.Locale, _ = chromedp.Run(ctx, chromedp.Evaluate[string]("navigator.language"))
	origins := map[string]bool{}
	result.Environment.TimeZone, _ = chromedp.Run(ctx, chromedp.Evaluate[string]("Intl.DateTimeFormat().resolvedOptions().timeZone"))
	if opt.Login {
		tree, err := cdp.Call(ctx, t, page.GetFrameTree, cdp.Empty{})
		if err != nil {
			return result, err
		}
		loginRecord := record(ctx, t, "login", opt.Auth, tree.FrameTree.Frame.ID)
		defer loginRecord.stop()
		nav, nc := context.WithTimeout(ctx, 30*time.Second)
		_, e = cdp.Call(nav, t, page.Navigate, page.NavigateParams{URL: urls[0]})
		nc()
		if e != nil {
			return result, fmt.Errorf("The login page could not open.")
		}
		if opt.LoginWait == nil {
			return result, fmt.Errorf("Manual login needs an Enter signal from the terminal.")
		}
		if e = opt.LoginWait(ctx); e != nil {
			return result, e
		}
		loginRecord.stop()
		loginRecord.Lock()
		for origin := range loginRecord.origins {
			origins[origin] = true
		}
		loginRecord.Unlock()
	}
	widths := []packagefmt.Viewport{{Name: "desktop", Width: 1440, Height: 900, Scale: 1}, {Name: "mobile", Width: 390, Height: 844, Scale: 2}}
	if opt.Tablet {
		widths = append(widths, packagefmt.Viewport{Name: "tablet", Width: 768, Height: 1024, Scale: 2})
	}
	for n, raw := range urls {
		u, _ := url.Parse(raw)
		slug := strings.Trim(strings.Map(func(r rune) rune {
			if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
				return r
			}
			return '-'
		}, u.Hostname()+u.Path), "-")
		if len(slug) > 100 {
			slug = strings.TrimRight(slug[:100], "-")
		}
		payload := fmt.Sprintf("web/%03d-%s", positions[n], slug)
		dir := filepath.Join(work, filepath.FromSlash(payload))
		if e = os.MkdirAll(dir, 0700); e != nil {
			return result, e
		}
		target := packagefmt.Target{Kind: "web", Name: MaskURL(raw), URL: MaskURL(raw), Payload: payload, State: "complete"}
		networkRecords := []*request{}
		headers := []headerRecord{}
		redirects := []redirectRecord{}
		console := []consoleRecord{}
		success := 0
		for _, v := range widths {
			if opt.Progress != nil {
				opt.Progress(fmt.Sprintf("Capturing %s at %s (%d×%d)", MaskURL(raw), v.Name, v.Width, v.Height))
			}
			captured, rec, err := captureViewport(ctx, t, raw, dir, v, opt)
			if err != nil {
				if captured.State == "failed" {
					captured.Notes = append(captured.Notes, "The viewport could not be captured.")
				} else {
					captured.State = "problems"
					captured.Notes = append(captured.Notes, "Some viewport evidence could not be saved.")
				}
			}
			if rec != nil {
				rec.stop()
				rec.Lock()
				networkRecords = append(networkRecords, rec.requests...)
				headers = append(headers, rec.headers...)
				redirects = append(redirects, rec.redirects...)
				console = append(console, rec.console...)
				for o := range rec.origins {
					origins[o] = true
				}
				if v.Name == "desktop" && captured.State != "failed" {
					target.FinalURL = rec.final
					target.Title = rec.title
				}
				rec.Unlock()
			}
			target.Viewports = append(target.Viewports, captured)
			for _, note := range captured.Notes {
				target.Notes = append(target.Notes, v.Name+": "+note)
			}
			if captured.State != "failed" {
				success++
			} else {
				target.Notes = append(target.Notes, v.Name+": no target document was captured")
			}
			if captured.State != "complete" {
				target.State = "problems"
			}
		}
		if success == 0 {
			target.State = "failed"
		}
		for name, value := range map[string]any{"network.json": struct {
			Type     string     `json:"type"`
			Requests []*request `json:"requests"`
		}{"sealjar.network/v1", networkRecords}, "headers.json": headers, "redirects.json": redirects, "console.json": console} {
			if e = writeJSON(filepath.Join(dir, name), value); e != nil {
				return result, e
			}
		}
		result.Targets = append(result.Targets, target)
	}
	if ctx.Err() != nil {
		return result, fmt.Errorf("Capture cancelled. Chrome was closed.")
	}
	list := []string{}
	for o := range origins {
		list = append(list, o)
	}
	sort.Strings(list)
	result.Hosts, e = CollectHosts(ctx, work, list)
	return result, e
}

func captureViewport(ctx context.Context, t *chromedp.Target, raw, dir string, v packagefmt.Viewport, opt Options) (packagefmt.Viewport, *recorder, error) {
	v.State = "failed"
	// Leave the previous document before starting this viewport's event record.
	reset, cancel := context.WithTimeout(ctx, 5*time.Second)
	_, _ = cdp.Call(reset, t, page.StopLoading, cdp.Empty{})
	err := chromedp.Do(reset, chromedp.Navigate("about:blank"))
	cancel()
	if err != nil {
		return v, nil, err
	}
	_, err = cdp.Call(ctx, t, emulation.SetDeviceMetricsOverride, emulation.SetDeviceMetricsOverrideParams{Width: int64(v.Width), Height: int64(v.Height), DeviceScaleFactor: v.Scale, Mobile: v.Name != "desktop"})
	if err != nil {
		return v, nil, err
	}
	tree, err := cdp.Call(ctx, t, page.GetFrameTree, cdp.Empty{})
	if err != nil {
		return v, nil, err
	}
	r := record(ctx, t, v.Name, opt.Auth, tree.FrameTree.Frame.ID)
	start := time.Now()
	nav, nc := context.WithTimeout(ctx, 30*time.Second)
	nr, navErr := cdp.Call(nav, t, page.Navigate, page.NavigateParams{URL: raw})
	nc()
	if navErr == nil && nr.ErrorText != "" {
		v.Notes = append(v.Notes, "Navigation failed: "+redactText(nr.ErrorText, opt.Auth))
		return v, r, fmt.Errorf("Navigation failed")
	}
	remaining := 30*time.Second - time.Since(start)
	idle := false
	if remaining > 0 {
		idle = waitIdle(ctx, r, remaining, true)
	}
	r.Lock()
	available := r.available
	r.Unlock()
	if !idle {
		v.Timeouts = append(v.Timeouts, "load and network idle (30s)")
	}
	if !available {
		v.Notes = append(v.Notes, "No target document became available within the navigation budget")
		return v, r, fmt.Errorf("No target document became available")
	}
	if err = pause(ctx, opt.Wait); err != nil {
		return v, r, err
	}
	evidence, ec := context.WithTimeout(ctx, 40*time.Second)
	defer ec()
	height := float64(v.Height)
	for y := 0; y < 20000; y += v.Height {
		height, err = chromedp.Run(evidence, chromedp.Evaluate[float64]("Math.max(document.documentElement.scrollHeight,document.body?.scrollHeight||0)"))
		if err != nil {
			return v, r, err
		}
		if _, err = chromedp.Run(evidence, chromedp.Evaluate[any](fmt.Sprintf("window.scrollTo(0,%d)", y))); err != nil {
			return v, r, err
		}
		if err = pause(evidence, 150*time.Millisecond); err != nil {
			return v, r, err
		}
		if float64(y+v.Height) >= height {
			break
		}
	}
	if !waitIdle(evidence, r, 10*time.Second, false) {
		v.Timeouts = append(v.Timeouts, "post-scroll network idle (10s)")
	}
	_, err = chromedp.Run(evidence, chromedp.Evaluate[any]("window.scrollTo(0,0)"))
	if err != nil {
		return v, r, err
	}
	height, err = chromedp.Run(evidence, chromedp.Evaluate[float64]("Math.max(document.documentElement.scrollHeight,document.body?.scrollHeight||0)"))
	if err != nil {
		return v, r, err
	}
	if height > 20000 {
		v.Truncated = true
		height = 20000
	}
	if height < float64(v.Height) {
		height = float64(v.Height)
	}
	shot, err := cdp.Call(evidence, t, page.CaptureScreenshot, page.CaptureScreenshotParams{Format: page.CaptureScreenshotFormatPng, CaptureBeyondViewport: boolPtr(true), Clip: &page.Viewport{X: 0, Y: 0, Width: float64(v.Width), Height: height, Scale: 1}})
	if err != nil {
		return v, r, err
	}
	if err = os.WriteFile(filepath.Join(dir, v.Name+".png"), shot.Data, 0600); err != nil {
		return v, r, err
	}
	v.State = "complete"
	v.PasswordForm, _ = chromedp.Run(evidence, chromedp.Evaluate[bool](`Array.from(document.querySelectorAll('input[type=password]')).some(e=>e.getClientRects().length>0 && getComputedStyle(e).visibility!=='hidden' && getComputedStyle(e).display!=='none')`))
	if v.Name == "desktop" {
		dom, e := chromedp.Run(evidence, chromedp.Evaluate[string]("document.documentElement.outerHTML"))
		if e != nil {
			return v, r, e
		}
		if e = os.WriteFile(filepath.Join(dir, "dom.html"), []byte(dom), 0600); e != nil {
			return v, r, e
		}
		title, e := chromedp.Run(evidence, chromedp.Title())
		if e != nil {
			return v, r, e
		}
		r.Lock()
		r.title = redactText(title, opt.Auth)
		r.Unlock()
		pdf, e := cdp.Call(evidence, t, page.PrintToPDF, page.PrintToPDFParams{PrintBackground: boolPtr(true), PaperWidth: 8.27, PaperHeight: 11.69})
		if e != nil {
			return v, r, e
		}
		if e = os.WriteFile(filepath.Join(dir, "page.pdf"), pdf.Data, 0600); e != nil {
			return v, r, e
		}
	}
	r.Lock()
	v.DocumentStatus = r.status
	v.MixedContent = r.mixed
	for _, req := range r.requests {
		u, _ := url.Parse(req.URL)
		if u != nil && u.Path == "/favicon.ico" {
			continue
		}
		if req.Failure != "" || req.Status != nil && *req.Status >= 400 {
			v.FailedRequests++
			if len(v.FailedURLs) < 10 {
				v.FailedURLs = append(v.FailedURLs, req.URL)
			}
		}
	}
	r.Unlock()
	v.State = "complete"
	if v.DocumentStatus < 200 || v.DocumentStatus >= 300 {
		v.Notes = append(v.Notes, fmt.Sprintf("Document returned status %d", v.DocumentStatus))
	}
	if v.FailedRequests > 0 {
		v.Notes = append(v.Notes, fmt.Sprintf("%d failed requests", v.FailedRequests))
	}
	if v.MixedContent {
		v.Notes = append(v.Notes, "Mixed content")
	}
	if v.Truncated {
		v.Notes = append(v.Notes, "Content exceeds 20,000 CSS px")
	}
	if v.PasswordForm {
		v.Notes = append(v.Notes, "Page shows a login form")
	}
	for _, limit := range v.Timeouts {
		v.Notes = append(v.Notes, "Wait limit reached: "+limit)
	}
	if len(v.Notes) > 0 {
		v.State = "problems"
	}
	return v, r, nil
}
