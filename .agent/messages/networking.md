# networking — messages
---
## 🚀 Released: v0.1.2 (from Release Manager)

**Tag**: v0.1.2 | **Date**: 2026-05-19 | **SHA**: ecb8683
**GitHub**: https://github.com/g-savitha/bolt/releases/tag/v0.1.2

**What shipped**:
- 🔒 Go 1.25.10 — closes 15 stdlib vulns (BOLT-024)
- 🔒 TTY sanitization for peer strings (BOLT-025)
- 🔒 Daemon env allowlist — shell secrets no longer inherited (BOLT-026)
- 🐛 Atomic config writes — survives kill-9 (BOLT-003)
- 🐛 Clean daemon shutdown on SIGTERM (BOLT-004)
- 🐛 daemon.pid lifecycle fixed (BOLT-005)
- 🐛 IPC socket chmod 0600, stale socket guard, chat version mismatch, PublishChat lock (BUG-6/7/8/10)
- ✨ Pluggable stream-handler registry (BOLT-006 / BUG-9)

**Still open** (deferred to Phase 2 by design):
- BUG-1 (#23) — TOFU stub, fix is BOLT-001
- BUG-4 (#26) — ChunkMsg base64, fix is BOLT-008

**What's next**: Phase 2 Wave 2 — BOLT-010 (loopback harness) is the next pickup.


---
## 2026-05-19 — Phase 2 GO/NO-GO discussion (from Manager, Savvy directive)

Savvy wants the full team to weigh in. Your part:

1. **Protocol blockers**: Of your 5 Phase 2 prereqs, which must be done *before* Backend writes a single line of sender/receiver code? Which can be done in parallel?
2. **BOLT-007 (quic.Config) vs BOLT-008 (wire schema)**: Can Backend start BOLT-010 (loopback harness) safely before BOLT-007 and BOLT-008 land, or will the harness need to be rewritten when those land?
3. **TOFU plumbing gap**: Server-side `VerifyPeerCertificate` is missing. Is this a Phase 2 blocker or can the TOFU IPC round-trip (BOLT-001) land without it if we gate on BOLT-002 shipping in the same bundle?
4. **D3/D4 decisions** (quic.Config defaults, Ed25519 PoP): If Savvy doesn't lock these this week, what's the maximum amount of Phase 2 work that can proceed safely?

Discuss with Architect (ADR-003) and Security (BOLT-002 threat model sign-off). Write your verdict to `.agent/messages/manager.md`.

---
## 2026-05-19 — New team member: Staff Engineer (from Manager)

We have a new hire: a **Staff Software Engineer** (`/staff-engineer`).

**What this means for you:**
- When networking decisions need to be implemented in Go (BOLT-007 QUIC config locking, BOLT-002 Ed25519 PoP, BOLT-008 wire schema freeze), the Staff Engineer is your implementation partner. Write to `.agent/messages/staff-engineer.md` with the protocol requirement and they'll translate it into idiomatic Go.
- PRs in your domain (transport, TLS, QUIC config) will get a Staff Engineer review focused on Go correctness, with your protocol-level sign-off remaining separate.

**How to reach them:** Write to `.agent/messages/staff-engineer.md`.