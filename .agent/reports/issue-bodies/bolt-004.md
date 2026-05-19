## Description
As an operator, I want `SIGTERM` to a daemon that has an attached `bolt chat` subscriber to result in a clean exit within 5 s, so that CI tests, graceful restarts, and signal handling are not blocked by a parked `conn.Read`.

`IPCServer.Close()` currently calls `ln.Close()` then `wg.Wait()`, but `handleSubscribe` is blocked on `conn.Read(buf)` indefinitely. A `bolt chat` open + `SIGTERM` leaves the daemon never exiting; operator must `kill -9`.

## Why
- Blocks integration tests for shutdown behavior.
- Blocks Phase 2 transfer-checkpoint-on-SIGTERM behavior.
- Sources: QA `BUG-2`, Backend review `B-8`, Architect `A-7`.

## Acceptance Criteria
- [ ] `IPCServer.Close` iterates `s.subs` under `s.subsMu`, calls `conn.Close()` on each subscriber, clears the map, **then** calls `wg.Wait()`.
- [ ] `handleSubscribe` either takes a context derived from the daemon run-context and exits on `ctx.Done`, or returns cleanly when its `conn.Read` errors out (the `conn.Close` above guarantees this).
- [ ] Integration test: start a daemon, attach a subscriber over the IPC socket, send `SIGTERM`, assert the daemon exits within 2 s and `daemon.sock` is removed.
- [ ] `go test -race` clean: no goroutine leak detected on the shutdown integration test.
- [ ] Existing single-shot IPC commands (`status`, `connect`, `send_chat`) still return normally before the daemon exits.

## Notes
- Files: `internal/daemon/ipc.go` (IPCServer.Close + handleSubscribe), `internal/daemon/daemon.go` (shutdown defer ordering).
- Source story: `.agent/backlog/tasks.md` BOLT-004.
