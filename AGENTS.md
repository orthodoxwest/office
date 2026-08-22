# Data maintenance and diurnal ingestion guardrails

The received monastic diurnal is evidence for reviewing and, when the
mechanical gate allows, updating the office corpus. Extraction, OCR, heading
mapping, and model output remain witnesses — they are not printed-page
verification.

## Non-negotiable boundaries

- Automated diurnal ingestion, OCR cleanup, and model-generated corpus text
  may change `data/texts/` **only** through `office review apply` on an
  apply-queue packet that passed the mechanical gate.
- A narrow direct edit under `data/texts/` is allowed only when a maintainer
  explicitly names the exact target and requested action. This includes
  deletion of a named orphan proper scaffold whose owner is absent and which
  has no live sections. Do not use this exception for agent-initiated, OCR,
  extracted, or model-generated wording. Keep the edit reviewable and verify
  it with the relevant validation, audit, calendar, and office-composition
  tests.
- Do not directly edit `data/texts/psalms/`, which has been independently
  validated, unless a maintainer explicitly requests a particular correction
  backed by an independently checked source.
- First-wave generated applies must not write `data/review/` ledgers, feast
  metadata, hour defs, signoffs, attestations, or prescreen flags.
- Never represent an OCR result, a match score, a model verdict, or a merged
  apply PR as a human verification. `review attest` remains a separate human
  act. Agent-written `# SOURCE:` comments must keep the hedge
  `agent-proposed, not attested`.
- The newest local archdiocesan ordo is the authority for the current year.
  If it conflicts with older ordos or the normative rubrics, flag the issue for
  clergy rather than selecting a side. Do not apply those packets.
- Do not commit copyrighted book pages, extracted text, OCR text, images, or
  page coordinates. Generated intake and review artifacts belong only beneath
  the gitignored `output/` directory.
- Preserve source witness identity (document hash, PDF page, printed folio,
  extractor/OCR route, and text hash) in every packet. Do not combine text
  from different documents or mixed extractor routes. Same-document
  consecutive continuation of one mapped slot is required for page-spanning
  entries, not forbidden.

## Agent roles and provider context

- Use the project `diurnal_reconciler` role (Luna, medium reasoning) for bounded,
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
4. Run agents with immutable packet hashes. A cleaner may emit an apply
   packet; the gate, not the model score, decides whether it is writable.
5. `office review apply` writes gated ingestion packets on a branch. A
   maintainer merges the PR. Attestation is a later, separate `review attest`.

Explicit maintainer-directed repository maintenance is outside the automated
ingestion path and may use the narrow exception above. It must remain small
enough to review and must not relabel agent work as human attestation.

Follow the operational details in `DIURNAL-INGESTION.md`.
