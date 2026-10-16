# OrgDoc Runtime

OrgDoc Runtime gives organization-aware agents one local CLI for discovering document capabilities and producing consistent, auditable artifacts. Each organization repository versions its operations, brand profiles, document rules, agent skills, and renderer capabilities. The runtime resolves that context explicitly, validates input, runs the chosen capability, and records which versions produced each artifact.

The supported document set includes:

- **Branded spreadsheet**, with typed cells, formulas, tables, and charts rendered to an editable XLSX workbook.
- **Branded presentation**, with structured text, tables, images, and native charts rendered to an editable PowerPoint deck.
- **Fillable PDF forms**, with typed AcroForm fields, template filling, and optional explicitly requested flattening.
- **Invoice PDF**, with typed financial fields rendered by a ReportLab worker.
- **Narrative report**, with brand voice applied to prose and rendered as Markdown.

Spreadsheet requests select a formula policy: deliver formulas for recalculation by a spreadsheet application, or explicitly request LibreOffice recalculation. The worker never claims calculated results in the first mode; an organization can restrict the allowed policy in its document request schema.

Presentation requests use explicit title, section, content, or two-column layouts and support text, raster images, tables, and native bar, line, and pie charts. Image paths are restricted to assets inside the organization repository. LibreOffice can optionally render a PDF preview for visual review. Font families are resolved by the machine opening or rendering the deck, so brands should select widely available fonts or install their chosen font on the render host.

PDF forms use two separate capabilities: generate an interactive form from typed text, checkbox, radio, and choice fields; or fill an existing AcroForm PDF whose path is confined to the organization repository. Filling preserves the source and remains interactive by default. A static flattened output is created only when the request explicitly sets `flatten: true`. XFA forms, signing, PDF/UA accessibility certification, legal validity, and fiscal submission are outside the current scope.

An organization can define multiple brands and operations. Each operation names its brand, so the runtime never guesses when an operation or brand is ambiguous. Visual tokens, document business rules, and writing guidance remain separate versioned inputs. Humanizer is an optional agent skill for requested narrative edits; it cannot modify amounts, identifiers, dates, quotations, citations, or other typed fields. See the [Humanizer policy](humanizer-policy.md) for its field boundary and voice rules.

Local media transcription and reporting remain available as optional capabilities. Their Python and native workers are installed independently from the Go runtime. Core CLI startup, help, organization validation, and capability discovery do not start Python, load a model, or access a network service.

## Names and compatibility

- Product: **OrgDoc Runtime**
- Go module: `github.com/smaje99/orgdoc`
- Command: `orgdoc`
- Core distribution: versioned platform binaries and source builds; no replacement PyPI package is published during migration.
- Compatibility: `media-report` remains available as a shim for one migration release. Removal requires a later major release, command parity, a migration guide, and release notes.

The current Python project remains the compatibility implementation while the new command reaches the migration gates in the plan. See the [migration guide](../migration/from-media-report-cli.md) for command and configuration mappings and [performance-baseline.md](../migration/performance-baseline.md) for measured startup and batch behavior.

## Product promise

Given a selected organization operation, document type, and valid input, OrgDoc Runtime uses the declared brand and document rules to produce a validated artifact with enough provenance to reproduce or audit the run. Agents discover capabilities and receive stable JSON; people can use readable terminal output. Workers may use Go, Python, C, or another declared runtime behind one versioned JSON process contract.

The runtime is a document consistency and execution layer, not a general writing agent. Skills explain how an agent should prepare and review work; schemas, validation, and renderers enforce the document contract.

Capabilities are executable code declared in the selected organization's local repository and run with the current user's operating-system permissions. Today, extending an organization means adding a trusted local capability manifest and worker; there is no marketplace, remote code discovery, or worker auto-update. OrgDoc's release installer installs only worker projects published in its own catalog. Humanizer remains an agent Skill. Manifest permission declarations describe intended access; they are not an operating-system sandbox. The runner constrains time, captured output, concurrency, inherited environment variables, and accepted artifact paths, so organizations should only enable capability code they trust. See the [security model](security-model.md) for exact controls and the [third-party capability gate](../architecture/third-party-capabilities.md) for the future adoption criteria.

## Profile format v1

`organization.yaml` and every brand, operation, and document-type profile declare `schema_version: 1`. Brand and document-type profiles also declare a version string for artifact provenance. The organization manifest lists profile files using paths relative to the organization repository; an operation selects its brand with `brand_id`, and each document type declares a request JSON Schema and renderer capability ID. OrgDoc validates that request schema and the document input before it creates a run directory or starts a worker. The version 1 profile schemas are packaged with the CLI, so profile validation works offline and against the exact schema release used by the runtime.

## Agent CLI

```text
orgdoc organization validate --organization PATH --format json
orgdoc capabilities list --organization PATH --format json
orgdoc brands list --organization PATH --format json
orgdoc context --organization PATH --operation ID --document-type ID --format json
orgdoc document render --organization PATH --operation ID --document-type ID --input INPUT.json --output-root ARTIFACTS --format json [--pre-humanized-text ORIGINAL.md]
orgdoc batch run JOBS.json --max-workers 4 --format json
```

Each render uses a new `job_id` directory and writes `run-manifest.json` with selected profile and capability versions, input references, timestamps, output paths, and stage statuses. Reusing an existing job ID fails without changing its artifacts. Send `--input -` to read the JSON request from standard input. Pass `--pre-humanized-text ORIGINAL.md` when the original prose should be retained verbatim in the run record.

Batch input is a versioned JSON file. Organization and output paths are resolved relative to that file when they are not absolute. Each job uses the same operation, document type, and typed input fields as `document render`:

```json
{
  "schema_version": 1,
  "organization": "./organization-docs",
  "output_root": "./artifacts",
  "jobs": [
    {
      "job_id": "invoice-2026-001",
      "operation": "consulting",
      "document_type": "invoice",
      "input": {
        "invoice_number": "HT-2026-001",
        "issue_date": "2026-10-14",
        "due_date": "2026-10-28",
        "customer": {
          "name": "Cliente de ejemplo",
          "tax_id": "900000000-1",
          "address": "Bogotá, Colombia"
        },
        "lines": [
          {
            "description": "Servicio de consultoría",
            "quantity": 1,
            "unit_price": 100000,
            "tax_rate": 0.19,
            "total": 100000
          }
        ],
        "subtotal": 100000,
        "tax_total": 19000,
        "grand_total": 119000,
        "currency": "COP"
      }
    }
  ]
}
```

Results retain input order. A failed job does not prevent independent jobs from completing; reusing a job ID leaves its previous artifacts unchanged. `--max-workers` sets the total and CPU concurrency. GPU/model and memory-heavy capabilities remain limited to one active job each by default.

Media capabilities can be chained through their JSON results: `media-scan` finds supported files, `media-transcription` writes or resumes the schema-v2 sibling artifact tree, `media-report` generates the narrative from reusable transcripts, and `media-pdf` renders a completed Markdown report. Run dependent jobs in successive batches so the agent can pass the preceding result's artifact path into the next request. Scanning and transcription accept either a file or directory; directory recursion must be requested explicitly.
