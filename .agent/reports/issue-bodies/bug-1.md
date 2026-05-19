## Description
`peerVerifier()` (`internal/daemon/connect.go:75-83`) returns `(true, nil)` for any fingerprint that is not on the explicit block list. The same policy runs on inbound (`authenticateIncoming` only rejects if `existing != nil && existing.Trust == TrustBlock`). Any first-contact peer is silently accepted: no `tofu_prompt` IPC event is emitted, no randomart is rendered, the user is never asked. The peer is then persisted to `peers.toml` with `TrustAllowOnce`, which collapses semantically to "always allow" because no subsequent prompt path exists either.

## Steps to Reproduce
1. Build daemon at HEAD.
2. Read `internal/daemon/connect.go:76-83`. The verifier ignores its `fingerprint` argument for any policy decision other than blocklist lookup.
3. Cross-reference `plan.md` §"TOFU fingerprint UX" and Phase 1 verification bullet 4 ("Unknown peer → TOFU prompt with randomart").

## Expected Behavior
On unknown fingerprint, daemon pushes a `tofu_prompt` IPC event with fingerprint + randomart, waits up to 30 s for `tofu_response`, accepts only if user types `y`. Same prompt repeats on every connection from peers with `TrustAllowOnce`.

## Actual Behavior
Accepts silently; `bolt id` is the only place randomart is ever rendered.

## Environment
- `go version go1.25.1 darwin/arm64`
- `quic-go v0.59.1`, `BurntSushi/toml v1.6.0`, `gofrs/flock v0.12.1`
- macOS 25.2.0
- Repo at `e267dd2` ("fix(lint): resolve all golangci-lint issues")

## Suggested Fix
Define `IPCEvent{Type:"tofu_prompt", Payload:{fingerprint,nickname,randomart,addr,nonce}}` + `IPCRequest{Command:"tofu_response", Payload:{nonce, decision}}`. `peerVerifier` blocks on a per-nonce channel with `time.After(30 * time.Second)` returning `(false, "user did not respond")` on timeout. Move the prompt to the **bolt-level** handshake (after TLS, before any other stream) rather than inside `VerifyPeerCertificate` so the synchronous 10s `HandshakeIdleTimeout` is not the wall clock. Tracked as story BOLT-001.
