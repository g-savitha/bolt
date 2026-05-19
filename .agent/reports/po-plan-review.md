# PO Plan Review — 2026-05-19

Reviewer: Product Owner. Sources: architect critique, backend review, networking review, QA bug hunt (all in `.agent/reports/`). No re-investigation; this is synthesis only.

---

## TL;DR

- `plan.md` is structurally sound and fit-for-purpose for Phase 2 **with a focused set of edits and a short pre-Phase-2 sprint** — not as-is.
- Four blockers are non-negotiable before Phase 2 code is cut: (1) TOFU is documented but unwired and silently accepts every peer (A-3, B-1, N-§2, Q-BUG-1); (2) `ChunkMsg.Data` is JSON+base64-encoded and breaches the 8 MiB envelope cap at 4 MiB chunks (B-9, N-§3, Q-BUG-4); (3) `peers.toml` / `config.toml` writes are truncate-then-encode (A-19, B-3, Q-BUG-3); (4) `quic.Config` is silently under-specified — no `MaxIdleTimeout`, no receive-window tuning, no 0-RTT posture, no datagram bit (N-§1).
- 10 binary decisions must be locked by Savvy before any Phase-2 sender/receiver code is written. They are all already pre-recommended by Networking; PO is asking for explicit sign-off, not re-debate.
- The Windows posture in `plan.md` line 29/70-71 ("Linux + macOS only") is silently contradicted by shipped Windows code and a PowerShell install line in `README.md` (A-5). Savvy must commit: delete the Windows files, or commit to an `IPCTransport` interface. PO recommends Option B (delete) for v1 to keep scope honest.
- The smallest set of edits to close the plan/code gap is captured in §"Where plan.md needs editing" below — ~10 surgical edits, none of which change the phase boundaries.
- **Backend pickup order** is locked in §"Backend pickup order (sequential)" below and mirrored in `.agent/backlog/tasks.md` — work top-to-bottom; one PR per BOLT unless tightly coupled (BOLT-004+BOLT-005 may share one PR).
- **GitHub traceability**: 30 issues filed on `g-savitha/bolt` (#7–#36) via `.agent/reports/file-issues.sh` (2026-05-19). Re-run the script after `gh auth login` if titles are missing; it is idempotent (skips open duplicates).

---

## Where plan.md needs editing (concrete diffs)

| # | Section in plan.md | What plan says today | What reviewers showed is wrong / missing / under-specified | Proposed edit (paste-ready) | Source |
|---|---|---|---|---|---|
| 1 | §"TOFU fingerprint UX" (line 64-65) and Phase 1 verification bullet "Unknown peer → TOFU prompt with randomart" (line 248) | Implies TOFU prompt is in place; "Accept? [y/N]" with randomart. | `peerVerifier` auto-accepts every non-blocked peer; server `VerifyPeerCertificate` is not even installed; no `tofu_prompt` IPC event, no `tofu_response` command in the IPC schema. | Replace the Phase-1 verification bullet with: *"Unknown peer → daemon emits `tofu_prompt` IPC event with fingerprint + randomart; CLI subscriber renders randomart and prompts y/N; daemon blocks the verifier on a per-nonce channel up to 30 s; on timeout, **default reject**; on accept, persist trust level via atomic write to `peers.toml`; second connection from `always-allow` peer → no prompt; from `allow-once` peer → prompt repeats."* | A-3, B-1, N-§2, Q-BUG-1 |
| 2 | §"Key Data Structures — Wire Protocol" (line 209-220) | `ChunkMsg` listed as a JSON message type carrying chunk data; framing capped at 8 MiB. | `ChunkMsg.Data []byte` is base64-encoded by `encoding/json` (~33% inflation); 4 MiB chunk → ~5.6 MiB envelope; JSON unmarshal of multi-MB blobs creates GC pressure; 8 MiB cap is tight. | Replace ChunkMsg bullet with: *"`FileHeader`, `TransferAck`, `TransferDone`, `CancelMsg` are length-prefixed JSON. Chunk payloads use **header-then-raw framing**: each chunk opens a dedicated `StreamFile` chunk-stream carrying `[stream_type][length-prefixed ChunkHeader JSON][chunk_len][raw chunk bytes]`. `ChunkHeader = {v:1, transfer_id, index, chunk_hash, len}`. `maxFrameSize` raised to 16 MiB for JSON envelopes; raw chunk bytes are not subject to the envelope cap."* | B-9, N-§3, Q-BUG-4 |
| 3 | §"Phase 1 — Core Transport" step 5 (line 237) | `MaxIncomingStreams:1000, KeepAlivePeriod:15s` — that's it. | No `MaxIdleTimeout`, `EnableDatagrams`, `Allow0RTT`, receive-window tuning, ALPN lock, or `SessionTicketsDisabled`. Default 6 MiB max stream window caps a single stream at ~30 MB/s on a 200 ms RTT; 15 s keep-alive is *less than* the 30 s default idle timeout — works by accident. | Replace step 5 with the locked `quic.Config` block: *"`MaxIdleTimeout=60s`, `KeepAlivePeriod=20s`, `HandshakeIdleTimeout=10s`, `EnableDatagrams=true`, `Allow0RTT=false`, `InitialStreamReceiveWindow=4 MiB`, `MaxStreamReceiveWindow=64 MiB`, `InitialConnectionReceiveWindow=8 MiB`, `MaxConnectionReceiveWindow=256 MiB`, `MaxIncomingStreams=1000`. TLS: `NextProtos=[bolt/1]`, `SessionTicketsDisabled=true`, `MinVersion=TLS13`."* Add citations RFC 9000 §10.1, RFC 9001 §5.6. | N-§1, N-decisions #3/4/5/6/7 |
| 4 | §"Design Decisions" line 17 + §"Resolved Design Issues — TOFU fingerprint UX" (line 64-65) | "Ed25519 keypair per machine, TOFU (SSH-style)". Implies SSH-style proof of host key. | Cert↔Ed25519-key binding is by construction (`SubjectKeyId`), not by signature; an attacker copying the Ed25519 public key can mint a cert that fingerprints identically. SSH proves the host key during the live handshake; bolt does not today. | Add a new resolved-design-issue: *"**Ed25519 proof-of-possession.** After TLS completes, each side signs `tls.ExportKeyingMaterial("bolt-handshake", nil, 32)` (RFC 5705) with its Ed25519 private key and includes the signature in `HandshakeMsg.Signature`. Receiver verifies against the public key from `SubjectKeyId`; mismatch → close stream with error code 0x1001. This closes the cert/key-binding hole."* | N-§2, N-decision #8 |
| 5 | §"Resolved Design Issues — Concurrent peers.toml writes" (line 76-77) | Says all writes go through `PeerStore` under `sync.RWMutex`. Implies durability. | `flushLocked` does `O_WRONLY\|O_CREATE\|O_TRUNC` with no fsync and no rename. Crash mid-write loses trust state; combined with the silent-accept TOFU bug, lost trust state silently re-accepts everyone. | Add to that section: *"Writes go through `writeAtomic(path, fn)` — encode to `path.tmp` in the same directory, `f.Sync()`, `f.Close()`, then `os.Rename`. Same helper used by `config.toml` and `peers.toml`. A crash leaves the file either at the previous version or the new version, never empty or half-written."* | A-19, B-3, Q-BUG-3 |
| 6 | §"Design Decisions" line 29 + §"Resolved Design Issues — Windows unsupported" (line 70-71) | "Linux + macOS only; Windows support is post-v1." | `internal/daemon/spawn_windows.go` and `internal/daemon/ipc_endpoint_windows.go` are committed; `README.md:29-66` advertises a Windows install; `.goreleaser.yaml` ships a Windows binary. Reality is shipping Windows, plan denies it. | Commit one way. PO recommends: *"v1 supports Linux + macOS only. Windows files (`*_windows.go`) and README install lines are removed in BOLT-016. Post-v1 Windows support is gated on the `IPCTransport` interface (ADR-002 placeholder)."* If Savvy picks Option A, replace this paragraph with the `IPCTransport` interface contract from the architect's ADR-002 sketch. | A-5 |
| 7 | §"Resolved Design Issues — Signal handling in daemon" (line 73-74) | "On signal: ... remove `daemon.sock`, update `daemon.pid`." | `daemon.pid` is *never* removed on clean shutdown; it is *written by `spawnDaemon` before the child proves liveness*. After a clean shutdown the PID file points at a dead PID; after a failed spawn it points at a never-listening PID. Plus: `IPCServer.Close` does not close active subscriber conns → daemon hangs forever on SIGTERM if any subscriber is attached. | Replace bullet with: *"On signal: cancel run context → close listener and all live IPC subscriber connections (so `handleSubscribe` returns from `conn.Read`) → close all peer QUIC conns → checkpoint in-progress transfers → remove `daemon.sock` → remove `daemon.pid` → exit within 5 s. `daemon.pid` is written by `Daemon.Run` after the listener succeeds, not by `spawnDaemon`."* | A-7, B-4, B-8, Q-BUG-2, Q-BUG-5, Q-BUG-12 |
| 8 | §"Test strategy" (line 100-101) | Unit tests required for `internal/config/`, `internal/proto/wire.go`, `internal/identity/`, `internal/transfer/chunk.go`. Integration tests via "two in-process QUIC endpoints". | Loopback harness does not exist. `internal/config` and `internal/daemon` are at 0% coverage. Plan does not name a single file where the loopback harness lives. | Replace with: *"`internal/testutil/loopback.go` exposes `NewLoopbackPair(t *testing.T) (a, b *transport.PeerConn)` over real in-process QUIC + real Ed25519 identities. Every integration test that requires two endpoints uses this. Unit-test floor: ≥ 60% coverage per non-cmd package, enforced by `make ci-coverage-gate`. Fuzz target: `FuzzReadFrame` in `internal/proto`."* | B-Phase-2-prereq §H, Q-coverage-gap |
| 9 | §"Key Data Structures — IPC Protocol" (line 222-224) | IPC types live in `internal/proto/ipc.go`. | They live in `internal/daemon/ipc.go`; `cmd/bolt/main.go` imports `internal/daemon` just to name command strings, dragging transport + chat + quic-go into the CLI binary. | Add an implementation note: *"IPC request/response/event types and framing helpers live in `internal/proto/ipc.go`. `IPCClient` lives in `internal/daemonclient/`. `cmd/bolt` imports only `internal/proto` and `internal/daemonclient` — never `internal/daemon`."* | A-2 |
| 10 | §"Critical Files" (line 363-374) and project structure (line 124-185) | Lists `internal/transport/peer_conn.go` first; no mention of an abstraction. | `*quic.Stream` / `*quic.Conn` leak from `internal/transport` into `internal/chat` and `internal/daemon` — every consumer is bound to `quic-go`. Future test-loopback, WebTransport, or `quic-go` breaking change forces touching every site. | Add a note under "Critical Files": *"`internal/transport` exposes `transport.Conn` and `transport.Stream` interfaces (io.ReadWriteCloser + `Close(code, msg)` + `StreamID()`). The `quic-go` types do not leave the `internal/transport` package. Future transports (in-process loopback, WebTransport) implement the same interfaces."* | A-1 |
| 11 | §"Phase 6 — Polish + Release" (line 332-340) | No mention of observability. | All current logs are ad-hoc `fmt.Fprintf(os.Stderr, ...)`. No leveling, no JSON output, no counters. Debugging a P2P transport in the wild needs structured logs. | Add a Phase-6 step: *"Adopt `log/slog` throughout; text handler by default, JSON when `BOLT_LOG=json`. Counters: `peers_connected_total`, `tls_handshake_failures_total`, `tofu_prompts_total{decision}`, `transfer_bytes_sent_total{path}`, exposed via `bolt status`."* This can also be moved to Phase 1 if Savvy wants it earlier. | A-14 |
| 12 | §"Context" (line 5) + README "Open source client (this repo)" | Frames bolt as "library/tool"; everything is under `internal/`. | Nothing is importable by other Go programs. Either commit to a public `pkg/bolt` API or remove the "library" framing. | Add a one-liner in §"Known Limitations" (line 377-385): *"v1 is CLI-only. `pkg/bolt` library surface is post-v1 (see ADR-011)."* If Savvy chooses to carve out `pkg/bolt`, replace with the contract from ADR-011. | A-15 |

---

## Decisions Savvy must lock before Phase 2 starts

These are binary or short multi-choice. Recommendations are pre-cited; PO is asking for sign-off, not re-debate. Until all 10 are answered, Phase 2 sender/receiver code cannot start.

| # | Decision | Recommended answer | Rationale (one sentence) | Consequence if deferred |
|---|---|---|---|---|
| D1 | Chunk payload framing: JSON+base64 in one frame vs **header-then-raw** on a dedicated stream. | **Header-then-raw.** | Removes 33% wire overhead and a multi-MB JSON unmarshal per chunk; eliminates the 8 MiB envelope cap risk (N-§3, B-9, Q-BUG-4). | Phase 2 hits a hard `message too large` on the first 4 MiB LAN chunk; transfer feature is DOA. |
| D2 | Parallel stream count per path: constant 8 vs **path-aware (LAN=8, ICE-direct=4, TURN=1)**. | **Path-aware.** | 8 streams over a single TURN allocation actively hurt (sender-side parallelism, not network-side); QUIC already congestion-controls the connection (N-§4). | TURN-relayed transfers are slower than necessary and burn relay bandwidth on overhead. |
| D3 | `quic.Config` defaults locked: `MaxIdleTimeout=60s`, `KeepAlivePeriod=20s`, `EnableDatagrams=true`, `Allow0RTT=false`, receive windows tuned. | **Yes, lock all 10 fields in one PR.** | Defaults inherited from `quic-go` v0.59.1 leave a single stream below 30 MB/s on 200 ms RTT and silently rely on a coincidence between keep-alive and idle timeout (N-§1). | Throughput goal ("gigabit on LAN") is unreachable; intercontinental transfers in Phase 5 cap at a few MB/s. |
| D4 | Ed25519 proof-of-possession in `HandshakeMsg.Signature` (over RFC 5705 keying material). | **Yes, ship before Phase 2.** | Without it, an attacker who copies a peer's Ed25519 public key can mint a cert that fingerprints identically — TOFU is weaker than SSH's (N-§2). | The TOFU model is advertised as SSH-style but is not; a single LAN-side MITM defeats it. |
| D5 | Windows posture: **commit to `IPCTransport` interface (Option A)** vs delete Windows code + PowerShell installer (Option B). | **Option B (delete) for v1.** | Reality (Windows files + installer) silently violates plan.md line 29/70-71; v1 scope is honest only if Windows is genuinely deferred (A-5). | Contributors keep adding Windows-flavoured code in a project that "doesn't support Windows"; release artifacts drift from documentation. |
| D6 | Library + CLI vs **CLI-only for v1**. | **CLI-only.** | Nothing is importable today (everything under `internal/`); README's "library/tool" framing is aspirational (A-15). Carving out `pkg/bolt` is a post-v1 ADR. | Public API surface accretes by accident; future refactors break unknown downstream importers. |
| D7 | Cut a `v0.1` tag (Phase-1 only, no transfer) before starting Phase 2. | **Yes.** | Gives Savvy a fixed Phase-1 artifact to point at while Phase 2 churns the wire schema; isolates regression scope. | Phase 2 changes get blamed for Phase 1 bugs; no rollback target if Phase 2 stalls. |
| D8 | Move IPC types to `internal/proto/ipc.go` and split `internal/daemonclient/` from `internal/daemon`. | **Yes.** | `cmd/bolt` imports `internal/daemon` only for command-name constants today; that drags chat + transport + quic-go into the CLI binary (A-2). | Layering inversion compounds as Phase 2 adds new IPC commands; eventual refactor is painful. |
| D9 | Observability baseline (adopt `log/slog`, counter registry, JSON output gated by `BOLT_LOG=json`) — **Phase 1** or Phase 6. | **Phase 1.** | The cost is small now (one PR replacing ad-hoc `fmt.Fprintf(os.Stderr, …)`); the cost in Phase 5 (relay, ICE) is enormous (A-14). | Phase 5 debugging is blind; users hit weird internet-path failures and there is no log to grep. |
| D10 | Resume-state checkpoint granularity: every-N-chunks vs **every-N-MiB-of-acknowledged-data**. | **Every 32 MiB of acknowledged data.** | "Every 10 chunks" couples redo work to chunk size (40 MiB on LAN, 2.5 MiB on TURN) — asymmetric and not in proportion to risk (N-§4). | Crash in the middle of a 256 KiB-chunk TURN transfer redos 2.5 MiB unnecessarily; crash in the middle of a 4 MiB-chunk LAN transfer redos 40 MiB. |

---

## Phase 1 acceptance gap (must close before Phase 2)

Closing these IS the pre-Phase-2 sprint. Each maps to a BOLT story below.

- [ ] **`bolt init` → identity files exist with correct permissions** — PASS today (covered by `identity_test.go`).
- [ ] **`bolt id` → same fingerprint on repeated calls** — PASS today.
- [ ] **`tcpdump` on UDP 7799 → zero plaintext** — PASS today (TLS 1.3 enforced).
- [ ] **Unknown peer → TOFU prompt with randomart; second connection → no prompt** — **FAIL**. Closed by BOLT-001 (TOFU IPC round-trip) + BOLT-002 (server-side verifier + Ed25519 PoP).
- [ ] **Kill daemon mid-operation → no stale socket, clean PID** — **PARTIAL** (socket cleaned; PID never removed; subscriber-attached shutdown hangs forever). Closed by BOLT-004 (shutdown subscriber close) + BOLT-005 (symmetric pid write/remove).
- [ ] **Tests exist for `internal/config`** — **FAIL** (plan §"Test strategy" explicitly mandates this; current coverage is 0%). Closed by BOLT-011.

---

## Phase 2 entry checklist (Definition of Ready)

Savvy's go / no-go gate. Phase 2 does not start until every box is ticked.

- [ ] All Phase-1 verification bullets above pass on Linux **and** macOS (after BOLT-016 Windows decision).
- [ ] All P0 Ready-for-Dev stories (BOLT-001 through BOLT-008) are merged.
- [ ] All 10 decisions in the previous section are answered by Savvy and reflected in `plan.md` via the edits in §"Where plan.md needs editing".
- [ ] Loopback test harness `internal/testutil/loopback.go` (BOLT-010) lands and is used by at least one integration test in BOLT-001.
- [ ] `quic.Config` defaults locked with a golden-snapshot test (BOLT-007).
- [ ] Wire schemas for `FileHeader`, `ChunkHeader` (header-then-raw), `TransferAck`, `TransferDone`, `CancelMsg` frozen with round-trip tests (BOLT-008); `ChunkMsg.Data []byte` is removed.
- [ ] `peers.toml` and `config.toml` writes go through `writeAtomic` (BOLT-003); kill-during-write test asserts file is either old or new contents, never empty.
- [ ] Daemon clean shutdown completes within 5 s with subscribers attached (BOLT-004); `daemon.pid` is gone on clean shutdown (BOLT-005).
- [ ] `internal/transfer` package skeleton exists with godoc and `ErrNotImplemented` stubs (BOLT-013); Phase-2 IPC types (`CmdSendFile`, `EventTransferProgress`, …) defined but unwired.
- [ ] CI runs `build + vet + lint + test + test -race` on `ubuntu-latest` and `macos-latest` (BOLT-015).
- [ ] Disk-space helper `FreeBytes(path)` available for receiver to call (BOLT-014).
- [ ] Windows posture decided (BOLT-016) and reflected in README + `.goreleaser.yaml`.

---

## Risk register (top 5)

| # | Risk | Likelihood | Impact | Owner | Mitigation |
|---|---|---|---|---|---|
| 1 | Wire schema churn mid-Phase-2 because D1/D2/D10 are not locked early. | H | H | Networking | Freeze schemas in BOLT-008 **before** any sender/receiver code is written; round-trip golden tests fail on drift. |
| 2 | `peers.toml` corruption + silent TOFU re-acceptance creates a defense-in-depth collapse: lost trust state is silently rebuilt with the attacker's fingerprints. | M | H | Backend + Security | BOLT-001 (real TOFU prompt) + BOLT-003 (atomic writes) ship together; integration test in BOLT-001 simulates a `peers.toml` reset and asserts the user is re-prompted, not silently re-accepted. |
| 3 | `pion/ice` ↔ `quic-go` `Transport` integration is the riskiest Phase-5 plumbing and has no prototype (N-§7). | H | H | Networking | Out-of-band 3-5 day spike at Phase 5 start; budget Phase 5 with this risk priced in; treat the `net.PacketConn` adapter as its own backlog item before Phase 5 kicks off. |
| 4 | Daemon shutdown hang (BUG-2) breaks CI test stability and any future graceful-restart story; manifests only when a CLI subscriber is attached at SIGTERM. | H | M | Backend | BOLT-004 closes the hang; the integration test in BOLT-004 runs every CI build to catch regressions. |
| 5 | TOFU model is weaker than advertised (no Ed25519 proof-of-possession): a LAN attacker copying a peer's public key can fingerprint-match (N-§2). | M | H | Security + Networking | BOLT-002 lands before Phase 2; new test forges a cert with stolen pubkey + different priv → handshake fails. |

---

## Sources

- `.agent/reports/architecture-critique.md` — *"`architecture.md` is a clean aspirational picture; it does not reflect the code that exists today."* — 20 findings, 11 proposed ADRs, drift table between architecture, plan, and code.
- `.agent/reports/backend-review.md` — *"Phase 1 completeness, honest call: ~70%. … The TOFU UX — explicitly named in Phase 1's acceptance bullets — is not wired."* — 14 code findings P0–P2, Phase-2 prereq breakdown across A-H.
- `.agent/reports/networking-review.md` — *"Three top critiques: ChunkMsg base64 inflation, under-specified `quic.Config`, TOFU server-side verifier missing + no Ed25519 proof-of-possession."* — 10 decisions to lock, 11 Phase-2 networking prereqs, RFC citations throughout.
- `.agent/reports/qa-bug-hunt.md` — *"13 bugs, 4 Critical: TOFU stub, daemon shutdown hang on subscriber, non-atomic TOML writes, ChunkMsg base64 inflation will breach 8 MiB cap. 9 Important: PID file lifecycle, socket permissions, double-spawn race, fan-out lock contention, et al."* — coverage gap table by package.
- `.agent/reports/security-review.md` — *"15 reachable stdlib advisories (SEC-1 → BOLT-024), terminal escape injection (SEC-2 → BOLT-025), daemon env inheritance (SEC-3 → BOLT-026), symlink pre-creation (SEC-6 → BOLT-027)."*
- `.agent/reports/devops-runbook.md` — `gh auth` recovery + `file-issues.sh` usage.

---

## Security findings folded in

Security review (2026-05-19) added four net-new items beyond the QA/architect overlap. **SEC-1 (Critical)** — Go toolchain pinned at 1.25.1 with 15 reachable stdlib CVEs (`govulncheck`); closed by **BOLT-024** (bump to 1.25.10, no Savvy decision). **SEC-2 (High)** — peer-controlled nicknames and chat bodies reach the TTY without sanitization (ANSI/CSI/OSC injection, `\r` forgery); closed by **BOLT-025** (sanitize on receive + display). **SEC-3 (High)** — spawned daemon inherits the full parent shell environment (`AWS_*`, `GITHUB_TOKEN`, …); closed by **BOLT-026** (explicit env allowlist). **SEC-6 (Medium)** — symlink pre-creation on `~/.config/bolt/` can redirect first writes; closed by **BOLT-027** (0700 dir check, `O_NOFOLLOW|O_EXCL` on first writes; depends on BOLT-003+BOLT-005). SEC-4/5/7/8/10 are folded into existing BOLT stories (see traceability matrix). SEC-9 (TOCTOU on `os.Executable`) stays report-only until Phase 2.

---

## Backend pickup order (sequential)

Work top-to-bottom. Parallel pairs are noted; do not start Wave 3 until Savvy locks D1–D10 (or PO pre-approves recommended defaults in the decisions table above).

| Pickup # | ID | Title | Owner | Est | Depends on | GitHub |
|---:|---|---|---|---|---:|---|
| 1 | BOLT-024 | Bump Go toolchain to 1.25.10 (SEC-1) | Backend | 0.5d | — | #33 |
| 2 | BOLT-003 | Atomic-rename writer for peers.toml / config.toml | Backend | 1d | — | #9 |
| 3 | BOLT-004 | Daemon shutdown closes IPC subscribers (BUG-2) | Backend | 1d | — | #10 |
| 4 | BOLT-005 | daemon.pid symmetric write/remove (BUG-5) | Backend | 0.5d | parallel with #3 | #11 |
| 5 | BOLT-025 | Sanitize peer strings before TTY (SEC-2) | Backend | 1d | parallel with #3–4 | #34 |
| 6 | BOLT-026 | Daemon env allowlist (SEC-3) | Backend | 1d | parallel with #3–4 | #35 |
| 7 | BOLT-010 | Loopback test harness | Backend / QA | 2d | — | #16 |
| 8 | BOLT-011 | internal/config tests + kill-during-write | Backend | 1d | BOLT-003 | #17 |
| 9 | BOLT-012 | FuzzReadFrame + length-prefix bounds | QA | 1d | — | #18 |
| 10 | BOLT-007 | Lock quic.Config defaults | Networking | 1d | **D3** locked (or use pre-recommended defaults — pending Savvy sign-off) | #13 |
| 11 | BOLT-001 | TOFU prompt IPC round-trip | Backend | 2d | BOLT-003, BOLT-010 | #7 |
| 12 | BOLT-002 | Ed25519 PoP + server VerifyPeerCertificate | Networking | 1–2d | ship with #11 (TOFU bundle) | #8 |
| 13 | BOLT-008 | Freeze Phase-2 wire schema | Networking | 2d | **D1, D2** locked | #14 |
| 14 | BOLT-006 | Stream-handler registry | Backend | 1d | before transfer | #12 |
| 15 | BOLT-013 | transfer package skeleton + IPC types | Backend | 1d | BOLT-008, BOLT-006 | #19 |
| 16 | BOLT-014 | Disk-space helper | Backend | 0.5d | as capacity | #20 |
| 17 | BOLT-015 | CI matrix linux + macOS | DevOps | 1d | as capacity | #21 |
| 18 | BOLT-016 | Windows posture spike / ADR-002 | PO + Architect | spike | **D5** | #22 |
| 19 | BOLT-009 | IPC types → internal/proto + daemonclient | Backend | 2d | **D8**; can slip pre–Phase 2 heavy coding | #15 |

**Wave summary (no Savvy decisions):** #1 → #2 → (#3+#4+#5+#6 parallel) → #7 → #8+#9.  
**Wave 2 (test substrate):** #7–#9.  
**Wave 3 (TOFU bundle):** #11+#12 after #2+#7; do not merge without BOLT-003 landed.  
**Wave 4 (protocol freeze):** #13+#14+#15 only after D1/D2/D4 sign-off.

---

## Traceability matrix

| ID | Type | Priority | Source refs | GitHub | Blocks Phase 2? |
|---|---|---|---|---|:---:|
| BOLT-024 | Story | P0 | SEC-1 | #33 | Y (security) |
| BOLT-003 | Story | P0 | A-19, B-3, Q-BUG-3, SEC-4 | #9 | Y |
| BOLT-004 | Story | P0 | A-7, B-8, Q-BUG-2 | #10 | Y |
| BOLT-005 | Story | P0 | A-7, B-4, Q-BUG-5, Q-BUG-12 | #11 | Y |
| BOLT-025 | Story | P1 | SEC-2 | #34 | N (pre-Phase-2 hardening) |
| BOLT-026 | Story | P1 | SEC-3 | #35 | N |
| BOLT-010 | Story | P0 | B-§H, Q-coverage | #16 | Y |
| BOLT-011 | Story | P0 | B-§H-3, Q-coverage | #17 | Y |
| BOLT-012 | Story | P1 | N-§3, Q-coverage | #18 | N |
| BOLT-007 | Story | P0 | N-§1, SEC-5 | #13 | Y (needs D3) |
| BOLT-001 | Story | P0 | A-3, B-1, N-§2, Q-BUG-1, SEC-7 | #7 | Y |
| BOLT-002 | Story | P0 | A-3, N-§2, SEC-5 | #8 | Y |
| BOLT-008 | Story | P0 | B-9, N-§3, Q-BUG-4, SEC-8 | #14 | Y (needs D1/D2) |
| BOLT-006 | Story | P0 | A-1, B-2, Q-BUG-9 | #12 | Y |
| BOLT-013 | Story | P0 | B-§A, A-1 | #19 | Y (skeleton) |
| BOLT-014 | Story | P1 | B-§C-2 | #20 | N |
| BOLT-015 | Story | P1 | B-§I-1 | #21 | N |
| BOLT-016 | Story | P1 | A-5, D5 | #22 | Y (platform posture) |
| BOLT-009 | Story | P0 | A-2, D8 | #15 | Y (layering) |
| BOLT-027 | Story | P2 | SEC-6 | #36 | N |
| BUG-1 | Bug | P0 | Q-BUG-1 | #23 | Y (dup BOLT-001) |
| BUG-2 | Bug | P0 | Q-BUG-2 | #24 | Y (dup BOLT-004) |
| BUG-3 | Bug | P0 | Q-BUG-3 | #25 | Y (dup BOLT-003) |
| BUG-4 | Bug | P0 | Q-BUG-4 | #26 | Y (dup BOLT-008) |
| BUG-5 | Bug | P1 | Q-BUG-5 | #27 | N (dup BOLT-005) |
| BUG-6 | Bug | P1 | Q-BUG-6 | #28 | N |
| BUG-7 | Bug | P1 | Q-BUG-7 | #29 | N |
| BUG-8 | Bug | P1 | Q-BUG-8 | #30 | N |
| BUG-9 | Bug | P1 | Q-BUG-9 | #31 | N (dup BOLT-006) |
| BUG-10 | Bug | P1 | Q-BUG-10 | #32 | N (→ BOLT-021 backlog) |

---

## QA re-verification checklist (after each P0 merge)

Run after Backend merges the linked PR; comment `verified on <branch>` on the GitHub issue and close the duplicate BUG if applicable.

| After merge | Re-run | Pass criteria |
|---|---|---|
| BOLT-024 / #33 | `govulncheck ./...` | 0 reachable stdlib advisories |
| BOLT-003 / #9 | `go test -race ./internal/config/...` + kill-during-write test | file byte-identical or fully new after injected failure |
| BOLT-004 / #10 | Integration: daemon + `bolt chat` subscriber + `SIGTERM` | exit &lt; 5 s, socket removed |
| BOLT-005 / #11 | Integration: clean shutdown | `daemon.pid` absent |
| BOLT-001+#002 / #7+#8 | Loopback TOFU test (BOLT-010) | unknown peer prompts; stolen-key handshake fails 0x1001 |
| BOLT-008 / #14 | Round-trip 4 MiB chunk | wire size ≤ 4 MiB + 256 B (no base64) |

**Pickup verification order for QA:** BUG-3/#25 (BOLT-003) first, then BUG-2/#24 (BOLT-004), then BUG-1/#23 (BOLT-001).

---

## GitHub issue filing (Savvy / DevOps)

If `gh auth status` fails, run (from `.agent/reports/devops-runbook.md`):

```bash
gh auth login -h github.com --scopes "repo,read:org" --web
gh auth status
.agent/reports/file-issues.sh --dry-run   # inspect
.agent/reports/file-issues.sh             # create (idempotent)
```

As of 2026-05-19, all 30 bodies in `.agent/reports/issue-bodies/` are filed on `g-savitha/bolt` as issues **#7–#36**. No separate SEC-* issues — SEC-1/2/3/6 map to BOLT-024/025/026/027.
