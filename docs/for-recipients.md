# Received a Signetleaf package?

The ZIP holds captured pages, code and checking information. A folder beside it
contains reports for reading. Open `report/report.en.html` in that folder, keeping
its images and other files together. The report PDF can be read on its own.
No Signetleaf installation is needed to read either copy.

Start with the summary. Check the listed URLs, repository commit and exclusions.
Read each capture problem: a failed page or missing resource can leave gaps.
Screenshots show the recorded desktop and mobile views, including any banners
or login forms. They do not demonstrate every interaction on a website.

The report points to `SEAL.html` for timestamp results. The seal is a reading aid,
made after the manifest; it is outside the manifest's file list. Its trust text
reflects the check when it was made. Verification checks tokens directly, and
report extraction regenerates the seal from checked data.

## Ask three separate questions

After [installing](quickstart.md#install), check the original ZIP:

```sh
signetleaf verify package.signetleaf.zip
```

Capture describes completeness and problems recorded by the operator's tool.
Files says whether the listed files match their manifest hashes. Timestamp says
whether a token is cryptographically valid and trusted. Exit 0 does not mean a
capture was complete. Exit 10 identifies changed or invalid content. Exit 11
means no trusted timestamp, including an untimestamped package or demo token.
Keep the original files; do not edit or repair them to make verification pass.

Matching hashes establish agreement with the manifest. They alone cannot
authenticate an untimestamped manifest, because files and hashes could be
replaced together. A trusted timestamp gives an upper bound for when sealed
files existed, rather than an independently checked capture time.

Trust comes from system roots, the embedded FreeTSA root or independently
supplied explicit roots. Certificates merely carried in the package do not
establish trust. Offline checks cannot see later revocation. Signetleaf v0.1
does not assess eIDAS qualification or label a timestamp qualified.

## Check without installing Signetleaf

`VERIFY.txt` in the ZIP and the [verification guide](verify.md#without-installing-signetleaf)
provide a Python 3 manifest-file hash check and an OpenSSL timestamp check.
Those steps need Python/OpenSSL even though they do not need Signetleaf.
The timestamp check uses independently obtained roots and `-attime` with the
token's genTime. Reading a token's text alone does not verify its signature.

A package cannot establish truthful or complete content, honest operation, that
a server sent the bytes, delivery or acceptance, or that captured code produced
the website. It does not replace an acceptance protocol or a delivery record.
No legal effect or admissibility is promised. For important disputes, consider
a neutral expert or notary present during capture.

Capture contacts pages and resources, DNS and timestamp authorities. A TSA
receives a hash and normal network metadata. Timestamp requests use standard
proxies; doctor's probes do not. Masking and exclusions cannot remove private
content from screenshots, DOM, source or history. Inspect the package before
forwarding it, and agree separately on who may receive it.

For German readers: [Pakete lesen und prüfen](de/fuer-empfaenger.md).
