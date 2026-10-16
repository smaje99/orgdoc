# OrgDoc presentation worker

This isolated Python worker renders structured slide requests into editable `.pptx` files using `python-pptx`. It supports title and section slides, single-content and two-column layouts, text, raster images, tables, and native bar, line, and pie charts.

## Install

From this directory, install the locked worker environment and entry point:

```sh
uv sync --locked
uv run orgdoc-render-presentation < request.json
```

The worker requires Python 3.10 or newer. LibreOffice is optional; install it only when a request sets `validation.render_pdf: true`. That option emits an additional PDF preview and returns a clear diagnostic if LibreOffice is unavailable.

## Request and assets

The input contract is in `examples/organization-docs/capabilities/presentation-pptx/input.schema.json`; a full example is in `examples/quarterly-review.json`. Supply the organization, operation, brand, and document type context. Slide blocks are rendered in input order. Text and table/chart data stay editable in PowerPoint. Unsupported layouts and block types fail validation before a deck is written.

Image blocks refer to assets by ID. Asset paths must be relative to `organization_root`, resolve to regular files inside that root, and use PNG or JPEG. Brand logos use the same raster formats; unsupported logo formats are skipped. Brand typography is applied as a font family name and falls back to Aptos when no family is supplied. Availability of a named font depends on the machine opening or rendering the deck.

## Test

```sh
uv run pytest
```
