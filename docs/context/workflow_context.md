# Workflow Context

## Pipeline Shape

1. Validate the input path.
2. Discover supported media files.
3. Create an artifact directory for each media file.
4. Extract audio when the source is video.
5. Normalize audio for transcription.
6. Transcribe and persist raw plus structured transcript data.
7. Clean and segment transcript text.
8. Render a prompt from a packaged template.
9. Generate a report through a configured LLM provider.
10. Convert Markdown to PDF.

## Artifact Policy

Each source file gets a sibling artifact directory named:

```text
<source_stem>_media_report/
```

Expected artifacts include:

- `metadata.json`
- `pipeline.log`
- `audio_extracted.wav`
- `audio_normalized.wav`
- `transcript_raw.txt`
- `transcript_segments.json`
- `transcript_clean.md`
- `prompt_used.md`
- `llm_response_raw.txt`
- `report.md`
- `report.pdf`

## Failure Handling

- Preserve upstream artifacts if a later stage fails.
- Keep stage status in metadata for future resume support.
- Never lose Markdown when PDF generation fails.

## OrgDoc Optional Worker Pack

The `media-scan` capability discovers supported local files without loading a model. `media-transcription` accepts a file or directory. Directory scans are sorted by path; recursion is opt-in. It runs extraction, normalization, and transcription stages and supports resume. GPU/model and memory-heavy scheduling is limited to one active job by default. The existing faster-whisper adapter records requested/effective device and any automatic GPU-to-CPU fallback in schema-v2 `metadata.json`.

The `media-report` capability consumes a source file or reusable artifact directory and runs the report stage. `media-pdf` renders a completed Markdown report without regenerating its LLM content. The workers reuse existing transcript artifacts and preserve `prompt_used.md`, `llm_response_raw.txt`, `report.md`, and `report.pdf` where produced. Selecting OpenAI or another remote-compatible provider adds a worker diagnostic warning that report content may leave the machine; credentials stay in user configuration or environment and are not copied into OrgDoc run summaries.

Both workers are optional Python packages under `workers/python/`; their setup instructions are in `capabilities/media/README.md`. Their per-job OrgDoc output contains a `media-result.json` index that points to the compatibility artifact directory beside the source media. The Go runtime and invoice/report document workers do not import Python or load speech models.
