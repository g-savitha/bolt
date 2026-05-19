## Description
As a bolt user whose daemon faces the LAN (and post-Phase-5 the open internet), I want the Go stdlib running my TLS/QUIC stack to be patched, so that a network attacker cannot trigger known DoS or state-confusion bugs via every handshake.

`go.mod` pins `go 1.25.1` and the local toolchain is 1.25.1. `govulncheck ./...` reports **15 reachable Go stdlib advisories** across `crypto/tls`, `crypto/x509`, `encoding/asn1`, `encoding/pem`, `net/url`, and `net`. Highest-impact reachable items:

- `GO-2026-4870` — unauthenticated TLS 1.3 `KeyUpdate` causes persistent connection retention / DoS (`crypto/tls`, fixed in 1.25.9). Reachable via every QUIC handshake (`internal/transport/quic.go:56`).
- `GO-2026-4340` — handshake messages processed at the wrong encryption level (TLS state confusion; fixed in 1.25.6).
- `GO-2026-4337` — unexpected session resumption (fixed in 1.25.7).
- `GO-2025-4011` — DER parsing memory exhaustion in `encoding/asn1` (fixed in 1.25.2). Reachable via `identity.TLSCertificate` → `tls.X509KeyPair` (`internal/identity/identity.go:174`).
- `GO-2025-4009` — quadratic PEM parsing (fixed in 1.25.2). Reachable via `tls.X509KeyPair` on cert-load.
- `GO-2025-4008` — ALPN error string contains attacker-controlled bytes (fixed in 1.25.2).

## Why
- Reachable from every inbound and outbound QUIC handshake; trivial to trigger by any network-reachable peer.
- One-PR change, no source code churn, closes all 15 reachable advisories with one toolchain bump.
- Sources: Security review `SEC-1` (`.agent/reports/security-review.md` §New findings — Critical), `govulncheck` Symbol Results.

## Acceptance Criteria
- [ ] `go.mod` line 3: `go 1.25.1` → `go 1.25.10` (or current latest patch on the 1.25.x line at merge time).
- [ ] `go.mod` gains a `toolchain go1.25.10` directive so contributors with older Go installs fail loudly instead of silently building against an unpatched stdlib.
- [ ] `go mod tidy` clean; `go build ./...`, `go vet ./...`, `golangci-lint run`, `go test ./...`, `go test -race ./...` all PASS.
- [ ] `govulncheck ./...` exits 0 (no reachable advisories) — paste the before/after Symbol Results section into the PR description for traceability.
- [ ] CI workflows already pin via `go-version-file: go.mod`; verify the next CI run on the PR installs 1.25.10 automatically.

## Notes
- Files: `go.mod`.
- Independent of all other Phase-2 stories — no decision deps, can start immediately.
- After merge, `goreleaser` will pick up the patched stdlib on the next release build without further action.
- Source story: `.agent/backlog/tasks.md` BOLT-024. Originating finding: `.agent/reports/security-review.md` SEC-1.
