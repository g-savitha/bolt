## Description
As a maintainer, I want `cmd/bolt` to import only `internal/proto` and `internal/daemonclient`, so that the CLI binary does not transitively drag in `quic-go`, `chat.Service`, and the daemon's accept loops just to name a command string.

## Why
- Pre-requisite for adding new Phase-2 IPC types (BOLT-001, BOLT-013) in the right package.
- Source: Architect critique `A-2`.

## Acceptance Criteria
- [ ] `IPCRequest`, `IPCResponse`, `IPCEvent`, all payload structs (existing: `StatusPayload`, `ConnectPayload`, `SendChatPayload`, `ChatEventPayload`; new for Phase 2: `SendFilePayload`, `TransferProgressPayload`, `TransferCompletePayload`, `TransferSkippedPayload`, `TOFUPromptPayload`, `TOFUResponsePayload`) live in `internal/proto/ipc.go`.
- [ ] `writeIPCFrame` / `readIPCFrame` move to `internal/proto/ipc.go`.
- [ ] `IPCClient` moves to a new `internal/daemonclient/` package.
- [ ] `cmd/bolt/main.go` imports only `internal/proto` and `internal/daemonclient` (verified by `go list -deps ./cmd/bolt | grep internal/daemon` returning empty).
- [ ] `internal/daemon` retains only `IPCServer`, `acceptLoop`, `dispatch`, `handleConn`, `handleSubscribe`, plus the daemon orchestration.
- [ ] All existing tests still pass; no behaviour change.

## Notes
- Files: `internal/proto/ipc.go` (new — most content moves from `internal/daemon/ipc.go`), `internal/daemonclient/client.go` (new), `internal/daemon/ipc.go` (slim down), `cmd/bolt/main.go` (import updates).
- Source story: `.agent/backlog/tasks.md` BOLT-009.
