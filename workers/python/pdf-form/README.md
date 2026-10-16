# OrgDoc PDF form worker

This worker has two isolated CLI entry points. `orgdoc-generate-pdf-form` creates an editable AcroForm PDF from typed field definitions; `orgdoc-fill-pdf-form` fills a supplied AcroForm PDF while preserving its source. Both use the versioned OrgDoc JSON protocol on stdin/stdout.

## Install

From this directory, install the locked environment and entry points:

```sh
uv sync --locked
uv run orgdoc-generate-pdf-form < request.json
uv run orgdoc-fill-pdf-form < fill-request.json
```

Python 3.10 or newer is required. The worker uses ReportLab 5.0.1 for generation and pypdf 6.4.1 for form inspection, filling, and explicitly requested flattening.

## Generate

See `examples/contact-request.json`. The supported field types are text, checkbox, radio, and a single-select dropdown. A generated document has one A4 or Letter page. Field `x` and `y` coordinates are measured in PDF points from the upper-left of the page; labels are drawn above text and choice controls and beside checkbox/radio controls. Out-of-page fields and duplicate names are rejected before writing. Brand primary/accent colors and brand name are applied to document styling and metadata.

Generated controls remain interactive. Text and option defaults are optional; the worker emits a field summary containing only names, types, and whether each field starts populated.

## Fill an existing form

See `examples/fill-contact-request.json`. Store the source template under the organization repository and set `template_path` relative to its root. The worker rejects path escapes, symlinks resolving outside the organization, encrypted PDFs, XFA forms, malformed PDFs, unknown names, unsupported field types, and values that do not match the source field type or allowed options. It leaves the source file untouched and writes a new `completed-form.pdf` inside the artifact directory. Result summaries never include submitted field values.

Filling stays interactive by default. Set `flatten: true` only when a static copy is explicitly required; flattening removes all widgets and the AcroForm tree from the output. The source remains unchanged. Signed forms are not modified.

## Scope and limits

This worker does not apply digital signatures, certify PDF/UA accessibility, determine legal validity, or submit tax/electronic invoices. It supports AcroForm widgets only; XFA and signature fields are not fillable. Image assets and multi-page form layout are outside this initial version.

## Test

```sh
uv run pytest
```
