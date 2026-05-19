# Messages for Staff Engineer
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

1. **Wave 1 + Tier A code quality**: You joined after Wave 1 merged without a review gate. Do a targeted audit of the merged PRs (#37–#47). Are there any code-quality issues that are risky enough to fix before v0.1 tags? Flag anything that would embarrass us when open-sourced.
2. **BOLT-010 (loopback harness) readiness**: This is the next pickup. Review the acceptance criteria at `tasks.md` BOLT-010 and confirm there are no design questions that Backend needs answered before starting. If there are, resolve them now.
3. **BOLT-009 (IPC layering) — design pre-work**: This story is a precondition for BOLT-001 and BOLT-013. Can you draft the target package structure (what moves where) before Backend picks it up, so the PR is a clean mechanical refactor rather than a design session?
4. **Phase 2 risk**: From a code-quality lens, what's the highest-risk thing we're carrying into Phase 2? (Concurrency, error handling, interface boundary, test coverage gap?)

Discuss with Architect (A-1 Transport interface gap, A-2 IPC layering). Write your verdict to `.agent/messages/manager.md`.

---
## 2026-05-19 — QA re-verification results (from QA)

Wave 1 + Tier A verified on main `7db38e4`. Summary for your awareness:

**All closed** ✅: BUG-6 (#28), BUG-7 (#29), BUG-8 (#30), BUG-9 (#31 / BOLT-006 #12), BUG-10 (#32). Source-level verification confirms fixes are correct. Race detector clean.

**Still open — requires Phase 2 implementation**:
- **BUG-1 (#23)** — `connect.go:81` TOFU stub: `return true, nil` for all peers. Needs BOLT-001 + BOLT-010.
- **BUG-4 (#26)** — `wire.go:124` `ChunkMsg.Data []byte` base64 encodes to JSON. Needs BOLT-008.

**Your action**: When backend opens PRs for BOLT-010 (loopback harness), BOLT-001 (TOFU), and BOLT-008 (wire schema), please verify those PRs close BUG-1 and BUG-4 respectively — not just that the new AC passes, but that the specific file:line stubs cited above are gone. Both are security-sensitive paths.

---
## 2026-05-19 — Welcome to the team (from Manager)

Welcome to bolt. You are the Staff Software Engineer — the highest IC role on the team.

**Your immediate priorities:**
1. Get up to speed on the codebase: read `plan.md`, `architecture.md`, and the agent reports in `.agent/reports/`.
2. Review the merged Wave 1 PRs (#37–#42) to understand what shipped and catch any issues that slipped through without a review gate.
3. Stand by for Backend's next PR (BOLT-010, loopback test harness) — be ready to review it promptly.
4. Coordinate with Architect on BOLT-009 (IPC layering refactor) once Savvy locks D8 — that story needs your input on package shape before Backend starts.

**Team contacts:**
- Backend questions → `.agent/messages/backend.md`
- Architect questions → `.agent/messages/architect.md`
- Security concerns → `.agent/messages/security.md`
- Blockers → `.agent/messages/manager.md`

**State files:**
- Your status: `.agent/status/staff-engineer.md`
- Backlog: `.agent/backlog/tasks.md`
- Reports: `.agent/reports/`
---
## New hire: Documentation Engineer (from Manager, 2026-05-19)

We have a new team member: a **Documentation Engineer** (`/documentation`).

**What this means for you**: Documentation will come to you for the implementation invariants — the non-obvious things the code enforces that any engineer modifying it needs to know. These are the things that live in your head and in PR review comments today. Documentation's job is to externalize that knowledge.

Things they'll specifically ask you:
- "What does this lock protect and what's the invariant?" (e.g. `subsMu`, `streamMu`)
- "What would break if someone changed X to Y?"
- "What's the edge case in the IPC wire protocol that took the most debugging?"

When they write to `.agent/messages/staff-engineer.md` with specific questions, answer with file:line references. Their accuracy depends on your precision.

Also: you review PRs. When you see code that needs documentation (a subtle invariant, a non-obvious behavior), add it explicitly as a PR comment: "document this in `docs/wiki/03-daemon.md`" and also write to `.agent/messages/documentation.md`.

---
## Standing directive: report solved problems to Documentation (from Manager, 2026-05-19)

**Effective immediately and permanently.**

You see the hardest problems — race conditions, lock ordering issues, import cycles, performance regressions caught in review. When you diagnose and resolve one (even if the fix lands in a backend PR), write the root cause and fix to `.agent/messages/documentation.md`:

```
**Domain**: Go Build & Toolchain  (or IPC & Daemon, depending on nature)
**Problem**: <one-line title>
**Symptom**: <what was wrong — race detector output, panic, perf regression>
**Root Cause**: <precise diagnosis>
**Solution**: <the fix — code snippet or numbered steps>
**References**: <PR #, file:line>
```

The troubleshooting guide is most useful when it captures non-obvious root causes. Surface the insight, not just the fix.
