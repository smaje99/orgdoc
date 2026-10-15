# OrgDoc spreadsheet worker

Install the matching worker bundle from the extracted OrgDoc release archive:

```sh
./install-worker.sh spreadsheet 0.1.0
```

The worker requires Python 3.10 or newer and `uv` 0.11.8 or newer. It writes an editable `.xlsx` workbook from typed JSON cells, applies the selected brand's colors, and supports Excel formulas, number formats, worksheet tables, and bar, line, or pie charts.

Copy `examples/quarterly-revenue.json` to an input file and render it with an organization that has the `spreadsheet-xlsx` capability and `spreadsheet` document type:

```sh
orgdoc document render \
  --organization ./organization-docs \
  --operation consulting \
  --document-type spreadsheet \
  --input quarterly-revenue.json \
  --output-root ./generated-documents
```

The request schema requires an explicit `formula_policy`. With `formula_policy: spreadsheet_app`, the worker writes formulas and marks the workbook for recalculation when opened in a spreadsheet application. OrgDoc does not compute or report formula results in this mode. Set `formula_policy: libreoffice` to request recalculation, or `validation.render_pdf: true` to request a PDF preview. These options require `libreoffice` or `soffice` on `PATH`; the worker returns a clear diagnostic if requested and unavailable. The PDF preview is an additional artifact; the `.xlsx` remains the editable source. An organization can restrict the policy in its request schema when every workbook must follow the same rule.

Only the declared typed cells and chart types are supported. Formula inputs that reference external workbooks or invoke external-content and macro functions are rejected. Sheet protection prevents accidental edits in spreadsheet applications; it is not a security boundary. Brand raster images are embedded when present; SVG logos are not embedded by this worker. Typography uses spreadsheet application defaults with Aptos as the preferred font.

For local development:

```sh
uv sync --extra dev
uv run pytest
```
