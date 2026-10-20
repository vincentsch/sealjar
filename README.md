# Signetleaf

Signetleaf captures web pages and current Git files in a portable package.
It records capture problems, checks file hashes, adds independent timestamps
and restores repository snapshots into a new folder.

Capture writes English and German HTML reports and PDFs, a bilingual seal page,
and a reader folder beside the ZIP. Timestamp checks work offline.

## Build and capture

Go 1.27.2 builds the binary and can download the pinned toolchain automatically.
Repository capture needs Git. Website capture needs installed Chrome or Chromium.
Verify and restore need only the compiled binary.

```sh
go build -o bin/signetleaf ./cmd/signetleaf
bin/signetleaf capture . --dry-run
bin/signetleaf capture . --confirm --out ./captures
```

Capture accepts HTTP(S) URLs, paths inside one repository, or both. It uses the repository root and
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

Doctor checks Chrome's path and version in a temporary isolated browser,
Git, output write access and configured
TSA connections. Each failed check includes a fix. HTTPS checks include the TLS
handshake. No timestamp request is sent, and reachability does not prove issuance.
Report copies stored reports, but captures do not yet generate them. Report
returns exit 5 when these artifacts are absent.

## Try the bundled demo

```sh
bin/signetleaf demo
bin/signetleaf demo --out ./bakery-demo --json
```

The demo captures two local bakery pages and a Git repository, then verifies
the package. It prints the ZIP path, reader folder and report path. Open the
report in that folder to read it locally. Chrome and Git are required.

Default timestamp authorities are tried first. Without external network access,
the demo uses deliberately public test material and says “Demo timestamp, not
trusted”. This never adds a trusted root. Demo still exits 0; ordinary verify
on that package exits 11. Explicit `--tsa-root` trust is shown separately from
the token's demo provenance.

The printed verify command checks `demo-tampered.signetleaf.zip`. This readable
ZIP has one changed screenshot; verify exits 10 and names it. Repeated demos
keep earlier artifacts and add `-2`, `-3` and further suffixes to occupied names.
`--dry-run` starts no services and writes nothing. `--json` returns one document
with capture details, verification answers, the verify exit code and actual paths.

## Capture web pages

```sh
bin/signetleaf capture https://example.com --dry-run
bin/signetleaf capture https://example.com . --confirm --out ./captures
bin/signetleaf capture https://staging.example.com/account/ --login --confirm
bin/signetleaf doctor --out ./captures
```

Each URL gets desktop (1440×900) and mobile (390×844, scale 2) captures.
`--tablet` adds 768×1024 at scale 2. Chrome starts with a temporary isolated
profile and closes when capture finishes. Use `--chrome <path>` to choose it.
Signetleaf waits for load and one second without unfinished HTTP requests,
with a shared 30-second limit. `--wait 2s` adds time after that wait, up to 30s.
Scrolling loads lazy content, with a separate ten-second idle limit afterwards.
Cookie banners stay untouched. Screenshots stop at 20,000 CSS px and record
truncation. Desktop capture also saves a PDF and rendered DOM.

Network metadata, document headers, console messages and redirects are stored
per viewport. Failed requests, HTTP errors, visible password forms, mixed
content, truncation and reached wait limits become capture problems. An
unavailable target is recorded and other targets continue. If any target
succeeds, capture writes a package and normally exits 0. `--strict` exits 10
after writing when any target has problems or failed. All targets failing
exits 2 and writes no package. A captured 401 or 404 page is a problem.

Use `SIGNETLEAF_AUTH=user:password` or `--auth user:password` for basic auth.
Credentials are sent only to explicitly listed target origins, including their
ports. URLs containing user info are rejected. `--login` needs an interactive
terminal: log in in the visible Chrome window, then press Enter in the terminal.
Capture uses the same browser context and tab, keeping session cookies and
session storage. Chrome closes and its temporary profile is removed afterwards.
Ctrl+C cancels capture and removes the temporary browser profile too.

Metadata masks credentials, cookies and known secret query values. Screenshots,
PDFs and DOM can still contain private information; captured content is never
rewritten. Review a package before sharing it. Separate host folders record
system DNS results and TLS chains for each origin, including redirects. These
facts come from separate connections and include metadata lookup failures.

The ZIP stores page files under numbered `web/` folders and host facts under
`hosts/<origin-id>/`, where the ID is the SHA-256 of the canonical origin.
Each screenshot also has a 600 px JPEG thumbnail linked to its full PNG in the report.

## Read and share reports

Capture stores reports at `report/report.en.html`, `report/report.de.html` and
matching `.pdf` paths. Use `--lang en`, `--lang de` or `--lang both` (the default).
PDF generation uses Chrome. A repository capture without Chrome stores HTML
only and explains the missing PDFs before and after capture.

The reader folder is next to the ZIP, with the same name minus `.zip`.
Open `<reader folder>/report/report.en.html` or the German report locally.
The folder includes all report images, available report PDFs, `VERIFY.txt` and
`SEAL.html`. Send the ZIP for verification, or a PDF for a standalone report.

```sh
bin/signetleaf report capture.signetleaf.zip --out reader --lang de --pdf
bin/signetleaf report capture.signetleaf.zip --out html-reader --lang both
```

`report` checks files and timestamp tokens before writing. It copies the
selected stored HTML reports and images. `--pdf` also copies stored PDFs;
extraction needs no Chrome. Without `--out`, it uses the package path minus
`.zip`. A non-empty destination is refused with exit 1. An absent requested
language or PDF returns exit 5; changed or missing authenticated files return
exit 10. No reader output is written in these cases.

The seal page shows the manifest hash and timestamp trust. It is outside the
manifest and serves as a reading aid. Capture, stamp and report generate it
from checked data; report never copies a stored seal page. `--tsa-root` adds
independently obtained trust roots for report extraction too. An intact package
with missing or untrusted timestamps can still be extracted; matching hashes
alone do not establish an untimestamped manifest's authenticity.

Stamp writes a fresh reader folder beside its new ZIP. It refreshes the seal
while preserving reports, payload, manifest and existing tokens byte for byte.

## Check and restore

Use the package path printed by capture:

```sh
bin/signetleaf verify capture.signetleaf.zip
bin/signetleaf verify capture.signetleaf.zip --json
bin/signetleaf restore capture.signetleaf.zip --to recovered --dry-run
bin/signetleaf restore capture.signetleaf.zip --to recovered
```

Verify gives separate capture, file and timestamp answers, with authority, time and trust in normal text. `--json` also gives the full
signer subject, serial, accuracy and trust reason. Matching files with
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
