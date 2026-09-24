# Diurnal ingestion guardrails

Ingest texts only through the page-image workflow in
[scripts/DIURNAL-PIPELINE.md](scripts/DIURNAL-PIPELINE.md). OCR locates pages;
wording comes from reading the page images.

## Boundaries

- Keep source PDFs outside the repository. Never commit book pages, extracted
  or OCR text, images, or page coordinates; caches, prompts, reader results,
  and reports belong under ignored `output/`.
- Keep the page cache with its document hash and render settings, and keep PDF
  page identity, printed labels, and reader results in run artifacts. Attest
  the page actually found, not the queue's citation.
- Exact and near readings may be attested. Replacements and discovered propers
  need independent reader agreement; psalm and canticle replacements stay
  `needs-human`. OCR or model confidence never establishes wording.
- Write the corpus with `office corpus put` and attestations with
  `office review attest`. Ingestion never changes feast metadata, hour
  definitions, composition rules, or prescreen flags.
- The newest local archdiocesan ordo governs the current year. If it conflicts
  with older ordos or the rubrics, flag it for clergy and don't apply the
  affected entries.

## Providers

- Use the scripts' bounded reader calls and structured results. Readers return
  evidence; the scripts control corpus writes. Claude Sonnet supplies the
  independent second reading.
- Preserve provider login environments (`HOME`, XDG variables). A sandbox
  authentication error may mean the context is unavailable, not that the
  account is logged out.
- Run the `diurnal_reviewer` role (Terra, high reasoning) before any PR that
  changes the pipeline or its safety controls.

## Workflow

1. `make pages` renders and indexes pages.
2. `make transcribe` prepares prompts from the provenance queue; `make discover`
   prepares feast dossiers from the resolution inventory. Distinguish printed
   propers from fallbacks.
3. Inspect the cache and prompts, then rerun with `APPLY=1`.
4. Review the run report (`make transcribe-report` / `make discover-report RUN=…`)
   and handle `needs-human` rows separately. Keep corpus changes and
   attestations reviewable in Git; a Codex attestation has the limited meaning
   given in the pipeline guide.
