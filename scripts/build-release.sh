#!/bin/sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
VERSION=${1:-$(cat "$ROOT/VERSION")}
OUTPUT_DIR=${OUTPUT_DIR:-"$ROOT/build/release"}

case "$VERSION" in
  *[!A-Za-z0-9.+-]* | "")
    echo "invalid release version: $VERSION" >&2
    exit 2
    ;;
esac

command -v go >/dev/null 2>&1 || { echo "go is required" >&2; exit 1; }
mkdir -p "$OUTPUT_DIR"
TEMP_DIR=$(mktemp -d)
trap 'rm -rf "$TEMP_DIR"' EXIT HUP INT TERM

for TARGET in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64; do
  GOOS=${TARGET%/*}
  GOARCH=${TARGET#*/}
  ASSET="orgdoc-${VERSION}-${GOOS}-${GOARCH}.tar.gz"
  PACKAGE_DIR="$TEMP_DIR/${GOOS}-${GOARCH}"
  mkdir -p "$PACKAGE_DIR"
  (
    cd "$ROOT"
    CGO_ENABLED=0 GOOS="$GOOS" GOARCH="$GOARCH" go build \
      -trimpath \
      -ldflags "-s -w -X github.com/smaje99/orgdoc/internal/cli.Version=${VERSION}" \
      -o "$PACKAGE_DIR/orgdoc" \
      ./cmd/orgdoc
  )
  cp "$ROOT/INSTALL.md" "$ROOT/LICENSE" "$ROOT/scripts/install-worker.sh" "$PACKAGE_DIR/"
  mkdir -p "$PACKAGE_DIR/skills/orgdoc"
  cp "$ROOT/skills/orgdoc/SKILL.md" "$PACKAGE_DIR/skills/orgdoc/"
  mkdir -p "$PACKAGE_DIR/docs/installation" "$PACKAGE_DIR/docs/product"
  cp "$ROOT"/docs/installation/*.md "$PACKAGE_DIR/docs/installation/"
  cp "$ROOT/docs/product/security-model.md" "$PACKAGE_DIR/docs/product/"
  tar -C "$PACKAGE_DIR" -czf "$OUTPUT_DIR/$ASSET" .
done

(
  cd "$OUTPUT_DIR"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum orgdoc-"$VERSION"-*.tar.gz > SHA256SUMS
  else
    shasum -a 256 orgdoc-"$VERSION"-*.tar.gz > SHA256SUMS
  fi
)

echo "Release archives and checksums written to $OUTPUT_DIR"
