# Received a Sealjar package?

Sealjar is a free, open-source CLI that captures web pages and a Git repository
into one ZIP and seals it with SHA-256 hashes and a timestamp. Someone used it
and sent you a package. You can read the report without installing anything.
Checking the package takes Sealjar, or Python 3 and OpenSSL.

The ZIP holds the captured pages and code and the instructions for checking
them. The folder beside it holds the reports. Open `report/report.en.html` in
that folder and keep the folder together, because the images are linked
relatively. The PDF report stands on its own.

Start with the summary. Do the URLs and the repository commit match what you
expected? Which files were left out? Read each capture problem, because a page
that failed to load or a missing resource leaves a gap. The screenshots show the
desktop and mobile views as they loaded, including any cookie banner or login
form. They do not show how the site behaved when someone used it.

The report points to `SEAL.html` for the timestamp result. The seal page is a
reading aid. It was written after the manifest and is not in the manifest's file
list, and its trust text reflects the check at that time. Verify checks the
tokens themselves, and `sealjar report` writes a fresh seal page from checked
data.

## Ask three separate questions

After [installing Sealjar](quickstart.md#install), check the original ZIP:

```sh
sealjar verify package.sealjar.zip
```

| Result | What it tells you |
| --- | --- |
| Capture | Did the capture finish? Which problems did the operator's run record? |
| Files | Does every listed file still match its hash in the manifest? |
| Timestamp | Is a timestamp token valid and signed by a trusted authority? |

Exit 0 means the files match and at least one timestamp is trusted. The capture
may still have recorded problems. Exit 10 means a file changed or is invalid.
Exit 11 means there is no trusted timestamp, for example because the package was
never timestamped or carries the demo's local token. Keep the original files as
they are. Never edit them to get a clean result.

Matching hashes show that the files agree with the manifest. On their own they
cannot vouch for a manifest without a timestamp, because files and hashes could
have been replaced together. A trusted timestamp shows that the sealed files
existed no later than its time. It does not check when the capture was made.

Verify trusts the system's root certificates and the FreeTSA root that ships
with Sealjar. You can add more roots with `--tsa-root` if you got them from a
source you trust. Certificates that only come inside the package are not
trusted. An offline check cannot see a later revocation. Sealjar v0.1 does not
assess eIDAS qualification and never calls a timestamp qualified. The demo token
is signed with a public key and says nothing about time, even if a root you supply trusts it.

## Check without installing Sealjar

`VERIFY.txt` inside the ZIP and the
[verification guide](verify.md#without-installing-sealjar) give the commands. A
Python 3 script compares every file with the hashes in the manifest. OpenSSL
checks the timestamp token with roots you got yourself and with `-attime` set to
the token's genTime. Printing a token as text does not check its signature.

## What the package cannot establish

A package cannot show that its content is true or complete. It cannot show that
the operator was honest or that a server sent the saved bytes. It cannot
show that a project was delivered or accepted, and it cannot link the captured
code to the website. It does not replace an acceptance protocol or a delivery
record. Sealjar promises no legal effect and no admissibility. For an important
dispute, have a neutral person present during the capture, for example an
expert or a notary.

## Before forwarding the package

Capture loaded the pages with everything they load. It asked DNS servers about
the hosts and sent a hash to the timestamp authority, which received that hash
and normal network metadata. Screenshots and HTML can show private content, and
source files and Git history can contain secrets. Masking and exclusions do not
catch everything. Look through the package before you forward it, and agree on
who may receive it.

Auf Deutsch: [Ein erhaltenes Paket lesen und prüfen](de/fuer-empfaenger.md).
