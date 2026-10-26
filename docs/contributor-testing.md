# Test capture with local fixtures

Sealjar's source ships a test website and a test timestamp authority. With them
you can run captures without an external site, including basic-auth
routes. Build both from a separate Sealjar checkout:

```sh
go build -o /tmp/sealjar-testsite ./internal/testsite/cmd/testsite
go build -o /tmp/sealjar-testtsa ./internal/tsa/testtsa/cmd/testtsa
```

Create temporary directories and keep both servers running in this shell:

```sh
SITE_DIR=$(mktemp -d)
TSA_DIR=$(mktemp -d)
/tmp/sealjar-testsite --out "$SITE_DIR" &
SITE_PID=$!
/tmp/sealjar-testtsa --out "$TSA_DIR" &
TSA_PID=$!
```

Wait until each directory contains a file named `endpoint`, then read those
files. The test authority also writes its public `root.pem`; its private keys
stay in memory.

```sh
SITE=$(cat "$SITE_DIR/endpoint")
TSA=$(cat "$TSA_DIR/endpoint")
sealjar capture "$SITE/" . --out .. --tsa "$TSA" --tsa-root "$TSA_DIR/root.pem" --dry-run
```

Check the plan, including the exclusions and the absolute output directory. If
it is right, run:

```sh
sealjar capture "$SITE/" . --out .. --tsa "$TSA" --tsa-root "$TSA_DIR/root.pem" --confirm --no-prompt
sealjar capture "$SITE/private/" --auth client:handover --out .. --tsa "$TSA" --tsa-root "$TSA_DIR/root.pem" --confirm --no-prompt
```

Put a printed ZIP path in place of `package.sealjar.zip` to check that capture:

```sh
sealjar verify package.sealjar.zip --tsa-root "$TSA_DIR/root.pem"
```

Run these captures from the project you are capturing. `client:handover` is public fixture data. For your own credentials, use
`SEALJAR_AUTH` instead of `--auth`. The root comes
from the running fixture, not from the package. When you are done, stop your
fixture processes and remove their directories:

```sh
kill "$SITE_PID" "$TSA_PID"
wait "$SITE_PID" "$TSA_PID"
rm -rf "$SITE_DIR" "$TSA_DIR"
rm /tmp/sealjar-testsite /tmp/sealjar-testtsa
```
