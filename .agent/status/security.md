# Security — status

**Date:** 2026-05-19
**Owner:** Security Engineer
**Status:** Review complete. Awaiting Savvy sign-off on new BOLT story
creation and a toolchain bump.

## Summary numbers

- New findings (this review): **Critical 1, High 2, Medium 4, Low 3**.
- Findings already covered by parallel reports and **not** re-raised: 12
  (mapped to BOLT-001..BOLT-008 — see §Coverage cross-check in the
  report).
- Tooling pass:
  - `govulncheck ./...` → exits 3, **15 reachable stdlib advisories**
    (all fixed by Go 1.25.10).
  - `go mod verify` → all modules verified.
  - `go list -m -u all` → 9 dependencies have an available upgrade; none
    flagged with an active CVE in the local vuln DB. `quic-go v0.59.1`,
    `BurntSushi/toml v1.6.0`, `cobra v1.10.2`, `crypto v0.51.0`,
    `gofrs/flock v0.12.1`, `x/sys v0.44.0` are current or close enough.
  - `go.mod` has no `replace` directives.
  - `golangci-lint run` already passes at 0 issues (per backend-review);
    no additional sec-only finding from this surface.

## Top 3 NEW critical/high

1. **[SEC-1, Critical]** `go.mod` pins `go 1.25.1`; `govulncheck` finds
   15 reachable stdlib advisories including a TLS 1.3 KeyUpdate DoS
   (GO-2026-4870, fixed in 1.25.9) and an ASN.1 DER memory exhaustion
   (GO-2025-4011, fixed in 1.25.2) reachable via every QUIC handshake
   (`internal/transport/quic.go:56`) and every cert load
   (`internal/identity/identity.go:174`). Fix: bump `go` directive to
   1.25.10 and add a `toolchain` line.
   → Proposed story **BOLT-024**.
2. **[SEC-2, High]** Peer-controlled `HandshakeMsg.Nickname`,
   `ChatMsg.From`, `ChatMsg.Body` are printed raw to TTY via
   `fmt.Fprintf` / `fmt.Printf`
   (`internal/daemon/daemon.go:188,190,210` and
   `cmd/bolt/main.go:281`). A malicious peer can hijack the terminal
   title, clear the scrollback, forge prior messages via `\r`
   overwrites, or set the system clipboard via OSC-52 on macOS
   Terminal.app. Fix: `internal/proto/sanitize.go` helper applied on
   the receive side (daemon) and again on the display side (CLI).
   → Proposed story **BOLT-025**. Pair with BOLT-001 so the TOFU prompt
   itself is also safe to render.
3. **[SEC-3, High]** Spawned daemon inherits the parent shell's full
   environment because `internal/daemon/spawn.go:43-50` does not set
   `cmd.Env`. Every `AWS_*`, `GITHUB_TOKEN`, `OPENAI_API_KEY`, etc. is
   carried into the long-lived background daemon and visible via `ps
   Eww` / `/proc/<pid>/environ`. Fix: explicit env allow-list
   (`PATH`, `HOME`, `USER`, `LANG`, `TZ`, plus `BOLT_*` once
   observability lands).
   → Proposed story **BOLT-026**.

## Medium / Low — for tracking

- **[SEC-4, Medium]** `config.toml` / `peers.toml` written `0644` — fold
  into BOLT-003 (atomic-write helper creates `0600` files).
- **[SEC-5, Medium]** Post-auth resource exhaustion: 1000 streams ×
  8 MiB frames per peer, no `SetReadDeadline`, no per-peer in-flight
  cap — extend BOLT-007 (quic.Config) + BOLT-008 (wire schema
  freeze; lower JSON `maxFrameSize` to 128 KiB once chunks move to
  header-then-raw framing).
- **[SEC-6, Medium]** Symlink pre-creation attack on `~/.config/bolt/`
  — proposed BOLT-027: use `O_NOFOLLOW|O_EXCL` for first-write of
  identity/token/pid, refuse to start if dir mode is wider than 0700.
- **[SEC-7, Medium]** Nickname-confusion in `ResolvePeer` —
  fold into BOLT-001 (TOFU prompt warns on nickname collision;
  `ResolvePeer` returns "ambiguous").
- **[SEC-8, Low]** JSON decoder accepts unknown fields silently —
  fold into BOLT-008 (`dec.DisallowUnknownFields()` on wire and IPC).
- **[SEC-9, Low]** `os.Executable()` TOCTOU on daemon respawn —
  documentation only (`SECURITY.md` once it exists).
- **[SEC-10, Low]** Default `Nickname` = `os.Hostname()` leaks
  identifying info — one-line config-default tweak, fold into BOLT-022.

## Phase 5 (relay) security pre-work — flagged for design lock

Listed in full in the report under §Phase 5 pre-work. Headlines:

- HKDF-derived per-epoch relay tokens (replaces flat
  `HMAC(relay_secret, fingerprint)`); 24 h rotation overlap window.
- Constant-time token comparison on the relay (same primitive as
  Q-BUG-13's in-tree IPC token fix).
- CIDR-aware rate limiting (per-IP is inadequate under CGNAT).
- TURN allocation TTL hard cap + per-allocation bandwidth quota +
  reject `/punch` for unknown fingerprints (no relay reflector).
- Relay binary is a **pure forwarder** — no peer-trust state stored
  on the relay, by policy.
- `SECURITY.md` + threat model document published before relay code
  lands (also closes OSSF Scorecard "Security-Policy" check).
- Symmetric-NAT fallback is direct STUN — relay must not be able to
  coerce fallback.
- Pin relay TLS cert via existing `Config.RelayCertFingerprint`.
- Relay logs `hash(fingerprint || daily_salt)` only, not raw
  fingerprints.

## What I need from Savvy

1. **Sign-off to bump the Go toolchain** to `go 1.25.10` (one-line
   change in `go.mod`, plus a `toolchain` directive). This single change
   closes 15 of the 15 reachable stdlib advisories. No code churn.
2. **Approve creation of BOLT-024 (toolchain bump), BOLT-025 (terminal
   escape sanitizer), BOLT-026 (daemon env allow-list), BOLT-027
   (symlink-hardened identity/token writes).** PO can fold the rest
   into existing BOLT-001 / BOLT-003 / BOLT-007 / BOLT-008 / BOLT-022
   as outlined.
3. **Decide whether to file `[SECURITY]` issues** to GitHub before
   merging the fixes (DevOps is fixing the `gh` auth — once that's in,
   the ready-to-paste bodies in the report can be filed). Standard
   practice for a project this size is to open security-impact issues
   privately via GitHub Security Advisories rather than public issues;
   recommend the latter path for SEC-1..SEC-3.
4. **Decide on the Phase 5 relay security pre-work review.** I would
   like a 30-min review with networking + architect to lock the 9 items
   in §Phase 5 pre-work before any code in `boltd/` is written.

## Next on my plate

- Stand by for BOLT-024..BOLT-027 story creation + assignment.
- Open `docs/threat-model.md` skeleton (separate from the relay
  pre-work) once Savvy says go.
- Re-run `govulncheck` after the toolchain bump and confirm 0
  reachable advisories.

## Files written this turn

- `.agent/reports/security-review.md`
- `.agent/status/security.md` (this file)

No source files modified. No GitHub issues filed (`gh` auth is broken;
DevOps owns the fix per the task instructions).
