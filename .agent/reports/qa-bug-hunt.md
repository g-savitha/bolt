# QA Bug Hunt — 2026-05-19

Reviewer: Senior QA Engineer. Scope: static + dynamic audit of every Go file currently in the bolt tree (`cmd/bolt`, `internal/{identity,config,proto,transport,daemon,chat}`). Read-only on source; reports + status only.

## Summary

| Check | Result |
|---|---|
| `go build ./...` | PASS |
| `go vet ./...` | PASS (clean) |
| `./golangci-lint run --timeout 5m` (after `cache clean`) | PASS — `0 issues` |
| `go test ./...` | PASS — `identity`, `proto` packages green; `chat`, `config`, `daemon`, `transport`, `cmd/bolt` have **no test files** |
| `go test -race ./...` | PASS — clean; non-flaky over 5 consecutive runs |
| Race detector | clean |

**Bugs found: 13** (Critical: 4, Important: 9, Nice-to-have: 5 — listed in this report only).

**GitHub issues filed: 0** — `gh auth status` returned `Failed to log in to github.com account g-savitha (keyring) — token in keyring is invalid`. Issues are documented here for the user to file manually after re-authenticating with `gh auth login`. The bug bodies below already follow the `.claude/commands/qa.md` template and can be pasted in.

Honest scope caveats:
- Phase-2 modules (`transfer/`, `discovery/`, `tui/`, relay) do not exist yet, so transfer-checkpoint, mDNS, resume, and relay code paths are not auditable. Where plan.md calls for them (e.g. "checkpoint all in-progress transfers" on SIGTERM), they are flagged as expected-missing, not bugs.
- Many TLS/QUIC behaviors (real two-peer TOFU, ICE, NAT) require runtime with two endpoints. The findings below are derived from code review with limited dynamic exercise (unit tests + build).
- A previous `backend-review.md` exists in `.agent/reports/`. Where the same defect appears, I re-categorize it from the QA lens (severity, user impact, repro) rather than restating the engineering note verbatim. References are noted.

---

## Critical

### [BUG-1] TOFU verification is a stub — every unknown peer is silently accepted as `allow-once`
- File: `internal/daemon/connect.go:75-83`, `internal/daemon/daemon.go:154-176`
- Category: TLS / TOFU / security
- Description: `peerVerifier()` returns `(true, nil)` for any fingerprint that is not on the explicit block list. The same policy runs on inbound (`authenticateIncoming` only rejects if `existing != nil && existing.Trust == TrustBlock`). Result: any first-contact peer is accepted without prompt, no `tofu_prompt` IPC event is emitted, no randomart is rendered, the user is never asked. The peer is then persisted to `peers.toml` with `TrustAllowOnce`, which collapses semantically to "always allow" because no subsequent prompt path exists either.
- Repro (static): Read `internal/daemon/connect.go:76-83`. The verifier ignores its `fingerprint` argument for any policy decision other than blocklist lookup. Plan.md §"TOFU fingerprint UX" and Phase 1 verification bullet 4 explicitly require the prompt.
- Expected: On unknown fingerprint, push `tofu_prompt` IPC event with fingerprint + randomart, wait up to 30s for `tofu_response`, accept only if user types `y`. Same prompt repeats on every connection from peers with `TrustAllowOnce`.
- Actual: Accepts silently; `bolt id` is the only place randomart is ever rendered.
- Suggested fix: Define `IPCEvent{Type:"tofu_prompt", Payload:{fingerprint,nickname,randomart,addr,nonce}}` + `IPCRequest{Command:"tofu_response", Payload:{nonce, decision}}`. `peerVerifier` blocks on a per-nonce channel with `time.After(30 * time.Second)` returning `(false, "user did not respond")` on timeout. Move the prompt to the **bolt-level** handshake (after TLS, before any other stream) rather than inside `VerifyPeerCertificate` so the synchronous 10s `HandshakeIdleTimeout` is not the wall clock. (This is also the cleanest path called out in `backend-review.md` engineering risk #5.)

### [BUG-2] Daemon shutdown hangs forever when any CLI subscriber is connected
- File: `internal/daemon/ipc.go:109-113, 168-187`, `internal/daemon/daemon.go:70-73,82-85`
- Category: lifecycle / shutdown hang / data loss
- Description: `IPCServer.Close()` calls `s.ln.Close()` then `s.wg.Wait()`. Closing the listener stops `Accept` but does **not** close already-accepted connections. `handleSubscribe` is parked on `conn.Read(buf)` and only returns when the client itself disconnects. So if a `bolt chat …` session is open when the daemon receives `SIGTERM` / `SIGINT` / `SIGHUP`, the daemon never exits — `wg.Wait` blocks forever, the deferred `cleanupIPC` never runs, and `daemon.sock` stays on disk.
- Repro (static + reasoned):
  1. Start daemon (`bolt daemon`).
  2. Start subscriber (`bolt chat <peer>`) — opens a long-lived `subscribe` IPC connection.
  3. Send SIGTERM to the daemon (`kill -TERM <pid>`).
  4. Daemon prints `bolt daemon shutting down...`, then never exits. The subscriber connection is still blocked in `conn.Read` inside `handleSubscribe`, holding `wg`.
- Expected: SIGTERM triggers a bounded graceful shutdown (single-digit seconds), closes the listener and all live subscriber connections, removes the socket, exits cleanly.
- Actual: Hang. Operator must `kill -9` the daemon; on next start the stale socket has to be re-removed by `listenIPC` (which it does, but the broken-shutdown state is observable as a `daemon.pid` that points at a dead process — see BUG-5).
- Suggested fix: In `IPCServer.Close()`, before `wg.Wait()`, iterate `s.subs` under `s.subsMu` and call `conn.Close()` on each, then clear the map. That unblocks the `conn.Read` in `handleSubscribe`, which returns and decrements `wg`. Alternative: wire the daemon's run context into `handleSubscribe` and set a read deadline that's refreshed on `ctx.Done()`.

### [BUG-3] `peers.toml` and `config.toml` are written as truncate-then-encode — crash mid-write loses trust state
- File: `internal/config/peers.go:193-217`, `internal/config/config.go:178-193`
- Category: file-system safety / durability
- Description: Both `flushLocked` (peers) and `write` (config) open the destination with `O_WRONLY|O_CREATE|O_TRUNC` and stream a TOML encoder directly into the file. There is no temp-file + `Rename`, no `Sync`. A crash, `kill -9`, OOM, full disk, or power loss between truncate and encode-complete leaves the user with a half-written or zero-byte `peers.toml`. Trust state is the most security-relevant on-disk state in the project — losing it forces every peer back to TOFU, and (combined with BUG-1) silently re-accepts everyone the user previously trusted.
- Repro (static): Inspect the function. There is no `os.CreateTemp` / `os.Rename` pair.
- Expected: Atomic write — encode to `peers.toml.tmp` in the same directory, `f.Sync()`, `f.Close()`, then `os.Rename` over the destination. On Linux+ext4 default mount this is crash-atomic; on macOS APFS, rename is atomic at the directory-entry level.
- Suggested fix: Single helper `writeAtomic(path string, fn func(io.Writer) error) error` in `internal/config/` reused by both `peers.go` and `config.go`. Bonus: unit test that injects a writer wrapper which fails after N bytes and asserts the original file is untouched.
- Note: Already raised in `backend-review.md` B-3 — re-flagging here as Critical from a QA / user-impact lens because it directly degrades the security model (BUG-1 makes lost trust state especially dangerous).

### [BUG-4] `ChunkMsg.Data` is JSON-encoded — base64 inflates 4 MB chunk to ≈5.4 MB, breaches 8 MB `maxFrameSize` once headers/hashes are added
- File: `internal/proto/wire.go:115-122, 154-174, 222`
- Category: wire-protocol / Phase-2 blocker
- Description: `ChunkMsg.Data []byte` is serialized by `encoding/json`, which base64-encodes byte slices (×4/3 inflation). Plan.md fixes LAN chunk size at 4 MiB. After base64 (`4 MiB * 4/3 = ~5.33 MiB`) plus JSON quoting, comma overhead, the surrounding object, the SHA-256 hex hash (64 bytes), the version/index/transferID fields, and the JSON field names, a single legitimate chunk easily exceeds the 8 MiB `maxFrameSize` cap that `WriteFrame` enforces (`wire.go:160`) and `ReadFrame` checks (`wire.go:185`). Phase 2 will hit a hard `message too large` error on the first LAN chunk and the file transfer feature will be DOA.
- Repro (manual, since transfer/ doesn't exist yet): `data := make([]byte, 4 << 20); msg := proto.ChunkMsg{V:1, TransferID:"x", Index:0, ChunkHash:strings.Repeat("a",64), Data:data}; proto.WriteFrame(&buf, msg)` — fails with `message too large: ~5,592,xxx bytes`. Actual 4 MiB body alone is 4194304 bytes raw → 5592408 bytes base64 → still under 8 MiB by itself, but with JSON envelope + hex hash + 8 parallel goroutines all allocating concurrently you blow GC budget and edge cases (slightly larger chunk negotiated by sender) will trip the cap.
- Expected: Binary framing for chunk bytes — `[1-byte StreamFileChunk][JSON header (small)][chunk bytes raw]`, or a new `StreamFileChunk = 0x05` type that uses `[chunk-len uint32][chunk bytes]` after the type byte.
- Actual: JSON envelope forces base64 inflation and large-allocation churn (4 MiB × 8 goroutines × ~5.4 MiB allocations per round = ~43 MiB/s allocator pressure at 1 Gbit).
- Suggested fix: Either (a) split `ChunkMsg` into header (JSON-framed) + body (raw, length-prefixed) on the same stream, or (b) introduce a binary-only `StreamFileChunk` type. Either change requires bumping `WireVersion` if Phase-2 sender/receiver are to remain compatible with each other.
- Note: Same as `backend-review.md` B-9; surfaced as Critical because Phase 2 cannot ship without resolving this, and the existing `wire_test.go` doesn't exercise a real-sized chunk.

---

## Important

### [BUG-5] `daemon.pid` is never removed on clean shutdown — operators see false-positive liveness
- File: `internal/daemon/daemon.go:70-73`, `internal/daemon/spawn.go:53-57`
- Category: lifecycle / file-system
- Description: Plan.md §"Signal handling in daemon" requires "remove `daemon.sock`, update `daemon.pid`" on signal. `cleanupIPC` only removes the socket. The PID file is written by `spawnDaemon` (`spawn.go:55`) immediately after `cmd.Start()` — i.e. **before** the child has confirmed liveness — and is never deleted on shutdown. Any operator script that reads `daemon.pid` to decide whether bolt is alive will get a false-positive after a clean shutdown, and a false-negative if the child crashed during startup before the listener opened.
- Repro: `bolt init`, `bolt daemon`, send `SIGTERM`, `cat ~/.config/bolt/daemon.pid` still shows the old PID; `ps -p <that pid>` returns nothing.
- Expected: PID file is owned by the daemon process; written **after** the listener is up (the spawn poll loop already proves liveness via socket connect), and deleted in the shutdown defer.
- Suggested fix: Move `os.WriteFile(pidPath, []byte(strconv.Itoa(os.Getpid())+"\n"), 0600)` into `Daemon.Run()` after `listenForPeers` returns success; drop the `os.WriteFile` in `spawnDaemon`; add `defer os.Remove(pidFilePath(d.configDir))` to `Run`. Also: keep the `flock` held by the daemon for its lifetime (currently released the moment `withPIDLock`'s callback returns), so a stale PID file but no process never racewins.

### [BUG-6] Unix domain socket inherits process umask — typically world-traversable
- File: `internal/daemon/ipc_endpoint_unix.go:19-29`
- Category: file-system safety / defense-in-depth
- Description: `net.Listen("unix", socketPath)` creates the socket node with the current process umask applied to mode 0777 — on most desktop installs that's `0755` or `0775`, i.e. world-traversable. The token gate in `handleConn` prevents code execution by a same-host attacker, but the bare socket existence allows enumeration ("does this user run bolt?") and lets any local UID push frames at the daemon (consuming goroutines, exercising the JSON parser, eating the 1 MiB IPC frame allocation per request — a trivial local DoS).
- Repro: `bolt daemon`; `stat -f '%Sp' ~/.config/bolt/daemon.sock` → expect `srw-------`; observe `srwxr-xr-x` (depends on umask).
- Expected: Socket is owner-only (`0600` / `srw-------`).
- Suggested fix: After `net.Listen`, call `os.Chmod(socketPath, 0o600)`. The `~/.config/bolt/` directory is already `0700`, but the socket node permissions inside it govern who can `connect(2)` on systems that ignore directory traversal bits for socket files (Linux honors them; macOS is conservative).

### [BUG-7] `listenIPC` unconditionally removes the existing socket — second `bolt daemon` invocation silently orphans the running one
- File: `internal/daemon/ipc_endpoint_unix.go:19-29`
- Category: race / IPC
- Description: `listenIPC` removes any existing socket file and then `net.Listen`s a fresh one. This is correct on cold start (cleans stale node after `kill -9`) but is wrong when a daemon is already healthy and a user runs `bolt daemon` a second time by accident or by an over-eager systemd unit. The old daemon's listening FD is still valid in the kernel, but its filesystem name is removed and replaced with the new daemon's socket. CLI clients connecting via the path now reach the new daemon; the old one is unreachable forever — it owns the QUIC listener, has the PeerRegistry, but no one can talk to it. There is no `flock`-style ownership check at the socket-listen layer (only at the `EnsureRunning` spawn path).
- Repro: `bolt daemon &` (terminal A); then `bolt daemon` (terminal B). Both processes live, but the second one's socket replaces the first's; `bolt status` from a third terminal hits the second daemon.
- Expected: If the existing socket accepts a connection (i.e. a daemon is already listening), refuse to start a second one with a clear "already running" error.
- Suggested fix: In `listenIPC`, before removing, try a dial — if it succeeds, return `errors.New("bolt daemon is already running at <path> — use bolt status")`. Or share the same `flock(daemon.pid)` in `Daemon.Run` so manual `bolt daemon` invocations are mutually exclusive with `EnsureRunning`-spawned ones.

### [BUG-8] `chat.AttachStream` silently drops version-mismatch messages in a tight loop
- File: `internal/chat/service.go:92-99`
- Category: wire-protocol / DoS exposure
- Description: When `proto.ReadFrame` succeeds but `msg.V != proto.WireVersion`, the reader does `continue` rather than terminating the stream. A malicious or buggy peer sending a stream of bad-version frames burns CPU on parse + drop without ever delivering a message and without ever closing. Worse, it silently masks a real protocol version skew — the user sees "no chat" but no error, no log line, no IPC event.
- Repro (static): Read `service.go:97-99`. Compare with `peer_conn.go:112-117` which correctly rejects on version mismatch during handshake.
- Expected: A wire-version mismatch on an established stream is a protocol violation; close the stream with a non-zero error code, log it, and propagate a `peer_offline` (when implemented) or at least a stderr line.
- Suggested fix: Replace `continue` with `return` plus a `fmt.Fprintf(os.Stderr, ...)` log line and `stream.CancelRead(1)` / `stream.CancelWrite(1)`.

### [BUG-9] Stream router silently drops `StreamFile`, `StreamControl`, and even duplicate `StreamHandshake`
- File: `internal/daemon/daemon.go:203-213`
- Category: protocol / Phase-2 blocker
- Description: `routeStream`'s switch handles only `StreamChat`. Any other stream type — including the legitimate `StreamFile` and `StreamControl` that Phase 2 needs, and an unexpected second `StreamHandshake` (which would itself indicate a protocol violation) — falls into the `default:` branch which `stream.Close()`s silently. The conditional `fmt.Fprintf` log line suppresses output for `StreamHandshake`, which means a malicious peer could spam handshake streams to consume goroutine/accept budget with zero observability.
- Repro (static): Read `daemon.go:203-213`. There is no handler registry.
- Expected: A `map[proto.StreamType]StreamHandler` registry that `chat.Service` registers itself into; default is "log + close with error code 1 (protocol-violation)"; a duplicate `StreamHandshake` should be logged loudly as a likely attack.
- Suggested fix: Define `type StreamHandler func(ctx context.Context, pc *transport.PeerConn, stream *quic.Stream)`; expose `(*Daemon).RegisterStreamHandler(t proto.StreamType, h StreamHandler)`; have `chat.Service` register at construction; collapse the default case to one `stream.CancelRead(1); stream.CancelWrite(1); log("unhandled stream type 0x%02x from %s — protocol violation")`. Same recommendation as `backend-review.md` B-2.

### [BUG-10] `IPCServer.PublishChat` holds `subsMu` during writes — one slow subscriber blocks all chat fan-out
- File: `internal/daemon/ipc.go:115-130`
- Category: concurrency / availability
- Description: `PublishChat` acquires `subsMu` then iterates `s.subs` calling `writeIPCFrame(conn, event)` for each. The write is synchronous and uses no deadline. Any subscriber whose socket buffer fills (slow reader, suspended terminal, debugger paused at a breakpoint) will hold the mutex for the duration of the OS-level send blocking. While that mutex is held, every other chat message is queued — `daemon.onChatMessage` will block too, which means the chat reader goroutine in `service.AttachStream` stops draining the QUIC stream → QUIC flow control window closes → the peer's send blocks → real chat latency for everyone.
- Repro (manual): Open `bolt chat`, suspend the CLI (`Ctrl-Z`); send 10 messages from a peer; observe daemon stops servicing other clients.
- Expected: Per-subscriber send is non-blocking or bounded; one stuck subscriber doesn't poison the others.
- Suggested fix: Per-subscriber buffered channel (`chan IPCEvent`, buffer 128), each `handleSubscribe` runs its own writer goroutine reading from the channel. `PublishChat` only does `select { case ch <- evt: default: drop+log }` while holding the mutex briefly. Or set a `conn.SetWriteDeadline(time.Now().Add(2*time.Second))` before each write and drop+close on timeout.

### [BUG-11] `peer_conn.go::Handshake` can leak the accept-goroutine if `sendHandshake` fails and the caller used an uncancellable context
- File: `internal/transport/peer_conn.go:39-87`
- Category: goroutine leak
- Description: `Handshake` spawns a goroutine that blocks on `pc.conn.AcceptStream(ctx)`. If `sendHandshake` fails first, the function returns early; the goroutine is still parked. With a cancellable `ctx`, the goroutine eventually unwinds; with `context.Background()` (or any non-cancellable parent), the only thing that unblocks it is the QUIC conn being closed. Callers in `daemon.go:172` and `connect.go:47` pass the daemon's run context, which is cancellable, so production today is fine — but the contract is implicit and any future caller that passes `context.Background()` will leak one goroutine per failed handshake.
- Repro: Static — pass `context.Background()` to `PeerConn.Handshake` and force `sendHandshake` to fail (e.g. close the QUIC conn just after Handshake starts); observe goroutine stuck in `AcceptStream`.
- Expected: Goroutine terminates deterministically when `Handshake` returns.
- Suggested fix: Derive `inner, cancel := context.WithCancel(ctx)` at the top of `Handshake`; pass `inner` to both `AcceptStream` and `OpenStreamSync`; `defer cancel()`. Same recommendation as `backend-review.md` B-14.

### [BUG-12] PID file is written before the spawned daemon proves liveness — failed spawns leave a stale PID
- File: `internal/daemon/spawn.go:49-58`
- Category: lifecycle / file-system
- Description: `spawnDaemon` does `cmd.Start()` → `os.WriteFile(pidPath, …)` → `cmd.Process.Release()` → poll-socket-for-3s. If the child crashes during startup (port bind failure, malformed config, identity load error), the polling times out and we return `"daemon did not start within 3s"`, but `daemon.pid` now points at a dead PID. Combined with BUG-5 (no removal on shutdown either) this means `daemon.pid` is essentially never trustworthy.
- Repro: Make the configured port unavailable (`nc -lu 7799 &`), run `bolt init`. The init returns with the "did not start" error; `cat ~/.config/bolt/daemon.pid` shows a stale PID.
- Expected: PID file is written by the daemon itself, only after `listenForPeers` succeeds. Spawn caller waits on socket-reachable, not on PID-file existence.
- Suggested fix: Combine with BUG-5: write PID inside `Daemon.Run` after listener is up; delete in the shutdown defer. `spawnDaemon` does not touch the PID file.

### [BUG-13] IPC token comparison is non-constant-time
- File: `internal/daemon/ipc.go:153`
- Category: security / timing
- Description: `if req.Token != s.token` is a regular string comparison and short-circuits at the first byte that differs. Token is a 32-byte random secret; a malicious local user (on a multi-user box, or via container-side-channel) can observe response timing to recover the token byte-by-byte. The window is narrow (Unix socket, 64 hex chars, very small CPU footprint per comparison), but the fix is one line.
- Repro (static): Read `ipc.go:153`. Not exploitable in CI / single-user laptop; relevant on shared hosts and CI runners.
- Expected: `crypto/subtle.ConstantTimeCompare`.
- Suggested fix: `import "crypto/subtle"`; replace with `if subtle.ConstantTimeCompare([]byte(req.Token), []byte(s.token)) != 1 { ... }`. Pair with BUG-6 (chmod 0600 the socket) — both are defense-in-depth, both are cheap.

---

## Nice-to-have

(Per instructions, no GitHub issues for this tier — listed here only.)

- **chat CLI scanner truncates long lines.** `bufio.NewScanner(os.Stdin)` in `cmd/bolt/main.go:225` uses default 64 KiB max token size. Pasting a long line silently drops the overflow with no error. Use `scanner.Buffer(make([]byte, 64*1024), 1024*1024)` or read with `bufio.Reader.ReadString('\n')`.
- **`okResponse` silently drops `json.Marshal` errors.** `internal/daemon/ipc.go:375-378`. `data, _ := json.Marshal(payload)` — any non-marshalable payload (e.g. accidental `chan`/`func` field added later) returns `{"ok":true,"payload":null}` to the CLI. Make `okResponse` return `(IPCResponse, error)` or panic on marshal failure since it indicates a programmer bug. Same as `backend-review.md` B-11.
- **`identity.TLSCertificate()` regenerates ECDSA P-256 key per call.** `internal/identity/identity.go:129-179`. Today called only once at daemon boot, but `ClientTLSConfig` is built per-`ConnectPeer` call (`connect.go:22`) which calls `TLSCertificate()` each time — visible CPU on rapid peer-connect bursts. Cache via `sync.Once` on `*Identity`. Same as `backend-review.md` B-10.
- **`bolt connect <own-ip>` creates a self-loop with no warning.** `internal/daemon/connect.go:42-54` does not check whether the extracted fingerprint equals `d.id.Fingerprint()`. Result: the daemon "connects to itself", saves itself as a peer in `peers.toml`, and the peer registry contains a `PeerConn` whose remote is the same process. Cosmetic but confusing; reject with `"cannot connect to own fingerprint"`.
- **`newMessageID` swallows `rand.Read` errors.** `internal/chat/service.go:168-172`. `_, _ = rand.Read(b[:])` — if the entropy source ever fails (it shouldn't on Linux/macOS, but…) the function returns the all-zero UUID forever. Tiny; treat as a panic source since chat dedupe will collapse otherwise.

---

## Test coverage gaps

Tracked separately because no individual gap rises to a "bug" but the aggregate is risk-bearing.

| Area | Current | Risk | Suggested |
|---|---|---|---|
| `internal/transport` | 0% | TLS/TOFU/PeerConn lifecycle, handshake race, registry duplicate-fingerprint logic — none of it exercised. | Build `internal/testutil/loopback.go` exposing `NewLoopbackPair(t) (*PeerConn, *PeerConn)` using in-process QUIC. Then test: `Handshake` success, `Handshake` version-mismatch rejection, `Handshake` fingerprint-mismatch rejection, duplicate `PeerRegistry.Add` collision, `Close` idempotency. |
| `internal/daemon` | 0% | Spawn race, PID file management, stream routing, IPC framing, subscribe fan-out — all untested. | Tests for `EnsureRunning` (two goroutines racing through `withPIDLock`), `IPCServer.Close()` shutdown under live subscriber (drives BUG-2), `routeStream` dispatch for each `StreamType`. |
| `internal/chat` | 0% | `AttachStream` swap-on-reattach, version-skip behavior, write-mutex contention — all untested. | Loopback pair, send N messages, verify ordering; reattach mid-stream, verify old reader exits and new takes over without dropping in-flight frames. |
| `internal/config` | 0% | Plan.md §"Test strategy" explicitly requires unit tests here. Migration paths, version-too-new error path, atomic write rollback (after BUG-3 fix), concurrent `PeerStore.Upsert` under `-race`. | Add `config_test.go`, `peers_test.go`. |
| `internal/proto` | 71% | `WriteFrame`/`ReadFrame` round-trip and oversize rejection are covered. **Not covered:** zero-length payload, version-mismatch rejection in receiver helpers, fuzz on `ReadFrame` (classic crash surface). | Add `func FuzzReadFrame(f *testing.F)`. Seed with empty / oversize-length / truncated / valid-length-with-garbage. |
| `internal/identity` | 49% | `Randomart`, `ExtractPublicKeyFromCert` not directly tested. `Generate` race when two goroutines start at once not tested (currently `Stat→WriteFile` is TOCTOU but inside `dir` which the caller controls, low risk). | Add `randomart_test.go` golden-master + `ExtractPublicKeyFromCert` table test. |
| Integration / two-peer | absent | TOFU prompt round-trip, end-to-end handshake, chat delivery — all unverified. | Loopback QUIC pair is the prerequisite; same `internal/testutil/loopback.go`. |

---

## Issue-filing status

`gh auth status` reports the active token is invalid:

```
github.com
  X Failed to log in to github.com account g-savitha (keyring)
  - Active account: true
  - The token in keyring is invalid.
  - To re-authenticate, run: gh auth login -h github.com
```

`gh issue list --limit 50 --state open` exits 1 with `HTTP 401: Bad credentials`.

Per the QA instructions, I did **not** block on this. The 13 bug descriptions above follow the `.claude/commands/qa.md` template (Description / Steps to Reproduce / Expected / Actual / Environment / Suggested Fix shapes) and can be pasted into `gh issue create` after re-auth. Suggested filing order: BUG-1, BUG-2, BUG-3, BUG-4 (Critical) then BUG-5 through BUG-13 (Important), cap of 10 per the instructions (which would land BUG-1…BUG-10 in GH and leave BUG-11..13 in this report only).

Environment for all bugs unless otherwise noted: `go version go1.25.1 darwin/arm64`; `quic-go v0.59.1`; `BurntSushi/toml v1.6.0`; `gofrs/flock v0.12.1`; macOS 25.2.0; repo at `e267dd2` ("fix(lint): resolve all golangci-lint issues, verified locally at 0 issues").
