# Timestamps and trust

Sealjar is a free, open-source CLI that captures web pages and code into a
sealed ZIP. Each package gets a timestamp from a timestamp authority (TSA). This
page explains where that timestamp comes from, what it shows and how to add one
later.

The authority signs the SHA-256 hash of the manifest together with its own clock
time and returns an RFC 3161 token. It receives the hash with a random request
number (nonce) and sees normal network metadata. The package itself stays on
your computer.

Capture asks DigiCert at `http://timestamp.digicert.com` first. If DigiCert
returns no usable token, it asks FreeTSA at `https://freetsa.org/tsr`. The
token's signature and certificate chain protect it, so plain HTTP is fine.
Repeat `--tsa` to replace both defaults; every authority you list is asked. For
your own authority:

```sh
sealjar capture . --out .. --tsa https://tsa.example/tsr --tsa-root roots.pem --confirm --no-prompt
```

Get the root from the authority itself, never from a package. If every request
fails, capture still writes the package and tells you how to stamp it later.
With `--strict`, the exit code is 10 after writing when timestamping or capture
quality failed. `--no-timestamp` skips the requests on purpose and does not count
as a failure. A valid token with an unknown root may be kept in the package as
untrusted.

## Add another token later

```sh
sealjar stamp package.sealjar.zip --dry-run
sealjar stamp package.sealjar.zip
sealjar stamp package.sealjar.zip --tsa https://tsa.example/tsr --tsa-root roots.pem
```

Stamp writes a new `-stamped.sealjar.zip` next to the original. The manifest,
the captured files and the existing tokens stay as they are. The new token shows
that the sealed files existed no later than its own, later time. It says nothing
about the earlier capture time. Run `verify` on the new file to check its
tokens.

## Decide what to trust

Verify checks that the token's hash matches the manifest and that its CMS
signature is valid. It checks that the signing certificate belongs to the signer
and is meant for timestamping. It builds the chain to a trusted root as of the
token's genTime. Trusted roots are the system's root certificates, the FreeTSA
root that ships with Sealjar, and the files you supply with `--tsa-root`.
Certificates that only come inside the package are not trusted. An offline
check cannot see a later revocation.

Sealjar v0.1 does not assess eIDAS qualification and never calls a timestamp
qualified. Whether a signature checks out and whether a service is qualified are
two different questions. For a manual check,
[verify](verify.md#without-installing-sealjar) shows the Python hash check and
OpenSSL with your own roots and `-attime`.

A trusted timestamp shows that the sealed files existed no later than its time.
It does not check when the capture was made. A package cannot show that its
content is true or complete. It cannot show that the operator was honest or
that a server sent the saved bytes. It cannot show that a project was
delivered or accepted, and it cannot link the captured code to the website.
Sealjar promises no legal effect and no admissibility. For an important dispute,
have a neutral person present during the capture, for example an expert or a
notary.

## Network and demo behaviour

Timestamp requests use `HTTP_PROXY`, `HTTPS_PROXY` and `NO_PROXY`. Each
authority gets 10 seconds, and responses over 1 MiB are rejected. Doctor only
probes the connections. It sends no timestamp request and ignores proxy
settings.

The demo asks the default authority first. If that fails, it signs with a local
key and labels the token “Demo timestamp, not trusted”. Its demo root
identifies the token as a demo token without trusting it, and verify exits 11.
The signing key is public, and anyone can make such a token. It says nothing
about when the files existed, even if you supply a root that trusts it.

With `stamp --json`, the `verification` object describes the new output package,
checked with the roots you supplied. In a dry run it describes the input
package.

## macOS and DigiCert

During the v0.1.0 launch check, a native macOS arm64 runner returned exit 11 for
a DigiCert timestamp with the default roots. The same package verified with
exit 0 after passing a root bundle exported from macOS. FreeTSA verified with
exit 0 both with the default roots and with its official root. These are the
observed results for v0.1.0, not a guarantee for every Mac.

If verify reports an unknown authority for a DigiCert token on macOS, export the system roots and supply the file:

```sh
sealjar_roots_dir=$(mktemp -d)
security find-certificate -a -p \
  /System/Library/Keychains/SystemRootCertificates.keychain \
  > "$sealjar_roots_dir/system-roots.pem"
shasum -a 256 "$sealjar_roots_dir/system-roots.pem"
sealjar verify package.sealjar.zip --tsa-root "$sealjar_roots_dir/system-roots.pem"
```

Keep the exported file and its hash with the verification output. This changes nothing in
the system's trust settings. Never take a root from the package you are
checking. For a new capture, `--tsa https://freetsa.org/tsr` asks FreeTSA
instead of DigiCert first.
