# Product Backlog
_Last updated: 2026-05-19 by PO+Scrum_

Source citation legend: A-x = architect critique, B-x = backend review, N-§y = networking review section, Q-BUGn = QA bug-hunt, SEC-n = security review.

## Pickup order for Backend (work top to bottom)

| # | ID | Priority | Title | Owner | Est | Depends on | GitHub |
|---:|---|---|---|---|---:|---|---|
| 1 | BOLT-024 | P0 | Bump Go toolchain to 1.25.10 (SEC-1) | Backend | 0.5d | — | #33 |
| 2 | BOLT-003 | P0 | Atomic-rename writer for peers.toml / config.toml | Backend | 1d | — | #9 |
| 3 | BOLT-004 | P0 | Daemon shutdown closes IPC subscribers | Backend | 1d | — | #10 |
| 4 | BOLT-005 | P0 | daemon.pid symmetric write/remove | Backend | 0.5d | parallel #3 | #11 |
| 5 | BOLT-025 | P1 | Sanitize peer strings before TTY (SEC-2) | Backend | 1d | parallel #3–4 | #34 |
| 6 | BOLT-026 | P1 | Daemon env allowlist (SEC-3) | Backend | 1d | parallel #3–4 | #35 |
| 7 | BOLT-010 | P0 | Loopback test harness | Backend / QA | 2d | — | #16 |
| 8 | BOLT-011 | P0 | internal/config tests | Backend | 1d | BOLT-003 | #17 |
| 9 | BOLT-012 | P1 | FuzzReadFrame | QA | 1d | — | #18 |
| 10 | BOLT-007 | P0 | Lock quic.Config defaults | Networking | 1d | D3 sign-off | #13 |
| 11 | BOLT-001 | P0 | TOFU prompt IPC round-trip | Backend | 2d | BOLT-003, BOLT-010 | #7 |
| 12 | BOLT-002 | P0 | Ed25519 PoP + server verifier | Networking | 1–2d | bundle #11 | #8 |
| 13 | BOLT-008 | P0 | Freeze Phase-2 wire schema | Networking | 2d | D1, D2 | #14 |
| 14 | BOLT-006 | P0 | Stream-handler registry | Backend | 1d | before transfer | #12 |
| 15 | BOLT-013 | P0 | transfer skeleton + IPC types | Backend | 1d | BOLT-008, BOLT-006 | #19 |
| 16 | BOLT-014 | P1 | Disk-space helper | Backend | 0.5d | — | #20 |
| 17 | BOLT-015 | P1 | CI linux + macOS matrix | DevOps | 1d | — | #21 |
| 18 | BOLT-016 | P1 | Windows posture / ADR-002 | PO + Architect | spike | D5 | #22 |
| 19 | BOLT-009 | P0 | IPC types → proto + daemonclient | Backend | 2d | D8; can slip | #15 |

---

## Ready for Dev

_P0 blockers first, then P1. Full acceptance criteria unchanged; see **Pickup order** column above for sequence._

### P0 — Phase 2 blockers

### [BOLT-001] Implement TOFU prompt IPC round-trip with randomart and 30 s default-reject

**Type**: Feature (security-critical) | **Priority**: P0 | **Owner**: Backend (Security review)

**User Story**: As a bolt user, I want my daemon to prompt me with the fingerprint + randomart of every unknown peer before any connection completes, so that I can verify peer identity out-of-band and not have my machine silently trust strangers.

**Acceptance Criteria**:
- [ ] Daemon emits `IPCEvent{Type:"tofu_prompt", Payload:{fingerprint, nickname, randomart, addr, nonce, deadline_utc}}` on first contact from an unknown peer.
- [ ] Randomart is visible in the prompt (rendered via `internal/identity/randomart.go`, same algorithm `bolt id` uses).
- [ ] CLI subscriber reads `y` / `N` / `b` (always / once / block) from user, sends `IPCRequest{Command:"tofu_response", Payload:{nonce, decision}}`; UI defaults to N on Enter.
- [ ] `peerVerifier` blocks on a per-nonce channel up to **30 s**; on timeout returns `(false, "tofu timeout")` — i.e. **default reject**.
- [ ] On accept (`always` or `once`), trust level is persisted via atomic write to `peers.toml` (depends on BOLT-003).
- [ ] Integration test using `internal/testutil/loopback.go` (depends on BOLT-010): first contact triggers prompt, accept-once is honored, second contact from the same peer with `allow-once` re-prompts; `always-allow` peer does not re-prompt; `block` is rejected at TLS time (depends on BOLT-002).

**Notes**: **Pickup order:** 11 | **GitHub:** #7. Files: `internal/daemon/connect.go` (replace `peerVerifier` stub), `internal/daemon/daemon.go` (install verifier on inbound too), `internal/proto/ipc.go` (new event + command types — depends on BOLT-009), `cmd/bolt/main.go` (prompt rendering inside `connectCmd` and a subscriber path for inbound). Move the prompt to **after** TLS but **before** the bolt-level handshake completes, so the 10 s `HandshakeIdleTimeout` is not the wall clock (B-1, Q-BUG-1 §"Suggested fix"). Sources: A-3, B-1, N-§2, Q-BUG-1.

**Security addendum**: when the new peer's nickname collides with an existing trusted peer's nickname (different fingerprint), the prompt MUST display "name collision" plus both fingerprints; `transport.PeerRegistry.ResolvePeer` returns an explicit "ambiguous: N peers named '<name>'" error instead of silently picking one via map-iteration order. Source: SEC-7.

---

### [BOLT-002] Add Ed25519 proof-of-possession to HandshakeMsg + enforce server-side VerifyPeerCertificate

**Type**: Feature (security-critical) | **Priority**: P0 | **Owner**: Networking + Security

**User Story**: As a bolt user, I want every peer to cryptographically prove it holds the Ed25519 private key matching its fingerprint, so that an attacker who copies my public key cannot impersonate me on the LAN.

**Acceptance Criteria**:
- [ ] `HandshakeMsg` gains a `Signature []byte` field; each side signs `tls.ConnectionState.ExportKeyingMaterial("bolt-handshake", nil, 32)` (RFC 5705) with its Ed25519 private key.
- [ ] Receiver verifies `Signature` against the public key extracted from `SubjectKeyId`; on mismatch, close the stream with application error code `0x1001` and reject the connection.
- [ ] `ServerTLSConfig.VerifyPeerCertificate` is installed with the same TOFU verifier as `ClientTLSConfig` (today server side has none — the blocklist check happens post-handshake).
- [ ] `tls.Config.SessionTicketsDisabled = true` is set on both client and server configs (explicit, documented).
- [ ] Unit test: forge a cert with a stolen Ed25519 pubkey but a different Ed25519 priv → handshake fails with code 0x1001.
- [ ] Wire round-trip test: `HandshakeMsg{Signature: ...}` marshals/unmarshals through `WriteFrame`/`ReadFrame` without loss.

**Notes**: **Pickup order:** 12 | **GitHub:** #8. Files: `internal/proto/wire.go` (HandshakeMsg field + version stays `v=1` — additive change, forward-compatible), `internal/transport/tls.go` (server-side `VerifyPeerCertificate`), `internal/transport/peer_conn.go` (sign on send, verify on receive), `internal/identity/identity.go` (`Identity.Sign(data)` helper if not present). Sources: A-3, N-§2, N-decision #8.

---

### [BOLT-003] Atomic-rename writer for peers.toml and config.toml

**Type**: Bug fix (data durability) | **Priority**: P0 | **Owner**: Backend

**User Story**: As a bolt user, I want my trust state and configuration to survive a crash, kill -9, or power loss, so that a single bad shutdown does not silently re-expose me to TOFU re-acceptance of all peers.

**Acceptance Criteria**:
- [ ] New helper `writeAtomic(path string, fn func(io.Writer) error) error` in `internal/config/atomic.go`: writes to `path.tmp` in the same directory, `f.Sync()`, `f.Close()`, then `os.Rename(path.tmp, path)`.
- [ ] `PeerStore.flushLocked` (`internal/config/peers.go`) and `Config.write` (`internal/config/config.go`) route through `writeAtomic`.
- [ ] Unit test: inject an `io.Writer` that fails after N bytes → original file is untouched (byte-identical to pre-call state).
- [ ] Kill-during-write test: panic mid-encode → on disk the file is either the previous version or the new version, never empty or half-written.
- [ ] `go test -race ./internal/config/...` passes with concurrent `PeerStore.Upsert` from 100 goroutines.

**Notes**: **Pickup order:** 2 | **GitHub:** #9. Files: `internal/config/atomic.go` (new), `internal/config/peers.go`, `internal/config/config.go`. Same temp file must live in the same directory as the target (otherwise rename crosses filesystems and silently becomes a copy on some FSes). Sources: A-19, B-3, Q-BUG-3.

**Security addendum**: `writeAtomic` MUST create the temp file with mode `0600` (not `0644`) and `Rename` MUST land the target at `0600` — drop the existing G306 `//nolint:gosec` annotations on `config.go:183` and `peers.go:207`. Source: SEC-4. (Symlink-hardening of the temp file via `O_NOFOLLOW|O_EXCL` is tracked separately in BOLT-027.)

---

### [BOLT-004] Daemon shutdown closes IPC subscriber connections (close BUG-2)

**Type**: Bug fix (lifecycle) | **Priority**: P0 | **Owner**: Backend

**User Story**: As an operator, I want `SIGTERM` to a daemon that has an attached `bolt chat` subscriber to result in a clean exit within 5 s, so that CI tests, graceful restarts, and signal handling are not blocked by a parked `conn.Read`.

**Acceptance Criteria**:
- [ ] `IPCServer.Close` iterates `s.subs` under `s.subsMu`, calls `conn.Close()` on each subscriber, clears the map, **then** calls `wg.Wait()`.
- [ ] `handleSubscribe` either takes a context derived from the daemon run-context and exits on `ctx.Done`, or returns cleanly when its `conn.Read` errors out (the `conn.Close` above guarantees this).
- [ ] Integration test: start a daemon, attach a subscriber over the IPC socket, send `SIGTERM`, assert the daemon exits within 2 s and `daemon.sock` is removed.
- [ ] `go test -race` clean: no goroutine leak detected by `go test -race` on the shutdown integration test.
- [ ] Existing single-shot IPC commands (`status`, `connect`, `send_chat`) still return normally before the daemon exits.

**Notes**: **Pickup order:** 3 | **GitHub:** #10. Files: `internal/daemon/ipc.go` (IPCServer.Close + handleSubscribe), `internal/daemon/daemon.go` (shutdown defer ordering). Sources: A-7, B-8, Q-BUG-2.

---

### [BOLT-005] daemon.pid symmetric write/remove on clean shutdown

**Type**: Bug fix (lifecycle) | **Priority**: P0 | **Owner**: Backend

**User Story**: As an operator, I want `daemon.pid` to exactly mirror process liveness, so that scripts and supervisors can rely on its presence to mean "the bolt daemon is running and accepting connections".

**Acceptance Criteria**:
- [ ] `daemon.pid` is written by `Daemon.Run` **after** `listenForPeers` succeeds, not by `spawnDaemon`.
- [ ] `daemon.pid` is removed in the `Daemon.Run` shutdown defer.
- [ ] `spawnDaemon` no longer touches the PID file; it relies on the socket poll for liveness (already in place at `spawn.go`).
- [ ] Integration test: start daemon, send `SIGTERM`, assert `~/.config/bolt/daemon.pid` does not exist.
- [ ] Integration test: simulate a failed spawn (bind port already taken) → `daemon.pid` does not exist after the spawn returns its error.
- [ ] Documentation note in code: `daemon.pid` is for human / supervisor introspection; the `flock` on it (already in `spawn.go`) is what guards against double-spawn.

**Notes**: **Pickup order:** 4 | **GitHub:** #11. Files: `internal/daemon/daemon.go` (Run), `internal/daemon/spawn.go` (spawnDaemon — remove PID write). Sources: A-7, B-4, Q-BUG-5, Q-BUG-12.

---

### [BOLT-006] Stream-handler registry on *Daemon (pluggable StreamType dispatch)

**Type**: Refactor (Phase-2 enabler) | **Priority**: P0 | **Owner**: Backend

**User Story**: As a backend engineer, I want `transfer.Service` to plug into `routeStream` without editing `daemon.go`, so that Phase-2 file transfer code can be added without churn in the daemon core.

**Acceptance Criteria**:
- [ ] New type `StreamHandler func(ctx context.Context, pc *transport.PeerConn, stream transport.Stream)`.
- [ ] `(*Daemon).RegisterStreamHandler(t proto.StreamType, h StreamHandler)` — concurrent-safe map (`sync.RWMutex`).
- [ ] `chat.Service.Register(d *Daemon)` wires `StreamChat` at boot; existing chat tests still pass.
- [ ] Unknown stream type → `stream.CancelRead(0x1000)`, `stream.CancelWrite(0x1000)`, log line `"unhandled stream type 0x%02x from %s — protocol violation"`. Define `proto.ErrCodeUnknownStreamType = 0x1000` as a named constant.
- [ ] Unit test (uses BOLT-010 loopback): register a handler for `0x05`, send a stream with that type, assert the handler is invoked with the correct `PeerConn` and stream.
- [ ] Unit test: send an unknown `0xFF` stream → stream is closed with code `0x1000`; no goroutine leak under `-race`.

**Notes**: **Pickup order:** 14 | **GitHub:** #12. Files: `internal/daemon/stream_handler.go` (new), `internal/daemon/daemon.go` (replace switch with registry lookup), `internal/chat/service.go` (auto-register at boot), `internal/proto/wire.go` (named error code). Sources: A-1, B-2, N-§5, Q-BUG-9.

---

### [BOLT-007] Lock quic.Config defaults (idle, windows, datagrams, 0-RTT, ALPN)

**Type**: Feature (protocol hardening) | **Priority**: P0 | **Owner**: Networking

**User Story**: As a bolt user transferring a file over LAN, I want a single QUIC stream to actually fill a gigabit pipe, and as a Phase-5 user transferring over the internet, I want connections to survive 60 s of idleness through home-router NATs.

**Acceptance Criteria**:
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

**Notes**: **Pickup order:** 10 | **GitHub:** #13. Files: `internal/transport/quic.go`, `internal/transport/tls.go`, `internal/transport/quic_test.go` (new). Sources: N-§1, N-decisions #3/#4/#5/#6/#7.

**Security addendum**: enforce a per-peer concurrent-stream budget in `transport.PeerConn` (cap at 32 chat streams + N parallel chunk streams; over-budget streams get `CancelRead/Write(0x1002)`); every `proto.ReadFrame` call site wraps the read in `stream.SetReadDeadline(time.Now().Add(30 * time.Second))` so a peer that announces a length but never sends bytes cannot park a goroutine + 8 MiB allocation indefinitely. Source: SEC-5.

---

### [BOLT-008] Freeze wire schema for Phase 2 (FileHeader, ChunkHeader, TransferAck, TransferDone, CancelMsg)

**Type**: Feature (protocol freeze) | **Priority**: P0 | **Owner**: Networking

**User Story**: As a backend engineer about to write the Phase-2 sender and receiver, I want every wire shape they exchange to be fully defined with golden round-trip tests, so that sender and receiver can be developed in parallel against a stable contract.

**Acceptance Criteria**:
- [ ] `FileHeader{v:1, transfer_id, name, size_bytes, chunk_size, total_chunks, file_hash}` defined and round-trip tested.
- [ ] `ChunkHeader{v:1, transfer_id, index, chunk_hash, len}` defined; wire framing on a chunk stream is `[1-byte StreamFile][length-prefixed JSON ChunkHeader][exactly ChunkHeader.len raw bytes]`. Stream ends after the raw bytes are written by the sender (one-chunk-per-stream).
- [ ] `TransferAck{v:1, transfer_id, accepted, reason, resume_from_chunks []int}` defined and round-trip tested.
- [ ] `TransferDone{v:1, transfer_id, file_hash}` defined and round-trip tested.
- [ ] `CancelMsg{v:1, transfer_id, reason}` defined and round-trip tested.
- [ ] `ChunkMsg.Data []byte` is **removed** from `internal/proto/wire.go`; any reference outside the deleted code site fails compilation.
- [ ] Round-trip test for a 4 MiB chunk via `ChunkHeader + raw bytes`: serialized size on wire is `≤ 4*(1<<20) + 256` bytes (no base64 inflation).
- [ ] `maxFrameSize` raised to 16 MiB for JSON envelopes only; documented in code that raw chunk bytes are not subject to this cap.

**Notes**: **Pickup order:** 13 | **GitHub:** #14. Files: `internal/proto/wire.go`, `internal/proto/wire_test.go`. Lock D1 (header-then-raw framing) and the chunk-stream contract before any sender/receiver code is written. Sources: B-9, N-§3, N-decision #1, Q-BUG-4.

**Security addendum**: replace `json.Unmarshal(data, dst)` in `proto.ReadFrame` (and the matching IPC frame reader in `internal/daemon/ipc.go:372`) with `dec := json.NewDecoder(...); dec.DisallowUnknownFields(); dec.Decode(dst)` — catches peer-introduced wire drift at the receiver and prevents silent acceptance of mis-shaped frames; lower JSON `maxFrameSize` to 128 KiB for control frames now that raw chunk bytes live outside the JSON envelope. Sources: SEC-5, SEC-8.

---

### [BOLT-009] Move IPC types to internal/proto + split internal/daemonclient

**Type**: Refactor (layering) | **Priority**: P0 | **Owner**: Architect + Backend

**User Story**: As a maintainer, I want `cmd/bolt` to import only `internal/proto` and `internal/daemonclient`, so that the CLI binary does not transitively drag in `quic-go`, `chat.Service`, and the daemon's accept loops just to name a command string.

**Acceptance Criteria**:
- [ ] `IPCRequest`, `IPCResponse`, `IPCEvent`, all payload structs (existing: `StatusPayload`, `ConnectPayload`, `SendChatPayload`, `ChatEventPayload`; new for Phase 2: `SendFilePayload`, `TransferProgressPayload`, `TransferCompletePayload`, `TransferSkippedPayload`, `TOFUPromptPayload`, `TOFUResponsePayload`) live in `internal/proto/ipc.go`.
- [ ] `writeIPCFrame` / `readIPCFrame` move to `internal/proto/ipc.go`.
- [ ] `IPCClient` moves to a new `internal/daemonclient/` package.
- [ ] `cmd/bolt/main.go` imports only `internal/proto` and `internal/daemonclient` (verified by `go list -deps ./cmd/bolt | grep internal/daemon` returning empty).
- [ ] `internal/daemon` retains only `IPCServer`, `acceptLoop`, `dispatch`, `handleConn`, `handleSubscribe`, plus the daemon orchestration.
- [ ] All existing tests still pass; no behaviour change.

**Notes**: **Pickup order:** 19 | **GitHub:** #15. Files: `internal/proto/ipc.go` (new — most content moves from `internal/daemon/ipc.go`), `internal/daemonclient/client.go` (new), `internal/daemon/ipc.go` (slim down), `cmd/bolt/main.go` (import updates). This is a precondition for BOLT-001 and BOLT-013 to add new IPC types in the right package. Sources: A-2.

---

### [BOLT-010] Loopback test harness internal/testutil/loopback.go

**Type**: Test scaffolding | **Priority**: P0 | **Owner**: Backend / QA

**User Story**: As a backend engineer writing Phase-2 integration tests, I want a one-call helper that returns two authenticated `PeerConn` instances connected over real in-process QUIC, so that I can test sender / receiver / TOFU / chat round-trips without mocks.

**Acceptance Criteria**:
- [ ] `internal/testutil/loopback.go` exposes `NewLoopbackPair(t *testing.T) (a, b *transport.PeerConn)`.
- [ ] Uses real Ed25519 identities (generated per-call into a `t.TempDir`), real TLS handshake, real QUIC listener on `127.0.0.1:0`.
- [ ] `t.Cleanup` closes both `PeerConn`s and the underlying listener — no goroutine leak under `-race` across 100 sequential calls.
- [ ] Used by at least one integration test in BOLT-001 (TOFU round-trip) and one in BOLT-008 (4 MiB chunk header-then-raw round-trip).
- [ ] Godoc on the harness names intended consumers: Phase-2 transfer tests, chat tests, TOFU prompt tests; documents the pair is *authenticated* and *handshake-complete* on return.

**Notes**: **Pickup order:** 7 | **GitHub:** #16. Files: `internal/testutil/loopback.go` (new). This unblocks every integration test in BOLT-001, BOLT-006, BOLT-008, BOLT-011, and Phase-2 transfer tests downstream. Sources: B-Phase-2-prereq §H, Q-coverage-gap.

**Security addendum**: the harness MUST exercise the strict-decode path from BOLT-008 (one round-trip test that sends a frame with an extra unknown JSON field and asserts the receiver rejects it via `DisallowUnknownFields`) so wire drift is caught the moment it lands rather than at Phase-5 user reports. Source: SEC-8.

---

### [BOLT-011] Tests for internal/config (load/save/migration + atomic-write kill test)

**Type**: Test coverage | **Priority**: P0 | **Owner**: Backend

**User Story**: As a maintainer, I want `internal/config` covered by unit tests including migration paths, version-too-new rejection, concurrent `PeerStore.Upsert` under `-race`, and a kill-during-write that asserts atomic-write semantics, so that future config changes do not silently corrupt user state.

**Acceptance Criteria**:
- [ ] `LoadOrInit` on a missing file initialises with `Version=1` and defaults; `Stat` confirms `0600` on private files.
- [ ] Migration test: write `Version=0` file, load → `Version=1`, all missing fields filled with defaults.
- [ ] Version-too-new test: write `Version=99` file, load → returns explicit error (does not silently downgrade or overwrite).
- [ ] Concurrent `PeerStore.Upsert` from 100 goroutines under `go test -race`: no race, final state has all 100 entries.
- [ ] Atomic-write kill test (depends on BOLT-003): inject `io.Writer` that fails after N bytes → original `peers.toml` is byte-identical to pre-call state.
- [ ] Per-package coverage on `internal/config` ≥ 70% as measured by `go test -cover`.

**Notes**: **Pickup order:** 8 | **GitHub:** #17. Files: `internal/config/config_test.go` (new), `internal/config/peers_test.go` (new). Plan §"Test strategy" explicitly mandates this; current coverage is 0%. Sources: B-Phase-2-prereq §H-3, Q-coverage-gap.

---

### P1 — Pre-Phase-2 (not all block Phase 2 entry)

### [BOLT-012] Fuzz test for wire framing (FuzzReadFrame) + length-prefix bounds

**Type**: Test scaffolding | **Priority**: P1 | **Owner**: QA

**User Story**: As a maintainer, I want `proto.ReadFrame` to never panic regardless of input bytes, so that a malformed or hostile peer cannot crash the daemon by sending crafted framing.

**Acceptance Criteria**:
- [ ] `internal/proto/wire_fuzz_test.go` defines `func FuzzReadFrame(f *testing.F)`.
- [ ] Seed corpus includes: empty input, length-prefix = 0, length-prefix > `maxFrameSize`, valid length + truncated payload, valid length + random-byte payload, valid JSON envelope with unknown fields.
- [ ] `ReadFrame` never panics on any input under the default fuzz budget; assertion uses `defer recover()` to fail the test on panic.
- [ ] Explicit unit tests for length-prefix bounds: length = 0 → returns `io.ErrUnexpectedEOF` or equivalent; length = `maxFrameSize + 1` → returns `ErrFrameTooLarge`.
- [ ] CI invocation `go test -fuzz=FuzzReadFrame -fuzztime=30s ./internal/proto/...` exits 0.

**Notes**: **Pickup order:** 9 | **GitHub:** #18. Files: `internal/proto/wire_fuzz_test.go` (new). Length-prefix framing is a classic crash surface; the cost of a fuzz target is small and the upside is bug-for-life. Sources: N-§3, Q-coverage-gap.

---

### [BOLT-013] internal/transfer package skeleton + Phase-2 IPC types (no behavior yet)

**Type**: Scaffolding (Phase-2 enabler) | **Priority**: P0 | **Owner**: Backend

**User Story**: As a backend engineer, I want the `internal/transfer` package and the new Phase-2 IPC types to exist as compile-clean stubs with godoc, so that Phase-2 PRs can land iteratively without inventing the layout at the same time as the algorithm.

**Acceptance Criteria**:
- [ ] `internal/transfer/chunk.go` declares `Chunk{Index int; Data []byte; Hash [32]byte}` and the `ChunkFile(r io.Reader, chunkSize int) (<-chan Chunk, <-chan error)` signature; body returns `ErrNotImplemented`.
- [ ] `internal/transfer/sender.go` declares `Sender.Send(ctx, pc, src, name, size) (TransferResult, error)` with godoc explaining the per-chunk-per-stream model from BOLT-008; body returns `ErrNotImplemented`.
- [ ] `internal/transfer/receiver.go` declares `Receiver.Accept(ctx, pc, FileHeader) (TransferAck, *Sink, error)` with godoc explaining trust-check + disk-space + sink semantics; body returns `ErrNotImplemented`.
- [ ] `internal/transfer/service.go` declares `Service` with `Register(*Daemon)` that wires a `StreamFile` handler (depends on BOLT-006) returning `ErrNotImplemented`.
- [ ] IPC types added (no handlers yet) in `internal/proto/ipc.go` (depends on BOLT-009): `CmdSendFile`, `EventTransferProgress`, `EventTransferComplete`, `EventTransferSkipped`, plus their payload structs.
- [ ] `go build ./...` PASS; `go vet ./...` PASS; `golangci-lint run` PASS. No behavior tests yet — skeleton only.

**Notes**: **Pickup order:** 15 | **GitHub:** #19. Files: `internal/transfer/{chunk.go, sender.go, receiver.go, service.go}` (new), `internal/proto/ipc.go` (additions). Explicitly excluded: any actual transfer behaviour, retry logic, resume, disk-space check — those land in Phase 2 proper. Sources: B-Phase-2-prereq §A-1 through §A-5, Architect's "Transport / IPCTransport / Discoverer interface view".

---

### [BOLT-014] Disk-space check helper (syscall.Statfs wrapper)

**Type**: Feature (Phase-2 enabler) | **Priority**: P1 | **Owner**: Backend

**User Story**: As a bolt receiver, I want to reject a transfer with `TransferAck{Accepted:false, Reason:"insufficient_disk"}` before any chunk is written, so that I do not half-write a 10 GiB file onto a 1 GiB-free partition.

**Acceptance Criteria**:
- [ ] `internal/transfer/diskspace_unix.go` with build tag `//go:build !windows` declares `FreeBytes(path string) (int64, error)` wrapping `syscall.Statfs`.
- [ ] Returns explicit error on a non-existent path (no zero-int silent failure that the caller might mistake for "no free space").
- [ ] Unit test: `FreeBytes(t.TempDir())` returns a positive int.
- [ ] Unit test: `FreeBytes("/nonexistent/path/that/cannot/exist")` returns a non-nil error.
- [ ] Build-tagged so the rest of the codebase compiles on Linux + macOS regardless of the Windows decision in BOLT-016.

**Notes**: **Pickup order:** 16 | **GitHub:** #20. Files: `internal/transfer/diskspace_unix.go` (new), `internal/transfer/diskspace_test.go` (new). Sources: B-Phase-2-prereq §C-2.

---

### [BOLT-015] CI workflow: build + vet + lint + test + race on linux/macOS

**Type**: DevOps | **Priority**: P1 | **Owner**: DevOps

**User Story**: As a maintainer, I want every PR to be gated by `build + vet + lint + test + test -race` on both `ubuntu-latest` and `macos-latest`, so that platform-specific bugs are caught before merge.

**Acceptance Criteria**:
- [ ] `.github/workflows/ci.yml` runs a matrix on `ubuntu-latest` and `macos-latest` with Go 1.25.x.
- [ ] Steps: `go build ./...`, `go vet ./...`, `golangci-lint run`, `go test ./...`, `go test -race ./...`.
- [ ] GitHub Actions are pinned by SHA (consistent with existing `govulncheck` / `trivy` / `CodeQL` jobs noted in backend review §"Strengths").
- [ ] `go mod tidy -diff` or equivalent guard fails the build on a dirty `go.sum`.
- [ ] Coverage is uploaded as an artifact; a `make ci-coverage-gate` target asserts ≥ 60% per non-`cmd` package (initially advisory; flipped to required after BOLT-010 + BOLT-011 land).

**Notes**: **Pickup order:** 17 | **GitHub:** #21. Files: `.github/workflows/ci.yml`, `Makefile`. Sources: B-Phase-2-prereq §I-1.

---

### [BOLT-016] Windows posture decision: remove Windows code OR commit to IPCTransport interface

**Type**: Spike + ADR | **Priority**: P1 | **Owner**: PO (spike) + Architect (ADR-002)

**User Story**: As Savvy, I want the v1 Windows posture committed in one place (code, README, plan, goreleaser), so that contributors do not continue adding Windows-specific code to a project whose plan says "Linux + macOS only".

**Acceptance Criteria**:
- [ ] PO writes a 1-page spike summarising Option A (`IPCTransport` interface + keep `*_windows.go`) vs Option B (delete `*_windows.go` + drop PowerShell installer + drop Windows from `.goreleaser.yaml`).
- [ ] Architect drafts ADR-002 reflecting Savvy's chosen option.
- [ ] **If Option B (PO recommendation)**: delete `internal/daemon/spawn_windows.go` and `internal/daemon/ipc_endpoint_windows.go`; remove the Windows install lines from `README.md` (lines 29, 46-54, 66); drop the `windows` entries from `.goreleaser.yaml` and `Makefile release-local`; add `//go:build !windows` build constraint on `cmd/bolt/main.go` only if needed to keep the build clean. `plan.md` line 70-71 stays as-is.
- [ ] **If Option A**: introduce `IPCTransport` interface (`Listen(configDir) (net.Listener, error)`, `Dial(configDir) (net.Conn, error)`, `Cleanup(configDir)`, `Reachable(configDir) bool`) in `internal/daemon`; route both Unix-socket and Windows-loopback implementations through it; update `plan.md` line 29 + 70-71 to remove "Windows unsupported"; document the Windows install in `README.md` with caveats.
- [ ] `go build ./...` PASS on the chosen target set (Linux + macOS for Option B; +Windows for Option A).
- [ ] `.goreleaser.yaml` and Makefile match the chosen target set.

**Notes**: **Pickup order:** 18 | **GitHub:** #22. Files: `internal/daemon/spawn_windows.go`, `internal/daemon/ipc_endpoint_windows.go`, `README.md`, `.goreleaser.yaml`, `Makefile`, `docs/adr/ADR-002-windows-posture.md` (new). Sources: A-5.

---

### [BOLT-024] Bump Go toolchain to 1.25.10 to close 15 reachable stdlib advisories

**Type**: Bug fix (security) | **Priority**: P0 | **Owner**: Backend

**User Story**: As a bolt user whose daemon faces the LAN (and post-Phase-5 the open internet), I want the Go stdlib running my TLS/QUIC stack to be patched, so that a network attacker cannot trigger known DoS or state-confusion bugs via every handshake.

**Acceptance Criteria**:
- [ ] `go.mod` line 3: `go 1.25.1` → `go 1.25.10` (or current latest patch on the 1.25.x line at merge time).
- [ ] `go.mod` gains a `toolchain go1.25.10` directive so contributors with older Go fail loudly instead of silently building against an unpatched stdlib.
- [ ] `go mod tidy` clean; `go build ./...`, `go vet ./...`, `golangci-lint run`, `go test ./...`, `go test -race ./...` all PASS.
- [ ] `govulncheck ./...` exits 0 (no reachable advisories) — paste the before/after Symbol Results section into the PR description for traceability.
- [ ] CI workflows already pin via `go-version-file: go.mod`; verify the next CI run on the PR installs 1.25.10 automatically.

**Notes**: **Pickup order:** 1 | **GitHub:** #33. Files: `go.mod`. One-PR change, no decision deps, no source-code churn. Closes 15 reachable stdlib advisories incl. GO-2026-4870 (TLS 1.3 KeyUpdate DoS, fixed in 1.25.9), GO-2025-4011 (ASN.1 DER memory exhaustion, fixed in 1.25.2), GO-2025-4009 (quadratic PEM parsing, fixed in 1.25.2), GO-2026-4340 (handshake at wrong encryption level, fixed in 1.25.6). Sources: SEC-1 (`.agent/reports/security-review.md` §New findings — Critical).

---

### [BOLT-025] Sanitize peer-controlled strings before TTY render / persistence

**Type**: Bug fix (security) | **Priority**: P1 | **Owner**: Backend

**User Story**: As a bolt user, I want peer-supplied nicknames and chat bodies to be stripped of ANSI/CSI/OSC escape sequences before they hit my terminal or get persisted to `peers.toml`, so that a malicious peer cannot clear my scrollback, set my terminal title, forge prior chat lines via `\r`, hide invisible text with `\x1b[8m`, or hijack my clipboard via OSC-52 on macOS Terminal.app.

**Acceptance Criteria**:
- [ ] New helper `func SanitizeDisplay(s string, maxRunes int) string` in `internal/proto/sanitize.go` replaces every byte `< 0x20` (except `\n`, `\t`) and `\x7f` with the literal escaped form (`\x1b` → `\\x1b`) and truncates to `maxRunes`.
- [ ] Called on **receive** in `internal/transport/peer_conn.go::validateHandshake` (nickname; cap 64 runes) before assigning to `pc.peerNickname`.
- [ ] Called on **receive** in `internal/chat/service.go::AttachStream` for `msg.From` and `msg.Body` (cap From at 64 runes, Body at 4 KiB) before passing into `s.emit`.
- [ ] Called on **display** in `cmd/bolt/main.go::chatCmd` before `fmt.Printf` of `payload.From` and `payload.Body` (defence in depth).
- [ ] Unit test: nickname containing `"alice\x1b[2J\x1b[H"` round-trips through the daemon as `"alice\\x1b[2J\\x1b[H"`; chat body containing `\r` is escaped, not rendered.
- [ ] Wire bytes are NOT sanitized at marshal — keep the wire faithful for forensic logs; sanitize on display only.

**Notes**: **Pickup order:** 5 | **GitHub:** #34. Files: `internal/proto/sanitize.go` (new), `internal/proto/sanitize_test.go` (new), `internal/transport/peer_conn.go`, `internal/chat/service.go`, `cmd/bolt/main.go`. Independent of other stories — can land any time. Pair with BOLT-001 so the TOFU prompt itself safely renders peer-supplied nicknames. Sources: SEC-2 (`.agent/reports/security-review.md` §New findings — High; file:line evidence at `internal/daemon/daemon.go:188,190,210` and `cmd/bolt/main.go:281`).

---

### [BOLT-026] Restrict spawned daemon environment to an explicit allowlist

**Type**: Bug fix (security / defence-in-depth) | **Priority**: P1 | **Owner**: Backend

**User Story**: As a bolt user, I want the long-lived daemon to NOT inherit my shell's secrets (`AWS_SECRET_ACCESS_KEY`, `GITHUB_TOKEN`, `OPENAI_API_KEY`, `SSH_AUTH_SOCK`, …), so that any future RCE in the daemon cannot exfiltrate credentials I never intended to share with bolt.

**Acceptance Criteria**:
- [ ] `internal/daemon/spawn.go` sets `cmd.Env` to an explicit allowlist: `PATH`, `HOME`, `USER`, `LANG`, `TZ` (and `BOLT_LOG`, `BOLT_QLOG` once observability lands — gated on whether they are set in the parent, no blank entries).
- [ ] Unit / integration test spawns the daemon with synthetic env `SHOULD_NOT_LEAK=secret` set in the parent, then asserts the variable is absent from the child's environment (read via `/proc/<pid>/environ` on Linux, or via a test-only IPC introspection command).
- [ ] No behaviour regression in `bolt init` / `bolt daemon` / `EnsureRunning` — manual smoke check: `ps Eww -p $(cat ~/.config/bolt/daemon.pid)` shows only the allow-listed variables.
- [ ] Documented in code comment that adding a new env var to the allowlist is a deliberate decision and should be reviewed.

**Notes**: **Pickup order:** 6 | **GitHub:** #35. Files: `internal/daemon/spawn.go` (lines 43-50), `internal/daemon/spawn_test.go` (new). Independent of other stories. Sources: SEC-3 (`.agent/reports/security-review.md` §New findings — High).

---

## Backlog

P2 / P3 items that are not blocking Phase 2.

### [BOLT-017] Observability baseline: adopt log/slog + counter registry

**Type**: Feature (operations) | **Priority**: P2 | **Owner**: Backend (with DevOps review)

**Acceptance Criteria**:
- [ ] All `fmt.Fprintf(os.Stderr, ...)` call sites in `internal/daemon`, `internal/transport`, `internal/chat` replaced with `log/slog`.
- [ ] Default `slog.TextHandler`; `slog.JSONHandler` when `BOLT_LOG=json`.
- [ ] In-memory counter registry exposing: `peers_connected_total`, `peers_disconnected_total`, `tls_handshake_failures_total`, `tofu_prompts_total{decision=accept|reject|timeout}`, `chat_messages_sent_total`, `chat_messages_received_total`, `transfer_bytes_sent_total{path=lan|turn}`, `transfer_bytes_received_total{path=lan|turn}`.
- [ ] `bolt status` JSON includes a `counters` map, version-bumped in `StatusPayload`.

**Notes**: Sources: A-14. If Savvy answers D9 = "Phase 1", this becomes P0 and moves into Ready-for-Dev; today it sits in backlog per default ordering.

---

### [BOLT-018] Public pkg/bolt API surface decision (or commit to CLI-only)

**Type**: ADR + scaffolding | **Priority**: P2 | **Owner**: Architect + PO

**Acceptance Criteria**:
- [ ] ADR-011 written deciding: carve out `pkg/bolt` (library) or commit to CLI-only for v1.
- [ ] **If CLI-only**: remove "library" / "library/tool" framing from `README.md` and `architecture.md`.
- [ ] **If library**: `pkg/bolt/{client.go, identity.go, events.go, peer.go}` exposes `Identity`, `Client` (wraps `IPCClient`), `Event`, `Peer`; imports only `internal/proto` and `internal/daemonclient`.

**Notes**: Sources: A-15. PO recommends CLI-only for v1 (D6).

---

### [BOLT-019] QUIC datagrams enablement carve-out for Phase 4 chat presence

**Type**: Feature (forward compat) | **Priority**: P3 | **Owner**: Networking

**Acceptance Criteria**:
- [ ] Define `ChatPresenceMsg{v:1, fingerprint, status, ts}` and `TypingMsg{v:1, fingerprint, ts}` in `internal/proto/wire.go` with JSON encoded payload ≤ 1100 bytes (PMTUD-safe).
- [ ] Documented `proto.DatagramMaxPayload = 1100` constant.
- [ ] No behavior wired; just the types and the doc note.

**Notes**: Sources: A-11, N-§1 / N-§8. `EnableDatagrams=true` is already required by BOLT-007, so this is forward-compatible without a wire-version bump.

---

### [BOLT-020] Create ADRs 001-011 under docs/adr/

**Type**: Documentation | **Priority**: P2 | **Owner**: Architect

**Acceptance Criteria**:
- [ ] ADR-001 (Transport interface) through ADR-011 (`pkg/bolt`) drafted and committed to `docs/adr/`.
- [ ] Each ADR references the architect-critique section it closes.
- [ ] Status: Proposed; PO flips to Accepted once Savvy signs off on the corresponding decision.

**Notes**: Sources: Architect critique §"Proposed ADRs to create". ADR-002 specifically is owned by BOLT-016.

---

### [BOLT-021] IPC subscriber per-conn bounded queue (fix BUG-10)

**Type**: Bug fix (concurrency) | **Priority**: P2 | **Owner**: Backend

**Acceptance Criteria**:
- [ ] Per-subscriber buffered channel `chan IPCEvent` (buffer 128); each `handleSubscribe` runs its own writer goroutine.
- [ ] `PublishChat` does non-blocking send under brief mutex: `select { case ch <- evt: default: drop+log }`.
- [ ] Integration test: suspending one subscriber (close its read side or `SIGSTOP` the CLI in a test rig) does not delay event publication to other subscribers.
- [ ] No goroutine leak under `-race` over a load test of 1000 events to 10 subscribers.

**Notes**: Sources: A-12, Q-BUG-10.

---

### [BOLT-022] README + QUICKSTART alignment after Windows decision

**Type**: Documentation | **Priority**: P2 | **Owner**: PO / DevOps

**Acceptance Criteria**:
- [ ] After BOLT-016 lands, `README.md` install / quickstart matches the committed posture exactly.
- [ ] `CONTRIBUTING.md` (create if missing) notes the supported target platform set.
- [ ] No dangling references to Windows-only commands or paths in any docs.

**Notes**: Sources: A-5, derived. Cannot start until BOLT-016 is signed off.

**Security addendum**: default `Config.Nickname` changes from `os.Hostname()` to `"bolt-" + first-6-chars(fingerprint)` (so cafe-Wi-Fi peers do not see `acme-corp-laptop-12345`); `bolt init` first-run output prints "your nickname is visible to every peer — edit `~/.config/bolt/config.toml` to change it". Source: SEC-10.

---

### [BOLT-023] bolt chat command rewritten as bubbletea TUI

**Type**: Feature | **Priority**: P2 | **Owner**: Backend

**Acceptance Criteria**:
- [ ] `internal/tui/chat_model.go` implements a `bubbletea.Model` with `viewport` (history) + `textinput` (entry).
- [ ] Handles `tea.WindowSizeMsg` for terminal reflow on resize.
- [ ] IPC events arrive as `tea.Msg`s through a subscriber goroutine fed into the program loop; no separate `bufio.Scanner` reading stdin.
- [ ] `Ctrl+C` quits cleanly: IPC subscriber connection is closed, terminal restored.
- [ ] Replaces the raw `bufio.Scanner` + `fmt.Print("> ")` path in `cmd/bolt/main.go::chatCmd`.

**Notes**: Files: `internal/tui/{chat_model.go, styles.go}` (new), `cmd/bolt/main.go` (chatCmd rewrite). Sources: A-20, Q-NTH (`bufio.NewScanner` truncates long lines).

---

### [BOLT-027] Refuse insecure config-dir permissions; O_NOFOLLOW|O_EXCL on first writes

**Type**: Bug fix (security / hardening) | **Priority**: P2 | **Owner**: Backend

**User Story**: As a bolt user on a multi-user host, I want `bolt init` to refuse to run if `~/.config/bolt/` exists with mode wider than `0700`, and I want `private.key` / `ipc.token` / `daemon.pid` to never be written through an attacker-planted symlink, so that a low-privilege local attacker cannot redirect bolt's first-time writes onto `~/.ssh/id_ed25519` or any other file I can write to.

**Acceptance Criteria**:
- [ ] After `os.MkdirAll(dir, 0700)` in `internal/config/config.go` and `peers.go`, call `os.Stat` and error out if `info.Mode().Perm() & 0o077 != 0` — explicit error names the offending path and required mode.
- [ ] First-write of `private.key` (`internal/identity/identity.go:206`), `ipc.token` (`internal/daemon/ipc_token.go:38`), and `daemon.pid` (post-BOLT-005, owned by `Daemon.Run`) uses `os.OpenFile(path, O_WRONLY|O_CREATE|O_EXCL|O_NOFOLLOW, 0600)`.
- [ ] `writeAtomic` from BOLT-003 opens its `*.tmp` file with `O_EXCL|O_NOFOLLOW|O_CREATE|O_WRONLY` so the temp file cannot be redirected through a pre-planted symlink either.
- [ ] Unit test: pre-create `~/.config/bolt/private.key` as a symlink to `/tmp/pwn`, run `Identity.Generate`, assert it errors (does NOT write through the symlink) and `/tmp/pwn` is unchanged.
- [ ] Unit test: pre-create `~/.config/bolt/` with mode 0755, run `LoadOrInit`, assert it errors with a message naming the directory and "0700".
- [ ] Documented in `architecture.md` (or follow-up `SECURITY.md`) that the config dir must be `0700` and bolt enforces this.

**Notes**: **Pickup order:** — (after Wave 1) | **GitHub:** #36. Files: `internal/identity/identity.go`, `internal/daemon/ipc_token.go`, `internal/daemon/daemon.go` (PID write post-BOLT-005), `internal/config/{config.go, peers.go, atomic.go}`. Depends on BOLT-003 (for the atomic-write temp-file flag change) and BOLT-005 (for the PID write moving into `Daemon.Run`). Sources: SEC-6 (`.agent/reports/security-review.md` §New findings — Medium).

---

## In Progress

_None — next pickup: BOLT-010 / #16 (loopback harness)_

## Done

| ID | GitHub | Branch | PR |
|---|---|---|---|
| BOLT-024 | #33 | `bolt-024-go-toolchain` | https://github.com/g-savitha/bolt/pull/37 |
| BOLT-003 | #9 | `bolt-003-atomic-writes` | https://github.com/g-savitha/bolt/pull/38 |
| BOLT-004 | #10 | `bolt-004-ipc-shutdown` | https://github.com/g-savitha/bolt/pull/39 |
| BOLT-005 | #11 | `bolt-005-daemon-pid` | https://github.com/g-savitha/bolt/pull/40 |
| BOLT-025 | #34 | `bolt-025-sanitize-tty` | https://github.com/g-savitha/bolt/pull/41 |
| BOLT-026 | #35 | `bolt-026-daemon-env` | https://github.com/g-savitha/bolt/pull/42 |
