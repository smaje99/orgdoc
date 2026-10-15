# ReportLab invoice worker

Install this worker in an isolated Python environment:

```sh
uv sync --python 3.12 --project workers/python/reportlab-invoice
```

Add `workers/python/reportlab-invoice/.venv/bin` to `PATH` for the `orgdoc` process. The Go CLI and its installation do not install ReportLab. The worker reads one OrgDoc JSON protocol request from standard input and writes one result object to standard output. It writes `invoice.pdf` only in the request's fresh artifact directory.

The worker uses typed invoice values, checks line arithmetic and totals, and applies the selected profile's colors and available SVG or raster logo. It uses ReportLab's built-in Helvetica faces when the profile's named fonts are not bundled with the organization.
