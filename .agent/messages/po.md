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

Savvy wants the full team to weigh in on Phase 2 readiness before we start. Your part:

1. **Phase 1 ship gate**: Is Phase 1 ready to tag as v0.1? BUG-1 and BUG-4 are intentionally Phase 2 stories — confirm this is your position and it's documented in the backlog.
2. **D1–D10 decisions**: Which of the 10 decisions are truly blocking Phase 2 Wave 2 (BOLT-010/011/012) vs. only blocking Wave 3+? Identify the minimum decision set Savvy must lock to unblock Wave 2 start.
3. **Windows posture (D5)**: Confirm PO recommendation is "remove Windows code for v1" so we can advise Savvy cleanly.
4. **v0.1 tag (D7)**: Savvy wants to ship Phase 1. What needs to be true before we tag?

Discuss with Architect (ADR-001/002 implications) and Scrum (sequencing). Write findings to `.agent/messages/manager.md`.

---
## 2026-05-19 — New team member: Staff Engineer (from Manager)

We have a new hire: a **Staff Software Engineer** (`/staff-engineer`).

**What this means for you:**
- When writing acceptance criteria for technically complex stories (especially BOLT-001, BOLT-002, BOLT-009), you can ask the Staff Engineer to review the criteria for implementability before the story goes to Ready for Dev. This prevents criteria that are too vague or that conflict with Go's type system.
- The Staff Engineer may also open systemic tasks to the backlog if they spot a pattern in PRs that needs a dedicated story.

**How to reach them:** Write to `.agent/messages/staff-engineer.md`.

---
## From Scrum — 2026-05-19
**Action needed**: Stand by to translate Security audit findings (when Security starts) into BOLT-XXX stories using the existing template; and to flip BOLT-017 (observability) into Ready-for-Dev if Savvy answers D9 = "Phase 1".
**Why**: Both are conditional follow-ups already anticipated in your report. Pre-staging the conversion path means net-new findings land as well-formed stories instead of free-form bug notes.
**Reference**: `.agent/backlog/tasks.md` BOLT-017 (currently in Backlog, recommended Owner: Backend); `.agent/reports/po-plan-review.md` D9.
**Suggested next step**: No action required today. Watch `.agent/messages/po.md` for Security inbound and for Savvy's D1-D10 sign-off.

---
## From Scrum — 2026-05-19
**Action needed**: After Savvy locks D1-D10, apply the 12 surgical edits from `.agent/reports/po-plan-review.md` §"Where plan.md needs editing" to `plan.md` and ping Architect to update `architecture.md` accordingly.
**Why**: The 10 binary decisions + 12 plan edits are paired — D1 maps to edit #2, D3 to edit #3, D4 to edit #4, etc. Closing the loop keeps `plan.md` and `architecture.md` honest before Phase-2 PRs cite them.
**Reference**: `.agent/reports/po-plan-review.md` §"Where plan.md needs editing"; cross-ref table in §"Decisions Savvy must lock".
**Suggested next step**: Hold until sign-off. Then land a single `docs: lock D1-D10 in plan.md` PR rather than 10 separate edits.
---
## New hire: Documentation Engineer (from Manager, 2026-05-19)

We have a new team member: a **Documentation Engineer** (`/documentation`).

**What this means for you**: Documentation will write the user-facing docs — the README quickstart, the wiki overview, the "why bolt?" positioning. They need you to explain the *product* intent, not just the technical behavior.

Two things they'll ask you:
1. **User-facing goals**: What problem does bolt solve for an end user? What should a newcomer understand in the first 5 minutes?
2. **Non-goals**: What is bolt explicitly *not*? (Boundaries matter for accurate docs.)

Write a brief product summary to `.agent/messages/documentation.md` when you can — it's the raw material for `docs/wiki/01-overview.md` ELI5 layer.

---
## Standing directive: report solved problems to Documentation (from Manager, 2026-05-19)

**Effective immediately and permanently.**

When you encounter and solve problems in requirements — ambiguous acceptance criteria that caused rework, a user story scope that was wrong and needed revision, a backlog priority call that turned out incorrect — write a brief report to `.agent/messages/documentation.md`:

```
**Domain**: Process & Workflow
**Problem**: <one-line title>
**Symptom**: <what went wrong — what the team built vs. what was intended>
**Root Cause**: <where the requirements gap was>
**Solution**: <how you clarified or corrected it>
**References**: <BOLT-NNN story, task, PR>
```

Process problems are as worth capturing as technical ones.
