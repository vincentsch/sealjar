# Timestamps and trust

A timestamp authority (TSA) signs a hash with its time. Signetleaf uses RFC 3161
responses over the SHA-256 hash of the original manifest bytes. The authority
receives a hash, a nonce and normal network metadata, rather than the package.

Capture tries DigiCert at `http://timestamp.digicert.com`, then FreeTSA at
`https://freetsa.org/tsr` if DigiCert returns no usable token. Signatures and
certificate checks protect the response even over HTTP. Repeat `--tsa` to
replace those defaults and request every supplied authority. After
[installation](quickstart.md#install), for a custom authority:

```sh
signetleaf capture . --out .. --tsa https://tsa.example/tsr --tsa-root roots.pem --confirm --no-prompt
```

Obtain the root independently from the authority. Capture still writes a package
when all timestamp requests fail and explains how to stamp later. `--strict`
returns 10 after writing if timestamp acquisition or capture quality failed.
`--no-timestamp` deliberately skips requests and does not trigger strict failure.
An otherwise valid token with an unknown trust anchor may be retained as untrusted.

## Add another token later

```sh
signetleaf stamp package.signetleaf.zip --dry-run
signetleaf stamp package.signetleaf.zip
signetleaf stamp package.signetleaf.zip --tsa https://tsa.example/tsr --tsa-root roots.pem
```

Stamp writes a new `-stamped.signetleaf.zip` beside the original, preserving the
manifest, payload and existing token bytes. The new token bounds existence by
its own later time. It does not authenticate the operator's earlier capture time.
Existing tokens remain separately checkable. Use `verify` on the printed new path.

## Decide what to trust

Verification checks imprint, CMS signature, signer binding, timestamp-only
certificate purpose and the chain at the token's genTime. System roots, the
embedded FreeTSA root and independently supplied `--tsa-root` files provide
trust anchors. Certificates merely carried in the package cannot grant trust.
Offline checks cannot see later certificate revocation.

Signetleaf v0.1 does not assess eIDAS qualification or label a timestamp qualified.
Cryptographic trust and a service's qualification are separate questions.
For a manual check, [verify](verify.md#without-installing-signetleaf) shows Python
hash checks and OpenSSL with independently obtained roots and `-attime`.

A trusted timestamp gives an upper bound for when sealed files existed. It does
not establish truthful or complete content, honest operation, that a server
sent the bytes, delivery, acceptance, or that captured code produced the website.
No legal effect or admissibility is promised. For important disputes, consider
a neutral expert or notary present during capture.

## Network and demo behaviour

Timestamp requests use `HTTP_PROXY`, `HTTPS_PROXY` and `NO_PROXY`, with a
10-second deadline per authority and a 1 MiB response limit. Doctor only probes
connections, sends no timestamp request and does not honour proxy settings.

Demo tries the default authority. If unavailable, it uses a local token labelled
“Demo timestamp, not trusted”. Its public demo root identifies provenance,
without granting trust. Verify normally exits 11 for that fallback. Do not
present it as independent timestamp evidence.
