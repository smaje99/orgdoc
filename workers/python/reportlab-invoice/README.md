# ReportLab invoice worker

For an installed OrgDoc release, install the matching worker bundle from the extracted core package:

```sh
./install-worker.sh reportlab-invoice 0.1.0
```

Replace `0.1.0` with the OrgDoc release version. This installs the worker into its own environment and links `orgdoc-render-invoice` into `$HOME/.local/bin`. It requires Python 3.10 or newer and `uv` 0.11.8 or newer.

Install this worker in an isolated Python environment:

```sh
uv sync --python 3.12 --project workers/python/reportlab-invoice
```

Add `workers/python/reportlab-invoice/.venv/bin` to `PATH` for the `orgdoc` process. The Go CLI and its installation do not install ReportLab. The worker reads one OrgDoc JSON protocol request from standard input and writes one result object to standard output. It writes `invoice.pdf` only in the request's fresh artifact directory.

The worker uses typed invoice values, checks line arithmetic and totals, and applies the selected profile's colors and available SVG or raster logo. It uses ReportLab's built-in Helvetica faces when the profile's named fonts are not bundled with the organization.
