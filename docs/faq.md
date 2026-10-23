# Questions about captures

## Does Sealjar crawl the website?

No. List the URLs you want. Each gets desktop and mobile screenshots; `--tablet`
adds a third view. Capture can span several hosts alongside one Git repository. See
[capture](capture.md).

## Do I need Chrome or Git?

Web capture and demo need Chrome or Chromium. Code capture and demo need Git.
Verify, stored-report extraction and snapshot restore need neither. Checking out a
history bundle needs Git. Repository-only capture without Chrome produces HTML
reports without PDFs. `report --pdf` only copies stored PDFs.

## Does exit 0 mean the website was captured completely?

No. Read Capture, Files and Timestamp separately. Verify exit 0 means matching files
and at least one trusted timestamp; capture problems stay visible. Exit 10 means
changed or invalid content. Exit 11 means no trusted timestamp. The [verification
guide](verify.md) explains these results.

## Can someone change the manifest too?

Hashes alone cannot authenticate an untimestamped manifest. A trusted timestamp
binds its exact bytes to a time. Together with matching hashes, it shows that the
sealed files existed by that time. It does not independently check when capture
happened. A package cannot show whether its content is true or complete. It cannot
establish that the operator was honest or that a server sent the saved bytes. It
also cannot establish delivery or acceptance of a project, or link the captured code
to the website. No legal effect or admissibility is promised. For important
disputes, consider a neutral expert or notary during capture.

## Is the seal page itself authenticated?

No. `SEAL.html` is created after timestamping and is outside the manifest. Verify
checks tokens directly; report extraction regenerates the seal from checked data.
The [package format](package-format.md) explains sealing order.

## Is every valid timestamp trusted?

No. Trust needs system roots, the embedded FreeTSA root or independently supplied
explicit roots, never certificates merely carried in the package. Offline checks
cannot see later revocation. Sealjar v0.1 does not assess eIDAS qualification or
label timestamps qualified. The demo fallback is explicitly untrusted.

The demo fallback uses a public signing key. It gives no independent time evidence,
even if an explicitly supplied root grants the token trust.

## Does capture remove secrets?

No. Exclusions and metadata masking reduce exposure, but cannot remove all private content. Screenshots and rendered HTML may show it; source files and Git
history may contain it too. Inspect packages before sharing. Capture contacts target
pages and resources, DNS and timestamp authorities. The TSA receives a hash and
normal network metadata. Timestamp requests honour standard proxy settings; doctor's
connection probes do not.

## Can I retry a timestamp or recover the code?

[Stamp](timestamps.md#add-another-token-later) adds tokens to a new ZIP without
changing the manifest or payload.
[Restore](package-format.md#read-or-restore-a-copy) recovers checked snapshot files
into a new or empty folder. History remains a separate bundle and may contain old
secrets and unrelated work.

## Why keep the reader folder together?

HTML image links are relative. Moving the report alone breaks those links. Use the
PDF for a standalone reading copy. The ZIP remains the complete package for
independent verification. [Recipients](for-recipients.md) can check it with Sealjar
or with Python 3 and OpenSSL.