# Security Review — 2026-05-19

Reviewer: Security Engineer
Scope: read-only static audit of the bolt repo plus tooling pass (`govulncheck`,
`go list -m -u all`, `go mod verify`). New findings only; previously-flagged
issues are listed in §Coverage cross-check and not re-litigated.

---

## Coverage cross-check

Issues already captured by the parallel reviewers — confirmed during this pass
and **not** re-raised in §New findings:

- TOFU silent-accept (`peerVerifier` auto-accepts non-blocked peers,
  `authenticateIncoming` same, no `tofu_prompt` IPC event) — Q-BUG-1 / B-1 /
  A-3 → **BOLT-001**.
- `ServerTLSConfig` ships with `ClientAuth: tls.RequireAnyClientCert` and no
  `VerifyPeerCertificate` — N-§2 / Q-BUG-1 → **BOLT-001 + BOLT-002**.
- No Ed25519 proof-of-possession in `HandshakeMsg` — cert↔key binding is
  construction-time only (`SubjectKeyId = pubkey`) — N-§2 / N-decision-4 →
  **BOLT-002**.
- `peers.toml` and `config.toml` are `O_WRONLY|O_CREATE|O_TRUNC` rewrites with
  no temp+rename and no fsync — Q-BUG-3 / B-3 / A-19 → **BOLT-003**.
- Daemon SIGTERM hangs forever when a subscriber is attached
  (`IPCServer.Close` does not close subscriber conns) — Q-BUG-2 / A-7 / B-8 →
  **BOLT-004**.
- `daemon.pid` written by `spawnDaemon` before liveness, never removed on
  shutdown — Q-BUG-5 / Q-BUG-12 / B-4 → **BOLT-005**.
- IPC token comparison is `req.Token != s.token` (timing-leakable) —
  Q-BUG-13 → covered (listed there as Important, recommended `subtle.
  ConstantTimeCompare`). **Confirmed present in code at `ipc.go:153`.** Not
  re-raised here, but folded into the IPC-hardening pre-work list in §Phase 5.
- Unix domain socket inherits umask (not chmodded to 0600 explicitly) —
  Q-BUG-6 → covered. Not re-raised.
- `listenIPC` unconditionally removes any pre-existing socket, allowing a
  second `bolt daemon` to silently orphan the running one — Q-BUG-7. Not
  re-raised.
- `ChunkMsg.Data []byte` JSON+base64 inflation breaches 8 MiB cap — Q-BUG-4 /
  B-9 / N-§3 → **BOLT-008** (chunk framing freeze).
- `quic.Config` under-specified (no `MaxIdleTimeout`, no `Allow0RTT`, no
  receive-window tuning, no `EnableDatagrams`, no `SessionTicketsDisabled`) —
  N-§1, N-decisions-3..7 → **BOLT-007** with `Allow0RTT=false` and
  `SessionTicketsDisabled=true` locked.
- 0-RTT replay-risk posture — N-§9, N-decision-4 → **BOLT-007** (`Allow0RTT
  =false`).
- Phase-5 relay HMAC token single-secret blast radius — N-§10. Carried into
  §Phase 5 pre-work below.

---

## Tooling output

### `go version` / `go mod verify`

- `go version go1.25.1 darwin/arm64`.
- `go mod verify` → `all modules verified` (every module's `.zip` matches
  `go.sum`).
- No `replace` directives in `go.mod` (verified).

### `govulncheck ./...`

Installed via `go install golang.org/x/vuln/cmd/govulncheck@latest` (v1.3.0
vuln DB pulled at review time). Raw output saved to `/tmp/govulncheck.txt`
during the review; the analysis below is the triaged form.

- **Symbol Results (reachable from bolt code): 15 stdlib vulnerabilities** in
  `crypto/tls`, `crypto/x509`, `encoding/asn1`, `encoding/pem`, `net/url`,
  `net`. All shipped in Go 1.25.1; fixes are in Go 1.25.2 through 1.25.10. See
  **[SEC-1]** below for the consolidated finding and the bump plan.
- **Package Results (imported but not called by symbol trace): 4** — `net`,
  `internal/syscall/unix`, `os`, `net/url`. Same toolchain bump fixes them.
- **Module Results (in `require` chain, not in symbol trace): 14** — all
  stdlib, all fixed by the same toolchain bump.
- No third-party module (`quic-go`, `BurntSushi/toml`, `gofrs/flock`,
  `cobra`, `golang.org/x/crypto`, `golang.org/x/net`, `golang.org/x/sys`)
  shows an advisory in the local DB at review time.

Highest-severity reachable issues (one-liners; full Go IDs in [SEC-1]):

| ID            | Subsystem    | Description                                                                       |
| ------------- | ------------ | --------------------------------------------------------------------------------- |
| GO-2026-4870  | crypto/tls   | Unauthenticated TLS 1.3 KeyUpdate DoS (persistent connection retention). **Reachable** via `transport.Dial` → `quic.DialAddr` → `tls.QUICConn.HandleData`. |
| GO-2026-4337  | crypto/tls   | Unexpected session resumption. **Reachable** via `tls.QUICConn.Start`. |
| GO-2026-4340  | crypto/tls   | Handshake messages processed at incorrect encryption level. **Reachable** via QUIC handshake. |
| GO-2025-4008  | crypto/tls   | ALPN error string contains attacker-controlled bytes. **Reachable** via QUIC handshake. |
| GO-2025-4011  | encoding/asn1 | DER parsing memory exhaustion. **Reachable** via `identity.TLSCertificate` → `tls.X509KeyPair`. |
| GO-2025-4009  | encoding/pem | Quadratic complexity parsing crafted PEM. **Reachable** via `tls.X509KeyPair` on cert-load. |
| GO-2026-4947  | crypto/x509  | Unexpected work during chain building. **Reachable** via `x509.Certificate.Verify`. |

### `go list -m -u all`

Stale dependencies with an available upgrade (none flagged with an active
CVE in the local DB, but the spread is wide enough that any new advisory
would not land cleanly):

```
github.com/gofrs/flock           v0.12.1 → v0.13.0
github.com/spf13/pflag           v1.0.9  → v1.0.10
golang.org/x/mod                 v0.27.0 → v0.36.0
golang.org/x/net                 v0.53.0 → v0.54.0
golang.org/x/sync                v0.16.0 → v0.20.0
golang.org/x/tools               v0.36.0 → v0.45.0
github.com/cpuguy83/go-md2man/v2 v2.0.6  → v2.0.7   (indirect, cobra dep)
github.com/jordanlewis/gcassert  v0.0.0-… (indirect)
github.com/rogpeppe/go-internal  v1.10.0 → v1.14.1  (test-only)
go.uber.org/mock                 v0.5.2  → v0.6.0   (test-only)
```

`quic-go v0.59.1`, `BurntSushi/toml v1.6.0`, `cobra v1.10.2`, `crypto v0.51.0`,
`sys v0.44.0` are at current latest (no upgrade reported).

### Lint (security category)

- `golangci-lint run` already passes at `0 issues` (per backend-review). The
  configured set includes `gosec`. No additional sec-only run was needed.
- The repo annotates the few legitimate `//nolint:gosec` lines with
  justifications (G304 path traversal where the path is daemon-controlled,
  G115 size cast where bounded, G306 mode where the file is intentionally
  public). All annotations were re-validated during this review and the
  justifications are accurate.

---

## New findings (not covered by other reports)

### Critical

#### [SEC-1] Reachable Go stdlib vulnerabilities — toolchain pinned at 1.25.1

- Severity: Critical
- Vulnerability: 15 known Go stdlib advisories are reachable through bolt's
  call graph. The most consequential are
  (a) **GO-2026-4870** — an unauthenticated TLS 1.3 `KeyUpdate` record can
  cause persistent connection retention and DoS (`crypto/tls`, fixed in
  1.25.9). Reachable via every incoming and outgoing QUIC handshake.
  (b) **GO-2025-4011** — DER parsing memory exhaustion in `encoding/asn1`
  (fixed in 1.25.2). Reachable via `identity.TLSCertificate()` →
  `tls.X509KeyPair`, run on every daemon boot and every outgoing `Dial`.
  (c) **GO-2025-4009** — quadratic PEM parsing (fixed in 1.25.2). Reachable
  via `tls.X509KeyPair` on cert-load.
  (d) **GO-2025-4008** — ALPN negotiation error contains
  attacker-controlled information (`crypto/tls`, fixed in 1.25.2). Could leak
  attacker-chosen bytes into bolt's error wraps written to stderr.
  (e) **GO-2026-4340** — handshake messages processed at the wrong
  encryption level (TLS state confusion; fixed in 1.25.6).
- Attack vector: a network attacker who can reach UDP/7799 on a daemon (or a
  malicious peer the daemon dials) can trigger TLS-stack behaviours that
  hold or kill connections, or send malformed PEM/DER if the cert path is
  ever exercised against attacker-supplied bytes (today the cert path
  ingests only the local identity's own bytes, but the codepath is
  reachable from the QUIC handshake).
- Impact: DoS of the daemon (most advisories); the KeyUpdate one specifically
  keeps a TLS connection alive after the peer is gone, leaking goroutines
  and file descriptors. None are RCE.
- Reproduction / static evidence:
  - `go version` → `go1.25.1 darwin/arm64`; `go.mod` line 3 says
    `go 1.25.1`. No `toolchain` directive.
  - `govulncheck ./...` exits 3 (vulns found). Symbol Results enumerate 15
    advisories, all stdlib, all with reachable call traces. Raw output in
    §Tooling output above.
  - Example trace for GO-2026-4870:
    `internal/transport/quic.go:56:28: transport.Dial calls quic.DialAddr,
    which eventually calls tls.QUICConn.HandleData`.
- Recommended fix: bump the Go toolchain pin and the CI matrix to Go
  1.25.10 (or whatever the latest patch on the 1.25.x line is at fix time).
  Concrete steps:
  1. `go.mod` line 3 → `go 1.25.10` (or current latest patch).
  2. Add a `toolchain go1.25.10` line so contributors with older Go
     installs fail loudly instead of silently building against an
     unpatched stdlib.
  3. CI workflows already pin via `go-version-file: go.mod` (good); after
     the bump they will install 1.25.10 automatically.
  4. `goreleaser` runs in CI; after the bump every release binary picks up
     the patched stdlib without further action.
  5. Re-run `govulncheck ./...` and confirm 0 reachable advisories.
- Maps to: **new story BOLT-024 (proposed)** — "Bump Go toolchain to 1.25.10
  to close 15 reachable stdlib advisories." Cite govulncheck output snapshot.

---

### High

#### [SEC-2] Terminal escape injection via peer-controlled `Nickname` and `ChatMsg.Body`

- Severity: High
- Vulnerability: every peer-controlled string (`HandshakeMsg.Nickname`,
  `ChatMsg.From`, `ChatMsg.Body`) is printed to a real terminal — daemon
  stderr and chat CLI stdout — via `fmt.Fprintf` / `fmt.Printf` with no
  filtering of ASCII control bytes or ANSI/CSI/OSC escape sequences. A
  malicious peer (once authenticated, which today is automatic per BOLT-001)
  can:
  - Send a nickname containing `\x1b]2;evil\x07` (OSC-2 set-title) — changes
    the user's terminal window title without the user noticing.
  - Send a nickname containing `\x1b[2J\x1b[H` (clear-screen + cursor-home)
    — wipes the user's chat history.
  - Send a chat body containing `\r` — overwrites the previous chat line, can
    forge prior messages from another peer in the scrollback view.
  - Send `\x1b[8m`-wrapped text (hidden) — sneaks invisible content into
    chat scrollback that copies/pastes later.
  - On macOS Terminal.app, `\x1b]52;c;...\x07` (OSC-52) sets the system
    clipboard — a peer can plant arbitrary text in the user's clipboard
    when they receive a chat.
- Attack vector: any authenticated peer (which post-TOFU-fix in BOLT-001
  means any peer the user has accepted at least once, or — until BOLT-001
  ships — any peer at all on the LAN).
- Impact: clipboard hijack, scrollback forgery / phishing (impersonating
  another known peer in chat scrollback), terminal title spoofing, screen
  clear. Combined with [SEC-8] (nickname confusion), a malicious peer can
  impersonate a trusted contact in chat scrollback persuasively.
- Reproduction / static evidence:
  - `internal/transport/peer_conn.go:137` — `pc.peerNickname = msg.Nickname`
    with no sanitisation of `msg.Nickname` (peer-supplied string).
  - `internal/daemon/daemon.go:188` —
    `fmt.Fprintf(os.Stderr, "peer connected: %s (%s)\n", pc.PeerNickname()
    , ...)`. Raw printf of peer-controlled bytes to a real TTY.
  - `internal/daemon/daemon.go:190, 210` — `peer disconnected`,
    `unhandled stream 0x%02x from %s\n` — same pattern.
  - `internal/daemon/daemon.go:141` —
    `fmt.Fprintf(os.Stderr, "authentication failed: %v\n", err)`. The
    error chain wraps `pc.peerNickname` via
    `internal/transport/peer_conn.go:158,173,179`.
  - `cmd/bolt/main.go:281` — `fmt.Printf("\n[%s] %s: %s\n> ", ts,
    payload.From, payload.Body)`. Both `From` and `Body` are unfiltered
    peer input.
  - `encoding/json` *does* escape control bytes on marshal, but
    `json.Unmarshal` reconstitutes them — so when the CLI subscriber
    decodes an IPC event and prints `payload.Body`, the raw escape bytes
    are back in the string.
- Recommended fix: introduce a single `sanitizeDisplayString(s string)
  string` helper in (say) `internal/proto/sanitize.go` that:
  - Replaces every byte `< 0x20` (except `\n` and `\t` if you want to allow
    multi-line chat, or replace them too if not) with `\uFFFD` or the
    literal escaped form `\x1b` → `\\x1b`.
  - Replaces `\x7f` (DEL) the same way.
  - Truncates length (e.g. nickname ≤ 64 runes; chat body ≤ 4 KiB).
  Apply at exactly two surfaces:
  1. On *receive* in the daemon, before logging or persisting (`peer_conn.
     go::validateHandshake` sets `pc.peerNickname = sanitizeDisplayString
     (msg.Nickname)`; `chat.AttachStream` sanitises `msg.From` and
     `msg.Body` before passing into `s.emit`).
  2. In the CLI subscriber before `fmt.Printf` (defence in depth: if the
     daemon ever forgets, the CLI still scrubs).
  Do **not** sanitise on marshal — keep the wire faithful for forensic
  logging. Sanitise on display only.
- Maps to: **new story BOLT-025 (proposed)** — "Sanitise peer-controlled
  strings before printing to TTY / persisting to peers.toml. Add
  `internal/proto/sanitize.go` and call it at the two receive surfaces."
  Pair with **BOLT-001** so the surface is closed at the same time TOFU
  prompts start displaying peer fingerprints + nicknames.

#### [SEC-3] Daemon process inherits parent shell environment — credential carry-over

- Severity: High
- Vulnerability: `internal/daemon/spawn.go:43-50` spawns the daemon via
  `exec.Command(exe, "daemon", "--config-dir", configDir)` without setting
  `cmd.Env`. Go's `os/exec` defaults to `cmd.Env = os.Environ()` (i.e. full
  inherit). The bolt daemon then runs forever in the background carrying
  every secret the parent shell had loaded: `AWS_SECRET_ACCESS_KEY`,
  `GITHUB_TOKEN`, `OPENAI_API_KEY`, `SSH_AUTH_SOCK`, `KUBECONFIG`, `HOME`,
  `PWD`, …. If the daemon is later compromised (RCE through a parsing bug,
  TLS bug, supply-chain issue), the attacker reads `/proc/<pid>/environ` /
  `ps eauxw` and exfiltrates the lot.
- Attack vector: any future RCE in the daemon (e.g. a new parser bug in a
  Phase-2 file-transfer handler) yields more than just bolt data — it
  yields the spawning user's full shell credentials, which the user
  almost certainly did not intend to share with bolt.
- Impact: defence-in-depth failure. Not exploitable on its own; multiplies
  the blast radius of any future daemon vulnerability.
- Reproduction / static evidence:
  - `internal/daemon/spawn.go:43-50`:
    ```
    cmd := exec.Command(exe, "daemon", "--config-dir", configDir)
    cmd.Stdin = nil
    cmd.Stdout = nil
    cmd.Stderr = nil
    setDetached(cmd)
    ```
    No `cmd.Env = ...`.
  - On a running daemon: `ps Eww -p $(cat ~/.config/bolt/daemon.pid)`
    prints the full inherited environment.
- Recommended fix: set an explicit allow-list for the spawn's environment
  in `internal/daemon/spawn.go`:
  ```
  cmd.Env = []string{
      "PATH=" + os.Getenv("PATH"),
      "HOME=" + os.Getenv("HOME"),
      "USER=" + os.Getenv("USER"),
      "LANG=" + os.Getenv("LANG"),
      "TZ=" + os.Getenv("TZ"),
      // Optional: BOLT_LOG, BOLT_QLOG once observability lands.
  }
  ```
  Nothing in the daemon today reads beyond those. Add a unit test that
  starts the daemon under a synthetic env containing
  `SHOULD_NOT_LEAK=secret`, then `ps Eww` / `os.Environ()` from the child
  (test-only IPC command) asserts the leak variable is absent.
- Maps to: **new story BOLT-026 (proposed)** — "Restrict spawned daemon
  environment to a whitelist; document in `architecture.md` graceful-
  shutdown section." Independent of BOLT-001..023; small, ship anytime.

---

### Medium

#### [SEC-4] `config.toml` and `peers.toml` written with mode 0644 instead of 0600

- Severity: Medium
- Vulnerability: `internal/config/config.go:183` and
  `internal/config/peers.go:207` both `os.OpenFile(... O_TRUNC, 0644)`. The
  containing `~/.config/bolt/` directory is created `0700` (correct), but
  the file permissions inside it are world-readable. On systems where the
  directory bit is honoured (Linux, macOS), other local users cannot
  traverse, so this is defence-in-depth — but:
  - Backup tools that walk file modes (rsync `--perms`, `tar` default,
    Time Machine on macOS preserves modes) will preserve `0644` and the
    file then lands in a backup whose permissions may be wider than
    `~/.config/bolt`'s.
  - On a multi-user host where the dir mode is later widened by accident
    (e.g. `chmod -R a+rX` run from `~`), the file modes become the
    enforcement layer and they are world-readable.
  - `peers.toml` carries the **trust state for every known peer** — losing
    confidentiality here gives an attacker the user's social graph (which
    peers they have accepted) and their fingerprints (which are
    public-ish, but combined with `block`-listed peers reveal who the user
    has explicitly rejected).
- Attack vector: any reader with at-rest access to the home directory's
  file mode bits (backup, forensic image, accidental directory-perm widen).
- Impact: confidentiality of the user's trust graph and relay
  configuration (incl. `RelayCertFingerprint`).
- Reproduction / static evidence:
  - `internal/config/config.go:183`:
    `f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)`
  - `internal/config/peers.go:207`: same pattern.
  - The two `//nolint:gosec` annotations on those lines explain G304 (path
    derived from `DefaultConfigDir()`) but do **not** address G306 (file
    mode too permissive). The mode is the unflagged issue.
- Recommended fix: change both `0644` to `0600`. There is no consumer that
  needs the file world-readable (the daemon and CLI both run as the same
  user; both can read 0600 fine). Land alongside **BOLT-003**
  (atomic-rename) so the temp file in `writeAtomic` also gets created
  `0600` from the start. Drop the G306 nolint.
- Maps to: extend **BOLT-003** acceptance criteria with "files are created
  with mode 0600" (one extra line in the atomic-write helper).

#### [SEC-5] Post-auth resource exhaustion: 1000 streams × 8 MiB frames per peer

- Severity: Medium
- Vulnerability: `quic.Config.MaxIncomingStreams = 1000` per connection
  (`internal/transport/quic.go:20,33`), and every accepted stream is
  routed to a goroutine that calls `proto.ReadFrame`, which allocates up
  to `maxFrameSize = 8 MiB` (`internal/proto/wire.go:222`) on every read.
  After the TLS+TOFU+handshake gauntlet, a single authenticated peer can
  open 1000 streams and stall each one mid-frame (send the 4-byte length
  prefix announcing 8 MiB, then send nothing), parking 1000 goroutines and
  causing the daemon to allocate up to 8 GiB of `[]byte` scratch buffers
  (`internal/daemon/ipc.go:368` and `internal/proto/wire.go:189` both
  `make([]byte, length)` *after* reading the length, before reading the
  payload). There is no `SetReadDeadline` on the stream, no per-frame
  timeout, no in-flight allocation cap per connection, no maximum
  concurrent goroutines per peer.
- Attack vector: any peer that completes TOFU (post-BOLT-001 that means
  any explicitly-accepted peer; pre-BOLT-001, any peer at all). One bad
  authenticated peer can OOM-kill the daemon.
- Impact: daemon DoS by an authenticated peer; no isolation between peers
  — a single rogue peer takes down chat with every other peer.
- Reproduction / static evidence:
  - `internal/transport/quic.go:20` — `maxIncomingStreams = 1000`.
  - `internal/proto/wire.go:189` —
    `data := make([]byte, length)` (length is up to 8 MiB).
  - `internal/daemon/ipc.go:368` — same pattern for IPC frames (1 MiB cap
    so smaller blast radius locally, but the architecture issue is
    identical).
  - `internal/daemon/daemon.go:194-200` —
    `for { stream, ... := pc.AcceptStream(ctx); go d.routeStream(...) }`
    spawns a goroutine per stream with no per-peer cap.
  - `internal/chat/service.go:92-97` — `proto.ReadFrame` loop with no
    `stream.SetReadDeadline`.
- Recommended fix: layered defence.
  1. **Per-peer concurrent stream budget.** Track open-stream count per
     `PeerConn` (atomic int); reject (`stream.CancelRead(0x1002); stream
     .CancelWrite(0x1002)`) when the local view exceeds, say, 32 for chat
     paths and 16 + N parallel chunk streams for transfer paths.
     `MaxIncomingStreams=1000` is the QUIC-level upper bound; bolt should
     enforce a tighter app-level bound per peer.
  2. **Per-frame deadline.** Every `proto.ReadFrame` site wraps the read
     in `stream.SetReadDeadline(time.Now().Add(30 * time.Second))`. A
     peer that announces 8 MiB but never sends bytes hits the deadline
     and the goroutine returns. The chunk-stream sites already need this
     for Phase 2 (Networking review §3 / N-prereq-9).
  3. **Lazy length allocation.** Replace `data := make([]byte, length)`
     with a streaming reader that copies from the QUIC stream into a
     `sync.Pool`-backed scratch buffer of fixed size (say 64 KiB),
     decoding the JSON envelope from that. For now `ReadFrame` is JSON-
     only so the simpler fix is to lower `maxFrameSize` to 64 KiB *for
     control messages* (handshake, chat, file headers, acks) and force
     the bulk-bytes path to use the header-then-raw framing from
     **BOLT-008**. After BOLT-008, no JSON frame should ever need >
     128 KiB.
- Maps to: extend **BOLT-007** (quic.Config) acceptance criteria with
  "per-peer stream budget enforced by `transport.PeerConn`" and tie
  per-stream `SetReadDeadline` to the Networking pre-req-9 already
  mentioned. The lower-`maxFrameSize`-for-JSON change rides in
  **BOLT-008**.

#### [SEC-6] Symlink pre-creation attack on `~/.config/bolt/`

- Severity: Medium
- Vulnerability: every write in the config / identity / token / pid paths
  uses `os.WriteFile` (or `os.OpenFile(... O_CREATE ...)` without
  `O_NOFOLLOW`). Go's `os.WriteFile` follows symlinks. An attacker who
  can create files inside `~/.config/bolt/` before `bolt init` runs (e.g.
  an unprivileged shell session that runs before the user's first `bolt
  init` on a freshly-imaged machine; a malicious dotfile-management
  script; post-exploitation of an unrelated low-privilege bug) can plant
  symlinks named `private.key`, `daemon.pid`, `ipc.token`, `config.toml`,
  `peers.toml` pointing at attacker-chosen destinations the user has
  write access to. On the next `bolt init`:
  - `private.key` symlinked at `~/.ssh/id_ed25519` → user's SSH key is
    overwritten with a bolt PEM block.
  - `daemon.pid` symlinked at `~/.config/systemd/user/bolt.pid` → user's
    systemd unit liveness file is overwritten with a stale PID.
  - `ipc.token` symlinked at any world-writable location → token leaks.
  Also: `os.MkdirAll(dir, 0700)` does NOT change permissions on a dir
  that already exists with different mode. If `~/.config/bolt/` exists
  with 0755 (e.g. created by a hostile parent shell), `MkdirAll` returns
  success and the bolt daemon happily runs with a world-traversable
  config dir.
- Attack vector: pre-positioned attacker on the same host (low-priv
  user, exploit chain). Requires write to `~/.config/`. Not exploitable
  by a network attacker.
- Impact: overwrite of arbitrary files the user can write; possible
  identity-key destruction; possible token leak.
- Reproduction / static evidence:
  - `internal/identity/identity.go:206,220` —
    `os.WriteFile(path, data, 0600)` and `... 0644)` without
    `O_NOFOLLOW`.
  - `internal/daemon/ipc_token.go:38` — `os.WriteFile(path, ..., 0600)`.
  - `internal/daemon/spawn.go:55` — `os.WriteFile(pidPath, ..., 0600)`.
  - `internal/config/config.go:179` and `peers.go:203` —
    `os.MkdirAll(dir, 0700)` (creates with 0700 only if it didn't
    exist; otherwise no-op on permissions).
  - Manual test: `mkdir -p ~/.config/bolt && chmod 755 ~/.config/bolt &&
    ln -s /tmp/pwn ~/.config/bolt/private.key && rm -rf
    ~/.config/bolt/identity` and run `bolt init` — `private.key` is
    actually written to `/tmp/pwn`. (Don't run on a machine with a real
    bolt identity.)
- Recommended fix:
  1. Refuse to start if `~/.config/bolt/` exists with mode wider than
     `0700`. Call `os.Stat` after `MkdirAll`, compare `info.Mode().Perm()
     & 0o077 != 0` → error.
  2. Use `os.OpenFile(path, O_WRONLY|O_CREATE|O_EXCL|O_NOFOLLOW, 0600)`
     for first-write of `private.key`, `ipc.token`. `O_EXCL` refuses to
     follow into an existing file (symlink or not); `O_NOFOLLOW` refuses
     to write through a symlink. On macOS+Linux both flags are honoured.
  3. For `peers.toml` / `config.toml`, the `writeAtomic` from BOLT-003
     writes to `*.tmp` in the same directory, then `os.Rename`. Have the
     temp-file create use `O_EXCL|O_NOFOLLOW` as well — symlink-followed
     temp file is just as bad.
  4. Document in `architecture.md` that the config dir must be `0700`,
     and tighten `MkdirAll` with a post-check.
- Maps to: **new story BOLT-027 (proposed)** — "Refuse insecure config dir
  permissions; use `O_NOFOLLOW`+`O_EXCL` for identity / token / config
  writes." Pair with **BOLT-003** since the atomic-write helper is the
  natural home for the `O_NOFOLLOW` flag.

#### [SEC-7] Nickname-confusion / chat-routing ambiguity

- Severity: Medium
- Vulnerability: peer nicknames are **peer-controlled, unverified, and
  not unique**. `PeerStore` and `PeerRegistry` both key by fingerprint
  (correct), but the user-facing UX (`bolt chat <name>`, `bolt connect`,
  `bolt status`) resolves by nickname-or-fingerprint-prefix via
  `transport.PeerRegistry.ResolvePeer`
  (`internal/transport/peer_conn.go:241-261`). The `GetByNickname` path
  takes the **first** matching peer; ties are broken implicitly. A
  malicious peer can claim the same nickname as a peer the user already
  trusts (`"alice"`, `"my-laptop"`), and after the user `bolt connect`s
  the bad peer, `bolt chat alice` may route to the wrong fingerprint
  depending on `range` ordering of the registry map (Go map iteration is
  randomised → non-deterministic).
- Attack vector: any peer (post-BOLT-001 they need to be at least
  prompted; the user just clicks `y` if the nickname looks familiar —
  which is exactly the bait).
- Impact: chat / file transfer routed to the wrong peer. Combined with
  [SEC-2] (escape injection), a malicious peer can impersonate a trusted
  contact persuasively in the scrollback (forge `\r[14:32:01] alice: ...`).
- Reproduction / static evidence:
  - `internal/transport/peer_conn.go:228-238` — `GetByNickname` returns
    "a connected peer with an exact nickname match", first hit in map
    iteration.
  - `internal/transport/peer_conn.go:241-261` — `ResolvePeer` calls
    `GetByNickname` first, only falls back to prefix-match if no nickname
    hit.
  - `internal/daemon/connect.go:90` —
    `record.Nickname = pc.PeerNickname()` (peer-controlled, unverified).
- Recommended fix: in the TOFU prompt path (BOLT-001), surface the
  fingerprint **with** the nickname; if a peer claims a nickname that
  matches an already-trusted peer's nickname but a different fingerprint,
  show a "name collision" warning in the prompt. `ResolvePeer`: when
  multiple registry entries share the same nickname, return
  `"ambiguous: 2 peers named 'alice'"` instead of silently picking one.
  This is one extra branch in `ResolvePeer` and one extra message in the
  TOFU prompt.
- Maps to: extend **BOLT-001** (TOFU prompt) acceptance criteria with
  "warn the user if the new peer's nickname collides with an existing
  trusted peer's nickname", and add a unit test for `ResolvePeer` returning
  "ambiguous" on nickname tie.

---

### Low

#### [SEC-8] JSON decoder accepts unknown fields silently

- Severity: Low
- Vulnerability: `proto.ReadFrame` (`internal/proto/wire.go:194`) and the
  IPC frame reader (`internal/daemon/ipc.go:372`) both use `json.Unmarshal`
  with default decoder settings. Unknown fields are silently ignored. This
  is the JSON-decoder equivalent of `0644` defence-in-depth — not a CVE,
  but a hostile or buggy peer can stuff additional fields in `HandshakeMsg`,
  `ChatMsg`, `ConnectPayload` etc. that are silently dropped. This breaks
  fail-fast diagnostics (a peer running an unknown protocol extension does
  not get rejected — its extra fields are eaten and the connection looks
  fine), and during Phase 2 / Phase 4 it can mask a wire-version drift
  bug in production for a long time.
- Attack vector: peer sending extra fields. Not directly exploitable.
- Impact: silent acceptance of mis-shaped frames; reduced ability to
  detect protocol drift.
- Reproduction / static evidence:
  - `internal/proto/wire.go:194` — `json.Unmarshal(data, dst)`.
  - `internal/daemon/ipc.go:372` — `json.Unmarshal(data, dst)`.
  - No `decoder.DisallowUnknownFields()` call anywhere in the tree.
- Recommended fix: replace both unmarshal sites with a
  `json.NewDecoder(bytes.NewReader(data))` + `dec.DisallowUnknownFields()`
  + `dec.Decode(dst)`. For the IPC frame, this is purely defensive (we
  control both ends). For the wire frame, this catches peer-introduced
  drift at the receiver.
- Maps to: extend **BOLT-008** (wire schema freeze) acceptance criteria
  with "decoder uses `DisallowUnknownFields()` on both wire and IPC
  framing".

#### [SEC-9] `os.Executable()` swap during daemon respawn (TOCTOU)

- Severity: Low
- Vulnerability: `internal/daemon/spawn.go:38` uses `os.Executable()` to
  re-exec the daemon. On macOS this returns the resolved path; on Linux
  it returns the resolution of `/proc/self/exe`. If an attacker can
  replace the bolt binary on disk between `bolt init` and the spawn (TOCTOU
  window: a few milliseconds), the spawned daemon is the attacker's
  binary. Requires write access to `$PATH/bolt` (which means the attacker
  already pwned the install — moot in most scenarios), but worth
  documenting.
- Attack vector: local attacker with write to the bolt install location.
- Impact: arbitrary code execution as the user (but the attacker already
  had write access to the binary, so this is mostly noise).
- Reproduction / static evidence:
  - `internal/daemon/spawn.go:38` — `exe, err := os.Executable()`.
  - `internal/daemon/spawn.go:43` — `exec.Command(exe, ...)`.
- Recommended fix: not actionable in v1. Document in `SECURITY.md`
  (which does not exist yet — see also Phase 5 pre-work below) that
  protecting the install location is the user's responsibility (`chmod
  -w` the binary, install via Homebrew which writes to `/opt/homebrew/bin/`
  owned by `root` on macOS, or by `dpkg` to `/usr/bin/` on Linux). No
  code change.
- Maps to: documentation only — note in the future `SECURITY.md` /
  `docs/threat-model.md`. **No new BOLT-XXX story**.

#### [SEC-10] Hostname leaks via default `Nickname` to every connected peer

- Severity: Low
- Vulnerability: `internal/config/config.go:134-138, 150-152` defaults
  `Nickname` to `os.Hostname()`. Every peer connection then sends this in
  `HandshakeMsg.Nickname`. Many users' hostnames contain identifying
  information (`savitha-macbook-pro`, `acme-corp-laptop-12345`,
  `<dept>-<region>-<host>`). On a public LAN (cafe, conference, hotel
  Wi-Fi) every peer that completes a TLS handshake with the daemon
  receives the hostname — which can be enough to fingerprint the user.
- Attack vector: any peer the user connects to or that completes the
  TLS handshake to the daemon (post-BOLT-001: any peer the user has
  accepted via TOFU; pre-BOLT-001: any peer at all).
- Impact: minor de-anonymisation; identifying-information leak in
  multi-org Wi-Fi networks.
- Reproduction / static evidence:
  - `internal/config/config.go:134` — `hostname, err := os.Hostname()`.
  - `internal/config/config.go:74-87` — `defaults(hostname).Nickname =
    hostname`.
  - `internal/transport/peer_conn.go:100-104` — handshake msg carries
    `Nickname: local.Nickname`.
- Recommended fix: default `Nickname` to a generic value such as
  `bolt-<short-fingerprint-6-chars>` instead of the system hostname. The
  user can override via `~/.config/bolt/config.toml` if they want their
  hostname shown. Also: document in `bolt init` first-run output that
  "your nickname is visible to every peer; edit `config.toml` to change
  it".
- Maps to: small docs + one-line config default change. **Suggest folding
  into BOLT-022 (README + QUICKSTART after Windows decision)** or a
  trivial follow-up story.

---

## GitHub issue bodies (ready-to-paste)

### Issue 1 — `[SECURITY] Reachable Go stdlib vulnerabilities — toolchain pinned at 1.25.1`

```
## Severity
Critical

## Vulnerability
The bolt module pins `go 1.25.1` in `go.mod` and the local toolchain is
1.25.1. `govulncheck ./...` reports 15 reachable Go stdlib advisories
across crypto/tls, crypto/x509, encoding/asn1, encoding/pem, net/url,
and net. Highest-impact reachable items:
- GO-2026-4870 — unauthenticated TLS 1.3 KeyUpdate causes persistent
  connection retention / DoS (crypto/tls, fixed in 1.25.9). Reachable
  via every QUIC handshake.
- GO-2026-4340 — handshake messages processed at the wrong encryption
  level (TLS state confusion; fixed in 1.25.6).
- GO-2026-4337 — unexpected session resumption (fixed in 1.25.7).
- GO-2025-4011 — DER parsing memory exhaustion in encoding/asn1 (fixed
  in 1.25.2). Reachable via identity.TLSCertificate → tls.X509KeyPair.
- GO-2025-4009 — quadratic PEM parsing (fixed in 1.25.2). Reachable via
  tls.X509KeyPair.
- GO-2025-4008 — ALPN error string contains attacker-controlled bytes
  (fixed in 1.25.2).

Full govulncheck symbol output saved at the time of the security review;
see `.agent/reports/security-review.md` §Tooling output.

## Attack Vector
A network attacker who can reach UDP/7799 on a daemon (or a malicious
peer the daemon dials) can trigger TLS-stack behaviours that hold or
kill connections. The KeyUpdate one specifically holds a TLS connection
alive after the peer is gone, leaking goroutines and file descriptors.

## Impact
DoS of the daemon. None of the reachable advisories are RCE on their
own, but several enable amplification of other bugs (state confusion,
session resumption).

## Reproduction
1. `go install golang.org/x/vuln/cmd/govulncheck@latest`
2. `govulncheck ./...` — exits 3, reports 15 stdlib advisories under
   `=== Symbol Results ===`.
3. Each finding includes a call-trace line from bolt code into the
   vulnerable stdlib function.

## Fix
1. Update `go.mod`: change `go 1.25.1` → `go 1.25.10` (or the current
   latest patch on the 1.25.x line).
2. Add `toolchain go1.25.10` so contributors with older Go installs
   fail loudly instead of silently building against an unpatched
   stdlib.
3. CI already pins via `go-version-file: go.mod`; the bump propagates
   automatically.
4. Re-run `govulncheck ./...` after the bump and verify 0 reachable
   advisories.

## Suggested labels
security-critical
```

### Issue 2 — `[SECURITY] Terminal escape injection via peer-controlled Nickname and ChatMsg.Body`

```
## Severity
High

## Vulnerability
Peer-controlled strings (HandshakeMsg.Nickname, ChatMsg.From,
ChatMsg.Body) are printed to real terminals — daemon stderr and chat
CLI stdout — via fmt.Fprintf / fmt.Printf with no filtering of ASCII
control bytes or ANSI/CSI/OSC escape sequences. A malicious peer can:
- Send a nickname `\x1b]2;evil\x07` (OSC-2 set-title) — changes the
  user's terminal title without notice.
- Send `\x1b[2J\x1b[H` — clears the user's chat scrollback.
- Send `\r` in chat body — overwrites the previous chat line, forging
  prior messages.
- Send `\x1b[8m`-wrapped hidden text — sneaks invisible content into
  chat scrollback that copies/pastes later.
- On macOS Terminal.app, `\x1b]52;c;...\x07` (OSC-52) sets the system
  clipboard.

## Attack Vector
Any authenticated peer (which today is automatic — see BOLT-001).
Post-BOLT-001, this becomes "any peer the user has accepted at least
once via TOFU". The attacker does not need to be on the same LAN if
relay routing is in use.

## Impact
Clipboard hijack, scrollback forgery / phishing (impersonating another
trusted peer in chat scrollback), terminal title spoofing, screen
clear. Combined with [SEC-7] nickname confusion, a malicious peer can
impersonate a trusted contact in the chat scrollback persuasively.

## Reproduction
Static (no transfer code exists yet, so reproduce via chat):
1. Two peers A, B. A is malicious.
2. A connects and completes handshake (auto-accept until BOLT-001).
3. A's daemon has `nickname = "alice\x1b[2J\x1b[H"` in config.toml
   (just edit it).
4. On B, run `bolt chat alice` and watch the terminal clear when the
   first message arrives.

File:line evidence:
- `internal/transport/peer_conn.go:137` — `pc.peerNickname =
  msg.Nickname` with no sanitisation.
- `internal/daemon/daemon.go:188,190,210` —
  `fmt.Fprintf(os.Stderr, "peer connected: %s ...", pc.PeerNickname())`
  to a real TTY.
- `cmd/bolt/main.go:281` — `fmt.Printf("\n[%s] %s: %s\n> ", ts,
  payload.From, payload.Body)` — both From and Body are raw peer
  input.

## Fix
Introduce `internal/proto/sanitize.go` with a single helper:
```go
func SanitizeDisplay(s string, max int) string {
    // Replace bytes < 0x20 (except \n if multiline allowed) and 0x7f
    // with the literal escaped form \x1b -> \\x1b. Truncate to max
    // runes.
}
```
Apply at two surfaces:
1. On receive in the daemon (peer_conn.go::validateHandshake;
   chat.AttachStream before emit; before any logging).
2. In the CLI subscriber before fmt.Printf (defence in depth).

Do not sanitise on marshal — keep the wire faithful for forensic
logs. Sanitise on display only.

## Suggested labels
security-high
```

### Issue 3 — `[SECURITY] Spawned daemon inherits parent shell environment`

```
## Severity
High

## Vulnerability
`internal/daemon/spawn.go:43-50` spawns the daemon via `exec.Command`
without setting `cmd.Env`. Go's os/exec defaults to `cmd.Env =
os.Environ()` — full inherit. The bolt daemon then runs in the
background carrying every secret loaded in the parent shell:
AWS_SECRET_ACCESS_KEY, GITHUB_TOKEN, OPENAI_API_KEY, SSH_AUTH_SOCK,
KUBECONFIG, …. The daemon does not use these env vars — but if it is
ever compromised (parser bug, TLS bug, supply-chain), the attacker
reads /proc/<pid>/environ or `ps eauxw` and exfiltrates them.

## Attack Vector
Any future RCE in the daemon multiplies its blast radius: not just
bolt data, but the spawning user's full shell credentials.

## Impact
Defence-in-depth failure. Not exploitable on its own; multiplies the
impact of any future daemon vulnerability.

## Reproduction
1. Start daemon: `bolt init` (or `bolt daemon`).
2. `ps Eww -p $(cat ~/.config/bolt/daemon.pid)` — prints the full
   inherited environment.
3. On Linux: `cat /proc/$(cat ~/.config/bolt/daemon.pid)/environ |
   tr '\0' '\n'` — same.

File:line evidence:
- `internal/daemon/spawn.go:43-50` — no `cmd.Env = ...`.

## Fix
Set an explicit env allow-list in `internal/daemon/spawn.go`:
```go
cmd.Env = []string{
    "PATH=" + os.Getenv("PATH"),
    "HOME=" + os.Getenv("HOME"),
    "USER=" + os.Getenv("USER"),
    "LANG=" + os.Getenv("LANG"),
    "TZ=" + os.Getenv("TZ"),
    // Optional once observability lands: BOLT_LOG, BOLT_QLOG.
}
```
Nothing in the daemon today reads beyond those. Add a unit test that
runs `bolt daemon` under a synthetic env containing
`SHOULD_NOT_LEAK=secret` and asserts the leak variable is absent from
the daemon's environment (read via a test-only IPC introspection
command, or via `/proc/<pid>/environ` on Linux).

## Suggested labels
security-high
```

---

## Phase 5 (relay) security pre-work

Items the team must agree on **before** the `boltd` relay binary is
written. Each is a 1-page design decision, not a backlog story yet.

1. **HMAC token derivation — HKDF + per-epoch rotation.** Networking review
   §10 already raised the single-`relay_secret` blast-radius issue. The
   security position: do not derive tokens with a flat
   `HMAC-SHA256(relay_secret, fingerprint)`. Use
   ```
   token = HKDF-SHA256(
       relay_secret,
       salt=relay_id || epoch_id,
       info="bolt-relay-token-v1" || peer_fingerprint,
       len=32 bytes,
   )
   ```
   `epoch_id` rotates monthly; relay accepts current + previous epoch
   tokens during a 24 h overlap. Decision needed: who owns the
   `relay_secret` rotation (operator? automation? config in the relay's
   `boltd.toml`?), and how peers learn about the new epoch (pull from
   a public `/well-known/epoch` endpoint? push via heartbeat response?).

2. **Constant-time token verification on the relay.** The relay's HTTPS
   handlers must compare incoming tokens with `subtle.ConstantTimeCompare`.
   This is the same pre-work as the in-tree IPC token (Q-BUG-13) — both
   use timing-leakable string `==` today.

3. **CIDR-aware rate limiting.** Plan §"relay" says "per-IP 10 req/s
   burst 100". On CGNAT (Jio, Airtel in India, mobile carriers
   generally), a single CIDR /32 represents thousands of subscribers.
   Per-IP rate limit is insufficient. Pre-work decision: rate-limit
   key = `{ remote_ip, peer_fingerprint }` rather than `remote_ip` alone,
   with a separate global cap on `peer_fingerprint`-less endpoints
   (`/lookup` before authentication). Document in the relay deploy
   guide.

4. **TURN credential lifetime + amplification cap.** RFC 8656 default
   allocation is 600 s, refreshed indefinitely. For bolt:
   - Allocation TTL: 600 s, hard cap of 30 min total (= 3 refreshes),
     forces re-auth.
   - Per-allocation bandwidth quota (configured per-peer or per-CIDR
     via coturn `--max-bps`).
   - Reject `/punch` requests for unknown fingerprints (don't act as a
     reflector for arbitrary internet packets).
   - Document in the relay deploy guide that `--no-loopback-peers` and
     `--no-multicast-peers` are mandatory.

5. **Relay binary scope.** Decide before code is written: is the relay
   binary `boltd` allowed to read `peers.toml`-style data, or is it a
   pure forwarding service with no peer-trust state? Security position:
   pure forwarding (the relay never has trust-relevant data; it only
   stores `{fingerprint, last-seen-ip, expires_at}` rows). This bounds
   the blast radius of relay compromise: an attacker with relay code
   exec can MITM ICE coordination *but* cannot impersonate any peer to
   any other peer, because the trust ground-truth is the peer's
   Ed25519 key + the BOLT-002 PoP signature.

6. **`SECURITY.md` + threat model document.** Today there is no
   `SECURITY.md`, no `docs/threat-model.md`. Before Phase 5 — when the
   surface area expands to a public TURN/relay — bolt needs to publish
   a threat model that names:
   - The actor model (LAN attacker, CGNAT peer, relay operator,
     compromised binary, supply-chain attacker).
   - What bolt does and does not defend against (e.g. it does not
     defend against a malicious user running `bolt` on the user's own
     machine, because that user *is* the trust root).
   - A coordinated-disclosure email and PGP key (or
     `https://github.com/g-savitha/bolt/security/advisories/new` flow).
   This is also a pre-condition for adding bolt to OSSF's `scorecard`
   "Security-Policy" check (the workflow already runs the scan; today
   it scores 0/10 on that check because the file is missing).

7. **Symmetric-NAT detection signal.** Networking review notes that ICE
   gathers candidates and falls back to TURN. The security position
   here: the relay must not be able to *coerce* a fallback by selectively
   dropping STUN responses (relay-induced fallback = relay reads more
   of the user's traffic). Decision: the relay does not see STUN
   exchanges (STUN is direct peer↔STUN-server, separate from
   `/register` / `/heartbeat`). Pre-work: document this in the relay
   design before anyone wires `/punch` into the STUN flow.

8. **Pinned TLS to relay (`RelayCertFingerprint`).** `Config.
   RelayCertFingerprint` already exists (`internal/config/config.go:61`)
   but is unused. Pre-work: in the relay client (when it lands), the
   TLS dial must verify the relay's cert fingerprint against this
   value; mismatch → abort with "relay cert changed — possible MITM
   or relay rotation, contact relay operator". This is the relay
   equivalent of TOFU.

9. **Logging hygiene for the relay.** The relay should not log
   `{peer_fingerprint, peer_ip}` pairs in plain text indefinitely.
   Decision: log only `{hash(fingerprint || daily_salt), hash(ip ||
   daily_salt)}`. Daily-salt rotation prevents long-term linkability
   while preserving short-term operational debuggability.

---

## Final user-visible summary

**Counts (new findings only):** Critical 1, High 2, Medium 4, Low 3.

**Top 3 NEW critical/high one-liners:**

1. **[SEC-1, Critical]** `go.mod` pins `go 1.25.1`; `govulncheck` reports
   15 reachable stdlib advisories incl. TLS 1.3 KeyUpdate DoS
   (`internal/transport/quic.go:56`) — bump toolchain to 1.25.10+.
2. **[SEC-2, High]** Peer-controlled `Nickname` and `ChatMsg.Body` are
   printed raw to TTY (`cmd/bolt/main.go:281`,
   `internal/daemon/daemon.go:188`) — terminal escape / clipboard
   hijack / scrollback forgery.
3. **[SEC-3, High]** Spawned daemon inherits parent shell env
   (`internal/daemon/spawn.go:43-50`, no `cmd.Env = ...`) — credentials
   carry into the long-lived daemon process.

**Tooling status:**
- `govulncheck ./...` → exits 3, **15 reachable stdlib advisories** (all
  fixed by Go 1.25.10).
- `go mod verify` → **all modules verified**.

**Report path:** `.agent/reports/security-review.md`.
