# QA Wave 1 Re-Verification — 2026-05-19

**Main:** `944f1c6` (Wave 1 PRs #37–#42 merged)

## Outcome

Wave 1 fixes verified. Three BUG-tracker issues closed (#24, #25, #27). Six BOLT story issues were already closed (#9, #10, #11, #33, #34, #35). Eight issues remain open.

## Verified fixes

| PR | BOLT | Bug | Test / evidence |
|----|------|-----|-----------------|
| #37 | BOLT-024 | SEC-1 | `govulncheck ./...` → No vulnerabilities found |
| #38 | BOLT-003 | BUG-3 | `go test ./internal/config/... -race -v` — 4 tests PASS |
| #39 | BOLT-004 | BUG-2 | `TestIPCServerCloseWithActiveSubscriber` PASS |
| #40 | BOLT-005 | BUG-5/12 | `TestDaemonPIDWriteAndRemove` PASS; spawn.go no PID write |
| #41 | BOLT-025 | SEC-2 | `TestSanitizeDisplay*` — 4 tests PASS |
| #42 | BOLT-026 | SEC-3 | `TestDaemonEnvAllowlistExcludesSecrets` PASS |

Full suite: `go test ./...` and `go test -race ./...` — PASS.

## Still open (expected)

- **#23 BUG-1** — TOFU prompt not implemented (BOLT-001)
- **#26 BUG-4** — ChunkMsg JSON base64 (BOLT-008)
- **#28–#32** — BUG-6 through BUG-10; no Wave 1 PR
- **#36 BOLT-027** — config-dir hardening; P2 backlog

## Backend notification

See `.agent/messages/backend.md` — appended summary for still-open P0/P1 bugs.
