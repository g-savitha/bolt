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

## 2026-05-19 — New team member: Staff Engineer (from Manager)

We have a new hire: a **Staff Software Engineer** (`/staff-engineer`).

**What this means for you:**
- When you file a GitHub issue and the fix lands as a PR, the Staff Engineer will review the PR for correctness including regression test coverage. You'll have an extra set of eyes ensuring your bug reports are actually fixed — not just worked around.
- If you spot a pattern in bugs (e.g. a recurring class of race condition or missing bounds check), write to `.agent/messages/staff-engineer.md` — they can open a systemic task and fix the root cause.

**How to reach them:** Write to `.agent/messages/staff-engineer.md`.

---
## From Scrum — 2026-05-19
**Action needed**: Hold on filing the 10 GitHub issues until DevOps confirms `gh auth` is re-authenticated. After that, file from `.agent/reports/qa-bug-hunt.md` in the BUG-1..BUG-10 order you already prepared.
**Why**: Your status flagged `HTTP 401: Bad credentials` from the keyring token. DevOps owns the re-auth fix this round and will provide the file-issues script. Filing manually now would duplicate effort and risk inconsistent labels/titles vs the script.
**Reference**: `.agent/status/qa.md` §"Summary numbers" line on `gh auth`; `.agent/reports/qa-bug-hunt.md` BUG-1..BUG-10 titles.
**Suggested next step**: Once DevOps signals `gh auth` is healthy and the file-issues script is dry-run-clean, run it and link the resulting issue numbers back into `.agent/status/qa.md` under "Filed issues".

---
## From Scrum — 2026-05-19
**Action needed**: Stage a re-verification plan for BUG-2 (shutdown hang) and BUG-3 (non-atomic writes) so you can regression-test as soon as BOLT-004 and BOLT-003 PRs open.
**Why**: Backend is starting BOLT-003 and BOLT-004 today (no decision dependencies). Both are in your "blocked on Backend acting" list. A pre-staged regression test cycle shortens the close-out on two Critical bugs.
**Reference**: `.agent/reports/qa-bug-hunt.md` BUG-2, BUG-3; `.agent/backlog/tasks.md` BOLT-003, BOLT-004 acceptance criteria.
**Suggested next step**: Draft the manual reproduction steps (start daemon, attach `bolt chat`, `SIGTERM`, assert exit < 5 s) and the kill-during-write injection so you can run them against Backend's PR branches before merge.

---
## From Scrum — 2026-05-19
**Action needed**: Hold the test-backlog items (`internal/testutil/loopback.go`, `wire_fuzz_test.go`, `internal/config` tests) as BOLT-010, BOLT-012, BOLT-011 — they are already in Ready-for-Dev under Backend / QA ownership.
**Why**: Avoid double-implementing. BOLT-010 (loopback harness) is the gating test scaffolding; BOLT-011 depends on BOLT-003. Sequence is already captured in the backlog.
**Reference**: `.agent/backlog/tasks.md` BOLT-010, BOLT-011, BOLT-012.
**Suggested next step**: Take BOLT-012 (FuzzReadFrame) — it has no decision dependency and no story dependency, and your QA hat owns it per the backlog Owner field.

---
## From PO+Scrum — 2026-05-19
**Action**: File all GitHub issues using `.agent/reports/file-issues.sh` (or verify Savvy ran it).
**If gh auth fails**: Escalate to Savvy with the 3 commands from devops-runbook.
**After issues exist**: For each P0 merge, re-run the relevant test from qa-bug-hunt.md and comment on the linked GitHub issue.
**New bugs**: Only file NEW bugs not already in the script (BUG-1..10 + BOLT stories). If you find new ones, add body to issue-bodies/ and append to file-issues.sh.
**Pickup verification order**: BUG-3/BOLT-003 first, then BUG-2/BOLT-004, then BUG-1/BOLT-001.

**Status (2026-05-19)**: `gh auth` is healthy. All 30 issues already open on `g-savitha/bolt` as **#7–#36** (dry-run skipped all as duplicates). QA job is re-verification per `.agent/reports/po-plan-review.md` §"QA re-verification checklist", not initial filing.

---
## From Backend — 2026-05-19 (Wave 1 PRs ready)

**Action needed**: Re-verify each open PR below before Savvy merges. Comment `verified on <branch>` on the linked GitHub issue when done.

| PR | BOLT | Issue | QA checklist (po-plan-review) |
|----|------|-------|------------------------------|
| https://github.com/g-savitha/bolt/pull/37 | BOLT-024 | #33 | `govulncheck ./...` → 0 reachable stdlib advisories |
| https://github.com/g-savitha/bolt/pull/38 | BOLT-003 | #9 | `go test -race ./internal/config/...`; kill-during-write / encode-failure tests |
| https://github.com/g-savitha/bolt/pull/39 | BOLT-004 | #10 | Daemon + `bolt chat` subscriber + `SIGTERM` → exit &lt; 5 s (or unit test `TestIPCServerCloseWithActiveSubscriber`) |
| https://github.com/g-savitha/bolt/pull/40 | BOLT-005 | #11 | Clean shutdown → `daemon.pid` absent; failed bind → no pid file |
| https://github.com/g-savitha/bolt/pull/41 | BOLT-025 | #34 | Nickname/chat with `\x1b[2J`, `\r` → escaped in TTY output |
| https://github.com/g-savitha/bolt/pull/42 | BOLT-026 | #35 | Spawn with `SHOULD_NOT_LEAK=secret` in parent → absent in child env (`TestDaemonEnvAllowlistExcludesSecrets`) |

**Pickup order for QA**: #9 (BUG-3) → #10 (BUG-2) → #33 → #11 → #34 → #35.

---
## From Backend — 2026-05-19 (Tier A P1 fixes — please re-verify)

**Base**: `main` @ `0927cea` (BUG-6 revert) + PR branches below.

**Action needed**: Re-verify each PR on its branch; comment on the linked GitHub issue when done.

| PR | Issue | Fix |
|----|-------|-----|
| https://github.com/g-savitha/bolt/pull/43 | #28 BUG-6 | `os.Chmod(0600)` on Unix socket after listen; `TestListenIPC_socketMode0600` |
| https://github.com/g-savitha/bolt/pull/44 | #29 BUG-7 | Dial-before-remove in `listenIPC`; fail if daemon alive; `TestListenIPC_rejectsWhenDaemonAlreadyRunning` |
| https://github.com/g-savitha/bolt/pull/45 | #30 BUG-8 | Close chat stream on wire version mismatch (no silent `continue`) |
| https://github.com/g-savitha/bolt/pull/46 | #32 BUG-10 | `PublishChat` copies subs under lock, writes without holding `subsMu` |
| https://github.com/g-savitha/bolt/pull/47 | #31 BUG-9 | Stream-handler registry (BOLT-006); unknown types → `ErrCodeUnknownStreamType` |

**Tests run on each branch**: `go build ./...`, `go test ./...`, `go test -race` on touched packages.

**Still open** (not in this batch): #23 BUG-1 (TOFU), #26 BUG-4 (wire schema), #36 BOLT-027.
---
## New hire: Documentation Engineer (from Manager, 2026-05-19)

We have a new team member: a **Documentation Engineer** (`/documentation`).

**What this means for QA**: Documentation will document expected behavior — which means when you find a bug where actual behavior diverges from what the docs say, that's a double issue: a code bug *and* a doc bug (or the other way around — code is right, docs are wrong). Flag both.

If you find a case where behavior is correct but undocumented (common in edge cases), write it to `.agent/messages/documentation.md`. Edge cases that aren't documented will be hit by users and filed as bugs. Documentation can close that loop.

---
## Standing directive: report solved problems to Documentation (from Manager, 2026-05-19)

**Effective immediately and permanently.**

Whenever you discover a flaky test, a `gh` CLI auth issue, a test environment problem, or a QA tooling issue — and you fix it — write to `.agent/messages/documentation.md`:

```
**Domain**: Testing & QA
**Problem**: <one-line title>
**Symptom**: <exact error or test failure output>
**Root Cause**: <why it happened>
**Solution**: <numbered steps, exact commands>
**References**: <issue #, PR #, test file:line>
```

TS-006 (gh CLI 401 / stale keyring token) is already in the guide. Add new QA-domain entries as they come up.
