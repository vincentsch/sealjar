# Test capture with local fixtures

Source contributors can use the fixture site, including its basic-auth routes. From
a separate Sealjar source checkout, build both helpers:

```sh
go build -o /tmp/sealjar-testsite ./internal/testsite/cmd/testsite
go build -o /tmp/sealjar-testtsa ./internal/tsa/testtsa/cmd/testtsa
```

Create temporary directories and keep the servers in this shell:

```sh
SITE_DIR=$(mktemp -d)
TSA_DIR=$(mktemp -d)
/tmp/sealjar-testsite --out "$SITE_DIR" &
SITE_PID=$!
/tmp/sealjar-testtsa --out "$TSA_DIR" &
TSA_PID=$!
```

Wait until each directory contains `endpoint`, then read those files. The TSA also
writes its public `root.pem`; its private keys stay in memory.

```sh
SITE=$(cat "$SITE_DIR/endpoint")
TSA=$(cat "$TSA_DIR/endpoint")
sealjar capture "$SITE/" . --out .. --tsa "$TSA" --tsa-root "$TSA_DIR/root.pem" --dry-run
sealjar capture "$SITE/" . --out .. --tsa "$TSA" --tsa-root "$TSA_DIR/root.pem" --confirm --no-prompt
sealjar capture "$SITE/private/" --auth client:handover --out .. --tsa "$TSA" --tsa-root "$TSA_DIR/root.pem" --confirm --no-prompt
sealjar verify package.sealjar.zip --tsa-root "$TSA_DIR/root.pem"
```

Run these captures from the project being captured. `client:handover` is public
fixture data, never a suggestion for real credentials. The root is independently
supplied by this running fixture, not extracted from the package. When finished,
stop and remove your exact fixture processes and directories:

```sh
kill "$SITE_PID" "$TSA_PID"
wait "$SITE_PID" "$TSA_PID"
rm -rf "$SITE_DIR" "$TSA_DIR"
rm /tmp/sealjar-testsite /tmp/sealjar-testtsa
```

