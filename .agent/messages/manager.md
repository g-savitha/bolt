---
## 2026-05-19 — New hire: Staff Software Engineer (from Savvy)

**Action needed**: Onboard the new Staff Engineer to the team.

Savvy has added a Staff Software Engineer (`/staff-engineer`) to the bolt team. This is an expert-level IC who reviews all backend PRs and supports the architect and backend engineer on implementation details.

**Your tasks:**
1. Announce the new hire to all agents (messages routed to each agent's inbox — done).
2. Create a status file at `.agent/status/staff-engineer.md` (idle/onboarding).
3. Brief the Staff Engineer on the current state: open PRs, active tasks, and priority items from the backlog.
4. Ensure Scrum Master adds Staff Engineer to standup collection.
5. Update the heartbeat report to include Staff Engineer in the team roster.

**Why:** PR review has been missing from the workflow. All PRs since Wave 1 (BOLT-024 through BOLT-026) merged without a formal code review gate. The Staff Engineer closes that gap and provides implementation mentorship for the complex Wave 3 tasks (BOLT-001, BOLT-002, BOLT-009).

---
## Scrum Master Update — 2026-05-19 10:25 IST

### Done since last standup
- PO: 16 Ready-for-Dev stories (BOLT-001..016) + 7 backlog items (BOLT-017..023), plan-review report at `.agent/reports/po-plan-review.md` with 10 binary decisions, 12 plan edits, Phase-2 entry checklist, risk register.
- Architect: 20 findings (7 P0, 8 P1, 5 P2), 11 ADRs proposed, 6 mermaid additions; full report at `.agent/reports/architecture-critique.md`.
- Backend: Phase-1 ~70% complete (honest call); build/vet/lint/test/race all green; 5 Phase-2 prerequisites locked and sized; 3 code-quality P0s (B-1 TOFU stub, B-2 stream router, B-3 non-atomic writes); report at `.agent/reports/backend-review.md`.
- Networking: 10 decisions queued for Savvy; 3 top critiques (chunk framing, `quic.Config`, TOFU + PoP); 5 Phase-2 prereqs scoped totalling ~9 critical-path days; report at `.agent/reports/networking-review.md`.
- QA: 13 bugs found (4 Critical, 9 Important, 5 Nice-to-have); ready-to-paste issue bodies; gh-auth blocker reported; report at `.agent/reports/qa-bug-hunt.md`.

### In Progress
- Security: audit not yet started (status: Idle/Never).
- DevOps: gh auth fix + CI baseline + file-issues script (status: Idle/Never — message routed).
- Scrum: this update.

### Blocked
- Backend: blocked on Savvy locking D1-D10 from `.agent/reports/po-plan-review.md` before starting BOLT-001/002/006/008/013/016. Workaround: start BOLT-003 (atomic writes) and BOLT-004 (shutdown subscriber close) today — no decision dependencies. Message routed to backend.md.
- Architect: blocked on Savvy ADR sign-off for ADR-001..011. Workaround: draft ADR-001 (transport interface) and ADR-002 (IPC transport / Windows posture) as `Status: Proposed` stubs under `docs/adr/` today. Message routed to architect.md.
- QA (filing GitHub issues): blocked on `gh auth` HTTP 401 from keyring token. Resolution: DevOps owns the re-auth + file-issues script. `gh issue list --state open` from this snapshot returned `Forbidden` (api.github.com graphql) — consistent with the broken auth. Messages routed to qa.md and devops.md.
- Networking: blocked on PO sign-off of the 10 decisions (transitively blocked on Savvy). No workaround; coding PRs against `quic.Config` / wire schemas before D1/D3/D4 risks a rewrite.
- PO: blocked on Savvy responses to D1-D10 + ADR list. No workaround.
- Researcher: blocked (explicit, out of scope this round; not contacted).
- Marketing: out of scope this round (not contacted).

### Sequencing concerns (soft blockers, surface for awareness)
- BOLT-010 (loopback harness) is on the integration-test critical path for BOLT-001 / BOLT-002 / BOLT-006 / BOLT-008 / BOLT-013. Recommend Backend take BOLT-010 in parallel with BOLT-003/004 so it's ready when the decision-gated stories unblock.
- BOLT-009 (move IPC types to `internal/proto`) is a precondition for BOLT-001 and BOLT-013 to add new IPC types in the right package. Pull it forward immediately after D8 sign-off.
- BOLT-016 (Windows posture) gates BOLT-022 (README/QUICKSTART alignment) and the Phase-2 entry checklist; needs Savvy's D5 to be actionable.

### Needs Savvy decision
1. **Lock the 10 binary decisions** in `.agent/reports/po-plan-review.md` §"Decisions Savvy must lock" (D1 chunk framing, D2 stream count, D3 `quic.Config`, D4 Ed25519 PoP, D5 Windows posture, D6 lib-vs-CLI, D7 v0.1 tag, D8 IPC layering, D9 observability phase, D10 resume granularity). Recommended answers are pre-cited; this is sign-off, not re-debate.
2. **Approve the 11-ADR list** — at minimum ADR-001 (transport interface) and ADR-002 (IPC transport / Windows posture) so BOLT-016 isn't ambiguous and Backend can cite ADR-001 in BOLT-006.
3. **Confirm D7**: cut a `v0.1` tag (Phase-1 only) before Phase 2 starts?
4. **Confirm D5**: Windows IN (commit to `IPCTransport` interface) or OUT (delete `*_windows.go` + PowerShell installer) for v1? PO recommends OUT.

### Risks
- Phase-2 start date slips if D1-D10 are not locked in the next 48h. BOLT-003/004/010/011/012 can absorb up to ~3 days of work in parallel; beyond that, Backend stalls.
- `gh` auth being broken in the keyring may recur in CI runs that need to comment on PRs or file issues. DevOps message includes a request to add a `gh auth status` pre-flight to BOLT-015 CI work.
- TOFU defense-in-depth collapse risk persists until BOLT-001 + BOLT-002 + BOLT-003 ship together (lost trust state silently re-accepts everyone). Track as a single shippable bundle.

### Path forward (suggested 3-day sequencing)
- **Day 1 (today, parallelisable)**:
  - Savvy: lock D1-D10 + ADR-001..011 go/no-go.
  - DevOps: fix `gh auth`; draft `.agent/reports/file-issues.sh`; dry-run before live.
  - Backend: start BOLT-003 (atomic writes) and BOLT-004 (shutdown hang) — no decision deps.
  - Architect: write ADR-001 and ADR-002 stubs in `docs/adr/`.
  - QA: stage regression repros for BUG-2 / BUG-3 against Backend's PR branches.
- **Day 2**:
  - QA: file BUG-1..BUG-10 via DevOps's script; take BOLT-012 (FuzzReadFrame).
  - Backend: BOLT-010 (loopback harness) + BOLT-011 (config tests, depends on BOLT-003) in parallel.
  - Networking: pair with Backend on BOLT-002 (Ed25519 PoP) and BOLT-007 (`quic.Config`) once D3/D4 land.
  - Architect: draft ADR-003 (TOFU as IPC round-trip).
- **Day 3**:
  - Backend: BOLT-001 (TOFU, depends on BOLT-003 + BOLT-009 + BOLT-010), BOLT-008 (wire schema freeze), BOLT-013 (transfer skeleton, depends on BOLT-006 + BOLT-009).
  - PO: land the 12 `plan.md` edits in one PR.
  - Phase-2 GO/NO-GO checkpoint against `.agent/reports/po-plan-review.md` §"Phase 2 entry checklist".

### Stalled-tasks check
- In Progress section of the backlog is empty at this snapshot — no >24h stalls to flag.
- Policy going forward: any story in In Progress without a status update for >24h is flagged in the next standup.

---
## Scrum Master Update — 2026-05-19 (PO+Scrum consolidation)
### Done
- PO+Scrum: consolidated pickup order, updated po-plan-review, reorganized backlog
- GitHub issues: **30 already filed** on `g-savitha/bolt` (#7–#36); `gh auth` OK; `file-issues.sh --dry-run` skipped all as duplicates
### In Progress
- Backend: awaiting pickup from row #1 (BOLT-024 / #33)
### Blocked
- GitHub traceability: **unblocked** (issues exist)
- Phase 2 coding: still blocked on Savvy D1–D10 for BOLT-001/002/007/008/016
### Needs Savvy
- Lock D1–D10 for Wave 3+ (recommended answers in po-plan-review)
- Approve ADR-001 + ADR-002
- Confirm D5 (Windows) and D7 (v0.1 tag)
