#!/bin/sh
set -eu

if [ "$#" -ne 1 ]; then
  echo "usage: scripts/smoke-release.sh RELEASE_ARCHIVE.tar.gz" >&2
  exit 2
fi

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
ARCHIVE=$1
case "$ARCHIVE" in
  /*) ;;
  *) ARCHIVE="$ROOT/$ARCHIVE" ;;
esac
ARCHIVE_DIR=$(dirname -- "$ARCHIVE")
ARCHIVE_NAME=$(basename -- "$ARCHIVE")
if [ ! -f "$ARCHIVE_DIR/SHA256SUMS" ]; then
  echo "SHA256SUMS was not found next to $ARCHIVE" >&2
  exit 1
fi
EXPECTED=$(awk -v asset="$ARCHIVE_NAME" '$2 == asset { print $1 }' "$ARCHIVE_DIR/SHA256SUMS")
if [ -z "$EXPECTED" ]; then
  echo "checksum for $ARCHIVE_NAME was not found in SHA256SUMS" >&2
  exit 1
fi
if command -v sha256sum >/dev/null 2>&1; then
  ACTUAL=$(sha256sum "$ARCHIVE" | awk '{print $1}')
else
  ACTUAL=$(shasum -a 256 "$ARCHIVE" | awk '{print $1}')
fi
if [ "$ACTUAL" != "$EXPECTED" ]; then
  echo "checksum verification failed for $ARCHIVE_NAME" >&2
  exit 1
fi
TEMP_DIR=$(mktemp -d)
trap 'rm -rf "$TEMP_DIR"' EXIT HUP INT TERM

tar -xzf "$ARCHIVE" -C "$TEMP_DIR"
mkdir -p "$TEMP_DIR/empty-bin" "$TEMP_DIR/home"
BIN="$TEMP_DIR/orgdoc"

env -i PATH="$TEMP_DIR/empty-bin" HOME="$TEMP_DIR/home" "$BIN" --version
env -i PATH="$TEMP_DIR/empty-bin" HOME="$TEMP_DIR/home" "$BIN" --help >/dev/null
env -i PATH="$TEMP_DIR/empty-bin" HOME="$TEMP_DIR/home" "$BIN" \
  organization validate --organization "$ROOT/examples/organization-docs" --format json
env -i PATH="$TEMP_DIR/empty-bin" HOME="$TEMP_DIR/home" "$BIN" \
  capabilities list --organization "$ROOT/examples/organization-docs" --format json >/dev/null

echo "Release archive smoke checks passed without worker executables on PATH."
