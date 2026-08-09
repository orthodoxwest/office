# Diurnal ingestion guardrails

The received monastic diurnal is evidence for a human review workflow, not an
automatic source of corpus edits. Treat every extraction, OCR result, heading
mapping, and runtime comparison as an advisory witness until a maintainer has
reviewed it against the printed page.

## Non-negotiable boundaries

- Agents may propose structured findings, diffs, and questions. They must not
  modify `data/texts/`, `data/review/`, provenance records, signoffs,
  attestations, prescreen flags, or decision files.
- Never represent an OCR result, a match score, or an agent conclusion as a
  human verification. Printed-book evidence requires human review.
- The newest local archdiocesan ordo is the authority for the current year.
  If it conflicts with older ordos or the normative rubrics, flag the issue for
  clergy rather than selecting a side.
- Do not commit copyrighted book pages, extracted text, OCR text, images, or
  page coordinates. Generated intake and review artifacts belong only beneath
  the gitignored `output/` directory.
- Preserve source witness identity (document hash, PDF page, printed folio,
  extractor/OCR route, and text hash) in every proposal. Do not combine text
  from different witnesses.

## Agent roles and provider context

- Use the project `diurnal_reconciler` role (Luna, low reasoning) for bounded,
  repeatable packet classification. Its output is advisory JSON, never a patch.
- Use Grok for inexpensive independent replicas only where ambiguity warrants
  comparison. Invoke its authenticated `grok` CLI in the host context; do not
  clear `HOME`, XDG variables, or other auth environment. A sandbox auth error
  means execution context is unavailable, not that the account is logged out.
- Reserve Claude Sonnet for sparse adjudication because its rate limits are
  limited. Use it to compare competing advisory findings, not to make edits.
- Use the `diurnal_reviewer` role (Terra, high reasoning) before any pull
  request that changes the ingestion pipeline or its safety controls.

## Safe workflow

1. Keep source files outside the repository and run intake into `output/`.
2. Inspect extraction/OCR QA before segmentation or matching.
3. Compare only against a generated runtime resolution inventory; distinguish
   a printed proper from an equal fallback.
4. Run agents with immutable packet hashes and collect their structured advice.
5. A human records a decision, then separately prepares and reviews any corpus
   change and provenance attestation through the normal PR workflow.

Follow the operational details in `DIURNAL-INGESTION.md`.
