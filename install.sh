#!/usr/bin/env bash
# Install a release binary after checking its SHA-256 checksum.
set -euo pipefail
scratch=
staged=
cleanup() {
  [[ -z "$scratch" ]] || rm -rf "$scratch"
  [[ -z "$staged" ]] || rm -f "$staged"
}
trap cleanup EXIT
fail() { printf '%s\n' "$*" >&2; exit 1; }

os=$(uname -s | tr '[:upper:]' '[:lower:]')
arch=$(uname -m | tr '[:upper:]' '[:lower:]')
case "$os" in
  linux) os=linux ;;
  darwin) os=darwin ;;
  mingw*|msys*|cygwin*) os=windows ;;
  *) fail "Unsupported operating system: $os. Use Linux, macOS or Git Bash on Windows." ;;
esac
case "$arch" in
  x86_64|amd64|x64) arch=amd64 ;;
  arm64|aarch64) arch=arm64 ;;
  *) fail "Unsupported architecture: $arch. Download an amd64 or arm64 release on a supported machine." ;;
esac
command -v curl >/dev/null 2>&1 || fail 'curl was not found. Install curl, then run this installer again.'
if command -v sha256sum >/dev/null 2>&1; then
  hash=(sha256sum)
elif command -v shasum >/dev/null 2>&1; then
  hash=(shasum -a 256)
else
  fail 'A SHA-256 tool was not found. Install sha256sum or shasum, then retry.'
fi
asset="sealjar-$os-$arch"
binary=sealjar
if [[ "$os" == windows ]]; then asset+=.exe; binary+=.exe; fi
base=${SEALJAR_DOWNLOAD_BASE:-https://github.com/vincentsch/sealjar/releases}
base=${base%/}
if [[ -n "${SEALJAR_VERSION:-}" ]]; then
  base="$base/download/$SEALJAR_VERSION"
else
  base="$base/latest/download"
fi
scratch=$(mktemp -d /tmp/sl-install.XXXXXX) || fail 'Could not create a temporary directory. Check free space and permissions in /tmp.'
for name in "$asset" checksums.txt; do
  curl --fail --silent --show-error --location "$base/$name" -o "$scratch/$name" || fail "Could not download $name. Check the release tag, network and SEALJAR_DOWNLOAD_BASE, then retry."
done
expected=$(awk -v name="$asset" '$2 == name {print $1}' "$scratch/checksums.txt")
[[ "$expected" =~ ^[[:xdigit:]]{64}$ ]] || fail "A single SHA-256 checksum for $asset was not found. Check the release's checksums.txt before retrying."
actual=$("${hash[@]}" "$scratch/$asset" | awk '{print $1}')
[[ "$(printf '%s' "$actual" | tr '[:upper:]' '[:lower:]')" == "$(printf '%s' "$expected" | tr '[:upper:]' '[:lower:]')" ]] || fail "Checksum verification failed for $asset. The installed binary was kept. Download again or report the broken release."
INSTALL_DIR=${INSTALL_DIR:-"$HOME/.local/bin"}
mkdir -p "$INSTALL_DIR" || fail 'Could not create INSTALL_DIR. Choose a writable directory and retry.'
INSTALL_DIR=$(cd "$INSTALL_DIR" && pwd -P)
# Stage beside the destination so replacement stays on the same filesystem.
staged=$(mktemp "$INSTALL_DIR/.sealjar.XXXXXX") || fail 'Could not write INSTALL_DIR. Choose a writable directory and retry.'
cp "$scratch/$asset" "$staged" && chmod 755 "$staged" || fail 'Could not stage the binary. Check disk space and directory permissions.'
mv -f "$staged" "$INSTALL_DIR/$binary" || fail 'Could not replace the binary. Close any running Sealjar process and check directory permissions.'
staged=
result=$("$INSTALL_DIR/$binary" --version) || fail 'The installed binary could not run. Check that the release matches your operating system and architecture.'
printf '%s\nInstalled: %s\n' "$result" "$INSTALL_DIR/$binary"
case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *) printf 'Add this directory to PATH in your shell profile:\n  export PATH=%q:"$PATH"\n' "$INSTALL_DIR" ;;
esac
printf '%s\n' 'Chrome or Chromium is required for website capture and the demo.' 'Git is required for repository capture and the demo.' 'Run sealjar doctor to check prerequisites, or sealjar --help to see commands.'
