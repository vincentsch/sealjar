// Package version provides build-time version information for the CLI.
// The Version and Commit variables are set via ldflags during build.
package version

import (
	"runtime/debug"
	"strings"
)

// Version is the build label, such as "v1.0.0". Builds set it via ldflags.
var Version = "dev"

// Commit is the short git commit hash (e.g., "abc1234"). Set via ldflags.
var Commit = "dev"

func init() {
	info, ok := debug.ReadBuildInfo()
	if ok {
		Version, Commit = resolve(Version, Commit, info)
	}
}

func resolve(ver, commit string, info *debug.BuildInfo) (string, string) {
	// Fall back to Go's embedded build metadata when ldflags are not set (e.g., go install).
	if ver != "dev" && commit != "dev" {
		return ver, commit
	}

	if ver == "dev" && info.Main.Version != "" && info.Main.Version != "(devel)" {
		ver = info.Main.Version
	}

	if commit == "dev" {
		if revision := commitFromSettings(info.Settings); revision != "" {
			commit = revision
		} else if revision := commitFromVersion(info.Main.Version); revision != "" {
			commit = revision
		}
	}
	return ver, commit
}

func commitFromSettings(settings []debug.BuildSetting) string {
	for _, setting := range settings {
		if setting.Key == "vcs.revision" && setting.Value != "" {
			return shortCommit(setting.Value)
		}
	}
	return ""
}

func commitFromVersion(version string) string {
	if version == "" || version == "(devel)" {
		return ""
	}
	// Pseudo-versions encode the VCS revision as the last hyphen segment.
	base := strings.SplitN(version, "+", 2)[0]
	parts := strings.Split(base, "-")
	if len(parts) < 3 || len(parts[len(parts)-2]) < 14 {
		return ""
	}
	stamp := parts[len(parts)-2]
	stamp = stamp[len(stamp)-14:]
	for _, r := range stamp {
		if r < '0' || r > '9' {
			return ""
		}
	}
	commit := parts[len(parts)-1]
	if len(commit) < 7 || !isHex(commit) {
		return ""
	}
	return shortCommit(commit)
}

func shortCommit(commit string) string {
	if len(commit) > 7 {
		return commit[:7]
	}
	return commit
}

func isHex(value string) bool {
	for _, r := range value {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'a' && r <= 'f':
		case r >= 'A' && r <= 'F':
		default:
			return false
		}
	}
	return true
}
