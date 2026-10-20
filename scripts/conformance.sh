#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
repo_root=$PWD
mkdir -p .vroni/wip/scratch
scratch=$(mktemp -d "$repo_root/.vroni/wip/scratch/conformance-XXXXXX")
trap 'rm -rf "$scratch"' EXIT
mkdir "$scratch/tmp"
export TMPDIR="$scratch/tmp"
export GIT_CONFIG_NOSYSTEM=1
export GIT_CONFIG_GLOBAL=/dev/null
CGO_ENABLED=0 go build -o "$scratch/signetleaf" ./cmd/signetleaf
cp -R testdata/example-repo "$scratch/example-repo"
git -C "$scratch/example-repo" init -q
git -C "$scratch/example-repo" add .
git -C "$scratch/example-repo" -c user.name='Snapshot Fixture' -c user.email=snapshot@example.invalid -c commit.gpgsign=false commit -qm 'Add example project'
mkdir "$scratch/output"
"$scratch/signetleaf" capture "$scratch/example-repo" --confirm --no-timestamp --out "$scratch/output" --json > "$scratch/capture.json"
package=$(python3 -c 'import json,sys,os; print(os.path.abspath(json.load(open(sys.argv[1]))["package"]))' "$scratch/capture.json")
go run github.com/vincentsch/rungrad/cmd/rungrad@v0.3.2 score "$scratch/signetleaf" \
  --manifest auto --strict \
  --read "restore $package --to $scratch/restored --dry-run" \
  --mutate "capture $scratch/example-repo --confirm --no-timestamp --out $scratch/output" \
  --not-found "verify $scratch/missing.signetleaf.zip"
