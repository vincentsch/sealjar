# Make your first capture

Sealjar is a free, open-source CLI that captures web pages and a Git repository
into one ZIP, sealed with SHA-256 hashes and a timestamp. It is for developers
and agencies who hand over a website and want a record of the site and the code
on that day. The steps below install Sealjar, run the bundled demo and capture your own project. If someone sent you a package, read the
[recipient guide](for-recipients.md) instead.

## Install

On Linux, on macOS or in Git Bash on Windows, download and run the installer:

```sh
curl -fsSL https://raw.githubusercontent.com/vincentsch/sealjar/main/install.sh -o /tmp/sealjar-install.sh
bash /tmp/sealjar-install.sh
```

WSL gets the Linux build. The installer checks the download against
`checksums.txt` and prints the absolute path on its `Installed:` line. Use that
path if `sealjar` is not on PATH. Set `INSTALL_DIR` to install somewhere else.
Delete `/tmp/sealjar-install.sh` when you are done.

<details>
<summary>Choose a version or install with Go</summary>

To install a specific release, keep the script and run:

```sh
SEALJAR_VERSION=v0.1.0 bash /tmp/sealjar-install.sh
```

With Go installed, you can build from source instead:

```sh
go install github.com/vincentsch/sealjar/cmd/sealjar@latest
go env GOBIN GOPATH
```

The executable lands in GOBIN. If GOBIN is empty, it lands in the `bin`
directory of the first GOPATH entry.

</details>

## Native Windows

<details>
<summary>Download and check the executable with PowerShell</summary>

PowerShell downloads the executable and checks its SHA-256 hash. Go is not
needed. Set `$arch` to `arm64` on Windows on ARM. The example puts the file in a
new folder under your temporary directory.

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

Keep `$exe` for the next commands and call it as `& $exe capture ...`. The
checksum file comes from the same release. It catches a corrupted download, but
it cannot tell you that the download came from the right place.

</details>

## Check prerequisites

```sh
sealjar doctor
```

Doctor checks for Chrome or Chromium and for Git. It also checks that it can
write the output and that the timestamp authorities answer. Those connection
probes send no timestamp request and ignore proxy settings.

Web capture and the demo need Chrome or Chromium. Code capture and the demo need
Git. Verify, report and restore need neither. Checking out a history bundle
needs Git. If Chrome is installed in an unusual place, give its path with `--chrome`.

## Try a bundled example

The demo needs Chrome or Chromium and Git. Run it where the parent directory can
take the output:

```sh
sealjar demo --out ../bakery-demo
```

Open the report at the printed path. The demo also writes a copy of the package
with one changed screenshot. Run `sealjar verify` on that copy and the output
names the file. If no public timestamp authority answers, the demo signs with a
local key and labels the timestamp “Demo timestamp, not trusted”. Verify then
exits 11. The key is public, so that token says nothing about when the files
existed, even if you supply a root that trusts it.

## Capture your project

Run from the root of a Git repository with at least one commit. Replace the URL
with a page you are allowed to capture. The output directory must be outside the
repository, and a relative `--out` is resolved from your current directory.

```sh
OUT=$(cd .. && pwd -P)
sealjar capture https://staging.example.com . --out "$OUT" --dry-run
```

The dry run prints the plan. It starts no browser, makes no network request and
writes nothing. Check which files go in and which are left out, and check the
absolute output directory. If the plan is right, run the capture:

```sh
sealjar capture https://staging.example.com . --out "$OUT" --confirm --no-prompt
```

Capture prints the ZIP path and writes a reader folder with the reports and
screenshots beside it. Put the printed path in place of `package.sealjar.zip`:

```sh
sealjar verify package.sealjar.zip
```

To write somewhere else, use an absolute path with `--out` in both capture
commands. The [capture guide](capture.md) covers logins, more pages and exclusions.

## Try the instructions without an external site

The [bundled demo](#try-a-bundled-example) runs on your own computer. If you
change Sealjar's source, you can
[test capture with the local fixtures](contributor-testing.md). They include
basic-auth routes and a test timestamp authority.

[Verification](verify.md) explains the file check and timestamp trust. Look
through the captured files for private content before you share them.
