# Monastic diurnal ingestion

This tooling prepares evidence for reviewing a received monastic diurnal and,
when the mechanical gate allows, writing corpus edits through
`office review apply`. It does not attest provenance or decide rubrics.
Keep the PDF and all source-derived artifacts outside Git; intake, discovery,
and apply-queues live under ignored `output/`. Corpus writes happen only via
the apply command on a reviewable branch.

## Workflow

1. **Register, extract, and inspect.**

   ```bash
   scripts/diurnal-intake.py doctor
   scripts/diurnal-intake.py ingest /secure/path/diurnal.pdf --source-key monastic-diurnal
   scripts/diurnal-intake.py qa output/diurnal-ingest/monastic-diurnal-<hash>
   ```

   Intake first tests the native text layer, then routes weak pages to rendered
   images and OCR when available. Check mixed, unavailable, and failed pages before
   trusting a segment. OCR is optional: install `tesseract` on the host for
   scanned pages. A missing OCR tool is an explicit
   `ocr-unavailable` result, never a fabricated transcription.

2. **Generate the actual runtime baseline.**

   ```bash
   ./office review resolution-inventory -start 2026 -years 28 -json > output/resolution-inventory.json
   ```

   The inventory records the owner, requested slot, direct targets, selected
   target, fallback tier, reason, and observed dates. It is essential for
   recognizing printed text that merely agrees with an inherited/common/ordinary
   fallback.

3. **Create an edition profile and discover witnesses.**

   Copy `scripts/diurnal-profiles/template.json` to an ignored profile beneath
   `output/diurnal-profiles/`, then map only headings and slots you can identify
   in this edition. Keeping the working profile beneath repository `output/`
   lets discovery record a portable, confined dependency rather than a private
   host-absolute path. Validate its shape with
   `scripts/diurnal-profiles/schema.json` if a JSON-schema tool is available.

   ```bash
   scripts/source-reconcile.py discover \
     --intake output/diurnal-ingest/monastic-diurnal-<hash> \
     --inventory output/resolution-inventory.json \
     --profile output/diurnal-profiles/monastic-diurnal-profile.json \
     --output output/source-reconcile
   ```

   Discovery writes deterministic `proper-discovery.json`/`.csv`,
   `printed-unmapped.csv`, `runtime-unwitnessed.csv`, and
   `proper-discovery-summary.json`. It classifies each witness as one of:
   `verify-existing`, `missing-override`, `fallback-equal`,
   `existing-different`, `unmodeled-slot`, `known-owner-unobserved`,
   `unknown-feast`, `ambiguous-owner/context`, or `rubrical-complex`.
   Source-first rows for known calendar owners remain visible even if the
   selected runtime date span did not observe them.

4. **Use bounded, provider-neutral agent passes.**

   ```bash
   scripts/diurnal-agent.py doctor
   scripts/diurnal-agent.py plan output/source-reconcile/proper-discovery.json
   scripts/diurnal-agent.py run                 # dry run: shows ready jobs
   scripts/diurnal-agent.py run --execute       # one bounded attempt per ready job
   scripts/diurnal-agent.py status
   scripts/diurnal-agent.py collect
   scripts/diurnal-agent.py adjudicate          # only after replica disagreement
   scripts/diurnal-agent.py run --execute       # bounded Claude adjudication
   ```

   Luna is the default cheap reconciler; use Grok replicas for ambiguous rows
   and Claude only for rate-limited adjudication. Agent commands preserve the
   host login environment—do not use `env -i`, clear `HOME`/XDG, or force an
   alternate Codex user config. Discovery records full, repository-relative
   file hashes plus aggregate intake/corpus tree hashes. A task is stale when
   its packet, profile, prompt, policy, inventory, intake artifacts, corpus, or
   Git revision changes; re-plan rather than reusing its answer. Workers have
   leases and bounded retries; `status` and
   `reap` recover interrupted work without silently changing a verdict.
   Ambiguous candidates run Luna and Grok as separate attempts; invoke
   `run --execute` again for the replica after the first provider succeeds.
   Failed transient attempts remain queued up to the policy cap, so the same
   command retries them; `reap` releases an expired interrupted lease.
   Claude is disabled in the tracked policy by default to protect its limited
   quota; enable `providers.claude.enabled` in a local policy copy beneath
   repository `output/` and pass it to `plan` before using `adjudicate`.

5. **Gate, apply, then a corpus PR.**

   High-confidence packets (`missing-override` or `existing-different` that
   pass `internal/review` Gate) become an apply-queue under `output/`. The
   only writer is:

   ```bash
   scripts/source-reconcile.py apply-queue --output output/source-reconcile
   ./office review apply --dry-run output/source-reconcile/apply-queue.json
   ./office review apply output/source-reconcile/apply-queue.json
   ```

   Apply never writes provenance, signoffs, psalms, or feast metadata. A
   merged apply PR accepts wording into the corpus; it is not a printed-page
   attestation. Record `review attest` separately after a person checks the
   page.

   Unresolved `rubrical-complex`, `needs-ruling`, ambiguous owners, and
   ordo-vs-diurnal conflicts stay in `output/` and do not apply.

## Artifact contract

An intake page record carries the source/document SHA-256, PDF page, optional
printed folio and bounding box, extractor route, confidence/QA route, raw-text
SHA-256, and canonical text. Discovery adds the canonical owner, runtime slot,
selected target/tier/reason, mapping confidence, classification, and note. A
schema-v2 discovery packet also carries a content-free dependency manifest:
full hashes for each materialized input and tree hashes that detect later file
additions or deletions. Intake locators remain beneath repository `output/`;
the manifest contains no host-absolute paths or extracted source text. A
new diurnal candidate ID is stable per document hash, page span, and witness
hash: `DI-<source12>-P<page>-<witness12>` for one page, or
`DI-<source12>-P<first>-P<last>-<witness12>` for a continuation. Legacy `SR-*`
reconciliation packets and their `decisions.csv` remain independent.

## Recovery and safety checklist

- Never delete a run to “fix” OCR; rerun or resume it and preserve the page
  witness/hash trail.
- Resolve ambiguous headings, multi-option rubrics, and office-boundary
  crossings manually before requesting an agent pass. Same-document
  consecutive continuation of one mapped slot is joined automatically.
- Do not treat a score as proof: compare the rendered image and source page.
- If a current ordo differs from older material, flag it for a ruling; the
  newest-year ordo governs current practice.
- Before opening a PR for tooling, run the adversarial `diurnal_reviewer` and
  address its safety findings. Never commit the copyrighted PDF or any source
  extraction artifact.
