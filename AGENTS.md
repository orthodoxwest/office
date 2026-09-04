# Diurnal ingestion guardrails

Use the page-image transcription and discovery workflow documented in
[scripts/DIURNAL-PIPELINE.md](scripts/DIURNAL-PIPELINE.md). OCR locates pages;
wording comes from reading the cited page images.

## Boundaries

- Keep source PDFs outside the repository. Do not commit copyrighted book
  pages, extracted text, OCR text, images, or page coordinates. Generated
  caches, prompts, reader results, and reports belong beneath ignored
  `output/`.
- Preserve the page cache with its document hash and render settings. Keep
  PDF page identity, detected or inferred printed labels, and reader results
  in run artifacts.
  Attest the found printed page, not an unresolved queue citation.
- Use the workflow's comparison checks before applying. Exact/near readings
  may be attested; replacements and discovered propers require independent
  reader agreement. Automatic psalm and canticle replacements remain
  `needs-human`. OCR and model confidence alone do not establish wording.
- Corpus updates use `office corpus put`; source attestations use
  `office review attest`. Ingestion does not change feast metadata, hour
  definitions, structural signoffs, or prescreen flags.
- The newest local archdiocesan ordo is the authority for the current year.
  If it conflicts with older ordos or normative rubrics, flag the issue for
  clergy rather than selecting a side. Do not apply affected entries.

## Providers and review

- Use the transcription/discovery scripts' bounded reader calls and structured
  results. The old packet reconciler role and agent scheduler are retired.
- Preserve provider login environments, including `HOME` and XDG variables.
  A sandbox authentication error means the execution context may be
  unavailable; it does not establish that the account is logged out.
- Claude Sonnet supplies independent second readings where required by the
  workflow. Readers return evidence; the scripts control corpus writes.
- Use the `diurnal_reviewer` role (Terra, high reasoning) before any pull
  request that changes the ingestion pipeline or its safety controls.

## Workflow

1. Render and index pages with `make pages`.
2. Prepare prompts with `make transcribe`, or feast dossiers with
   `make discover`. The provenance queue and generated runtime resolution
   inventory remain the baselines; distinguish printed propers from fallbacks.
3. Inspect the cache and prompts, then use `APPLY=1` to enable readers and
   application through the workflow's checks.
4. Review the run report and handle `needs-human` rows separately. Keep corpus
   changes and hash-bound source attestations reviewable in Git. A Codex
   attestation has the limited meaning documented in the pipeline guide.
