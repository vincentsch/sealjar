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

## Release verification checklist

This is the shared verification list for release and onboarding work. Check off an item
only after recording its host, command, result and date. Local fixture checks do
not satisfy public-download or hosted checks.

- [x] Public installer download, latest and pinned, after publishing release assets.
- [x] Public `go install github.com/vincentsch/sealjar/cmd/sealjar@vX.Y.Z`,
  using an empty module cache after publishing a public module tag.
- [x] Agent prompt fetched from GitHub after publishing its source file.
- [x] First hosted Linux CI run, including Chrome-required tests and conformance.
- [x] First hosted macOS CI run.
- [x] First hosted Windows CI run.
- [x] Native macOS CLI smoke check, including installation, demo and verification.
- [x] Native Windows CLI smoke check, including the `.exe`, demo and verification.
- [x] First hosted release run: confirm CI completes before release work starts,
  the release job depends on successful CI, and all six correctly named binaries plus checksums.txt
  are uploaded. Requires a published tag and release run.
- [x] Hosted provenance verification of all six downloaded binaries with
  `gh attestation verify <binary> --repo vincentsch/sealjar` after release.
- [x] Native macOS timestamp trust checks for DigiCert and FreeTSA as described below.

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
sealjar capture "$work/repo" --confirm --tsa http://timestamp.digicert.com \
  --name digicert --out "$work/captures" --json > "$work/digicert.json"
sealjar capture "$work/repo" --confirm --tsa https://freetsa.org/tsr \
  --name freetsa --out "$work/captures" --json > "$work/freetsa.json"
```

Explicit `--tsa` prevents fallback from satisfying the DigiCert check with FreeTSA.
Read each JSON result's `package` field for the package path. Run these commands
for each package, replacing placeholders with that authority's paths:

```sh
sealjar verify <package> --json > <authority>-default.json
printf 'default trust exit: %s\n' "$?"
sealjar verify <package> --tsa-root <independently-obtained-root.pem> \
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

## Public v0.1.0 verification, 2026-10-23

Published [sealjar v0.1.0](https://github.com/vincentsch/sealjar/releases/tag/v0.1.0)
from `0711fe2fa4a548531d9da2624b9c9aa328be5b82`. The [release workflow](https://github.com/vincentsch/sealjar/actions/runs/39947650187)
passed Linux, macOS and Windows CI before the publishing job started. The release
contains six platform executables and `checksums.txt`. All six downloaded SHA-256
hashes matched. All six GitHub build attestations verified with `gh attestation verify`, the release workflow identity, tag ref and source digest pinned, and
`--deny-self-hosted-runners`.

On Linux amd64, latest and pinned (`v0.1.0`) public installers succeeded in fresh
temporary folders. The installed CLI's version and bundled demo were checked.
`go install github.com/vincentsch/sealjar/cmd/sealjar@v0.1.0` succeeded with an
initially empty module cache, the public Go proxy and checksum database. A separate
empty-cache `go install` with `@latest` also resolved v0.1.0 successfully. The
README agent prompt and `AGENTS.md` were fetched anonymously from GitHub and
matched the published source. Fetching the prompt does not establish a trial
with every supported coding agent.

The [native public-release smoke run](https://github.com/vincentsch/sealjar/actions/runs/39948682481)
passed on Linux amd64, macOS arm64 and Windows amd64: public native downloads,
checksum checks, latest and pinned installation where supported, version and demo
checks. Windows checked the `.exe` hash with PowerShell `Get-FileHash`. This is
native execution coverage for those three hosts; the other three assets were
cross-built and their checksums and provenance verified.

On macOS, DigiCert verification with default roots returned exit 11 (unknown
authority); an independently exported macOS system-root bundle gave exit 0.
FreeTSA gave exit 0 with default and explicit official roots. The default macOS
DigiCert trust claim remains withheld. See [the workaround](docs/timestamps.md#macos-and-digicert).
The Linux public demo verified its real DigiCert timestamp with exit 0; a
tampered package was rejected with exit 10.
