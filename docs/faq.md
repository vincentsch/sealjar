# Questions about captures

Sealjar is a free, open-source CLI that captures web pages and a Git repository
into one sealed, timestamped ZIP. Short answers, with links to the pages that
go deeper.

## Does Sealjar crawl the website?

No. You list the URLs yourself. Each gets desktop and mobile screenshots, and
`--tablet` adds a third view. One package can hold pages from several hosts and
one Git repository. See [capture](capture.md).

## Do I need Chrome or Git?

Web capture and the demo need Chrome or Chromium. Code capture and the demo need
Git. Verify, report and restore need neither. Checking out a history bundle
needs Git. A code-only capture without Chrome writes HTML reports but no PDFs,
and `report --pdf` can only copy PDFs that exist.

## Does exit 0 mean the website was captured completely?

No. Read Capture, Files and Timestamp separately. Exit 0 means the files match
and at least one timestamp is trusted; capture problems are listed anyway. Exit
10 means changed or invalid content. Exit 11 means no trusted timestamp. The
[verification guide](verify.md) explains the results.

## Can someone change the manifest too?

In a manifest without a timestamp, yes. Hashes alone cannot vouch for it,
because files and hashes could be replaced together. A trusted timestamp binds
the exact manifest bytes to a time. With matching hashes, it shows that the
sealed files existed no later than that time. It does not check when the
capture was made.

A package cannot show that its content is true or complete. It cannot show that
the operator was honest or that a server sent the saved bytes. It cannot
show that a project was delivered or accepted, and it cannot link the captured
code to the website. Sealjar promises no legal effect and no admissibility. For
an important dispute, have a neutral person present during the capture, for
example an expert or a notary.

## Is the seal page itself authenticated?

No. `SEAL.html` is written after timestamping and is not in the manifest.
Verify checks the tokens directly, and `sealjar report` writes a fresh seal
page from checked data. The [package format](package-format.md) explains the
sealing order.

## Is every valid timestamp trusted?

No. Verify trusts the system's root certificates, the FreeTSA root that ships
with Sealjar, and roots you supply yourself. Certificates that only come inside
the package are not trusted. An offline check cannot see a later revocation.
Sealjar v0.1 does not assess eIDAS qualification and never calls a timestamp
qualified. The demo's local token is labelled untrusted. Its key is public, and it says nothing about time, even if you supply a root that trusts it.

## Does capture remove secrets?

No. Exclusions and masking reduce what gets in, but they cannot remove all
private content. Screenshots and HTML can show it, and source files and Git
history can contain it. Look through a package before you share it. Capture
contacts the listed pages, DNS servers and the timestamp authority, which
receives a hash and normal network metadata. Timestamp requests use the
standard proxy variables; doctor's connection probes do not.

## Can I retry a timestamp or recover the code?

Yes. [Stamp](timestamps.md#add-another-token-later) adds tokens in a new ZIP
without changing the manifest or the payload.
[Restore](package-format.md#read-or-restore-a-copy) writes the checked snapshot
files into a new or empty folder. The history stays a separate bundle and can
contain old secrets and unrelated work.

## Why keep the reader folder together?

The HTML report links to its images relatively. Moved on its own, it shows no images. Use the PDF when you want one file. The ZIP stays the complete package
for checking. [Recipients](for-recipients.md) can check it with Sealjar or with
Python 3 and OpenSSL.
