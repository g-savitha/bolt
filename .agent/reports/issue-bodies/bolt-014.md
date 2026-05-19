## Description
As a bolt receiver, I want to reject a transfer with `TransferAck{Accepted:false, Reason:"insufficient_disk"}` before any chunk is written, so that I do not half-write a 10 GiB file onto a 1 GiB-free partition.

## Why
- Required by the receiver accept path (Phase 2 BOLT-013 + downstream receiver work).
- Source: Backend Phase-2 prereq §C-2.

## Acceptance Criteria
- [ ] `internal/transfer/diskspace_unix.go` with build tag `//go:build !windows` declares `FreeBytes(path string) (int64, error)` wrapping `syscall.Statfs`.
- [ ] Returns explicit error on a non-existent path (no zero-int silent failure that the caller might mistake for "no free space").
- [ ] Unit test: `FreeBytes(t.TempDir())` returns a positive int.
- [ ] Unit test: `FreeBytes("/nonexistent/path/that/cannot/exist")` returns a non-nil error.
- [ ] Build-tagged so the rest of the codebase compiles on Linux + macOS regardless of the Windows decision in BOLT-016.

## Notes
- Files: `internal/transfer/diskspace_unix.go` (new), `internal/transfer/diskspace_test.go` (new).
- Source story: `.agent/backlog/tasks.md` BOLT-014.
