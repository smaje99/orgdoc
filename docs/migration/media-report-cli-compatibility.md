# `media-report-cli` Python compatibility package

`media-report-cli` remains the Python command-line package for local media workflows during the OrgDoc Runtime migration. Install it with `uv tool install media-report-cli` or `pipx install media-report-cli`; the console command remains `media-report` and the import package remains `media_report`.

The package accepts local audio/video files, uses FFmpeg for audio preparation, optionally installs faster-whisper for transcription, and writes traceable artifacts beside each source. The `process`, `transcribe`, `report`, `doctor`, `config`, and `templates` commands retain their legacy behavior for the compatibility period. See [the migration guide](from-media-report-cli.md) for command, option, and configuration mappings.

This package does not install the OrgDoc Go runtime or its renderer workers. Its configuration remains at `${XDG_CONFIG_HOME:-$HOME/.config}/media-report/config.toml`, with provider secrets redacted by `media-report config show`. Do not place secrets in organization profiles or job files.
