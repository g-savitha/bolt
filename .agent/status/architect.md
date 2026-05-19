# architect Status

**Last Updated**: 2026-05-19
**Status**: Reviewed
**Current Task**: Architecture critique of `architecture.md` vs `plan.md` vs code
**Blockers**: None

## Top 3 P0 issues

1. **[A-1] No `Transport` interface — `quic-go` types leak into `daemon` and `chat`.**
   `internal/chat/service.go` and `internal/daemon/daemon.go` import
   `*quic.Stream` / `*quic.Conn` directly through `transport.PeerConn`.
   Forecloses test-loopback, WebTransport, alternate QUIC stack.
2. **[A-3] TOFU prompt is drawn but not implemented.** `peerVerifier`
   (`internal/daemon/connect.go:75-83`) auto-accepts every non-`block` peer;
   incoming-side never runs a verifier (`daemon.go:154-176`); IPC schema
   (`daemon/ipc.go:12-50`) has no `tofu_prompt` event or `tofu_response`
   command. Diagrams in `architecture.md` lie about a flow that is absent.
3. **[A-5] Windows code exists despite "Windows unsupported in v1".**
   `spawn_windows.go` and `ipc_endpoint_windows.go` ship; `README.md`
   advertises a PowerShell installer. Architecture must either introduce an
   `IPCTransport` interface and own Windows, or delete the Windows code.

Other P0s (full detail in the report): A-2 (IPC types in wrong package),
A-4 (Transport-Layer diagram conflates three responsibilities), A-6
(Discovery subgraph is aspirational), A-7 (no shutdown/lifecycle diagram).

## Full report

`.agent/reports/architecture-critique.md`

## Recent output

Findings: 20 total — 7 P0, 8 P1, 5 P2.
Proposed: 11 ADRs (not created yet — awaiting sign-off).
Proposed: 6 new mermaid diagrams in the report, paste-ready.

## Next steps

1. Get user sign-off on the ADR list (especially ADR-001 transport-interface
   and ADR-002 ipc-transport-abstraction, which are blockers for the
   `internal/discovery/`, `internal/transfer/`, `internal/tui/` work
   currently sitting in empty directories).
2. Pin down ADR-003 (TOFU as IPC round-trip) before any TOFU UI work begins.
3. Decide ADR-005 (per-message v vs handshake capabilities) before bumping
   the wire to v2 for any reason.
4. Once ADRs land, propose a diff against `architecture.md` covering the
   6 new diagrams and the Transport / IPCTransport / Discoverer interface
   view.
