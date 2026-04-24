BIN     := nstat
PKG     := github.com/amine-khemissi/nstat/version
GIT_COMMIT := $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
GIT_TAG    := $(shell git describe --tags --exact-match 2>/dev/null || echo "")
GIT_DIRTY  := $(shell git diff --quiet 2>/dev/null || echo "dirty")
VERSION_FLAGS := -X $(PKG).GitCommit=$(GIT_COMMIT) -X $(PKG).GitTag=$(GIT_TAG) -X $(PKG).GitDirty=$(GIT_DIRTY)
LDFLAGS := -ldflags="-s -w $(VERSION_FLAGS)"
INSTALL := $(HOME)/.local/bin/$(BIN)

# Build for the current OS/arch (development)
build:
	CGO_ENABLED=0 go build $(LDFLAGS) -o bin/$(BIN) .

# Build release binaries for all supported platforms
dist:
	mkdir -p dist
	CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BIN)-linux-amd64   .
	CGO_ENABLED=0 GOOS=linux   GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BIN)-linux-arm64   .
	CGO_ENABLED=0 GOOS=darwin  GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BIN)-darwin-amd64  .
	CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BIN)-darwin-arm64  .
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BIN)-windows-amd64.exe .

install: build
	cp bin/$(BIN) $(INSTALL)
	@echo "installed: $(INSTALL)"

clean:
	rm -rf bin/$(BIN) dist/

.PHONY: build dist install clean
