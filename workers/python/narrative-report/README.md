# Narrative report worker

For an installed OrgDoc release, install the matching worker bundle from the extracted core package:

```sh
./install-worker.sh narrative-report 0.1.0
```

Replace `0.1.0` with the OrgDoc release version. This installs the worker into its own environment and links `orgdoc-render-report` into `$HOME/.local/bin`. It requires Python 3.10 or newer and `uv` 0.11.8 or newer.

This worker uses only the Python standard library. Install its entry point in an isolated environment:

```sh
uv sync --python 3.12 --project workers/python/narrative-report
```

Add `workers/python/narrative-report/.venv/bin` to `PATH` for the `orgdoc` process. It writes `report.md` only in the request's fresh artifact directory. The selected brand, operation, document rules, language, voice tone, and primary color are included as Markdown front matter; source references, quotations, and citations are kept in separate sections.
