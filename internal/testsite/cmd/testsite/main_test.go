package main

import (
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestSiteProcess(t *testing.T) {
	if os.Getenv("SEALJAR_TEST_SITE_PROCESS") == "1" {
		if err := run(os.Getenv("SEALJAR_TEST_SITE_OUT")); err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}
	if runtime.GOOS == "windows" {
		t.Skip("Windows cannot send os.Interrupt to a child process")
	}
	out := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=^TestSiteProcess$")
	cmd.Env = append(os.Environ(), "SEALJAR_TEST_SITE_PROCESS=1", "SEALJAR_TEST_SITE_OUT="+out)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	deadline := time.Now().Add(10 * time.Second)
	var endpoint string
	for time.Now().Before(deadline) {
		b, err := os.ReadFile(filepath.Join(out, "endpoint"))
		if err == nil && len(b) > 0 {
			endpoint = string(b)
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !strings.HasPrefix(endpoint, "http://127.0.0.1:") {
		t.Fatalf("missing loopback endpoint: %q", endpoint)
	}
	client := &http.Client{Timeout: 5 * time.Second}
	for _, tc := range []struct {
		path string
		auth bool
		want int
	}{
		{"/", false, 200},
		{"/private/", false, 401},
		{"/private/", true, 200},
	} {
		req, _ := http.NewRequest(http.MethodGet, endpoint+tc.path, nil)
		if tc.auth {
			req.SetBasicAuth("client", "handover")
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != tc.want {
			t.Errorf("%s auth=%t: got %d, want %d", tc.path, tc.auth, resp.StatusCode, tc.want)
		}
	}
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("unclean shutdown: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("fixture did not stop on interrupt")
	}
}

func TestMissingOutputDirectory(t *testing.T) {
	if err := run(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("expected an error for a nonexistent output directory")
	}
}
