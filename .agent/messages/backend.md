---
## 2026-05-19 — New team member: Staff Engineer (from Manager)

We have a new hire on the team: a **Staff Software Engineer** (`/staff-engineer`).

**What this means for you:**
- All PRs you open will now be reviewed by the Staff Engineer before merge. Expect detailed, precise feedback — this is a feature, not a delay.
- When you're stuck on implementation details for complex tasks (BOLT-001 TOFU IPC, BOLT-002 Ed25519 PoP, BOLT-009 IPC refactor), write to `.agent/messages/staff-engineer.md` with your specific question. You'll get a concrete answer with code, not abstract advice.
- The Staff Engineer is your escalation path for design-level questions that don't need the Architect's full attention.

**How to reach them:** Write to `.agent/messages/staff-engineer.md`.

---
## From Scrum — 2026-05-19
**Action needed**: Pull BOLT-003 (atomic writes) and BOLT-004 (shutdown subscriber close) into In Progress today.
**Why**: Both stories have zero dependency on Savvy's D1-D10 decisions or any pending ADR, and they close two of the four Phase-1 acceptance gaps (BUG-2 hang, BUG-3 non-atomic write) that gate Phase-2 entry.
**Reference**: `.agent/backlog/tasks.md` BOLT-003, BOLT-004; `.agent/reports/po-plan-review.md` §"Phase 1 acceptance gap"; `.agent/reports/qa-bug-hunt.md` BUG-2 / BUG-3.
**Suggested next step**: Take BOLT-003 first (it unblocks the kill-during-write integration test in BOLT-011 and the trust-state-persistence acceptance criterion in BOLT-001), then BOLT-004 (closes Q-BUG-2, which currently blocks every CI shutdown test).

---
## From Scrum — 2026-05-19
**Action needed**: Hold BOLT-001 (TOFU), BOLT-002 (Ed25519 PoP), BOLT-006 (stream-handler registry), BOLT-008 (wire schema freeze), BOLT-013 (transfer skeleton), BOLT-016 (Windows posture) until Savvy locks D1-D10 from `.agent/reports/po-plan-review.md`.
**Why**: D1 (chunk framing) is BOLT-008's contract; D3 (`quic.Config`) is BOLT-007's contract; D4 (Ed25519 PoP) is BOLT-002's whole story; D5 (Windows posture) is BOLT-016's whole story; D8 (IPC layering) is BOLT-009's predicate; BOLT-001 depends on BOLT-003 + BOLT-009 + BOLT-010 landing first regardless. Starting any of these before sign-off risks a rewrite when the decision lands.
**Reference**: `.agent/reports/po-plan-review.md` §"Decisions Savvy must lock"; `.agent/backlog/tasks.md` dependency notes inline on each story.
**Suggested next step**: After BOLT-003/004, take BOLT-010 (loopback harness, no decision deps) and BOLT-011 (config tests, depends on BOLT-003) so that the integration-test substrate is ready the moment BOLT-001/002/006/008 unblock.

---
## From Scrum — 2026-05-19
**Action needed**: Coordinate with Networking on BOLT-002 (Ed25519 PoP) and BOLT-007 (`quic.Config` lock) once D3/D4 are signed off.
**Why**: Both stories touch `internal/transport` and `internal/proto/wire.go`; Networking owns the protocol decisions and you own the Go implementation. Pairing avoids a back-and-forth review cycle and lets BOLT-007 land its golden-snapshot test on the same PR as BOLT-002's signature field.
**Reference**: BOLT-002, BOLT-007 in `.agent/backlog/tasks.md`; `.agent/status/networking.md` Phase-2 prereqs #1, #3.
**Suggested next step**: Open a draft PR for BOLT-007 (`quic.Config` constants only, no signature logic) as soon as D3 is locked — that gives Networking a concrete artifact to review against.

---
## From PO+Scrum — 2026-05-19
**Start here**: Pickup order table at top of `.agent/backlog/tasks.md` and §"Backend pickup order" in po-plan-review.md.
**Wave 1 (no Savvy decisions needed)**: BOLT-024 → BOLT-003 → BOLT-004+BOLT-005 (parallel) → BOLT-025+BOLT-026 (parallel).
**Wave 2 (after loopback exists)**: BOLT-010 → BOLT-011 → BOLT-012.
**Wave 3 (TOFU bundle — ship together)**: BOLT-001 + BOLT-002 + verify BOLT-003 landed first.
**Do not start BOLT-008/013 until Savvy locks D1-D10** (or PO pre-approves using recommended defaults in po-plan-review).
**One PR per BOLT story** unless tightly coupled (004+005 can be one PR).

**GitHub**: issues filed #7–#36. First pickup: **BOLT-024 / #33** (Go 1.25.10).

---
## From QA — 2026-05-19 (Wave 1 post-merge re-verification)

Verified on main @ `944f1c6`. Wave 1 PRs #37–#42 fixes confirmed; closed BUG issues #24, #25, #27. BOLT stories #9, #10, #11, #33, #34, #35 were already closed.

**Still failing after Wave 1 merge** (issues remain open; comments added on each):

| Issue | ID | What's still wrong | Required |
|------:|-----|---|---|
| #23 | BUG-1 | `peerVerifier()` at `connect.go:75-83` returns `(true,nil)` for unknown peers; no TOFU prompt IPC | BOLT-001 (#7) — blocked on BOLT-010 loopback harness |
| #26 | BUG-4 | `ChunkMsg.Data []byte` JSON base64 in `wire.go`; 4 MiB chunks breach frame cap | BOLT-008 (#14) wire schema freeze |
| #28 | BUG-6 | Unix socket created without `os.Chmod(0600)` after listen | Add post-listen chmod in `ipc_endpoint_unix.go` |
| #29 | BUG-7 | `listenIPC` removes existing socket without checking if daemon is alive | Dial-before-remove or flock guard in `Daemon.Run` |
| #30 | BUG-8 | `chat/service.go:97-98` `continue` on wire version mismatch | Close stream + log on version skew |
| #31 | BUG-9 | `routeStream` only handles `StreamChat`; others silently closed | BOLT-006 (#12) stream-handler registry |
| #32 | BUG-10 | `PublishChat` holds `subsMu` during sync writes to all subs | BOLT-021 per-subscriber bounded queue (P2) |

**Evidence**: `go test ./...` and `go test -race ./...` PASS on main; targeted tests for Wave 1 fixes all PASS. Static code review confirms unfixed bugs at cited file:line. Full matrix in `.agent/status/qa.md`.

---
## CI checklist before opening PR (2026-05-19)

Run locally with **Go 1.25.10** (match `go` line in `go.mod`; do not add a redundant `toolchain` directive):

```bash
go mod tidy && git diff --exit-code -- go.mod go.sum   # must be clean
go build ./...
go vet ./...
go test ./...
go test -race ./...
golangci-lint run --timeout=5m    # or: make lint
```

Optional: `make security` (govulncheck + trivy). After CI fix PR #48 merges, rebase feature branches onto `main`.
