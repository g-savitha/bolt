# backend Status
**Last Updated**: 2026-05-19
**Status**: GHA CI fix shipped; Tier A PRs updated
**Current Task**: Wait for CI green on #43–#48; Tier B blocked on Savvy D1/D10
**Blockers**: None for CI; `dependency-review` disabled until DevOps enables Dependency graph

## CI failure root cause (fixed)

| Check | Failure | Fix |
|-------|---------|-----|
| test, lint, govulncheck, CodeQL analyze | `go: updates to go.mod needed` | Removed redundant `toolchain go1.25.10` from `go.mod` |
| dependency-review | Dependency graph not enabled on repo | Job commented out in `ci.yml` (PR #48) |
| security (SARIF upload) | `trivy-results.sarif` missing after earlier step failed | `hashFiles` guard on upload step |

**CI fix PR**: https://github.com/g-savitha/bolt/pull/48 (`ci/fix-gha-checks`)

Cherry-picked `4a03c21` onto all Tier A branches (#43–#47).

## Tier A PRs

| PR | Branch | CI fix pushed |
|----|--------|---------------|
| #43 | `fix/bug-6-ipc-chmod` | yes |
| #44 | `fix/bug-7-ipc-socket-guard` | yes |
| #45 | `fix/bug-8-chat-version` | yes |
| #46 | `fix/bug-10-publish-chat-lock` | yes |
| #47 | `fix/bolt-006-stream-registry` | yes |

Re-check with: `gh pr checks 43` … `48`

## Links

- CI checklist: [.agent/messages/backend.md](../messages/backend.md)
- DevOps note: [.agent/messages/devops.md](../messages/devops.md)
