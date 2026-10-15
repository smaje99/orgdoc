#!/bin/sh
set -eu

if [ "$#" -ne 1 ]; then
  echo "usage: scripts/install-orgdoc.sh VERSION" >&2
  exit 2
fi

VERSION=${1#v}
case "$VERSION" in
  "" | *[!A-Za-z0-9.+-]*) echo "invalid release version: $VERSION" >&2; exit 2 ;;
esac
REPOSITORY=${ORGDOC_REPOSITORY:-smaje99/media-report-cli}
INSTALL_DIR=${INSTALL_DIR:-"${HOME:?HOME must be set}/.local/bin"}
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$OS/$ARCH" in
  linux/x86_64) TARGET=linux-amd64 ;;
  linux/aarch64 | linux/arm64) TARGET=linux-arm64 ;;
  darwin/x86_64) TARGET=darwin-amd64 ;;
  darwin/arm64) TARGET=darwin-arm64 ;;
  *) echo "unsupported platform: $OS/$ARCH" >&2; exit 1 ;;
esac

ASSET="orgdoc-${VERSION}-${TARGET}.tar.gz"
RELEASE_URL=${ORGDOC_RELEASE_BASE_URL:-"https://github.com/${REPOSITORY}/releases/download/v${VERSION}"}
TEMP_DIR=$(mktemp -d)
trap 'rm -rf "$TEMP_DIR"' EXIT HUP INT TERM

curl -fsSL "$RELEASE_URL/$ASSET" -o "$TEMP_DIR/$ASSET"
curl -fsSL "$RELEASE_URL/SHA256SUMS" -o "$TEMP_DIR/SHA256SUMS"
EXPECTED=$(awk -v asset="$ASSET" '$2 == asset { print $1 }' "$TEMP_DIR/SHA256SUMS")
if [ -z "$EXPECTED" ]; then
  echo "checksum for $ASSET was not found in SHA256SUMS" >&2
  exit 1
fi
if command -v sha256sum >/dev/null 2>&1; then
  ACTUAL=$(sha256sum "$TEMP_DIR/$ASSET" | awk '{print $1}')
else
  ACTUAL=$(shasum -a 256 "$TEMP_DIR/$ASSET" | awk '{print $1}')
fi
if [ "$ACTUAL" != "$EXPECTED" ]; then
  echo "checksum verification failed for $ASSET" >&2
  exit 1
fi

mkdir -p "$TEMP_DIR/extracted" "$INSTALL_DIR"
tar -xzf "$TEMP_DIR/$ASSET" -C "$TEMP_DIR/extracted"
install -m 755 "$TEMP_DIR/extracted/orgdoc" "$INSTALL_DIR/orgdoc"
echo "Installed orgdoc $VERSION in $INSTALL_DIR"
