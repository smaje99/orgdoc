# Roadmap

The migration is governed by [the task-by-task plan](../superpowers/plans/2026-10-14-agent-document-runtime-migration.md).

1. Lock product scope, names, compatibility matrix, and performance baseline.
2. Build the Go CLI shell and stable human/JSON command contract.
3. Define and validate organization, operation, brand, and document profiles.
4. Discover capabilities and invoke versioned workers safely.
5. Resolve document context and render validated artifacts with provenance.
6. Add the optional Humanizer skill with protected-field rules.
7. Deliver the invoice PDF and narrative report workers.
8. Add bounded batch scheduling and benchmark gates.
9. Move media workflows into an optional capability pack with artifact parity.
10. Package, document, and cut over only after all migration gates pass.

Keep the Python `media-report` command during the migration release. Remove it only in a later major release after parity and migration documentation are complete.
