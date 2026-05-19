## Description
As an operator, I want `daemon.pid` to exactly mirror process liveness, so that scripts and supervisors can rely on its presence to mean "the bolt daemon is running and accepting connections".

Today `daemon.pid` is written by `spawnDaemon` before the child confirms liveness, and never removed on shutdown — combined, `daemon.pid` is essentially never trustworthy.

## Why
- Removes a stale-liveness footgun before transfers start touching `incomplete/*.json`.
- Sources: QA `BUG-5` and `BUG-12`, Backend review `B-4`, Architect `A-7`.

## Acceptance Criteria
- [ ] `daemon.pid` is written by `Daemon.Run` **after** `listenForPeers` succeeds, not by `spawnDaemon`.
- [ ] `daemon.pid` is removed in the `Daemon.Run` shutdown defer.
- [ ] `spawnDaemon` no longer touches the PID file; it relies on the socket poll for liveness (already in place at `spawn.go`).
- [ ] Integration test: start daemon, send `SIGTERM`, assert `~/.config/bolt/daemon.pid` does not exist.
- [ ] Integration test: simulate a failed spawn (bind port already taken) → `daemon.pid` does not exist after the spawn returns its error.
- [ ] Documentation note in code: `daemon.pid` is for human / supervisor introspection; the `flock` on it (already in `spawn.go`) is what guards against double-spawn.

## Notes
- Files: `internal/daemon/daemon.go` (Run), `internal/daemon/spawn.go` (spawnDaemon — remove PID write).
- Source story: `.agent/backlog/tasks.md` BOLT-005.
