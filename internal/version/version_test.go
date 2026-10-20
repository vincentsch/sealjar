package version

import (
	"runtime/debug"
	"testing"
)

func TestBuildMetadataFallback(t *testing.T) {
	for _, tc := range []struct {
		name, injectedVersion, injectedCommit, module, revision, wantVersion, wantCommit string
	}{
		{"unknown", "dev", "dev", "(devel)", "", "dev", "dev"},
		{"tag without revision", "dev", "dev", "v0.1.0", "", "v0.1.0", "dev"},
		{"vcs", "dev", "dev", "v0.1.0", "abcdef1234567890", "v0.1.0", "abcdef1"},
		{"pseudo", "dev", "dev", "v0.0.0-20260102123456-123456abcdef", "", "v0.0.0-20260102123456-123456abcdef", "123456a"},
		{"prerelease pseudo", "dev", "dev", "v0.1.1-0.20260102123456-123456abcdef+incompatible", "", "v0.1.1-0.20260102123456-123456abcdef+incompatible", "123456a"},
		{"ordinary prerelease", "dev", "dev", "v0.1.0-rc1-abcdef123", "", "v0.1.0-rc1-abcdef123", "dev"},
		{"overrides", "v1.2.3", "custom", "v0.1.0", "abcdef1234567890", "v1.2.3", "custom"},
		{"version override", "v1.2.3", "dev", "(devel)", "abcdef1234567890", "v1.2.3", "abcdef1"},
		{"commit override", "dev", "custom", "v0.1.0", "abcdef1234567890", "v0.1.0", "custom"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			info := &debug.BuildInfo{Main: debug.Module{Version: tc.module}, Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: tc.revision}}}
			ver, commit := resolve(tc.injectedVersion, tc.injectedCommit, info)
			if ver != tc.wantVersion || commit != tc.wantCommit {
				t.Fatalf("got %q (%q), want %q (%q)", ver, commit, tc.wantVersion, tc.wantCommit)
			}
		})
	}
}
