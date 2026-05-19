## Description
As a bolt user, I want my trust state and configuration to survive a crash, kill -9, or power loss, so that a single bad shutdown does not silently re-expose me to TOFU re-acceptance of all peers.

Today `flushLocked` (peers) and `write` (config) open the destination with `O_WRONLY|O_CREATE|O_TRUNC` and stream a TOML encoder directly into the file — no temp file, no `Rename`, no `Sync`.

## Why
- Combined with the TOFU stub (`BOLT-001`), lost trust state silently re-accepts every previously trusted peer.
- Sources: QA `BUG-3`, Backend review `B-3`, Architect `A-19`.

## Acceptance Criteria
- [ ] New helper `writeAtomic(path string, fn func(io.Writer) error) error` in `internal/config/atomic.go`: writes to `path.tmp` in the same directory, `f.Sync()`, `f.Close()`, then `os.Rename(path.tmp, path)`.
- [ ] `PeerStore.flushLocked` (`internal/config/peers.go`) and `Config.write` (`internal/config/config.go`) route through `writeAtomic`.
- [ ] Unit test: inject an `io.Writer` that fails after N bytes → original file is untouched (byte-identical to pre-call state).
- [ ] Kill-during-write test: panic mid-encode → on disk the file is either the previous version or the new version, never empty or half-written.
- [ ] `go test -race ./internal/config/...` passes with concurrent `PeerStore.Upsert` from 100 goroutines.

## Notes
- Files: `internal/config/atomic.go` (new), `internal/config/peers.go`, `internal/config/config.go`.
- Temp file must live in the same directory as the target (otherwise rename crosses filesystems and silently becomes a copy on some FSes).
- Source story: `.agent/backlog/tasks.md` BOLT-003.
