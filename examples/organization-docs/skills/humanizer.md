# Humanizer for this organization

Use the repository's [`humanizer` skill](../../../skills/humanizer/SKILL.md) only for requested narrative edits. Resolve the operation before editing and read the `voice` object from its selected brand profile.

- Henko Technologies: clear, professional, direct Spanish; avoid unsupported claims and unnecessary jargon.
- Henko Academy: didactic, approachable, respectful Spanish.

Use each document type's `protected_fields` and request schema as the editing boundary. Invoice descriptions can change only when requested; amounts, identifiers, dates, quantities, rates, taxes, totals, quotations, and citations stay exactly as provided. Report summaries and explanatory prose can be edited while source references, quotations, and citations remain unchanged.

Keep original prose in the run record only when the user or organization enables it. Then pass the original file with `--pre-humanized-text` during `document render`.
