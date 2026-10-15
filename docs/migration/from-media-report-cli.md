# Migrate from `media-report-cli` to OrgDoc Runtime

OrgDoc Runtime is the Go control plane for organization-aware documents. The Python distribution `media-report-cli`, import package `media_report`, and command `media-report` remain the compatibility implementation during the migration release. Keep using the Python command for workflows or settings without an OrgDoc replacement; the Go CLI does not read the legacy TOML file or provider environment variables.

## Command mapping

| Legacy command | OrgDoc equivalent | Migration notes |
| --- | --- | --- |
| `media-report --help` | `orgdoc --help` | Lists the new command surface; commands are not one-to-one aliases. |
| `media-report process PATH` | `orgdoc batch run JOBS.json` using `media-transcription`, followed by report/PDF jobs as needed | For a single source, submit one job. For a directory, use `media-scan` first; set `recursive` explicitly. `resume` and `overwrite` retain legacy semantics in the transcription worker. |
| `media-report transcribe PATH` | `orgdoc document render --operation media --document-type media-transcription ...` | Accepts a source file or directory. Schema-v2 artifacts remain beside the source; the OrgDoc run writes an index that references them. |
| `media-report report PATH` | `media-report` capability, followed by `media-pdf` when a PDF is needed | The report worker accepts a source path or reusable legacy artifact directory. `include_pdf` controls PDF rendering. Remote providers produce a warning. |
| `media-report doctor` | No core equivalent yet | Keep the Python command for checks of FFmpeg, Pandoc, TeX, providers, and legacy configuration. OrgDoc does not execute optional workers during startup or discovery. |
| `media-report config init [--force] [--path PATH]` | No user-level core configuration equivalent | Organization profiles are versioned repository files. Continue using the Python command and its `--force`/`--path` options to create the compatibility TOML file. |
| `media-report config show` | No core equivalent yet | Continue using the Python command; it redacts credentials. Organization profile inspection is available through `orgdoc context`. |
| `media-report templates list` | `orgdoc capabilities list` and `orgdoc capabilities describe ID` | Capabilities and document profiles are repository-scoped. The legacy command remains the way to list packaged Python prompts. |
| Hidden `media-report config example` | No public replacement | Keep using the legacy hidden command if a script depends on it. |

Common command options map to worker request fields:

| Legacy option | Worker request field | Notes |
| --- | --- | --- |
| `--recursive` | `recursive` | Defaults to false. |
| `--resume` | `resume` | Reuses completed stages after legacy metadata/artifact validation. |
| `--overwrite` | `overwrite` | Existing legacy behavior is resume-like; it does not mean an unconditional destructive replacement. |
| `--language` | `language` | Applies to transcription. |
| `--provider` | `provider` | Provider selection remains within the optional media report worker. Remote selection emits a warning. |
| `--model` | `model` | For transcription this selects the speech model; for report generation it selects the LLM model. |
| `--template` | `template` | Selects the existing report prompt/template. |
| `--output-format` | `output_format` | Selects Markdown or PDF where supported by the worker. |
| `--only-transcribe` | `media-transcription` only | Submit only the transcription capability. |
| `--only-report` | `media-report`, optionally followed by `media-pdf` | Requires reusable transcription artifacts. |

The Go batch format can execute independent document/media jobs with bounded resource classes. A single `document render` request is equivalent when no batch is needed. See the [runtime guide](../product/agent-document-runtime.md) for JSON examples.

## Configuration mapping

| Legacy TOML key | Environment override | OrgDoc migration target |
| --- | --- | --- |
| `llm.provider` | `MEDIA_REPORT_LLM_PROVIDER` | Optional media worker request `provider`. No global provider setting in the core. |
| `llm.model` | `MEDIA_REPORT_LLM_MODEL` | Optional media worker request `model`/`llm_model`. No global model setting in the core. |
| `llm.output_format` | `MEDIA_REPORT_OUTPUT_FORMAT` | Optional media worker request `output_format`. |
| `openai.base_url` | `MEDIA_REPORT_OPENAI_BASE_URL` | Keep in legacy user configuration for Python workers. |
| `openai.api_key` | `MEDIA_REPORT_OPENAI_API_KEY` | Keep in a secret environment variable or legacy user configuration; never put credentials in organization profiles or job JSON. |
| `ollama.base_url` | `MEDIA_REPORT_OLLAMA_BASE_URL` | Keep in legacy user configuration for Python workers. |
| `transcription.model` | `MEDIA_REPORT_WHISPER_MODEL` | Optional media transcription request `model`. |
| `transcription.device` | `MEDIA_REPORT_WHISPER_DEVICE` | Optional media transcription request `device`; effective device/fallback remains in schema-v2 metadata. |
| `logging.level` | `MEDIA_REPORT_LOG_LEVEL` | No equivalent core setting yet. OrgDoc returns structured command diagnostics. |

The legacy file remains at `${XDG_CONFIG_HOME:-$HOME/.config}/media-report/config.toml`. Do not copy provider secrets into the organization repository. OrgDoc organization profiles describe operations, brand appearance/voice, document schemas, protected fields, capabilities, and skills; they are not replacements for user-specific credentials or model endpoints.

## Artifact compatibility

The media workers continue to read and resume the established `<source_stem>_media_report/` directory with schema-v2 `metadata.json`, `pipeline.log`, and the existing audio, transcript, prompt, raw response, Markdown, and PDF files. They do not rewrite old runs into a new layout. The OrgDoc job directory contains a JSON index pointing to the legacy artifact paths. Keep backups and permissions for both locations if the source files are moved.

## Skills and brand consistency

Copy the organization's versioned skills with the profile repository. The included Humanizer skill is read by the agent before requested prose edits; it is not executed by `orgdoc` as a text-generation stage. Resolve the operation first so the agent uses exactly one brand voice. Validate edited input against its schema and keep protected fields such as amounts, identifiers, dates, quotations, references, and citations unchanged. `--pre-humanized-text` records source prose only when the organization enables that retention.

## Deprecation

During the migration release, keep the Python package and all public `media-report` commands available. Removal is reserved for a later major release after worker parity, a deprecation announcement, migration support, and release notes. Commands and settings without a replacement remain usable through the Python compatibility distribution until that cutover.
