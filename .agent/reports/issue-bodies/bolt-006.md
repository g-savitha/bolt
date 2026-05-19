## Description
As a backend engineer, I want `transfer.Service` to plug into `routeStream` without editing `daemon.go`, so that Phase-2 file transfer code can be added without churn in the daemon core.

Today `routeStream` only handles `StreamChat`; every other stream type (including legitimate `StreamFile`, `StreamControl`, and unexpected second `StreamHandshake`) falls into a silent `default:` close.

## Why
- Phase 2 sender/receiver register their stream handler here; required scaffolding.
- Sources: QA `BUG-9`, Backend review `B-2`, Architect `A-1`, Networking review §5.

## Acceptance Criteria
- [ ] New type `StreamHandler func(ctx context.Context, pc *transport.PeerConn, stream transport.Stream)`.
- [ ] `(*Daemon).RegisterStreamHandler(t proto.StreamType, h StreamHandler)` — concurrent-safe map (`sync.RWMutex`).
- [ ] `chat.Service.Register(d *Daemon)` wires `StreamChat` at boot; existing chat tests still pass.
- [ ] Unknown stream type → `stream.CancelRead(0x1000)`, `stream.CancelWrite(0x1000)`, log line `"unhandled stream type 0x%02x from %s — protocol violation"`. Define `proto.ErrCodeUnknownStreamType = 0x1000` as a named constant.
- [ ] Unit test (uses loopback from BOLT-010 / #TBD): register a handler for `0x05`, send a stream with that type, assert the handler is invoked with the correct `PeerConn` and stream.
- [ ] Unit test: send an unknown `0xFF` stream → stream is closed with code `0x1000`; no goroutine leak under `-race`.

## Notes
- Files: `internal/daemon/stream_handler.go` (new), `internal/daemon/daemon.go` (replace switch with registry lookup), `internal/chat/service.go` (auto-register at boot), `internal/proto/wire.go` (named error code).
- Source story: `.agent/backlog/tasks.md` BOLT-006.
