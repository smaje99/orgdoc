#!/bin/sh
set -eu

if [ "$#" -lt 1 ] || [ "$#" -gt 2 ]; then
  echo "usage: scripts/smoke-worker-bundles.sh VERSION [RELEASE_DIR]" >&2
  exit 2
fi

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
VERSION=${1#v}
RELEASE_DIR=${2:-"$ROOT/build/release"}
case "$RELEASE_DIR" in
  /*) ;;
  *) RELEASE_DIR="$ROOT/$RELEASE_DIR" ;;
esac
RELEASE_DIR=$(CDPATH= cd -- "$RELEASE_DIR" && pwd)

case "$(uname -s)" in
  Linux) OS=linux ;;
  Darwin) OS=darwin ;;
  *) echo "unsupported platform: $(uname -s)" >&2; exit 1 ;;
esac
case "$(uname -m)" in
  x86_64 | amd64) ARCH=amd64 ;;
  aarch64 | arm64) ARCH=arm64 ;;
  *) echo "unsupported architecture: $(uname -m)" >&2; exit 1 ;;
esac

CORE_ARCHIVE="$RELEASE_DIR/orgdoc-$VERSION-$OS-$ARCH.tar.gz"
if [ ! -f "$CORE_ARCHIVE" ] || [ ! -f "$RELEASE_DIR/SHA256SUMS" ]; then
  echo "release core archive or SHA256SUMS is missing for $OS/$ARCH" >&2
  exit 1
fi

TEMP_DIR=$(mktemp -d)
trap 'rm -rf "$TEMP_DIR"' EXIT HUP INT TERM
mkdir -p "$TEMP_DIR/core" "$TEMP_DIR/home" "$TEMP_DIR/data" "$TEMP_DIR/bin" \
  "$TEMP_DIR/cache" "$TEMP_DIR/work"
tar -xzf "$CORE_ARCHIVE" -C "$TEMP_DIR/core"

export HOME="$TEMP_DIR/home"
export XDG_DATA_HOME="$TEMP_DIR/data"
export BIN_DIR="$TEMP_DIR/bin"
export UV_CACHE_DIR="$TEMP_DIR/cache"
export ORGDOC_RELEASE_BASE_URL="file://$RELEASE_DIR"
unset PYTHONPATH PYTHONHOME VIRTUAL_ENV

EXPECTED_WORKERS=$(python3 - "$ROOT/workers/catalog.json" <<'PY'
import json
import sys
from pathlib import Path

catalog = json.loads(Path(sys.argv[1]).read_text(encoding="utf-8"))
for worker in catalog["workers"]:
    print(worker["id"])
PY
)
WORKER_COUNT=0
for WORKER_ID in $EXPECTED_WORKERS; do
  BUNDLE="$RELEASE_DIR/orgdoc-worker-$WORKER_ID-$VERSION-source.tar.gz"
  if [ ! -f "$BUNDLE" ]; then
    echo "worker bundle is missing: $(basename "$BUNDLE")" >&2
    exit 1
  fi
  "$TEMP_DIR/core/install-worker.sh" "$WORKER_ID" "$VERSION"

  INSTALL_ROOT="$XDG_DATA_HOME/orgdoc/workers/$WORKER_ID/$VERSION"
  python3 - "$INSTALL_ROOT/worker-bundle.json" "$INSTALL_ROOT" "$TEMP_DIR/work" <<'PY'
import json
import subprocess
import sys
from pathlib import Path

metadata_path, install_root, work_dir = map(Path, sys.argv[1:])
metadata = json.loads(metadata_path.read_text(encoding="utf-8"))
project = install_root / metadata["project_path"]
for entrypoint in metadata["entrypoints"]:
    executable = project / ".venv" / "bin" / entrypoint
    if not executable.is_file() or not executable.stat().st_mode & 0o111:
        raise SystemExit(f"worker entrypoint is missing or not executable: {entrypoint}")
    result = subprocess.run(
        [str(executable)],
        input="{}\n",
        text=True,
        capture_output=True,
        cwd=work_dir,
        check=False,
    )
    if result.returncode not in (0, 1):
        raise SystemExit(f"worker entrypoint failed to start: {entrypoint}: {result.stderr}")
    try:
        response = json.loads(result.stdout)
    except json.JSONDecodeError as error:
        raise SystemExit(f"worker entrypoint did not return JSON: {entrypoint}") from error
    if response.get("protocol_version") != 1 or response.get("status") != "failed":
        raise SystemExit(f"worker entrypoint returned an unexpected protocol response: {entrypoint}")
PY
  WORKER_COUNT=$((WORKER_COUNT + 1))
done

if [ "$WORKER_COUNT" -eq 0 ]; then
  echo "no worker bundles were installed" >&2
  exit 1
fi
echo "Installed and invoked $WORKER_COUNT worker bundles outside the repository checkout."
