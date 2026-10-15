# Media transcription worker

For an installed OrgDoc release, install the matching worker bundle from the extracted core package:

```sh
./install-worker.sh media-transcription 0.1.0
```

Replace `0.1.0` with the OrgDoc release version. This one environment provides the `media-scan` and `media-transcription` capabilities. It includes the compatible `media-report-cli` Python package and its resources. Python 3.11 or newer, `uv` 0.11.8 or newer, and FFmpeg are required. Model files are not bundled or downloaded by the installer.

For source development, install from the repository checkout:

```sh
uv sync --project workers/python/media-transcription
```
