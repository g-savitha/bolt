# devops Status
**Last Updated**: 2026-05-19
**Status**: Done — dependency-review PR open; `gh` auth still needs Savvy for issue filing
**Current Task**: Monitor `ci/enable-dependency-review` PR checks; BOLT-015 matrix CI when prioritized

## Blockers
- **Soft** — Issue filing for the 16 Ready-for-Dev stories + 10 top QA bugs is staged but cannot run until Savvy re-authenticates `gh` (browser-interactive). See runbook `.agent/reports/devops-runbook.md`.
- **None** for dependency-review — Dependency graph enabled; job restored in CI.

## Recent Output
- **2026-05-19** — Re-enabled `dependency-review` job in `.github/workflows/ci.yml` after Savvy turned on Dependency graph (`g-savitha/bolt` → Settings → Security analysis). PR branch: `ci/enable-dependency-review`.
- `.agent/reports/devops-runbook.md` — operator runbook for Savvy (gh fix + CI gaps + branch protection snippet).
- `.agent/reports/proposed-ci.yml` — BOLT-015 matrix proposal (macOS, Go N-1, concurrency); not merged yet.
- `.agent/reports/file-issues.sh` — dry-run capable; blocked on `gh auth`.

## CI scorecard (live `.github/workflows/ci.yml`)
- `dependency-review` active on PRs (requires Dependency graph — now on).
- Remaining BOLT-015 gaps: no `macos-latest` matrix, no Go N-1, no explicit `go vet` / `go build`, no concurrency group — see `.agent/reports/proposed-ci.yml`.

## Next Steps
- Merge dependency-review PR after green checks.
- Savvy: `gh auth login` then `.agent/reports/file-issues.sh --dry-run` → live.
- DevOps: open BOLT-015 PR from `.agent/reports/proposed-ci.yml` when scheduled.
