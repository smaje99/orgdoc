# Release and Distribution

OrgDoc Runtime is distributed as versioned Go release archives for Linux and macOS. The Python `media-report-cli` package remains a separate compatibility distribution on PyPI during the migration release; do not remove its packaging metadata as part of an OrgDoc binary release.

The Go core has no Python or ML dependency. Local document and media processing remains in optional workers. During development the media workers resolve the `media-report-cli` package from the checkout; their release bundles include the required compatibility package sources and metadata so installation does not depend on a repository checkout.

## OrgDoc Binary Release

Tag releases as `v<VERSION>` matching the root `VERSION` file. The GitHub release workflow runs Go and Python compatibility checks, builds four core archives, packages every catalogued worker as a separate source bundle, verifies one combined `SHA256SUMS`, installs and invokes every worker outside the checkout, and uploads the verified assets with generated release notes.

## Release candidate checks

Run the full candidate checks from a clean checkout before tagging. Use a temporary release directory if the default `build/release` contains old files:

```sh
VERSION=$(cat VERSION)
RELEASE_DIR=$(mktemp -d)
scripts/check-release-version.sh "v$VERSION"
go vet ./...
go test ./...
uv sync --extra dev
uv run ruff check .
uv run ruff format --check .
uv run pytest
uv run python -m build
uv run twine check dist/*
OUTPUT_DIR="$RELEASE_DIR" scripts/build-release.sh "$VERSION"
OUTPUT_DIR="$RELEASE_DIR" python scripts/package-workers.py "$VERSION"
python scripts/verify-release-assets.py "$VERSION" "$RELEASE_DIR"
(cd "$RELEASE_DIR" && sha256sum -c SHA256SUMS)
scripts/smoke-release.sh "$RELEASE_DIR/orgdoc-$VERSION-linux-amd64.tar.gz"
scripts/smoke-worker-bundles.sh "$VERSION" "$RELEASE_DIR"
```

The worker smoke installs each published bundle into a new temporary user-data directory, runs every installed entrypoint with a protocol probe, and uses an empty working directory outside the source checkout. It does not download ML models or install optional native tools such as FFmpeg, Pandoc, TeX, or LibreOffice. Worker package tests also reject model, cache, and credential paths in archives. Set `UV_PYTHON` if your local `uv` would otherwise choose a Python build missing required standard-library modules.

## Tag, publish, and review

1. Update the root `VERSION` and the unreleased section of `CHANGELOG.md`; review the compatibility and migration notes and ensure the working tree is ready for a release.
2. Run `scripts/check-release-version.sh "v$(cat VERSION)"`, then create and push the matching tag:

   ```sh
   VERSION=$(cat VERSION)
   git tag -a "v$VERSION" -m "OrgDoc Runtime $VERSION"
   git push origin "v$VERSION"
   ```

3. Wait for the `Release` workflow to pass. It creates the GitHub Release and attaches the four core archives, one source bundle per worker in `workers/catalog.json`, and `SHA256SUMS`. The action generates release notes from merged changes.
4. Review the release title and notes, confirm the asset set matches `scripts/verify-release-assets.py`, and download the manifest and archives for a final checksum check. SHA-256 detects accidental corruption; it does not authenticate the publisher.
5. Install or update OrgDoc using the verified release. Keep the previous version number available for rollback.

The release directory must contain exactly the expected Linux/macOS `amd64`/`arm64` core archives, the catalogued worker source bundles, and the combined checksum manifest. Do not attach local build logs, Python environments, model files, credentials, or caches.

## Install, update, and rollback

After downloading and verifying the release assets, a user can install OrgDoc from the repository checkout with:

```sh
scripts/install-orgdoc.sh 0.1.0
```

The installer downloads the matching OS/architecture archive and verifies its checksum before replacing `$HOME/.local/bin/orgdoc`. A direct archive install is documented in [INSTALL.md](../INSTALL.md). To update, run the installer with the new release version. To roll back, run it again with the previous release version; it reinstalls that version's binary. Worker installations are versioned separately: install the previous worker version with the archived `install-worker.sh` to point its command links back to that environment.

For a local candidate, set `ORGDOC_RELEASE_BASE_URL="file://$PWD/build/release"` when invoking either installer. This exercises checksum and platform selection without publishing a release.

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
