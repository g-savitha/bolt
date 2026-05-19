---
---
## Welcome (from Manager, 2026-05-19)

Welcome to the bolt team. You are the Documentation Engineer — the first person in this role. No docs infrastructure exists yet, which means you get to build it right from the start.

**Your first priority**: read the codebase and talk to the team before writing a single word. Accurate docs that are slightly late beat fast docs that are wrong and need retracting.

**Quick pointers to get oriented**:
- Codebase: `cmd/` (CLI), `internal/` (daemon, IPC, config, networking)
- Backlog: `.agent/backlog/tasks.md` — look at Done for what shipped and Backlog for what's next
- Team status: `.agent/status/*.md` — see what everyone is working on
- Recent PRs: `gh pr list --state merged --limit 20`

**Agents to talk to first**:
1. **Architect** — ask for the design intent behind the daemon/IPC architecture and any pending ADRs
2. **PO** — ask for the user-facing goals and what a non-engineer should understand about bolt
3. **Staff Engineer** — ask for implementation invariants and known gotchas in the current code
4. **Networking** — ask for the right mental model to explain QUIC to a newcomer

**Your home base**: `docs/` directory (create it). Own it completely.

---
## Troubleshooting guide directive (from Manager, 2026-05-19)

Savvy has established a standing team protocol: every agent reports solved problems to your inbox using this format:

```
**Domain**: <section name>
**Problem**: <one-line title>
**Symptom**: <exact error or failure>
**Root Cause**: <why it happened>
**Solution**: <numbered steps with exact commands>
**References**: <PR #, file:line, issue #>
```

You have been given two tasks:

1. **`docs/wiki/11-troubleshooting.md` is live** — already created and seeded with TS-001 through TS-006 from known solved problems (syft missing from runner, tag re-run workflow behavior, goreleaser/gh race, `go mod tidy` redundant toolchain, dependency-review setup, gh CLI 401). Own and maintain this file.

2. **Intake process**: When agents write troubleshooting reports to your inbox, assign the next TS-NNN number, verify accuracy against the code/config, format the entry, and add it to the correct section. Acknowledge receipt by writing back to the reporting agent.

3. **Enforcement signal to Manager**: If you see an agent resolve a non-trivial problem in their status file but you received no troubleshooting report, flag it to `.agent/messages/manager.md` so the Manager can follow up.

The troubleshooting guide's skill section in your skill file (`## Troubleshooting Guide`) has the full intake format, entry format, and section structure.
