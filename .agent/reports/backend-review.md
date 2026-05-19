# Backend Review — 2026-05-19

Reviewer: Senior Go Engineer (backend) for PO. Scope: Go code quality vs FAANG-bar and concrete Phase 2 readiness.
Tree at review time: `cmd/bolt`, `internal/{identity,config,proto,transport,daemon,chat}` populated; `internal/{discovery,transfer,tui}`, `relay/`, `deploy/`, `cmd/boltd/` empty/absent.

---

## Build / test / lint snapshot

| Check | Result |
|---|---|
| `go build ./...` | PASS (Go 1.25.1) |
| `go vet ./...` | PASS (clean) |
| `./golangci-lint run` (v2 config, cache=/tmp) | PASS — `0 issues` |
| `go test ./...` | PASS — 12 tests across 2 packages |
| `go test -race ./...` | PASS (no race-instrumented test packages today other than identity/proto) |
| Coverage | identity 48.5% · proto 71.0% · everyone else 0.0% |

Per-package coverage:

```
github.com/bolt/bolt/cmd/bolt           0.0%
github.com/bolt/bolt/internal/chat      0.0%
github.com/bolt/bolt/internal/config    0.0%
github.com/bolt/bolt/internal/daemon    0.0%
github.com/bolt/bolt/internal/identity  48.5%
github.com/bolt/bolt/internal/proto     71.0%
github.com/bolt/bolt/internal/transport 0.0%
```

Important context: the lint pass on its own looks great, but it covers a codebase whose hot paths (`transport`, `daemon`, `config`, `chat`) carry zero unit tests. Clean lint on untested code is a weak signal.

---

## Strengths (genuine, not gratuitous)

- Wire framing is correct, length-capped (`maxFrameSize = 8 MB` in `internal/proto/wire.go:222`) and round-tripped under test. The version byte (`"v": 1`) is enforced on receive (`peer_conn.go:112`, `service.go:97`).
- Identity layer is clean: deterministic fingerprint, private key written `0600` (verified by `identity_test.go:108`), pubkey-vs-privkey sanity check on load (`identity.go:92`), `ExtractPublicKeyFromCert` is a one-place ground-truth helper used on both sides of the handshake.
- Concurrency model is sober where it exists: `PeerStore` and `PeerRegistry` are both `sync.RWMutex`-protected; nicknames are kept as display-only and fingerprints are the primary key everywhere.
- TLS layer cleanly separates `RequireAnyClientCert` server config from `InsecureSkipVerify + VerifyPeerCertificate` client config (`internal/transport/tls.go:35,57`); ALPN is set; `MinVersion = TLS 1.3` is enforced.
- Tooling baseline is FAANG-ish already: pinned-SHA GitHub Actions, govulncheck + trivy + CodeQL + scorecard + dependabot, SonarCloud wired, `golangci-lint v2` with gosec/staticcheck/govet-all/errcheck/ineffassign/unused. `.goreleaser.yaml` produces SBOMs and SHA256SUMS.

---

## Phase 1 completeness

| # | Plan item | Status | Evidence | Notes |
|---|---|---|---|---|
| 1 | Scaffold: go.mod, cobra root + stubs, Makefile | **Done** | `cmd/bolt/main.go:22`, `go.mod`, `Makefile`, `.golangci.yml`, `.goreleaser.yaml` | Subcommands `init`, `id`, `daemon`, `status`, `connect`, `chat`, `version` wired. No `peers`, `send`, `trust`, `receive-dir` stubs yet. |
| 2 | identity: keygen, fingerprint, TLS cert, randomart, tests | **Done** | `internal/identity/identity.go`, `randomart.go`, `identity_test.go` | TLS cert wraps Ed25519 in ECDSA P-256 for the handshake and stuffs the Ed25519 pubkey into `SubjectKeyId` (`identity.go:154`). Coverage 48.5% (Randomart and ExtractPublicKeyFromCert not directly tested). |
| 3 | config: versioned load/save + migration; PeerStore | **Partial** | `internal/config/config.go`, `peers.go` | Migration logic exists (`config.go:133`), `PeerStore` is RWMutex-protected (`peers.go:71`). **Zero tests** — plan explicitly requires them. `peers.toml` write is a clobber-rewrite under write lock (no atomic rename), see B-3. |
| 4 | transport/tls: TOFU verifier wiring | **Partial** | `internal/transport/tls.go`, `connect.go:75` | The plumbing is there. The TOFU **policy** is a stub: `peerVerifier` returns `true` for any non-blocked peer (`connect.go:76`) and `authenticateIncoming` does the same on inbound (`daemon.go:166`). No prompt, no IPC event, no `allow-once` reprompt. See P0/B-1. |
| 5 | transport/quic: Listen/Dial wrappers, keep-alive, max streams | **Done** | `internal/transport/quic.go:30` | `KeepAlivePeriod=15s`, `MaxIncomingStreams=1000`, `MaxIncomingUniStreams=1000`, `HandshakeIdleTimeout=10s`. |
| 6 | TOFU handshake flow (prompt, accept/reject, save to PeerStore, emit `peer_online`) | **Missing (functional)** | `daemon.go:154`, `connect.go:76` | TLS completes, fingerprint is extracted, app-handshake runs, peer is saved with `TrustAllowOnce`. But no `tofu_prompt` IPC event is published, no `peer_online` IPC event is published, no human-in-loop. See P0/B-1. |
| 7 | Signal handling: SIGTERM/SIGINT/SIGHUP → checkpoint → cleanup socket+PID → exit | **Partial** | `daemon.go:54`, `ipc_endpoint_unix.go:42` | `signal.NotifyContext` traps all three. `daemon.sock` is removed by deferred `cleanupIPC`. **`daemon.pid` is never removed**, see P1/B-4. No transfer checkpoint hook exists yet (no transfers — fine for Phase 1, but the seam is missing). |
| 8 | `bolt init`: keygen + versioned config + flock-guarded spawn | **Done** | `cmd/bolt/main.go:68`, `internal/daemon/spawn.go:24` | `EnsureRunning` does sock-probe → flock pid → re-probe → spawn → poll. `flock` lifetime is tied to `withPIDLock` callback. |
| 9 | `bolt id`: load identity, print fingerprint + randomart | **Done** | `cmd/bolt/main.go:94`, `identity/randomart.go:107` | |

**Phase 1 completeness, honest call: ~70%.** The scaffolding is solid; the daemon plumbing is solid; the TOFU UX — which is one of the five named Phase 1 acceptance bullets — is not wired. Tests on `config` are missing.

---

## Phase 1 verification readiness

| Verification | Passes today? | Why |
|---|---|---|
| `bolt init` → identity files exist with correct perms | **Yes** | Verified by `identity_test.go:108`; `Generate` writes `0600`, `MkdirAll` uses `0700` (`identity.go:52,206`). |
| `bolt id` → same fingerprint on repeated calls | **Yes** | `TestFingerprintIsStable` (`identity_test.go:53`). |
| tcpdump on UDP 7799 → zero plaintext | **Yes** | `MinVersion=TLS13` on both sides (`tls.go:41,65`); QUIC encrypts the entire datagram payload by spec. |
| Unknown peer → TOFU prompt with randomart; second connection → no prompt | **No** | No prompt path. `peerVerifier` and `authenticateIncoming` auto-accept any non-blocked peer. The randomart is only printed via `bolt id`. **This is the Phase-1 acceptance gap.** |
| Kill daemon mid-operation → no stale socket, clean PID | **Partial** | Socket: yes (cleanupIPC defers on `daemon.go:73`). PID: no — `daemon.pid` is left on disk after shutdown and only overwritten on next spawn. A reader treating the PID file as liveness would lie. See B-4. |

---

## Code quality findings (prioritized)

### P0

#### [B-1] TOFU prompt path does not exist — Phase 1 acceptance gap and Phase 2 blocker
- File: `internal/daemon/connect.go:75-83`, `internal/daemon/daemon.go:154-176`
- Problem: `peerVerifier` returns `(true, nil)` for any fingerprint not on the block list. `authenticateIncoming` mirrors the same policy on inbound. There is no `tofu_prompt` IPC event, no waiting on a CLI response, no enforcement of `TrustAllowOnce` semantics ("prompt each time"). Result: a stranger's first connection is silently accepted, the user never sees the fingerprint/randomart, and `TrustAllowOnce` collapses to `TrustAlwaysAllow` in practice.
- Recommendation: define `IPCEvent{Type:"tofu_prompt", Payload:{fingerprint,nickname,randomart,addr,nonce}}` and `IPCRequest{Command:"tofu_response", Payload:{nonce,decision:always|once|block}}`. The verifier blocks on a channel keyed by `nonce` with a 30s timeout; on timeout return `(false, "user did not respond")`. The same path is used for `allow-once` on each subsequent connection. Until this lands, anything that depends on it (file transfer accept prompt in Phase 2) is blocked.
- Effort: M (1–2 days).

#### [B-2] Stream-router silently drops every non-chat stream type
- File: `internal/daemon/daemon.go:203-213`
- Problem: `routeStream` only handles `StreamChat`. `StreamFile`, `StreamControl`, and any unknown type fall into `default:` which closes the stream and prints a stderr line. There is no dispatch registry, no map from `proto.StreamType` to handler. Phase 2 needs to drop file/control handlers in here without forcing every contributor to edit the switch and recompile the whole daemon.
- Recommendation: introduce `type StreamHandler func(ctx, *transport.PeerConn, *quic.Stream)` and `(d *Daemon) RegisterStreamHandler(t proto.StreamType, h StreamHandler)`; default is "close + log". `chat.Service` gets registered at boot; `transfer.Service` plugs in for Phase 2 without touching `daemon.go`.
- Effort: S.

#### [B-3] `PeerStore.flushLocked` is a non-atomic clobber-rewrite
- File: `internal/config/peers.go:193-217`, mirrored in `config.go:178-193`
- Problem: `os.OpenFile(O_WRONLY|O_CREATE|O_TRUNC)` plus `toml.NewEncoder(f).Encode(pf)` truncates the file at the start and writes back. A crash, kill -9, or full disk between truncate and encode-complete leaves the user with a half-written or empty `peers.toml`. Trust state is the most security-relevant on-disk state in the project — losing it forces every peer back to TOFU.
- Recommendation: write to `peers.toml.tmp` (in the same directory), `Sync()`, then `os.Rename` over the destination. Same for `config.toml`. On macOS this gives crash-atomic semantics; on Linux+ext4 default `data=ordered` it's also safe enough. Add a unit test that crash-simulates by killing the encoder mid-write (use a small bytes-limited writer wrapper).
- Effort: S.

### P1

#### [B-4] `daemon.pid` is never removed on clean shutdown
- File: `internal/daemon/daemon.go:70-73`, `internal/daemon/spawn.go:53-57`
- Problem: Plan §"Signal handling in daemon" calls for "remove `daemon.sock`, update `daemon.pid`" on signal. `cleanupIPC` only removes the socket. The PID file remains and is overwritten on the next `EnsureRunning` race-flock cycle. Anyone (or any operator script) reading `daemon.pid` to decide liveness will get a false-positive after a clean shutdown.
- Recommendation: in `shutdown()` (or the deferred block in `Run`), `os.Remove(pidFilePath(d.configDir))` after the lock is released. Also: write the PID only after the spawned child reports liveness (currently spawn.go writes PID before polling — race).
- Effort: S.

#### [B-5] `Daemon.runCtx` is a stored package-private context; nil-deref risk pre-`Run`
- File: `internal/daemon/daemon.go:29`, `ipc.go:241,252`
- Problem: `d.runCtx` is set inside `Run()`. `IPCServer.handleConnect/handleSendChat` dereference it directly. Today the IPC server is only constructed inside `Run()` so the order is fine, but the contract is implicit. Storing a context in a struct is also a known Go-vet smell that this repo's vet config explicitly tolerates by leaving `contextcheck`/`containedctx` off.
- Recommendation: pass `ctx` as the first argument to every IPC dispatch path (each IPC connection already owns a goroutine; derive a per-request `ctx` from `d.runCtx` and pass it down). Eventually delete the struct field.
- Effort: S.

#### [B-6] IPC connections are single-shot for every command except `subscribe`
- File: `internal/daemon/ipc.go:146-187`, `cmd/bolt/main.go:239-256`
- Problem: `handleConn` reads one frame, dispatches, writes a response, returns. Every chat message in `chatCmd` opens a fresh Unix socket connection for one `send_chat`. For Phase 2 `send_file` this is fine, but for `chat` interactivity at 50–100 msgs/sec it allocates a fd-per-message.
- Recommendation: read-loop instead of read-once; client closes the connection or sends `bye`. Less urgent for Phase 2, but worth fixing alongside the IPC schema growth.
- Effort: S.

#### [B-7] `chat.Service.AttachStream` ignores its `ctx` parameter
- File: `internal/chat/service.go:73-111`
- Problem: The reader loop reads frames until `ReadFrame` returns an error. The `ctx` argument is never consulted — `ctx.Done()` cannot stop the read. In practice, daemon shutdown closes the QUIC connection (`shutdown()` calls `pc.Close()`), which fails the read, so the goroutine terminates. But it's a fragile invariant — if a future code path stops closing the conn first, the goroutine leaks.
- Recommendation: drop a `<-ctx.Done(); stream.CancelRead(0)` watchdog goroutine, or accept the leak explicitly with a comment. Either is fine — being silent about it is not.
- Effort: S.

#### [B-8] `acceptLoop` and `routeStream` goroutines have no wait-group
- File: `internal/daemon/daemon.go:123-201,215-219`
- Problem: `shutdown()` closes the registered peer connections but does not wait for `acceptLoop` (returns when listener errors), `servePeer` (returns when AcceptStream fails), or `routeStream` goroutines to finish. Today everything is best-effort and works because closing the QUIC conn cancels the cascade. In Phase 2 receivers will hold open files; we need a clean drain.
- Recommendation: a `sync.WaitGroup` registered on every spawned goroutine, plus a bounded `shutdownTimeout` (e.g. 5s) on `wg.Wait` with a context.
- Effort: S.

### P2

#### [B-9] Wire chunk payloads are JSON-encoded `[]byte`
- File: `internal/proto/wire.go:115-122`
- Problem: `ChunkMsg.Data []byte` will be base64-encoded by `encoding/json`. A 4 MB chunk becomes ~5.3 MB JSON. With 8 parallel streams that's 8×5.3 = 42 MB/s of allocator churn at 1 Gbit. The 8 MB `maxFrameSize` is already tight against a 4 MB chunk after base64.
- Recommendation: keep `ChunkMsg{Index,Hash,V,TransferID}` as a small JSON header, then write raw chunk bytes after the framed header on the same stream (`[header-len][header-json][chunk-len][chunk-bytes]`). Or open a dedicated raw-bytes substream type `StreamFileChunk` (0x05). Either way, decouples binary data from JSON encoding. Plan §"Chunk size" already mentions 4 MB / 256 KB — neither plays well with base64.
- Effort: M.

#### [B-10] `identity.TLSCertificate()` regenerates a fresh ECDSA wrapper key on every call
- File: `internal/identity/identity.go:129-179`
- Problem: Each call produces a new ECDSA P-256 key and a new cert (different signature, same `SubjectKeyId`). The cert is therefore non-deterministic per-process. `daemon.listenForPeers` builds a server TLS config once at boot, so this is fine in production. But every test that calls `TLSCertificate()` twice cannot compare certs by raw DER. It's also a wasted P-256 keygen on every daemon restart.
- Recommendation: cache the cert on the `Identity` struct (lazy init under a `sync.Once`). Optionally seed the ECDSA key deterministically from the Ed25519 private key — but that's a footgun; lazy cache is enough.
- Effort: S.

#### [B-11] `okResponse` silently drops `json.Marshal` errors
- File: `internal/daemon/ipc.go:375-378`
- Problem: `data, _ := json.Marshal(payload)` swallows. If a payload type is ever extended with an `unsupported` field (e.g. `chan`, `func`), the IPC client will get `{"ok":true,"payload":null}` instead of an error.
- Recommendation: return `IPCResponse, error` from `okResponse`; let the caller convert to `errorResponse`.
- Effort: S.

#### [B-12] `cmd/bolt/main.go` does not honor `--config-dir` consistently
- File: `cmd/bolt/main.go:40-53,73,99`
- Problem: `var configDir` is captured by `PersistentFlags().StringVar`. The captured `*configDir` is passed by pointer into every command. Because `Use:"bolt"` and subcommands run only after Cobra parses flags, the pointer dereference is fine — but two of the subcommands (`statusCmd`, `connectCmd`, `chatCmd`) capture `dir := *configDir` *inside* `RunE`, while `initCmd` and `idCmd` do the same. It works, but it's twelve copies of the same boilerplate. Minor.
- Recommendation: a single `withConfigDir(fn func(string) error) cobra.RunEFunc` helper, or pass the cobra command into a `runtimeContext` constructor.
- Effort: S.

#### [B-13] Magic numbers / strings without typed constants
- `1<<20` in `ipc.go:365` for max IPC frame size (1 MB) — not a named constant.
- `0` as the QUIC close error code in `peer_conn.go:187`, `daemon.go:142, 146`, `connect.go:34,40,48` — not a named "normal closure" constant.
- Spawn timing in `spawn.go:16-17` already typed — good baseline; mirror that elsewhere.
- Effort: S.

#### [B-14] `peer_conn.go::Handshake` writes its outbound stream while a goroutine accepts the inbound one — handshake error path leaks the goroutine on outbound failure
- File: `internal/transport/peer_conn.go:39-87`
- Problem: If `pc.sendHandshake` returns an error after the accept-goroutine has been launched, `Handshake` returns immediately. The goroutine is still parked on `AcceptStream(ctx)`. If `ctx` is cancellable it will unwind; if it's not (e.g. `context.Background`), the goroutine waits until the QUIC conn is closed by the caller.
- Recommendation: derive a per-call context from `ctx`, cancel it in a `defer` so the goroutine returns deterministically.
- Effort: S.

---

## Phase 2 prerequisites (the headline list, sized for backlog)

Grouped by Phase 2 sub-step. Each item is one backlog story.

### A. Chunking (`internal/transfer/chunk.go`)

- **A-1 Create `internal/transfer` package skeleton.** Today the directory is empty. Add `chunk.go`, `sender.go`, `receiver.go`, `resume.go`, `progress.go`, `service.go`, plus matching `*_test.go`. (S)
- **A-2 `ChunkFile(r io.Reader, chunkSize int) (<-chan Chunk, <-chan error)` with `Chunk{Index int; Data []byte; Hash [32]byte}`.** Boundary cases: zero-byte file (one chunk with `Data:nil`, `Hash` of empty), file of exactly `N*chunkSize` bytes (no short tail), stdin (`io.Reader` size unknown). (S)
- **A-3 Whole-file hashing.** Streaming `sha256.New()` updated as chunks are produced; final digest stored on a `FileHasher` value, used to populate `FileHeader.FileHash`. (S)
- **A-4 Path-aware chunk size selection.** Add `PathType` to `transport.PeerConn` (enum `PathLAN`, `PathRelay`, default `PathLAN`). `transfer.ChunkSizeFor(pc) int` returns 4 MiB on LAN, 256 KiB on relay. The selection must be reflected in `FileHeader.ChunkSize`. (S)
- **A-5 Table-driven test for chunk boundaries.** Random sizes from 0 B to 16 MiB at three different `chunkSize` values, verifying re-assembly hash matches `sha256.Sum256(input)`. (S)

### B. Sender (`internal/transfer/sender.go`)

- **B-1 `PeerConn.OpenStream` already exists** (`peer_conn.go:155`) — good, no change needed. Confirm it returns a `*quic.Stream` not a `*quic.SendStream` so we get bidirectional flow for the control stream.
- **B-2 `Sender.Send(ctx, pc, src io.Reader, name string, size int64) (TransferResult, error)`.** Opens control stream, writes `FileHeader`, reads `TransferAck`, fans out `min(8, totalChunks)` worker goroutines on dedicated chunk streams pulling from a shared work queue. (M)
- **B-3 Per-chunk retry on transient stream error.** Chunk-level error → re-enqueue the chunk index, do not abort the whole transfer. Bound total retries per chunk (e.g. 3) before failing. (S)
- **B-4 `CancelMsg` wiring on the control stream.** If the receiver sends `CancelMsg`, all worker goroutines must return promptly. Use a `context.WithCancelCause(ctx)` so the error surface explains why. (S)

### C. Receiver (`internal/transfer/receiver.go`)

- **C-1 `Receiver.Accept(ctx, pc, header FileHeader) (TransferAck, *Sink, error)`.** Trust check (`peers.Get(pc.Fingerprint).Trust`) → `block` rejects, `always-allow` accepts, `allow-once` triggers TOFU-style prompt (depends on **B-1 / P0**). (M)
- **C-2 Disk space syscall wrapper.** New `internal/transfer/diskspace_unix.go` (build tag `!windows`) using `syscall.Statfs` to return `FreeBytes(path string) (int64, error)`. Sender's `FileHeader.SizeBytes` is checked before sending `TransferAck{Accepted:true}`; insufficient disk → `TransferAck{Accepted:false, Reason:"insufficient_disk: need X, have Y on dir"}`. (S)
- **C-3 Receive directory resolution.** `config.Config.ReceiveDir` defaults to `~/Downloads`. There is **no tilde expansion today**; resolve at config-load time using `os.UserHomeDir()`. Also implement `bolt receive-dir <path>` IPC command + `Daemon.SetReceiveDir(path)` for session override. (S)
- **C-4 Sink: `ftruncate` + `WriteAt`.** Open `~/.local/share/bolt/incomplete/<uuid>.tmp` (`0600`), `Truncate(header.SizeBytes)`, then concurrent `WriteAt(chunk.Data, int64(chunk.Index)*int64(chunk.ChunkSize))` from receiver worker goroutines. Must be safe on macOS APFS (concurrent WriteAt is documented safe in `os.File`). (M)
- **C-5 Resume state checkpoint.** Every 10 chunks, serialise `ResumeState{TransferID, Filename, SizeBytes, ChunkSize, TotalChunks, ReceivedIndices []int, FileHash}` to `incomplete/<uuid>.json` via tmp+rename. (S)
- **C-6 Whole-file verify on `TransferDone`.** Re-hash the `.tmp` file with `sha256` and compare to `header.FileHash`. Mismatch → discard, surface `TransferComplete{ok:false}`. Match → rename to `receive_dir/<filename>` with collision suffixing `_1`, `_2`. (S)
- **C-7 Resume scan on daemon startup.** Background goroutine in `Daemon.Run` scans `incomplete/*.json` and, when a peer reconnects with a matching `TransferID`, sends `TransferAck{ResumeFromChunks: missing}`. Owning code lives in `transfer.Service`. (M)

### D. IPC schema growth (`internal/daemon/ipc.go`, new `internal/proto/ipc.go`)

- **D-1 New IPC commands.** `CmdSendFile`, `CmdReceiveDir`, `CmdTrust`, `CmdTofuResponse`, `CmdListPeers` (a real list, not status). Each with a typed payload struct. (S)
- **D-2 New IPC events.** `transfer_progress`, `transfer_complete`, `transfer_skipped`, `tofu_prompt`, `peer_online`, `peer_offline`. `IPCServer.Publish<T>` symmetric to `PublishChat`. (S)
- **D-3 TofuPrompt round-trip.** Daemon-side: a `pendingPrompts map[nonce]chan TofuDecision` protected by a mutex; verifier blocks on the channel up to 30s. CLI-side: `bolt connect <ip>` subscribes, renders the randomart, sends `tofu_response`. This is the same primitive used by **C-1** for per-receive trust gating. (M)

### E. Daemon plumbing

- **E-1 Stream-handler registry** (see B-2 above). Required so `transfer.Service` plugs into `routeStream` without editing daemon.go. (S)
- **E-2 Wait-group + context cleanup** (B-8). Required so a `bolt daemon` `SIGTERM` during an in-flight transfer flushes resume state. (S)
- **E-3 Remove `daemon.pid` on clean shutdown** (B-4). Removes a stale-liveness footgun before transfers start touching `incomplete/*.json`. (S)

### F. CLI

- **F-1 `bolt send <file> <peer> [peers...]`.** Multi-peer fan-out per plan §"`bolt send` partial failure behaviour": parallel sends, per-peer summary, exit code reflects overall success. (M)
- **F-2 `bolt send - <peer>` stdin path.** Buffer stdin to `incomplete/pipe-<uuid>.tmp` first (plan §"Stdin"), then sender treats it as a regular file. (S)
- **F-3 Progress rendering.** `bubbles/progress` against the `transfer_progress` event stream. Plan §"Phase 2/4 progress.go"; new dep — add `github.com/charmbracelet/bubbles` to `go.mod`. (S)
- **F-4 `bolt trust <peer> always|once|block`.** CmdTrust round-trip; `daemon.peers.SetTrust`. (S)
- **F-5 `bolt receive-dir <path>`.** Session-only override; persisted via CmdReceiveDir. (S)

### G. Wire protocol gaps

- **G-1 Binary chunk framing.** See [B-9]. Either swap `ChunkMsg.Data` for raw bytes after a small header (length-prefixed binary), or introduce `StreamFileChunk = 0x05` with binary framing only. (M)
- **G-2 `WireVersion` rejection plumbing.** `ReadFrame` already unmarshals to a typed `V`. The chat path checks `msg.V != WireVersion` and skips (`service.go:97`). For file transfer, version-skew must be a hard reject with a `CancelMsg{Reason:"version mismatch"}` — define the policy in a `VersionCheck(msg.V) error` helper. (S)

### H. Test scaffolding (cannot be Phase 2 without this)

- **H-1 `internal/testutil/loopback.go`.** Two `*transport.PeerConn` instances over a real in-process QUIC pair, returned by a single `NewLoopbackPair(t *testing.T) (a, b *PeerConn)` helper. Used by chat tests, transfer tests, receiver tests. (M)
- **H-2 `internal/proto/wire_fuzz_test.go`.** `func FuzzReadFrame(f *testing.F)`. Seed with empty, oversize-length, zero-length, valid-length+truncated payload, valid-length+garbage-payload. Must terminate within Go's default fuzz budget. (S)
- **H-3 `internal/config/{config,peers}_test.go`.** Plan-mandated. Concurrent `PeerStore.Upsert` calls (race-detector validates), missing-version migration, version-too-new error path, partial-write rollback (the new atomic-rename in B-3). (S)
- **H-4 Integration test for chunk corruption + retry.** Use **H-1** to wire a sender and receiver, inject a flip in one chunk's `Data`, assert the chunk-hash mismatch triggers a retry, transfer completes, whole-file hash matches. (M)
- **H-5 Integration test for mid-transfer kill + resume.** Cancel context after ~3 chunks, restart the receiver, assert it sends `TransferAck{ResumeFromChunks:...}` and the eventual file hash matches. (M)

### I. Build / CI

- **I-1 Coverage gate.** `go test -coverprofile` already produced by Makefile; CI uploads coverage but doesn't enforce a floor. Add a `make ci-coverage-gate` target asserting ≥60% per non-cmd package before Phase 2 merges. (S)
- **I-2 goreleaser audit.** `.goreleaser.yaml` builds Windows binaries; plan §"Platform support" says **Windows unsupported in v1**. Either delete the `windows` entry or document the exclusion as a deliberate "stub only" build. (S)
- **I-3 `make release-local` matches plan.** Currently builds windows/amd64 too — same comment as I-2. (S)

---

## Top engineering risks for Phase 2

1. **Concurrent `WriteAt` + `Truncate` on macOS APFS vs Linux ext4.** Go's `*os.File.WriteAt` is documented safe to call concurrently as long as the writes don't overlap. But: `Truncate(N)` followed by concurrent `WriteAt` at arbitrary offsets *does* race on file metadata (size, allocation) in some Linux configurations and on APFS with sparse files. *Mitigation:* truncate first, fsync once before workers start; on shutdown, fsync each completed chunk's range or fsync the whole file at `TransferDone`. Add a CI matrix job on `macos-latest` and `ubuntu-latest` running the receiver integration test under `-race`.

2. **Chunk goroutine cancellation racing the chunker.** The sender will spawn 8 goroutines reading from a shared `<-chan Chunk`. If the receiver cancels mid-transfer, the chunker goroutine producing into the channel must observe cancellation or it blocks indefinitely on the send. *Mitigation:* `select { case ch <- chunk: ; case <-ctx.Done(): close(ch); return }` inside the chunker, and the workers `select` on receive vs ctx-done. Single source of truth for cancellation: `context.CancelCauseFunc` plumbed from `Sender.Send`.

3. **JSON-encoding multi-MB chunk bodies.** Base64 inflation (4 MB → 5.3 MB), plus a fresh `[]byte` allocation per chunk per goroutine, will hammer the GC during a 1 GiB transfer (~250 chunks × 8 parallel = 2000 allocations and ~10 GiB of bytes). *Mitigation:* B-9 / G-1 — binary framing, or at minimum pre-allocate a `sync.Pool` of 4 MiB scratch buffers.

4. **Stream backpressure starves the control stream.** With 8 chunk streams sharing the QUIC connection's flow control window, a slow receiver can stall the entire connection's congestion budget. The control stream (`StreamFile` control + `TransferAck`/`CancelMsg`) must not be one of those 8. *Mitigation:* a dedicated `StreamControl` (0x04) stream per peer, opened at handshake time and never closed; cancellation messages flow there, not on the file streams. quic-go's `MaxIncomingStreams=1000` is plenty of budget.

5. **TOFU prompt blocks the QUIC handshake.** `VerifyPeerCertificate` runs synchronously inside the TLS handshake. Blocking it for 30 seconds while a user reads randomart and presses `y` is technically legal but will trigger `HandshakeIdleTimeout` (currently 10s in `quic.go:25`) and abort the connection. *Mitigation:* either raise the handshake timeout to 60s, or move the prompt to *after* the application handshake — accept TLS-level, then in the bolt-level handshake refuse to proceed and emit `tofu_prompt` to the CLI, blocking on user response before allowing any other streams. The latter is cleaner; the former is one line.

6. **Resume scan on startup blocks daemon boot.** If `~/.local/share/bolt/incomplete/` contains many large `*.json` files, deserializing them and re-hashing partial `.tmp` files at startup could take seconds. *Mitigation:* scan in a background goroutine started from `Daemon.Run` after the IPC server is up. Reconnection-driven resumes don't need the scan to be complete; they need the resume state for *their* `TransferID` to be loadable. Open the file lazily when the peer reconnects with that ID.

7. **`peers.toml` write contention vs `transfer_progress` events.** Phase 2 will save transfer history entries via `PeerStore` updates. If a transfer emits a progress event every 100 ms and each progress update writes back to disk, the global write-lock + TOML re-encode + clobber-write (B-3) will dominate I/O. *Mitigation:* progress is in-memory only; persistent state (transfer log) goes to `~/.local/share/bolt/logs/transfers-YYYY-MM-DD.log` per plan, never to `peers.toml`.

---

## Recommended new tooling / scaffolding before Phase 2

| Item | Why | Effort |
|---|---|---|
| `internal/testutil/loopback.go` exposing `NewLoopbackPair(t) (*PeerConn, *PeerConn)` | Unlocks integration tests for chat, transfer, TOFU prompt; **prerequisite for H-1 / H-4 / H-5** | M |
| `internal/proto/wire_fuzz_test.go` | Length-prefix framing is a classic crash surface; CI fuzz target for `ReadFrame` | S |
| `internal/config/{config,peers}_test.go` | Plan mandates; covers migration, concurrent Upsert under `-race`, partial-write rollback | S |
| `internal/transfer/diskspace_unix.go` (build-tagged) | Receiver disk-space guard. Wraps `syscall.Statfs` to one `FreeBytes(path) int64` | S |
| Stream-handler registry on `*Daemon` | Lets `transfer.Service` plug into `routeStream` without editing `daemon.go` | S |
| `internal/transfer/service.go` skeleton (interfaces only) | Lets the architect critique nail down sender/receiver boundaries before B-2 / C-4 land | S |
| Atomic-rename writer helper in `internal/config` (used by both config.toml and peers.toml) | Closes B-3 | S |
| CI coverage floor (`make ci-coverage-gate`) | Enforces the test backlog instead of relying on willpower | S |
| `Makefile` `release-local` and `.goreleaser.yaml`: drop or comment Windows | Aligns with plan §"Windows unsupported in v1" | S |

---

## Closing notes

The skeleton is genuinely good for week 1–2 output. The code is small, the names are honest, the linter is clean, the framing layer is sane. Where it sits below FAANG-bar today:

- **Test coverage is concentrated in two unit-friendly packages and absent from the four packages that actually do work** (`transport`, `daemon`, `chat`, `config`). This is the single most consequential gap before Phase 2.
- **The TOFU UX — explicitly named in Phase 1's acceptance bullets — is not wired.** Until B-1 lands, "bolt has TOFU" is aspirational.
- **A handful of small concurrency footguns** (B-7, B-8, B-14) that don't bite today but will once Phase 2 adds long-running file goroutines.

Once B-1 (TOFU), B-2 (stream registry), B-3 (atomic peers.toml), H-1 (loopback testutil), and H-3 (config tests) are merged, Phase 2 can start without inheriting any Phase-1 debt.
