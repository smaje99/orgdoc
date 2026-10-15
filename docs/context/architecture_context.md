# Architecture Context

## Target Style

Keep a small Go control plane with clear boundaries between CLI, application orchestration, organization/document domain rules, and infrastructure adapters. Capability workers are separate processes behind a versioned JSON contract.

## Components

- `cmd/orgdoc`: process entry point
- `internal/cli`: argument parsing, output formats, exit codes
- `internal/application`: use-case orchestration
- `internal/orgconfig`, `internal/document`, `internal/capability`: profile rules, resolution, and capability discovery
- `internal/worker`, `internal/scheduler`: process boundary and resource-aware execution
- `workers/`: optional language-specific renderers and media operations
- Organization repository: versioned brands, operations, document types, assets, skills, and capability manifests

## Invariants

- A selected operation resolves exactly one brand; ambiguous or missing references fail before worker startup.
- Brand appearance, document business rules, and agent writing instructions are separate versioned inputs.
- A worker cannot write outside its job directory or replace prior successful artifacts until its result validates.
- The control plane does not install dependencies or interpret arbitrary shell strings.
- Worker failures return structured diagnostics and preserve prior artifacts.
- Provenance records selected profile versions, worker identity/version, inputs, outputs, and stage status.

## Migration Boundary

The existing `media_report` hexagonal Python application remains the compatibility path until its workflows and artifact semantics are represented by optional worker capabilities. The Go CLI must not import or start that application for core commands.
