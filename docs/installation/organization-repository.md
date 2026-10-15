# Organization document repository

An organization repository versions its organization profile, operation-to-brand mappings, document types, capabilities, local assets, and organization-specific Skills. It can be committed and reviewed like other business configuration.

Initialize a new path with the scaffold packaged into the OrgDoc binary:

```sh
orgdoc organization init --path ./organization-docs
orgdoc organization validate --organization ./organization-docs --format json
```

Initialization refuses any existing destination and prints the next validation command. It copies files only into the path the user selected. It does not overwrite files, install workers, or copy organization Skills into the agent's global directory.

## Repository contents

The scaffold includes:

- `organization.yaml` listing the versioned brand, operation, and document-type profiles.
- `brands/` with independent visual and voice profiles and local assets.
- `operations/` where each operation selects exactly one brand and may hold legal/business details.
- `document-types/` with request schemas, protected fields, and renderer capability IDs.
- `capabilities/` with worker manifests and input/output schemas.
- `schemas/` with document request schemas.
- `skills/` with organization-owned agent guidance, including the example Humanizer policy.

An organization can define several brands. Each operation's `brand_id` selects one brand; an agent should inspect that mapping and call `orgdoc context` with an explicit operation and document type instead of guessing or mixing profiles.

## Workers and Skills

The repository owns capability manifests and organization-specific Skills. Install only the matching worker projects needed for a workflow by following [worker setup](workers.md). The portable OrgDoc CLI Skill is distributed in OrgDoc's `skills/orgdoc/` directory and in core release archives. Installing that Skill does not install an organization's local Skills, and initializing this repository does not install global agent Skills.

Validate changes before using them:

```sh
orgdoc organization validate --organization ./organization-docs --format json
orgdoc capabilities list --organization ./organization-docs --format json
orgdoc doctor --organization ./organization-docs --format json
```

Treat worker code as trusted local code: declared manifest permissions describe intended behavior but do not restrict OS access. Keep credentials out of organization files and document requests. See the [security model](../product/security-model.md) for the current boundary.
