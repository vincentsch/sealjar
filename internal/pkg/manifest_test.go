package pkg

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestNormativeManifestFormat(t *testing.T) {
	b, e := os.ReadFile("testdata/manifest.example.json")
	if e != nil {
		t.Fatal(e)
	}
	var m Manifest
	if e = json.Unmarshal(b, &m); e != nil {
		t.Fatal(e)
	}
	if m.Type != ManifestType {
		t.Fatal("example version")
	}
	if e = m.Validate(); e != nil {
		t.Fatal(e)
	}
	encoded, e := JSON(m)
	if e != nil {
		t.Fatal(e)
	}
	if !bytes.Equal(encoded, b) {
		t.Fatal("manifest struct differs from the normative example")
	}
	var fields map[string]json.RawMessage
	json.Unmarshal(b, &fields)
	want := []string{"type", "tool_version", "capture_id", "started", "ended", "time_zone", "labels", "settings", "targets", "environment", "timestamp_authorities", "files"}
	if len(fields) != len(want) {
		t.Fatal("example fields")
	}
	for _, key := range want {
		if _, ok := fields[key]; !ok {
			t.Errorf("missing %s", key)
		}
	}
	original := append([]File{}, m.Files...)
	m.Files = append(m.Files, m.Files[0])
	if m.Validate() == nil {
		t.Fatal("accepted duplicate/unsorted manifest file")
	}
	m.Files = original
	m.Files[0].SHA256 = "not a hash"
	if m.Validate() == nil {
		t.Fatal("accepted invalid hash")
	}
}
func TestArchivePaths(t *testing.T) {
	for _, name := range []string{"/etc/passwd", "../file", "a/../b", "a\\b", "C:/file", "C:file", "./file", "a//b", "a\x00b", "."} {
		if SafePath(name) == nil {
			t.Errorf("accepted %q", name)
		}
	}
	for _, name := range []string{"repo/example/files.json", "VERIFY.txt", "a b.txt", "文.txt"} {
		if e := SafePath(name); e != nil {
			t.Errorf("%s: %v", name, e)
		}
	}
	for _, target := range []string{"../outside", "/absolute", "C:/absolute", `..\outside`} {
		if SafeLink("link", target) == nil {
			t.Errorf("accepted link to %q", target)
		}
	}
	if e := SafeLink("src/link", "../README.md"); e != nil {
		t.Fatal(e)
	}
	got := []bool{Exempt("manifest.json"), Exempt("manifest.json.example.tsr"), Exempt("SEAL.html"), Exempt("repo/manifest.json.example.tsr"), Exempt("report/a.html")}
	if !reflect.DeepEqual(got, []bool{true, true, true, false, false}) {
		t.Fatal(got)
	}
}
