# Release and Distribution

OrgDoc Runtime is distributed as versioned Go release archives for Linux and macOS. The Python `media-report-cli` package remains a separate compatibility distribution on PyPI during the migration release; do not remove its packaging metadata as part of an OrgDoc binary release.

The Go core has no Python or ML dependency. Local document and media processing remains in optional workers. The current media worker environments resolve the `media-report-cli` source package and therefore need this repository checkout during this compatibility phase.

## OrgDoc Binary Release

Tag releases as `v<VERSION>` matching the root `VERSION` file. The GitHub release workflow runs Go and Python compatibility checks, builds four archives, verifies a Linux archive in a clean environment, and uploads the archives with `SHA256SUMS`.

For a local release candidate, run:

```sh
go vet ./...
go test ./...
uv sync --extra dev
uv run ruff check .
uv run ruff format --check .
uv run pytest
uv run python -m build
uv run twine check dist/*
scripts/build-release.sh
(cd build/release && sha256sum -c SHA256SUMS)
scripts/smoke-release.sh "build/release/orgdoc-$(cat VERSION)-linux-amd64.tar.gz"
INSTALL_DIR="$(mktemp -d)" ORGDOC_RELEASE_BASE_URL="file://$PWD/build/release" scripts/install-orgdoc.sh "$(cat VERSION)"
```

Review the invoice PDFs for each configured brand at normal and high zoom, including long line wrapping, page breaks, margins, typography, footer placement, and brand colors. Review each narrative report in a Markdown preview and confirm its front matter, source references, and run manifest identify the selected brand and profile versions. If a report-to-PDF renderer is added, include its output in this visual release gate.

Keep `pyproject.toml` and the `media-report` console entry point for the compatibility period. A future major-version task may remove them only after command/configuration migration is communicated and the media artifact parity gate is accepted.

## Optional OrgDoc Workers

From the repository root, provision only the isolated environments needed by the selected capabilities:

```sh
uv sync --project workers/python/reportlab-invoice
uv sync --project workers/python/narrative-report
uv sync --project workers/python/media-transcription
uv sync --project workers/python/media-report
```

Add the selected `.venv/bin` directory to `PATH`, then copy the corresponding capability manifests, document profiles, schemas, operations, brands, and skills from `examples/organization-docs` into the organization's repository. `orgdoc capabilities list --organization PATH --format json` shows the declared capabilities. Renderer and report worker setup is documented in each `workers/python/*/README.md`. FFmpeg is required for extraction/normalization; Pandoc and XeLaTeX or LuaLaTeX are required for PDF output. faster-whisper loads a model only when a transcription job runs. The Go CLI does not download models or install worker environments.

`media-report` remains the compatibility path and continues to use the existing Python command surface. Its metadata schema v2, sibling `<source_stem>_media_report/` layout, stage status, and resume behavior remain readable. OrgDoc workers publish an index in their run directory while preserving those artifacts at their established paths. For a remote LLM provider, the report worker emits an explicit warning in its diagnostic result. See the [migration guide](migration/from-media-report-cli.md) for command and configuration mappings.
