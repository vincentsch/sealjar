# Timestamps and trust

A timestamp authority (TSA) signs a hash with its time. Sealjar uses RFC 3161
responses over the SHA-256 hash of the original manifest bytes. The authority
receives the hash with a random request identifier (nonce). It also sees normal
network metadata. The package stays on your computer.

Capture tries DigiCert at `http://timestamp.digicert.com`, then FreeTSA at
`https://freetsa.org/tsr` if DigiCert returns no usable token. Signatures and
certificate checks protect the response even over HTTP. Repeat `--tsa` to replace
those defaults and request every supplied authority. To use a custom authority:

```sh
sealjar capture . --out .. --tsa https://tsa.example/tsr --tsa-root roots.pem --confirm --no-prompt
```

Obtain the root independently from the authority. Capture still writes a package
when all timestamp requests fail and explains how to stamp later. `--strict` returns
10 after writing if timestamp acquisition or capture quality failed.
`--no-timestamp` deliberately skips requests and does not trigger strict failure. An
otherwise valid token with an unknown trust anchor may be retained as untrusted.

## Add another token later

```sh
sealjar stamp package.sealjar.zip --dry-run
sealjar stamp package.sealjar.zip
sealjar stamp package.sealjar.zip --tsa https://tsa.example/tsr --tsa-root roots.pem
```

Stamp writes a new `-stamped.sealjar.zip` beside the original, preserving the
manifest and captured files. Existing tokens also stay unchanged. The new token
shows that the sealed files existed by its own later time. It does not authenticate
the operator's earlier capture time. Use `verify` on the printed new path to check
its tokens.

## Decide what to trust

Verification checks imprint, CMS signature, signer binding, timestamp-only
certificate purpose and the chain at the token's genTime. System roots, the embedded
FreeTSA root and independently supplied `--tsa-root` files provide trust anchors.
Certificates merely carried in the package cannot grant trust. Offline checks cannot
see later certificate revocation.

Sealjar v0.1 does not assess eIDAS qualification or label a timestamp qualified.
Cryptographic trust and a service's qualification are separate questions. For a
manual check, [verify](verify.md#without-installing-sealjar) shows Python hash
checks and OpenSSL with independently obtained roots and `-attime`.

A trusted timestamp shows that the sealed files existed by the time it records. It
does not independently check when capture happened. A package cannot show whether
its content is true or complete. It cannot establish that the operator was honest or
that a server sent the saved bytes. It also cannot establish delivery or acceptance
of a project, or link the captured code to the website. No legal effect or
admissibility is promised. For important disputes, consider a neutral expert or
notary present during capture.

## Network and demo behaviour

Timestamp requests use `HTTP_PROXY`, `HTTPS_PROXY` and `NO_PROXY`, with a 10-second
deadline per authority and a 1 MiB response limit. Doctor only probes connections,
sends no timestamp request and does not honour proxy settings.

Demo tries the default authority. If unavailable, it uses a local token labelled
“Demo timestamp, not trusted”. Its public demo root identifies the demo without
granting trust. Verify normally exits 11 for that fallback. The signing key is
public, so anyone can make such a token. It gives no independent time evidence, even
if an explicitly supplied root grants it trust.

With `stamp --json`, `verification` describes the completed output package, checked
with the supplied roots. In a dry run it describes the input package.

## macOS and DigiCert

The launch check on a native macOS arm64 runner returned verify exit 11
for a DigiCert timestamp with default roots. The same package verified with
exit 0 after supplying a root bundle exported independently from macOS.
FreeTSA verified with exit 0 using both default roots and its official root.
These are the observed v0.1.0 results, not a guarantee for every host.

If DigiCert verification reports an unknown authority on macOS, export the
system roots and pass that file explicitly:

```sh
sealjar_roots_dir=$(mktemp -d)
security find-certificate -a -p \
  /System/Library/Keychains/SystemRootCertificates.keychain \
  > "$sealjar_roots_dir/system-roots.pem"
shasum -a 256 "$sealjar_roots_dir/system-roots.pem"
sealjar verify package.sealjar.zip --tsa-root "$sealjar_roots_dir/system-roots.pem"
```

Keep the exported file and its hash with your verification notes. This does
not install a certificate or change system trust. Do not take a trust anchor
from the package being checked. For a new capture, an explicit
`--tsa https://freetsa.org/tsr` selects FreeTSA instead of the default
DigiCert-first sequence.
