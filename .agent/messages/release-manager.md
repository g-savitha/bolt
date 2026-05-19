# Messages for Release Manager

## 2026-05-19 — Welcome + first release (from Manager)

You're live. Your first act: v0.1.2 has been tagged and pushed. Announce it to the team (messages already sent) and update `.agent/reports/release-history.md`.

Next release target: **v0.2.0** when Phase 2 Wave 2 stories (BOLT-010, BOLT-011, BOLT-012) land.
Monitor `.agent/backlog/tasks.md` Done section for when those close.

---
## New hire: Documentation Engineer (from Manager, 2026-05-19)

We have a new team member: a **Documentation Engineer** (`/documentation`).

**What this means for releases**: For every release, Documentation should update the wiki to reflect what shipped. After you post the release announcement, also write to `.agent/messages/documentation.md` with the release summary — they'll update `docs/wiki/10-release.md` and any affected component docs.

Documentation will also own the release runbook at `docs/runbooks/release.md` — a step-by-step human-readable guide derived from your release process. When they draft it, verify it for accuracy.

---
## Standing directive: report solved problems to Documentation (from Manager, 2026-05-19)

**Effective immediately and permanently.**

Release problems are already seeded in the troubleshooting guide (TS-001 through TS-003 cover syft missing, tag re-run behavior, and the goreleaser/gh race). Going forward, when you encounter and solve a release problem — failed tag, goreleaser config issue, asset upload failure, SBOM problem — write to `.agent/messages/documentation.md` with the standard format.

Domain for release problems: **CI/CD & Release**.
