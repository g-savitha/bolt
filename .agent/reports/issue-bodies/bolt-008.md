## Description
As a backend engineer about to write the Phase-2 sender and receiver, I want every wire shape they exchange to be fully defined with golden round-trip tests, so that sender and receiver can be developed in parallel against a stable contract.

## Why
- Removes the base64 inflation foot-gun documented in `QA BUG-4`.
- Sources: QA `BUG-4`, Backend review `B-9`, Networking review §3 / decision #1.

## Acceptance Criteria
- [ ] `FileHeader{v:1, transfer_id, name, size_bytes, chunk_size, total_chunks, file_hash}` defined and round-trip tested.
- [ ] `ChunkHeader{v:1, transfer_id, index, chunk_hash, len}` defined; wire framing on a chunk stream is `[1-byte StreamFile][length-prefixed JSON ChunkHeader][exactly ChunkHeader.len raw bytes]`. Stream ends after the raw bytes are written by the sender (one-chunk-per-stream).
- [ ] `TransferAck{v:1, transfer_id, accepted, reason, resume_from_chunks []int}` defined and round-trip tested.
- [ ] `TransferDone{v:1, transfer_id, file_hash}` defined and round-trip tested.
- [ ] `CancelMsg{v:1, transfer_id, reason}` defined and round-trip tested.
- [ ] `ChunkMsg.Data []byte` is **removed** from `internal/proto/wire.go`; any reference outside the deleted code site fails compilation.
- [ ] Round-trip test for a 4 MiB chunk via `ChunkHeader + raw bytes`: serialized size on wire is `≤ 4*(1<<20) + 256` bytes (no base64 inflation).
- [ ] `maxFrameSize` raised to 16 MiB for JSON envelopes only; documented in code that raw chunk bytes are not subject to this cap.

## Notes
- Files: `internal/proto/wire.go`, `internal/proto/wire_test.go`.
- Lock D1 (header-then-raw framing) and the chunk-stream contract before any sender/receiver code is written.
- Source story: `.agent/backlog/tasks.md` BOLT-008.
