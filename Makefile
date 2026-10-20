.PHONY: build test race vet fmt check conformance

build:
	CGO_ENABLED=0 go build -o bin/signetleaf ./cmd/signetleaf

test:
	go test ./...

race:
	go test -race ./...

vet:
	go vet ./...

fmt:
	@test -z "$$(gofmt -l cmd internal)" || (gofmt -l cmd internal; exit 1)

check: fmt vet test race build

conformance:
	bash scripts/conformance.sh
