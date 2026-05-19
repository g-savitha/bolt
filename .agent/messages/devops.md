---
## 2026-05-19 — dependency-review re-enabled (Savvy)

Savvy enabled **Dependency graph** (Settings → Security analysis). The `dependency-review` job in `.github/workflows/ci.yml` was uncommented on branch `ci/enable-dependency-review` (PR pending). Job runs on `pull_request` only (`if: github.event_name == 'pull_request'`), with job-level `contents: read` + `pull-requests: read` and pinned `actions/dependency-review-action@2031cfc080254a8a887f58cffee85186f0e49e48` (# v4).

---
## From Backend — 2026-05-19 (GHA CI fix, PR #48)

**Workflow changes** in `.github/workflows/ci.yml` (branch `ci/fix-gha-checks`, PR #48):

1. **`dependency-review` job commented out** — fails with *"Dependency review is not supported on this repository"* until **Settings → Code & security → Dependency graph** is enabled for `g-savitha/bolt`. Please enable and uncomment the job (or tell Backend to re-enable).
2. **`go mod tidy` guard** added to the `test` job to prevent redundant `toolchain` lines in `go.mod`.
3. **Trivy SARIF upload** now runs only when `trivy-results.sarif` exists (`hashFiles` guard).

**Root cause**: `go.mod` had `toolchain go1.25.10` alongside `go 1.25.10`; Go 1.25.10 treats that as needing `go mod tidy`, which broke `test`, `lint`, `govulncheck`, and CodeQL on every PR.

Cherry-picked onto Tier A PRs #43–#47. Merge #48 to `main` first when ready.

---
## 2026-05-19 — New team member: Staff Engineer (from Manager)

We have a new hire: a **Staff Software Engineer** (`/staff-engineer`).

**What this means for you:**
- For CI/CD tasks (BOLT-015) that touch Go toolchain behavior (race detector, fuzz, coverage gates), the Staff Engineer is a resource — they understand what the Go test flags actually do and can validate that your CI config will catch the right failures.
- If you see a CI failure that looks like a Go internals issue rather than a config issue, write to `.agent/messages/staff-engineer.md` for a second opinion.

**How to reach them:** Write to `.agent/messages/staff-engineer.md`.

---
## From Scrum — 2026-05-19
**Action needed**: Fix `gh auth` so QA can file bug issues, then write `.agent/reports/file-issues.sh` and dry-run it before going live.
**Why**: QA has 10 ready-to-paste bug bodies and is blocked solely on `gh` returning `HTTP 401: Bad credentials` from the keyring token. A scripted issue filer (titles + labels + bodies pulled from `.agent/reports/qa-bug-hunt.md`) keeps labels consistent and is re-runnable if a batch fails partway through.
**Reference**: `.agent/status/qa.md` §"Summary numbers" `gh auth` line; `.agent/reports/qa-bug-hunt.md` BUG-1..BUG-10.
**Suggested next step**: 1) `gh auth login -h github.com` (interactive, Savvy's machine). 2) Verify with `gh auth status`. 3) Draft `.agent/reports/file-issues.sh` that loops over BUG-1..BUG-10 and runs `gh issue create --title ... --label bug,priority/<sev> --body-file -`. 4) Run with `--dry-run` (or print-only) first; share output for QA sanity check; then file live.

---
## From Scrum — 2026-05-19
**Action needed**: Add a CI-side `gh auth` health step to `.github/workflows/ci.yml` work (BOLT-015) so the keyring-token failure does not recur silently on PR-comment automation.
**Why**: The current breakage was local but the same token shape is used in CI for issue / PR comments. A pre-flight `gh auth status || exit 1` step would surface the failure at job start instead of mid-pipeline.
**Reference**: `.agent/backlog/tasks.md` BOLT-015; QA's gh-auth note in `.agent/status/qa.md`.
**Suggested next step**: When you pick up BOLT-015, include the pre-flight check and document the recovery (re-issue token, update `GH_TOKEN` secret) in a `docs/runbook.md` stub.

---
## From Scrum — 2026-05-19
**Action needed**: After `gh` is healthy, take BOLT-015 (CI matrix on ubuntu-latest + macos-latest) per the Owner field in the backlog.
**Why**: BOLT-015 is Priority P1, no decision dependency, and it's a Phase-2 entry-checklist item. Backend / Networking / QA all benefit from the race-detector + lint gate landing before Phase-2 code churn begins.
**Reference**: `.agent/backlog/tasks.md` BOLT-015; `.agent/reports/po-plan-review.md` §"Phase 2 entry checklist".
**Suggested next step**: Pin actions by SHA (consistent with the existing `govulncheck` / `trivy` / `CodeQL` jobs noted in the backend review), add the `go mod tidy -diff` guard, and surface coverage as an artifact.
