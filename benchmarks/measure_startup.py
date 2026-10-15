from __future__ import annotations

import argparse
import math
import os
import platform
import statistics
import subprocess
import time
from pathlib import Path


def percentile(values: list[float], fraction: float) -> float:
  ordered = sorted(values)
  return ordered[max(0, math.ceil(fraction * len(ordered)) - 1)]


def main() -> int:
  parser = argparse.ArgumentParser(
    description="Measure cold OrgDoc process startup and peak RSS on Linux."
  )
  parser.add_argument("binary", type=Path, help="path to a previously built orgdoc binary")
  parser.add_argument("--samples", type=int, default=30)
  args = parser.parse_args()
  if args.samples < 30:
    parser.error("--samples must be at least 30 for release measurements")
  if not Path("/usr/bin/time").is_file():
    parser.error("this measurement helper requires GNU /usr/bin/time on Linux")

  elapsed: list[float] = []
  rss_mib: list[float] = []
  for _ in range(args.samples):
    started = time.perf_counter()
    measured = subprocess.run(
      ["/usr/bin/time", "-f", "%M", str(args.binary), "--help"],
      check=True,
      stdout=subprocess.DEVNULL,
      stderr=subprocess.PIPE,
      text=True,
    )
    elapsed.append(time.perf_counter() - started)
    rss_mib.append(int(measured.stderr.strip().splitlines()[-1]) / 1024)
  print(f"host={platform.platform()} machine={platform.machine()} cpu_count={os.cpu_count()}")
  median_ms = statistics.median(elapsed) * 1000
  p95_ms = percentile(elapsed, 0.95) * 1000
  print(f"samples={args.samples} median_ms={median_ms:.2f} p95_ms={p95_ms:.2f}")
  print(f"peak_rss_mib_median={statistics.median(rss_mib):.2f} peak_rss_mib_max={max(rss_mib):.2f}")
  return 0


if __name__ == "__main__":
  raise SystemExit(main())
