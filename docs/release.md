# Release Notes

## Scope

This project is packaged for PyPI and should always be validated as an installable CLI, not only as a source checkout.

OrgDoc is migrating the core control plane to Go. Keep `media-report` installed as the Python compatibility entrypoint during the migration release. Local media processing is an optional worker pack; the Go runtime does not install Python or machine-learning packages. The isolated worker environments resolve the current `media-report-cli` source package during this compatibility phase and therefore need this repository checkout.

## Release Checklist

1. Run `uv sync --extra dev`.
2. Run `uv run pytest`.
3. Run `uv run ruff check .`.
4. Run `uv run ruff format --check .`.
5. Run `uv run python -m build`.
6. Run `uv run twine check dist/*`.
7. Verify `uv tool install .`.
8. Verify `pipx install .`.
9. Run `media-report doctor`.
10. Smoke test `media-report templates list` and `media-report process tests/fixtures/example.mp3`.

## Platform Policy

- Linux and macOS are the supported release targets.
- Windows is experimental and should not block release unless a regression was explicitly introduced there.

## Packaging Constraints

- Keep runtime resources inside `src/media_report/templates`.
- Load resources through `importlib.resources`.
- Do not add heavyweight external tools to Python runtime dependencies.
- Maintain the public console script name `media-report`.

## Optional OrgDoc Media Workers

From the repository root, provision each isolated environment explicitly:

```sh
uv sync --project workers/python/media-transcription
uv sync --project workers/python/media-report
```

Add the corresponding `.venv/bin` directory to `PATH`, then copy the worker capability manifests and media document profiles from `examples/organization-docs` into the organization's repository. `orgdoc capabilities list --organization PATH --format json` will show the installed declarations. FFmpeg is required for extraction/normalization; Pandoc and XeLaTeX or LuaLaTeX are required for PDF output. faster-whisper downloads/loads a model only when a transcription job runs. No automatic model downloads or worker environment installs occur from the Go CLI.

`media-report` remains the compatibility path and continues to use the existing Python command surface. Its metadata schema v2, sibling `<source_stem>_media_report/` layout, stage status, and resume behavior remain readable. OrgDoc workers publish an index in their run directory while preserving those artifacts at their established paths. For a remote LLM provider, the report worker emits an explicit warning in its diagnostic result.
