## Description
`listenIPC` (`internal/daemon/ipc_endpoint_unix.go:19-29`) removes any existing socket file and then `net.Listen`s a fresh one. This is correct on cold start (cleans stale node after `kill -9`) but is wrong when a daemon is already healthy and a user runs `bolt daemon` a second time by accident or by an over-eager systemd unit. The old daemon's listening FD is still valid in the kernel, but its filesystem name is removed and replaced with the new daemon's socket. CLI clients connecting via the path now reach the new daemon; the old one is unreachable forever — it owns the QUIC listener, has the PeerRegistry, but no one can talk to it.

## Steps to Reproduce
1. Terminal A: `bolt daemon &`.
2. Terminal B: `bolt daemon`.
3. Terminal C: `bolt status` — hits the second daemon, not the first.
4. The first daemon is now orphaned; only `kill -9` can clear it.

## Expected Behavior
If the existing socket accepts a connection (i.e. a daemon is already listening), refuse to start a second one with a clear "already running" error.

## Actual Behavior
Second daemon replaces the first's socket; first becomes unreachable.

## Environment
- `go version go1.25.1 darwin/arm64`
- macOS 25.2.0
- Repo at `e267dd2`

## Suggested Fix
In `listenIPC`, before removing the socket file, attempt a dial — if it succeeds, return `errors.New("bolt daemon is already running at <path> — use bolt status")`. Or share the same `flock(daemon.pid)` in `Daemon.Run` so manual `bolt daemon` invocations are mutually exclusive with `EnsureRunning`-spawned ones.
