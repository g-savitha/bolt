# bolt build and release helpers
VERSION ?= 0.1.0
DIST    ?= dist
BINARY  := bolt
MAIN    := ./cmd/bolt

.PHONY: build test clean release-snapshot install

build:
	go build -ldflags "-s -w" -X github.com/bolt/bolt/internal/transport.BoltVersion=$(VERSION)" -o $(BINARY) $(MAIN)

test:
	go test ./...

clean:
	rm -f $(BINARY) bolt.exe
	rm -rf $(DIST)

release-snapshot:
	goreleaser build --snapshot --clean

release-local: clean
	@mkdir -p $(DIST)
	GOOS=darwin GOARCH=arm64 go build -ldflags "-s -w" -o $(DIST)/$(BINARY)_darwin_arm64 $(MAIN)
	GOOS=darwin GOARCH=amd64 go build -ldflags "-s -w" -o $(DIST)/$(BINARY)_darwin_amd64 $(MAIN)
	GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o $(DIST)/$(BINARY)_linux_amd64 $(MAIN)
	GOOS=linux GOARCH=arm64 go build -ldflags "-s -w" -o $(DIST)/$(BINARY)_linux_arm64 $(MAIN)
	GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -o $(DIST)/$(BINARY)_windows_amd64.exe $(MAIN)
	@echo "Binaries in $(DIST)/"

install: build
	install -m 755 $(BINARY) $(DESTDIR)/usr/local/bin/$(BINARY)
