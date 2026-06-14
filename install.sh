#!/bin/sh
# nstat installer — Linux & macOS, any architecture.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/amine-khemissi/nstat/main/install.sh | sh
#
# Environment overrides:
#   INSTALL_DIR   target directory   (default: /usr/local/bin)
#   VERSION       release tag        (default: latest)

set -eu

REPO="amine-khemissi/nstat"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
VERSION="${VERSION:-latest}"

os=$(uname -s | tr '[:upper:]' '[:lower:]')
arch=$(uname -m)

case "$os" in
  linux | darwin) ;;
  *) echo "nstat: unsupported OS: $os (only linux and darwin are supported)" >&2; exit 1 ;;
esac

case "$arch" in
  x86_64 | amd64) arch=amd64 ;;
  aarch64 | arm64) arch=arm64 ;;
  *) echo "nstat: unsupported architecture: $arch (only x86_64 and arm64 are supported)" >&2; exit 1 ;;
esac

if [ "$VERSION" = "latest" ]; then
  url="https://github.com/${REPO}/releases/latest/download/nstat-${os}-${arch}"
else
  url="https://github.com/${REPO}/releases/download/${VERSION}/nstat-${os}-${arch}"
fi

echo "nstat: downloading nstat-${os}-${arch} (${VERSION})"
echo "       from ${url}"

tmp=$(mktemp)
trap 'rm -f "$tmp"' EXIT

if ! curl -fSL "$url" -o "$tmp"; then
  echo "nstat: download failed — check that a release asset exists for ${os}/${arch}" >&2
  exit 1
fi
chmod +x "$tmp"

# Use sudo only if we can't write to the target directory ourselves.
if [ -w "$INSTALL_DIR" ] || { [ ! -e "$INSTALL_DIR" ] && [ -w "$(dirname "$INSTALL_DIR")" ]; }; then
  mkdir -p "$INSTALL_DIR"
  mv "$tmp" "$INSTALL_DIR/nstat"
else
  echo "nstat: installing to ${INSTALL_DIR} (requires sudo)"
  sudo mkdir -p "$INSTALL_DIR"
  sudo mv "$tmp" "$INSTALL_DIR/nstat"
fi
trap - EXIT

echo "nstat: installed to ${INSTALL_DIR}/nstat"
if command -v nstat >/dev/null 2>&1; then
  echo "nstat: $(nstat --version)"
else
  echo "nstat: ${INSTALL_DIR} is not on your PATH — add it to use 'nstat' directly"
fi
