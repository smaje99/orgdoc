# Roadmap

The initial runtime migration is governed by [the migration plan](../superpowers/plans/2026-10-14-agent-document-runtime-migration.md). Release distribution and capability expansion follow [the installation and capability plan](../superpowers/plans/2026-10-15-orgdoc-installation-and-capability-expansion.md).

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

## Capability sharing and external extensions

The current extension seam is an organization-local capability manifest plus trusted worker code. The CLI discovers manifests only from the selected organization repository; the release installer installs only worker projects in OrgDoc's own catalog. There is no third-party marketplace, remote code discovery, or automatic extension update. Humanizer remains an agent Skill, and agents integrate through CLI plus Skills.

Revisit third-party worker distribution when either multiple independent organizations request reusable worker sharing, an external maintainer is ready to publish a worker with repeated user demand, or manually maintaining local manifests and worker installs becomes a recurring operational burden. Assess actual adoption and maintenance capacity before designing distribution.

Any future distribution requires publisher identity and signatures, immutable and pinned versions, CLI/protocol/schema compatibility rules, dependency provenance, reviewable install/uninstall and rollback, permission review, network policy, and OS-enforced isolation. These are design gates, not current product claims. Fiscal e-invoicing, tax-authority submission, and fiscal certification remain out of scope. See the [third-party capability architecture note](../architecture/third-party-capabilities.md).
