# Diurnal page transcription pipeline

The supported ingestion workflow. It maps corpus keys to cited printed pages,
renders those pages into a content-bound cache under ignored `output/`, has a
vision reader transcribe them literally, and compares that reading with the
live corpus. OCR only locates pages; it is never accepted as wording.

The cache records the PDF SHA-256 and render DPI, one PNG per page, and an
index of plain and layout OCR plus detected printed labels (roman front
matter, arabic body pages, starred appendix pages). A missing label is
inferred only inside a bounded consecutive run of the same series; roman and
starred runs are repaired from their sequence so an OCR misread can't split a
run.

Source PDFs, page images, OCR, prompts, reader output, and reports all stay
under ignored `output/` and must never be committed.

## Commands

Render the main Diurnal and every PDF in `../resources/books/supplements/`:

```bash
make pages                       # DPI=150 by default
make pages DPI=200
make pages BOOKS_DIR=/secure/resources/books
```

Prepare a run. This writes prompts under `output/transcribe/<run-id>/` and
calls no provider and changes nothing:

```bash
make transcribe
make transcribe KEYS=proper/st-athanasius/collect
```

After inspecting the prompts and cache, enable readers and gated application,
then print the PR summary for the reported run:

```bash
make transcribe APPLY=1 KEYS=proper/st-athanasius/collect
make transcribe-report RUN=20260902T180000Z
```

Runs include queue rows in both `needs-review` and `source-unknown`. With no
citation, `source-unknown` searches start from the entry's opening words, then
the feast name and slot. Restrict with a repeatable flag:

```bash
python3 scripts/diurnal-transcribe.py run --dry-run --status source-unknown
```

Page diagnostics:

```bash
python3 scripts/diurnal-pages.py locate monastic-diurnal 595
python3 scripts/diurnal-pages.py locate monastic-diurnal xxvi
python3 scripts/diurnal-pages.py find monastic-diurnal "blessed Athanasius"
python3 scripts/diurnal-pages.py feast-pages 7 17 "Translation of St. Osmund"
```

## Classification and application

- `exact`: trimmed strings are identical.
- `near`: equal after normalization of whitespace, quotes, ligatures, soft and
  line-end hyphens, terminal punctuation, response sigils, leading slot labels,
  flex/mediant marks, and case. Collect bodies also drop the conclusion cue,
  which the corpus stores separately. Normalization never supplies words.
- `different`: differs after normalization, whatever the similarity score.
- `not-found`: section absent or empty, or page unresolvable.
- `low-confidence`: reader reports low confidence or returns no valid result.

With `APPLY=1`, exact and near rows are attested. A `different` row goes to a
second independent reader (Claude Sonnet); only non-empty readings that agree
exactly after normalization replace the section and are then attested.
Disagreement, unreadable text, missing pages, and psalm or canticle
replacements stay `needs-human`.

The reader tries, in order: the cited printed label, the same number as a PDF
page, a short OCR search for the corpus text, and a feast/slot OCR search. It
stops at the first hit, with at most three reader calls per key. Results record
the `locate_strategy`, the cited label, and both readers as `first` and
`second`. Attestations always cite the **found** page's printed label, never
the queue's citation.

"Attested by codex" is a mechanical, hash-bound statement that the corpus entry
matches the located page image word-for-word after normalization. The ledger
stores the corpus hash, source, printed page, reviewer, date, and a
content-free note pointing to the cached PNG. It does not mean Codex supplied
wording, resolved a rubric, or certified another edition.

## Reviewed low-similarity replacements

The wrong-page check (similarity below 0.5) and replacement gate (below 0.6)
normally block a replacement. When the old text is genuinely unrelated,
prepare a held packet for explicit review:

```bash
python3 scripts/diurnal-transcribe.py prepare-replacement RUN --key proper/FEAST/SLOT \
  --context "Feast, exact slot, current Ordo/source appointment and scope checked"
```

This calls no provider. It writes a packet under the run's `replacements/`
directory, prints its SHA-256, and binds the current corpus body and
citations, the candidate reading, the final formatted body, engine and
calendar inputs, the PDF, render settings, page index, and images. It is
limited to the main Diurnal and excludes psalms and canticles. A stale reading,
a disputed printed label, or a collect containing its conclusion cue needs a
fresh transcription.

Inspect the images, feast heading, neighboring section boundaries, the
candidate **and output body**, and the appointment context. Confirm the current
Ordo supports the slot and no open ruling affects it; reader confidence and
agreement don't establish the appointment. Then apply the digest you inspected:

```bash
python3 scripts/diurnal-transcribe.py apply-replacement output/transcribe/RUN/replacements/PACKET.json \
  --packet-sha256 DIGEST --reviewer codex \
  --review-note "Page identity, feast/slot boundaries, wording and appointment checked against …"
```

Application makes two fresh readings (primary, then independent Claude Sonnet),
neither shown the candidate. Both must identify the reviewed page and agree
after normalization with each other, the candidate, and the output body;
anything else stays `needs-human`. Any source or appointment change
invalidates the packet. The script rechecks corpus and source under the
ingestion write lock before `office corpus put` and `office review attest`. A
failed attempt never tries another page; prepare a new packet instead.

## Discovering silent proper fallthroughs

Discovery sweeps the 2026 fallback resolution inventory and groups eligible
sanctoral, commemoration, and explicitly modelled temporal rows into one
dossier per feast. It considers only section kinds printed in feast propers,
excludes weekday/temporal-week fallthroughs, and holds appended-office rows
until an owner mapping exists. OCR date lines, running heads, and titles locate
at most eight cached pages.

```bash
make discover
make discover FEASTS=st-stephen-hungary
make discover MONTH=9 LIMIT=10
make discover SLOTS=collect,chapter-lauds LIMIT=10
```

Each run writes `output/discover/<run-id>/queue.md`: one task per feast, with
linked scans, requested corpus keys, current fallbacks, example appointments,
and reading results. Start with a few feasts and one or two slots. `SLOTS`
takes exact names (`chapter-first-vespers` and `chapter-vespers` differ).

Before running readers, inspect the pages and prompts: right feast and
boundaries, sections matching the office, and no pending clergy question or
ordo conflict. OCR location doesn't check any of this. Tasks with no located
pages need source research first.

Lent Ember searches need both a Lent running head and an explicit Ember
weekday heading (possibly mid-page). They include a continuation leaf through
the next weekday boundary within the eight-page cap and never fall back to an
Advent or Pentecost match. Searches bind to the feast ID, so aliases can't
bypass these checks. A located leaf can still contain neighboring offices, so
the feast and slot need page-image review.

A volunteer can take a task from `queue.md` and return the slot ID with a
literal reading and printed page, the visible cross-reference or absence, or
an uncertainty note. Keep that evidence in `output/`; the queue is not
permission to write or attest.

To run the model reader over the prepared batch, three tasks at a time and
without corpus writes:

```bash
make discover-resume RUN=20260902T190000Z LIMIT=3
```

Repeat to advance. `results.jsonl` keeps every reading and its prompt;
`queue.md` shows the latest result and slot explanations. Review those too: a
correct negative can cite a neighboring feast. Low-confidence absence is
`needs-human`, and a negative reading closes only that page search — it does
**not** show the fallback is correct. Missing-page and uncertain tasks are held;
retry deliberately with `FEASTS=<id> RETRY=1`.

Resume verifies the PDF, images, label index, feast context, appointments, and
fallback wording against the prepared batch; if any changed, or the run
predates these fingerprints, prepare a fresh batch. Successful applications
change the inventory, so retrying them also needs fresh preparation. A per-run
lock prevents concurrent resumes; let preparation finish first.

After reviewing a printed-proper candidate, enable gated application (or use
`make discover APPLY=1 FEASTS=<id>` on a fresh run):

```bash
make discover-resume RUN=20260902T190000Z FEASTS=st-stephen-hungary APPLY=1
make discover-report RUN=20260902T190000Z
```

The first reader sees all located pages in one call and classifies each
fallback as printed proper text or absent/cross-referred; unrequested sections
are recorded as `extra` and never applied. A high/medium candidate at least 0.9
similar to the current fallback is rejected as `same-as-fallback`. Otherwise a
Claude Sonnet reading of the named slot and page must agree exactly after
normalization before `office corpus put` and attestation; similarity scores
are diagnostic only. A saved positive result never authorizes a write by
itself. Word disagreement, low confidence, an unrecognised page label, or an
apply error stays `needs-human` with a web URL in the report. Queue files and
reports describe one search, not an authoritative missing-proper count.
