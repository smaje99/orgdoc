# OrgDoc Installation And Capability Expansion Implementation Plan

> **For agentic workers:** Use the `executing-plans` skill to implement this plan task by task on `main`. Steps use checkbox syntax for tracking. Commit each completed task with a staged-area commit message.

**Goal:** Make OrgDoc straightforward to install from GitHub Releases, install its CLI Skill and only the required workers, explain and tighten its real security boundaries, and add reusable enterprise document capabilities for Excel, presentations, and PDF forms.

**Architecture:** Keep `orgdoc` as a standalone Go binary distributed in checksummed release archives. Publish separately packaged worker source bundles so Python dependencies remain optional and are installed into isolated environments only when selected. Keep agent integration as portable Skills plus the stable CLI/JSON interface; keep document formats as independently declared capabilities. Treat worker code as trusted local code until a future adoption-driven project introduces a stronger sandbox or third-party extension ecosystem.

**Tech Stack:** Go CLI; GitHub Actions Releases; POSIX shell packagers/installers; Python worker projects managed with `uv`; YAML capability and document profiles; JSON Schema; ReportLab, openpyxl or XlsxWriter, python-pptx, and PDF form libraries where justified by each capability.

**Spec:** Product and runtime decisions in `docs/product/agent-document-runtime.md`, installation details in `INSTALL.md` and `docs/release.md`, and this plan's scope decisions below.

## Global Constraints

- Agent integration is CLI plus portable Agent Skills; do not add MCP.
- The core binary must run without Python, workers, models, or network access for help, version, organization validation, and capability discovery.
- Publish portable `tar.gz` release packages for Linux/macOS `amd64` and `arm64`; native `.deb`, `.rpm`, and `.pkg` installers are outside this first release-packaging scope.
- Publish worker bundles as separate GitHub Release assets. Do not include virtual environments, API credentials, or ML model weights in bundles.
- Install only selected workers. Keep worker environments separate from the core CLI and from one another.
- Keep the Humanizer as an agent Skill. It may edit narrative prose only and must preserve protected structured fields.
- Treat organization capability code as trusted code running with the current user's OS permissions. Manifest permissions are declarations, not enforced sandbox policy.
- Do not implement a marketplace, remote plugin registry, or third-party capability discovery in this personal-use phase. Record adoption triggers and the future trust requirements instead.
- Document generation may include invoices/receipts, reports, Excel workbooks, presentations, and fillable or completed PDF forms. Electronic invoicing, tax submission, and fiscal certification are outside OrgDoc's responsibility.
- Keep Linux and macOS as official targets; Windows remains experimental.
- Work task-by-task on `main`; finish each task with focused verification and a commit based only on staged changes.

## Review Focus

- A release asset must match its OS/architecture and have a checksum that verifies before installation.
- The installer must use `github.com/smaje99/orgdoc` as its default source and install to a user-owned directory without requiring administrator rights.
- A worker bundle must install outside the repository checkout; media workers must still resolve the legacy Python package and packaged resources after extraction.
- A missing worker or system dependency must produce a precise doctor diagnostic without loading a model or making a network call.
- Agent Skills and organization profiles must not be conflated: the OrgDoc usage Skill teaches CLI invocation, while organization Skills express local brand/workflow guidance.
- A worker's declared permissions, timeout, and output path checks must not be described as an OS sandbox or guarantee that it cannot read/write elsewhere.
- Spreadsheet formulas, slide rendering, and PDF form appearances must be verified with the appropriate renderer or document reader, not inferred from file creation success.

---

## Scope And Release Contract

The first install experience uses portable release archives. “Packagers” in this plan means build scripts that produce versioned, uploadable GitHub Release assets; it does not mean native package-manager formats in this phase. The core release contains the `orgdoc` binary and install/license documentation. Each optional worker has a separate source bundle containing its code, dependency lock, capability metadata, and setup instructions. Python environments and large/system dependencies are provisioned after download on the user's machine.

The installation flow is:

1. Install or update `orgdoc` from a verified core release archive.
2. Install/copy the portable OrgDoc Skill into the chosen agent's Skills directory, or use it from the project.
3. Initialize or select an organization repository containing its brands, operations, document profiles, capabilities, and organization Skills.
4. Install only the worker bundles required by that organization and workflow.
5. Run `orgdoc doctor` to check declared capabilities and their local prerequisites.

No automatic third-party plugin support is part of the implementation tasks. A future decision to distribute extensions beyond the owner's organization must first define publisher identity, signatures/checksums, version compatibility, update/rollback behavior, and an enforceable execution boundary.

## File Map

- `scripts/build-release.sh`: cross-compile and package the four core targets.
- `scripts/install-orgdoc.sh`: download, checksum-verify, and install the core release.
- `scripts/package-workers.py`: create source bundles for the selected worker projects from `workers/catalog.json`.
- `scripts/install-worker.sh`: download one versioned worker bundle, verify its checksum, install it into a user-owned data directory, and expose its entrypoints.
- `.github/workflows/release.yml`: verify, package, smoke-test, and upload all release assets.
- `internal/cli/doctor.go`: inspect the selected organization's declared worker executables and system prerequisites.
- `internal/cli/organization.go`: add organization initialization from the packaged example scaffold.
- `skills/orgdoc/SKILL.md`: portable instructions for agents to discover and use the CLI safely.
- `docs/installation/`: core, Skills, organization repository, worker, update, and troubleshooting guides.
- `docs/product/security-model.md`: current trust model, enforced controls, limitations, and future security gates.
- `workers/python/{spreadsheet,presentation,pdf-form}/`: isolated document workers and tests.
- `examples/organization-docs/`: sample profiles, schemas, and capabilities for the new document types.

---

## Implementation Tasks

### Task 1: Complete the core release packager and installer

**Files:**
- Modify: `scripts/build-release.sh`
- Modify: `scripts/install-orgdoc.sh`
- Modify: `INSTALL.md`
- Modify: `README.md`
- Test: `scripts/tests/test-release-packaging.sh` (or the repository's existing shell-test convention)

**Interfaces:**
- Produces core assets named `orgdoc-<version>-<os>-<arch>.tar.gz` for Linux/macOS `amd64` and `arm64`, plus one `SHA256SUMS` file.
- The installer accepts a version and optional `INSTALL_DIR`; it defaults to the `smaje99/orgdoc` GitHub Releases URL and installs `orgdoc` into `$HOME/.local/bin`.

- [x] Add packager checks for all four expected archives, archive contents (`orgdoc`, `INSTALL.md`, `LICENSE`), and checksums.
- [x] Fix the installer default repository from `smaje99/media-report-cli` to `smaje99/orgdoc`; retain an explicit environment override for mirrors/testing.
- [x] Verify installer behavior for valid checksum, missing asset/checksum, invalid checksum, unsupported platform, and custom install directory using a local file URL fixture.
- [x] Keep the core archive worker-free; smoke-test help, version, organization validation, and capability discovery with an empty `PATH`.
- [x] Document that the initial downloadable formats are portable archives; do not advertise native package-manager installers.
- [x] Run the packaging and clean-path smoke checks, then commit as `🔧 chore(release): package and install core binaries`.

### Task 2: Package optional worker bundles for GitHub Releases

**Files:**
- Create: `workers/catalog.json`
- Create: `scripts/package-workers.py`
- Create: `scripts/install-worker.sh`
- Modify: `workers/python/*/pyproject.toml` and lockfiles only where required for standalone bundles
- Modify: `workers/python/*/README.md`
- Modify: `.github/workflows/release.yml`
- Test: worker packaging/install smoke fixtures

**Interfaces:**
- Bundle names: `orgdoc-worker-<worker-project-id>-<orgdoc-release-version>-source.tar.gz`; `<orgdoc-release-version>` matches the root `VERSION`/GitHub tag, and each project bundle can expose one or more capability IDs.
- Each bundle contains only the worker project, its lockfile, required source/resources, `LICENSE`/attribution where applicable, capability manifest/schema, and setup instructions. It excludes `.venv`, secrets, caches, and model data.
- `scripts/install-worker.sh <worker-project-id> <orgdoc-release-version>` downloads that release asset, verifies it against `SHA256SUMS`, provisions one isolated environment under `${XDG_DATA_HOME:-$HOME/.local/share}/orgdoc/workers/<worker-project-id>/<orgdoc-release-version>`, and exposes declared entrypoints in the user-local binary directory.
- One bundle is produced per Python worker project, not per capability: current media projects each provide two entrypoints that share one environment. This avoids duplicate dependency installation; capabilities in one project therefore move together at release time.

- [x] Add `workers/catalog.json` mapping each worker project ID to its source directory, capability IDs, package-relative layout, entrypoints, minimum Python/uv versions, and external system prerequisites.
- [x] Package each current Python worker from a temporary extracted bundle and prove installation does not depend on the source checkout.
- [x] Include the legacy `media_report` package and its packaged resources inside media worker bundles, or replace the local path dependency with an independently distributable pinned dependency; choose one approach and validate the archive in a clean temporary directory.
- [x] Keep FFmpeg, Pandoc/TeX, GPU runtimes, and model weights outside bundles; report these as explicit prerequisites.
- [x] Publish one source bundle per worker project beside core archives and add all of them to the same checksum manifest.
- [x] Test that installing one worker does not install other worker environments and does not modify the core binary.
- [x] Commit as `📦 build(workers): publish optional worker bundles`.

### Task 3: Add worker diagnostics and guided setup

**Files:**
- Create: `internal/cli/doctor.go`
- Modify: `internal/cli/root.go`, `internal/cli/root_test.go`
- Modify: `internal/capability/manifest.go` and schemas for typed prerequisite declarations
- Create/modify: `internal/cli/doctor_test.go`
- Modify: `docs/installation/workers.md`

**Interfaces:**
- `orgdoc doctor [--organization PATH] [--format human|json]` returns stable checks with `id`, `status` (`ok`, `missing`, `warning`, `unsupported`), and actionable `message` fields.
- Checks are metadata/executable-presence checks only; doctor must not run arbitrary worker jobs, download models, or call a network service.

- [x] Add tests for no organization, no declared workers, missing entrypoint, missing system executable, unsupported platform, and successful worker presence.
- [x] Add optional manifest prerequisites for executable names and version notes; do not treat declared permissions as enforced.
- [x] Implement JSON and human diagnostics with stable exit codes and deterministic ordering.
- [x] Document selected-worker setup and exact prerequisite commands for invoice, report, media, spreadsheet, presentation, and PDF form capabilities as they are added.
- [x] Run focused CLI/doctor tests and commit as `✨ feat(cli): diagnose worker installations`.

### Task 4: Deliver the CLI Skill and organization setup workflow

**Files:**
- Create: `skills/orgdoc/SKILL.md`
- Create: `docs/installation/agents.md`
- Create: `docs/installation/organization-repository.md`
- Modify: `internal/cli/organization.go`, `internal/cli/root.go`, and corresponding tests if adding initialization
- Modify: `scripts/build-release.sh`, `scripts/tests/test-release-packaging.sh`
- Modify: `README.md`, `INSTALL.md`
- Modify: `examples/organization-docs/` only for documented setup corrections

**Interfaces:**
- The portable `orgdoc` Skill explains capability discovery, explicit operation/brand selection, JSON CLI invocation, artifact review, and untrusted-input handling.
- `orgdoc organization init --path PATH` copies the versioned example scaffold without overwriting existing files and prints the next validation command.

- [x] Add Skill examples for discovering capabilities, resolving a brand context, rendering an invoice/report, and running doctor; identify Humanizer as an optional separate Skill.
- [x] Document project-local and user-level Skill installation without assuming every agent uses the same directory.
- [x] Add tests for organization scaffold success and existing destination refusal; do not implement `--force` in this phase.
- [x] Ensure organization Skills remain owned by the organization repository and are never silently installed into a user's global Skills directory.
- [x] Run the relevant CLI tests and commit as `📚 docs(agents): package the OrgDoc CLI skill`.

### Task 5: Define and implement the worker trust and security boundary

**Files:**
- Create: `docs/product/security-model.md`
- Modify: `internal/worker/runner.go`, worker protocol/manifests, and tests only for concrete controls chosen below
- Modify: `docs/product/agent-document-runtime.md`, `INSTALL.md`, and `docs/installation/workers.md`
- Modify: `scripts/build-release.sh`, `scripts/tests/test-release-packaging.sh`

**Interfaces:**
- The security guide separates enforced runtime controls from policy declarations and future OS-level isolation.
- Worker execution stays noninteractive and local-first. Any new opt-in or environment policy must have a stable CLI/config contract and preserve explicit remote-provider warnings.

- [ ] Document current controls: direct process invocation without shell interpolation, input/schema validation, timeouts, cancellation, concurrency limits, bounded stdout/stderr, fresh artifact directories, and validation of reported output paths.
- [ ] Document missing guarantees: workers inherit user OS permissions and environment by default; manifest permissions do not restrict filesystem/network access; output checks do not prevent arbitrary side effects; concurrency classes do not enforce memory/CPU/disk quotas; checksum is integrity against corruption, not publisher identity.
- [ ] Add tests for secret redaction across worker stderr and structured diagnostics, including common authorization headers and JSON key/value forms; do not claim complete secret detection.
- [ ] Decide and implement a minimal child-process environment policy that preserves explicitly required provider credentials while excluding unrelated inherited variables; document the allowlist.
- [ ] Verify timeout/cancellation terminates the worker process tree on supported Unix platforms and document platform differences.
- [ ] Add capability trust guidance to CLI output/docs: install/run only organization code the user trusts; do not imply a sandbox.
- [ ] Run focused runner/security tests and commit as `🔒 feat(worker): clarify and tighten execution boundaries`.

### Task 6: Add the branded spreadsheet capability

**Files:**
- Create: `workers/python/spreadsheet/`
- Create: `examples/organization-docs/capabilities/spreadsheet-xlsx/`
- Create: spreadsheet input/output schemas and a document-type profile
- Modify: `examples/organization-docs/organization.yaml` and examples/tests

**Interfaces:**
- Capability ID: `spreadsheet-xlsx`.
- Initial output: `.xlsx` generated from typed data, a declared workbook layout, and the selected brand profile.
- Formula policy is explicit per document profile: formulas are either delivered for recalculation by a spreadsheet application or recalculated with an installed compatible engine; the worker must not imply formula results were computed when they were not.

- [ ] Add schemas for workbook sheets, typed cells/tables, formulas, number formats, and charts supported by the initial worker.
- [ ] Implement a worker using one selected library (openpyxl or XlsxWriter) with deterministic entrypoint and JSON contract.
- [ ] Add golden workbooks for two brands; verify sheets, formulas, styles, protected values, and chart/table metadata.
- [ ] Add optional LibreOffice recalculation/render validation only when installed; return a clear capability diagnostic when requested and unavailable.
- [ ] Package/install the worker through Task 2 and include it in doctor checks.
- [ ] Run worker tests and inspect generated workbook content/render; commit as `✨ feat(documents): render branded spreadsheets`.

### Task 7: Add the branded presentation capability

**Files:**
- Create: `workers/python/presentation/`
- Create: `examples/organization-docs/capabilities/presentation-pptx/`
- Create: presentation schemas and document-type profile
- Modify: example organization and tests

**Interfaces:**
- Capability ID: `presentation-pptx`.
- Initial output: editable `.pptx` from structured slides and a brand profile, with explicit supported layout types and media inputs.

- [ ] Define typed slide blocks for title, section, text, image, table, and chart; reject unsupported block types before starting the renderer.
- [ ] Implement presentation generation using python-pptx or another justified library, with theme colors, logo, typography fallbacks, and slide master/layout rules.
- [ ] Add tests for slide ordering, protected text/data, image paths confined to supplied assets, and profile isolation across both brands.
- [ ] Add optional LibreOffice PDF/image rendering for visual QA and report when rendering is unavailable.
- [ ] Package/install the worker through Task 2 and include it in doctor checks.
- [ ] Run worker tests and inspect rendered slides; commit as `✨ feat(documents): render branded presentations`.

### Task 8: Add PDF form generation and completion capabilities

**Files:**
- Create: `workers/python/pdf-form/`
- Create: `examples/organization-docs/capabilities/pdf-form/`
- Create: PDF form schemas/profiles and fixtures
- Modify: example organization and tests

**Interfaces:**
- Separate capability IDs for generating fillable PDF forms and filling a supplied AcroForm template; do not silently flatten a form or change a source template.
- Return the generated PDF and a field/result summary; preserve source template and previous successful artifacts.

- [ ] Add fixture forms with text, checkbox, radio, and choice fields and document supported field types.
- [ ] Implement generation/fill using ReportLab and pypdf (or an equivalent chosen stack); validate field names and value types before rendering.
- [ ] Test field values, appearance streams, metadata, source preservation, unknown fields, malformed PDFs, and output path isolation.
- [ ] Document that signing, PDF/UA accessibility certification, legal validity, and tax submission are outside the capability unless later specified.
- [ ] Package/install through Task 2 and include system prerequisites in doctor checks.
- [ ] Run tests and inspect PDFs in an independent reader/render; commit as `✨ feat(documents): support fillable PDF forms`.

### Task 9: Integrate all release packagers and publish-ready checks

**Files:**
- Modify: `.github/workflows/release.yml`
- Modify: `scripts/build-release.sh`, `scripts/package-workers.py`, release smoke scripts
- Modify: `docs/release.md`, `README.md`, `CHANGELOG.md`

**Interfaces:**
- A version tag matching `VERSION` produces core archives, worker source bundles, one combined checksum list, and release notes suitable for GitHub Releases.
- CI uploads those assets only after Go, Python, package, worker-install, and clean-environment smoke checks pass.

- [ ] Test tag/version mismatch failure and asset naming for all official OS/architecture targets.
- [ ] Build and install a release candidate in a clean temporary environment with no repository checkout; test the core CLI and each selected worker bundle independently.
- [ ] Verify the checksum manifest covers every uploaded archive and no unbundled model/secret/cache files are present.
- [ ] Document manual release creation/tagging, asset review, checksum verification, install, update, and rollback.
- [ ] Run the full release candidate checks and commit as `🚀 chore(release): publish core and worker assets`.

### Task 10: Record adoption-gated extension and security roadmap

**Files:**
- Modify: `docs/product/agent-document-runtime.md`
- Modify: `docs/context/roadmap.md`
- Create: `docs/architecture/third-party-capabilities.md`

- [ ] State that current extension means adding a trusted local capability manifest and worker; no marketplace or remote code discovery exists.
- [ ] Define adoption triggers for revisiting third-party distribution, such as external organizations requesting worker sharing or independently maintained workers.
- [ ] Record future requirements: publisher identity, signatures, immutable versions, compatibility policy, dependency provenance, uninstall/rollback, permission review, network policy, and OS-enforced isolation.
- [ ] Keep electronic invoicing and fiscal-service integrations explicitly out of scope.
- [ ] Review links and product claims against actual CLI behavior; commit as `📝 docs(product): define adoption-gated extensions`.

## Completion Criteria

- A user can download a matching verified core archive from GitHub Releases and install it into a user-owned directory.
- Release automation produces and attaches the four core archives and independently installable worker bundles with checksums.
- The user can install the portable OrgDoc Skill, select an organization repository, install only required workers, and use `orgdoc doctor` to see missing prerequisites.
- The current security behavior and its limits are accurate in both CLI guidance and documentation.
- Spreadsheet, presentation, and PDF form workers each have versioned schemas, examples, tests, packaging, and independently reviewed output.
- Humanizer remains a Skill; no MCP or third-party marketplace is introduced; electronic invoicing remains out of scope.
- Release artifacts can be installed and smoke-tested without access to the repository checkout.

## Deferred

- Native Linux `.deb`/`.rpm` and macOS `.pkg`/Homebrew distribution. Revisit if portable archives prove inconvenient for actual users.
- Automatic download of Python, FFmpeg, TeX, GPU runtimes, or ML models.
- Third-party marketplace, remote plugin discovery, or extension auto-update.
- Strong OS sandboxing and enforced filesystem/network permissions. Revisit before running untrusted worker code or sharing the runtime with multiple organizations/users.
- Electronic invoicing, tax authority submission, and fiscal certification.
