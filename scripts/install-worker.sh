#!/bin/sh
set -eu

if [ "$#" -ne 2 ]; then
  echo "usage: scripts/install-worker.sh WORKER_PROJECT_ID ORGDOC_RELEASE_VERSION" >&2
  exit 2
fi

WORKER_ID=$1
VERSION=${2#v}
case "$WORKER_ID" in
  "" | *[!a-z0-9-]*) echo "invalid worker project ID: $WORKER_ID" >&2; exit 2 ;;
esac
case "$VERSION" in
  "" | *[!A-Za-z0-9.+-]*) echo "invalid release version: $VERSION" >&2; exit 2 ;;
esac

REPOSITORY=${ORGDOC_REPOSITORY:-smaje99/orgdoc}
RELEASE_URL=${ORGDOC_RELEASE_BASE_URL:-"https://github.com/${REPOSITORY}/releases/download/v${VERSION}"}
DATA_HOME=${XDG_DATA_HOME:-"${HOME:?HOME must be set}/.local/share"}
WORKER_BASE="$DATA_HOME/orgdoc/workers/$WORKER_ID"
INSTALL_ROOT="$WORKER_BASE/$VERSION"
BIN_DIR=${BIN_DIR:-"${HOME:?HOME must be set}/.local/bin"}
ASSET="orgdoc-worker-${WORKER_ID}-${VERSION}-source.tar.gz"

for REQUIRED_COMMAND in curl python3 uv tar awk sed readlink dirname mv mkdir ln mktemp rm; do
  command -v "$REQUIRED_COMMAND" >/dev/null 2>&1 || {
    echo "required command '$REQUIRED_COMMAND' is unavailable; install it before installing this worker" >&2
    exit 1
  }
done
if ! command -v sha256sum >/dev/null 2>&1 && ! command -v shasum >/dev/null 2>&1; then
  echo "required command 'sha256sum' or 'shasum' is unavailable; install one before installing this worker" >&2
  exit 1
fi

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

python3 - "$TEMP_DIR/$ASSET" <<'PY'
import sys
import tarfile
from pathlib import PurePosixPath

archive_path = sys.argv[1]
with tarfile.open(archive_path, "r:gz") as archive:
    for member in archive.getmembers():
        path = PurePosixPath(member.name)
        if path.is_absolute() or ".." in path.parts or member.issym() or member.islnk():
            raise SystemExit(f"unsafe worker bundle path: {member.name}")
PY

mkdir "$TEMP_DIR/extracted"
tar -xzf "$TEMP_DIR/$ASSET" -C "$TEMP_DIR/extracted"
BUNDLE_ROOT="$TEMP_DIR/extracted"
METADATA=$(python3 - "$BUNDLE_ROOT/worker-bundle.json" "$WORKER_ID" "$VERSION" <<'PY'
import json
import re
import subprocess
import sys
from pathlib import Path, PurePosixPath

metadata_path, requested_id, requested_version = sys.argv[1:]
metadata = json.loads(Path(metadata_path).read_text(encoding="utf-8"))
if metadata.get("schema_version") != 1:
    raise SystemExit("unsupported worker bundle schema")
if metadata.get("worker_id") != requested_id or metadata.get("orgdoc_release_version") != requested_version:
    raise SystemExit("worker bundle ID or release version does not match the request")

project_path = PurePosixPath(metadata.get("project_path", ""))
if project_path.is_absolute() or not project_path.parts or ".." in project_path.parts:
    raise SystemExit("worker bundle has an unsafe project path")
entrypoints = metadata.get("entrypoints")
if not isinstance(entrypoints, list) or not entrypoints:
    raise SystemExit("worker bundle must declare entrypoints")
if any(not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9._-]*", entrypoint) for entrypoint in entrypoints):
    raise SystemExit("worker bundle contains an invalid entrypoint name")

def version_tuple(value):
    match = re.search(r"(\d+)\.(\d+)(?:\.(\d+))?", value)
    if not match:
        raise SystemExit(f"cannot parse version: {value}")
    return tuple(int(part or 0) for part in match.groups())

minimum_python = version_tuple(metadata["minimum_python"])
actual_python = sys.version_info[:3]
if actual_python < minimum_python:
    raise SystemExit(
        f"worker requires Python {metadata['minimum_python']} or newer; found {sys.version.split()[0]}"
    )
uv_result = subprocess.run(["uv", "--version"], check=True, capture_output=True, text=True)
minimum_uv = version_tuple(metadata["minimum_uv"])
actual_uv = version_tuple(uv_result.stdout)
if actual_uv < minimum_uv:
    raise SystemExit(f"worker requires uv {metadata['minimum_uv']} or newer; found {uv_result.stdout.strip()}")

print(project_path.as_posix())
for entrypoint in entrypoints:
    print(entrypoint)
for prerequisite in metadata.get("system_prerequisites", []):
    print(f"prerequisite:{prerequisite}")
PY
)
PROJECT_PATH=$(printf '%s\n' "$METADATA" | sed -n '1p')
ENTRYPOINTS=$(printf '%s\n' "$METADATA" | sed -n '2,$p' | sed '/^prerequisite:/d')
PREREQUISITES=$(printf '%s\n' "$METADATA" | sed -n 's/^prerequisite://p')
PROJECT_DIR="$BUNDLE_ROOT/$PROJECT_PATH"
if [ ! -f "$PROJECT_DIR/pyproject.toml" ] || [ ! -f "$PROJECT_DIR/uv.lock" ]; then
  echo "worker bundle is missing its project or lockfile" >&2
  exit 1
fi

if [ -e "$INSTALL_ROOT" ]; then
  echo "worker $WORKER_ID $VERSION is already installed at $INSTALL_ROOT" >&2
  exit 1
fi
mkdir -p "$BIN_DIR" "$(dirname "$INSTALL_ROOT")"
for ENTRYPOINT in $ENTRYPOINTS; do
  LINK="$BIN_DIR/$ENTRYPOINT"
  if [ -e "$LINK" ] || [ -L "$LINK" ]; then
    if [ ! -L "$LINK" ]; then
      echo "refusing to replace existing non-symlink command: $LINK" >&2
      exit 1
    fi
    LINK_TARGET=$(readlink "$LINK")
    case "$LINK_TARGET" in
      "$WORKER_BASE"/*) ;;
      *) echo "refusing to replace a command owned outside worker $WORKER_ID: $LINK" >&2; exit 1 ;;
    esac
  fi
done

mv "$BUNDLE_ROOT" "$INSTALL_ROOT"
PROJECT_DIR="$INSTALL_ROOT/$PROJECT_PATH"
if ! UV_PYTHON_DOWNLOADS=never uv sync --locked --project "$PROJECT_DIR"; then
  rm -rf "$INSTALL_ROOT"
  echo "failed to provision worker $WORKER_ID $VERSION; removed partial installation" >&2
  exit 1
fi
for ENTRYPOINT in $ENTRYPOINTS; do
  if [ ! -x "$PROJECT_DIR/.venv/bin/$ENTRYPOINT" ]; then
    rm -rf "$INSTALL_ROOT"
    echo "worker environment did not create entrypoint $ENTRYPOINT" >&2
    exit 1
  fi
done
for ENTRYPOINT in $ENTRYPOINTS; do
  ln -sfn "$PROJECT_DIR/.venv/bin/$ENTRYPOINT" "$BIN_DIR/$ENTRYPOINT"
done

echo "Installed worker $WORKER_ID $VERSION in $INSTALL_ROOT"
if [ -n "$PREREQUISITES" ]; then
  printf 'External prerequisites to install separately:\n%s\n' "$PREREQUISITES"
fi
