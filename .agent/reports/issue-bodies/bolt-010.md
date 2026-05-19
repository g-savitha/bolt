## Description
As a backend engineer writing Phase-2 integration tests, I want a one-call helper that returns two authenticated `PeerConn` instances connected over real in-process QUIC, so that I can test sender / receiver / TOFU / chat round-trips without mocks.

## Why
- Unblocks every integration test in BOLT-001, BOLT-006, BOLT-008, BOLT-011, and Phase-2 transfer tests downstream.
- Sources: Backend Phase-2 prereq §H, QA coverage gap.

## Acceptance Criteria
- [ ] `internal/testutil/loopback.go` exposes `NewLoopbackPair(t *testing.T) (a, b *transport.PeerConn)`.
- [ ] Uses real Ed25519 identities (generated per-call into a `t.TempDir`), real TLS handshake, real QUIC listener on `127.0.0.1:0`.
- [ ] `t.Cleanup` closes both `PeerConn`s and the underlying listener — no goroutine leak under `-race` across 100 sequential calls.
- [ ] Used by at least one integration test in BOLT-001 (TOFU round-trip) and one in BOLT-008 (4 MiB chunk header-then-raw round-trip).
- [ ] Godoc on the harness names intended consumers: Phase-2 transfer tests, chat tests, TOFU prompt tests; documents the pair is *authenticated* and *handshake-complete* on return.

## Notes
- Files: `internal/testutil/loopback.go` (new).
- Source story: `.agent/backlog/tasks.md` BOLT-010.
