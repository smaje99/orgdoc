# Humanizer policy

Humanizer is an optional skill for narrative edits requested by a person or an agent workflow. It supports OrgDoc's document consistency promise by applying the selected brand voice while keeping the document's facts and typed fields stable.

## Editing contract

- Spanish is the default language unless the request or document requires another language.
- Only edit narrative fields allowed by the request and document type. Report summaries and explanatory prose may be revised. Invoice descriptions may be revised when requested.
- Keep protected fields exactly as supplied. This includes identifiers, dates, quantities, prices, taxes, totals, typed values, quotations, source references, and citations.
- Preserve factual meaning, attribution, evidence, uncertainty, and scope. Do not invent specificity to make prose sound more natural.
- Validate the edited document against its request schema and compare protected values with the original before rendering.
- Preserve the pre-edited narrative in the run manifest only when the user or organization enables that retention. `orgdoc document render --pre-humanized-text PATH` stores the source text verbatim.
- Do not claim human authorship or help evade AI detectors. Detector results are not a measure of factual accuracy or brand fit.

## Brand voice

The selected operation determines one brand. Use that profile's `voice.language`, `voice.tone`, `voice.avoid`, and optional `voice.approved_examples`. Examples teach style, not facts. Do not blend voice rules or assets from another brand.

## Public skill review

Reviewed on 2026-10-14:

| Candidate | License | Observed behavior | Decision |
| --- | --- | --- | --- |
| [blader/humanizer](https://github.com/blader/humanizer) | MIT, as stated by its repository | General prose editing based on 26 writing patterns; the README emphasizes editing for readers, fact checks, voice samples, and says detector evasion is not a goal. Its pattern catalogue is based on Wikipedia's "Signs of AI writing". | Use only the broad ideas of conservative editing, factual checks, and voice matching. Do not copy its skill, examples, pattern list, or Wikipedia-derived catalogue. |
| [hannsxpeter/humanizer](https://github.com/hannsxpeter/humanizer) | MIT, in the repository LICENSE | Pure-prompt editing with voice matching, restrained edits, anti-fabrication checks, and optional text hygiene. | Use only the broad ideas of preserving meaning and writer voice. Do not copy its prompt, examples, or references. Text hygiene is outside this skill's scope. |

The OrgDoc skill and examples are independently written for Spanish-first organizational documents and field-level preservation. No external skill text or pattern catalogue is redistributed.
