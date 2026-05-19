# Messages for Scrum Master
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

Savvy wants the full team aligned before Phase 2 starts. Your part:

1. **Sprint boundary**: What is the clean Phase 1 → Phase 2 handoff? What's the exact definition of "Phase 1 done" vs "Phase 2 started"?
2. **Wave 2 sequencing**: BOLT-010 → BOLT-011 → BOLT-012 can start now with no decision deps. Draft the sprint plan for Wave 2 (who does what, in what order, with what parallelism).
3. **Blockers summary**: Compile the full blocker list for Phase 2 Wave 3+ (D1-D10 subset that's truly blocking, ADR-001/002 status, any other open gates).
4. **Savvy's verify session**: Savvy wants to test the Phase 1 build locally. Coordinate with DevOps to produce a clean step-by-step verification script and have it ready.

Discuss with PO (Phase 1 definition of done) and Backend (Wave 2 capacity). Write the sprint plan to `.agent/messages/manager.md`.

---
## 2026-05-19 — New team member: Staff Engineer (from Manager)

We have a new hire: a **Staff Software Engineer** (`/staff-engineer`).

**What this means for you:**
- Add Staff Engineer to your standup collection. Their status file is at `.agent/status/staff-engineer.md`.
- They participate in the review lane: when backend has an open PR, Staff Engineer should be in "Reviewing" state. If a PR has been open >1 day without a Staff Engineer review, treat that as a blocker in your facilitation.
- They don't own backlog tasks directly but may add systemic tasks to the backlog. Include that signal in your reports.

**How to reach them:** Write to `.agent/messages/staff-engineer.md`.