#!/usr/bin/env bash
# Install bolt from GitHub Releases.
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/g-savitha/bolt/main/scripts/install.sh | bash
#   ./scripts/install.sh [version]   # default: latest
set -euo pipefail

OWNER="g-savitha"
REPO="bolt"

# Resolve version: explicit arg > env var > latest release from GitHub API
if [ "${1:-}" != "" ]; then
  VERSION="$1"
elif [ "${BOLT_VERSION:-}" != "" ]; then
  VERSION="$BOLT_VERSION"
else
  VERSION="$(curl -fsSL "https://api.github.com/repos/${OWNER}/${REPO}/releases/latest" \
    | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"v\([^"]*\)".*/\1/')"
  if [ -z "$VERSION" ]; then
    echo "error: could not resolve latest release from GitHub API" >&2
    exit 1
  fi
fi
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch="$(uname -m)"
case "$arch" in
  x86_64|amd64) arch="amd64" ;;
  arm64|aarch64) arch="arm64" ;;
  *)
    echo "unsupported architecture: $arch" >&2
    exit 1
    ;;
esac

case "$os" in
  darwin) ;;
  linux) ;;
  *)
    echo "use scripts/install.ps1 on Windows" >&2
    exit 1
    ;;
esac

asset="bolt_${VERSION}_${os}_${arch}.tar.gz"
url="https://github.com/${OWNER}/${REPO}/releases/download/v${VERSION}/${asset}"
checksums_url="https://github.com/${OWNER}/${REPO}/releases/download/v${VERSION}/SHA256SUMS"

tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

echo "Downloading ${url} ..."
curl -fsSL "$url" -o "${tmpdir}/${asset}"

if curl -fsSL "$checksums_url" -o "${tmpdir}/SHA256SUMS" 2>/dev/null; then
  expected="$(grep " ${asset}\$" "${tmpdir}/SHA256SUMS" | awk '{print $1}')"
  if [ -n "$expected" ]; then
    actual="$(cd "$tmpdir" && shasum -a 256 "$asset" | awk '{print $1}')"
    if [ "$expected" != "$actual" ]; then
      echo "checksum mismatch for ${asset}" >&2
      exit 1
    fi
    echo "Checksum verified."
  fi
fi

tar -xzf "${tmpdir}/${asset}" -C "$tmpdir"
binary="${tmpdir}/bolt"

if [ ! -f "$binary" ]; then
  echo "archive did not contain bolt binary" >&2
  exit 1
fi

chmod +x "$binary"
if [ -w "$INSTALL_DIR" ]; then
  mv "$binary" "${INSTALL_DIR}/bolt"
else
  sudo mv "$binary" "${INSTALL_DIR}/bolt"
fi

echo "Installed bolt ${VERSION} to ${INSTALL_DIR}/bolt"
bolt --version 2>/dev/null || true
