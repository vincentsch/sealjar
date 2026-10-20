#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
repo_root=$PWD
mkdir -p .vroni/wip/scratch
scratch=$(mktemp -d "$repo_root/.vroni/wip/scratch/conformance-XXXXXX")
tsa_pid=
cleanup() {
  if [[ -n "$tsa_pid" ]]; then kill "$tsa_pid" 2>/dev/null || true; wait "$tsa_pid" 2>/dev/null || true; fi
  rm -rf "$scratch"
}
trap cleanup EXIT
mkdir "$scratch/tmp"
export TMPDIR="$scratch/tmp"
export GIT_CONFIG_NOSYSTEM=1
export GIT_CONFIG_GLOBAL=/dev/null
CGO_ENABLED=0 go build -o "$scratch/signetleaf" ./cmd/signetleaf
CGO_ENABLED=0 go build -o "$scratch/testtsa" ./internal/tsa/testtsa/cmd/testtsa
"$scratch/testtsa" --out "$scratch" &
tsa_pid=$!
python3 - "$scratch/endpoint" <<'PYREADY'
from pathlib import Path
import sys, time
for _ in range(100):
    if Path(sys.argv[1]).exists():
        break
    time.sleep(0.05)
else:
    raise SystemExit("Local timestamp authority did not start")
PYREADY
tsa_url=$(cat "$scratch/endpoint")
cp -R testdata/example-repo "$scratch/example-repo"
git -C "$scratch/example-repo" init -q
git -C "$scratch/example-repo" add .
git -C "$scratch/example-repo" -c user.name='Snapshot Fixture' -c user.email=snapshot@example.invalid -c commit.gpgsign=false commit -qm 'Add example project'
mkdir "$scratch/output"
"$scratch/signetleaf" capture "$scratch/example-repo" --confirm --tsa "$tsa_url" --tsa-root "$scratch/root.pem" --out "$scratch/output" --json > "$scratch/capture.json"
package=$(python3 -c 'import json,sys,os; print(os.path.abspath(json.load(open(sys.argv[1]))["package"]))' "$scratch/capture.json")
go run github.com/vincentsch/rungrad/cmd/rungrad@v0.3.2 score "$scratch/signetleaf" \
  --manifest auto --strict \
  --read "verify $package --tsa-root $scratch/root.pem" \
  --mutate "capture $scratch/example-repo --confirm --no-timestamp --out $scratch/output" \
  --not-found "verify $scratch/missing.signetleaf.zip"
