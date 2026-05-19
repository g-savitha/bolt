---
---
## 🚀 Released: v0.1.2 (from Release Manager)

**Tag**: v0.1.2 | **Date**: 2026-05-19 | **SHA**: ecb8683
**GitHub**: https://github.com/g-savitha/bolt/releases/tag/v0.1.2

**What shipped**:
- 🔒 Go 1.25.10 — closes 15 stdlib vulns (BOLT-024)
- 🔒 TTY sanitization for peer strings (BOLT-025)
- 🔒 Daemon env allowlist — shell secrets no longer inherited (BOLT-026)
- 🐛 Atomic config writes — survives kill-9 (BOLT-003)
- 🐛 Clean daemon shutdown on SIGTERM (BOLT-004)
- 🐛 daemon.pid lifecycle fixed (BOLT-005)
- 🐛 IPC socket chmod 0600, stale socket guard, chat version mismatch, PublishChat lock (BUG-6/7/8/10)
- ✨ Pluggable stream-handler registry (BOLT-006 / BUG-9)

**Still open** (deferred to Phase 2 by design):
- BUG-1 (#23) — TOFU stub, fix is BOLT-001
- BUG-4 (#26) — ChunkMsg base64, fix is BOLT-008

**What's next**: Phase 2 Wave 2 — BOLT-010 (loopback harness) is the next pickup.

## 2026-05-19 — Phase 2 GO/NO-GO discussion (from Manager, Savvy directive)

Savvy wants the full team to weigh in before Phase 2 starts. Your part:

1. **Architecture blockers**: Which of your 20 findings (A-1 through A-20) are hard blockers that will cause Phase 2 code to be thrown away vs. tech-debt we can carry? Be specific.
2. **ADR-001 (Transport interface)**: Is this required *before* Backend starts BOLT-010 (loopback harness), or can loopback be written and then refactored behind the interface? What's the cost of deferring it?
3. **ADR-002 (Windows posture)**: PO recommends dropping Windows code. If Savvy agrees, what exactly must be deleted? Give Backend a precise file list.
4. **BOLT-009 (IPC layering)**: This is a precondition for BOLT-001 and BOLT-013. Can it be done before Savvy locks D8, or does the package structure depend on that decision?

Discuss with PO (D1-D10 prioritization), Networking (protocol contract for BOLT-008), and Staff Engineer (implementation feasibility). Write your verdict to `.agent/messages/manager.md`.

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
---
## New hire: Documentation Engineer (from Manager, 2026-05-19)

We have a new team member: a **Documentation Engineer** (`/documentation`).

**What this means for you specifically**: Documentation will be your closest partner for translating architecture decisions into something the whole team can reason from. They want two things from you:

1. **ADR content**: You design the system; they write it down. When you make a significant design decision, write the bullet points to `.agent/messages/documentation.md` — they'll shape it into a proper ADR (Context, Decision, Consequences, Alternatives Considered).
2. **Architecture diagrams**: They'll ask you to describe component relationships in plain language, then turn those descriptions into Mermaid diagrams. If a diagram they produce misrepresents the design, flag it immediately — wrong diagrams are a form of technical debt.

**Standing ask**: Write a brief summary of the daemon/IPC architecture intent to `.agent/messages/documentation.md` when you have a moment. It's the first thing they'll need to write `docs/wiki/02-architecture.md`.

---
## Standing directive: report solved problems to Documentation (from Manager, 2026-05-19)

**Effective immediately and permanently.**

When you work through a design problem — package layout conflicts, interface design trade-offs, concurrency model decisions that needed revision — and land on a solution, write it to `.agent/messages/documentation.md`:

```
**Domain**: IPC & Daemon  (or whichever subsystem applies)
**Problem**: <one-line title>
**Symptom**: <what made the original design wrong — test failure, import cycle, runtime issue>
**Root Cause**: <the design gap>
**Solution**: <the corrected design decision>
**References**: <ADR #, PR #, file:line>
```

Architecture-level problems often get captured in ADRs, but the troubleshooting entry captures the *journey* — what was tried first and why it failed — which ADRs often omit.
