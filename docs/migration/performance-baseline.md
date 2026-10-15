# Migration Baseline and Compatibility Matrix

## Naming and release decision

- Product: **OrgDoc Runtime**
- Go module: `github.com/smaje/orgdoc`
- New command: `orgdoc`
- Existing distribution/import/command: `media-report-cli` / `media_report` / `media-report`
- Keep `media-report` as a compatibility shim for one migration release. Remove the Python package and command only in a later major release after parity, migration notes, and an announced deprecation path. Do not publish an `orgdoc` replacement package before the release gate.

These names are repository decisions, not a claim that distribution or trademark availability has been externally cleared.

See [from-media-report-cli.md](from-media-report-cli.md) for complete command, option, configuration-key, and environment-variable migration mappings.

## Current public command matrix

| Current command | Current behavior | Migration target | Compatibility decision |
| --- | --- | --- | --- |
| `media-report` / `--help` | Lists the command surface | `orgdoc --help` | Preserve help behavior; legacy alias remains through compatibility release |
| `process PATH` | Finds media, creates sibling artifact roots, prepares audio, transcribes, supports resume and report selectors | `orgdoc batch run` plus `capabilities` and media worker commands | Preserve flags/semantics through shim during parity; map `--recursive`, `--resume`, `--overwrite`, `--provider`, `--model`, `--language`, `--template`, `--output-format`, `--only-transcribe`, `--only-report` |
| `transcribe PATH` | Transcribes source media or reuses an artifact directory | Media transcription capability and batch job | Preserve input forms, `--language`, `--model`, scoped `--overwrite`, and trace metadata |
| `report PATH` | Generates Markdown/PDF from reusable transcription artifacts | `orgdoc document render` with report capability; media report worker feeds it | Preserve `--template`, `--provider`, `--model`, `--overwrite`, remote-provider warning, and reusable artifact behavior |
| `doctor` | Checks external tools, provider availability, templates, and config | `orgdoc doctor [--format human|json]` | Preserve as an additive diagnostic command; report optional workers distinctly |
| `config init [--force] [--path]` | Writes user config TOML | `orgdoc config init` during shim lifetime; organization repo initialization can be added separately | Preserve until compatibility release; map runtime configuration without exposing secrets |
| `config show` | Shows effective config with secrets redacted | `orgdoc config show [--format human|json]` | Preserve redaction and distinguish user runtime settings from organization profiles |
| `templates list` | Lists packaged prompt and PDF templates | `orgdoc capabilities list` and document profile discovery | Keep through compatibility release; discover organization document types and packaged templates separately |
| Hidden `config example` | Prints default config data | No public replacement | Keep hidden shim behavior if scripts depend on it; do not make it a new public command without need |

## Configuration and artifact compatibility

Current TOML sections and keys: `llm.provider`, `llm.model`, `llm.output_format`, `openai.base_url`, `openai.api_key`, `ollama.base_url`, `transcription.model`, `transcription.device`, and `logging.level`. Environment overrides are `MEDIA_REPORT_LLM_PROVIDER`, `MEDIA_REPORT_LLM_MODEL`, `MEDIA_REPORT_OPENAI_API_KEY`, `MEDIA_REPORT_OPENAI_BASE_URL`, `MEDIA_REPORT_OLLAMA_BASE_URL`, `MEDIA_REPORT_WHISPER_MODEL`, `MEDIA_REPORT_WHISPER_DEVICE`, `MEDIA_REPORT_OUTPUT_FORMAT`, and `MEDIA_REPORT_LOG_LEVEL`. The new core uses its own versioned runtime configuration; a compatibility adapter maps the above keys during the migration release and always redacts credentials.

The current media artifact root is `<source_stem>_media_report/`. Metadata is `metadata.json` with schema version 2; logs are `pipeline.log`. Outputs include extracted/normalized audio, raw/structured/clean transcripts, prompt text, raw LLM response, `report.md`, and `report.pdf`. Reuse requires metadata plus outputs for completed stages. Keep these names and meanings readable by the media compatibility workers; do not rewrite prior runs in place.

## Measured Python baseline

Measured 2026-10-14 on Linux x86_64, Ubuntu 24.04 kernel 6.17, Intel Core i7-8550U (4 cores / 8 threads), 23 GiB RAM, Python from the repository `.venv`. Each command ran five times with `/usr/bin/time`; RSS is maximum resident set in KiB. The first invocation and later warm process starts are included; OS filesystem caches were not explicitly dropped. These are process startup measurements, not model inference measurements.

| Command | Elapsed seconds (min / median / max) | Peak RSS (median) |
| --- | ---: | ---: |
| `media-report --help` | 0.70 / 0.73 / 0.85 | 42,376 KiB (41.4 MiB) |
| `media-report doctor` | 0.97 / 1.26 / 1.40 | 55,692 KiB (54.4 MiB) |
| `media-report templates list` | 0.64 / 0.69 / 0.74 | 39,988 KiB (39.1 MiB) |

Initial Go shell comparison on the same host (Go 1.27.1, one `/usr/bin/time` sample): `orgdoc --help` completed in `<0.01 s` at the timer's 0.01-second resolution and peaked at 2,808 KiB (2.74 MiB). This is below the 100 ms / 25 MiB gate and is about 73% lower RSS than the Python help median. Collect the planned 30+ samples before publishing p50/p95 claims.

Invocation: direct `.venv/bin/media-report`; a separate attempt via `uv run` was discarded because its cache directory was read-only in the execution environment. The repository has no isolated batch scheduler/no-op worker command, so a valid orchestration-only batch throughput or peak-RSS baseline cannot be measured without mixing media/model work. No model was downloaded or loaded for this baseline. Establish the fixed worker fixture and collect serial plus bounded-parallel measurements in Task 8.

## Performance gates

Use the same host, OS, fixture set, output sink, and measurement method for comparison. Measure at least 30 process starts for p50/p95 in release benchmarks; keep the five-run values above as the initial directional baseline.

- `orgdoc --help`: p50 at or below 100 ms and p95 at or below 150 ms; peak RSS at or below 25 MiB.
- `orgdoc doctor`, profile validation, and capability discovery without workers: p50 at or below 150 ms and p95 at or below 250 ms; peak RSS at or below 40 MiB.
- A four-worker run of 100 deterministic no-op worker jobs must achieve at least 2x serial throughput on the reference host, preserve stable result ordering, and keep coordinator RSS below 40 MiB. Report worker process RSS separately.
- For CPU, GPU/model, and memory-heavy workers, publish throughput and peak RSS by declared resource class and device. No universal inference throughput target: model load, hardware, device memory, model version, and workload must be stated.
- Core CLI startup and memory goals exclude worker/model RSS, but end-to-end batch peak RSS must also be recorded.

If the reference host changes, rerun the Python baseline and Go measurements on that host before making comparative release claims.
