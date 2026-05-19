## Description
As a maintainer, I want `proto.ReadFrame` to never panic regardless of input bytes, so that a malformed or hostile peer cannot crash the daemon by sending crafted framing.

## Why
- Length-prefix framing is a classic crash surface; cheap fuzz target, lifelong payoff.
- Sources: Networking review §3, QA coverage gap.

## Acceptance Criteria
- [ ] `internal/proto/wire_fuzz_test.go` defines `func FuzzReadFrame(f *testing.F)`.
- [ ] Seed corpus includes: empty input, length-prefix = 0, length-prefix > `maxFrameSize`, valid length + truncated payload, valid length + random-byte payload, valid JSON envelope with unknown fields.
- [ ] `ReadFrame` never panics on any input under the default fuzz budget; assertion uses `defer recover()` to fail the test on panic.
- [ ] Explicit unit tests for length-prefix bounds: length = 0 → returns `io.ErrUnexpectedEOF` or equivalent; length = `maxFrameSize + 1` → returns `ErrFrameTooLarge`.
- [ ] CI invocation `go test -fuzz=FuzzReadFrame -fuzztime=30s ./internal/proto/...` exits 0.

## Notes
- Files: `internal/proto/wire_fuzz_test.go` (new).
- Source story: `.agent/backlog/tasks.md` BOLT-012.
