# Received a Sealjar package?

The ZIP holds the captured pages and code, along with instructions for checking
them. A folder beside it contains reports for reading. Open `report/report.en.html`
in that folder, keeping its images and other files together. The report PDF can be
read on its own. No Sealjar installation is needed to read either copy.

Start with the summary. Do the URLs and repository commit match what you expected?
Check which files were excluded. Read each capture problem: a failed page or missing
resource can leave gaps. Screenshots show the recorded desktop and mobile views,
including any banners or login forms. They do not demonstrate every interaction on a
website.

The report points to `SEAL.html` for timestamp results. The seal is a reading aid,
made after the manifest; it is outside the manifest's file list. Its trust text
reflects the check when it was made. Verification checks tokens directly, and report
extraction regenerates the seal from checked data.

## Ask three separate questions

After [installing](quickstart.md#install), check the original ZIP:

```sh
sealjar verify package.sealjar.zip
```

| Result | What it tells you |
| --- | --- |
| Capture | Did the capture finish? Which problems did the operator's tool record? |
| Files | Do the listed files match the hashes in the manifest? |
| Timestamp | Is a timestamp token cryptographically valid and trusted? |

Exit 0 means the files match and at least one timestamp is trusted. The capture may
still have problems. Exit 10 identifies changed or invalid content. Exit 11 means no
trusted timestamp, including a package without a timestamp or with the demo
fallback. Keep the original files; do not edit them to make verification pass.

Matching hashes establish agreement with the manifest. They alone cannot
authenticate an untimestamped manifest, because files and hashes could be replaced
together. A trusted timestamp shows that the sealed files existed by the time it
records. It does not independently check when capture happened.

Trust comes from system roots, the embedded FreeTSA root or independently supplied
explicit roots. Certificates merely carried in the package do not establish trust.
Offline checks cannot see later revocation. Sealjar v0.1 does not assess eIDAS
qualification or label a timestamp qualified.

The demo fallback uses a public signing key. It gives no independent time evidence,
even if an explicitly supplied root grants the token trust.

## Check without installing Sealjar

`VERIFY.txt` in the ZIP and the [verification
guide](verify.md#without-installing-sealjar) provide a Python 3 manifest-file hash
check and an OpenSSL timestamp check. Those steps need Python/OpenSSL even though
they do not need Sealjar. The timestamp check uses independently obtained roots and
`-attime` with the token's genTime. Reading a token's text alone does not verify its
signature.

A package cannot show whether its content is true or complete. It cannot establish
that the operator was honest or that a server sent the saved bytes. It also cannot
establish delivery or acceptance of a project, or link the captured code to the
website. It does not replace an acceptance protocol or a delivery record. No legal
effect or admissibility is promised. For important disputes, consider a neutral
expert or notary present during capture.

Capture loads pages and their resources. It also contacts DNS servers and timestamp
authorities. An authority receives a hash and normal network metadata. Timestamp
requests use standard proxies; doctor's probes do not. Masking and exclusions cannot
remove private content from screenshots, DOM, source or history. Inspect the package
before forwarding it, and agree separately on who may receive it.

For German readers: [Pakete lesen und prüfen](de/fuer-empfaenger.md).