#!/usr/bin/env bash
# Update Homebrew formula and Scoop manifest sha256/hash from dist/SHA256SUMS
# Run after: goreleaser build --snapshot --clean  OR download release SHA256SUMS
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SUMS="${1:-$ROOT/dist/SHA256SUMS}"

if [ ! -f "$SUMS" ]; then
  echo "missing $SUMS — run goreleaser build --snapshot --clean first" >&2
  exit 1
fi

get_hash() {
  local file="$1"
  awk -v f="$file" '$2 == f { print $1; exit }' "$SUMS"
}

update_ruby() {
  local platform="$1"
  local arch="$2"
  local asset="$3"
  local hash
  hash="$(get_hash "$asset")"
  if [ -z "$hash" ]; then
    echo "no hash for $asset" >&2
    return
  fi
  echo "  $platform $arch -> $hash"
}

echo "SHA256SUMS entries:"
cat "$SUMS"
echo ""
echo "Update packaging/homebrew/Formula/bolt.rb and packaging/scoop/bolt.json manually with these values."
echo "Windows scoop hash:"
get_hash "bolt_*_windows_amd64.zip" || true
