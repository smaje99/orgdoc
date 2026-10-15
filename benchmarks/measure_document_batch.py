from __future__ import annotations

import argparse
import json
import os
import platform
import subprocess
import tempfile
import time
from pathlib import Path


def process_tree_rss_kib(root_pid: int) -> tuple[int, int]:
  processes: dict[int, tuple[int, int]] = {}
  try:
    process_dirs = Path("/proc").iterdir()
  except OSError:
    return 0, 0
  for path in process_dirs:
    if not path.name.isdigit():
      continue
    try:
      status = (path / "status").read_text(encoding="ascii")
      values = {
        line.split(":", 1)[0]: line.split(":", 1)[1].strip()
        for line in status.splitlines()
        if ":" in line
      }
      rss = int(values.get("VmRSS", "0 kB").split()[0])
      parent = int(values.get("PPid", "0"))
      processes[int(path.name)] = (parent, rss)
    except (OSError, ValueError, IndexError):
      continue
  children: dict[int, list[int]] = {}
  for pid, (parent, _) in processes.items():
    children.setdefault(parent, []).append(pid)
  tree = [root_pid]
  for pid in tree:
    tree.extend(children.get(pid, []))
  coordinator = processes.get(root_pid, (0, 0))[1]
  return coordinator, sum(processes.get(pid, (0, 0))[1] for pid in set(tree))


def run_batch(
  binary: Path,
  organization: Path,
  fixture: dict,
  workers: int,
  worker_paths: list[Path],
  jobs_count: int,
) -> dict:
  with tempfile.TemporaryDirectory(prefix=f"orgdoc-batch-{workers}-") as temp:
    root = Path(temp)
    definition = {
      "schema_version": 1,
      "organization": str(organization),
      "output_root": str(root / "runs"),
      "jobs": [
        {
          "job_id": f"invoice-{index:03d}",
          "operation": "consulting",
          "document_type": "invoice",
          "input": fixture,
        }
        for index in range(jobs_count)
      ],
    }
    job_file = root / "batch.json"
    job_file.write_text(json.dumps(definition), encoding="utf-8")
    env = os.environ.copy()
    env["PATH"] = os.pathsep.join([*(str(path) for path in worker_paths), env.get("PATH", "")])
    command = [
      str(binary),
      "--format",
      "json",
      "batch",
      "run",
      str(job_file),
      "--max-workers",
      str(workers),
    ]
    started_at = time.perf_counter()
    process = subprocess.Popen(
      command, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, env=env
    )
    coordinator_peak = 0
    tree_peak = 0
    while process.poll() is None:
      coordinator, tree = process_tree_rss_kib(process.pid)
      coordinator_peak = max(coordinator_peak, coordinator)
      tree_peak = max(tree_peak, tree)
      time.sleep(0.01)
    stdout, stderr = process.communicate()
    process_elapsed = time.perf_counter() - started_at
    coordinator, tree = process_tree_rss_kib(process.pid)
    coordinator_peak = max(coordinator_peak, coordinator)
    tree_peak = max(tree_peak, tree)
    if process.returncode != 0:
      raise RuntimeError(
        f"batch failed: exit={process.returncode} stderr={stderr} stdout={stdout[:1000]}"
      )
    result = json.loads(stdout)
    if result["status"] != "succeeded" or len(result["results"]) != jobs_count:
      raise RuntimeError(f"unexpected result: {result['status']} for {len(result['results'])} jobs")
    return {
      "workers": workers,
      "jobs": jobs_count,
      "seconds": process_elapsed,
      "jobs_per_second": jobs_count / process_elapsed,
      "coordinator_peak_rss_mib": coordinator_peak / 1024,
      "process_tree_peak_rss_mib": tree_peak / 1024,
    }


def main() -> int:
  parser = argparse.ArgumentParser(
    description="Measure many small invoice jobs through the OrgDoc CLI batch contract."
  )
  parser.add_argument("binary", type=Path)
  parser.add_argument("--organization", type=Path, default=Path("examples/organization-docs"))
  parser.add_argument(
    "--invoice-worker-bin", type=Path, default=Path("workers/python/reportlab-invoice/.venv/bin")
  )
  parser.add_argument("--jobs", type=int, default=100)
  args = parser.parse_args()
  binary = args.binary.resolve()
  organization = args.organization.resolve()
  worker_bin = args.invoice_worker_bin.resolve()
  fixture = json.loads((Path("tests/fixtures/documents/invoice.json")).read_text(encoding="utf-8"))
  print(f"host={platform.platform()} machine={platform.machine()} python_worker={worker_bin}")
  results = [
    run_batch(binary, organization, fixture, workers, [worker_bin], args.jobs) for workers in (1, 4)
  ]
  for result in results:
    print(json.dumps(result, sort_keys=True))
  print(f"throughput_speedup={results[1]['jobs_per_second'] / results[0]['jobs_per_second']:.2f}x")
  return 0


if __name__ == "__main__":
  raise SystemExit(main())
