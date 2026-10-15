# Optional media capability pack

The organization-facing manifests live under `examples/organization-docs/capabilities/` so the core runtime can discover them with its existing repository-scoped registry. Copy these capability directories, document type profiles, request schemas, and the `media` operation into an organization repository to enable local media workflows. The scan, transcription, report, and PDF stages are separate discoverable capabilities; dependent work should be submitted in successive batches.

The Python entrypoints are installed independently:

```sh
uv sync --project workers/python/media-transcription
uv sync --project workers/python/media-report
```

Add the relevant worker environment's `bin` directory to `PATH` when running OrgDoc. The transcription worker installs the project's optional `transcription` extra; the Go core installs neither environment. FFmpeg and, for PDF output, Pandoc plus XeLaTeX or LuaLaTeX remain system dependencies. Models load only when a transcription job starts. No setup command downloads a model automatically.

The workers wrap the current Python application services to preserve schema-v2 metadata and resume behavior during migration. Their `media-result.json` output records the stable paths of the legacy artifact tree beside each source file. The `filesystem:write_media_artifacts` permission marks that sidecar write explicitly. Remote LLM providers emit a diagnostic warning; credentials are read from the existing user configuration/environment and are never written into the summary.

For a GitHub Release installation, use the `install-worker.sh` included in the extracted core archive. Install `media-transcription` when scanning/transcription is needed and `media-report` when generating reports or PDFs is needed. These are worker project IDs, and each project supplies two related capability entrypoints. `media-transcription` requires Python 3.11+, `uv` 0.11.8+, and FFmpeg. `media-report` requires Python 3.11+ and `uv` 0.11.8+; PDF output also requires Pandoc and XeLaTeX or LuaLaTeX. Model weights and external system tools are not bundled or installed automatically.
