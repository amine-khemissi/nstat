BIN     := nstat
LDFLAGS := -ldflags="-s -w"
INSTALL := $(HOME)/.local/bin/$(BIN)

# Build for the current OS/arch (development)
build:
	CGO_ENABLED=0 go build $(LDFLAGS) -o $(BIN) .

# Build release binaries for all supported platforms
dist:
	mkdir -p dist
	CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BIN)-linux-amd64   .
	CGO_ENABLED=0 GOOS=linux   GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BIN)-linux-arm64   .
	CGO_ENABLED=0 GOOS=darwin  GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BIN)-darwin-amd64  .
	CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BIN)-darwin-arm64  .
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BIN)-windows-amd64.exe .

install: build
	cp $(BIN) $(INSTALL)
	@echo "installed: $(INSTALL)"

clean:
	rm -rf $(BIN) dist/

.PHONY: build dist install clean
