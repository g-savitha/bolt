## Description
`ChunkMsg.Data []byte` (`internal/proto/wire.go:115-122`) is serialized by `encoding/json`, which base64-encodes byte slices (×4/3 inflation). Plan.md fixes LAN chunk size at 4 MiB. After base64 (`4 MiB * 4/3 = ~5.33 MiB`) plus JSON quoting, the surrounding object, the SHA-256 hex hash (64 bytes), version/index/transferID fields, and JSON field names, a single legitimate chunk easily exceeds the 8 MiB `maxFrameSize` cap that `WriteFrame` enforces (`wire.go:160`) and `ReadFrame` checks (`wire.go:185`). Phase 2 will hit a hard `message too large` error on the first LAN chunk and the file transfer feature will be DOA on the wire.

## Steps to Reproduce
1. (Manual, since `internal/transfer/` doesn't exist yet)
   ```go
   data := make([]byte, 4 << 20)
   msg := proto.ChunkMsg{V:1, TransferID:"x", Index:0, ChunkHash:strings.Repeat("a",64), Data:data}
   proto.WriteFrame(&buf, msg)
   ```
2. Observe failure with `message too large: ~5,592,xxx bytes` once the envelope plus base64 inflation pass the 8 MiB cap.
3. Even when individual chunks squeak under 8 MiB, 8 parallel goroutines all allocating ~5.4 MiB simultaneously create ~43 MiB/s of GC churn at 1 Gbit.

## Expected Behavior
Binary framing for chunk bytes — either `[1-byte StreamFileChunk][JSON header (small)][chunk bytes raw]` or a new `StreamFileChunk = 0x05` type that uses `[chunk-len uint32][chunk bytes]` after the type byte. No base64.

## Actual Behavior
JSON envelope forces base64 inflation and large-allocation churn.

## Environment
- `go version go1.25.1 darwin/arm64`
- macOS 25.2.0
- Repo at `e267dd2`

## Suggested Fix
Either (a) split `ChunkMsg` into header (JSON-framed) + body (raw, length-prefixed) on the same stream, or (b) introduce a binary-only `StreamFileChunk` type. Either change requires bumping `WireVersion` if Phase-2 sender/receiver are to remain compatible with each other. Tracked as story BOLT-008.
