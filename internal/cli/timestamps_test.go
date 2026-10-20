package cli

import (
	"bytes"
	"crypto"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/digitorus/timestamp"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/vincentsch/signetleaf/internal/tsa"
	"github.com/vincentsch/signetleaf/internal/tsa/testtsa"
	"github.com/vincentsch/signetleaf/internal/verify"
)

func localTSA(t *testing.T, opt testtsa.Options) (string, string) {
	t.Helper()
	a, e := testtsa.New(opt)
	if e != nil {
		t.Fatal(e)
	}
	server := httptest.NewServer(a)
	t.Cleanup(server.Close)
	root := filepath.Join(t.TempDir(), "root.pem")
	write(t, root, a.RootPEM())
	return server.URL, root
}
func timestampCapture(t *testing.T, code int, root, out string, flags ...string) captureResult {
	t.Helper()
	args := []string{"capture", root, "--confirm", "--out", out, "--json"}
	args = append(args, flags...)
	b, stderr := invoke(t, code, args...)
	if len(stderr) > 0 {
		t.Fatalf("unexpected stderr: %s", stderr)
	}
	var r captureResult
	oneJSON(t, b, &r)
	return r
}
func assertVerify(t *testing.T, code int, path string, flags ...string) verify.Result {
	t.Helper()
	args := []string{"verify", path, "--json"}
	args = append(args, flags...)
	b, err := invoke(t, code, args...)
	if len(err) > 0 {
		t.Fatalf("completed verification emitted stderr: %s", err)
	}
	var r verify.Result
	oneJSON(t, b, &r)
	if r.Capture.State == "" || r.Files.State == "" || r.Timestamp.State == "" {
		t.Fatalf("missing answers: %s", b)
	}
	return r
}
func stamp(t *testing.T, path string, flags ...string) string {
	t.Helper()
	args := []string{"stamp", path, "--json"}
	args = append(args, flags...)
	b, _ := invoke(t, 0, args...)
	var r struct {
		Package string `json:"package"`
	}
	oneJSON(t, b, &r)
	if r.Package == "" {
		t.Fatalf("no package: %s", b)
	}
	return r.Package
}
func TestTimestampCaptureAndStamp(t *testing.T) {
	repo := fixture(t)
	out := t.TempDir()
	url, root := localTSA(t, testtsa.Options{})
	result := timestampCapture(t, 0, repo, out, "--tsa", url, "--tsa-root", root)
	assertVerify(t, 0, result.Package, "--tsa-root", root)
	assertVerify(t, 11, result.Package)
	before := entries(t, result.Package)
	orphan := strings.TrimSuffix(result.Package, ".signetleaf.zip") + "-stamped.signetleaf"
	if e := os.Mkdir(orphan, 0700); e != nil {
		t.Fatal(e)
	}
	stamped := stamp(t, result.Package, "--tsa", url, "--tsa-root", root)
	if !strings.HasSuffix(stamped, "-stamped-2.signetleaf.zip") {
		t.Fatalf("reader collision ignored: %s", stamped)
	}
	after := entries(t, stamped)
	for n, b := range before {
		if !bytes.Equal(after[n], b) {
			t.Fatalf("stamp changed %s", n)
		}
	}
	if len(after) != len(before)+1 {
		t.Fatal("stamp did not add a unique token")
	}
	assertVerify(t, 0, stamped, "--tsa-root", root)
	url2, root2 := localTSA(t, testtsa.Options{})
	twice := stamp(t, stamped, "--tsa", url2, "--tsa-root", root2)
	last := entries(t, twice)
	for n, b := range after {
		if !bytes.Equal(last[n], b) {
			t.Fatalf("second stamp changed %s", n)
		}
	}
	assertVerify(t, 0, twice, "--tsa-root", root2)
	unsigned := captured(t, repo, t.TempDir())
	signed := stamp(t, unsigned, "--tsa", url, "--tsa-root", root)
	assertVerify(t, 0, signed, "--tsa-root", root)
	invoke(t, 0, "restore", signed, "--to", filepath.Join(t.TempDir(), "recovered"), "--tsa-root", root, "--json")
	// A rewritten but structurally valid manifest must still fail its imprint.
	altered := entries(t, signed)
	var m map[string]any
	json.Unmarshal(altered["manifest.json"], &m)
	m["tool_version"] = "changed"
	altered["manifest.json"], _ = json.Marshal(m)
	changed := filepath.Join(t.TempDir(), "changed.signetleaf.zip")
	zipEntries(t, changed, altered)
	verdict := assertVerify(t, 10, changed, "--tsa-root", root)
	if !strings.Contains(verdict.Tokens[0].Reason, "imprint mismatch") {
		t.Fatal(verdict)
	}
	dir := t.TempDir()
	bad := filepath.Join(dir, "corrupt.signetleaf.zip")
	write(t, bad, []byte("not a ZIP"))
	invoke(t, 10, "stamp", bad, "--tsa", url, "--json")
	zipEntries(t, bad, altered)
	invoke(t, 10, "stamp", bad, "--tsa", url, "--json")
	names, _ := os.ReadDir(dir)
	if len(names) != 1 {
		t.Fatal("failed stamp wrote output")
	}
}
func TestCaptureTimestampFailureAndPartialSuccess(t *testing.T) {
	repo := fixture(t)
	good, root := localTSA(t, testtsa.Options{})
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503) }))
	defer bad.Close()
	old := tsa.Defaults
	tsa.Defaults = []string{bad.URL, good}
	defer func() { tsa.Defaults = old }()
	for _, tc := range []struct {
		name                   string
		flags                  []string
		code, tokens, attempts int
	}{
		{"default fallback", []string{"--strict", "--tsa-root", root}, 0, 1, 2},
		{"partial explicit", []string{"--strict", "--tsa", bad.URL, "--tsa", good, "--tsa-root", root}, 0, 1, 2},
		{"unknown root usable", []string{"--strict", "--tsa", good}, 0, 1, 1},
		{"all fail", []string{"--tsa", bad.URL}, 0, 0, 1},
		{"all fail strict", []string{"--strict", "--tsa", bad.URL}, 10, 0, 1},
		{"skip strict", []string{"--strict", "--no-timestamp", "--tsa", bad.URL}, 0, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := timestampCapture(t, tc.code, repo, t.TempDir(), tc.flags...)
			files := entries(t, r.Package)
			count := 0
			for n := range files {
				if strings.HasSuffix(n, ".tsr") {
					count++
				}
			}
			if count != tc.tokens || len(r.Attempts) != tc.attempts {
				t.Fatalf("tokens=%d result=%+v", count, r)
			}
			if strings.Contains(tc.name, "fail") && (r.Timestamp != "Not timestamped" || r.Attempts[0].Error == "") {
				t.Fatal(r)
			}
			if tc.name == "unknown root usable" {
				assertVerify(t, 11, r.Package)
			}
		})
	}
	unsigned := captured(t, repo, t.TempDir())
	invoke(t, 2, "stamp", unsigned, "--tsa", bad.URL, "--json")
	for _, opt := range []testtsa.Options{{WrongNonce: true}, {WrongImprint: true}, {WrongBinding: true}, {MissingSigner: true}, {NonCritical: true}, {MultiPurpose: true}, {InvalidSignature: true}} {
		url, _ := localTSA(t, opt)
		r := timestampCapture(t, 10, repo, t.TempDir(), "--tsa", url, "--strict")
		if r.Timestamp != "Not timestamped" || len(entries(t, r.Package)) == 0 || r.Attempts[0].Error == "" {
			t.Fatal(r)
		}
	}
}
func TestTimestampDryRunsAndVerdictPrecedence(t *testing.T) {
	repo := fixture(t)
	url, root := localTSA(t, testtsa.Options{})
	var requests atomic.Int32
	spy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1); w.WriteHeader(500) }))
	defer spy.Close()
	out := filepath.Join(t.TempDir(), "absent")
	timestampCapture(t, 0, repo, out, "--tsa", spy.URL, "--dry-run")
	if _, e := os.Stat(out); !os.IsNotExist(e) {
		t.Fatal("dry run created output")
	}
	unsigned := captured(t, repo, t.TempDir())
	invoke(t, 0, "stamp", unsigned, "--tsa", spy.URL, "--dry-run", "--json")
	if requests.Load() != 0 {
		t.Fatal("dry run contacted authority")
	}
	signed := stamp(t, unsigned, "--tsa", url, "--tsa-root", root)
	files := entries(t, signed)
	for _, tc := range []struct {
		name string
		opt  testtsa.Options
		code int
	}{{"missing certificate", testtsa.Options{MissingSigner: true}, 0}, {"binding", testtsa.Options{WrongBinding: true}, 0}, {"signature", testtsa.Options{InvalidSignature: true}, 10}, {"imprint", testtsa.Options{WrongImprint: true}, 10}} {
		a, e := testtsa.New(tc.opt)
		if e != nil {
			t.Fatal(e)
		}
		q, e := tsaRequest(files["manifest.json"])
		if e != nil {
			t.Fatal(e)
		}
		der, e := a.Response(q)
		if e != nil {
			t.Fatal(e)
		}
		files["manifest.json.additional.tsr"] = der
		path := filepath.Join(t.TempDir(), "mixed.signetleaf.zip")
		zipEntries(t, path, files)
		r := assertVerify(t, tc.code, path, "--tsa-root", root)
		if len(r.Tokens) != 2 {
			t.Fatal(r)
		}
		// Remove the trusted token: trust failures give 11, invalid tokens give 10.
		solo := map[string][]byte{}
		for n, b := range files {
			if !strings.HasSuffix(n, ".tsr") || n == "manifest.json.additional.tsr" {
				solo[n] = b
			}
		}
		zipEntries(t, path, solo)
		expected := 11
		if tc.code == 10 {
			expected = 10
		}
		assertVerify(t, expected, path, "--tsa-root", root)
	}
	files["manifest.json.additional.tsr"] = []byte("malformed")
	path := filepath.Join(t.TempDir(), "malformed.signetleaf.zip")
	zipEntries(t, path, files)
	assertVerify(t, 10, path, "--tsa-root", root)
}

func tsaRequest(manifest []byte) (*timestamp.Request, error) {
	b, e := timestamp.CreateRequest(bytes.NewReader(manifest), &timestamp.RequestOptions{Hash: crypto.SHA256, Certificates: true, Nonce: big.NewInt(123)})
	if e != nil {
		return nil, e
	}
	return timestamp.ParseRequest(b)
}

func TestTrustFlagsOnDiagnosticsAndStoredReports(t *testing.T) {
	repo := fixture(t)
	unsigned := captured(t, repo, t.TempDir())
	url, root := localTSA(t, testtsa.Options{})
	var requests atomic.Int32
	diagnostic := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1) }))
	defer diagnostic.Close()
	invoke(t, 0, "doctor", "--tsa", diagnostic.URL, "--json")
	if requests.Load() != 0 {
		t.Fatal("connection check sent an HTTP request")
	}
	invoke(t, 1, "doctor", "--tsa", "ftp://example.invalid", "--json")
	out := filepath.Join(t.TempDir(), "reader")
	invoke(t, 5, "report", unsigned, "--out", out, "--tsa-root", root, "--json")
	if _, e := os.Stat(out); !os.IsNotExist(e) {
		t.Fatal("missing report wrote output")
	}
	files := entries(t, unsigned)
	var manifest struct {
		Files []struct {
			Path string `json:"path"`
			Size int    `json:"size"`
			Hash string `json:"sha256"`
		} `json:"files"`
	}
	// Extend the existing manifest without changing any captured repository bytes.
	var m map[string]json.RawMessage
	if e := json.Unmarshal(files["manifest.json"], &m); e != nil {
		t.Fatal(e)
	}
	if e := json.Unmarshal(files["manifest.json"], &manifest); e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"report/report.en.html", "report/report.de.html", "report/report.de.pdf"} {
		b := []byte("stored report copy")
		files[name] = b
		h := sha256.Sum256(b)
		manifest.Files = append(manifest.Files, struct {
			Path string `json:"path"`
			Size int    `json:"size"`
			Hash string `json:"sha256"`
		}{name, len(b), hex.EncodeToString(h[:])})
	}
	sort.Slice(manifest.Files, func(i, j int) bool { return manifest.Files[i].Path < manifest.Files[j].Path })
	m["files"], _ = json.Marshal(manifest.Files)
	files["manifest.json"], _ = json.Marshal(m)
	stored := filepath.Join(t.TempDir(), "stored.signetleaf.zip")
	zipEntries(t, stored, files)
	signed := stamp(t, stored, "--tsa", url, "--tsa-root", root)
	// A stored seal is unauthenticated and must never reach the reader folder.
	forged := entries(t, signed)
	forged["SEAL.html"] = []byte("Timestamped by DigiCert at 2019-01-01. Trusted.")
	zipEntries(t, signed, forged)
	assertVerify(t, 0, signed, "--tsa-root", root)
	b, _ := invoke(t, 0, "report", signed, "--out", out, "--lang", "de", "--pdf", "--tsa-root", root, "--dry-run", "--json")
	var result struct {
		Verification verify.Result `json:"verification"`
	}
	oneJSON(t, b, &result)
	if result.Verification.Timestamp.State != "Trusted" {
		t.Fatal(result)
	}
	if _, e := os.Stat(out); !os.IsNotExist(e) {
		t.Fatal("dry run wrote reader")
	}
	invoke(t, 0, "report", signed, "--out", out, "--lang", "de", "--pdf", "--tsa-root", root, "--json")
	for _, n := range []string{"report/report.de.html", "report/report.de.pdf", "VERIFY.txt"} {
		b, e := os.ReadFile(filepath.Join(out, n))
		if e != nil || !bytes.Equal(b, files[n]) {
			t.Fatalf("copy %s: %v", n, e)
		}
	}
	if _, e := os.Stat(filepath.Join(out, "SEAL.html")); !os.IsNotExist(e) {
		t.Fatalf("unauthenticated seal copied: %v", e)
	}
	if _, e := os.Stat(filepath.Join(out, "report/report.en.html")); !os.IsNotExist(e) {
		t.Fatal("wrong language copied")
	}
}

func TestTimestampMetadataOutput(t *testing.T) {
	repo := fixture(t)
	url, root := localTSA(t, testtsa.Options{OmitAccuracy: true})
	capture := timestampCapture(t, 0, repo, t.TempDir(), "--tsa", url, "--tsa-root", root)
	if len(capture.Attempts) != 1 || capture.Attempts[0].Token.Accuracy != "unspecified" {
		t.Fatalf("capture JSON: %+v", capture)
	}
	args := []string{"--tsa", url, "--tsa-root", root}
	out, _ := invoke(t, 0, append([]string{"capture", repo, "--out", t.TempDir(), "--confirm"}, args...)...)
	if !bytes.Contains(out, []byte("accuracy unspecified")) {
		t.Fatalf("capture text: %s", out)
	}
	result := assertVerify(t, 0, capture.Package, "--tsa-root", root)
	if result.Tokens[0].Accuracy != "unspecified" {
		t.Fatalf("verify JSON: %+v", result)
	}
	out, _ = invoke(t, 0, "verify", capture.Package, "--tsa-root", root)
	if !bytes.Contains(out, []byte("accuracy unspecified")) {
		t.Fatalf("verify text: %s", out)
	}
	out, _ = invoke(t, 0, append([]string{"stamp", capture.Package}, args...)...)
	if !bytes.Contains(out, []byte("accuracy unspecified")) {
		t.Fatalf("stamp text: %s", out)
	}
	out, _ = invoke(t, 0, append([]string{"stamp", capture.Package, "--json"}, args...)...)
	var stamped struct {
		Attempts []tsa.Attempt `json:"timestamp_attempts"`
	}
	oneJSON(t, out, &stamped)
	if len(stamped.Attempts) != 1 || stamped.Attempts[0].Token.Accuracy != "unspecified" {
		t.Fatalf("stamp JSON: %s", out)
	}
	files := entries(t, capture.Package)
	files["manifest.json.bad.tsr"] = []byte("malformed")
	broken := filepath.Join(t.TempDir(), "broken.signetleaf.zip")
	zipEntries(t, broken, files)
	result = assertVerify(t, 10, broken, "--tsa-root", root)
	for _, token := range result.Tokens {
		if token.Path == "manifest.json.bad.tsr" && (token.Time != "unavailable" || token.Serial != "unavailable" || token.Accuracy != "unavailable") {
			t.Fatalf("unreadable JSON metadata: %+v", token)
		}
	}
	for _, command := range []string{"verify", "stamp"} {
		out, stderr := invoke(t, 10, command, broken, "--tsa-root", root)
		all := append(out, stderr...)
		if !bytes.Contains(all, []byte("time unavailable; serial unavailable; accuracy unavailable")) || bytes.Contains(all, []byte("0001-01-01")) {
			t.Fatalf("unreadable %s metadata: %s", command, all)
		}
	}
}
