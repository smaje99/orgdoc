# OrgDoc Runtime

OrgDoc Runtime gives organization-aware agents one local CLI for discovering document capabilities and producing consistent, auditable artifacts. Each organization repository versions its operations, brand profiles, document rules, agent skills, and renderer capabilities. The runtime resolves that context explicitly, validates input, runs the chosen capability, and records which versions produced each artifact.

The first supported document set is:

- **Invoice PDF**, with typed financial fields rendered by a ReportLab worker.
- **Narrative report**, with brand voice applied to prose and rendered as Markdown.

An organization can define multiple brands and operations. Each operation names its brand, so the runtime never guesses when an operation or brand is ambiguous. Visual tokens, document business rules, and writing guidance remain separate versioned inputs. Humanizer is an optional agent skill for requested narrative edits; it cannot modify amounts, identifiers, dates, quotations, citations, or other typed fields.

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

## Profile format v1

`organization.yaml` and every brand, operation, and document-type profile declare `schema_version: 1`. Brand and document-type profiles also declare a version string for artifact provenance. The organization manifest lists profile files using paths relative to the organization repository; an operation selects its brand with `brand_id`, and each document type declares a renderer capability ID. The version 1 schemas are packaged with the CLI, so validation works offline and against the exact schema release used by the runtime.
