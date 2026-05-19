# bolt — Engineering Heartbeat
**Date**: 2026-05-19
**Pulse**: Coordinating

## Headline
Eight specialist sessions completed today: 27 stories queued (20 Ready-for-Dev / 7 Backlog), 10 protocol decisions pre-recommended for sign-off, 13 QA bugs documented with paste-ready issue bodies, 10 new security findings folded in (1 Critical / 2 High / 4 Medium / 3 Low). Phase 2 cannot start until Savvy locks D1-D10, approves at least ADR-001 and ADR-002, and runs three DevOps commands to fix `gh auth` and file 30 GitHub issues — every other dependency is internal and being worked.

## Roster (one line each)

| Role        | Status       | Last output                                            | Blocked on |
| ----------- | ------------ | ------------------------------------------------------ | ---------- |
| PO          | Reviewed     | 23-story backlog + plan-review (10 decisions, 12 edits, Phase-2 checklist) | Savvy D1-D10 sign-off |
| Architect   | Reviewed     | 20 findings (7 P0 / 8 P1 / 5 P2), 11 ADRs proposed     | Savvy ADR-001/002 sign-off |
| Backend     | Reviewed     | Phase-1 ~70% complete; 5 Phase-2 prereqs sized         | D1-D10 (can pull BOLT-003/004/024 today) |
| Networking  | Reviewed     | 10 protocol decisions, wire schemas drafted            | Savvy D1-D10 (via PO) |
| QA          | Reviewed     | 13 bugs (4 Critical), 10 ready-to-file issue bodies    | `gh auth` (DevOps owns) |
| Security    | Reviewed     | 10 new findings (1 C / 2 H / 4 M / 3 L), 3 new BOLT stories proposed | None — proposals folded into backlog this turn |
| DevOps      | Done (this session) | gh runbook + proposed CI + idempotent file-issues.sh (now 30 issues) | Savvy to run 3 commands |
| Scrum       | Coordinating | Standup written, 6 unblock messages routed             | None |
| Researcher  | Out          | Explicit, this round                                   | -- |
| Marketing   | Out          | Explicit, this round                                   | -- |
| Manager     | Reporting    | This heartbeat; folded SEC-1/2/3/6 into backlog as BOLT-024..027 | 4 items pending Savvy |

## What landed today
- `.agent/reports/po-plan-review.md` — TL;DR, 10 binary decisions for sign-off, 12 surgical `plan.md` edits, Phase-2 entry checklist, top-5 risk register.
- `.agent/reports/architecture-critique.md` — 20 findings (7 P0), 11 ADRs proposed (ADR-001 transport interface, ADR-002 IPC transport / Windows posture are the two blocking the rest).
- `.agent/reports/backend-review.md` — honest Phase-1 call (~70%); build/vet/lint/test/race all green; coverage 48.5% identity, 71% proto, 0% everywhere else.
- `.agent/reports/networking-review.md` — `quic.Config` under-specified, `ChunkMsg` JSON+base64 inflation a Phase-2 blocker, TOFU has two real holes (server-side verifier missing + no Ed25519 PoP).
- `.agent/reports/qa-bug-hunt.md` — 13 bugs (4 Critical), 10 paste-ready issue bodies, coverage-gap table by package.
- `.agent/reports/security-review.md` — `govulncheck` finds 15 reachable stdlib advisories (SEC-1, all closed by Go 1.25.10); 2 High (SEC-2 terminal escape injection, SEC-3 daemon env inheritance); 4 Medium / 3 Low folded into existing stories.
- `.agent/reports/devops-runbook.md` + `.agent/reports/proposed-ci.yml` + `.agent/reports/file-issues.sh` (mode 0755, `set -euo pipefail`, `--dry-run` honest, idempotent — now files 30 issues including BOLT-024..027).
- `.agent/backlog/tasks.md` — BOLT-001..027 (20 Ready-for-Dev, 7 Backlog); security addenda landed inline on BOLT-001/003/007/008/010/022.
- `.agent/messages/manager.md` — Scrum standup with 3-day sequencing and risk register.
- `.agent/reports/issue-bodies/` — 30 markdown bodies (16 BOLT-001..016 + 10 BUG + 4 BOLT-024..027) following the PO/QA templates.

## Blocker map

1. **Savvy — lock D1-D10** (`.agent/reports/po-plan-review.md` §"Decisions Savvy must lock"). Unblocks: PO, Networking, Backend (BOLT-001/002/006/007/008/013/016). Recommended answers pre-cited; this is sign-off, not re-debate. ETA: same-day if reviewed in one sit-down.
2. **Savvy — approve ADR-001 (transport interface) + ADR-002 (IPC transport / Windows posture)**. Unblocks: Architect (rest of ADR-001..011 can wait), Backend BOLT-006/016. ETA: 30 min review.
3. **Savvy — run 3 DevOps commands** (`gh auth login`, `gh auth status`, `.agent/reports/file-issues.sh`). Unblocks: QA issue filing, full GitHub-side traceability for 30 stories+bugs. ETA: 5 min once the browser flow completes.
4. **Savvy — decide D5 Windows posture** (Option B / delete recommended). Unblocks: BOLT-016 → BOLT-022 → final `.goreleaser.yaml` + `README.md` alignment. Same-day decision, this round.
5. **Savvy — confirm D7 (cut `v0.1` tag before Phase 2)**. Unblocks: Phase-1 release artifact + rollback target. 1 min decision.
6. **Networking — transitively blocked** on items 1 + 2 above. No workaround; coding PRs against `quic.Config` / wire schemas before D1/D3/D4 land risks a rewrite.

## What Savvy must do next (priority order)
1. Run the 3 DevOps commands in `.agent/reports/devops-runbook.md` — fixes `gh auth`, files all 30 issues.
2. Lock D1-D10 from `.agent/reports/po-plan-review.md` §"Decisions Savvy must lock". Pre-recommended answers are in the table.
3. Approve ADR-001 (transport interface) and ADR-002 (IPC transport / Windows posture). The remaining ADR-003..011 can wait.
4. Pick D5 Windows posture: in (commit to `IPCTransport` interface) or out (delete `*_windows.go` + drop PowerShell installer) — PO recommends OUT for v1.
5. Confirm D7: cut `v0.1` tag (Phase-1 only, no transfer) before Phase 2 starts.

## Risks I'm watching
- **TOFU defense-in-depth collapse.** BOLT-001 (real TOFU prompt) + BOLT-002 (Ed25519 PoP + server-side verifier) + BOLT-003 (atomic writes) must ship as one shippable bundle; lost trust state silently re-accepts everyone if BOLT-003 lands without BOLT-001.
- **Phase-2 start date slip.** BOLT-003/004/010/011/012/024 can absorb ~3 days of work without Savvy decisions; beyond that, Backend stalls waiting on D1-D10.
- **`pion/ice` ↔ `quic-go` `Transport` plumbing is the Phase-5 nightmare item** (N-§7). No prototype exists; budget a 3-5 day spike at Phase 5 kickoff and price it in now.
- **Wire schema churn mid-Phase-2.** If D1/D2/D10 aren't locked before any sender/receiver code is written, golden round-trip tests will catch drift late and force rewrites.
- **CI gap on `macos-latest`** — Phase-2 receiver's concurrent `WriteAt`/`Truncate` semantics differ between APFS and ext4; the live CI cannot catch APFS-specific bugs. BOLT-015 closes it.
