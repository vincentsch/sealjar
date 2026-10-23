# Make your first capture

Sealjar writes a ZIP and a folder for reading its reports. Keep both outside the
repository you capture. The [recipient guide](for-recipients.md) explains what the
results mean.

## Install

On Linux, macOS or Git Bash on Windows:

```sh
curl -fsSL https://raw.githubusercontent.com/vincentsch/sealjar/main/install.sh -o /tmp/sealjar-install.sh
bash /tmp/sealjar-install.sh
```

WSL uses Linux assets. The installer checks `checksums.txt` and prints an absolute
`Installed:` path. Use that path if `sealjar` is not on PATH. `INSTALL_DIR`
selects a different directory. Remove `/tmp/sealjar-install.sh` when finished.

<details>
<summary>Choose a version or install with Go</summary>

To select a published version, keep the installer script and run:

```sh
SEALJAR_VERSION=v0.1.0 bash /tmp/sealjar-install.sh
```

If Go is available, you can install from source instead:

```sh
go install github.com/vincentsch/sealjar/cmd/sealjar@latest
go env GOBIN GOPATH
```

Use GOBIN when set, otherwise the first GOPATH entry's `bin` directory.

</details>

## Native Windows

<details>
<summary>Download and check the executable with PowerShell</summary>

PowerShell can download the executable without Go. Set `$arch` to `arm64` for
Windows on ARM. This example uses a new directory under your temporary folder.

```powershell
$arch = 'amd64'
$asset = "sealjar-windows-$arch.exe"
$base = 'https://github.com/vincentsch/sealjar/releases/latest/download'
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

Keep `$exe` for later commands and use `& $exe capture ...`. The checksum file comes
from the same release; it detects download corruption and does not replace checking
the source of the download.

</details>

## Check prerequisites

After installing, the examples use `sealjar` for the executable:

```sh
sealjar doctor
```

Web capture and demo need Chrome or Chromium. Code capture and demo need Git.
Doctor reports missing programs. It also checks output access and connections to
timestamp authorities. Its connection probes send no timestamp requests and do not
honour proxies. You can verify packages without Chrome or Git. You can also
extract stored reports and restore snapshot files without either program. Checking
out history needs Git. If Chrome is installed in an unusual location, pass its
executable path with `--chrome`.

## Try a bundled example

[Install Sealjar](#install) first. The demo needs Chrome or Chromium and Git. Run
from a directory whose parent can receive the output:

```sh
sealjar demo --out ../bakery-demo
```

Read the printed report path. Demo also writes a tampered ZIP; verify names the
changed screenshot. If online timestamping fails, the demo uses a local token
labelled “Demo timestamp, not trusted”; verify exits 11. The signing key is public.
Even if a supplied root grants this token trust, it gives no independent time
evidence.

## Capture your project

From the root of your committed Git repository, replace the URL with one you have
permission to capture. Check that the resolved destination is outside the
repository. A relative `--out` is resolved from your current working directory.

```sh
OUT=$(cd .. && pwd -P)
sealjar capture https://staging.example.com . --out "$OUT" --dry-run
```

Check the full plan, including exclusions and the absolute output directory. If it
matches what you want to capture, run:

```sh
sealjar capture https://staging.example.com . --out "$OUT" --confirm --no-prompt
```

Use the ZIP path printed by capture in place of `package.sealjar.zip`:

```sh
sealjar verify package.sealjar.zip
```

For a specific output directory, supply its absolute path in both capture
commands. The dry run starts no browser, makes no network requests and writes
nothing. For authentication, additional pages and exclusions, see
[capture](capture.md).

## Try the instructions without an external site

The [bundled demo](#try-a-bundled-example) runs locally. Source contributors can
[test capture with local fixtures](contributor-testing.md), including basic-auth
routes and a test timestamp authority.

[Verification](verify.md) explains file checks and timestamp trust. Inspect the
captured files for private information before sharing output.
