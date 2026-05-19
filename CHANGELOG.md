# Changelog

All notable changes to bolt are documented here.

Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)
Versioning: [Semantic Versioning](https://semver.org/spec/v2.0.0.html)

---

## [Unreleased]

---

## [0.1.2] — 2026-05-19

### Security
- Bumped Go toolchain to 1.25.10, resolving 15 stdlib CVEs (BOLT-024, #37)
- Restricted daemon spawned-process environment to an explicit allowlist (BOLT-026, #42)
- Sanitize peer display strings before writing to TTY to prevent terminal injection (BOLT-025, #41)
- Unix IPC socket chmod 0600 after listen — prevents other local users reading the socket (BUG-6, #43)

### Fixed
- Atomic-rename writes for `peers.toml` and `config.toml` — config is never left half-written on crash (BOLT-003, #38)
- Daemon shutdown now closes all IPC subscribers, eliminating a goroutine leak (BOLT-004, #39)
- `daemon.pid` write and remove are symmetric: PID file is only present when the process is live (BOLT-005, #40)
- `listenIPC` refuses to start if another daemon socket is already live — prevents silently replacing a running daemon (BUG-7, #44)
- Chat stream is closed with an error when a wire version mismatch is detected (BUG-8, #45)
- `PublishChat` lock scope tightened — lock was held across slow IPC writes, causing unnecessary contention (BUG-10, #46)
- Pluggable stream-handler registry on `Daemon` — stream types are no longer hard-coded (BOLT-006 / BUG-9, #47)

### CI & Tooling
- GHA CI fixes: `go mod tidy` guard, Trivy SARIF upload, golangci-lint upgraded to v2 (#48)
- `dependency-review` workflow re-enabled after Dependency graph was activated (#49)
- Release workflow: install syft via curl before goreleaser for SBOM generation

### Known open (not in this release)
- BUG-1 (#23): TOFU peer-verification stub always returns `(true, nil)` — Phase 2, BOLT-001
- BUG-4 (#26): `ChunkMsg.Data []byte` is base64-encoded in JSON wire format — Phase 2, BOLT-008
- BOLT-027 (#36): Symlink hardening for config-dir writes — Phase 2

---

## [0.1.1] — 2026-05-17

### Added
- Version string stamped into binary at build time via `ldflags` (`-X main.version`)
- Install scripts auto-resolve the latest release tag via GitHub API

### Fixed
- Removed `brews` and `scoops` publishers from goreleaser config that were blocking the release pipeline

---

## [0.1.0] — 2026-05-17

### Added
- Initial Phase 1 release of bolt (renamed from flick)
- QUIC-based P2P daemon with IPC socket, chat streams, and peer management
- Cross-platform goreleaser pipeline: linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64
- GitHub Actions CI: build, test-race, lint (golangci-lint v2), govulncheck, CodeQL, Scorecard, Trivy

---

[Unreleased]: https://github.com/g-savitha/bolt/compare/v0.1.2...HEAD
[0.1.2]: https://github.com/g-savitha/bolt/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/g-savitha/bolt/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/g-savitha/bolt/releases/tag/v0.1.0
