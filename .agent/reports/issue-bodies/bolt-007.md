## Description
As a bolt user transferring a file over LAN, I want a single QUIC stream to actually fill a gigabit pipe, and as a Phase-5 user transferring over the internet, I want connections to survive 60 s of idleness through home-router NATs.

## Why
- Locks tuning before Phase 2 stress-tests it.
- Source: Networking review §1 and decisions #3/#4/#5/#6/#7.

## Acceptance Criteria
- [ ] `internal/transport/quic.go::quicConfig()` sets **every** field below explicitly:
  - `MaxIdleTimeout = 60 * time.Second`
  - `KeepAlivePeriod = 20 * time.Second`
  - `HandshakeIdleTimeout = 10 * time.Second`
  - `EnableDatagrams = true`
  - `Allow0RTT = false`
  - `InitialStreamReceiveWindow = 4 * (1 << 20)` (4 MiB)
  - `MaxStreamReceiveWindow = 64 * (1 << 20)` (64 MiB)
  - `InitialConnectionReceiveWindow = 8 * (1 << 20)` (8 MiB)
  - `MaxConnectionReceiveWindow = 256 * (1 << 20)` (256 MiB)
  - `MaxIncomingStreams = 1000`
  - `MaxIncomingUniStreams = 1000`
- [ ] `internal/transport/tls.go`: `NextProtos = []string{"bolt/1"}` (ALPN locked), `SessionTicketsDisabled = true`, `MinVersion = tls.VersionTLS13`.
- [ ] Golden-snapshot test asserts every field above; the test fails on drift (so we notice if anyone "tunes" a value in a future PR).
- [ ] Code comment cites RFC 9000 §10.1 (idle), RFC 9001 §5.6 (0-RTT replay risk), RFC 5705 (keying material exporter, used by BOLT-002).
- [ ] `go test ./internal/transport/...` passes; manual loopback throughput check on `localhost` shows ≥ 700 Mbps on a single QUIC stream (sanity check, not a CI gate).

## Notes
- Files: `internal/transport/quic.go`, `internal/transport/tls.go`, `internal/transport/quic_test.go` (new).
- Source story: `.agent/backlog/tasks.md` BOLT-007.
