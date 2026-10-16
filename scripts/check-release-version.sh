#!/bin/sh
set -eu

if [ "$#" -ne 1 ]; then
  echo "usage: scripts/check-release-version.sh TAG_OR_VERSION" >&2
  exit 2
fi

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
TAG_VERSION=${1#v}
EXPECTED_VERSION=$(cat "$ROOT/VERSION")

case "$TAG_VERSION" in
  "" | *[!A-Za-z0-9.+-]*)
    echo "invalid release tag version: $TAG_VERSION" >&2
    exit 2
    ;;
esac

if [ "$TAG_VERSION" != "$EXPECTED_VERSION" ]; then
  echo "release tag version $TAG_VERSION does not match VERSION $EXPECTED_VERSION" >&2
  exit 1
fi

echo "Release tag version $TAG_VERSION matches VERSION."
