## Description
`routeStream` (`internal/daemon/daemon.go:203-213`) only handles `StreamChat`. Any other stream type — including the legitimate `StreamFile` and `StreamControl` that Phase 2 needs, and an unexpected second `StreamHandshake` (which would itself indicate a protocol violation) — falls into the `default:` branch which `stream.Close()`s silently. The conditional `fmt.Fprintf` log line suppresses output for `StreamHandshake`, which means a malicious peer could spam handshake streams to consume goroutine/accept budget with zero observability.

## Steps to Reproduce
1. Read `internal/daemon/daemon.go:203-213`.
2. There is no handler registry; the switch statement is the entire dispatch.

## Expected Behavior
A `map[proto.StreamType]StreamHandler` registry that `chat.Service` registers itself into; default is "log + close with error code 1 (protocol-violation)"; a duplicate `StreamHandshake` should be logged loudly as a likely attack.

## Actual Behavior
Silent close for everything but `StreamChat`; no registry, no Phase-2 extension point.

## Environment
- `go version go1.25.1 darwin/arm64`
- `quic-go v0.59.1`
- macOS 25.2.0
- Repo at `e267dd2`

## Suggested Fix
Define `type StreamHandler func(ctx context.Context, pc *transport.PeerConn, stream *quic.Stream)`; expose `(*Daemon).RegisterStreamHandler(t proto.StreamType, h StreamHandler)`; have `chat.Service` register at construction; collapse the default case to one `stream.CancelRead(1); stream.CancelWrite(1); log("unhandled stream type 0x%02x from %s — protocol violation")`. Tracked as story BOLT-006.
