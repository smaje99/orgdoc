# Agent Instructions

## Product and Migration State

The target product is OrgDoc Runtime: a local Go CLI for agent-discoverable document capabilities, organization-specific brand consistency, and traceable artifacts. The Go module is `github.com/smaje/orgdoc`; its command is `orgdoc`. The first documents are invoice PDF and narrative Markdown report. Media workflows are optional capabilities. See `docs/product/agent-document-runtime.md` and the migration plan for scope and gates.

The existing `media-report-cli` Python package (`media_report`, command `media-report`) remains the compatibility implementation during migration. Do not remove or republish it under a new name before the compatibility release and later major-version cutover criteria pass.

## Target Architecture

- Keep the Go core separated into CLI, application orchestration, domain/profile rules, and infrastructure.
- Keep language-specific renderers and media processing in independently managed workers.
- Use versioned JSON over stdin/stdout for worker requests and results. Send progress/diagnostics to stderr.
- Execute declared worker entry points directly, without shell interpolation. Enforce timeouts, cancellation, resource-specific concurrency, and per-job artifact roots.
- Validate organization and document configuration before starting workers. Resolve assets relative to the organization repository.
- Keep brand appearance, document business rules, and agent skills as distinct inputs.
- The core CLI must not require Python, ML dependencies, model loading, or network access for help, version, validation, and capability discovery.

## Compatibility Rules

- Preserve `media-report` as a shim for one migration release.
- Keep current public workflows and artifact semantics available through that release; document any command or config key without a replacement.
- Remove the old Python packaging only in a later major release after parity, migration notes, and release communication.
- Linux and macOS are official targets; Windows remains experimental until validated.

## Security and Data Integrity

- Never print API keys or tokens. Redact secrets in config output, metadata, logs, and diagnostics.
- Prefer local execution and make remote LLM use explicit and visible.
- Reject ambiguous operation/brand resolution and unsupported schema versions before rendering.
- Reject worker output paths outside the job directory. Failed runs preserve prior successful artifacts.
- Humanizer may edit requested narrative prose only. Preserve amounts, identifiers, dates, quotations, citations, and typed values.

## Testing and Verification

- Add unit tests for domain, schema, worker, scheduler, and adapter behavior.
- Add CLI integration tests when public command behavior changes.
- Keep heavy external dependencies mocked unless a test is explicitly an environment integration.
- Use test-first development for code changes and run the task-specific verification plus relevant full suite before marking a migration task complete.
- Benchmark core startup, coordinator memory, worker RSS, and model/device work separately.

## Development Commands

- Go: `gofmt -w`, `go vet ./...`, `go test ./...`
- Legacy Python while present: `uv sync --extra dev`, `uv run pytest`, `uv run ruff check .`, `uv run ruff format .`, `uv run python -m build`, `uv run twine check dist/*`

## Commit Conventions

Use focused conventional prefixes such as `feat:`, `fix:`, `docs:`, `test:`, or `chore:`. Keep packaging, docs, and code changes separated where practical.
