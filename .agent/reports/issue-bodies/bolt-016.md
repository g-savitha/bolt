## Description
As Savvy, I want the v1 Windows posture committed in one place (code, README, plan, goreleaser), so that contributors do not continue adding Windows-specific code to a project whose plan says "Linux + macOS only".

## Why
- Today `spawn_windows.go`, `ipc_endpoint_windows.go`, the PowerShell installer, and `goreleaser` all build Windows; `plan.md` declares Windows unsupported in v1. Drift will compound if not resolved before Phase 2.
- Source: Architect `A-5`.

## Acceptance Criteria
- [ ] PO writes a 1-page spike summarising Option A (`IPCTransport` interface + keep `*_windows.go`) vs Option B (delete `*_windows.go` + drop PowerShell installer + drop Windows from `.goreleaser.yaml`).
- [ ] Architect drafts ADR-002 reflecting Savvy's chosen option.
- [ ] **If Option B (PO recommendation)**: delete `internal/daemon/spawn_windows.go` and `internal/daemon/ipc_endpoint_windows.go`; remove the Windows install lines from `README.md` (lines 29, 46-54, 66); drop the `windows` entries from `.goreleaser.yaml` and `Makefile release-local`; add `//go:build !windows` build constraint on `cmd/bolt/main.go` only if needed to keep the build clean. `plan.md` line 70-71 stays as-is.
- [ ] **If Option A**: introduce `IPCTransport` interface in `internal/daemon`; route both Unix-socket and Windows-loopback implementations through it; update `plan.md` line 29 + 70-71 to remove "Windows unsupported"; document the Windows install in `README.md` with caveats.
- [ ] `go build ./...` PASS on the chosen target set (Linux + macOS for Option B; +Windows for Option A).
- [ ] `.goreleaser.yaml` and Makefile match the chosen target set.

## Notes
- Files: `internal/daemon/spawn_windows.go`, `internal/daemon/ipc_endpoint_windows.go`, `README.md`, `.goreleaser.yaml`, `Makefile`, `docs/adr/ADR-002-windows-posture.md` (new).
- Source story: `.agent/backlog/tasks.md` BOLT-016.
