VERSION ?= dev
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_TIME ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w -X github.com/IbodovElbek/tunnel-client/internal/build.Version=$(VERSION) -X github.com/IbodovElbek/tunnel-client/internal/build.Commit=$(COMMIT) -X github.com/IbodovElbek/tunnel-client/internal/build.BuildTime=$(BUILD_TIME)

.PHONY: build test release
build:
	go build -ldflags "$(LDFLAGS)" -o bin/tunnel ./cmd/tunnel
test:
	go test ./...
release:
	mkdir -p dist
	GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/tunnel-darwin-arm64 ./cmd/tunnel
	GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/tunnel-darwin-amd64 ./cmd/tunnel
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/tunnel-linux-amd64 ./cmd/tunnel
	GOOS=linux GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/tunnel-linux-arm64 ./cmd/tunnel
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/tunnel-windows-amd64.exe ./cmd/tunnel
