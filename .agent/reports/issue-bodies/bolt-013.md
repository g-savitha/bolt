## Description
As a backend engineer, I want the `internal/transfer` package and the new Phase-2 IPC types to exist as compile-clean stubs with godoc, so that Phase-2 PRs can land iteratively without inventing the layout at the same time as the algorithm.

## Why
- Lets sender / receiver / TOFU work proceed in parallel against a fixed package shape.
- Sources: Backend Phase-2 prereq §A-1 through §A-5, Architect's "Transport / IPCTransport / Discoverer interface view".

## Acceptance Criteria
- [ ] `internal/transfer/chunk.go` declares `Chunk{Index int; Data []byte; Hash [32]byte}` and the `ChunkFile(r io.Reader, chunkSize int) (<-chan Chunk, <-chan error)` signature; body returns `ErrNotImplemented`.
- [ ] `internal/transfer/sender.go` declares `Sender.Send(ctx, pc, src, name, size) (TransferResult, error)` with godoc explaining the per-chunk-per-stream model from BOLT-008; body returns `ErrNotImplemented`.
- [ ] `internal/transfer/receiver.go` declares `Receiver.Accept(ctx, pc, FileHeader) (TransferAck, *Sink, error)` with godoc explaining trust-check + disk-space + sink semantics; body returns `ErrNotImplemented`.
- [ ] `internal/transfer/service.go` declares `Service` with `Register(*Daemon)` that wires a `StreamFile` handler (depends on BOLT-006 / #TBD) returning `ErrNotImplemented`.
- [ ] IPC types added (no handlers yet) in `internal/proto/ipc.go` (depends on BOLT-009 / #TBD): `CmdSendFile`, `EventTransferProgress`, `EventTransferComplete`, `EventTransferSkipped`, plus their payload structs.
- [ ] `go build ./...` PASS; `go vet ./...` PASS; `golangci-lint run` PASS. No behavior tests yet — skeleton only.

## Notes
- Files: `internal/transfer/{chunk.go, sender.go, receiver.go, service.go}` (new), `internal/proto/ipc.go` (additions).
- Explicitly excluded: any actual transfer behaviour, retry logic, resume, disk-space check — those land in Phase 2 proper.
- Source story: `.agent/backlog/tasks.md` BOLT-013.
