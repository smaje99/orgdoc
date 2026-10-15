# Product Context

## Product

OrgDoc Runtime is a local, agent-facing document runtime for applying an organization's versioned brand profiles and document rules consistently across generated artifacts. Media processing is an optional capability pack that can provide source material for those documents.

## Target Users

- Organizations with more than one operation, service line, or brand
- Agents and professionals that generate recurring invoices, reports, and other controlled documents
- Teams that need reproducible outputs and traceable inputs without depending on a hosted document service

## First Document Set

- Invoice PDF rendered from typed fields through an isolated ReportLab worker
- Narrative report rendered as Markdown using the selected brand voice and a document profile

## Product Boundaries

- CLI first; agents discover and chain declared capabilities through stable JSON interfaces
- Organization configuration and skills live in a versioned repository
- Operation selection resolves a brand explicitly; ambiguous selection is an error
- Python is optional and worker-scoped; the Go core does not require Python or ML dependencies
- Local media transcription and report preparation are optional capabilities
- Linux and macOS are official targets; Windows remains experimental

See [agent-document-runtime.md](../product/agent-document-runtime.md) and the [migration plan](../superpowers/plans/2026-10-14-agent-document-runtime-migration.md).
