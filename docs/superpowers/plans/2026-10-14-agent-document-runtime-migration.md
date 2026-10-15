# Agent Document Runtime Migration Implementation Plan

> **For agentic workers:** Use the `executing-plans` skill to implement this plan task by task. Keep the migration incremental and do not remove the existing CLI until the new runtime has passed the cutover criteria.

**Goal:** Evolve `media-report-cli` into a fast, local, agent-facing document runtime that applies versioned organizational brand profiles and document rules, while retaining multimedia processing as an optional capability pack.

**Architecture:** Replace the Python CLI/orchestrator with a small Go control plane that discovers organization configuration, publishes machine-readable capabilities, schedules bounded work, and invokes language-neutral workers over a JSON process contract. Keep document and media operations in separately packaged workers; use Python where libraries such as ReportLab or faster-whisper provide concrete value. Treat skills as agent workflow instructions and structured configuration/renderers as the enforcement mechanism for brand and document invariants.

**Tech Stack:** Go CLI and orchestration; versioned YAML organization profiles validated against JSON Schema; JSON over stdin/stdout for worker contracts; Python worker environments managed independently from the Go binary; ReportLab for the initial PDF invoice worker; portable Agent Skills; optional MCP adapter after the CLI contract stabilizes.

**Spec:** This plan is the migration specification. It supersedes the Python-only product direction in `docs/context/product_context.md` and `docs/context/technical_context.md` after Task 1 is accepted.

## Global Constraints

- Linux and macOS remain the official targets; Windows remains experimental until separately validated.
- Preserve local-first operation; remote LLM use must be explicit and visible to the user or agent.
- Do not publish a replacement package or remove the `media-report` command before migration parity and cutover criteria pass.
- Keep Python optional and capability-scoped; the Go CLI must start and answer help, version, configuration, and capability discovery without importing or starting Python.
- Worker communication uses versioned JSON contracts, direct process execution, and no shell command interpolation.
- Never select a brand implicitly when the requested operation maps to multiple brands; require an explicit operation or brand identifier.
- Brand appearance, document business rules, and agent writing instructions are separate versioned inputs.
- Humanization may revise narrative prose only; financial values, identifiers, dates, quotations, source references, and other structured fields must remain unchanged.
- A document artifact records the selected operation, brand profile version, document profile version, worker identity/version, and input references needed to reproduce or audit the run.
- Keep the existing Python CLI runnable during migration; move or delete it only in the final cutover task.

## Review Focus

- Ambiguous or missing operation/brand selection: the CLI must stop with an actionable error instead of guessing.
- Invalid or newer organization schema: the CLI must report the schema version and validation location without partially rendering output.
- Worker missing, timing out, returning invalid JSON, or exiting nonzero: preserve existing artifacts and return a structured failure result.
- Batch concurrency exceeding memory/device capacity: enforce configured limits and cancel/skip work predictably.
- Humanizer modifying protected data or invoice fields: reject the result or restore protected values before rendering.

---

## Product Definition

### Positioning

An organization keeps its brand profiles, document rules, reusable skills, and capability declarations in a versioned repository. An agent discovers those capabilities, selects the correct operation and brand, prepares content, and asks the runtime to render and validate the artifact.

The product is not a general-purpose writing agent. Its core value is repeatable document consistency, discoverability, local execution, bounded batch processing, and traceable outputs. Multimedia transcription and reporting are secondary capabilities that can feed this document workflow.

### Organization Repository Model

The initial repository layout is:

```text
organization-docs/
  organization.yaml
  brands/
    henko.yaml
    secondary-brand.yaml
    assets/
  operations/
    consulting.yaml
    training.yaml
  document-types/
    invoice.yaml
    report.yaml
  skills/
    humanizer/SKILL.md
    documents/SKILL.md
  capabilities/
    invoice-pdf/capability.yaml
    media-transcription/capability.yaml
```

`organization.yaml` declares schema version and repository identity. Operations refer to a brand ID and can provide operation-specific organization/contact data. Brand profiles contain visual tokens and voice guidance. Document profiles define required fields, protected fields, layouts, and renderer capability IDs. Skills give agents procedural guidance and link to the relevant profiles and capabilities.

### Worker Contract

Each capability manifest declares a stable ID, version, description, input and output JSON Schemas, runtime, executable entry point, resource class, supported platforms, and required permissions. The Go CLI sends one versioned JSON request on stdin and reads one versioned JSON result from stdout. Progress and diagnostics go to stderr. File artifacts are written only under a runtime-provided job directory and are returned as relative paths plus media types.

The first worker runtimes are `python` and native executables. The manifest is trusted organization code: the CLI does not evaluate arbitrary shell strings. Execution has timeouts, cancellation, configurable concurrency, and per-capability filesystem roots. Automatic dependency installation is out of scope for the first release; missing worker environments produce a clear setup diagnostic.

### CLI Contract

The proposed new command name is `orgdoc` as a working name only; select the final distribution and command names in Task 1 before packaging work.

- `orgdoc capabilities list --format json`
- `orgdoc capabilities describe <id> --format json`
- `orgdoc organization validate <repo>`
- `orgdoc brands list [--organization <repo>] --format json`
- `orgdoc context <operation> --document-type <type> --format json`
- `orgdoc document render --operation <id> --type <type> --input <json-file>`
- `orgdoc batch run <job-file> --max-workers <n>`
- `orgdoc doctor [--format json]`

Human-readable output is for direct users. Every command used by agents supports stable JSON output, noninteractive behavior, and documented exit codes. MCP is a later adapter over the same capability registry; it must not contain a second implementation of business workflows.

---

## Migration Tasks

### Task 1: Lock product scope, naming, baseline, and compatibility policy

**Files:**
- Create: `docs/product/agent-document-runtime.md`
- Modify: `docs/context/product_context.md`
- Modify: `docs/context/technical_context.md`
- Modify: `docs/context/architecture_context.md`
- Modify: `docs/context/roadmap.md`
- Modify: `docs/media-report-cli-roadmap.md`
- Modify: `AGENTS.md`
- Create: `docs/migration/performance-baseline.md`

**Interfaces:**
- Consumes: current CLI behavior, existing artifact layout, this migration specification.
- Produces: accepted product scope, final product/package/command names, baseline methodology, and a compatibility matrix for all current public commands.

- [x] Inventory current public commands, config keys, metadata files, generated artifacts, and supported workflows in a compatibility matrix.
- [x] Measure startup and RSS for `--help`, `doctor`, and `templates list`; record why an orchestration-only batch baseline is not available and define the Task 8 fixture method.
- [x] Select the product name (**OrgDoc Runtime**), Go module (`github.com/smaje/orgdoc`), and CLI command (`orgdoc`).
- [x] Rewrite product and technical context to make organization-wide document consistency the primary product and media processing an optional capability pack.
- [x] Define release/cutover policy: preserve `media-report` as a compatibility shim for one migration release; remove it only in a later major release after command parity and user-facing migration notes.
- [x] Record measurable startup, memory, and batch goals against the baseline, separating core orchestration from model/device work.
- [x] Review the compatibility matrix and goals before implementation begins.

**Acceptance:** The docs state one product promise, an explicit supported first document set (invoice PDF and narrative report), a final command/package name, quantified or hardware-qualified performance goals, and a command-by-command migration decision.

### Task 2: Establish the Go CLI shell and machine-readable command contract

**Files:**
- Create: `go.mod`
- Create: `cmd/<command>/main.go`
- Create: `internal/cli/root.go`
- Create: `internal/cli/output.go`
- Create: `internal/cli/exitcodes.go`
- Create: `internal/cli/*_test.go`
- Create: `.github/workflows/go.yml`
- Modify: `Makefile`

**Interfaces:**
- Consumes: final command name and compatibility policy from Task 1.
- Produces: `Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int`, version/help handling, `--format human|json`, and stable exit-code constants.

- [x] Add tests for no-argument help, `--help`, `--version`, JSON error shape, and documented exit codes.
- [x] Implement a minimal Go command that passes those tests without importing organization config or worker packages.
- [x] Add Go formatting, vetting, and test commands to the developer workflow and CI.
- [x] Measure `--help` startup/RSS against Task 1 baseline and record results.

**Acceptance:** The binary handles help/version without Python, model, network, or organization repository initialization; machine-readable errors have a stable envelope.

### Task 3: Define and validate organization, operation, brand, and document profiles

**Files:**
- Create: `schemas/organization.schema.json`
- Create: `schemas/brand.schema.json`
- Create: `schemas/operation.schema.json`
- Create: `schemas/document-type.schema.json`
- Create: `internal/orgconfig/model.go`
- Create: `internal/orgconfig/load.go`
- Create: `internal/orgconfig/validate.go`
- Create: `internal/orgconfig/*_test.go`
- Create: `examples/organization-docs/organization.yaml`
- Create: `examples/organization-docs/brands/henko.yaml`
- Create: `examples/organization-docs/operations/consulting.yaml`
- Create: `examples/organization-docs/document-types/invoice.yaml`
- Create: `examples/organization-docs/document-types/report.yaml`

**Interfaces:**
- Consumes: schema version and field decisions recorded in Task 1.
- Produces: `LoadOrganization(root string) (Organization, error)`, `ValidateOrganization(root string) []Diagnostic`, `ResolveProfile(root, operationID, documentType string) (ResolvedProfile, error)`.

- [x] Write schema fixtures for valid organizations, multiple brands, multiple operations sharing a brand, missing brand references, invalid asset paths, unsupported schema versions, and ambiguous selection.
- [x] Implement profile loading with paths resolved relative to the organization repository, never the installed binary or current source checkout.
- [x] Validate all profile references before rendering; return file and field locations in diagnostics.
- [x] Implement explicit profile resolution; reject missing/ambiguous operation IDs.
- [x] Add an example organization with two brands and three operations: two share one brand and the third selects the other brand.

**Acceptance:** Profile validation is deterministic, can run offline, and rejects invalid or ambiguous configuration before any worker starts or output is created.

### Task 4: Add capability discovery and the versioned worker runner

**Files:**
- Create: `schemas/capability.schema.json`
- Create: `internal/capability/manifest.go`
- Create: `internal/capability/registry.go`
- Create: `internal/worker/protocol.go`
- Create: `internal/worker/runner.go`
- Create: `internal/worker/runner_test.go`
- Create: `internal/cli/capabilities.go`
- Create: `internal/cli/capabilities_test.go`
- Create: `tests/fixtures/workers/`

**Interfaces:**
- Consumes: JSON Schema validation and diagnostics from Task 3.
- Produces: `Discover(root string) ([]Capability, error)`, `Describe(root, id string) (Capability, error)`, `Run(ctx context.Context, capability Capability, request WorkerRequest) (WorkerResult, error)`.
- Worker protocol: request/result envelope has `protocol_version`, `job_id`, `operation_id`, `brand_id`, `document_type`, `input`, and `artifact_dir`; result has `status`, `outputs`, and structured diagnostics.

- [x] Add fixture workers for success, malformed JSON, timeout, nonzero exit, oversized output, and path traversal attempts.
- [x] Implement registry discovery and JSON capability listing/description.
- [x] Implement direct executable invocation with stdin/stdout JSON, stderr capture, context cancellation, timeout, and no shell evaluation.
- [x] Restrict returned artifact paths to the job directory; reject absolute paths and parent traversal.
- [x] Add concurrency semaphore keyed by resource class and configured worker limit.
- [x] Verify a failed worker cannot overwrite or delete prior artifact directories.

**Acceptance:** An agent can discover/describe capabilities as JSON, and the runner enforces protocol, cancellation, output location, and configured concurrency limits.

### Task 5: Add brand-aware document context and renderer orchestration

**Files:**
- Create: `internal/document/request.go`
- Create: `internal/document/context.go`
- Create: `internal/document/render.go`
- Create: `internal/document/validation.go`
- Create: `internal/document/*_test.go`
- Create: `internal/cli/organization.go` (organization validation, brands, context, and document commands)
- Create: `internal/cli/organization_test.go` and `internal/cli/document_test.go`
- Create: `internal/cli/document_test.go`
- Modify: `internal/cli/root.go` and `cmd/orgdoc/main.go`
- Modify: `internal/orgconfig/model.go`, `validate.go`, and profile tests
- Modify: `schemas/document-type.schema.json` and `schemas/validate.go`
- Create: `examples/organization-docs/schemas/*-request.schema.json`
- Modify: `examples/organization-docs/document-types/*.yaml`

**Interfaces:**
- Consumes: resolved profile from Task 3 and worker runner from Task 4.
- Produces: `BuildDocumentContext(profile ResolvedProfile) (DocumentContext, error)` and `RenderDocument(ctx context.Context, request RenderRequest) (DocumentResult, error)`.

- [x] Add golden JSON tests for context generation, including brand tokens, voice guidance, document fields, assets, and capability IDs.
- [x] Implement `organization validate`, `brands list`, and `context` commands with human and JSON output.
- [x] Implement document request validation against the selected document type schema before launching a renderer.
- [x] Persist a run manifest containing profile versions, capability/worker version, input references, timestamps, output paths, and stage statuses.
- [x] Ensure reruns preserve prior successful artifacts until replacement output has passed validation.

**Acceptance:** The same operation and profile versions resolve to the same machine-readable context; different brands cannot leak styles/assets into one another; invalid document input never reaches a renderer.

### Task 6: Add the Humanizer skill with brand voice and protected-field rules

**Files:**
- Create: `skills/humanizer/SKILL.md`
- Create: `skills/humanizer/references/brand-voice.md`
- Create: `docs/product/humanizer-policy.md`
- Create: `tests/fixtures/humanizer/`
- Create: `tests/test_humanizer_examples.py`
- Modify: `examples/organization-docs/skills/` and selected brands' `voice.approved_examples`
- Modify: `internal/document/request.go`, `render.go`, and related tests for optional original-text retention
- Modify: `internal/cli/organization.go`, `root.go`, and document CLI tests for `--pre-humanized-text`
- Modify: `docs/product/agent-document-runtime.md`

**Interfaces:**
- Consumes: brand voice and protected-field declarations from Tasks 3 and 5.
- Produces: portable agent instructions for optional narrative editing; humanizer is not a runtime worker or a required step for every document.

- [x] Define applicability by document field: narrative prose may be revised; amounts, identifiers, dates, quoted source text, citations, and typed values are protected.
- [x] Define Spanish-first behavior and allow each brand profile to supply voice preferences and approved examples.
- [x] Require factual preservation and a post-edit validation pass; preserve the pre-humanized text in the run record when configured.
- [x] Explicitly exclude detector evasion and fabricated human identity from the skill's goal.
- [x] Add skill examples for a report narrative and an invoice description; assert that protected invoice fields remain byte-for-byte unchanged.
- [x] Review candidate public Humanizer skills before reuse. Do not copy an external skill wholesale; record source, license, behavior, and chosen adaptation if one is selected.

**Acceptance:** The skill improves naturalness only where requested, follows the selected brand voice, and does not alter protected data or make authorship claims.

### Task 7: Implement the first document workers (invoice PDF and narrative report)

**Files:**
- Create: `workers/python/reportlab-invoice/`
- Create: `workers/python/narrative-report/`
- Create: `examples/organization-docs/capabilities/invoice-pdf/capability.yaml`
- Create: `examples/organization-docs/capabilities/report-markdown/capability.yaml`
- Create: `tests/integration/test_invoice_worker_contract.py`
- Create: `tests/integration/test_document_brand_profiles.py`
- Modify: `docs/context/testing_context.md`

**Interfaces:**
- Consumes: worker protocol from Task 4 and document contracts from Tasks 3 and 5.
- Produces: a ReportLab PDF invoice worker and a Markdown report worker using the same brand profile contract.

- [x] Inventory the user-designated Facturas workspace. Its source generates contractual receipts and explicitly excludes sales invoices; use the visual brand treatment as reference and implement the invoice renderer from OrgDoc's typed invoice schema. Do not copy customer invoice outputs.
- [x] Generate an invoice PDF from a typed JSON fixture using ReportLab and the resolved brand profile.
- [x] Generate a narrative Markdown report with brand voice and template context.
- [x] Render test PDFs to images and verify A4 page geometry, SVG logo placement, brand colors, Helvetica fallback, totals, and multi-page overflow behavior.
- [x] Run both document types for each of two brand profiles and verify visual tokens are selected from the right profile.
- [x] Keep worker dependencies isolated from the Go CLI install and report a clear diagnostic when the worker environment is absent.

**Acceptance:** Invoice and report workers run through the same Go contract, use separate document rules, and produce visually verified artifacts for two brands.

### Task 8: Add bounded batch scheduling and performance gates

**Files:**
- Create: `internal/scheduler/queue.go`
- Create: `internal/scheduler/policy.go`
- Create: `internal/scheduler/*_test.go`
- Create: `internal/cli/batch.go`
- Create: `internal/cli/batch_test.go`
- Create: `benchmarks/README.md`
- Create: `benchmarks/fixtures/`

**Interfaces:**
- Consumes: capability resource classes and worker runner from Task 4.
- Produces: `RunBatch(ctx context.Context, jobs []Job, limits ResourceLimits) BatchResult` with per-job status and stable ordering in the returned result.

- [x] Test maximum active jobs per resource class, cancellation, partial failure, retry safety, and artifact isolation.
- [x] Implement explicit `--max-workers` and configuration defaults; CPU, GPU/model, and memory-heavy capabilities have separate limits.
- [x] Do not infer that model-backed tasks can safely run concurrently; model workers declare a resource class and GPU/model or memory-heavy jobs default to one active job.
- [x] Benchmark cold CLI, many small document jobs, a mixed resource-class no-op proxy, and coordinator/process-tree RSS against Task 1 fixtures. Actual media/model load measurements follow Task 9, when the media worker exists.
- [x] Record machine and worker versions, median/p95 startup, throughput, and peak RSS in `benchmarks/README.md`. No model/device settings apply to this task because no model was loaded.
- [x] Tune defaults from evidence; startup, no-op batch throughput, invoice throughput, and coordinator memory met the Task 1 gates in the recorded run.

**Acceptance:** Batch execution improves throughput on independent jobs without exceeding declared concurrency or corrupting partial results; performance claims are backed by repeatable benchmark records.

### Task 9: Migrate existing media processing into an optional capability pack

**Files:**
- Create: `capabilities/media/`
- Create: `workers/python/media-transcription/`
- Create: `workers/python/media-report/`
- Create: `tests/integration/test_media_worker_parity.py`
- Modify: `docs/context/workflow_context.md`
- Modify: `docs/release.md`

**Interfaces:**
- Consumes: stable capability and batch contracts from Tasks 4 and 8.
- Produces: optional media scan, transcribe, report, and PDF capabilities that preserve supported existing artifact semantics during compatibility period.

- [ ] Inventory current CLI commands, metadata versions, artifact paths, and resume behavior against the Task 1 compatibility matrix.
- [ ] Wrap existing FFmpeg, faster-whisper, LLM, and Pandoc flows as separate workers; retain GPU/CPU fallback metadata and remote-provider warnings.
- [ ] Add parity fixtures proving valid existing artifacts can be read/resumed or migrated without losing transcripts, raw responses, logs, or metadata.
- [ ] Keep `media-report` as a compatibility entry point that forwards supported commands to the new runtime or legacy Python path during one migration release.
- [ ] Defer MCP, arbitrary third-party plugin discovery, and automatic Python environment installation until CLI worker protocol and organization profile format stabilize.

**Acceptance:** Media workflows are optional, discoverable, locally executable, bounded by scheduler policy, and do not regress artifact traceability or resume behavior.

### Task 10: Package, document, and cut over

**Files:**
- Modify: `README.md`
- Modify: `CHANGELOG.md`
- Modify: `docs/release.md`
- Modify: `pyproject.toml` (remove only after compatibility release)
- Create: `go.mod` release metadata and platform build workflow
- Create: install/smoke documentation and migration guide
- Modify: `AGENTS.md`

**Interfaces:**
- Consumes: working Go CLI, profile validation, worker contracts, initial document workers, media capability parity, and benchmark results.
- Produces: installable CLI binary, optional worker bundles/setup instructions, documented organization repository template, and migration path from `media-report-cli`.

- [ ] Build Linux and macOS binaries and validate checksums/install paths from a clean environment.
- [ ] Verify core install does not install Python or ML dependencies; verify optional worker setup is explicit and isolated.
- [ ] Document organization repository initialization, multiple operations/brands, capability discovery, JSON contracts, humanizer use, and media capability setup.
- [ ] Publish a migration table from every old `media-report` command and config key to the new command/capability or its removal rationale.
- [ ] Run the full documented checks, CLI integration suite, worker contract suite, rendered PDF visual checks, and performance benchmarks before release.
- [ ] Remove old Python CLI packaging only in a major release after compatibility period and migration guide are complete.

**Acceptance:** A new user can install the core runtime, validate an organization repository, discover capabilities, produce an invoice and report under either brand, and optionally enable media workers without installing unrelated dependencies.

## Cutover Gates

1. **Product gate:** final name, first user/document set, organization profile schema, and performance targets are accepted.
2. **Runtime gate:** Go CLI startup is measurably faster than baseline; JSON command contract and worker isolation are stable.
3. **Document gate:** invoice and report generation pass data validation and visual review for two brands.
4. **Agent gate:** a supported agent can discover capabilities and use the committed skills without hidden repository knowledge; CLI remains usable without agent-specific integrations.
5. **Media gate:** current media artifacts and resume behavior have documented parity or an explicit migration path.
6. **Release gate:** Linux/macOS packaging, installation, security review, and migration guide are complete.

## Deferred Decisions

- Final product, repository, package, and command names (decided in Task 1).
- Exact public Humanizer source or whether to author a project-owned skill from scratch (review in Task 6).
- MCP server transport and packaging (deferred until capability registry stabilizes).
- Automatic worker environment provisioning (deferred until runtime security, offline behavior, and dependency lifecycle are specified).
- Whether to deprecate PyPI `media-report-cli` immediately after the compatibility release or maintain it as a thin compatibility package.
