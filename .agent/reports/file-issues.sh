#!/usr/bin/env bash
# file-issues.sh — file the Phase-2 backlog (BOLT-001..016, BOLT-024..027) and
# the top-10 QA bugs (BUG-1..10) as GitHub issues on the repo of the current
# `origin` remote.
#
# Author: DevOps (2026-05-19). BOLT-024..027 appended by Manager (2026-05-19,
# late-arriving security findings SEC-1/2/3/6).
# Source of truth:
#   - Stories: .agent/backlog/tasks.md
#   - Bugs:    .agent/reports/qa-bug-hunt.md
#   - Bodies:  .agent/reports/issue-bodies/{bolt-NNN,bug-N}.md (one file per issue)
#
# Behaviour:
#   - Idempotent: skips any issue whose exact title is already open.
#   - Creates required labels first (idempotent via --force).
#   - --dry-run prints what would be created; never calls `gh issue create`.
#
# Pre-conditions:
#   - `gh auth status` must show authenticated.
#   - `git remote get-url origin` must resolve to a repo the user can write to.
#
# Usage:
#   .agent/reports/file-issues.sh [--dry-run]

set -euo pipefail

# ── Resolve paths ────────────────────────────────────────────────────────────
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BODIES_DIR="${SCRIPT_DIR}/issue-bodies"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

DRY_RUN=0
for arg in "$@"; do
  case "$arg" in
    --dry-run) DRY_RUN=1 ;;
    -h|--help)
      sed -n '2,25p' "${BASH_SOURCE[0]}"
      exit 0
      ;;
    *)
      echo "unknown flag: $arg" >&2
      exit 2
      ;;
  esac
done

# ── Sanity: required tools ───────────────────────────────────────────────────
command -v gh >/dev/null 2>&1 || {
  echo "error: gh CLI not on PATH (install: brew install gh)" >&2
  exit 1
}
command -v git >/dev/null 2>&1 || {
  echo "error: git not on PATH" >&2
  exit 1
}

# ── Sanity: auth ─────────────────────────────────────────────────────────────
if [[ "$DRY_RUN" -eq 0 ]]; then
  if ! gh auth status >/dev/null 2>&1; then
    echo "error: gh is not authenticated. Run:" >&2
    echo "       gh auth login -h github.com --scopes \"repo,read:org\" --web" >&2
    exit 1
  fi
fi

# ── Sanity: repo ─────────────────────────────────────────────────────────────
REPO_SLUG=""
if REPO_SLUG="$(gh repo view --json nameWithOwner --jq .nameWithOwner 2>/dev/null)"; then
  :
else
  # Fall back to parsing origin if gh can't resolve the repo (dry-run path).
  ORIGIN_URL="$(git -C "$REPO_ROOT" remote get-url origin 2>/dev/null || true)"
  case "$ORIGIN_URL" in
    git@github.com:*)   REPO_SLUG="${ORIGIN_URL#git@github.com:}"; REPO_SLUG="${REPO_SLUG%.git}" ;;
    https://github.com/*) REPO_SLUG="${ORIGIN_URL#https://github.com/}"; REPO_SLUG="${REPO_SLUG%.git}" ;;
    *) echo "error: could not derive repo slug from origin (${ORIGIN_URL:-<unset>})" >&2; exit 1 ;;
  esac
fi
echo "repo: ${REPO_SLUG}"
echo "dry-run: $([[ $DRY_RUN -eq 1 ]] && echo yes || echo no)"

# ── Labels (idempotent — gh label create --force is upsert) ──────────────────
declare -a LABELS=(
  "phase-1|#0E8A16|Phase 1 work (identity, daemon, TOFU, wire baseline)"
  "phase-2|#1D76DB|Phase 2 work (file transfer, chunking, resume)"
  "bug|#D73A4A|Defect or regression"
  "tech-debt|#FBCA04|Refactor, scaffolding, or developer-velocity work"
  "feature|#A2EEEF|New user-visible capability"
  "spike|#BFD4F2|Time-boxed investigation"
  "security-critical|#B60205|Security defect — drop everything"
  "security-high|#D93F0B|Security defect — fix this sprint"
  "security-medium|#E99695|Security defect — fix soon"
  "security-low|#F9D0C4|Security hardening — nice to have"
  "priority:P0|#B60205|Critical — blocks the release"
  "priority:P1|#D93F0B|High — must land in current phase"
  "priority:P2|#FBCA04|Medium — schedule normally"
)

create_label() {
  local spec="$1"
  local name color desc
  name="${spec%%|*}"; spec="${spec#*|}"
  color="${spec%%|*}"; spec="${spec#*|}"
  desc="${spec}"
  if [[ "$DRY_RUN" -eq 1 ]]; then
    printf '  [dry-run] gh label create %q --color %s --description %q --force\n' "$name" "${color#\#}" "$desc"
    return 0
  fi
  gh label create "$name" --color "${color#\#}" --description "$desc" --force >/dev/null 2>&1 \
    || gh label edit  "$name" --color "${color#\#}" --description "$desc"   >/dev/null 2>&1 \
    || true
}

echo "ensuring labels exist..."
for spec in "${LABELS[@]}"; do create_label "$spec"; done

# ── Issue list (id|title|labels|body-file) ───────────────────────────────────
# Title format: [<ID>] short title. Labels are comma-separated.
declare -a ISSUES=(
  "BOLT-001|[BOLT-001] TOFU prompt IPC round-trip + randomart + 30s default-reject|phase-2,feature,security-critical,priority:P0|bolt-001.md"
  "BOLT-002|[BOLT-002] Ed25519 proof-of-possession in HandshakeMsg + server-side VerifyPeerCertificate|phase-2,feature,security-critical,priority:P0|bolt-002.md"
  "BOLT-003|[BOLT-003] Atomic-rename writer for peers.toml and config.toml|phase-2,bug,priority:P0|bolt-003.md"
  "BOLT-004|[BOLT-004] Daemon shutdown closes IPC subscriber connections (closes BUG-2)|phase-2,bug,priority:P0|bolt-004.md"
  "BOLT-005|[BOLT-005] daemon.pid symmetric write/remove on clean shutdown|phase-2,bug,priority:P0|bolt-005.md"
  "BOLT-006|[BOLT-006] Stream-handler registry on *Daemon (pluggable StreamType dispatch)|phase-2,tech-debt,priority:P0|bolt-006.md"
  "BOLT-007|[BOLT-007] Lock quic.Config defaults (idle, windows, datagrams, 0-RTT, ALPN)|phase-2,feature,priority:P0|bolt-007.md"
  "BOLT-008|[BOLT-008] Freeze wire schema for Phase 2 (FileHeader, ChunkHeader, TransferAck, TransferDone, CancelMsg)|phase-2,feature,priority:P0|bolt-008.md"
  "BOLT-009|[BOLT-009] Move IPC types to internal/proto + split internal/daemonclient|phase-2,tech-debt,priority:P0|bolt-009.md"
  "BOLT-010|[BOLT-010] Loopback test harness internal/testutil/loopback.go|phase-2,tech-debt,priority:P0|bolt-010.md"
  "BOLT-011|[BOLT-011] Tests for internal/config (load/save/migration + atomic-write kill test)|phase-2,tech-debt,priority:P0|bolt-011.md"
  "BOLT-012|[BOLT-012] Fuzz test for wire framing (FuzzReadFrame) + length-prefix bounds|phase-2,tech-debt,priority:P1|bolt-012.md"
  "BOLT-013|[BOLT-013] internal/transfer package skeleton + Phase-2 IPC types (no behavior yet)|phase-2,tech-debt,priority:P0|bolt-013.md"
  "BOLT-014|[BOLT-014] Disk-space check helper (syscall.Statfs wrapper)|phase-2,feature,priority:P1|bolt-014.md"
  "BOLT-015|[BOLT-015] CI workflow: build + vet + lint + test + race on linux/macOS|phase-2,tech-debt,priority:P1|bolt-015.md"
  "BOLT-016|[BOLT-016] Windows posture decision: remove Windows code OR commit to IPCTransport interface|phase-2,spike,priority:P1|bolt-016.md"
  "BUG-1|[BUG-1] TOFU verification is a stub — every unknown peer silently accepted|bug,security-critical,priority:P0|bug-1.md"
  "BUG-2|[BUG-2] Daemon shutdown hangs forever when a CLI subscriber is connected|bug,priority:P0|bug-2.md"
  "BUG-3|[BUG-3] peers.toml and config.toml writes are non-atomic — crash mid-write erases trust state|bug,priority:P0|bug-3.md"
  "BUG-4|[BUG-4] ChunkMsg base64 JSON encoding breaches 8MB frame cap on legitimate 4MB chunks|bug,priority:P0|bug-4.md"
  "BUG-5|[BUG-5] daemon.pid is never removed on clean shutdown — false-positive liveness|bug,priority:P1|bug-5.md"
  "BUG-6|[BUG-6] Unix domain socket inherits process umask — typically world-traversable|bug,security-medium,priority:P1|bug-6.md"
  "BUG-7|[BUG-7] listenIPC unconditionally removes existing socket — second bolt daemon orphans the running one|bug,priority:P1|bug-7.md"
  "BUG-8|[BUG-8] chat.AttachStream silently drops version-mismatch messages in a tight loop|bug,security-medium,priority:P1|bug-8.md"
  "BUG-9|[BUG-9] Stream router silently drops StreamFile, StreamControl, and duplicate StreamHandshake|bug,priority:P1|bug-9.md"
  "BUG-10|[BUG-10] IPCServer.PublishChat holds subsMu during writes — one slow subscriber blocks all chat fan-out|bug,priority:P1|bug-10.md"
  # ── Late security additions (SEC-1/2/3/6 — 2026-05-19, post-PO backlog) ────
  "BOLT-024|[BOLT-024] Bump Go toolchain to 1.25.10 to close 15 reachable stdlib advisories|phase-2,bug,security-critical,priority:P0|bolt-024.md"
  "BOLT-025|[BOLT-025] Sanitize peer-controlled strings before TTY render / persistence|phase-2,bug,security-high,priority:P1|bolt-025.md"
  "BOLT-026|[BOLT-026] Restrict spawned daemon environment to an explicit allowlist|phase-2,bug,security-high,priority:P1|bolt-026.md"
  "BOLT-027|[BOLT-027] Refuse insecure config-dir permissions; O_NOFOLLOW + O_EXCL on first writes|phase-2,bug,security-medium,priority:P2|bolt-027.md"
)

# ── Idempotency: does an open issue with this exact title already exist? ─────
title_already_open() {
  local title="$1"
  if [[ "$DRY_RUN" -eq 1 ]]; then
    # Even in dry-run, query if auth is healthy so we report accurately.
    if ! gh auth status >/dev/null 2>&1; then
      return 1
    fi
  fi
  # gh issue list --search '"<title>" in:title' returns matching open issues.
  # We do an exact-title comparison on the JSON result to avoid substring matches.
  local hits
  hits="$(gh issue list \
            --state open \
            --search "\"${title}\" in:title" \
            --json title \
            --jq ".[] | select(.title == \"${title}\") | .title" \
            2>/dev/null || true)"
  [[ -n "$hits" ]]
}

# ── Create one issue ─────────────────────────────────────────────────────────
file_issue() {
  local id="$1" title="$2" labels="$3" body_file="$4"
  local body_path="${BODIES_DIR}/${body_file}"

  if [[ ! -f "$body_path" ]]; then
    echo "  [skip] ${id} — body file missing: ${body_path}" >&2
    return 1
  fi

  if title_already_open "$title"; then
    echo "  [skip] ${id} — already open: ${title}"
    return 0
  fi

  if [[ "$DRY_RUN" -eq 1 ]]; then
    printf '  [dry-run] gh issue create --title %q --label %s --body-file %q\n' \
      "$title" "$labels" "$body_path"
    return 0
  fi

  local url
  url="$(gh issue create \
          --title "$title" \
          --label "$labels" \
          --body-file "$body_path" 2>&1)"
  echo "  [ok]   ${id} -> ${url}"
}

# ── Drive ────────────────────────────────────────────────────────────────────
echo "filing ${#ISSUES[@]} issues..."
for row in "${ISSUES[@]}"; do
  IFS='|' read -r id title labels body_file <<<"$row"
  file_issue "$id" "$title" "$labels" "$body_file"
done

echo "done."
