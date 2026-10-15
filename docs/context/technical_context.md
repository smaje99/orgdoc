# Technical Context

## Target Runtime

- Go control plane and `orgdoc` CLI, module `github.com/smaje/orgdoc`
- Python and native capability workers installed independently when their libraries are needed
- Versioned YAML organization profiles validated before execution
- Versioned JSON over stdin/stdout for worker requests and results; diagnostics and progress use stderr
- Skills provide agent instructions; schemas, profile resolution, and renderers enforce rules

## Runtime Requirements

- Help, version, organization validation, and capability discovery run without Python, models, or network access.
- Invoke workers directly without shell interpolation. Apply timeouts, cancellation, bounded concurrency, and per-job artifact roots.
- Resolve organization assets relative to the organization repository, not the executable or source checkout.
- Keep structured data and protected fields unchanged through narrative editing and rendering.
- Persist profile, document, capability, worker, input, and output provenance.
- Keep remote LLM use explicit and visible.

## Compatibility Runtime

The Python `media-report` CLI and package remain available during migration. It is the reference for current media workflows and artifact semantics until the optional media workers pass parity checks. Retain the compatibility command for one migration release; remove it only in a later major release after parity and user-facing migration notes.

## Platform and Performance

Linux and macOS are official targets. Windows is experimental until separately validated. Startup and memory goals are measured against [the recorded Python baseline](../migration/performance-baseline.md), with model loading and device inference reported separately from CLI and worker orchestration.
