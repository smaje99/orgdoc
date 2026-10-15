# OrgDoc scheduler and runtime benchmarks

These measurements separate core process startup, batch coordination, and worker process cost. Run them on the same machine and fixture set when comparing changes. Do not include customer documents or downloaded models.

## Defaults and fixtures

`orgdoc batch run JOBS.json` defaults to four total jobs. CPU jobs may use all four slots; GPU/model and memory-heavy jobs each default to one active job, still subject to the total limit. `--max-workers N` changes the global and CPU limits. The worker declares its `resource_class`; OrgDoc never infers that a model is safe to load concurrently.

`fixtures/noop-worker.sh` implements the JSON worker contract without model or document work. The scheduler benchmarks launch 100 real worker processes per batch and compare serial with four-worker scheduling. A separate mixed-class benchmark applies the CPU/GPU/memory limits to no-op processes. These fixtures measure process and scheduler overhead; they do not represent media decoding or model inference.

## Run the measurements

Build the runtime once, then collect at least 30 startup samples:

```sh
go build -o /tmp/orgdoc ./cmd/orgdoc
python3 benchmarks/measure_startup.py /tmp/orgdoc --samples 30
```

Measure 100 actual invoice render jobs serially and with four workers. This requires the isolated invoice environment described in `workers/python/reportlab-invoice/README.md`:

```sh
python3 benchmarks/measure_document_batch.py /tmp/orgdoc --jobs 100
```

Collect scheduler process-launch throughput and allocation data:

```sh
go test ./internal/scheduler -run '^$' -bench 'Benchmark(Noop|MixedResourceClass)' -benchmem -benchtime=3x -count=3
```

The startup helper reports GNU `/usr/bin/time` peak RSS for the CLI process. The document helper samples `/proc` for coordinator RSS and aggregate process-tree RSS; the process-tree value includes worker processes and should be reported separately. Preserve each raw output with the commit, machine details, Go version, Python version, ReportLab/svglib versions, worker versions, and job count.

## Reference run

Measured 2026-10-14 on Linux x86_64, kernel 6.17.0-40-generic, Intel Core i7-8550U (4 cores / 8 threads), 23 GiB RAM; Go 1.27.1, Python 3.12.3, ReportLab 5.0.1, svglib 2.3.0. Values below are one controlled run; rerun the documented commands before release comparisons.

| Workload | Workers | Elapsed | Throughput | Coordinator peak RSS | Process tree peak RSS |
| --- | ---: | ---: | ---: | ---: | ---: |
| `orgdoc --help`, 30 samples | 1 | p50 10.60 ms / p95 14.23 ms | — | max 7.21 MiB | — |
| 100 actual invoice PDF jobs | 1 | 46.82 s | 2.14 jobs/s | 12.07 MiB | 58.39 MiB |
| 100 actual invoice PDF jobs | 4 | 21.23 s | 4.71 jobs/s (2.20x serial) | 12.62 MiB | 188.56 MiB |
| 100 no-op worker jobs | 1 | 646 ms | 154.8 jobs/s | — | — |
| 100 no-op worker jobs | 4 | 210 ms | 475.4 jobs/s (3.07x serial) | — | — |
| 100 mixed-class no-op jobs | CPU 4, GPU 1, memory 1 | 261 ms | 382.5 jobs/s | — | — |
| 100 mixed invoice PDF and local media-scan jobs | 1 | 51.14 s | 1.96 jobs/s | 12.16 MiB | 58.47 MiB |
| 100 mixed invoice PDF and local media-scan jobs | 4 | 21.03 s | 4.76 jobs/s (2.43x serial) | 12.94 MiB | 154.85 MiB |

The CLI startup gate, four-worker no-op throughput gate, invoice throughput comparison, mixed invoice/media-scan throughput comparison, and coordinator memory gate passed in this run. Process-tree RSS includes the Python workers and is shown separately; the larger four-worker value is expected from concurrent isolated worker processes. The no-op mixed-class figure only checks resource scheduling behavior. The mixed invoice/media-scan workload exercises local file discovery but does not decode audio or load a model; it is not an inference benchmark.

Model-load and inference measurements remain deferred until a named local model/device and representative media fixture are selected. No model was downloaded or loaded for this benchmark.

Release gates from Task 1 remain: `orgdoc --help` p50 <= 100 ms, p95 <= 150 ms, RSS <= 25 MiB; 100 deterministic no-op worker jobs at four workers achieve >= 2x serial throughput; coordinator RSS stays below 40 MiB. Model and media results must name their hardware, device, model version, and worker version.
