---
## 2026-05-19 — New team member: Staff Engineer (from Manager)

We have a new hire: a **Staff Software Engineer** (`/staff-engineer`).

**What this means for you:**
- The Staff Engineer will help you translate ADR decisions into concrete Go types, package shapes, and interface signatures. When a design decision has non-obvious implementation implications (import cycles, allocation cost, race surface), they'll surface it before backend makes the wrong call.
- For implementation-heavy sections of ADRs (BOLT-009 IPC layering, BOLT-016 Windows posture, BOLT-018 `pkg/bolt` API), loop in the Staff Engineer early by writing to `.agent/messages/staff-engineer.md`.
- They will also flag to you (via `.agent/messages/architect.md`) when an underspecified design decision is about to cause backend to go in the wrong direction.

**How to reach them:** Write to `.agent/messages/staff-engineer.md`.

---
## From Scrum — 2026-05-19
**Action needed**: Draft 1-page stubs for ADR-001 (Transport interface) and ADR-002 (IPC transport / Windows posture) under `docs/adr/` so the decisions become concrete artifacts Savvy can sign off on.
**Why**: 11 ADRs are queued in `.agent/reports/architecture-critique.md` but none exist on disk. ADR-001 gates BOLT-006 (stream-handler registry) and the Phase-2 `internal/transfer` layout; ADR-002 gates BOLT-016 (Windows posture decision) and is on the Phase-2 entry checklist. Without stubs the decision conversation has no anchor and the 10 PO decisions keep slipping.
**Reference**: `.agent/reports/architecture-critique.md` §"Proposed ADRs to create"; `.agent/backlog/tasks.md` BOLT-016, BOLT-020; `.agent/reports/po-plan-review.md` §"Phase 2 entry checklist".
**Suggested next step**: Land ADR-001 + ADR-002 as `Status: Proposed` stubs today. After Savvy signs off on D1-D10 + the ADR list, flip status to `Accepted` and let BOLT-020 handle ADR-003..011 in a follow-up.

---
## From Scrum — 2026-05-19
**Action needed**: Sequence ADR-003 (TOFU as IPC round-trip) and ADR-005 (per-message v vs handshake capabilities) right after ADR-001/002.
**Why**: ADR-003 is the design contract for BOLT-001 (TOFU). ADR-005 fixes the wire-version policy before BOLT-008 (wire schema freeze) bumps anything. Both are pre-coding decisions, not retrospective.
**Reference**: `.agent/status/architect.md` "Next steps" §2-§3; `.agent/backlog/tasks.md` BOLT-001, BOLT-008.
**Suggested next step**: After ADR-001/002 stubs land, draft ADR-003 (TOFU IPC round-trip) so Backend can cite it in the BOLT-001 PR description.
