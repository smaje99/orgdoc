import hashlib
import json
import os
import subprocess
import sys
import tarfile
import tomllib
import unittest
from pathlib import Path
from tempfile import TemporaryDirectory

ROOT = Path(__file__).resolve().parents[2]
CATALOG_PATH = ROOT / "workers" / "catalog.json"
PACKAGER_PATH = ROOT / "scripts" / "package-workers.py"
INSTALLER_PATH = ROOT / "scripts" / "install-worker.sh"
EXPECTED_WORKERS = {
  "spreadsheet": {"spreadsheet-xlsx"},
  "reportlab-invoice": {"invoice-pdf"},
  "narrative-report": {"report-markdown"},
  "media-report": {"media-report", "media-pdf"},
  "media-transcription": {"media-scan", "media-transcription"},
}
EXPECTED_COMPAT_RUNTIME = {
  "spreadsheet": False,
  "reportlab-invoice": False,
  "narrative-report": False,
  "media-report": True,
  "media-transcription": True,
}


class WorkerCatalogTests(unittest.TestCase):
  def test_catalog_maps_each_worker_project_and_capabilities(self):
    self.assertTrue(CATALOG_PATH.is_file(), "workers/catalog.json must exist")
    catalog = json.loads(CATALOG_PATH.read_text(encoding="utf-8"))

    self.assertEqual(catalog["schema_version"], 1)
    workers = {worker["id"]: worker for worker in catalog["workers"]}
    self.assertEqual(set(workers), set(EXPECTED_WORKERS))

    seen_capabilities = set()
    for worker_id, worker in workers.items():
      with self.subTest(worker=worker_id):
        self.assertEqual(set(worker["capabilities"]), EXPECTED_WORKERS[worker_id])
        self.assertTrue(worker["project_path"].startswith("workers/python/"))
        pyproject_path = ROOT / worker["project_path"] / "pyproject.toml"
        self.assertTrue(pyproject_path.is_file())
        pyproject = tomllib.loads(pyproject_path.read_text(encoding="utf-8"))
        self.assertEqual(set(worker["entrypoints"]), set(pyproject["project"]["scripts"]))
        self.assertIn("minimum_python", worker)
        self.assertIn("minimum_uv", worker)
        self.assertIsInstance(worker["system_prerequisites"], list)
        self.assertEqual(worker["include_compat_runtime"], EXPECTED_COMPAT_RUNTIME[worker_id])
        self.assertFalse(seen_capabilities.intersection(worker["capabilities"]))
        seen_capabilities.update(worker["capabilities"])

  def test_worker_packager_entrypoint_exists(self):
    self.assertTrue(PACKAGER_PATH.is_file(), "scripts/package-workers.py must exist")

  def test_worker_installer_entrypoint_exists(self):
    self.assertTrue(INSTALLER_PATH.is_file(), "scripts/install-worker.sh must exist")

  def test_cli_uses_output_dir_and_reports_missing_core_archives(self):
    with TemporaryDirectory() as temporary:
      environment = {**os.environ, "OUTPUT_DIR": temporary}
      result = subprocess.run(
        [sys.executable, str(PACKAGER_PATH), "0.1.0"],
        check=False,
        capture_output=True,
        text=True,
        env=environment,
      )
      self.assertEqual(list(Path(temporary).glob("orgdoc-worker-*.tar.gz")), [])

    self.assertEqual(result.returncode, 1)
    self.assertIn("expected all four core release archives", result.stderr)
    self.assertNotIn("AttributeError", result.stderr)

  def test_packager_bundles_worker_sources_and_rewrites_combined_checksums(self):
    version = (ROOT / "VERSION").read_text(encoding="utf-8").strip()
    with TemporaryDirectory() as temporary:
      output_dir = Path(temporary)
      targets = ("linux-amd64", "linux-arm64", "darwin-amd64", "darwin-arm64")
      for target in targets:
        (output_dir / f"orgdoc-{version}-{target}.tar.gz").write_bytes(b"core archive fixture")

      result = subprocess.run(
        [sys.executable, str(PACKAGER_PATH), version],
        check=False,
        capture_output=True,
        text=True,
        env={**os.environ, "OUTPUT_DIR": str(output_dir)},
      )
      self.assertEqual(result.returncode, 0, result.stderr)

      worker_ids = {
        "spreadsheet",
        "reportlab-invoice",
        "narrative-report",
        "media-report",
        "media-transcription",
      }
      bundles = {path.name for path in output_dir.glob("orgdoc-worker-*.tar.gz")}
      self.assertEqual(
        bundles,
        {f"orgdoc-worker-{worker_id}-{version}-source.tar.gz" for worker_id in worker_ids},
      )
      checksums = (output_dir / "SHA256SUMS").read_text(encoding="utf-8").splitlines()
      self.assertEqual(len(checksums), 4 + len(worker_ids))
      for line in checksums:
        expected_hash, filename = line.split("  ", maxsplit=1)
        self.assertEqual(
          hashlib.sha256((output_dir / filename).read_bytes()).hexdigest(), expected_hash
        )

      media_bundle = output_dir / f"orgdoc-worker-media-transcription-{version}-source.tar.gz"
      with tarfile.open(media_bundle, "r:gz") as archive:
        members = set(archive.getnames())
        self.assertIn("worker-bundle.json", members)
        self.assertIn("workers/python/media-transcription/pyproject.toml", members)
        self.assertIn("workers/python/media-transcription/uv.lock", members)
        self.assertIn(
          "examples/organization-docs/capabilities/media-transcription/capability.yaml", members
        )
        self.assertIn("pyproject.toml", members)
        self.assertIn("src/media_report/templates/prompts/generic.md", members)
        self.assertFalse(any(".venv" in name or "__pycache__" in name for name in members))
        metadata = json.load(archive.extractfile("worker-bundle.json"))
        self.assertEqual(metadata["worker_id"], "media-transcription")
        self.assertEqual(metadata["capabilities"], ["media-scan", "media-transcription"])

      invoice_bundle = output_dir / f"orgdoc-worker-reportlab-invoice-{version}-source.tar.gz"
      with tarfile.open(invoice_bundle, "r:gz") as archive:
        self.assertNotIn("src/media_report", set(archive.getnames()))

      spreadsheet_bundle = output_dir / f"orgdoc-worker-spreadsheet-{version}-source.tar.gz"
      with tarfile.open(spreadsheet_bundle, "r:gz") as archive:
        members = set(archive.getnames())
        self.assertIn("workers/python/spreadsheet/pyproject.toml", members)
        self.assertIn("workers/python/spreadsheet/uv.lock", members)
        self.assertIn("workers/python/spreadsheet/src/orgdoc_spreadsheet/worker.py", members)
        self.assertIn("workers/python/spreadsheet/examples/quarterly-revenue.json", members)
        self.assertIn(
          "examples/organization-docs/capabilities/spreadsheet-xlsx/capability.yaml", members
        )
        metadata = json.load(archive.extractfile("worker-bundle.json"))
        self.assertEqual(metadata["worker_id"], "spreadsheet")
        self.assertEqual(metadata["capabilities"], ["spreadsheet-xlsx"])

  def test_worker_installer_verifies_bundle_and_installs_one_isolated_project(self):
    version = (ROOT / "VERSION").read_text(encoding="utf-8").strip()
    with TemporaryDirectory() as temporary:
      base = Path(temporary)
      release_dir = base / "release"
      release_dir.mkdir()
      for target in ("linux-amd64", "linux-arm64", "darwin-amd64", "darwin-arm64"):
        (release_dir / f"orgdoc-{version}-{target}.tar.gz").write_bytes(b"core archive fixture")
      package = subprocess.run(
        [sys.executable, str(PACKAGER_PATH), version],
        check=False,
        capture_output=True,
        text=True,
        env={**os.environ, "OUTPUT_DIR": str(release_dir)},
      )
      self.assertEqual(package.returncode, 0, package.stderr)

      fake_bin = base / "fake-bin"
      fake_bin.mkdir()
      uv = fake_bin / "uv"
      uv.write_text(
        """#!/bin/sh
if [ "${1-}" = "--version" ]; then
  echo "uv 0.11.8"
  exit 0
fi
if [ "${1-}" != "sync" ]; then
  exit 2
fi
shift
PROJECT=
while [ "$#" -gt 0 ]; do
  if [ "$1" = "--project" ]; then
    PROJECT=$2
    shift 2
  else
    shift
  fi
done
mkdir -p "$PROJECT/.venv/bin"
printf '%s\\n' \\
  '#!/bin/sh' \\
  'SCRIPT=$1' \\
  'shift' \\
  'exec /bin/sh "$SCRIPT" "$@"' > "$PROJECT/.venv/bin/python"
chmod +x "$PROJECT/.venv/bin/python"
sed -n '/^\\[project\\.scripts\\]/,/^\\[/p' "$PROJECT/pyproject.toml" |
  sed '1d;$d;s/[[:space:]]*=.*//' |
  while IFS= read -r COMMAND; do
  [ -n "$COMMAND" ] || continue
  printf '%s\\n' \\
    "#!$PROJECT/.venv/bin/python" \\
    "echo installed:$COMMAND" > "$PROJECT/.venv/bin/$COMMAND"
  chmod +x "$PROJECT/.venv/bin/$COMMAND"
done
""",
        encoding="utf-8",
      )
      uv.chmod(0o755)
      bin_dir = base / "bin"
      bin_dir.mkdir()
      orgdoc = bin_dir / "orgdoc"
      orgdoc.write_text("existing orgdoc binary\n", encoding="utf-8")
      orgdoc.chmod(0o755)

      result = subprocess.run(
        ["sh", str(INSTALLER_PATH), "reportlab-invoice", version],
        check=False,
        capture_output=True,
        text=True,
        cwd=base,
        env={
          **os.environ,
          "PATH": f"{fake_bin}:{os.environ['PATH']}",
          "HOME": str(base / "home"),
          "XDG_DATA_HOME": str(base / "data"),
          "BIN_DIR": str(bin_dir),
          "ORGDOC_RELEASE_BASE_URL": f"file://{release_dir}",
        },
      )

      self.assertEqual(result.returncode, 0, result.stderr)
      installed_root = base / "data" / "orgdoc" / "workers" / "reportlab-invoice" / version
      self.assertTrue(
        (
          installed_root / "workers/python/reportlab-invoice/.venv/bin/orgdoc-render-invoice"
        ).is_file()
      )
      entrypoint = (
        installed_root / "workers/python/reportlab-invoice/.venv/bin/orgdoc-render-invoice"
      )
      self.assertEqual(
        entrypoint.read_text(encoding="utf-8").splitlines()[0],
        f"#!{installed_root}/workers/python/reportlab-invoice/.venv/bin/python",
      )
      self.assertTrue((bin_dir / "orgdoc-render-invoice").is_symlink())
      self.assertEqual(
        subprocess.check_output([bin_dir / "orgdoc-render-invoice"], text=True).strip(),
        "installed:orgdoc-render-invoice",
      )
      self.assertEqual(orgdoc.read_text(encoding="utf-8"), "existing orgdoc binary\n")
      self.assertFalse((base / "data" / "orgdoc" / "workers" / "media-report").exists())


if __name__ == "__main__":
  unittest.main()
