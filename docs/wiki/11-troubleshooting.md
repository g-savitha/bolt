# Troubleshooting Guide

> **About this document**: This is a living reference. Every agent on the bolt team has a standing directive to report problems they solve to the Documentation Engineer. Entries are added here and indexed by domain so any engineer can quickly find and apply a known fix.
>
> Entries are numbered globally (TS-001, TS-002, …) and never renumbered or removed.

---

## Table of Contents

- [CI/CD \& Release](#cicd--release-devops-release-manager)
- [Go Build \& Toolchain](#go-build--toolchain-backend-staff-engineer)
- [Networking \& QUIC](#networking--quic-networking)
- [Security](#security-security)
- [IPC \& Daemon](#ipc--daemon-backend-architect)
- [Testing \& QA](#testing--qa-qa)
- [Process \& Workflow](#process--workflow-scrum-manager-po)

---

## CI/CD & Release (DevOps, Release Manager)

### [TS-001] goreleaser produces no binary assets — syft not installed on runner
**Reported by**: Release Manager / DevOps | **Date**: 2026-05-19 | **Version**: v0.1.2

**Symptom**
```
exec: "syft": executable file not found in $PATH
```
goreleaser exits non-zero. The GitHub release is created (or pre-existed) but contains zero binary assets — no `.tar.gz`, no `.zip`, no `.sbom` files, no `SHA256SUMS`.

**Root Cause**
`.goreleaser.yaml` includes an `sboms` section that calls `syft` to generate a Software Bill of Materials for each binary. goreleaser v2 does **not** auto-install `syft` — it assumes the binary is already on `$PATH`. The `ubuntu-latest` GitHub Actions runner does not include `syft` by default.

**Solution**
Add a step to install `syft` via its official install script **before** the goreleaser step in `.github/workflows/release.yml`:

```yaml
- name: Install syft (for SBOM generation)
  run: curl -sSfL https://raw.githubusercontent.com/anchore/syft/main/install.sh | sh -s -- -b /usr/local/bin

- uses: goreleaser/goreleaser-action@<SHA>
  with:
    version: "~> v2"
    args: release --clean
  env:
    GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

Do **not** use the `anchore/sbom-action/download-syft` action — the pinned SHA must be verified against the action's actual release SHAs, and an incorrect SHA causes a hard action-not-found failure.

**Prevention**
Whenever the `.goreleaser.yaml` `sboms` section is modified or syft is updated, verify the install step is present in `release.yml` before merging. The `make release-local` target will also fail with the same error locally if `syft` is not in `$PATH` — run `which syft` first.

**See Also**
- [Release process](10-release.md)
- PR #48 (CI fixes), commit `cb4f8a7` (syft curl fix)

---

### [TS-002] Workflow fix on `main` not picked up by a re-run of a failed release
**Reported by**: Release Manager | **Date**: 2026-05-19 | **Version**: v0.1.2

**Symptom**
You push a fix to `.github/workflows/release.yml` on `main`, then click "Re-run failed jobs" on the failed release workflow run. The re-run still uses the old (broken) workflow — the fix has no effect.

**Root Cause**
GitHub Actions workflow re-runs use the workflow file **at the commit the original run was triggered from** — which is the tag's commit, not `main`. When you re-run a job triggered by `push: tags: v*`, the runner loads `release.yml` from the tagged commit's tree, not from the current `main`. Changes pushed to `main` after the tag was created are invisible to re-runs of that tag's workflow.

**Solution**
1. Delete the tag locally and on origin:
   ```bash
   git tag -d vX.Y.Z
   git push origin --delete vX.Y.Z
   ```
2. Push the workflow fix to `main` and confirm it is on `HEAD`:
   ```bash
   git log --oneline -3
   ```
3. Re-tag on the new `HEAD` of `main`:
   ```bash
   git tag -a vX.Y.Z -m "vX.Y.Z — <summary>" HEAD
   git push origin vX.Y.Z
   ```
4. The new tag push triggers a fresh workflow run using the corrected `release.yml`.

**Prevention**
Always tag **after** any workflow changes are merged to `main`. Never rely on re-runs to pick up workflow file changes — they won't.

**See Also**
- [TS-001] — typically the root cause that requires this re-tag procedure
- [Release process](10-release.md)

---

### [TS-003] goreleaser and `gh release create` race — release assets in inconsistent state
**Reported by**: Release Manager | **Date**: 2026-05-19 | **Version**: v0.1.2

**Symptom**
The GitHub release exists and has a description, but binary assets appear/disappear or are partially uploaded. Running `gh release view vX.Y.Z --json assets` shows some assets but not all, or the asset list is empty and then fills in minutes later.

**Root Cause**
`gh release create` was run manually **before** goreleaser finished (or at all). goreleaser v2 creates the GitHub release itself as part of `goreleaser release --clean`. Running both introduces a race: goreleaser may find the release pre-existing and update it, but asset upload ordering and release metadata can end up in an inconsistent intermediate state visible to users.

**Solution**
Delete the manually created release, then let goreleaser re-run (or re-tag to trigger a fresh run):

```bash
gh release delete vX.Y.Z --yes
# Then either re-run via re-tag (see TS-002) or wait for goreleaser to finish if still running
```

**Prevention**
**Never run `gh release create` for a version managed by goreleaser.** Goreleaser owns the GitHub release lifecycle end-to-end. The release manager skill enforces this.

**See Also**
- [TS-002] — re-tag procedure

---

### [TS-004] `go mod tidy` breaks CI — redundant `toolchain` line in `go.mod`
**Reported by**: Backend / DevOps | **Date**: 2026-05-19

**Symptom**
CI `test`, `lint`, `govulncheck`, and CodeQL jobs all fail with:
```
go: updates to go.mod needed; to update it, run:
    go mod tidy
```
But running `go mod tidy` locally produces no diff. The `go.mod` has both:
```
go 1.25.10
toolchain go1.25.10
```

**Root Cause**
When `go.mod` specifies `go 1.25.10` and `toolchain go1.25.10` with the same version, Go 1.25.10 considers this redundant and marks the module as needing `go mod tidy`. CI's `go mod tidy -diff` check (or equivalent) exits non-zero because the toolchain line is considered surplus.

**Solution**
Remove the redundant `toolchain` directive — the `go` line is sufficient:
```
go 1.25.10
# Remove: toolchain go1.25.10
```
Or keep the `toolchain` line only if it specifies a **different** version than the `go` line (e.g., `go 1.25` with `toolchain go1.25.10`).

**Prevention**
The `make pre-pr` target runs `go mod tidy && git diff --exit-code -- go.mod go.sum` before any PR. This catches the issue locally. The CI `test` job also has a `go mod tidy` guard — if it fires on CI but not locally, check that your local Go version matches `go.mod`.

**See Also**
- PR #48 (tidy guard added to CI), [Development guide](09-development.md)

---

### [TS-005] `dependency-review` GHA job fails — "not supported on this repository"
**Reported by**: DevOps / Backend | **Date**: 2026-05-19

**Symptom**
```
Error: Dependency review is not supported on this repository.
Ensure that Dependency graph is enabled.
```
The `dependency-review` job in `.github/workflows/ci.yml` fails on every PR.

**Root Cause**
The `actions/dependency-review-action` requires the **Dependency graph** feature to be enabled in the repository's security settings. On new or forked repositories it is off by default.

**Solution**
Enable it in GitHub: **Settings → Security analysis → Dependency graph → Enable**.

After enabling, the `dependency-review` job will pass on the next PR run without any code changes.

**Prevention**
This is a one-time repository setup step. Documented in the repo setup checklist. If re-encountered after a repo transfer or fork, check the setting first before touching the workflow.

**See Also**
- PR #49 (dependency-review re-enabled after Dependency graph was turned on)

---

## Go Build & Toolchain (Backend, Staff Engineer)

*No entries yet. When Backend or Staff Engineer solves a Go build or toolchain problem, they write it to `.agent/messages/documentation.md` and it is added here.*

---

## Networking & QUIC (Networking)

*No entries yet.*

---

## Security (Security)

*No entries yet.*

---

## IPC & Daemon (Backend, Architect)

*No entries yet. Known past issues (IPC socket lifecycle, Unix socket permissions) have been fixed in v0.1.2 — see [Daemon docs](03-daemon.md) when available for the correct behavior.*

---

## Testing & QA (QA)

### [TS-006] `gh` CLI returns HTTP 401 — stale keyring token
**Reported by**: QA / Scrum | **Date**: 2026-05-19

**Symptom**
```
HTTP 401: Bad credentials (https://api.github.com/...)
```
`gh issue create`, `gh pr list`, and other `gh` commands fail. `gh auth status` reports a token stored in the keyring that is no longer valid.

**Root Cause**
The token stored by `gh auth login` has expired or been revoked. The CLI reads from the system keyring and does not automatically refresh.

**Solution**
Re-authenticate:
```bash
gh auth login -h github.com    # interactive — choose HTTPS + browser or token
gh auth status                 # verify: should show "Logged in to github.com"
gh api user                    # smoke test: should return your user JSON
```

**Prevention**
Use a GitHub PAT with appropriate scopes (`repo`, `read:org`) and set a long expiry. Store it as `GH_TOKEN` in the environment or CI secrets rather than relying on the keyring. For CI automation, use `secrets.GITHUB_TOKEN` (auto-provisioned per run).

**See Also**
- [Process & Workflow → running QA scripts](#process--workflow-scrum-manager-po)

---

## Process & Workflow (Scrum, Manager, PO)

*No entries yet.*

---

## How to Add an Entry

Any agent can contribute a troubleshooting entry. Write to `.agent/messages/documentation.md` with:

```
**Domain**: <section name from Table of Contents>
**Problem**: <one-line title>
**Symptom**: <exact error or observable failure>
**Root Cause**: <why it happens>
**Solution**: <numbered steps with exact commands>
**References**: <PR #NNN, file paths, issue links>
```

The Documentation Engineer will assign a TS-NNN number, format the entry, verify accuracy, and add it to the appropriate section.

**Do not self-edit this file.** All entries go through Documentation to maintain formatting consistency and prevent numbering gaps.
