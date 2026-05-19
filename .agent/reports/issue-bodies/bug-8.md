## Description
When `proto.ReadFrame` succeeds but `msg.V != proto.WireVersion`, `chat.AttachStream` (`internal/chat/service.go:92-99`) does `continue` rather than terminating the stream. A malicious or buggy peer sending a stream of bad-version frames burns CPU on parse + drop without ever delivering a message and without ever closing. Worse, it silently masks a real protocol version skew — the user sees "no chat" but no error, no log line, no IPC event.

## Steps to Reproduce
1. Read `internal/chat/service.go:97-99`.
2. Compare with `internal/transport/peer_conn.go:112-117` which correctly rejects on version mismatch during handshake.
3. (Dynamic) Wire a fake peer that emits `MsgChat{V: 99}` in a tight loop; daemon CPU rises, no error surfaces.

## Expected Behavior
A wire-version mismatch on an established stream is a protocol violation; close the stream with a non-zero error code, log it, and propagate a `peer_offline` event (when implemented) or at least a stderr line.

## Actual Behavior
Silently `continue`s; no log, no observable signal.

## Environment
- `go version go1.25.1 darwin/arm64`
- `quic-go v0.59.1`
- macOS 25.2.0
- Repo at `e267dd2`

## Suggested Fix
Replace `continue` with `return` plus a `fmt.Fprintf(os.Stderr, ...)` log line and `stream.CancelRead(1)` / `stream.CancelWrite(1)`.
