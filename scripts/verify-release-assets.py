#!/usr/bin/env python3
"""Verify the complete OrgDoc release asset set before publishing it."""

from __future__ import annotations

import fnmatch
import hashlib
import json
import re
import sys
import tarfile
from pathlib import Path, PurePosixPath

ROOT = Path(__file__).resolve().parents[1]
CORE_TARGETS = ("linux-amd64", "linux-arm64", "darwin-amd64", "darwin-arm64")
FORBIDDEN_NAMES = (
  ".venv",
  "__pycache__",
  ".cache",
  ".model-cache",
  "model-cache",
  "models",
  ".env",
  ".env.*",
  "*.env",
  "credentials.json",
  "secrets",
  ".secrets",
  "*.pem",
  "*.key",
  "*.p12",
  "*.pfx",
  "*.pyc",
  "*.pyo",
)


def _expected_assets(version: str) -> set[str]:
  catalog = json.loads((ROOT / "workers" / "catalog.json").read_text(encoding="utf-8"))
  core = {f"orgdoc-{version}-{target}.tar.gz" for target in CORE_TARGETS}
  workers = {
    f"orgdoc-worker-{worker['id']}-{version}-source.tar.gz" for worker in catalog["workers"]
  }
  return core | workers


def _verify_archive(path: Path) -> None:
  with tarfile.open(path, "r:gz") as archive:
    for member in archive.getmembers():
      name = PurePosixPath(member.name)
      if name.is_absolute() or ".." in name.parts or member.issym() or member.islnk():
        raise ValueError(f"unsafe path in {path.name}: {member.name}")
      if any(fnmatch.fnmatch(name.name, pattern) for pattern in FORBIDDEN_NAMES):
        raise ValueError(
          f"release archive contains excluded secret/cache/model data: {member.name}"
        )


def verify(version: str, release_dir: Path) -> None:
  if not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9.+-]*", version):
    raise ValueError(f"invalid release version: {version!r}")
  expected = _expected_assets(version)
  manifest_path = release_dir / "SHA256SUMS"
  if not manifest_path.is_file():
    raise ValueError("SHA256SUMS is missing")

  entries = {path.name: path for path in release_dir.iterdir()}
  unexpected = set(entries) - expected - {"SHA256SUMS"}
  files = {name for name, path in entries.items() if name != "SHA256SUMS" and path.is_file()}
  missing = expected - files
  if missing or unexpected:
    details = []
    if missing:
      details.append("missing: " + ", ".join(sorted(missing)))
    if unexpected:
      details.append("unexpected: " + ", ".join(sorted(unexpected)))
    raise ValueError("release asset set is incomplete (" + "; ".join(details) + ")")
  if any(path.is_symlink() or not path.is_file() for path in entries.values()):
    raise ValueError("release directory may contain only regular files")

  manifest: dict[str, str] = {}
  for line_number, line in enumerate(manifest_path.read_text(encoding="utf-8").splitlines(), 1):
    match = re.fullmatch(r"([0-9a-f]{64})  ([^/\\]+)", line)
    if not match:
      raise ValueError(f"invalid SHA256SUMS entry on line {line_number}")
    digest, name = match.groups()
    if name in manifest:
      raise ValueError(f"duplicate SHA256SUMS entry: {name}")
    manifest[name] = digest

  if set(manifest) != expected:
    raise ValueError("SHA256SUMS must cover every release archive exactly once")

  for name in sorted(expected):
    path = release_dir / name
    hasher = hashlib.sha256()
    with path.open("rb") as stream:
      for chunk in iter(lambda: stream.read(1024 * 1024), b""):
        hasher.update(chunk)
    digest = hasher.hexdigest()
    if manifest[name] != digest:
      raise ValueError(f"checksum mismatch: {name}")
    _verify_archive(path)


def main(argv: list[str] | None = None) -> int:
  args = sys.argv[1:] if argv is None else argv
  if len(args) not in (1, 2):
    print("usage: scripts/verify-release-assets.py VERSION [RELEASE_DIR]", file=sys.stderr)
    return 2
  version = args[0].removeprefix("v")
  release_dir = Path(args[1]) if len(args) == 2 else ROOT / "build" / "release"
  try:
    verify(version, release_dir)
  except (OSError, ValueError, json.JSONDecodeError, tarfile.TarError) as error:
    print(f"release asset verification failed: {error}", file=sys.stderr)
    return 1
  print(f"Verified {len(_expected_assets(version))} release archives and SHA256SUMS.")
  return 0


if __name__ == "__main__":
  raise SystemExit(main())
