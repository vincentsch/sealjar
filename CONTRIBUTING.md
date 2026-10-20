# Contributing

Use the Go version in `go.mod`. Chrome or Chromium is needed for browser tests;
Git is needed for repository tests. Routine tests use local timestamp authorities.

Run these checks before submitting a change:

```sh
make build
make lint
make test
make race
make docs-check
make conformance
```

`make build-all` builds Linux, macOS and Windows binaries for amd64 and arm64,
with SHA-256 checksums in `dist/`. `make clean` removes `bin/` and `dist/`.
Set `SIGNETLEAF_REQUIRE_CHROME=1` to fail browser tests when Chrome is missing.
Otherwise those tests print `skipped: no Chrome` when they cannot run.

Update help and examples alongside command changes, then run `make docs`.
The [command reference](docs/reference/index.md) is generated; keep handwritten
pages and media outside `docs/reference/`. `make docs-check` checks for drift.

Installer checks run against two temporary local releases:

```sh
scripts/fake-release.sh python3 scripts/test-install.py
```

This check needs Linux amd64, Python 3 and curl. It builds all six release assets.
It downloads only from the local fixture and removes its server and files on exit.
Release-note tests run as part of `make test`.

Write plain sentences and support claims with observed behavior. Error messages
should explain what happened and give a practical next step. Write German as
natural German, rather than translating English word for word. Avoid promises
of legal effect or claims that a timestamp establishes more than the tool checks.

Keep changes focused and test the behavior a person relies on. Never commit
credentials, private captures or machine-specific files.
