# OrgDoc Runtime

OrgDoc Runtime is a local Go CLI for agents and people that need to discover document capabilities, apply an organization's brand and document rules, and produce traceable artifacts. Organization repositories can define multiple operations and brands. The selected operation chooses the brand; the renderer records the profile, capability, and input versions used for each run.

The Go core is independent of Python, media models, and network services. Optional document and media workers run in isolated environments and are installed only when needed. The existing `media-report-cli` Python package and `media-report` command remain available as the media compatibility implementation during migration.

## Install OrgDoc

Download a release archive for Linux or macOS from the [GitHub Releases page](https://github.com/smaje99/media-report-cli/releases). Each archive includes the `orgdoc` executable, `INSTALL.md`, and `LICENSE`; `SHA256SUMS` is published alongside the archives.

To build all supported binaries from a source checkout:

```sh
scripts/build-release.sh
```

To verify an extracted Linux release package without Python or other workers on `PATH`:

```sh
scripts/smoke-release.sh build/release/orgdoc-0.1.0-linux-amd64.tar.gz
```

See [installation details](INSTALL.md) for checksum verification, install paths, supported targets, and worker setup.

## Start with an organization repository

Copy the example repository and validate it:

```sh
cp -R examples/organization-docs "$HOME/orgdoc-organization"
orgdoc organization validate --organization "$HOME/orgdoc-organization" --format json
orgdoc brands list --organization "$HOME/orgdoc-organization" --format json
orgdoc capabilities list --organization "$HOME/orgdoc-organization" --format json
```

The example defines Henko Technologies and Henko Academy brands, several operations, invoice and narrative-report profiles, and a Humanizer skill. Each operation selects one brand explicitly. Read [the product guide](docs/product/agent-document-runtime.md) for the profile format and agent workflows.

## Render a document

Install the invoice worker in its isolated environment, then make its entry point available on `PATH`:

```sh
uv sync --project workers/python/reportlab-invoice
export PATH="$PWD/workers/python/reportlab-invoice/.venv/bin:$PATH"
```

Render an example invoice request:

```sh
orgdoc document render \
  --organization "$HOME/orgdoc-organization" \
  --operation consulting \
  --document-type invoice \
  --input tests/fixtures/documents/invoice.json \
  --output-root ./artifacts \
  --format json
```

The narrative-report worker is also optional and has its own setup in [the report worker guide](workers/python/narrative-report/README.md). The CLI can render with a selected operation/brand without installing those workers; only the chosen renderer must be installed to produce its artifact.

## Use the Humanizer skill

Humanizer is an agent skill, not a renderer or an automatic text-rewriting stage. Before editing narrative prose, an agent should read the organization's `skills/humanizer.md`, resolve the operation and brand, and respect the document type's `protected_fields`. Preserve amounts, identifiers, dates, quotations, citations, and typed values. When enabled by the organization, `--pre-humanized-text PATH` records the original prose in the run manifest. See [the Humanizer policy](docs/product/humanizer-policy.md).

## Optional local media workflows

The media pack provides `media-scan`, `media-transcription`, `media-report`, and `media-pdf` capabilities. Set up only the worker environments required by a workflow:

```sh
uv sync --project workers/python/media-transcription
uv sync --project workers/python/media-report
```

See [the media worker guide](capabilities/media/README.md) for FFmpeg, Pandoc, TeX, model, and compatibility artifact requirements. Transcription loads a model only when a transcription job starts. OrgDoc does not download models or install worker environments automatically.

## Agent CLI

```text
orgdoc --help
orgdoc --version
orgdoc organization validate --organization PATH --format json
orgdoc capabilities list --organization PATH --format json
orgdoc capabilities describe ID --organization PATH --format json
orgdoc brands list --organization PATH --format json
orgdoc context --organization PATH --operation ID --document-type ID --format json
orgdoc document render --organization PATH --operation ID --document-type ID --input INPUT.json --output-root ARTIFACTS --format json
orgdoc batch run JOBS.json --max-workers 4 --format json
```

Every command accepts stable JSON output where supported. Batch results preserve input order, and resource-class limits keep GPU/model and memory-heavy workers at one active job by default. The [runtime guide](docs/product/agent-document-runtime.md) describes request schemas and chaining media capabilities.

## Compatibility and migration

The `media-report` command and Python distribution remain supported during the migration release. Existing commands and settings are documented in the [migration guide](docs/migration/from-media-report-cli.md), including the target OrgDoc command or capability and any setting that has no core replacement yet. Do not remove the Python package until a later major release after parity and migration communication.

## Development

Go checks:

```sh
gofmt -w cmd internal
go vet ./...
go test ./...
```

Legacy Python compatibility checks:

```sh
uv sync --extra dev
uv run ruff check .
uv run ruff format --check .
uv run pytest
uv run python -m build
uv run twine check dist/*
```

See [release checks](docs/release.md) and [AGENTS.md](AGENTS.md) for the migration and contribution rules.
