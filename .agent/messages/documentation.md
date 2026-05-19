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
