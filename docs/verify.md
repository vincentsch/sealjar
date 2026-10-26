# Check a package

Verify the received ZIP:

```sh
sealjar verify package.sealjar.zip
sealjar verify package.sealjar.zip --json
sealjar verify package.sealjar.zip --tsa-root roots.pem
```

Obtain extra roots independently, never from package certificates alone. The three
answers mean different things:

| Answer | What to read |
| --- | --- |
| Capture | Complete, Problems or Failed, with target and quality details recorded during capture. |
| Files | Unchanged means listed files match the manifest; changed, missing and unexpected paths are named. |
| Timestamp | Trusted means a token's imprint, signature and certificate chain pass against trusted roots at its time. |

Exit 0 means matching files and at least one trusted timestamp. It does not mean the
capture was complete. Exit 10 means changed, malformed or invalid content, including
an imprint mismatch or invalid signature. Otherwise exit 11 means no trusted
timestamp, including no timestamp at all. Missing input is exit 5; unsupported
formats and operational failures return 2. Usage errors return 1. All available
answers are printed, including on exits 10 and 11.

Preserve the original ZIP and extracted files. Do not repair captured files to make
them pass. Request another capture or investigate the difference separately.

The demo fallback uses a public signing key. It gives no independent time evidence,
even if an explicitly supplied root grants the token trust.

## Without installing Sealjar

Independent checks use Python 3 for file hashes and OpenSSL for timestamp tokens.
Extract a copy of the ZIP into a new empty directory with an archive tool that rejects unsafe
paths and links. Keep the original ZIP. Work from the extracted package root. The
following is the Python file check from `VERIFY.txt`:

```python
from pathlib import Path
import hashlib
import json
import sys

manifest = json.loads(Path("manifest.json").read_text(encoding="utf-8"))
expected = {item["path"]: item for item in manifest["files"]}
problems = []
for name, item in expected.items():
    file = Path(name)
    if not file.is_file() or file.is_symlink():
        problems.append("missing: " + name)
        continue
    digest = hashlib.sha256()
    with file.open("rb") as source:
        for chunk in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(chunk)
    if file.stat().st_size != item["size"] or digest.hexdigest() != item["sha256"]:
        problems.append("changed: " + name)
for file in sorted(Path(".").rglob("*")):
    if file.is_dir() and not file.is_symlink():
        continue
    name = file.as_posix()
    token = "/" not in name and name.startswith("manifest.json.") and name.endswith(".tsr")
    if name not in expected and name not in ("manifest.json", "SEAL.html") and not token:
        problems.append("unexpected: " + name)
print("\n".join(problems) if problems else f"All {len(expected)} files match.")
sys.exit(1 if problems else 0)
```

Exit 0 means consistency with the file list; exit 1 names differences.
`manifest.json`, root-level timestamp tokens and `SEAL.html` are outside that list.
Someone could replace both files and hashes in an untimestamped manifest. Matching
hashes alone cannot authenticate it.

Choose an actual token path for `token.tsr`. Read its UTC genTime:

```sh
openssl ts -reply -in token.tsr -text
```

Convert the displayed time to Unix seconds. Replace the example value with the
token's own “Time stamp” value, without the label:

```sh
python3 - <<'PYTIME'
from datetime import datetime, timezone
import re
gen_time = "Oct 20 18:45:00 2026 GMT"  # replace with the token's UTC time
gen_time = re.sub(r'(\d{2}:\d{2}:\d{2})\.\d+', r'\1', gen_time)
print(int(datetime.strptime(gen_time, "%b %d %H:%M:%S %Y GMT")
          .replace(tzinfo=timezone.utc).timestamp()))
PYTIME
```

Obtain `roots.pem` independently from a trusted OS export, the FreeTSA root in
Sealjar source, or the authority's separately trusted certificate source. Use the
number above in place of `GEN_TIME_UNIX_SECONDS`:

```sh
openssl ts -verify -data manifest.json -in token.tsr -CAfile roots.pem -attime GEN_TIME_UNIX_SECONDS
```

Some authorities also require intermediate certificates; follow their guidance. The
`-attime` check uses certificate validity at the token's genTime. Without it,
OpenSSL uses today's date, which can reject an older valid timestamp. Reading a
token with `-text` alone does not verify it.

## What a successful check establishes

A trusted timestamp authenticates the exact manifest bytes. Together with matching
file hashes, it shows that the sealed files existed by the time it records. The
operator's recorded start and end times are not independently checked capture
times. Trust comes from system root certificates or the embedded FreeTSA root. You
can also supply root certificates obtained independently. Certificates carried
only in the package cannot grant trust. Offline checks cannot see later
revocation. Sealjar v0.1 does not assess eIDAS qualification or label a timestamp
qualified.

A package cannot show whether its content is true or complete. It cannot establish
that the operator was honest or that a server sent the saved bytes. It also cannot
establish delivery or acceptance of a project, or link the captured code to the
website. It carries no promise of legal effect or admissibility. For important
disputes, consider a neutral expert or notary present during capture.

See [timestamps](timestamps.md), [package format](package-format.md) and the
[recipient guide](for-recipients.md).
