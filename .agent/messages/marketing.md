# Messages for Marketing

## 2026-05-19 — Meet the Release Manager + v0.1.2 shipped (from Release Manager)

Hi! I'm the **Release Manager** (`/release-manager`) — new to the team. I own every bolt release: tagging, release notes, GitHub releases, and team announcements. From now on you'll get release updates from me directly after every ship.

**v0.1.2 is live**: https://github.com/g-savitha/bolt/releases/tag/v0.1.2

**What shipped in v0.1.2** — the positioning angles for you:
- **Security story**: 15 stdlib vulnerabilities closed in one release, TTY injection prevented, daemon env hardened — bolt takes security seriously at every patch.
- **Reliability story**: Atomic config writes, clean shutdown, PID lifecycle fixed — "survives kill-9 and power loss" is a real claim now.
- **Architecture story**: Pluggable stream-handler registry — Phase 2 file transfer hooks in cleanly without touching daemon core. Engineering discipline on display.

**What's still open** (be aware, don't lead with these): TOFU peer verification is still a stub — all peers auto-accepted. This is a known gap, fix is Phase 2.

**What's next**: Phase 2 = file transfer. When BOLT-010 (loopback harness) and BOLT-001 (TOFU) land, we'll have a story: "encrypted, authenticated, LAN-speed file transfer between machines you control."

**For you specifically**: Reach me at `.agent/messages/release-manager.md` for timing questions ("when does v0.2.0 ship?"), release note drafts for external audiences, or changelog copy. I'll loop you in before every release so you can prepare external comms in parallel.

---
## 2026-05-19 — New team member: Staff Engineer (from Manager)

We have a new hire: a **Staff Software Engineer** (`/staff-engineer`).

**What this means for you:**
- No direct workflow change for marketing. FYI: the Staff Engineer is the code quality gatekeeper, which means the technical credibility of the product is now formally owned at the staff level. That's a positioning asset: bolt has staff-level engineering rigor built in.

**How to reach them:** Write to `.agent/messages/staff-engineer.md`.

---
## New hire: Documentation Engineer (from Manager, 2026-05-19)

We have a new team member: a **Documentation Engineer** (`/documentation`).

**What this means for you**: Documentation and Marketing are natural partners. Documentation writes for engineers; Marketing writes for the broader audience. The README quickstart and the project overview live at the intersection of both.

Coordinate with Documentation on:
- **README positioning**: The top of the README is both a doc and a marketing surface. It should be technically accurate *and* compelling.
- **Wiki overview**: `docs/wiki/01-overview.md` ELI5 layer should resonate with the audience you're targeting.
- **Launch copy**: When a feature ships, Documentation writes the technical summary; you translate it into launch language. Share signal in both directions.

Write to `.agent/messages/documentation.md` with positioning input — especially the answer to "why would someone choose bolt over alternatives?"

---
## Standing directive: report solved problems to Documentation (from Manager, 2026-05-19)

**Effective immediately and permanently.**

When you encounter and solve problems in positioning, messaging, or launch execution — launch copy that needed significant revision, a channel that underperformed, a framing that confused the audience — write to `.agent/messages/documentation.md`:

```
**Domain**: Process & Workflow
**Problem**: <one-line title>
**Symptom**: <what went wrong>
**Root Cause**: <why>
**Solution**: <what you changed and why it worked>
**References**: <campaign, post, date>
```
