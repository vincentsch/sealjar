# Make your first capture

Signetleaf writes a ZIP and a folder for reading its reports. Keep both outside
the repository you capture. The [recipient guide](for-recipients.md) explains
what the results mean.

## Install

On Linux, macOS or Git Bash on Windows:

```sh
curl -fsSL https://raw.githubusercontent.com/vincentsch/signetleaf/main/install.sh -o /tmp/signetleaf-install.sh
bash /tmp/signetleaf-install.sh
```

WSL uses Linux assets. The installer checks `checksums.txt` and prints an absolute
`Installed:` path. Use that path if `signetleaf` is not on PATH. `INSTALL_DIR`
selects a different directory. To select a published version:

```sh
SIGNETLEAF_VERSION=v0.1.0 bash /tmp/signetleaf-install.sh
```

Remove `/tmp/signetleaf-install.sh` when finished with the installer. If Go is available,
this is an optional source fallback:

```sh
go install github.com/vincentsch/signetleaf/cmd/signetleaf@latest
go env GOBIN GOPATH
```

Use GOBIN when set, otherwise the first GOPATH entry's `bin` directory.

## Native Windows

PowerShell can download the executable without Go. Set `$arch` to `arm64` for
Windows on ARM. This example uses a new directory under your temporary folder.

```powershell
$arch = 'amd64'
$asset = "signetleaf-windows-$arch.exe"
$base = 'https://github.com/vincentsch/signetleaf/releases/latest/download'
$dir = Join-Path ([IO.Path]::GetTempPath()) ([guid]::NewGuid().ToString())
New-Item -ItemType Directory -Path $dir | Out-Null
$exe = Join-Path $dir $asset
Invoke-WebRequest "$base/$asset" -OutFile $exe
Invoke-WebRequest "$base/checksums.txt" -OutFile (Join-Path $dir 'checksums.txt')
$line = Get-Content (Join-Path $dir 'checksums.txt') | Where-Object { ($_ -split '\s+')[-1] -eq $asset }
if (@($line).Count -ne 1) { throw 'Expected one checksum entry' }
$expected = ($line -split '\s+')[0]
if ((Get-FileHash $exe -Algorithm SHA256).Hash -ine $expected) { throw 'Checksum mismatch' }
$exe = (Resolve-Path $exe).Path
Write-Output "Installed: $exe"
& $exe --version
& $exe doctor
```

Keep `$exe` for later commands and use `& $exe capture ...`. The checksum file
comes from the same release; it detects download corruption and does not replace
checking the source of the download.

## Check prerequisites

After installing, the examples use `signetleaf` for the executable:

```sh
signetleaf doctor
```

Web capture and demo need Chrome or Chromium. Code capture and demo need Git.
Doctor explains missing programs and checks output access and TSA connections.
Its connection probes send no timestamp requests and do not honour proxies.
Verify, stored-report extraction and snapshot restore need neither Chrome nor
Git. Checking out history needs Git. See [AGENTS.md](../AGENTS.md) for OS-specific
Chrome installation suggestions; system changes need the user's authority.

## Try a bundled example

Run from a directory whose parent may receive output:

```sh
signetleaf demo --out ../bakery-demo
```

Read the printed report path. Demo also writes a tampered ZIP; verify names the
changed screenshot. If online timestamping fails, the demo uses a local token labelled
“Demo timestamp, not trusted”; verify exits 11. The signing key is public.
Even if a supplied root grants this token trust, it gives no independent
time evidence.

## Capture your project

From the root of your committed Git repository, replace the URL with one you
have permission to capture. Check that the resolved destination is outside the
repository. A relative `--out` is resolved from your current working directory.

```sh
OUT=$(cd .. && pwd -P)
printf 'Output directory: %s\n' "$OUT"
signetleaf capture https://staging.example.com . --out "$OUT" --dry-run
signetleaf capture https://staging.example.com . --out "$OUT" --confirm --no-prompt
signetleaf verify package.signetleaf.zip
```

Replace `package.signetleaf.zip` with the printed ZIP path. Inspect the full plan
before capturing; the CLI plan does not print the output directory. For a
specific output directory, supply its absolute path in both capture commands.
The dry run starts no browser, makes no network requests and writes nothing.
For authentication, additional pages and exclusions, see [capture](capture.md).

## Try the instructions without an external site

Source contributors can use the fixture site, including its basic-auth routes.
From a separate Signetleaf source checkout, build both helpers:

```sh
go build -o /tmp/signetleaf-testsite ./internal/testsite/cmd/testsite
go build -o /tmp/signetleaf-testtsa ./internal/tsa/testtsa/cmd/testtsa
```

Create temporary directories and keep the servers in this shell:

```sh
SITE_DIR=$(mktemp -d)
TSA_DIR=$(mktemp -d)
/tmp/signetleaf-testsite --out "$SITE_DIR" &
SITE_PID=$!
/tmp/signetleaf-testtsa --out "$TSA_DIR" &
TSA_PID=$!
```

Wait until each directory contains `endpoint`, then read those files. The TSA
also writes its public `root.pem`; its private keys stay in memory.

```sh
SITE=$(cat "$SITE_DIR/endpoint")
TSA=$(cat "$TSA_DIR/endpoint")
signetleaf capture "$SITE/" . --out .. --tsa "$TSA" --tsa-root "$TSA_DIR/root.pem" --dry-run
signetleaf capture "$SITE/" . --out .. --tsa "$TSA" --tsa-root "$TSA_DIR/root.pem" --confirm --no-prompt
signetleaf capture "$SITE/private/" --auth client:handover --out .. --tsa "$TSA" --tsa-root "$TSA_DIR/root.pem" --confirm --no-prompt
signetleaf verify package.signetleaf.zip --tsa-root "$TSA_DIR/root.pem"
```

Run these captures from the project being captured. `client:handover` is public
fixture data, never a suggestion for real credentials. The root is independently
supplied by this running fixture, not extracted from the package.
When finished, stop and remove your exact fixture processes and directories:

```sh
kill "$SITE_PID" "$TSA_PID"
wait "$SITE_PID" "$TSA_PID"
rm -rf "$SITE_DIR" "$TSA_DIR"
rm /tmp/signetleaf-testsite /tmp/signetleaf-testtsa
```

[Verification](verify.md) explains integrity and trust. Inspect source, screenshots
and history for private information before sharing output.
