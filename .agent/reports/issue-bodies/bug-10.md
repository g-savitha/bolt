## Description
`IPCServer.PublishChat` (`internal/daemon/ipc.go:115-130`) acquires `subsMu` then iterates `s.subs` calling `writeIPCFrame(conn, event)` for each. The write is synchronous and uses no deadline. Any subscriber whose socket buffer fills (slow reader, suspended terminal, debugger paused at a breakpoint) will hold the mutex for the duration of the OS-level send blocking. While that mutex is held, every other chat message is queued — `daemon.onChatMessage` will block too, which means the chat reader goroutine in `service.AttachStream` stops draining the QUIC stream → QUIC flow control window closes → the peer's send blocks → real chat latency for everyone.

## Steps to Reproduce
1. Open `bolt chat` in terminal A; suspend the CLI with `Ctrl-Z`.
2. Send 10 chat messages from a peer.
3. Observe daemon stops servicing other clients (status, connect, etc.).

## Expected Behavior
Per-subscriber send is non-blocking or bounded; one stuck subscriber doesn't poison the others.

## Actual Behavior
One stuck subscriber stalls all fan-out and ultimately the QUIC stream.

## Environment
- `go version go1.25.1 darwin/arm64`
- `quic-go v0.59.1`
- macOS 25.2.0
- Repo at `e267dd2`

## Suggested Fix
Per-subscriber buffered channel (`chan IPCEvent`, buffer 128), each `handleSubscribe` runs its own writer goroutine reading from the channel. `PublishChat` only does `select { case ch <- evt: default: drop+log }` while holding the mutex briefly. Or set a `conn.SetWriteDeadline(time.Now().Add(2*time.Second))` before each write and drop+close on timeout. Tracked as backlog story BOLT-021.
