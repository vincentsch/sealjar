#!/usr/bin/env bash
# Serve two local release versions, optionally running a check against them.
set -euo pipefail
cd "$(dirname "$0")/.."
scratch=$(mktemp -d /tmp/sl-release.XXXXXX)
server_pid=
cleanup() {
  if [[ -n "$server_pid" ]]; then kill "$server_pid" 2>/dev/null || true; wait "$server_pid" 2>/dev/null || true; fi
  rm -rf "$scratch"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
export TMPDIR="$scratch"
mkdir -p "$scratch/download/v0.0.1" "$scratch/download/v0.1.0" "$scratch/latest/download"
GOOS=linux GOARCH=amd64 make build VERSION=v0.0.1
cp bin/sealjar "$scratch/download/v0.0.1/sealjar-linux-amd64"
(cd "$scratch/download/v0.0.1" && if command -v sha256sum >/dev/null 2>&1; then sha256sum sealjar-linux-amd64; else shasum -a 256 sealjar-linux-amd64; fi > checksums.txt)
make build-all VERSION=v0.1.0
cp dist/sealjar-* dist/checksums.txt "$scratch/download/v0.1.0/"
cp dist/sealjar-* dist/checksums.txt "$scratch/latest/download/"
python3 - "$scratch" <<'PY' &
from http.server import SimpleHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
import os, sys
root = Path(sys.argv[1])
os.chdir(root)
class Handler(SimpleHTTPRequestHandler):
    def log_message(self, fmt, *args):
        with (root / 'requests.log').open('a') as log:
            log.write((fmt % args) + '\n')
server = ThreadingHTTPServer(('127.0.0.1', 0), Handler)
(root / 'endpoint').write_text(f'http://127.0.0.1:{server.server_port}')
server.serve_forever()
PY
server_pid=$!
python3 - "$scratch/endpoint" <<'PY'
from pathlib import Path
import sys, time
for _ in range(100):
    if Path(sys.argv[1]).exists(): break
    time.sleep(0.05)
else: raise SystemExit('Local release server did not start')
PY
export SEALJAR_DOWNLOAD_BASE="$(cat "$scratch/endpoint")"
export SEALJAR_FIXTURE_DIR="$scratch"
printf 'Local releases: %s\n' "$SEALJAR_DOWNLOAD_BASE"
if [[ $# -gt 0 ]]; then "$@"; else
  printf 'Use SEALJAR_DOWNLOAD_BASE=%q bash install.sh in another terminal. Press Ctrl-C to stop.\n' "$SEALJAR_DOWNLOAD_BASE"
  wait "$server_pid"
fi
