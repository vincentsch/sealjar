// Package testutil shares prerequisite checks across browser tests.
package testutil

import (
	"os"
	"testing"
)

// RequireChrome makes a missing browser an error in browser-required CI runs.
func RequireChrome(t testing.TB, err error) {
	t.Helper()
	if err == nil {
		return
	}
	if os.Getenv("SIGNETLEAF_REQUIRE_CHROME") == "1" {
		t.Fatalf("Chrome is required by SIGNETLEAF_REQUIRE_CHROME=1: %v", err)
	}
	t.Skipf("skipped: no Chrome: %v", err)
}
