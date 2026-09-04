# Monastic diurnal ingestion

Use the [page-image transcription and discovery workflow](scripts/DIURNAL-PIPELINE.md).
It replaces the former intake, source-reconciliation, agent scheduler, and
`office review apply` pipeline.

- `make pages` renders and indexes source PDFs beneath ignored `output/`.
- `make transcribe` prepares page-reading prompts for the provenance queue.
- `make discover` prepares feast dossiers from the runtime resolution inventory.
- `APPLY=1` enables readers and the workflow's comparison checks before
  `office corpus put` and `office review attest`.
- `make transcribe-report RUN=...` and `make discover-report RUN=...` summarize
  completed runs and unresolved rows.

Keep source PDFs outside the repository and source-derived artifacts beneath
ignored `output/`. OCR locates pages; page-image readings supply wording.
The provenance tracker, queue, and attestations remain supported. Historical
artifacts and decisions from the retired workflow remain local under `output/`;
they are not inputs to the replacement workflow.
