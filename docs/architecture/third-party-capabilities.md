# Third-party capabilities: adoption gate and security requirements

## Decision

OrgDoc does not currently distribute or discover third-party workers. Do not build a marketplace, remote registry, plugin resolver, or automatic worker updater until external adoption creates a concrete need and the security requirements in this document can be supported.

The current extension seam is deliberately local: an organization adds a capability manifest and the worker executable it trusts to its own versioned repository. An agent discovers and chains those capabilities through the OrgDoc CLI and Skills. Humanizer remains an agent Skill; it is not a renderer plugin.

## What works today

- `orgdoc capabilities list` reads and validates `capabilities/<id>/capability.yaml` under the selected organization repository. The capability's input and output schemas are read from its local directory.
- A capability manifest declares a runtime, direct entrypoint arguments, platform support, intended permissions, timeout, output limit, and resource class. The worker runner starts the declared executable directly, without shell interpolation.
- Organization owners review and version the manifest, schemas, worker source, and related Skills in their repository. They install and maintain any non-catalogued worker themselves and must trust its code.
- `install-worker.sh` installs only project IDs in the first-party worker catalog and downloads them from the OrgDoc release configured for that installation. It is not a generic third-party package manager.
- There is no remote capability search, publisher account, marketplace, automatic update, or signature verification. `SHA256SUMS` detects asset corruption; it does not authenticate a publisher.

These behaviors are grounded in the current [capability registry](../../internal/capability/registry.go), [worker catalog](../../workers/catalog.json), and [worker installer](../../scripts/install-worker.sh). Product and installation details are in the [runtime guide](../product/agent-document-runtime.md), [organization repository guide](../installation/organization-repository.md), and [worker setup guide](../installation/workers.md).

## When to revisit distribution

Reopen the decision when at least one concrete adoption signal exists:

1. Multiple independent organizations ask to reuse and update the same worker without copying source into each repository.
2. An external maintainer is prepared to publish a worker and there is repeated user demand for it.
3. Local installation, version coordination, or review of shared workers has become a recurring burden across multiple users or organizations and is blocking adoption.

Before approving implementation, confirm who will maintain the distribution service, handle vulnerabilities and takedowns, review compatibility, and support failed installs and rollback. A single speculative integration or a request from the project's own organization is not sufficient evidence for a marketplace.

## Requirements before third-party distribution

Treat these as release gates for any future worker sharing mechanism:

- **Publisher identity and signatures:** identify the publisher, bind the signature to the exact package and metadata, and provide a trust-root and key-revocation process.
- **Immutable, pinned versions:** prohibit in-place replacement of published versions; let organizations pin versions and opt in to upgrades.
- **Compatibility policy:** declare supported OrgDoc CLI, worker protocol, schema, operating system, architecture, and language-runtime ranges. Reject incompatible packages before installation or execution.
- **Dependency provenance:** retain lockfiles and hashes, provide source provenance and a software bill of materials, and record licenses and build inputs.
- **Install lifecycle:** review contents before install; install atomically; track ownership; support uninstall, rollback, and recovery from interrupted upgrades without overwriting unrelated commands or files.
- **Permission review:** show requested filesystem, process, credential, and network access to the installing organization. Manifest declarations alone are not enforcement.
- **Network policy:** default worker network access to denied or explicitly opt-in, define permitted destinations, and explain how credentials are scoped. The policy must be enforced outside worker code.
- **OS-enforced isolation:** define filesystem, process, resource, and network boundaries using platform mechanisms. Run an explicit threat model and publish unsupported-platform limits before untrusted code is eligible for installation.
- **Operations:** define vulnerability reporting, package withdrawal, incident response, and audit records for publisher, version, approval, install, update, and removal.

Do not claim that signatures make code safe or that a manifest's permission list is a sandbox. Publisher authentication, package integrity, code review, and execution isolation solve different problems and require separate controls.

## Current trust boundary and limits

Today, worker code runs under the current user's operating-system identity. It may read or write any path available to that user, access the network, start child processes, and create side effects outside the job artifact directory. The runtime validates declared schemas and reported output paths, starts workers without a shell, bounds execution time and captured output, limits OrgDoc job concurrency, filters the child environment, and redacts common credential patterns. Those controls reduce accidental damage and leakage; they do not isolate a worker.

On Linux and macOS the runner terminates the worker process group on timeout or cancellation. On other platforms it does not guarantee termination of descendants. Checksums do not authenticate the publisher, and environment filtering cannot prevent a worker from reading credentials stored in accessible files. Use only code the organization has reviewed and trusts. See the [security model](../product/security-model.md) for the full enforced-control and non-guarantee lists.

## Explicit product boundaries

- Electronic invoicing, tax-authority submission, and fiscal certification are out of scope. OrgDoc may render an invoice document from typed business data; it does not certify or submit that document.
- Humanizer remains a Skill invoked by an agent for requested narrative edits; typed values and protected facts remain outside its editing scope.
- Agent integration remains CLI plus Skills. This adoption gate does not introduce MCP or a third-party marketplace.
