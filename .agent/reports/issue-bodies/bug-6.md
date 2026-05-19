## Description
`net.Listen("unix", socketPath)` (`internal/daemon/ipc_endpoint_unix.go:19-29`) creates the socket node with the current process umask applied to mode 0777 — on most desktop installs that's `0755` or `0775`, i.e. world-traversable. The token gate in `handleConn` prevents code execution by a same-host attacker, but the bare socket existence allows enumeration ("does this user run bolt?") and lets any local UID push frames at the daemon (consuming goroutines, exercising the JSON parser, eating the 1 MiB IPC frame allocation per request — a trivial local DoS).

## Steps to Reproduce
1. `bolt daemon`
2. `stat -f '%Sp' ~/.config/bolt/daemon.sock` → expect `srw-------`; observe `srwxr-xr-x` (depends on umask).

## Expected Behavior
Socket is owner-only (`0600` / `srw-------`).

## Actual Behavior
Socket permissions inherit umask; typically world-readable / world-traversable. (`~/.config/bolt/` is `0700` already, which helps on Linux but does not fully mitigate on systems that ignore directory traversal bits for socket files.)

## Environment
- `go version go1.25.1 darwin/arm64`
- macOS 25.2.0
- Repo at `e267dd2`

## Suggested Fix
After `net.Listen`, call `os.Chmod(socketPath, 0o600)`. Pair with the constant-time token comparison from BUG-10 (BUG-13 in QA report) as defense-in-depth; both are cheap.
