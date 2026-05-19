# QA Status — 2026-05-19

**Status:** Phase 1 verification complete — Phase 2 GO
**Main SHA:** `7db38e4`
**Verifier:** Senior QA Engineer

## Test suite (main @ 7db38e4)

| Check | Result |
|---|---|
| `go build ./...` | PASS |
| `go test ./...` | PASS — config, daemon, identity, proto |
| `go test -race ./...` | PASS — race detector clean |
| `govulncheck ./...` | PASS |

## Issue verification matrix (full history)

| Issue # | ID | Title | Result | Closed by |
|---:|---|---|---|---|
| #33 | BOLT-024 | Go toolchain 1.25.10 | **Closed** ✅ | PR #37 |
| #9 | BOLT-003 | Atomic writes peers.toml/config.toml | **Closed** ✅ | PR #38 |
| #25 | BUG-3 | Non-atomic writes (duplicate of BOLT-003) | **Closed** ✅ | PR #38 |
| #10 | BOLT-004 | Daemon shutdown closes IPC subscribers | **Closed** ✅ | PR #39 |
| #24 | BUG-2 | Daemon shutdown hang | **Closed** ✅ | PR #39 |
| #11 | BOLT-005 | daemon.pid symmetric write/remove | **Closed** ✅ | PR #40 |
| #34 | BOLT-025 | Sanitize peer strings before TTY | **Closed** ✅ | PR #41 |
| #27 | BUG-5/BUG-11 | TTY escape injection | **Closed** ✅ | PR #41 |
| #35 | BOLT-026 | Daemon env allowlist | **Closed** ✅ | PR #42 |
| #28 | BUG-6 | Unix socket chmod 0600 | **Closed** ✅ | PR #43 — verified `ipc_endpoint_unix.go:36` |
| #29 | BUG-7 | listenIPC dial-before-remove guard | **Closed** ✅ | PR #44 — verified `ipc_endpoint_unix.go:21-27` |
| #30 | BUG-8 | Chat stream closed on version mismatch | **Closed** ✅ | PR #45 — verified `service.go:98-103` returns (closes via defer) |
| #32 | BUG-10 | PublishChat lock scope | **Closed** ✅ | PR #46 — copies subs under lock, writes without lock |
| #31 | BUG-9 | Stream-handler registry | **Closed** ✅ | PR #47 — `stream_handler.go`, `ErrCodeUnknownStreamType` |
| #12 | BOLT-006 | Stream-handler registry (issue) | **Closed** ✅ | Manually closed 2026-05-19; loopback integration tests tracked under #16 |

## Still open — Phase 2 required

| Issue # | ID | Title | Blocked on | Action |
|---:|---|---|---|---|
| #23 | BUG-1 | TOFU peerVerifier stub (`connect.go:81` returns `true,nil`) | BOLT-001 (#7) + BOLT-010 (#16) | Re-verify when BOLT-001 lands |
| #26 | BUG-4 | ChunkMsg.Data base64 JSON blows frame cap | BOLT-008 (#14) | Re-verify when BOLT-008 lands |
| #36 | BOLT-027 | Insecure config-dir permissions; O_NOFOLLOW on first writes | — (P2, not blocking Phase 2) | Pick up after Wave 2 |

## Phase 2 GO / NO-GO

| Gate | Status |
|---|---|
| All Phase-1 P0 bugs fixed | ✅ (BUG-1 and BUG-4 intentionally deferred to Phase 2 stories) |
| `go test -race ./...` clean | ✅ |
| No new regressions from Wave 1 / Tier A PRs | ✅ |
| Wave 1 security fixes verified | ✅ (BOLT-025, BOLT-026) |
| Stream registry (BOLT-006) implemented | ✅ |

**PHASE 2: GO** — Backend is clear to start BOLT-010 (loopback harness).
