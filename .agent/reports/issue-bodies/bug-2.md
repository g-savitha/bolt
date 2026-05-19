## Description
`IPCServer.Close()` (`internal/daemon/ipc.go:109-113`) calls `s.ln.Close()` then `s.wg.Wait()`. Closing the listener stops `Accept` but does **not** close already-accepted connections. `handleSubscribe` (`ipc.go:168-187`) is parked on `conn.Read(buf)` and only returns when the client itself disconnects. If a `bolt chat …` session is open when the daemon receives `SIGTERM` / `SIGINT` / `SIGHUP`, the daemon never exits — `wg.Wait` blocks forever, the deferred `cleanupIPC` never runs, and `daemon.sock` stays on disk.

## Steps to Reproduce
1. Start daemon: `bolt daemon`.
2. Start subscriber: `bolt chat <peer>` (opens a long-lived `subscribe` IPC connection).
3. Send `SIGTERM` to the daemon: `kill -TERM <pid>`.
4. Daemon prints `bolt daemon shutting down...`, then never exits.

## Expected Behavior
SIGTERM triggers a bounded graceful shutdown (single-digit seconds), closes the listener and all live subscriber connections, removes the socket, exits cleanly.

## Actual Behavior
Hang. Operator must `kill -9` the daemon. On next start the stale socket has to be re-removed by `listenIPC`; `daemon.pid` points at a dead process (see BUG-5).

## Environment
- `go version go1.25.1 darwin/arm64`
- `quic-go v0.59.1`, `BurntSushi/toml v1.6.0`, `gofrs/flock v0.12.1`
- macOS 25.2.0
- Repo at `e267dd2`

## Suggested Fix
In `IPCServer.Close()`, before `wg.Wait()`, iterate `s.subs` under `s.subsMu` and call `conn.Close()` on each, then clear the map. That unblocks the `conn.Read` in `handleSubscribe`, which returns and decrements `wg`. Alternative: wire the daemon's run context into `handleSubscribe` and set a read deadline that's refreshed on `ctx.Done()`. Tracked as story BOLT-004.
