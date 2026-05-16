# flick — Terminal P2P LAN + Internet Chat & File Transfer Tool

## Context

Building a terminal-native, peer-to-peer tool for secure chat and fast file transfer between machines the user controls. Started as a broad P2P social network idea, scoped down to: machines on a LAN (or VPN-as-LAN) find each other automatically, authenticate via cryptographic identity, chat and transfer files from the terminal — then extended to support internet peers via a self-hosted relay. Mobile is out of scope for v1.

The gap this fills: no existing tool combines terminal-native UX + authenticated identity + LAN-auto-discovery + internet fallback + chat + fast file transfer in a single binary.

---

## Design Decisions (All Confirmed)

| Decision | Choice |
|---|---|
| Language | Go |
| Transport | QUIC over UDP (`quic-go`) |
| Identity | Ed25519 keypair per machine, TOFU (SSH-style) |
| Peer discovery | mDNS primary + manual IP fallback for VPN/internet |
| Internet path | `pion/ice` hole-punching + `coturn` TURN fallback + configurable relay |
| File transfer | Chunked parallel QUIC streams, SHA-256 verified, resumable, path-aware chunk size |
| Chat persistence | Ephemeral by default; opt-in local logging via `--log` |
| Trust model | per-peer: `always-allow` / `allow-once` (default) / `block` |
| Daemon | Auto-spawn on first command (like ssh-agent); Unix socket IPC with `flock` PID guard |
| TUI | charmbracelet/bubbletea + bubbles + lipgloss |
| Relay storage | `modernc.org/sqlite` (pure Go, no CGO) — survives restarts, cross-compiles cleanly |
| Wire versioning | All messages carry `"v":1`; config files carry `version = 1` |
| Relay auth | Per-peer HMAC tokens: `HMAC(relay_secret, peer_fingerprint)` — no shared secrets |
| Relay TLS | Let's Encrypt via `autocert` if domain available; self-signed + client-pinned if IP-only |
| Platform support | Linux + macOS v1; Windows explicitly unsupported (Unix socket IPC incompatible) |
| Receive directory | Configurable `receive_dir` in `config.toml`; default `~/Downloads` |
| Chunk size | Path-aware: 4MB on LAN, 256KB over TURN relay |

---

## Resolved Design Issues (All Critic Passes)

**Wire protocol versioning**
Every on-wire message type includes `"v": 1`. Clients reject unknown versions gracefully. Config files include `version = 1`; missing fields get written with defaults on load (forward-compatible migration).

**Relay rate limiting**
`flickd` enforces per-IP token bucket: 10 req/s, burst 100. Applied to registration, heartbeat, and hole-punch endpoints. Prevents trivial DoS.

**Peer nickname vs identity**
Nicknames are display-only and mutable. Canonical identity is always the fingerprint. CLI resolves peer names via fingerprint lookup. Ambiguous nicknames prompt for disambiguation. `peers.toml` uses fingerprint as primary key.

**Peer registry TTL + heartbeat**
Peers heartbeat the relay every 30s. Relay marks offline after 90s (3 missed). Background goroutine in `flickd` prunes stale entries every 60s. `flick peers` never shows ghost peers.

**File transfer size limits**
Configurable `max_relay_transfer` in `config.toml` (default: 5GB over TURN). No limit on LAN. Receiver checks available disk space before sending `TransferAck{Accepted:true}` — rejects with clear error if insufficient. Sender warned before initiating over-limit transfer.

**Group chat fan-out scope**
Fan-out only to peers with mutual established trust. Skipped peers reported to sender. File broadcast (`flick send --all`) explicitly blocked in v1. Documented limitation.

**Relay access control — per-peer HMAC tokens**
Each peer's relay token = `HMAC-SHA256(relay_secret, peer_fingerprint)`. Relay verifies without storing a token list. Revoking one peer (add fingerprint to a blocklist) doesn't affect others. No shared secret that leaks access for everyone.

**Transfer history**
No history by default. With `--log` or `log_transfers = true` in config: `~/.local/share/flick/logs/transfers-YYYY-MM-DD.log`. Format: `timestamp | direction | filename | size | peer | sha256 | duration`.

**Stealth mode scope**
`--stealth` suppresses both mDNS announce and relay registration. Machine fully dark — reachable only via direct `flick connect <ip>`. Documented explicitly.

**TOFU fingerprint UX**
Confirmation screen shows: full hex fingerprint + SSH-style randomart visual + instruction to verify out-of-band + `Accept? [y/N]`. Randomart implemented via `golang.org/x/crypto/ssh` internal algorithm (copy the ~80-line drunken bishop implementation — it's MIT licensed and not exported).

**SQLite — pure Go, no CGO**
`modernc.org/sqlite` instead of `go-sqlite3`. Same API, no CGO, cross-compiles cleanly with `GOOS=linux GOARCH=arm64 go build`. Eliminates toolchain dependency for relay binary.

**Windows unsupported in v1**
Unix socket IPC (`daemon.sock`) is incompatible with Windows without significant abstraction (named pipes). v1 explicitly supports Linux + macOS only. Windows support is post-v1 and requires replacing the IPC layer with a platform-agnostic alternative (e.g. TCP loopback with auth token).

**Signal handling in daemon**
`daemon.go` registers `SIGTERM`/`SIGINT`/`SIGHUP` handlers on startup. On signal: checkpoint all in-progress transfers (flush resume state JSON), close QUIC connections gracefully, stop relay heartbeat, remove `daemon.sock`, update `daemon.pid`. Prevents corrupt `.tmp` files and stale PID on crash/reboot.

**Concurrent `peers.toml` writes**
All reads and writes to `peers.toml` go through a single `PeerStore` struct protected by a `sync.RWMutex`. No goroutine touches the file directly — all access via `PeerStore` methods. This is initialised once in `daemon.go` and passed by pointer.

**Daemon PID race condition**
`EnsureDaemon()` acquires an exclusive `flock` on `daemon.pid` before reading or writing. If two CLI processes race, the second blocks until the first has either confirmed a running daemon or spawned one and released the lock. No double-spawn possible.

**`flick send` partial failure behaviour**
When sending to multiple peers, offline peers are skipped (not errors). Summary printed after all attempts: `✓ peer1 (2.3s)  ✗ peer2 (offline)  ✓ peer3 (1.1s)`. Exit code 0 if at least one succeeded, non-zero if all failed. Individual peer failures never abort in-progress transfers to other peers.

**Receive directory**
Configurable `receive_dir` in `config.toml`. Default: `~/Downloads`. Receiver resolves final filename at the point `TransferDone` is verified — never overwrites existing files (appends `_1`, `_2` suffix). User can override per-session with `flick receive-dir /tmp` (sets for current daemon session only).

**Path-aware chunk size**
LAN path: 4MB chunks (maximises throughput on gigabit). TURN relay path: 256KB chunks (reduces per-chunk round-trip cost on slower connections). Detection: if connection was established via ICE direct, use LAN chunk size; if via TURN, use relay chunk size. Negotiated in `FileHeader`.

**`flick chat --all` trust guard**
Fan-out skips peers that have not completed mutual TOFU or have trust level `block`. Sends only to peers in `PeerRegistry` with authenticated `PeerConn`. Skipped peers logged to stderr: `skipped peer3 — trust not established`.

**Config file versioning and migration**
Both `config.toml` and `peers.toml` include `version = 1`. On load, if version is missing, assume 0 and run migration (write defaults for all missing fields, bump version). If version is higher than understood, warn and exit rather than silently misparse.

**Relay TLS**
If `relay_domain` is set in `flickd.toml`: use `golang.org/x/crypto/acme/autocert` for automatic Let's Encrypt cert. If not set (IP-only deployment): generate self-signed cert on first start, store in `flickd_cert.pem`. Clients must pin the relay's cert fingerprint in `config.toml` (`relay_cert_fingerprint`). Documented in deploy guide.

**Test strategy**
Unit tests required for: `internal/identity/` (keygen, fingerprint determinism), `internal/transfer/chunk.go` (chunking, hash correctness), `internal/proto/wire.go` (framing encode/decode round-trips), `internal/config/` (TOML load/save, migration). Integration tests required for: TOFU handshake flow (two in-process QUIC endpoints), full file transfer with corruption injection, resume after mid-transfer kill. No mocks for crypto or network — use real implementations with loopback.

---

## CLI Interface

```bash
flick init                           # generate keypair, start daemon
flick id                             # show your fingerprint + randomart
flick peers                          # list online peers (LAN + relay)
flick connect <ip>                   # manually add peer (VPN / internet)
flick trust <peer> always|once|block # set per-peer trust level
flick send <file> <peer> [peer2...]  # send file; reports per-peer success/fail
cat file | flick send - <peer>       # pipe support
flick chat <peer>                    # 1:1 TUI chat
flick chat --all                     # group chat (trusted peers only)
flick daemon [--stealth]             # explicit daemon; --stealth = fully dark
flick status                         # daemon stats, peers, transfers, relay status
flick receive-dir <path>             # set receive directory for this session
```

---

## Project Structure

```
flick/
├── cmd/
│   ├── flick/main.go               # cobra root + subcommand wiring
│   └── flickd/main.go              # relay server binary
├── internal/
│   ├── identity/
│   │   ├── identity.go             # Ed25519 keygen, load, TLS cert
│   │   ├── fingerprint.go          # SHA-256 fingerprint + randomart
│   │   └── identity_test.go
│   ├── config/
│   │   ├── config.go               # config.toml load/save + migration
│   │   ├── peers.go                # PeerStore with RWMutex, TOML CRUD
│   │   └── config_test.go
│   ├── daemon/
│   │   ├── daemon.go               # main loop + signal handling
│   │   ├── spawn.go                # flock-guarded auto-spawn
│   │   ├── ipc_server.go           # Unix socket server
│   │   └── ipc_client.go           # Unix socket client
│   ├── transport/
│   │   ├── quic.go                 # QUIC listen + dial wrappers
│   │   ├── tls.go                  # TLS config from Ed25519 identity
│   │   ├── peer_conn.go            # PeerConn: long-lived QUIC connection per peer
│   │   ├── ice.go                  # pion/ice integration; path detection (LAN vs TURN)
│   │   └── stream_mux.go           # stream type byte routing
│   ├── discovery/
│   │   ├── mdns.go                 # zeroconf announce + browse
│   │   ├── relay.go                # relay registration, heartbeat, peer lookup
│   │   └── registry.go             # in-memory online peer registry (PeerRegistry)
│   ├── transfer/
│   │   ├── sender.go               # chunk + parallel stream send
│   │   ├── receiver.go             # reassemble + verify + write + disk check
│   │   ├── chunk.go                # Chunk struct, path-aware chunk size, SHA-256
│   │   ├── resume.go               # incomplete transfer state (JSON)
│   │   ├── progress.go             # bubbles progress bar integration
│   │   └── transfer_test.go        # unit + integration tests
│   ├── chat/
│   │   ├── message.go              # Message struct
│   │   ├── fanout.go               # trust-gated group broadcast
│   │   └── log.go                  # opt-in daily log files
│   ├── tui/
│   │   ├── chat_model.go           # bubbletea chat screen
│   │   ├── peers_model.go          # bubbletea peer list
│   │   └── styles.go               # lipgloss styles
│   └── proto/
│       ├── wire.go                 # on-wire types with v field; encode/decode
│       ├── wire_test.go            # round-trip framing tests
│       └── ipc.go                  # IPC request/response/event types
├── relay/
│   ├── server.go                   # flickd HTTP(S) server, rate limiter
│   ├── registry.go                 # SQLite peer registry (modernc.org/sqlite)
│   ├── auth.go                     # HMAC token verification + blocklist
│   └── tls.go                      # autocert or self-signed cert management
├── deploy/
│   ├── flickd.toml.example         # relay config template
│   ├── coturn.conf.example         # coturn config template
│   └── flickd.service              # systemd unit for relay
├── go.mod
└── Makefile
```

### Storage Layout

```
~/.config/flick/
  identity/private.key     (0600)
  identity/public.key
  peers.toml               # version=1; fingerprint→{name,ip,trust,first_seen}
  config.toml              # version=1; nickname, port, receive_dir, relay, relay_cert_fingerprint
  daemon.sock              # Unix socket (runtime, deleted on clean shutdown)
  daemon.pid               # flock-protected PID file

~/.local/share/flick/
  logs/<peer>-YYYY-MM-DD.log         # opt-in chat logs
  logs/transfers-YYYY-MM-DD.log      # opt-in transfer history
  incomplete/<uuid>.json             # resume state
  incomplete/<uuid>.tmp              # partial file data
```

---

## Key Data Structures

### Wire Protocol (`internal/proto/wire.go`)

Every QUIC stream starts with a 1-byte type header, then length-prefixed JSON frames. All JSON payloads include `"v":1`.

```
StreamTypeHandshake = 0x01   one-shot: exchange public keys + nickname
StreamTypeChat      = 0x02   persistent bidirectional stream per connection
StreamTypeFile      = 0x03   one control stream + N parallel chunk streams per transfer
StreamTypeControl   = 0x04   acks, cancel, heartbeat
```

Key message types: `HandshakeMsg`, `ChatMsg`, `FileHeader` (includes `chunk_size`), `ChunkMsg`, `TransferAck` (includes disk-space rejection reason), `TransferDone`.

### IPC Protocol (`internal/proto/ipc.go`)

Length-prefixed JSON over Unix socket. Commands: `status`, `send_file`, `send_chat`, `list_peers`, `trust`, `connect`, `subscribe`, `tofu_response`, `receive_dir`. Events: `peer_online`, `peer_offline`, `chat`, `transfer_progress`, `transfer_complete`, `transfer_skipped`, `tofu_prompt`.

---

## Phase-by-Phase Implementation

### Phase 1 — Core Transport (Week 1-2)
**Goal:** Two machines establish an authenticated encrypted QUIC connection.

1. Scaffold: `go mod init github.com/<user>/flick`, cobra skeleton, all subcommand stubs, Makefile
2. `internal/identity/`: Ed25519 keygen → PEM files (private 0600); `Fingerprint()` = colon-hex SHA-256; `TLSCertificate()` = self-signed x509; randomart from drunken-bishop; unit tests
3. `internal/config/`: versioned load/save with migration; `PeerStore` with `sync.RWMutex`; unit tests for migration
4. `internal/transport/tls.go`: server config (`RequireAnyClientCert`); client config with `VerifyPeerCertificate` for TOFU
5. `internal/transport/quic.go`: `Listen()` and `Dial()` wrappers; `MaxIncomingStreams:1000`, `KeepAlivePeriod:15s`
6. TOFU handshake: TLS completes → extract fingerprint → lookup in PeerStore → unknown: push `tofu_prompt` IPC event, wait 30s → accepted: write PeerStore → exchange `HandshakeMsg` → emit `peer_online`
7. Signal handling in `daemon.go`: `SIGTERM`/`SIGINT` → checkpoint transfers → cleanup socket/PID → exit
8. `flick init`: keygen + versioned config + flock-guarded daemon spawn
9. `flick id`: load identity, print fingerprint + randomart

**Phase 1 verification:**
- `flick init` → identity files exist, correct permissions
- `flick id` → same fingerprint on repeated calls
- `tcpdump` on UDP 7799 → zero plaintext
- Unknown peer → TOFU prompt with randomart; second connection → no prompt
- Kill daemon mid-operation → no stale socket, clean PID

---

### Phase 2 — File Transfer (Week 2-3)
**Goal:** `flick send file peername` with progress, hash verification, resume, disk-space guard.

1. `internal/transfer/chunk.go`: `ChunkFile(r io.Reader, chunkSize int)` → `<-chan Chunk`; per-chunk + whole-file SHA-256; unit tests for hash correctness and chunk boundary handling
2. `internal/transfer/sender.go`:
   - Negotiate chunk size with receiver via `FileHeader{chunk_size}`
   - Spawn `min(8, totalChunks)` goroutines, each on own QUIC stream
   - On partial failure (one goroutine fails): retry that chunk, not whole file
3. `internal/transfer/receiver.go`:
   - Check trust level; prompt if `allow-once`
   - Check disk space: `syscall.Statfs` → reject with `TransferAck{Accepted:false, Reason:"insufficient_disk"}` if needed
   - `ftruncate` to full size; `WriteAt` for concurrent writes
   - Checkpoint resume JSON every 10 chunks
   - On `TransferDone`: verify whole-file SHA-256; rename to `receive_dir/<filename>` (no overwrite — append `_1` suffix)
4. `internal/transfer/progress.go`: emit `transfer_progress` IPC events; CLI renders `bubbles/progress` bar
5. Stdin: buffer to `incomplete/pipe-<uuid>.tmp`; daemon sends that file
6. `flick send` multi-peer: parallel sends; collect results; print per-peer summary; exit code reflects overall success
7. Integration test: full transfer with injected chunk corruption → retry → arrival; mid-transfer kill → resume

---

### Phase 3 — Peer Discovery + Daemon (Week 3-4)
**Goal:** LAN auto-discovery; daemon auto-spawns with race-safe PID handling.

1. `internal/discovery/mdns.go`: `Announce()` registers `_flick._udp`; `Browse()` emits `DiscoveredPeer`
2. `internal/discovery/registry.go`: `PeerRegistry` (`map[fingerprint]*PeerConn`, `sync.RWMutex`); on discovery → dial → authenticate → `Add()`; on disconnect → `Remove()` + `peer_offline` event
3. `internal/daemon/daemon.go`: QUIC listener + mDNS + IPC server; stream type routing; signal handlers
4. `internal/daemon/spawn.go`: `flock` on `daemon.pid` before read/write; re-exec via `os.Executable()` with `Setsid:true`; poll socket max 2s
5. `internal/daemon/ipc_server.go`: `[4-byte len][JSON]` framing; fan-out events to subscribers
6. `flick peers`: `EnsureDaemon()` → `CmdListPeers` → lipgloss table with online/offline indicators
7. `flick connect <ip>`: `CmdConnect` → daemon dials → TOFU flow
8. Stealth: `Config.Stealth=true` → skip `Announce()` only (Browse still runs for passive discovery)

**Phase 3 verification:**
- Two machines on same LAN → `flick peers` lists each other within 5s
- Two terminals run `flick peers` simultaneously → single daemon spawned (no race)
- `--stealth` machine absent from peers list; reachable via `flick connect`

---

### Phase 4 — Chat TUI (Week 4-5)
**Goal:** Real-time 1:1 and group chat; trust-gated fan-out.

1. `internal/chat/message.go`: `Message{ID, From, FromFP, Body, Timestamp, GroupID}`
2. Daemon chat handler: `ChatMsg` frames → ring buffer (1000 msgs/peer) → push `chat` IPC event
3. `internal/chat/fanout.go`: `Fanout(registry, msg)` → sends only to peers with established `PeerConn`; returns skipped list
4. `internal/chat/log.go`: daily log `logs/<peer>-YYYY-MM-DD.log`; opt-in only
5. `internal/tui/chat_model.go`: `viewport` (scrollable history) + `textinput` (message entry); handles `tea.WindowSizeMsg` for resize; Ctrl+C to quit
6. `flick chat <peer>`: `EnsureDaemon()` → bubbletea program
7. `flick chat --all`: same model; `GroupID` set; daemon uses `Fanout`; skipped peers printed to stderr

---

### Phase 5 — Internet Path + Relay (Week 5-7)
**Goal:** flick works across the internet via self-hosted relay.

1. `relay/` package — `flickd` binary:
   - HTTPS server (`autocert` if domain set; self-signed + pinned if IP-only)
   - `modernc.org/sqlite` peer registry with TTL + heartbeat pruning
   - Per-IP token bucket rate limiting
   - HMAC-token auth: verify `HMAC-SHA256(relay_secret, peer_fingerprint)` without storing token list
   - Fingerprint blocklist for revocation
   - Endpoints: `/register`, `/heartbeat`, `/lookup/<fingerprint>`, `/punch` (hole-punch coordination)
2. `internal/transport/ice.go`:
   - `pion/ice` integration; ICE agent negotiation over relay `/punch` endpoint
   - After ICE: wrap established UDP socket as `net.PacketConn` → pass to `quic-go` `Transport` API (not `DialAddr`)
   - Record path type (`LAN` / `TURN`) on `PeerConn` → chunk size selection in `FileHeader`
3. `internal/discovery/relay.go`: register on startup (unless stealth); heartbeat every 30s; lookup peers by fingerprint
4. `deploy/`: `flickd.toml.example`, `coturn.conf.example`, `flickd.service` (systemd)
5. Relay endpoint configurable: `relay = "https://relay.example.com"` in `config.toml`; `relay_cert_fingerprint` for self-signed pinning
6. Stealth updated: suppresses both mDNS and relay registration

**Phase 5 verification:**
- Machine A (Hyderabad) and Machine B (Amsterdam) connect via relay
- `flick peers` shows remote peer; `flick send` delivers file
- File SHA-256 matches across internet transfer
- Kill relay mid-transfer: transfer fails gracefully with clear error, not hang

---

### Phase 6 — Polish + Release (Week 7-8)
**Goal:** Resume, logging, error messages, cross-compile.

1. Resume on daemon start: scan `incomplete/*.json` → reconnect + `TransferAck{ResumeFrom:[...]}`
2. Transfer log when `--log` active: `logs/transfers-YYYY-MM-DD.log`
3. Error message audit: every user-visible error must be actionable. Replace all generic "connection failed" with specific guidance.
4. Cross-compile Makefile: `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64` — Windows explicitly excluded with comment
5. `flick status`: relay connection status, active transfers with progress, peer list

---

## Go Libraries

```
github.com/quic-go/quic-go          transport (QUIC/TLS 1.3)
github.com/pion/ice/v2              ICE hole-punching; path detection
github.com/grandcat/zeroconf        mDNS announce + browse
golang.org/x/crypto                 Ed25519, acme/autocert
modernc.org/sqlite                  pure-Go SQLite for relay (no CGO)
github.com/charmbracelet/bubbletea  TUI framework
github.com/charmbracelet/bubbles    progress bar, textinput, viewport
github.com/charmbracelet/lipgloss   terminal styling
github.com/spf13/cobra              CLI structure
github.com/BurntSushi/toml          config files

# External — deploy on VPS, do not rewrite:
coturn                              battle-tested TURN server
```

---

## Critical Files (Implement These Carefully)

| File | Why |
|---|---|
| `internal/transport/peer_conn.go` | Every feature passes through here — QUIC stream routing, path type, registry lifecycle |
| `internal/transport/ice.go` | pion/ice → quic-go `Transport` API bridge is the hardest plumbing in the project |
| `internal/daemon/daemon.go` | Orchestrates all subsystems; signal handling must be correct or data is lost |
| `internal/proto/wire.go` | All on-wire types; framing correctness determines all peer communication |
| `internal/transfer/receiver.go` | Most complex state machine: trust, disk check, resume, concurrent writes, SHA-256 verify |
| `internal/daemon/ipc_server.go` | Unix socket framing + event fan-out — foundation for all CLI commands and TUI |
| `relay/auth.go` | HMAC token verification + blocklist — security boundary of the relay |

---

## Known Limitations (v1, Document Clearly)

- Windows not supported (Unix socket IPC)
- File broadcast (`flick send file --all`) not supported
- Group chat is sender-side fan-out (N copies of each message)
- No message queuing — both peers must be online simultaneously to chat or transfer
- Relay sees peer IP addresses and online presence (stated privacy tradeoff)
- ICE/hole-punch may fail on symmetric NAT + CGNAT; TURN fallback adds relay-side bandwidth cost
- No forward secrecy beyond TLS session keys (no Double Ratchet)

---

## Verification Per Phase

**Phase 1:** identity files created, correct permissions; same fingerprint on repeated `flick id`; tcpdump shows zero plaintext; TOFU prompt on first contact; clean shutdown leaves no stale socket

**Phase 2:** SHA-256 matches on received file; resume after kill completes faster; disk-full rejection surfaces clear error; multi-peer send reports per-peer result

**Phase 3:** LAN peers appear within 5s; simultaneous CLI invocations spawn single daemon; stealth invisible to peers but reachable direct

**Phase 4:** messages < 100ms on LAN; history gone on restart (ephemeral); group skips untrusted peers and reports them; TUI reflows on resize

**Phase 5:** Hyderabad↔Amsterdam file transfer works; SHA-256 matches; relay down → graceful error not hang; TURN path uses 256KB chunks

**Phase 6:** 10GB file resumed from 30%; all binaries run on target platforms without deps; every error message includes a suggested next action
