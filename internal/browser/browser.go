// Package browser starts an isolated Chrome process and removes its profile.
package browser

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	protocolbrowser "github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/chromedp"
)

func Find(explicit string) (string, error) {
	if explicit != "" {
		p, e := exec.LookPath(explicit)
		if e != nil {
			return "", fmt.Errorf("Chrome could not start at the supplied path. Check --chrome or install Google Chrome or Chromium.")
		}
		return p, nil
	}
	for _, n := range []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser"} {
		if p, e := exec.LookPath(n); e == nil {
			return p, nil
		}
	}
	var paths []string
	if runtime.GOOS == "darwin" {
		paths = []string{"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome", "/Applications/Chromium.app/Contents/MacOS/Chromium"}
	}
	if runtime.GOOS == "windows" {
		for _, base := range []string{os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)")} {
			for _, n := range []string{"Google/Chrome/Application/chrome.exe", "Chromium/Application/chrome.exe"} {
				paths = append(paths, filepath.Join(base, filepath.FromSlash(n)))
			}
		}
	}
	for _, p := range paths {
		if s, e := os.Stat(p); e == nil && !s.IsDir() {
			return p, nil
		}
	}
	return "", fmt.Errorf("Chrome was not found. Install Google Chrome or Chromium, or pass its path with --chrome.")
}

// Version probes only an isolated browser, including on Windows where
// chrome.exe --version can open the user's normal profile.
func Version(ctx context.Context, path string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	b, err := Start(ctx, path, false)
	if err != nil {
		return "", err
	}
	defer b.Close()
	return b.Version(ctx)
}
func (b *Browser) Version(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	version, err := cdp.Call(ctx, chromedp.FromContext(b.Context).Browser, protocolbrowser.GetVersion, cdp.Empty{})
	return version.Product, err
}

type Browser struct {
	Context context.Context
	Profile string
	close   func()
}

func Start(ctx context.Context, path string, visible bool, testOptions ...chromedp.ExecAllocatorOption) (*Browser, error) {
	profile, e := os.MkdirTemp("", "sealjar-chrome-")
	if e != nil {
		return nil, e
	}
	if e = os.MkdirAll(filepath.Join(profile, "Default"), 0700); e != nil {
		os.RemoveAll(profile)
		return nil, e
	}
	preferences := []byte(`{"credentials_enable_service":false,"credentials_enable_autosignin":false,"profile":{"password_manager_enabled":false,"password_manager_leak_detection":false},"autofill":{"profile_enabled":false,"credit_card_enabled":false},"translate":{"enabled":false},"signin":{"allowed":false}}`)
	if e = os.WriteFile(filepath.Join(profile, "Default", "Preferences"), preferences, 0600); e != nil {
		os.RemoveAll(profile)
		return nil, e
	}
	opts := append(chromedp.DefaultExecAllocatorOptions[:], chromedp.ExecPath(path), chromedp.UserDataDir(profile), chromedp.WindowSize(1440, 900), chromedp.Flag("disable-features", "Translate,TranslateUI,PasswordLeakDetection,MediaRouter"), chromedp.Flag("disable-notifications", true), chromedp.Flag("deny-permission-prompts", true))
	if visible {
		opts = append(opts, chromedp.VisibleWindow)
	}
	opts = append(opts, testOptions...)
	ac, ca := chromedp.NewExecAllocator(ctx, opts...)
	startupContext, cancelStartup := context.WithCancel(ac)
	tc, ct := chromedp.NewContext(startupContext)
	close := func() {
		closing, cancel := context.WithTimeout(tc, 5*time.Second)
		defer cancel()
		chromedp.Cancel(closing)
		ct()
		cancelStartup()
		ca()
		os.RemoveAll(profile)
	}
	// Cancel the parent without consuming chromedp's allocation semaphore.
	startup := time.AfterFunc(15*time.Second, cancelStartup)
	e = chromedp.Do(tc)
	startup.Stop()
	if e != nil {
		// A failed allocation may leave a single value in the allocation channel.
		// Only ct may consume it; calling chromedp.Cancel first would deadlock ct.
		ct()
		cancelStartup()
		ca()
		os.RemoveAll(profile)
		if errors.Is(e, chromedp.ErrNoDisplay) {
			return nil, fmt.Errorf("--login opens a visible Chrome window, but this session has no display. Run capture from a desktop session, or capture without --login.")
		}
		return nil, fmt.Errorf("Chrome could not start. Check --chrome and the installed browser.")
	}
	return &Browser{tc, profile, close}, nil
}
func (b *Browser) Close() { b.close() }
