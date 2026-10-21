// Package pkg reads and writes Sealjar packages without extracting ZIP entries.
package pkg

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path"
	"regexp"
	"strings"
	"time"
)

const ManifestType = "sealjar.manifest/v1"

type File struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}
type Target struct {
	Kind      string     `json:"kind"`
	Name      string     `json:"name"`
	State     string     `json:"state"`
	Commit    string     `json:"commit"`
	Changes   int        `json:"changes"`
	Payload   string     `json:"payload"`
	URL       string     `json:"url,omitempty"`
	FinalURL  string     `json:"final_url,omitempty"`
	Title     string     `json:"title,omitempty"`
	Viewports []Viewport `json:"viewports,omitempty"`
	Notes     []string   `json:"notes,omitempty"`
}
type Viewport struct {
	Name           string   `json:"name"`
	Width          int      `json:"width"`
	Height         int      `json:"height"`
	Scale          float64  `json:"scale"`
	State          string   `json:"state"`
	DocumentStatus int64    `json:"document_status"`
	FailedRequests int      `json:"failed_requests"`
	FailedURLs     []string `json:"failed_urls,omitempty"`
	MixedContent   bool     `json:"mixed_content"`
	Truncated      bool     `json:"truncated"`
	PasswordForm   bool     `json:"password_form"`
	Timeouts       []string `json:"timeouts,omitempty"`
	Notes          []string `json:"notes,omitempty"`
}
type Host struct {
	Origin   string `json:"origin"`
	OriginID string `json:"origin_id"`
}
type Settings struct {
	IncludeUntracked bool     `json:"include_untracked"`
	History          bool     `json:"history"`
	Exclude          []string `json:"exclude"`
	NoTimestamp      bool     `json:"no_timestamp"`
	BasicAuth        bool     `json:"basic_auth,omitempty"`
	ManualLogin      bool     `json:"manual_login,omitempty"`
	Wait             string   `json:"wait,omitempty"`
	Widths           []int    `json:"widths,omitempty"`
}
type Environment struct {
	OS            string `json:"os"`
	Arch          string `json:"arch"`
	GitVersion    string `json:"git_version"`
	ChromeVersion string `json:"chrome_version,omitempty"`
	UserAgent     string `json:"user_agent,omitempty"`
	Locale        string `json:"locale,omitempty"`
	TimeZone      string `json:"time_zone,omitempty"`
}
type Manifest struct {
	Type                 string            `json:"type"`
	ToolVersion          string            `json:"tool_version"`
	CaptureID            string            `json:"capture_id"`
	Started              time.Time         `json:"started"`
	Ended                time.Time         `json:"ended"`
	TimeZone             string            `json:"time_zone"`
	Labels               map[string]string `json:"labels"`
	Settings             Settings          `json:"settings"`
	Targets              []Target          `json:"targets"`
	Environment          Environment       `json:"environment"`
	TimestampAuthorities []string          `json:"timestamp_authorities"`
	Hosts                []Host            `json:"hosts,omitempty"`
	Files                []File            `json:"files"`
}

func JSON(v any) ([]byte, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}
func Hash(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func Exempt(name string) bool {
	return name == "manifest.json" || name == "SEAL.html" || (strings.HasPrefix(name, "manifest.json.") && strings.HasSuffix(name, ".tsr") && !strings.Contains(name, "/"))
}

var drive = regexp.MustCompile(`^[A-Za-z]:`)
var hash = regexp.MustCompile(`^[0-9a-f]{64}$`)

func SafePath(name string) error {
	if name == "" || strings.ContainsAny(name, "\\\x00") || strings.HasPrefix(name, "/") || drive.MatchString(name) || path.Clean(name) != name || name == "." {
		return fmt.Errorf("unsafe archive path %q", name)
	}
	for _, p := range strings.Split(name, "/") {
		if p == ".." {
			return fmt.Errorf("unsafe archive path %q", name)
		}
	}
	return nil
}
func SafeLink(name, target string) error {
	if target == "" || strings.ContainsAny(target, "\\\x00") || strings.HasPrefix(target, "/") || drive.MatchString(target) {
		return fmt.Errorf("link %q points outside the repository", name)
	}
	joined := path.Join(path.Dir(name), target)
	if joined == ".." || strings.HasPrefix(joined, "../") {
		return fmt.Errorf("link %q points outside the repository", name)
	}
	return nil
}
func (m Manifest) Validate() error {
	if m.ToolVersion == "" || m.CaptureID == "" || m.Started.IsZero() || m.Ended.Before(m.Started) || m.TimeZone == "" || len(m.Targets) == 0 || m.Files == nil {
		return fmt.Errorf("manifest is missing capture fields")
	}
	previous := ""
	for _, f := range m.Files {
		if err := SafePath(f.Path); err != nil {
			return err
		}
		if f.Path <= previous || Exempt(f.Path) || f.Size < 0 || !hash.MatchString(f.SHA256) {
			return fmt.Errorf("invalid manifest file record %q", f.Path)
		}
		previous = f.Path
	}
	for _, t := range m.Targets {
		if t.Name == "" || (t.Kind != "repo" && t.Kind != "web") || (t.State != "complete" && t.State != "problems" && t.State != "failed") {
			return fmt.Errorf("invalid capture target")
		}
		if t.Kind == "repo" && t.State != "complete" {
			return fmt.Errorf("invalid repository target")
		}
		if err := SafePath(t.Payload); err != nil {
			return err
		}
	}
	return nil
}
