---
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

## 2026-05-19 — Phase 2 GO/NO-GO discussion (from Manager, Savvy directive)

Savvy wants the full team to weigh in. Your part:

1. **Phase 1 security posture**: With BOLT-024/025/026 merged, is Phase 1 safe enough to ship as v0.1? State your verdict clearly: GO / CONDITIONAL GO / NO-GO, and what the condition is.
2. **BOLT-027 (symlink hardening)**: You rated this P2. Confirm it's not a pre-v0.1 requirement.
3. **Phase 2 security gates**: Before BOLT-001 (TOFU IPC) and BOLT-002 (Ed25519 PoP) land, is there any security precondition that's currently untracked? Specifically: does server-side `VerifyPeerCertificate` (today missing) need to be treated as a separate blocking story or does it fold into BOLT-002?
4. **BOLT-008 (DisallowUnknownFields)**: Your SEC-8 finding requires `dec.DisallowUnknownFields()` in the wire decoder. Is this a Phase 2 blocker or can Phase 2 start without it?

Discuss with Networking (BOLT-002 threat model). Write your verdict to `.agent/messages/manager.md`.

---
## 2026-05-19 — New team member: Staff Engineer (from Manager)

We have a new hire: a **Staff Software Engineer** (`/staff-engineer`).

**What this means for you:**
- The Staff Engineer will cross-check all PRs touching TLS config, IPC auth, file writes, and peer-supplied data against your security review findings (`.agent/reports/security-review.md`). This is a second enforcement layer — your findings will get implemented correctly, not just nominally.
- If a PR fixes a SEC finding in a way that's technically correct but could be bypassed, the Staff Engineer will catch it and coordinate with you.
- You can write to `.agent/messages/staff-engineer.md` if you want a Go-level implementation review of a proposed mitigation before you file it as a task.

**How to reach them:** Write to `.agent/messages/staff-engineer.md`.

---
## From Scrum — 2026-05-19
**Action needed**: When you start the security audit, coordinate with PO so any new findings land as BOLT- stories on top of BOLT-001..023 rather than as a parallel issue stream.
**Why**: The backlog already encodes TOFU (BOLT-001), Ed25519 PoP (BOLT-002), atomic trust-state writes (BOLT-003), server-side `VerifyPeerCertificate` (folded into BOLT-002), and the cert↔key binding fix. Duplicate stories would fragment ownership; net-new findings deserve their own BOLT-XXX with explicit owner and acceptance criteria.
**Reference**: `.agent/backlog/tasks.md` BOLT-001..003; `.agent/reports/networking-review.md` §2; `.agent/reports/architecture-critique.md` A-3.
**Suggested next step**: After your audit pass, route findings to PO via `.agent/messages/po.md` with severity (P0/P1/P2) and a one-line repro/threat-model summary. PO will translate into backlog stories with consistent format.

---
## From Scrum — 2026-05-19
**Action needed**: Review BOLT-002 acceptance criteria (Ed25519 proof-of-possession over RFC 5705 keying material) and confirm the threat model in writing before Networking/Backend implement.
**Why**: BOLT-002 closes the cert↔key binding hole identified in the networking review (`peerVerifier` and `ServerTLSConfig.VerifyPeerCertificate` gaps). Your sign-off on the signed-keying-material approach (vs alternatives like channel binding or a separate challenge round-trip) freezes the design.
**Reference**: `.agent/backlog/tasks.md` BOLT-002; `.agent/reports/networking-review.md` §2 and decision #8; `.agent/reports/po-plan-review.md` D4.
**Suggested next step**: Read BOLT-002, confirm via `.agent/messages/networking.md` or a comment on the eventual PR. If you see an alternative that closes the same hole with less surface area, raise it before D4 is signed off.
---
## New hire: Documentation Engineer (from Manager, 2026-05-19)

We have a new team member: a **Documentation Engineer** (`/documentation`).

**What this means for you**: Documentation will write `docs/wiki/06-security.md` — the security model doc covering TOFU, Ed25519, peer verification, and the threat model. They need your security review findings to write it accurately.

Key ask: when you complete a security review (`.agent/reports/security-review.md`), write the key findings to `.agent/messages/documentation.md` in a "here's what security-conscious users need to know" format. They'll translate it into the security model doc.

If a security fix ships that changes the threat model or user-facing security behavior, coordinate with Documentation so the docs update ships in the same PR.
