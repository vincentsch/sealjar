package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vincentsch/signetleaf/internal/browser"
	packagefmt "github.com/vincentsch/signetleaf/internal/pkg"
	"github.com/vincentsch/signetleaf/internal/testsite"
	"github.com/vincentsch/signetleaf/internal/verify"
)

func TestWebDryRunAndValidation(t *testing.T) {
	out := filepath.Join(t.TempDir(), "not-created")
	t.Setenv("PATH", t.TempDir())
	b, _ := invoke(t, 0, "capture", "https://example.com/?token=hidden", "--dry-run", "--json", "--chrome", "/missing", "--out", out)
	var result captureResult
	oneJSON(t, b, &result)
	if len(result.Pages) != 1 || strings.Contains(string(b), "hidden") {
		t.Fatal(string(b))
	}
	if _, e := os.Stat(out); !os.IsNotExist(e) {
		t.Fatal("dry run wrote output")
	}
	invoke(t, 1, "capture", "https://user:secret@example.com/", "--dry-run")
	invoke(t, 1, "capture", "https://example.com/", "--wait", "31s", "--dry-run")
	invoke(t, 1, "capture", "https://example.com/", "--login", "--confirm")
	invoke(t, 2, "capture", "https://example.com/", "--confirm", "--no-timestamp", "--out", out)
}
func TestStrictWebCaptureJSONAndMixedRestore(t *testing.T) {
	if _, e := browser.Find(""); e != nil {
		t.Skip(e)
	}
	s := httptest.NewServer(testsite.Handler())
	defer s.Close()
	root := fixture(t)
	out := t.TempDir()
	b, stderr := invoke(t, 10, "capture", s.URL+"/private/", "http://127.0.0.1:1/", root, "--confirm", "--strict", "--no-timestamp", "--out", out, "--json")
	var result captureResult
	oneJSON(t, b, &result)
	if len(stderr) != 0 || result.Package == "" {
		t.Fatalf("strict JSON: %s / %s", b, stderr)
	}
	if result.Targets[1].State != "problems" || result.Targets[2].State != "failed" {
		t.Fatalf("outcomes: %+v", result.Targets)
	}
	v, _ := invoke(t, 11, "verify", result.Package, "--json")
	var checked verify.Result
	oneJSON(t, v, &checked)
	if checked.Files.State != "Unchanged" || checked.Capture.State != "Problems" {
		t.Fatalf("verify: %+v", checked)
	}
	invoke(t, 0, "restore", result.Package, "--to", filepath.Join(t.TempDir(), "restore"))
	// Host folders and all page evidence survive the package round trip.
	r, e := packagefmt.Open(result.Package)
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	if len(r.Manifest.Hosts) != 2 {
		t.Fatalf("hosts: %+v", r.Manifest.Hosts)
	}
	for _, h := range r.Manifest.Hosts {
		if _, e = r.Read("hosts/" + h.OriginID + "/tls.json"); e != nil {
			t.Fatal(e)
		}
	}
	allFail := t.TempDir()
	for _, mode := range []string{"human", "json"} {
		args := []string{"capture", "http://127.0.0.1:1/?session=never-store", "http://127.0.0.1:2/", "--confirm", "--no-timestamp", "--out", allFail}
		if mode == "json" {
			args = append(args, "--json")
		}
		stdout, stderr := invoke(t, 2, args...)
		if mode == "json" {
			oneJSON(t, stderr, new(map[string]any))
		}
		output := string(stdout) + string(stderr)
		for _, want := range []string{"http://127.0.0.1:1/", "http://127.0.0.1:2/", "Navigation failed: net::ERR_UNSAFE_PORT"} {
			if !strings.Contains(output, want) {
				t.Errorf("all-failed %s output lacks %q: %s", mode, want, output)
			}
		}
		if strings.Contains(output, "never-store") {
			t.Fatal("all-failed error leaked secret query value")
		}
	}

	items, e := os.ReadDir(allFail)
	if e != nil || len(items) != 0 {
		t.Fatal("all-failed capture left a package or work folder")
	}
}
func TestDoctorPrerequisitesAndFixes(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("doctor sent an HTTP request") }))
	defer s.Close()
	out := filepath.Join(t.TempDir(), "not-created")
	b, _ := invoke(t, 0, "doctor", "--tsa", s.URL, "--out", out, "--json")
	var result struct {
		Prerequisites []struct {
			Reachable bool   `json:"reachable"`
			Fix       string `json:"fix"`
		}
		Authorities []struct {
			Reachable bool `json:"reachable"`
		} `json:"timestamp_authorities"`
	}
	oneJSON(t, b, &result)
	if len(result.Prerequisites) != 3 || len(result.Authorities) != 1 || !result.Authorities[0].Reachable {
		t.Fatal(string(b))
	}
	if _, e := os.Stat(out); !os.IsNotExist(e) {
		t.Fatal("doctor created output directory")
	}
	blocked := filepath.Join(t.TempDir(), "file")
	if e := os.WriteFile(blocked, []byte("keep"), 0600); e != nil {
		t.Fatal(e)
	}
	b, _ = invoke(t, 0, "doctor", "--chrome", "/missing", "--out", blocked, "--tsa", "http://127.0.0.1:1/")
	for _, text := range []string{"Fix: Install Google Chrome", "Fix: Choose a writable directory", "Fix: Check the endpoint"} {
		if !strings.Contains(string(b), text) {
			t.Fatalf("missing %q: %s", text, b)
		}
	}
}
func TestDoctorUsesTLSForHTTPS(t *testing.T) {
	s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprintln(w, "unexpected") }))
	defer s.Close()
	b, _ := invoke(t, 0, "doctor", "--tsa", s.URL, "--json")
	var result map[string]json.RawMessage
	oneJSON(t, b, &result)
	if !strings.Contains(string(result["timestamp_authorities"]), `"reachable": false`) {
		t.Fatal("untrusted TLS endpoint reported reachable")
	}
}

func TestBasicAuthFlagAndEnvironment(t *testing.T) {
	if _, e := browser.Find(""); e != nil {
		t.Skip(e)
	}
	s := httptest.NewServer(testsite.Handler())
	defer s.Close()
	for _, environment := range []bool{false, true} {
		t.Run(fmt.Sprintf("environment_%t", environment), func(t *testing.T) {
			args := []string{"capture", s.URL + "/private/", "--confirm", "--strict", "--no-timestamp", "--json", "--out", t.TempDir()}
			if environment {
				t.Setenv("SIGNETLEAF_AUTH", "client:handover")
			} else {
				args = append(args, "--auth", "client:handover")
			}
			b, stderr := invoke(t, 0, args...)
			var result captureResult
			oneJSON(t, b, &result)
			if len(stderr) != 0 || strings.Contains(string(b), "handover") {
				t.Fatalf("auth output: %s / %s", b, stderr)
			}
			if result.Targets[0].State != "complete" {
				t.Fatal("authenticated capture failed")
			}
			r, e := packagefmt.Open(result.Package)
			if e != nil {
				t.Fatal(e)
			}
			defer r.Close()
			if !r.Manifest.Settings.BasicAuth {
				t.Fatal("authentication method missing")
			}
		})
	}
}

func TestLoginEnterWaitCanBeCancelled(t *testing.T) {
	input, writer := io.Pipe()
	defer input.Close()
	defer writer.Close()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- waitForLoginEnter(ctx, input) }()
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancelled login succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("login kept waiting after cancellation")
	}
}

func TestRepositoryCaptureDoesNotClaimBasicAuth(t *testing.T) {
	t.Setenv("SIGNETLEAF_AUTH", "client:handover")
	name := captured(t, fixture(t), t.TempDir())
	reader, err := packagefmt.Open(name)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	if reader.Manifest.Settings.BasicAuth {
		t.Fatal("repository-only capture claimed basic auth was used")
	}
}
func TestFailedCertificateIsExplained(t *testing.T) {
	if _, e := browser.Find(""); e != nil {
		t.Skip(e)
	}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "Page") }))
	defer server.Close()
	for _, mode := range []string{"human", "json"} {
		args := []string{"capture", server.URL + "/?token=never-store", "--confirm", "--no-timestamp", "--out", t.TempDir()}
		if mode == "json" {
			args = append(args, "--json")
		}
		stdout, stderr := invoke(t, 2, args...)
		output := string(stdout) + string(stderr)
		if !strings.Contains(output, server.URL) || !strings.Contains(output, "ERR_CERT_AUTHORITY_INVALID") {
			t.Fatalf("certificate failure was not explained: %s", output)
		}
		if strings.Contains(output, "never-store") {
			t.Fatal("certificate failure leaked URL query")
		}
	}
}
