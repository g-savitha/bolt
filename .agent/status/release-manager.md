# Release Manager — Status

**Last updated**: 2026-05-19
**State**: Active

## Latest Release

**v0.1.2** — 2026-05-19
- Tag: pushed to origin ✅
- GitHub release: created ✅
- Team announced: ✅
- CHANGELOG: pending

## Release History

| Version | Date | Type | Summary |
|---------|------|------|---------|
| v0.1.2 | 2026-05-19 | patch | Bug fixes + security hardening (Wave 1 + Tier A) |
| v0.1.1 | 2026-05-17 | patch | Version stamping via ldflags |
| v0.1.0 | 2026-05-17 | minor | Initial Phase 1 release |

## Next Release

**v0.2.0** (Phase 2 Wave 2) — target when BOLT-010, BOLT-011, BOLT-012 land.
Pre-release checklist: `go test -race ./...` ✅ | `govulncheck ./...` ✅ | CI green ✅

## Notes

- v0.1.2 contained all Wave 1 + Tier A bug fixes merged after v0.1.1
- Two known open issues (BUG-1, BUG-4) deferred to Phase 2 by design — documented in release notes
