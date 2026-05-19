## Description
As a bolt user, I want every peer to cryptographically prove it holds the Ed25519 private key matching its fingerprint, so that an attacker who copies my public key cannot impersonate me on the LAN.

## Why
- Closes the "stolen pubkey → impersonation" hole left open by today's TOFU verifier.
- Lock the exporter-based proof-of-possession before Phase 2 streams start carrying file transfers.
- Sources: Architect `A-3`, Networking review §2 / decision #8.

## Acceptance Criteria
- [ ] `HandshakeMsg` gains a `Signature []byte` field; each side signs `tls.ConnectionState.ExportKeyingMaterial("bolt-handshake", nil, 32)` (RFC 5705) with its Ed25519 private key.
- [ ] Receiver verifies `Signature` against the public key extracted from `SubjectKeyId`; on mismatch, close the stream with application error code `0x1001` and reject the connection.
- [ ] `ServerTLSConfig.VerifyPeerCertificate` is installed with the same TOFU verifier as `ClientTLSConfig` (today server side has none — the blocklist check happens post-handshake).
- [ ] `tls.Config.SessionTicketsDisabled = true` on both client and server configs (explicit, documented).
- [ ] Unit test: forge a cert with a stolen Ed25519 pubkey but a different Ed25519 priv → handshake fails with code `0x1001`.
- [ ] Wire round-trip test: `HandshakeMsg{Signature: ...}` marshals/unmarshals through `WriteFrame`/`ReadFrame` without loss.

## Notes
- Files: `internal/proto/wire.go` (HandshakeMsg field — additive, stays `v=1`), `internal/transport/tls.go` (server-side `VerifyPeerCertificate`), `internal/transport/peer_conn.go` (sign on send, verify on receive), `internal/identity/identity.go` (`Identity.Sign(data)` helper if not present).
- Source story: `.agent/backlog/tasks.md` BOLT-002.
