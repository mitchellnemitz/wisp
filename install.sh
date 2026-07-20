#!/bin/sh
# Install the latest wisp and wisp-lsp release binaries.
#
#   curl -fsSL https://raw.githubusercontent.com/mitchellnemitz/wisp/main/install.sh | sh
#
# Override the install directory with PREFIX, and the version with WISP_VERSION:
#   PREFIX=$HOME/.local/bin WISP_VERSION=v0.2.0 sh install.sh
set -eu

REPO="mitchellnemitz/wisp"
PREFIX="${PREFIX:-/usr/local/bin}"

os=$(uname -s | tr '[:upper:]' '[:lower:]')
arch=$(uname -m)
case "$arch" in
  x86_64 | amd64) arch=amd64 ;;
  aarch64 | arm64) arch=arm64 ;;
  *) echo "wisp install: unsupported architecture: $arch" >&2; exit 1 ;;
esac
case "$os" in
  linux | darwin) ;;
  *) echo "wisp install: unsupported OS: $os (use the .zip from the releases page)" >&2; exit 1 ;;
esac

tag="${WISP_VERSION:-}"
if [ -z "$tag" ]; then
  tag=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
    | grep '"tag_name"' | head -1 | cut -d'"' -f4)
fi
if [ -z "$tag" ]; then
  echo "wisp install: could not determine the latest release" >&2
  exit 1
fi
version=${tag#v}

archive="wisp_${version}_${os}_${arch}.tar.gz"
url="https://github.com/$REPO/releases/download/$tag/$archive"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

echo "wisp install: downloading $tag for ${os}/${arch}"
curl -fsSL "$url" -o "$tmp/$archive"

# Verify the checksum when the release publishes one.
if curl -fsSL "https://github.com/$REPO/releases/download/$tag/checksums.txt" -o "$tmp/checksums.txt" 2>/dev/null; then
  want=$(grep " $archive\$" "$tmp/checksums.txt" | cut -d' ' -f1 || true)
  if [ -n "$want" ]; then
    if command -v sha256sum >/dev/null 2>&1; then
      got=$(sha256sum "$tmp/$archive" | cut -d' ' -f1)
    else
      got=$(shasum -a 256 "$tmp/$archive" | cut -d' ' -f1)
    fi
    if [ "$got" != "$want" ]; then
      echo "wisp install: checksum mismatch for $archive" >&2
      exit 1
    fi
  fi
fi

tar -xzf "$tmp/$archive" -C "$tmp"

if [ ! -w "$PREFIX" ] && [ "$(id -u)" -ne 0 ]; then
  echo "wisp install: $PREFIX is not writable; re-run with sudo or set PREFIX" >&2
  exit 1
fi
install -m 0755 "$tmp/wisp" "$PREFIX/wisp"
install -m 0755 "$tmp/wisp-lsp" "$PREFIX/wisp-lsp"

echo "wisp install: installed wisp and wisp-lsp $tag to $PREFIX"
