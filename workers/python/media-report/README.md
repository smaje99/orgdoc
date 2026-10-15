# Media report worker

For an installed OrgDoc release, install the matching worker bundle from the extracted core package:

```sh
./install-worker.sh media-report 0.1.0
```

Replace `0.1.0` with the OrgDoc release version. This one environment provides the `media-report` and `media-pdf` capabilities. It includes the compatible `media-report-cli` Python package and its templates. Python 3.11 or newer and `uv` 0.11.8 or newer are required. Pandoc and XeLaTeX or LuaLaTeX are additional prerequisites for PDF output.

For source development, install from the repository checkout:

```sh
uv sync --project workers/python/media-report
```
