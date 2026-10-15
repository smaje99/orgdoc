# Narrative report worker

This worker uses only the Python standard library. Install its entry point in an isolated environment:

```sh
uv sync --python 3.12 --project workers/python/narrative-report
```

Add `workers/python/narrative-report/.venv/bin` to `PATH` for the `orgdoc` process. It writes `report.md` only in the request's fresh artifact directory. The selected brand, operation, document rules, language, voice tone, and primary color are included as Markdown front matter; source references, quotations, and citations are kept in separate sections.
