## Description
As a maintainer, I want `internal/config` covered by unit tests including migration paths, version-too-new rejection, concurrent `PeerStore.Upsert` under `-race`, and a kill-during-write that asserts atomic-write semantics, so that future config changes do not silently corrupt user state.

## Why
- Plan §"Test strategy" explicitly mandates `internal/config` tests; current coverage is 0%.
- Sources: Backend Phase-2 prereq §H-3, QA coverage gap.

## Acceptance Criteria
- [ ] `LoadOrInit` on a missing file initialises with `Version=1` and defaults; `Stat` confirms `0600` on private files.
- [ ] Migration test: write `Version=0` file, load → `Version=1`, all missing fields filled with defaults.
- [ ] Version-too-new test: write `Version=99` file, load → returns explicit error (does not silently downgrade or overwrite).
- [ ] Concurrent `PeerStore.Upsert` from 100 goroutines under `go test -race`: no race, final state has all 100 entries.
- [ ] Atomic-write kill test (depends on BOLT-003 / #TBD): inject `io.Writer` that fails after N bytes → original `peers.toml` is byte-identical to pre-call state.
- [ ] Per-package coverage on `internal/config` ≥ 70% as measured by `go test -cover`.

## Notes
- Files: `internal/config/config_test.go` (new), `internal/config/peers_test.go` (new).
- Source story: `.agent/backlog/tasks.md` BOLT-011.
