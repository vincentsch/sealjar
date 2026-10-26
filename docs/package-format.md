# What is in the package?

The ZIP is the complete package. Capture also creates a paired reader folder beside
it, named like the ZIP without `.zip`. Keep that folder together: HTML reports use
relative image links. A report PDF is a standalone reading copy. The report directs
readers to `SEAL.html` and `verify` for timestamp results.

```text
manifest.json
manifest.json.<authority>.tsr
SEAL.html
VERIFY.txt
report/report.en.html   report/report.de.html
report/report.en.pdf    report/report.de.pdf
web/<position>-<page>/desktop.png  mobile.png  [tablet.png]
web/<position>-<page>/page.pdf  dom.html  *.thumb.jpg
web/<position>-<page>/network.json  headers.json  console.json  redirects.json
hosts/<origin-id>/tls.pem  tls.json  dns.json
repo/<name>/git.json  files.json  snapshot.tar.gz  [history.bundle]
```

PDFs are present only when generated during capture. HTTP hosts have no TLS PEM. The
reader folder contains reports, the seal, checking instructions and linked images,
rather than all raw payloads. `report` can recreate it from the ZIP.

## Manifest fields

`manifest.json` is UTF-8 JSON with `type: sealjar.manifest/v1`. Its [committed
example](../internal/pkg/testdata/manifest.example.json) fixes field names. Readers
must preserve the original bytes when checking tokens.

| Field | Meaning |
| --- | --- |
| `tool_version`, `capture_id` | Tool version and capture identifier. |
| `started`, `ended`, `time_zone` | Operator-recorded UTC times and local IANA time zone. |
| `labels`, `settings` | Optional labels and capture choices, including authentication interaction and viewports. |
| `targets` | Repository or web targets, states, payload paths, commits and quality details. |
| `environment` | OS, architecture and tool/browser environment. |
| `hosts` | Visited origins and their SHA-256 origin identifiers, linking the separate TLS/DNS payloads. |
| `timestamp_authorities` | Authorities requested, rather than a promise of acquired or trusted tokens. |
| `files` | Relative payload paths, sizes and SHA-256 hashes. |

Repository `git.json` records the commit and working-tree state, including deleted
paths. `files.json` lists each selected source path with its file mode and hash. It
also records link targets. `snapshot.tar.gz` stores selected working-tree files. LFS
pointers remain pointers. An optional history bundle is separate from the snapshot
and may contain old secrets.

## Sealing order

Capture writes payloads first, then reports and `VERIFY.txt`, then hashes these
files into the manifest. It requests RFC 3161 timestamps over the manifest hash.
Only then does it write `SEAL.html` and the ZIP. The manifest, root-level timestamp
tokens and seal page are excluded from `manifest.files` to avoid circular hashes.
Report PDFs are covered when present; the seal itself is not.

The seal is a reading aid whose trust text reflects the check when it was made.
`verify` checks token bytes directly. Report extraction regenerates the seal from
checked data and current roots, rather than trusting stored seal HTML. `stamp` adds
tokens to a new ZIP without changing manifest or payload bytes.

Matching hashes alone show agreement with a list. An untimestamped manifest can be
replaced along with its files. A trusted timestamp shows that the sealed files
existed by the time it records. It does not independently check when capture
happened. Read [verification](verify.md) for trust, manual checks and limits.

## Read or restore a copy

Use a new or empty destination:

```sh
sealjar report package.sealjar.zip --out ../reader
sealjar report package.sealjar.zip --lang de --pdf --out ../reader-de
sealjar restore package.sealjar.zip --to ../recovered
```

`report --pdf` copies stored PDFs; it cannot generate missing ones. Report and
restore need neither Chrome nor Git. Restore checks the package before writing and
refuses a nonempty destination. Safe snapshot links are created last. If history
exists, restore writes `recovered.history.bundle` beside the folder. Git is needed
only to check out that bundle, in a separate directory:

```sh
git clone ../recovered.history.bundle ../history-checkout
```

Inspect packages for private content before sharing or checking out history.