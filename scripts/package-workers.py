#!/usr/bin/env python3
"""Create source bundles for optional OrgDoc Python worker projects."""

from __future__ import annotations

import argparse
import fnmatch
import hashlib
import json
import os
import re
import shutil
import sys
import tarfile
import tempfile
from pathlib import Path
from typing import Any

ROOT = Path(__file__).resolve().parents[1]
CATALOG_PATH = ROOT / "workers" / "catalog.json"
IGNORE_PATTERNS = (
  ".venv",
  "__pycache__",
  ".pytest_cache",
  ".mypy_cache",
  ".ruff_cache",
  "*.pyc",
  "*.pyo",
  "dist",
  "build",
)
VERSION_PATTERN = re.compile(r"^[A-Za-z0-9][A-Za-z0-9.+-]*$")
WORKER_ID_PATTERN = re.compile(r"^[a-z0-9]+(?:-[a-z0-9]+)*$")


def _validate_relative_path(value: str) -> Path:
  path = Path(value)
  if path.is_absolute() or not path.parts or any(part in {".", ".."} for part in path.parts):
    raise ValueError(f"catalog path must be a safe relative path: {value!r}")
  return path


def load_catalog(repo_root: Path) -> list[dict[str, Any]]:
  catalog_path = repo_root / "workers" / "catalog.json"
  catalog = json.loads(catalog_path.read_text(encoding="utf-8"))
  if catalog.get("schema_version") != 1 or not isinstance(catalog.get("workers"), list):
    raise ValueError(f"unsupported worker catalog schema in {catalog_path}")

  workers: list[dict[str, Any]] = []
  worker_ids: set[str] = set()
  capabilities: set[str] = set()
  for item in catalog["workers"]:
    worker_id = item.get("id", "")
    if not WORKER_ID_PATTERN.fullmatch(worker_id) or worker_id in worker_ids:
      raise ValueError(f"invalid or duplicate worker project ID: {worker_id!r}")
    worker_ids.add(worker_id)

    project_path = _validate_relative_path(item.get("project_path", ""))
    if not (repo_root / project_path / "pyproject.toml").is_file():
      raise ValueError(f"worker {worker_id!r} has no pyproject.toml under {project_path}")

    worker_capabilities = item.get("capabilities")
    entrypoints = item.get("entrypoints")
    if not isinstance(worker_capabilities, list) or not worker_capabilities:
      raise ValueError(f"worker {worker_id!r} must declare at least one capability")
    if not isinstance(entrypoints, list) or not entrypoints:
      raise ValueError(f"worker {worker_id!r} must declare at least one entrypoint")
    duplicate_capabilities = capabilities.intersection(worker_capabilities)
    if duplicate_capabilities:
      names = ", ".join(sorted(duplicate_capabilities))
      raise ValueError(f"capability IDs are assigned to multiple workers: {names}")
    capabilities.update(worker_capabilities)

    for field in ("minimum_python", "minimum_uv"):
      if not isinstance(item.get(field), str) or not item[field]:
        raise ValueError(f"worker {worker_id!r} must declare {field}")
    if not isinstance(item.get("system_prerequisites"), list):
      raise ValueError(f"worker {worker_id!r} system_prerequisites must be a list")
    if not isinstance(item.get("include_compat_runtime"), bool):
      raise ValueError(f"worker {worker_id!r} must declare include_compat_runtime")
    workers.append({**item, "project_path": project_path.as_posix()})
  return workers


def _ignore(directory: str, names: list[str]) -> set[str]:
  return {
    name for name in names if any(fnmatch.fnmatch(name, pattern) for pattern in IGNORE_PATTERNS)
  }


def _copy_tree(source: Path, destination: Path) -> None:
  shutil.copytree(source, destination, dirs_exist_ok=True, ignore=_ignore)


def _stage_worker(worker: dict[str, Any], repo_root: Path, bundle_root: Path, version: str) -> None:
  project_path = Path(worker["project_path"])
  _copy_tree(repo_root / project_path, bundle_root / project_path)

  for capability_id in worker["capabilities"]:
    capability_path = Path("examples/organization-docs/capabilities") / capability_id
    source = repo_root / capability_path
    if not source.is_dir():
      raise ValueError(f"worker {worker['id']!r} references missing capability {capability_id!r}")
    _copy_tree(source, bundle_root / capability_path)

  shutil.copy2(repo_root / "LICENSE", bundle_root / "LICENSE")
  if worker["include_compat_runtime"]:
    shutil.copy2(repo_root / "pyproject.toml", bundle_root / "pyproject.toml")
    shutil.copy2(repo_root / "README.md", bundle_root / "README.md")
    _copy_tree(repo_root / "src" / "media_report", bundle_root / "src" / "media_report")

  metadata = {
    "schema_version": 1,
    "worker_id": worker["id"],
    "orgdoc_release_version": version,
    "project_path": worker["project_path"],
    "capabilities": worker["capabilities"],
    "entrypoints": worker["entrypoints"],
    "minimum_python": worker["minimum_python"],
    "minimum_uv": worker["minimum_uv"],
    "system_prerequisites": worker["system_prerequisites"],
    "include_compat_runtime": worker["include_compat_runtime"],
  }
  (bundle_root / "worker-bundle.json").write_text(
    json.dumps(metadata, indent=2, sort_keys=True) + "\n", encoding="utf-8"
  )


def _write_checksums(output_dir: Path, version: str) -> Path:
  assets = sorted(
    [
      *output_dir.glob(f"orgdoc-{version}-*.tar.gz"),
      *output_dir.glob(f"orgdoc-worker-*-{version}-source.tar.gz"),
    ]
  )
  expected_core = {
    f"orgdoc-{version}-linux-amd64.tar.gz",
    f"orgdoc-{version}-linux-arm64.tar.gz",
    f"orgdoc-{version}-darwin-amd64.tar.gz",
    f"orgdoc-{version}-darwin-arm64.tar.gz",
  }
  found_core = {asset.name for asset in assets if asset.name.startswith(f"orgdoc-{version}-")}
  if found_core != expected_core:
    missing = ", ".join(sorted(expected_core - found_core)) or "none"
    raise ValueError(
      f"expected all four core release archives before packaging workers; missing: {missing}"
    )

  manifest = output_dir / "SHA256SUMS"
  lines = []
  for asset in assets:
    digest = hashlib.sha256(asset.read_bytes()).hexdigest()
    lines.append(f"{digest}  {asset.name}")
  temporary = manifest.with_suffix(".tmp")
  temporary.write_text("\n".join(lines) + "\n", encoding="utf-8")
  temporary.replace(manifest)
  return manifest


def _validate_core_archives(output_dir: Path, version: str) -> list[Path]:
  expected = {
    f"orgdoc-{version}-linux-amd64.tar.gz",
    f"orgdoc-{version}-linux-arm64.tar.gz",
    f"orgdoc-{version}-darwin-amd64.tar.gz",
    f"orgdoc-{version}-darwin-arm64.tar.gz",
  }
  missing = sorted(name for name in expected if not (output_dir / name).is_file())
  if missing:
    raise ValueError(
      "expected all four core release archives before packaging workers; missing: "
      + ", ".join(missing)
    )
  return [output_dir / name for name in sorted(expected)]


def package_workers(version: str, output_dir: Path, repo_root: Path = ROOT) -> list[Path]:
  if not VERSION_PATTERN.fullmatch(version):
    raise ValueError(f"invalid release version: {version!r}")
  repo_root, output_dir = repo_root.resolve(), output_dir.resolve()
  workers = load_catalog(repo_root)
  output_dir.mkdir(parents=True, exist_ok=True)
  _validate_core_archives(output_dir, version)
  archives: list[Path] = []

  for worker in workers:
    archive_path = output_dir / f"orgdoc-worker-{worker['id']}-{version}-source.tar.gz"
    with tempfile.TemporaryDirectory(prefix=f"orgdoc-{worker['id']}-") as temporary:
      bundle_root = Path(temporary)
      _stage_worker(worker, repo_root, bundle_root, version)
      with tarfile.open(archive_path, "w:gz") as archive:
        for source in sorted(bundle_root.rglob("*")):
          archive.add(source, arcname=source.relative_to(bundle_root))
    archives.append(archive_path)

  manifest = _write_checksums(output_dir, version)
  print(f"Packaged {len(archives)} worker bundles and updated {manifest}")
  return archives


def main(argv: list[str] | None = None) -> int:
  parser = argparse.ArgumentParser(description=__doc__)
  parser.add_argument("version", help="OrgDoc release version without the leading v")
  args = parser.parse_args(argv)
  output_dir = (
    Path(os.environ["OUTPUT_DIR"]) if "OUTPUT_DIR" in os.environ else ROOT / "build" / "release"
  )
  try:
    package_workers(args.version, output_dir)
  except (OSError, ValueError, json.JSONDecodeError, tarfile.TarError) as error:
    print(f"worker packaging failed: {error}", file=sys.stderr)
    return 1
  return 0


if __name__ == "__main__":
  raise SystemExit(main())
