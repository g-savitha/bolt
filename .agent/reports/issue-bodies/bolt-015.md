## Description
As a maintainer, I want every PR to be gated by `build + vet + lint + test + test -race` on both `ubuntu-latest` and `macos-latest`, so that platform-specific bugs (especially APFS vs ext4 concurrent-write semantics for Phase 2 receiver) are caught before merge.

## Why
- Plan-mandated CI baseline for Phase 1/2.
- DevOps has drafted the proposed replacement at `.agent/reports/proposed-ci.yml`.
- Source: Backend Phase-2 prereq §I-1.

## Acceptance Criteria
- [ ] `.github/workflows/ci.yml` runs a matrix on `ubuntu-latest` and `macos-latest` with Go 1.25.x (and N-1 `1.24.x`).
- [ ] Steps: `go build ./...`, `go vet ./...`, `golangci-lint run`, `go test ./...`, `go test -race ./...`.
- [ ] GitHub Actions are pinned by SHA (consistent with existing `govulncheck` / `trivy` / `CodeQL` jobs).
- [ ] `go mod tidy -diff` or equivalent guard fails the build on a dirty `go.sum`.
- [ ] Coverage is uploaded as an artifact; a `make ci-coverage-gate` target asserts ≥ 60% per non-`cmd` package (initially advisory; flipped to required after BOLT-010 + BOLT-011 land).
- [ ] Workflow-level `permissions: contents: read`; concurrency group `ci-${{ github.workflow }}-${{ github.ref }}` cancels superseded runs.

## Notes
- Files: `.github/workflows/ci.yml`, `Makefile`.
- DevOps has the proposed YAML ready at `.agent/reports/proposed-ci.yml` — apply it after review.
- Source story: `.agent/backlog/tasks.md` BOLT-015.
