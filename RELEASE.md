# Release checks

Release preparation is local. Publication and hosted checks require a separate,
authorized release. Prepared automation does not establish a hosted result.

## Prepare a version

Update the matching `## vX.Y.Z` section in `CHANGELOG.md`. Remove its
` - unreleased` suffix before publication. The extractor also accepts that suffix
while preparing a release, and refuses a missing section.

```sh
make lint test race docs-check conformance
make build-all VERSION=vX.Y.Z
(cd dist && sha256sum --check checksums.txt)
python3 scripts/release-notes.py vX.Y.Z
scripts/fake-release.sh python3 scripts/test-install.py
```

On macOS, use `shasum -a 256 -c checksums.txt` for the checksum check.
Inspect all six executables with `file`. Run the binary for the current host
with `--version`, then run its bundled demo and verify the resulting package.
A demo timestamp without an independently trusted root gives verify exit 11.

Review source and built assets for secrets and private paths. The embedded demo
TSA key is public test material. Do not publish captured private data.

The tag-triggered release calls CI first. After CI succeeds, it builds all six
binaries and checksums, attests the binaries, and uploads those seven files.
The release job alone has write and attestation permissions. Release notes come
from the changelog section matching the tag and stop at the next version section.

## Pending checks

This is the shared pending list for release and onboarding work. Check off an item
only after recording its host, command, result and date. Local fixture checks do
not satisfy public-download or hosted checks.

- [ ] Public installer download, latest and pinned, after publishing release assets.
- [ ] Public `go install github.com/vincentsch/signetleaf/cmd/signetleaf@vX.Y.Z`,
  using an empty module cache after publishing a public module tag.
- [ ] Agent prompt fetched from GitHub after publishing its source file.
- [ ] First hosted Linux CI run, including Chrome-required tests and conformance.
- [ ] First hosted macOS CI run.
- [ ] First hosted Windows CI run.
- [ ] Native macOS CLI smoke check, including installation, demo and verification.
- [ ] Native Windows CLI smoke check, including the `.exe`, demo and verification.
- [ ] First hosted release run: confirm CI completes before release work starts,
  a failed CI blocks it, and all six correctly named binaries plus checksums.txt
  are uploaded. Requires a published tag and release run.
- [ ] Hosted provenance verification of all six downloaded binaries with
  `gh attestation verify <binary> --repo vincentsch/signetleaf` after release.
- [ ] Native macOS timestamp trust checks for DigiCert and FreeTSA as described below.

Public downloads, public module installation, prompt fetching and hosted
attestations need published content. Hosted CI needs GitHub runners and source
available there; it does not need a published release. Native smoke and timestamp
checks need the matching host and can use a locally built binary before publication.

## Check timestamp trust on macOS

Use a native macOS host with Chrome or Chromium and Git. Keep the trust gate
pending until both authorities have their own recorded result. Linux results do
not establish macOS trust.

Create a temporary repository and keep captures beside it:

```sh
work=$(mktemp -d /tmp/sl-trust.XXXXXX)
mkdir "$work/repo"
git -C "$work/repo" init
printf 'Timestamp trust check\n' > "$work/repo/example.txt"
git -C "$work/repo" add example.txt
git -C "$work/repo" -c user.name='Trust Check' \
  -c user.email=trust@example.invalid -c commit.gpgsign=false commit -m 'Add example'
signetleaf capture "$work/repo" --confirm --tsa http://timestamp.digicert.com \
  --name digicert --out "$work/captures" --json > "$work/digicert.json"
signetleaf capture "$work/repo" --confirm --tsa https://freetsa.org/tsr \
  --name freetsa --out "$work/captures" --json > "$work/freetsa.json"
```

Explicit `--tsa` prevents fallback from satisfying the DigiCert check with FreeTSA.
Read each JSON result's `package` field for the package path. Run these commands
for each package, replacing placeholders with that authority's paths:

```sh
signetleaf verify <package> --json > <authority>-default.json
printf 'default trust exit: %s\n' "$?"
signetleaf verify <package> --tsa-root <independently-obtained-root.pem> \
  --json > <authority>-explicit-root.json
printf 'explicit root exit: %s\n' "$?"
```

Obtain each root independently from that authority's official certificate source
or a separately trusted system export. Record its source URL or export method,
SHA-256 fingerprint and local path. Never use package certificates as trust anchors.
Inspect each verification result's token authority, timestamp and trust reason.
Record the actual issuing authority and both exit codes separately for DigiCert
and FreeTSA. Missing tokens or tokens from another authority leave that check pending.

Trust remains system roots plus embedded FreeTSA and explicit user roots. If default
trust fails, hold any macOS default-trust claim and document the independent root
path and explicit-root result. Do not silently add roots or change trust policy.
Remove the temporary repository and captures after recording concise results.

## Local documentation checks

The Linux amd64 documentation check on 2026-10-21 used an exact-HEAD local clone
with the intended source changes overlaid and staged. This records a source
snapshot, rather than claiming a clean implementation commit. The local release
fixture built all six assets and installed both its pinned and latest Linux
versions. The quickstart used the installed binary with the ephemeral website
Handler and an independently supplied local TSA root.

Local capture covered multiple pages, tablet views, basic auth, current code,
exclusions and history. Verification reported Complete, Unchanged and Trusted
with the supplied fixture root. The manual Python hash check passed before a
payload edit and named the changed path afterwards. OpenSSL verified the token
with the fixture root at its genTime using `-attime`. A readable tampered ZIP
returned 10 and named the changed screenshot. The demo used blocked public TSA
connections and its labelled untrusted fallback; verification returned 11.
Report and seal review covered 390, 768 and 1440 px and A4, without shared Chrome.

Manual `--login` was not executed because it needs a person. Public installer,
public module install and GitHub prompt fetching remain in the single pending
list above. Native Windows PowerShell download/hash commands and native macOS
execution were not run on this Linux host. Local fixture checks do not establish
public downloads, hosted jobs, provenance attestations or native platform results.
