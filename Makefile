VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo dev)
LDFLAGS := -s -w -X github.com/vincentsch/signetleaf/internal/version.Version=$(VERSION) -X github.com/vincentsch/signetleaf/internal/version.Commit=$(COMMIT)
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64

.PHONY: build build-all test race vet fmt lint check conformance docs docs-check clean

build:
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o bin/signetleaf ./cmd/signetleaf

build-all:
	@mkdir -p dist
	@set -e; \
	rm -f dist/signetleaf-* dist/checksums.txt; \
	for platform in $(PLATFORMS); do \
		os=$${platform%/*}; arch=$${platform#*/}; \
		asset=signetleaf-$$os-$$arch; \
		if [ "$$os" = windows ]; then asset=$$asset.exe; fi; \
		echo "Building $$asset"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -trimpath -ldflags "$(LDFLAGS)" -o dist/$$asset ./cmd/signetleaf; \
	done
	@cd dist && if command -v sha256sum >/dev/null 2>&1; then sha256sum signetleaf-*; else shasum -a 256 signetleaf-*; fi > checksums.txt

test:
	go test ./...
	python3 scripts/test-release-notes.py
	python3 scripts/test-docs.py

race:
	go test -race ./...

vet:
	go vet ./...

fmt:
	@test -z "$$(gofmt -l cmd internal)" || (gofmt -l cmd internal; exit 1)

lint: fmt vet

check: lint test race build docs-check

conformance:
	bash scripts/conformance.sh

docs:
	go run ./cmd/generate-command-docs

docs-check:
	go run ./cmd/generate-command-docs --check

clean:
	rm -rf bin dist
