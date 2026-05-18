# bolt build and release helpers
VERSION ?= dev
DIST    ?= dist
BINARY  := bolt
MAIN    := ./cmd/bolt

LDFLAGS := -s -w -X github.com/bolt/bolt/internal/transport.BoltVersion=$(VERSION)

.PHONY: build test lint security clean release-snapshot release-local install

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) $(MAIN)

test:
	go test -race -coverprofile=coverage.out -covermode=atomic ./...

# Run golangci-lint (includes gosec for OWASP checks).
# Install: https://golangci-lint.run/usage/install/
lint:
	golangci-lint run --timeout=5m

# Run all standalone security scanners locally.
# Install govulncheck: go install golang.org/x/vuln/cmd/govulncheck@latest
# Install trivy:       https://aquasecurity.github.io/trivy/latest/getting-started/installation/
security:
	govulncheck ./...
	trivy fs . --severity HIGH,CRITICAL --exit-code 1

clean:
	rm -f $(BINARY) bolt.exe coverage.out
	rm -rf $(DIST)

release-snapshot:
	goreleaser build --snapshot --clean

release-local: clean
	@mkdir -p $(DIST)
	GOOS=darwin  GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(DIST)/$(BINARY)_darwin_arm64  $(MAIN)
	GOOS=darwin  GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(DIST)/$(BINARY)_darwin_amd64  $(MAIN)
	GOOS=linux   GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(DIST)/$(BINARY)_linux_amd64   $(MAIN)
	GOOS=linux   GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(DIST)/$(BINARY)_linux_arm64   $(MAIN)
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(DIST)/$(BINARY)_windows_amd64.exe $(MAIN)
	@echo "Binaries in $(DIST)/"

install: build
	install -m 755 $(BINARY) $(DESTDIR)/usr/local/bin/$(BINARY)
