## Description
As a bolt user, I want my daemon to prompt me with the fingerprint + randomart of every unknown peer before any connection completes, so that I can verify peer identity out-of-band and not have my machine silently trust strangers.

Today `peerVerifier` returns `(true, nil)` for any non-blocked fingerprint, so Phase-1 acceptance bullet 4 ("Unknown peer → TOFU prompt with randomart") is unmet and `TrustAllowOnce` collapses to `TrustAlwaysAllow` in practice.

## Why
- Phase 1 acceptance bullet 4 is the security premise of the whole tool.
- Phase 2 file-transfer accept gating reuses the same IPC prompt primitive; blocked until this lands.
- Sources: QA `BUG-1`, Backend review `B-1`, Architect `A-3`, Networking review §2.

## Acceptance Criteria
- [ ] Daemon emits `IPCEvent{Type:"tofu_prompt", Payload:{fingerprint, nickname, randomart, addr, nonce, deadline_utc}}` on first contact from an unknown peer.
- [ ] Randomart is visible in the prompt (rendered via `internal/identity/randomart.go`, same algorithm `bolt id` uses).
- [ ] CLI subscriber reads `y` / `N` / `b` (always / once / block) from user, sends `IPCRequest{Command:"tofu_response", Payload:{nonce, decision}}`; UI defaults to N on Enter.
- [ ] `peerVerifier` blocks on a per-nonce channel up to **30 s**; on timeout returns `(false, "tofu timeout")` — default reject.
- [ ] On accept (`always` or `once`), trust level is persisted via atomic write to `peers.toml` (depends on BOLT-003 / #TBD).
- [ ] Integration test using `internal/testutil/loopback.go` (depends on BOLT-010 / #TBD): first contact triggers prompt, accept-once is honored, second contact from the same peer with `allow-once` re-prompts; `always-allow` peer does not re-prompt; `block` is rejected at TLS time (depends on BOLT-002 / #TBD).

## Notes
- Files: `internal/daemon/connect.go` (replace `peerVerifier` stub), `internal/daemon/daemon.go` (install verifier on inbound too), `internal/proto/ipc.go` (new event + command types — depends on BOLT-009 / #TBD), `cmd/bolt/main.go` (prompt rendering inside `connectCmd` and a subscriber path for inbound).
- Move the prompt to **after** TLS but **before** the bolt-level handshake completes, so the 10 s `HandshakeIdleTimeout` is not the wall clock (`B-1`, `Q-BUG-1` §"Suggested fix").
- Source story: `.agent/backlog/tasks.md` BOLT-001.
