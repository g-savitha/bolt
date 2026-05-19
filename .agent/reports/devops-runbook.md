# DevOps Runbook — 2026-05-19

Author: DevOps. Audience: Savvy. Length: one screen of imperative actions, then context.

## What's broken

1. **`gh` auth.** `gh --version` returns `gh version 2.50.0`; install is fine. `gh auth status` returns:
   ```
   X Failed to log in to github.com account g-savitha (keyring)
   - Active account: true
   - The token in keyring is invalid.
   - To re-authenticate, run: gh auth login -h github.com
   ```
   `gh issue list` exits 1 with `HTTP 401: Bad credentials`. The token in the macOS keychain has expired or been revoked. `gh auth refresh` cannot recover it (refresh requires a still-valid token) — full re-login is required.

2. **`.agent/messages/` was empty.** Created the directory floor: 8 role stubs (`architect`, `backend`, `devops`, `manager`, `networking`, `po`, `qa`, `security`) plus a `.gitkeep`, each with a minimal `# <role> — messages` / `No messages yet.` body so other roles can append without a race-y "file does not exist" failure.

3. **CI gaps.** Live `.github/workflows/ci.yml` covers `ubuntu-latest` on a single Go version (read from `go.mod`), no concurrency group, no explicit `go build` / `go vet` step, no `go mod tidy` guard, and no macOS matrix. Full table below.

4. **Drift on Windows posture.** `Makefile release-local` and `.goreleaser.yaml` still build Windows binaries; `plan.md` declares Windows unsupported in v1. Tracked as story BOLT-016 (no action from DevOps until Savvy picks Option A vs B).

## What Savvy needs to do right now (3 commands)

```bash
# 1. Re-authenticate gh — opens a browser, prompts for the device code on stdin.
gh auth login -h github.com --scopes "repo,read:org" --web

# 2. Verify it took.
gh auth status

# 3. File all 16 Ready-for-Dev stories + the top-10 QA bugs as GitHub issues.
.agent/reports/file-issues.sh
```

Expected prompts in step 1:
1. `What is your preferred protocol for Git operations on this host?` → `HTTPS` (or `SSH` if you keep using the SSH remote; either works, gh handles the API over HTTPS regardless).
2. `Authenticate Git with your GitHub credentials?` → `Yes` is fine.
3. `! First copy your one-time code: XXXX-XXXX` followed by `Press Enter to open github.com in your browser...`
4. Browser opens → paste the code → approve the `repo`, `read:org` scopes.
5. Terminal returns `✓ Logged in as g-savitha`.

If the browser flow is awkward (no GUI / SSH), substitute `--with-token` and pipe a PAT created at `https://github.com/settings/tokens` with scopes `repo` and `read:org`.

The minimum scope set the team needs:
- `repo` — `gh issue create`, `gh pr create`, `gh issue comment` (private repos and orgs included).
- `read:org` — `gh issue list --milestone <name>` if any milestone lives at the org level; harmless to include.
- `workflow` — only needed if you later modify `.github/workflows/*.yml` via API. Skip for now.
- `actions:read` (implied by `repo` on personal repos; only an explicit scope on classic PATs) — needed by anyone wanting `gh run list / gh run view`.

The recommended **minimum** scope set: `repo,read:org`. Add `workflow` only when DevOps automation starts editing workflows via API.

Optional Plan A — `gh auth refresh` — only works if the current token is still valid and you want to **add** scopes. With a 401 already, refresh will not recover; jump straight to `gh auth login` above.

## Git remote gotcha

```
origin  git@github.com:g-savitha/bolt.git (fetch)
origin  git@github.com:g-savitha/bolt.git (push)
```

The remote is `g-savitha/bolt` — a personal repo, not `bolt/bolt`. Savvy has full push and issue rights here, so `gh issue create` will land in `g-savitha/bolt`. No fork shenanigans, no upstream-vs-origin confusion. If you ever transfer this repo to a `bolt/` org, the `file-issues.sh` script picks the slug up from `gh repo view`, so no edit is needed.

## CI gap summary

Checklist scored against the live `.github/workflows/ci.yml`. "Severity" reflects what would actually break under Phase 2 once concurrent-write filesystem and platform-conditional code lands.

| # | Requirement | Status | Severity |
|---|---|---|---|
| 1 | Triggers on push-to-main and PR-against-main | OK | — |
| 2 | Go matrix: 1.25.x (stable) + 1.24.x (N-1) | **Gap** — only Go pinned via `go.mod` (single version) | High |
| 3 | OS matrix: `ubuntu-latest` + `macos-latest` | **Gap** — only `ubuntu-latest` | **Critical** (Phase 2 receiver's concurrent `WriteAt`+`Truncate` behaves differently on APFS vs ext4; BOLT-015 acceptance explicitly requires both) |
| 4 | Module cache (`actions/setup-go` `cache: true`) | OK | — |
| 5 | golangci-lint via pinned-SHA official action against repo `.golangci.yml` | OK (v7 @ `9fae48a`, `version: v2.12.2`) | — |
| 6 | Explicit `go vet ./...` step | **Gap** — relies on the `govet` checker inside golangci-lint v2 | Medium (separation of concerns; vet should fail fast before lint) |
| 7 | Explicit `go build ./...` step | **Gap** — `go test -race` builds implicitly | Medium (compile failures show as test failures, not build failures) |
| 8 | `go test -race ./...` | OK | — |
| 9 | `govulncheck ./...` | OK | — |
| 10 | Action versions SHA-pinned | OK (every action has a 40-char SHA + `# vN` comment — strong) | — |
| 11 | Concurrency group cancels superseded runs | **Gap** — no `concurrency:` block | Medium (every push to a branch runs the full matrix; wastes minutes) |
| 12 | `permissions:` block, default `contents: read` | OK | — |
| 13 | `go mod tidy -diff` guard against dirty `go.sum` | **Gap** — Dependabot can land a PR that compiles but leaves `go.sum` dirty | Medium |
| 14 | Required CI checks set as branch protection on `main` | **Cannot verify locally** — `gh api` call below | High (without protection, anyone with push can merge red CI) |

Score: 9 / 14 satisfied. Biggest gap, one-liner: **CI doesn't run on macOS — Phase 2's concurrent file-write code will hit APFS-vs-ext4 differences that today's CI cannot catch.**

To check + set branch protection after `gh` is re-authenticated:

```bash
# View current required-status-checks (empty if no protection set):
gh api repos/g-savitha/bolt/branches/main/protection --jq '.required_status_checks' 2>/dev/null || \
  echo "no branch protection on main"

# Set protection: require the CI matrix jobs and lint to pass, dismiss stale reviews:
gh api --method PUT repos/g-savitha/bolt/branches/main/protection \
  --field required_status_checks[strict]=true \
  --field 'required_status_checks[contexts][]=test (ubuntu-latest, go1.25.x)' \
  --field 'required_status_checks[contexts][]=test (ubuntu-latest, go1.24.x)' \
  --field 'required_status_checks[contexts][]=test (macos-latest, go1.25.x)' \
  --field 'required_status_checks[contexts][]=test (macos-latest, go1.24.x)' \
  --field 'required_status_checks[contexts][]=lint' \
  --field enforce_admins=false \
  --field required_pull_request_reviews[required_approving_review_count]=1 \
  --field required_pull_request_reviews[dismiss_stale_reviews]=true \
  --field restrictions=null
```

Defer until BOLT-015 merges (the check names above match the proposed workflow's job IDs).

## Proposed CI workflow

Reference: `.agent/reports/proposed-ci.yml`. Drop-in replacement for `.github/workflows/ci.yml`. Diff summary, smallest-to-largest:

1. Adds workflow-level `concurrency: { group: ci-${{ github.workflow }}-${{ github.ref }}, cancel-in-progress: true }` — kills superseded runs on the same branch.
2. Adds `strategy.matrix` over `os: [ubuntu-latest, macos-latest]` × `go: [1.25.x, 1.24.x]` (4 jobs).
3. Adds explicit `go build`, `go vet`, and `go mod tidy / git diff --exit-code` steps before the test step.
4. Only the `ubuntu-latest` + Go 1.25.x cell uploads coverage (avoids 4x duplicate artifacts feeding Sonar).
5. `setup-go` switches from `go-version-file: go.mod` to `go-version: ${{ matrix.go }}` with `check-latest: true` (so we always get the current 1.25 / 1.24 patch).
6. Lint / security / dependency-review / sonarcloud jobs unchanged — same SHAs, same flags.

All action SHAs are copied verbatim from the live workflow. No new dependencies; no new secrets.

## Proposed Makefile additions (do not apply without review)

These are not edited by this runbook — they belong in BOLT-015's PR.

```make
# Run the same checks CI runs, in CI's order. Use this before pushing.
ci: build vet lint test

vet:
	go vet ./...

# Coverage gate — advisory now, required after BOLT-010 + BOLT-011.
ci-coverage-gate: test
	@./scripts/coverage-gate.sh 60   # or inline awk — see scripts/
```

Plus an unrelated nit captured for BOLT-016: `release-local` currently builds `windows/amd64` (line 42 of Makefile). When Savvy picks Option B on BOLT-016, drop that line and the corresponding `.goreleaser.yaml` entry in the same PR.

## `.gitignore` review (no change recommended now)

Current `.gitignore` rules:
- Tracks `.agent/` and all subdirs (no rule excludes it). **Correct** — `.agent/` is the team's shared state and should be in git.
- Ignores `golangci-lint` (vendored binary at repo root, ~40 MB). **Correct** — should never be committed.
- Ignores `bolt`, `bolt.exe`, `dist/`, `*.tar.gz`, `*.zip`. **Correct**.
- Ignores `.env`, `*.pem`, `**/relay_secret*`, `**/boltd.toml`, `deploy/*.local.*`. **Correct**.

Potential additions (not applied — flagged for the team):
- `.agent/reports/heartbeat.md` — this file updates every 30 min by the Manager cron. If we want a clean diff stream on PRs, ignore it; if we want a full audit trail, leave tracked. Recommend **leave tracked** for now; Manager can rewrite-in-place rather than appending so noise is bounded.
- `coverage.out` — generated by `make test`. Currently not ignored. Recommend ignoring; produced fresh on every run.

Both deferred until Savvy decides — not gated on Phase 2.

## Future hardening (post-Phase 1)

- **Branch protection.** Required checks (the 4 matrix `test` jobs + `lint`), required PR review = 1, dismiss stale reviews, disallow force push. Use the `gh api` snippet above once BOLT-015's job names land.
- **govulncheck nightly.** Currently runs on every push + PR; cheap. Schedule an additional nightly invocation so newly disclosed CVEs are surfaced before the next PR.
- **Release pipeline via goreleaser.** Already exists at `.github/workflows/release.yml` triggered on `v*` tags. Once BOLT-016 settles Windows, drop the `windows` entries from `.goreleaser.yaml` and bump `v0.1` as Savvy's Phase-1 release.
- **SLSA provenance.** Add `slsa-framework/slsa-github-generator` to the release workflow; produces signed provenance attestations alongside the SHA256SUMS file, raises OpenSSF Scorecard score.
- **`make ci-coverage-gate`.** Advisory floor 60% per non-`cmd` package after BOLT-010 + BOLT-011 land. Flip from advisory to required when transport / daemon / chat / config all clear 60%.
- **Sigstore cosign for container images / artifacts** if we ever ship a relay container.
- **Renovate (or stick with Dependabot).** Dependabot is good enough; only switch if we adopt monorepo-style multi-module workspaces.

## Pointers

| Artifact | Path |
|---|---|
| This runbook | `.agent/reports/devops-runbook.md` |
| Proposed CI workflow | `.agent/reports/proposed-ci.yml` |
| Issue-filing script | `.agent/reports/file-issues.sh` (mode 0755) |
| Issue bodies | `.agent/reports/issue-bodies/` (16 `bolt-*.md` + 10 `bug-*.md`) |
| DevOps status | `.agent/status/devops.md` |
| `.agent/messages/` stubs | `.agent/messages/{architect,backend,devops,manager,networking,po,qa,security}.md` |
