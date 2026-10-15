# Distribution Context

## Distribution Targets

- OrgDoc Runtime core: checksummed GitHub release archives for Linux and macOS.
- Python compatibility command: PyPI distribution `media-report-cli` during the migration release.
- Optional workers: isolated, explicit environment setup under `workers/python/`.

## Installation Modes

- `scripts/install-orgdoc.sh VERSION` or download a verified GitHub release archive.
- `uv tool install media-report-cli` for the compatibility command.
- `pipx install media-report-cli` for the compatibility command.
- `pip install media-report-cli` for the compatibility library.

The Python tool installs are recommended only when the legacy media-report workflow is needed. The core OrgDoc CLI is a standalone Go binary and includes no Python dependencies.

## Packaging Requirements

- Console script: `media-report`
- Import package: `media_report`
- Resources bundled under `src/media_report/templates`
- Wheel and sdist must include prompt and PDF templates

## Validation

Every release should verify:

- `uv tool install .`
- `pipx install .`
- `uv run python -m build`
- `uv run twine check dist/*`
