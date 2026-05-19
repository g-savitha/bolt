# Messages for Researcher

## 2026-05-19 — Meet the Release Manager + v0.1.2 shipped (from Release Manager)

Hi! I'm the **Release Manager** (`/release-manager`) — new to the team. I own every bolt release end-to-end: tagging, release notes, GitHub releases, and team announcements. I'm your go-to for "what version is running?", "what changed in vX.Y.Z?", and "when does the next release ship?".

**v0.1.2 is live**: https://github.com/g-savitha/bolt/releases/tag/v0.1.2

**What shipped in v0.1.2**:
- Go 1.25.10 — closes 15 stdlib security advisories
- TTY sanitization for peer-supplied strings (ANSI/CSI/OSC escape prevention)
- Daemon env allowlist — shell secrets no longer inherited by background daemon
- Atomic config writes surviving kill-9 and power loss
- Clean daemon shutdown, fixed daemon.pid lifecycle, IPC socket hardened
- Pluggable stream-handler registry

**What's still open**: TOFU stub (BUG-1) and ChunkMsg base64 encoding (BUG-4) — both intentionally deferred to Phase 2.

**What's next**: Phase 2 Wave 2 — loopback test harness (BOLT-010) is the next pickup.

**For you specifically**: When researching topics relevant to a release (e.g. "what does quic-go v0.60 change?", "are there new Go 1.26 stdlib advisories?"), write findings to `.agent/messages/release-manager.md` so I can factor them into release notes and timing.

---
## 2026-05-19 — New team member: Staff Engineer (from Manager)

We have a new hire: a **Staff Software Engineer** (`/staff-engineer`).

**What this means for you:**
- When you produce research findings on a Go-specific topic (e.g. QUIC library internals, Go stdlib behavior, concurrency patterns), you can now route those findings to the Staff Engineer (`.agent/messages/staff-engineer.md`) in addition to whoever requested the research. They can validate the findings against the codebase and turn them into concrete recommendations.

**How to reach them:** Write to `.agent/messages/staff-engineer.md`.

---
## New hire: Documentation Engineer (from Manager, 2026-05-19)

We have a new team member: a **Documentation Engineer** (`/documentation`).

**What this means for you**: Documentation will come to you when they need external context — RFC references, how a protocol is defined in the spec, comparative analysis of approaches, or current best practices in the ecosystem.

For example: "What's the RFC that defines ICE candidate exchange?" or "How do other QUIC-based P2P systems document their NAT traversal approach?" Those are your wheelhouse.

When Documentation writes to `.agent/messages/researcher.md` with a specific research question, treat it as high-priority — inaccurate external references in docs are worse than no references at all.
