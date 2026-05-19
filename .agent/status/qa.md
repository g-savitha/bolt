# QA Status — 2026-05-19

**Status:** Verification run (Wave 1 post-merge re-verification)
**Main SHA:** `944f1c6` (`944f1c626f0177a7d13023e37dd07149d61978db`)
**Verifier:** Senior QA Engineer

## Test suite (main @ 944f1c6)

| Check | Result |
|---|---|
| `go build ./...` | PASS (after `go mod tidy` for flock v0.13.0) |
| `go test ./... -v` | PASS — 25 tests across config, daemon, identity, proto |
| `go test -race ./...` | PASS |
| `govulncheck ./...` | PASS — No vulnerabilities found |

## Issue verification matrix

| Issue # | Title | Result | Evidence |
|---:|---|---|---|
| #25 | [BUG-3] Non-atomic peers.toml/config.toml writes | **Closed** | `writeAtomic` in `internal/config/atomic.go`; `go test ./internal/config/... -race -v` all PASS |
| #24 | [BUG-2] Daemon shutdown hang with subscriber | **Closed** | `IPCServer.Close()` closes subs before `wg.Wait()`; `TestIPCServerCloseWithActiveSubscriber` PASS |
| #27 | [BUG-5] daemon.pid false-positive liveness | **Closed** | PID written in `Daemon.Run` after listener, removed on defer; `TestDaemonPIDWriteAndRemove` PASS |
| #9 | [BOLT-003] Atomic-rename writer | **Already closed** | Merged via #38; same evidence as #25 |
| #10 | [BOLT-004] IPC subscriber shutdown | **Already closed** | Merged via #39; same evidence as #24 |
| #11 | [BOLT-005] daemon.pid symmetric write/remove | **Already closed** | Merged via #40; same evidence as #27 |
| #33 | [BOLT-024] Go 1.25.10 toolchain bump | **Already closed** | `go 1.25.10` in go.mod; `govulncheck ./...` → 0 reachable vulns |
| #34 | [BOLT-025] Sanitize peer strings before TTY | **Already closed** | `proto.SanitizeDisplay` + 4 unit tests PASS; used in peer_conn, chat, daemon, main |
| #35 | [BOLT-026] Daemon env allowlist | **Already closed** | `daemonEnv()` allowlist in spawn.go; `TestDaemonEnvAllowlistExcludesSecrets` PASS |
| #23 | [BUG-1] TOFU stub — silent accept | **Still open** | `peerVerifier()` returns `(true,nil)` at `connect.go:75-83`; BOLT-001 (#7) not on main |
| #26 | [BUG-4] ChunkMsg base64 frame cap | **Still open** | `ChunkMsg.Data []byte` still JSON-encoded; BOLT-008 (#14) not on main |
| #28 | [BUG-6] Unix socket inherits umask | **Still open** | No `os.Chmod(0600)` after `net.Listen` in `ipc_endpoint_unix.go` |
| #29 | [BUG-7] listenIPC orphans running daemon | **Still open** | Unconditional `os.Remove(socketPath)` before listen |
| #30 | [BUG-8] Version mismatch silent drop | **Still open** | `continue` on bad version at `chat/service.go:97-98` |
| #31 | [BUG-9] Stream router silent drop | **Still open** | Hardcoded switch; only `StreamChat` handled; BOLT-006 (#12) not on main |
| #32 | [BUG-10] PublishChat blocks on slow subscriber | **Still open** | `subsMu` held during sync writes in `ipc.go:127-141` |
| #36 | [BOLT-027] Config-dir hardening | **Still open** | Not in Wave 1 scope; expected |

## Summary counts

- **Closed this run:** 3 (#24, #25, #27)
- **Already closed (Wave 1 stories):** 6 (#9, #10, #11, #33, #34, #35)
- **Still open:** 8 (#23, #26, #28, #29, #30, #31, #32, #36)
- **Reopened:** 0

## Next QA actions

1. Re-verify BUG-1 (#23) when BOLT-001 (#7) PR opens — requires loopback harness (BOLT-010).
2. Re-verify BUG-4 (#26) when BOLT-008 (#14) lands — 4 MiB chunk round-trip test.
3. Take BOLT-012 (#18) FuzzReadFrame — QA-owned, no Wave 1 dependency.
4. After BOLT-006 (#12) merges, re-verify BUG-9 (#31).
5. BUG-6/7/8/10 remain P1/P2 — track against future stories or backlog items.

## Notes

- BUG-11, BUG-12, BUG-13 from qa-bug-hunt were never filed as GitHub issues (cap was BUG-1..10). BUG-12 overlap addressed by BOLT-005 (#11).
- `go mod tidy` required locally before build (flock v0.13.0); go.sum may differ from CI — not a QA fix.
