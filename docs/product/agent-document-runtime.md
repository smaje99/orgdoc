# OrgDoc Runtime

OrgDoc Runtime gives organization-aware agents one local CLI for discovering document capabilities and producing consistent, auditable artifacts. Each organization repository versions its operations, brand profiles, document rules, agent skills, and renderer capabilities. The runtime resolves that context explicitly, validates input, runs the chosen capability, and records which versions produced each artifact.

The first supported document set is:

- **Invoice PDF**, with typed financial fields rendered by a ReportLab worker.
- **Narrative report**, with brand voice applied to prose and rendered as Markdown.

An organization can define multiple brands and operations. Each operation names its brand, so the runtime never guesses when an operation or brand is ambiguous. Visual tokens, document business rules, and writing guidance remain separate versioned inputs. Humanizer is an optional agent skill for requested narrative edits; it cannot modify amounts, identifiers, dates, quotations, citations, or other typed fields. See the [Humanizer policy](humanizer-policy.md) for its field boundary and voice rules.

Local media transcription and reporting remain available as optional capabilities. Their Python and native workers are installed independently from the Go runtime. Core CLI startup, help, organization validation, and capability discovery do not start Python, load a model, or access a network service.

## Names and compatibility

- Product: **OrgDoc Runtime**
- Go module: `github.com/smaje/orgdoc`
- Command: `orgdoc`
- Core distribution: versioned platform binaries and source builds; no replacement PyPI package is published during migration.
- Compatibility: `media-report` remains available as a shim for one migration release. Removal requires a later major release, command parity, a migration guide, and release notes.

The current Python project remains the compatibility implementation while the new command reaches the migration gates in the plan. The command-by-command mapping and baseline are in [performance-baseline.md](../migration/performance-baseline.md).

## Product promise

Given a selected organization operation, document type, and valid input, OrgDoc Runtime uses the declared brand and document rules to produce a validated artifact with enough provenance to reproduce or audit the run. Agents discover capabilities and receive stable JSON; people can use readable terminal output. Workers may use Go, Python, C, or another declared runtime behind one versioned JSON process contract.

The runtime is a document consistency and execution layer, not a general writing agent. Skills explain how an agent should prepare and review work; schemas, validation, and renderers enforce the document contract.

Capabilities are executable code owned by the organization repository and run with the current user's operating-system permissions. Manifest permission declarations describe intended access; they are not an operating-system sandbox. The runner constrains time, captured output, concurrency, and accepted artifact paths, so organizations should only enable capability code they trust.

## Profile format v1

`organization.yaml` and every brand, operation, and document-type profile declare `schema_version: 1`. Brand and document-type profiles also declare a version string for artifact provenance. The organization manifest lists profile files using paths relative to the organization repository; an operation selects its brand with `brand_id`, and each document type declares a request JSON Schema and renderer capability ID. OrgDoc validates that request schema and the document input before it creates a run directory or starts a worker. The version 1 profile schemas are packaged with the CLI, so profile validation works offline and against the exact schema release used by the runtime.

## Agent CLI

```text
orgdoc organization validate --organization PATH --format json
orgdoc capabilities list --organization PATH --format json
orgdoc brands list --organization PATH --format json
orgdoc context --organization PATH --operation ID --document-type ID --format json
orgdoc document render --organization PATH --operation ID --document-type ID --input INPUT.json --output-root ARTIFACTS --format json [--pre-humanized-text ORIGINAL.md]
```

Each render uses a new `job_id` directory and writes `run-manifest.json` with selected profile and capability versions, input references, timestamps, output paths, and stage statuses. Reusing an existing job ID fails without changing its artifacts. Send `--input -` to read the JSON request from standard input. Pass `--pre-humanized-text ORIGINAL.md` when the original prose should be retained verbatim in the run record.
