# Signetleaf for coding agents

If you are changing Signetleaf's source, read [CONTRIBUTING.md](CONTRIBUTING.md).
The instructions below are for capturing another project.

Signetleaf saves listed web pages and current Git files in a timestamped ZIP,
with a reader folder beside it. Use the URLs, repository and capture permission
already supplied. Ask only for missing scope, needed authentication, an unspecified
output destination, or a surprising dry-run result. Never guess private URLs.
Do not clone Signetleaf into the repository being captured. If source is needed,
use a separate temporary checkout and remove it afterwards.

## Install for the current shell

On Linux, macOS or Git Bash on Windows, download and run the public installer:

```sh
curl -fsSL https://raw.githubusercontent.com/vincentsch/signetleaf/main/install.sh -o /tmp/signetleaf-install.sh
bash /tmp/signetleaf-install.sh
rm /tmp/signetleaf-install.sh
```

In Git Bash, use `mktemp` for the script if `/tmp` is unavailable. WSL uses Linux
assets and the Linux installer. The installer checks the download against
`checksums.txt`. Use the absolute executable path on its `Installed:` line.
Do not assume the install directory is already on PATH or edit shell profiles.

On native Windows, follow the executable download and `Get-FileHash` instructions
in [quickstart](docs/quickstart.md#native-windows). Go is not required.
Use the absolute path printed by that step and PowerShell's `&` call operator.

If Go is available, an optional source fallback is:

```sh
go install github.com/vincentsch/signetleaf/cmd/signetleaf@latest
go env GOBIN GOPATH
```

Resolve the executable explicitly: use GOBIN if nonempty, otherwise the first
GOPATH entry's `bin` directory. On Windows its name is `signetleaf.exe`.
For the remaining shell examples set `SIGNETLEAF` to that absolute path:

```sh
SIGNETLEAF=/absolute/path/to/signetleaf
"$SIGNETLEAF" doctor
```

Explain unmet prerequisites before capturing. Chrome/Chromium is needed for web
capture and demo; Git is needed for code capture and demo. Suggest an appropriate
Chrome command, such as `sudo apt install chromium-browser` on Ubuntu where that
package is available, `sudo apt install chromium` on Debian,
`brew install --cask google-chrome` on macOS, or
`winget install --id Google.Chrome --exact` on Windows. Package availability
varies by distribution. In WSL use a Linux Chromium installation, or pass
`--chrome` to a compatible Linux executable. Honour existing authority for
system changes; do not install system packages without that authority.
Verify, stored-report extraction and snapshot restore need neither Chrome nor
Git. A history bundle needs Git to check it out. Repository-only capture can
write HTML without Chrome, but cannot make report PDFs.

## Show the plan, then capture

Resolve the output destination from the actual working directory. Check that
it is outside the captured repository, including after resolving links. Running
from the repository root, `--out ..` is one option. Use a destination the user
supplied when available. Show its resolved absolute path alongside the complete
plan; the CLI does not currently print the output directory in its plan.

```sh
OUT=$(cd .. && pwd -P)
printf 'Output directory: %s\n' "$OUT"
"$SIGNETLEAF" capture https://staging.example.com . --out "$OUT" --dry-run
"$SIGNETLEAF" capture https://staging.example.com . --out "$OUT" --confirm --no-prompt
```

Replace the example URL with the supplied URLs, and preserve supplied exclusions,
authorities and roots in both commands. Inspect targets, file counts, exclusions
and timestamp settings. Continue within the existing capture permission unless
something surprising needs clarification. Never silently broaden the scope.
URLs are listed individually; Signetleaf does not crawl links.

Use `SIGNETLEAF_AUTH` for basic auth when needed; never print or save real
credentials. Manual `--login` requires an interactive user to log in and press
Enter. It cannot be completed unattended. Do not change the user's browser
profile; capture uses an isolated temporary Chrome profile.

## Check and show the result

```sh
"$SIGNETLEAF" verify /absolute/path/to/package.signetleaf.zip
```

Preserve any supplied `--tsa-root` here too. Interpret all three answers:

- Capture says whether targets completed or had problems. Read those problems
  even after exit 0; verify's exit code does not grade capture quality.
- Files says whether listed files match the manifest. Exit 10 means changed,
  malformed or otherwise invalid content. Name affected paths; never repair them.
- Timestamp says whether a token is trusted. Exit 11 means no trusted timestamp,
  including an untimestamped package or the labelled demo fallback. Exit 0 means
  files match and at least one timestamp is trusted, subject to capture notes.

Show the ZIP path, paired reader-folder path and `report/report.en.html` (or
German report). Open the report when the environment allows it and the user
has not excluded GUI use. Otherwise print its path. Explain capture problems.
Keep the captured files and original ZIP unchanged. Never send a package anywhere.

The demo fallback uses a public signing key. It gives no independent time
evidence, even if an explicitly supplied root grants the token trust.

Matching hashes alone cannot authenticate an untimestamped manifest. A trusted
timestamp gives an upper bound for when sealed files existed, not an independently
checked capture time. It cannot establish truthful or complete content, honest
operation, that a server sent the bytes, delivery, acceptance, or that captured
code produced the website. For important disputes recommend a neutral expert
or notary during capture. Do not promise legal effect or admissibility.

Trust comes from system roots, the embedded FreeTSA root or independently supplied
explicit roots, never certificates merely carried in the package. Offline checks
cannot see later revocation. Signetleaf v0.1 does not assess eIDAS qualification
or label timestamps qualified.

Capture contacts target pages and resources, DNS and timestamp authorities.
A TSA receives a hash and normal network metadata. Timestamp requests use standard
proxy settings; doctor's connection probes do not honour proxies. Metadata masking
and exclusions cannot remove private content from screenshots, DOM, source or
history. Recommend inspecting packages before sharing.
