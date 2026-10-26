# What is in the package?

Sealjar is a free, open-source CLI that captures web pages and a Git repository
into one sealed ZIP. This page shows what is in that ZIP, how it is sealed and
how to get the reports and the code out again.

The ZIP is the complete package. Beside it, capture writes a reader folder with
the same name without `.zip`. The folder holds the reports with their images.
Keep it together, because the HTML reports link to the images relatively. The
PDF report stands on its own. The report points to `SEAL.html` and to `verify`
for the timestamp result.

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

PDFs exist only when they were generated during capture. HTTP hosts have no
`tls.pem`. The reader folder holds the reports, the seal page, `VERIFY.txt` and
the linked images, not the raw payloads. `sealjar report` can write it again
from the ZIP.

## Manifest fields

`manifest.json` is UTF-8 JSON with `type: sealjar.manifest/v1`. The
[committed example](../internal/pkg/testdata/manifest.example.json) fixes the
field names. Keep the file's bytes unchanged when you check a token against it.

| Field | Meaning |
| --- | --- |
| `tool_version`, `capture_id` | Tool version and capture identifier. |
| `started`, `ended`, `time_zone` | Start and end in UTC as recorded by the operator's computer, and the local IANA time zone. |
| `labels`, `settings` | Optional labels and the capture settings, including login interaction and viewports. |
| `targets` | Each repository or web target with its state, payload paths, commit and quality details. |
| `environment` | Operating system, architecture, tool and browser. |
| `hosts` | Visited origins with their SHA-256 origin identifiers, which name the TLS and DNS payloads. |
| `timestamp_authorities` | The authorities that were asked. This is not a promise that a token was received or is trusted. |
| `files` | Relative payload paths with sizes and SHA-256 hashes. |

`git.json` records the commit and the working-tree state, including deleted
paths. `files.json` lists every selected source file with its mode and hash,
and the target of every link. `snapshot.tar.gz` holds the files themselves. LFS
pointers stay pointers. The history bundle, when present, is separate from the
snapshot and can contain old secrets.

## Sealing order

Capture writes the payload files first, then the reports and `VERIFY.txt`. It
hashes all of them into the manifest and asks the timestamp authorities for
RFC 3161 tokens over the manifest hash. Only then does it write `SEAL.html` and
the ZIP. The manifest, the tokens next to it and the seal page are not in
`manifest.files`, because a file cannot contain its own hash. Report PDFs are
covered when present; the seal page is not.

The seal page is a reading aid, and its trust text reflects the check at the
time it was written. `verify` checks the token bytes directly. `sealjar report`
writes a fresh seal page from checked data and the current roots instead of
copying the stored one. `stamp` adds tokens in a new ZIP without changing the
manifest or the payload.

Matching hashes show that the files agree with the list. A manifest without a
timestamp could be replaced together with its files. A trusted timestamp shows
that the sealed files existed no later than its time. It does not check when
the capture was made. [Verification](verify.md) covers trust, manual checks
and limits.

## Read or restore a copy

Use a new or empty destination:

```sh
sealjar report package.sealjar.zip --out ../reader
sealjar report package.sealjar.zip --lang de --pdf --out ../reader-de
sealjar restore package.sealjar.zip --to ../recovered
```

`report --pdf` copies the stored PDFs; it cannot generate missing ones. Report
and restore need neither Chrome nor Git. Restore checks the package before it
writes and refuses a destination that is not empty. Safe symlinks from the
snapshot are created last. If the package has history, restore writes
`recovered.history.bundle` next to the folder. Git is needed only to check out
that bundle, in a separate directory:

```sh
git clone ../recovered.history.bundle ../history-checkout
```

Look through a package for private content before you share it or check out
its history.
