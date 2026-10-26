# Check a package

Sealjar is a free, open-source CLI that captures web pages and code into a
sealed ZIP. `sealjar verify` checks such a package offline and gives three
separate answers. If you received a package and do not want to install
Sealjar, you can check it [with Python 3 and OpenSSL](#without-installing-sealjar)
instead.

```sh
sealjar verify package.sealjar.zip
sealjar verify package.sealjar.zip --json
sealjar verify package.sealjar.zip --tsa-root roots.pem
```

Get extra roots from a source you trust, never from the certificates inside the
package. The three answers:

| Answer | What to read |
| --- | --- |
| Capture | Complete, Problems or Failed, with the targets and the quality problems recorded during capture. |
| Files | Unchanged means every listed file matches the manifest. Changed, missing and unexpected files are named. |
| Timestamp | Trusted means the token's hash, signature and certificate chain check out against trusted roots at the token's time. |

Exit 0 means the files match and at least one timestamp is trusted. It does not
say the capture was complete. Exit 10 means a file changed, is malformed or is
otherwise invalid, including a token whose hash does not match the manifest or
whose signature is bad. Exit 11 means no trusted timestamp, including no
timestamp at all. Exit 5 means the input file is missing. Exit 2 means an
unsupported format or a failure while checking. Exit 1 is a usage error. Verify
prints every answer it could compute, also on exits 10 and 11.

Keep the original ZIP and the extracted files as they are. Do not repair
captured files to get a clean result. Ask for a new capture or look into the
difference separately. The demo's local token is signed with a public key and
says nothing about time, even if you supply a root that trusts it.

## Without installing Sealjar

Python 3 checks the file hashes and OpenSSL checks the timestamp token. Extract
a copy of the ZIP into a new, empty directory with an archive tool that rejects
unsafe paths and links. Keep the original ZIP. Run the following from the
extracted package root. It is the same Python check as in `VERIFY.txt`:

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

Exit 0 means every listed file matches; exit 1 names the differences.
`manifest.json`, the timestamp tokens next to it and `SEAL.html` are not in the
list. In a manifest without a timestamp, someone could have replaced files and
hashes together, so matching hashes alone cannot vouch for it.

Pick one of the token files for `token.tsr` and read its genTime in UTC:

```sh
openssl ts -reply -in token.tsr -text
```

Convert that time to Unix seconds. Replace the example with the token's own
“Time stamp” value, without the label:

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

Get `roots.pem` from a source you trust. That can be an export of your
operating system's roots or the FreeTSA root in Sealjar's source. The
authority's own certificate works too, if you got it from a place you trust.
Put the number from above in place of `GEN_TIME_UNIX_SECONDS`:

```sh
openssl ts -verify -data manifest.json -in token.tsr -CAfile roots.pem -attime GEN_TIME_UNIX_SECONDS
```

Some authorities also need their intermediate certificates; follow their
instructions. `-attime` checks the certificates as of the token's genTime.
Without it, OpenSSL uses today's date and can reject an older timestamp that was
valid. Printing a token with `-text` does not check it.

## What a successful check establishes

A trusted timestamp vouches for the exact bytes of the manifest. Together with
matching hashes, it shows that the sealed files existed no later than the
timestamp's time. The start and end times in the manifest come from the
operator's computer and are not checked independently. Verify trusts the
system's root certificates, the FreeTSA root that ships with Sealjar, and roots you supply yourself. Certificates that only come inside the package are not
trusted. An offline check cannot see a later revocation. Sealjar v0.1 does not
assess eIDAS qualification and never calls a timestamp qualified.

A package cannot show that its content is true or complete. It cannot show that
the operator was honest or that a server sent the saved bytes. It cannot
show that a project was delivered or accepted, and it cannot link the captured
code to the website. Sealjar promises no legal effect and no admissibility. For
an important dispute, have a neutral person present during the capture, for
example an expert or a notary.

See also [timestamps](timestamps.md), the [package format](package-format.md)
and the [recipient guide](for-recipients.md).
