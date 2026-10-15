# Install the OrgDoc agent Skill

The portable OrgDoc Skill is versioned at `skills/orgdoc/SKILL.md`. It teaches an agent to use the CLI, resolve organization context explicitly, render supported documents, and review artifacts. It is separate from an organization's local Skills and does not configure or install workers.

## Project-local installation

Use this when everyone working in one organization repository should discover the same OrgDoc instructions. The core release archive includes `skills/orgdoc/SKILL.md`; a source checkout has the same path. Copy the `orgdoc` folder into the project-level Skills directory recognized by the target agent, or configure that agent to read the source path directly. Agents use different discovery paths, so check the selected agent's project Skill documentation before copying. Keep organization-owned instructions such as `organization-docs/skills/humanizer.md` inside the organization repository.

After replacing the example path with the project Skill root documented by the selected agent, the copy looks like this:

```sh
AGENT_PROJECT_SKILLS=/path/to/agent/project-skills
mkdir -p "$AGENT_PROJECT_SKILLS/orgdoc"
cp skills/orgdoc/SKILL.md "$AGENT_PROJECT_SKILLS/orgdoc/SKILL.md"
```

## User-level installation

Use this when one person wants OrgDoc available across their projects. Copy the `orgdoc` Skill folder into that agent's documented user-level Skills directory. The install path is agent-specific; do not assume a Codex, Claude, or other agent directory is interchangeable. Update the copied Skill when updating OrgDoc.

Replace the example path with the selected agent's documented user Skills directory:

```sh
AGENT_USER_SKILLS=/path/to/agent/user-skills
mkdir -p "$AGENT_USER_SKILLS/orgdoc"
cp skills/orgdoc/SKILL.md "$AGENT_USER_SKILLS/orgdoc/SKILL.md"
```

Humanizer remains optional and separate. Install it only if the agent supports Skills and the user wants narrative edits. `organization init` never writes to any user-level Skills directory and never installs Humanizer.

## Check the integration

After installing the Skill, ask the agent to run `orgdoc --help`, `orgdoc doctor --organization PATH --format json`, and `orgdoc capabilities list --organization PATH --format json`. The CLI must be installed separately, and only the workers required by the organization need to be installed.
