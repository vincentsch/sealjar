# Signetleaf

Signetleaf keeps the current files from a Git repository in a portable package.
It includes uncommitted edits, records excluded files, checks the package's
hashes, adds independent timestamps and restores the snapshot into a new folder.

This version captures repositories and verifies RFC 3161 timestamps offline.
Website capture and report generation are still being developed.

## Build and capture

Go 1.27.2 and Git are required to build and capture. Go can download the pinned
toolchain automatically. Verify and restore need only the compiled binary.

```sh
go build -o bin/signetleaf ./cmd/signetleaf
bin/signetleaf capture . --dry-run
bin/signetleaf capture . --confirm --out ./captures
```

Capture accepts paths inside one repository. It uses the repository root and
shows a plan before writing. Without `--confirm`, it asks in an interactive
terminal and refuses unattended capture. The repository must have a commit.

Only tracked files are included by default, with their current working-tree
bytes. `--include-untracked` adds files that Git does not ignore. Deleted files
are recorded but are absent from the snapshot. Submodules are listed without
their contents. Untracked nested repositories and linked worktrees are listed
as excluded, with their contents left out. Git LFS files keep their current
bytes; pointers are marked, and Signetleaf never fetches LFS content.

Secret-file patterns exclude `.env`, private keys and similar candidates.
`.env.example` and `.env.dist` are allowed by default. The plan names exclusions,
but exclusions cannot guarantee that no secret is included. Captured source
files may contain private information; Signetleaf does not rewrite them.

`--exclude '*.key'` matches names at any depth. `--exclude 'config/server.key'`
matches from the repository root. A trailing slash, such as `--exclude 'build/'`,
excludes matching folders and their contents. Patterns use Go's `path.Match`;
`**` is rejected. User exclusions also apply to `.env.example` and `.env.dist`.
Symbolic links are never followed; links outside the repository are excluded.

`--history` adds all Git history as a separate bundle. It may contain old
secrets and unrelated work. `--name handover` sets the package name, and
`--label client=Example` records a label. These flags can be combined:

```sh
bin/signetleaf capture . --no-timestamp --confirm --history \
  --include-untracked --exclude 'build/' --name handover --label client=Example
```

## Timestamp and add timestamps later

Capture sends only the manifest’s SHA-256 hash and a fresh nonce to DigiCert,
then tries FreeTSA if DigiCert returns no usable token. Repeat `--tsa <url>` to
replace those defaults and request every supplied authority. Requests honor
`HTTP_PROXY`, `HTTPS_PROXY` and `NO_PROXY`, with a ten-second deadline per
authority, including proxy connection time, and a 1 MiB response limit.

If every authority fails, capture still writes the package and explains how to
add a timestamp later. `--strict` makes that completed capture exit 10; otherwise
it exits 0. Successful fallback or partial success counts as success. A valid
token with an unknown trust anchor is retained as untrusted. `--no-timestamp`
deliberately skips requests and does not trigger strict failure.

```sh
bin/signetleaf capture . --confirm --tsa https://freetsa.org/tsr
bin/signetleaf stamp capture.signetleaf.zip --dry-run
bin/signetleaf stamp capture.signetleaf.zip
bin/signetleaf verify capture-stamped.signetleaf.zip
bin/signetleaf doctor --tsa https://freetsa.org/tsr
```

Stamp checks integrity first and writes a new `-stamped.signetleaf.zip` beside
the input. It preserves the manifest, payload and every existing token byte for
byte, including when adding to an already trusted package. Token names and
package names get numeric suffixes on collision. If no authority returns a
usable token, stamp exits 2 and writes no package.

Trust comes from system roots, the embedded FreeTSA root, and optional PEM roots
added with `--tsa-root roots.pem` on capture, stamp, verify, restore and report.
Certificates inside the package alone never supply trust anchors. Verification
checks certificate validity at the token’s `genTime`, rather than today. No
network connection is needed for verification. `VERIFY.txt` includes manual
OpenSSL commands with `-attime` for the same historical validity check.

Doctor currently checks timestamp endpoint connections without sending a
request; a connection does not prove issuance would succeed. Report copies
stored report artifacts with trust results, but this version’s captures do not
yet generate those artifacts and report returns exit 5 when they are absent.

## Check and restore

Use the package path printed by capture:

```sh
bin/signetleaf verify capture.signetleaf.zip
bin/signetleaf verify capture.signetleaf.zip --json
bin/signetleaf restore capture.signetleaf.zip --to recovered --dry-run
bin/signetleaf restore capture.signetleaf.zip --to recovered
```

Verify gives separate capture, file and timestamp answers, including each
token’s authority, time, serial, accuracy and trust reason. Matching files with
at least one trusted timestamp exit **0**. No trusted timestamp exits **11**.
Changed, missing or unexpected files, malformed tokens, bad signatures and
manifest imprint mismatches exit **10**, even if another token is trusted.
Missing or mismatched signer certificates, unsuitable certificate purposes and
untrusted chains cannot establish trust. Missing input exits 5,
invalid arguments exit 1, and unsupported formats or operational failures exit
2. Completed JSON verdicts contain one result on stdout, including at exits
10 and 11, and repeated checks return identical bytes.

Restore accepts an intact untimestamped package and checks its contents before
writing. It restores file bytes, permission modes, modification times and safe
links into a new or empty folder. History stays beside it as
`recovered.history.bundle`, with a printed `git clone` command for a separate
checkout. An existing history sidecar or non-empty destination is refused.

The ZIP contains `manifest.json`, complete timestamp responses as
`manifest.json.<authority>.tsr`, bilingual `VERIFY.txt` instructions, and the
repository's `git.json`, `files.json`, `snapshot.tar.gz` and optional
`history.bundle`. Names get `-2`, `-3` and further suffixes on collision. A
reserved reader-folder name also causes a new suffix. Every command that writes
files supports `--dry-run`, without creating files or contacting the network.

Hashes alone do not establish an untimestamped manifest's authenticity. Someone
could replace a file and its hash together. A trusted timestamp can establish
that files existed no later than its time; it does not independently confirm
capture time. Offline checks cannot see later certificate revocation. Signetleaf
does not assess eIDAS qualification. The package does not establish truthful content, honest operation,
delivery, acceptance, or that code produced a website. For important disputes,
consider having a neutral expert or notary present during capture.

## Development

```sh
make check
make conformance
```

The checks use local fixtures and command-local Git identities. They do not
change Git configuration or contact timestamp services. `make conformance` uses
an ephemeral local test authority and trusted verification as its successful
read probe. Test authority keys stay in memory.

## License

MIT
