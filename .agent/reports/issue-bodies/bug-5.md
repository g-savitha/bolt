## Description
Plan.md §"Signal handling in daemon" requires "remove `daemon.sock`, update `daemon.pid`" on signal. `cleanupIPC` only removes the socket. The PID file is written by `spawnDaemon` (`internal/daemon/spawn.go:55`) immediately after `cmd.Start()` — i.e. **before** the child has confirmed liveness — and is never deleted on shutdown. Any operator script that reads `daemon.pid` to decide whether bolt is alive will get a false-positive after a clean shutdown, and a false-negative if the child crashed during startup before the listener opened.

## Steps to Reproduce
1. `bolt init`
2. `bolt daemon`
3. `kill -TERM $(pgrep -f 'bolt daemon')`
4. `cat ~/.config/bolt/daemon.pid` still shows the old PID.
5. `ps -p <that pid>` returns nothing.

## Expected Behavior
PID file is owned by the daemon process; written **after** the listener is up (the spawn poll loop already proves liveness via socket connect), and deleted in the shutdown defer.

## Actual Behavior
PID file persists after shutdown and points at a dead process.

## Environment
- `go version go1.25.1 darwin/arm64`
- macOS 25.2.0
- Repo at `e267dd2`

## Suggested Fix
Move `os.WriteFile(pidPath, []byte(strconv.Itoa(os.Getpid())+"\n"), 0600)` into `Daemon.Run()` after `listenForPeers` returns success; drop the `os.WriteFile` in `spawnDaemon`; add `defer os.Remove(pidFilePath(d.configDir))` to `Run`. Also: keep the `flock` held by the daemon for its lifetime (currently released the moment `withPIDLock`'s callback returns), so a stale PID file but no process never racewins. Tracked as story BOLT-005.
