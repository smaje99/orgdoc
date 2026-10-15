#!/bin/sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
TEST_DIR=$(mktemp -d)
trap 'rm -rf "$TEST_DIR"' EXIT HUP INT TERM
VERSION=9.8.7
OUTPUT_DIR="$TEST_DIR/release"

if ! grep -Fq 'REPOSITORY=${ORGDOC_REPOSITORY:-smaje99/orgdoc}' "$ROOT/scripts/install-orgdoc.sh"; then
  echo "installer must default to smaje99/orgdoc" >&2
  exit 1
fi

OUTPUT_DIR="$OUTPUT_DIR" "$ROOT/scripts/build-release.sh" "$VERSION"

for TARGET in linux-amd64 linux-arm64 darwin-amd64 darwin-arm64; do
  ASSET="orgdoc-${VERSION}-${TARGET}.tar.gz"
  ARCHIVE="$OUTPUT_DIR/$ASSET"
  test -f "$ARCHIVE"
  tar -tzf "$ARCHIVE" | grep -Eq '(^|/)orgdoc$'
  tar -tzf "$ARCHIVE" | grep -Eq '(^|/)INSTALL\.md$'
  tar -tzf "$ARCHIVE" | grep -Eq '(^|/)LICENSE$'
  tar -tzf "$ARCHIVE" | grep -Eq '(^|/)install-worker\.sh$'
  tar -tzf "$ARCHIVE" | grep -Eq '(^|/)skills/orgdoc/SKILL\.md$'
  tar -tzf "$ARCHIVE" | grep -Eq '(^|/)docs/installation/agents\.md$'
  if tar -tzf "$ARCHIVE" | grep -Eiq '(^|/)(workers?|python|\.venv)(/|$)'; then
    echo "$ASSET unexpectedly contains worker files" >&2
    exit 1
  fi
  grep -F " $ASSET" "$OUTPUT_DIR/SHA256SUMS" >/dev/null
done

COUNT=$(wc -l < "$OUTPUT_DIR/SHA256SUMS" | tr -d ' ')
test "$COUNT" -eq 4
(
  cd "$OUTPUT_DIR"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum -c SHA256SUMS
  else
    shasum -a 256 -c SHA256SUMS
  fi
)

LINUX_ARCHIVE="$OUTPUT_DIR/orgdoc-${VERSION}-linux-amd64.tar.gz"
"$ROOT/scripts/smoke-release.sh" "$LINUX_ARCHIVE"

RELEASE_DIR="$TEST_DIR/releases/v${VERSION}"
mkdir -p "$RELEASE_DIR" "$TEST_DIR/home"
cp "$LINUX_ARCHIVE" "$OUTPUT_DIR/SHA256SUMS" "$RELEASE_DIR/"
INSTALL_DIR="$TEST_DIR/custom-bin" \
HOME="$TEST_DIR/home" \
ORGDOC_RELEASE_BASE_URL="file://$RELEASE_DIR" \
  "$ROOT/scripts/install-orgdoc.sh" "$VERSION"
test -x "$TEST_DIR/custom-bin/orgdoc"
test "$("$TEST_DIR/custom-bin/orgdoc" --version)" = "orgdoc version $VERSION"

if HOME="$TEST_DIR/home" ORGDOC_RELEASE_BASE_URL="file://$TEST_DIR/no-release" \
  "$ROOT/scripts/install-orgdoc.sh" "$VERSION" >"$TEST_DIR/missing-release.log" 2>&1; then
  echo "installer unexpectedly accepted a missing release asset" >&2
  exit 1
fi

MISSING_CHECKSUM="$TEST_DIR/missing-checksum"
mkdir -p "$MISSING_CHECKSUM"
cp "$LINUX_ARCHIVE" "$MISSING_CHECKSUM/"
if HOME="$TEST_DIR/home" ORGDOC_RELEASE_BASE_URL="file://$MISSING_CHECKSUM" \
  "$ROOT/scripts/install-orgdoc.sh" "$VERSION" >"$TEST_DIR/missing-checksum.log" 2>&1; then
  echo "installer unexpectedly accepted a missing checksum manifest" >&2
  exit 1
fi

INVALID_CHECKSUM="$TEST_DIR/invalid-checksum"
mkdir -p "$INVALID_CHECKSUM"
cp "$LINUX_ARCHIVE" "$INVALID_CHECKSUM/"
printf '0000000000000000000000000000000000000000000000000000000000000000  %s\n' \
  "$(basename "$LINUX_ARCHIVE")" > "$INVALID_CHECKSUM/SHA256SUMS"
if HOME="$TEST_DIR/home" ORGDOC_RELEASE_BASE_URL="file://$INVALID_CHECKSUM" \
  "$ROOT/scripts/install-orgdoc.sh" "$VERSION" >"$TEST_DIR/invalid-checksum.log" 2>&1; then
  echo "installer unexpectedly accepted an invalid checksum" >&2
  exit 1
fi

mkdir -p "$TEST_DIR/fake-bin"
cat > "$TEST_DIR/fake-bin/uname" <<'EOF'
#!/bin/sh
echo FreeBSD
EOF
chmod +x "$TEST_DIR/fake-bin/uname"
if PATH="$TEST_DIR/fake-bin:$PATH" HOME="$TEST_DIR/home" \
  "$ROOT/scripts/install-orgdoc.sh" "$VERSION" >"$TEST_DIR/unsupported.log" 2>&1; then
  echo "installer unexpectedly accepted an unsupported platform" >&2
  exit 1
fi
grep -F "unsupported platform: freebsd/" "$TEST_DIR/unsupported.log" >/dev/null

echo "Core release packaging and installer checks passed."
