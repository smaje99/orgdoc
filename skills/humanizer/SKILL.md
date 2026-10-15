---
name: humanizer
description: Revise requested narrative prose in OrgDoc documents in Spanish first, follow the selected brand voice, and preserve typed fields, facts, quotations, and citations.
---

# Humanizer

Use this skill only when someone asks to humanize, polish, or make narrative text sound more natural. It is an optional editorial pass, not a renderer or a default step in every document run.

## Choose what can change

1. Read the selected document type's `protected_fields` and request schema. Treat both as the contract for the current edit.
2. Read [brand-voice.md](references/brand-voice.md) when an organization profile is available. Apply only the selected operation's brand voice.
3. Edit narrative prose that the request or profile allows. For reports this can include the executive summary and explanatory prose in findings. For invoices, limit edits to an explicitly editable description field.
4. Keep protected values exactly as supplied. This includes amounts, quantities, rates, totals, identifiers, names, dates, typed values, quotations, source references, and citations. Do not reformat protected values.

When prose and structured values share one input, copy the structured values from the source into the result. Do not reconstruct them from the edited prose. If a requested change would require changing a protected value, leave it untouched and explain the conflict.

## Edit for the reader and brand

- Write in Spanish by default. Use another language when the user or document explicitly requires it.
- Prefer clear, specific sentences that preserve the original meaning and evidence.
- Follow the selected brand's `voice.language`, `voice.tone`, `voice.avoid`, and any `voice.approved_examples`. Treat approved examples as style guidance, not as facts to reuse.
- Keep uncertainty, attribution, and qualifications that the sources require. Do not add causes, claims, figures, sources, quotations, or interpretations.
- Preserve the author's intent and deliberate terminology. Change only what improves the requested passage.
- If the source does not support a detail needed to improve the prose, ask for that detail or leave the sentence as it is.

## Check the result

1. Compare every protected field with the original. Values must remain byte-for-byte unchanged; for structured input, reuse the original serialized value.
2. Recheck the edited document against its request schema and required fields.
3. Confirm each factual statement still has the same source and scope as before.
4. Return the edited narrative and a short note naming the narrative fields changed and confirming that protected fields were preserved. Do not imply that the text was written by a human.

If the user or organization has enabled preservation of the pre-edited text, retain the original narrative with the run record. For the OrgDoc CLI, pass its file with `--pre-humanized-text ORIGINAL.md`; this stores the file contents in `run-manifest.json`. Leave this option unused when retention is not enabled.

## Boundaries

This skill edits for readers, clarity, and brand consistency. It does not help evade AI detectors, promise detector outcomes, or fabricate a human identity, credentials, or authorship claim. Do not change prose that the user did not ask to edit.
