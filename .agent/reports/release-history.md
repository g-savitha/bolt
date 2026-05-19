# bolt — Release History

Maintained by: Release Manager (`/release-manager`)

---

## v0.1.2 — Bug fixes and security hardening
**Date**: 2026-05-19
**SHA**: cb4f8a7 (main)
**Tagged by**: Release Manager
**GHA run**: https://github.com/g-savitha/bolt/actions/runs/26094684633 ✅
**GitHub**: https://github.com/g-savitha/bolt/releases/tag/v0.1.2

### Included PRs
| PR | Story | Summary |
|----|-------|---------|
| #37 | BOLT-024 | Go toolchain → 1.25.10 (15 stdlib vulns) |
| #38 | BOLT-003 | Atomic-rename writes for peers.toml / config.toml |
| #39 | BOLT-004 | Daemon shutdown closes IPC subscribers |
| #40 | BOLT-005 | daemon.pid symmetric write/remove |
| #41 | BOLT-025 | Sanitize peer strings before TTY |
| #42 | BOLT-026 | Daemon env allowlist |
| #43 | BUG-6 | Unix socket chmod 0600 |
| #44 | BUG-7 | listenIPC dial-before-remove guard |
| #45 | BUG-8 | Chat stream closed on version mismatch |
| #46 | BUG-10 | PublishChat lock scope fix |
| #47 | BUG-9 / BOLT-006 | Stream-handler registry |
| #48 | — | GHA CI fixes (tidy guard, Trivy SARIF, golangci-lint v2) |
| #49 | — | dependency-review re-enabled |

### Known open at release time
- BUG-1 (#23) — TOFU stub (Phase 2, BOLT-001)
- BUG-4 (#26) — ChunkMsg base64 (Phase 2, BOLT-008)
- BOLT-027 (#36) — symlink hardening (Phase 2, P2)

---

## v0.1.1 — Version stamping
**Date**: 2026-05-17
**Tagged by**: Savvy
**Notes**: Stamp version into binary at build time via ldflags

---

## v0.1.0 — Initial Phase 1 release
**Date**: 2026-05-17
**Tagged by**: Savvy
**Notes**: Rename project, initial v1 release
