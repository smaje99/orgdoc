# Worker setup and diagnostics

OrgDoc's Go runtime does not install Python workers. Install only the worker projects needed by the organization. From an extracted core release archive, run `./install-worker.sh WORKER_PROJECT_ID VERSION`; the installer downloads the matching source bundle, checks its SHA-256 digest, creates an isolated environment under the user data directory, and links its commands into `$HOME/.local/bin`.

Install Python 3 and `uv` before installing a Python worker. The current worker projects require Python 3.10 or newer for invoice/report output and Python 3.11 or newer for media. They require `uv` 0.11.8 or newer. For example, `uv python install 3.11` provisions a supported Python version through uv; the worker installer itself does not download Python runtimes (`UV_PYTHON_DOWNLOADS=never`).

| Capability | Worker project | Extra prerequisites |
| --- | --- | --- |
| `invoice-pdf` | `reportlab-invoice` | None beyond Python and uv |
| `report-markdown` | `narrative-report` | None beyond Python and uv |
| `media-scan` | `media-transcription` | None; scanning only inspects paths and media extensions |
| `media-transcription` | `media-transcription` | `ffmpeg` 4 or newer; model files are managed separately and are not installed by OrgDoc |
| `media-report` | `media-report` | An LLM provider only when generating a report; remote providers require their own credentials and network access |
| `media-pdf` | `media-report` | Pandoc 3 or newer, plus either XeLaTeX or LuaLaTeX |

On Debian or Ubuntu, the common media tools can be installed with `sudo apt install ffmpeg pandoc texlive-xetex`. On macOS, `brew install ffmpeg pandoc` installs FFmpeg and Pandoc; install a TeX distribution that provides `xelatex` or `lualatex` separately. Check availability with `command -v ffmpeg pandoc xelatex lualatex`. Package repositories can lag the recommended versions; `doctor` currently checks executable presence and prints the declared version note, but does not parse or enforce tool versions.

Run `orgdoc doctor --organization PATH --format json` to check the selected organization's capability manifests, worker entrypoints, declared executable prerequisites, and current platform. Human output is the default. Checks are deterministic and metadata-only: doctor does not execute workers, inspect models, download anything, or make network requests. Exit code `0` means checks have no missing or unsupported requirement; warnings such as no declared capabilities are informational. Exit code `1` means at least one requirement is missing/unsupported or a runtime check failed; usage errors return `2`.

Example:

```sh
orgdoc doctor --organization ./organization-docs --format json
```

Worker code runs with the current user's operating-system permissions. Capability `permissions` describe intended access; they do not restrict filesystem or network access. Install and run only worker code trusted by the organization.

Spreadsheet, presentation, and PDF form workers are planned capabilities and do not have release bundles yet. Their exact prerequisites will be added here when those workers are implemented.
