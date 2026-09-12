# Diurnal page transcription pipeline

This is a narrow, page-first review loop. It maps corpus keys to cited printed
pages, renders those pages into a content-bound cache under ignored `output/`,
asks a vision reader for literal transcription, and compares that witness with
the live corpus. OCR is retained only for locating pages; it is never accepted
as wording. This is the supported ingestion workflow.

The cache records the PDF SHA-256 and render DPI. It stores one PNG per PDF page
and an index containing plain and layout OCR plus detected printed labels
(roman front matter, arabic body pages, and starred appendix pages). A missing
label is inferred only inside a bounded, consecutive run of the same numbering
series. Roman front matter and starred appendix runs are repaired from their
sequence before lookup, so valid-looking OCR errors cannot split a run.

## Commands

Render the main Diurnal and every PDF in `../resources/books/supplements/`:

```bash
make pages                       # DPI=150 by default
make pages DPI=200
make pages BOOKS_DIR=/secure/resources/books
```

Prepare a provider-free pilot. This writes prompts under a new
`output/transcribe/<run-id>/` directory and makes no corpus or review-ledger
changes:

```bash
make transcribe
make transcribe KEYS=proper/st-athanasius/collect
```

After inspecting the prompts and cache, explicitly enable provider execution
and gated application:

```bash
make transcribe APPLY=1 KEYS=proper/st-athanasius/collect
```

Then print the PR-ready summary using the run path reported by the command:

```bash
make transcribe-report RUN=20260902T180000Z
```

Rows whose queue status is `source-unknown` are included by default alongside
`needs-review`. Since they have no citation to resolve, their search begins
with the opening words of the live corpus entry and then tries the feast name
plus slot description. Limit a run to either class with a repeatable status
flag:

```bash
python3 scripts/diurnal-transcribe.py run --dry-run --status source-unknown
python3 scripts/diurnal-transcribe.py run --dry-run --status needs-review
```

## Discovering silent proper fallthroughs

The discovery command sweeps the 2026 fallback resolution inventory and groups
eligible sanctoral, commemoration, and explicitly modelled temporal rows into
one dossier per feast. It considers only the kinds of sections printed in a
feast proper and excludes weekday/temporal-week fallthroughs. OCR date lines,
running heads, and titles locate a run of at most eight cached pages; OCR never
supplies corpus wording.

Prepare dossiers and prompts without readers or corpus writes:

```bash
make discover
make discover FEASTS=st-stephen-hungary
make discover MONTH=9 LIMIT=10
make discover SLOTS=collect,chapter-lauds LIMIT=10
```

Each run writes `output/discover/<run-id>/queue.md`: one task per feast, with
linked scan images, exact requested corpus keys, current fallbacks, example
appointments and reading results. Start with a small, curated set of feasts and
one or two section types. `SLOTS` uses exact section names; for example,
`chapter-first-vespers` and `chapter-vespers` are separate appointments.

Before handing off the batch, inspect the linked pages and prompts. Check that
the range contains the right feast and its boundaries, that the requested
sections correspond to the office being examined, and that no pending clergy
question or ordo/source conflict affects the proposed appointment. Page location
uses OCR and does not perform this review. Tasks without located pages need
source research before they can become simple reading tasks.

A volunteer can take a named task from `queue.md` and return its slot ID, a
literal reading and printed page, or the visible cross-reference/absence, or
an uncertainty note. Keep that evidence in ignored `output/` for review; the
queue itself is not permission to write or attest a proper.

To use the bounded model reader on the **same prepared batch**, read the next
three unread tasks without corpus writes:

```bash
make discover-resume RUN=20260902T190000Z LIMIT=3
```

Repeat the command to advance through the batch. Completed and uncertain
readings and the actual prompt used are retained in `results.jsonl`; `queue.md`
shows the latest result, scope notes and slot explanations for each task.
Review those explanations too: a correct negative can still cite a neighboring
feast by mistake; retry it explicitly before accepting that search.
Low-confidence absence is `needs-human`, not a
completed negative search. A negative reading closes only that page search:
it does **not** establish that the runtime fallback is correct. Missing-page and
uncertain tasks are held rather than automatically retried. To deliberately
retry a selected task, use `FEASTS=<id> RETRY=1`.

Resume checks the prepared PDF, page images, page-label index, feast context,
appointments and fallback wording. If they changed, prepare and inspect a fresh
batch. Successful application results remain history, since their writes change
the fallback inventory; retrying those also requires fresh preparation. Older
runs without these source fingerprints must be prepared again. Wait for initial
preparation to finish before resuming. A per-run lock prevents two resume
processes from taking the same tasks concurrently.

After reviewing a printed-proper candidate, enable gated application. Resume
reads the candidate again and obtains the existing independent second reading;
a saved positive result never authorizes a write by itself:

```bash
make discover-resume RUN=20260902T190000Z FEASTS=st-stephen-hungary APPLY=1
make discover-report RUN=20260902T190000Z
```

The original fresh-run command, `make discover APPLY=1 FEASTS=<id>`, is also
available. Both paths use the same application checks below. Queue files and
reports describe a particular search, not an authoritative missing-proper count.

The first reader sees every located page for the feast in one call and must
classify each fallback as actual printed proper text or as absent/cross-referred.
It also records unrequested printed sections as `extra`; those are never
applied. A high/medium printed candidate is rejected as `same-as-fallback` when
it is at least 0.9 similar to the currently resolved fallback. Otherwise one
Claude Sonnet reading of the named slot and page must agree at 0.985 before
`office corpus put` and a Diurnal attestation. Disagreement, low confidence,
an unrecognised printed page, or an apply error remains `needs-human` with a
representative web URL in the report. All dossiers, prompts, reader output,
and body files remain under ignored `output/`.

For page diagnostics, the underlying helpers are also useful directly:

```bash
python3 scripts/diurnal-pages.py locate monastic-diurnal 595
python3 scripts/diurnal-pages.py locate monastic-diurnal xxvi
python3 scripts/diurnal-pages.py find monastic-diurnal "blessed Athanasius"
python3 scripts/diurnal-pages.py feast-pages 7 17 "Translation of St. Osmund"
```

## Classification and application

- `exact`: the trimmed transcription and corpus strings are identical.
- `near`: comparison normalization makes them equal, or their normalized
  similarity is at least 0.985. Normalization covers whitespace, quote forms,
  ligatures, soft and line-end hyphens, terminal punctuation, response sigils,
  leading printed slot labels, flex/mediant mark variants, and case; it does
  not supply words.
- `different`: the first readable transcription is below that threshold.
- `not-found`: the requested section is absent, empty, or its page cannot be
  resolved.
- `low-confidence`: the reader reports low confidence or cannot return a
  bounded, schema-valid result.

With `--apply`, exact and near rows are attested. A different row goes to a
second independent reader; only transcriptions agreeing at 0.985 or better may
replace the section and then be attested. Disagreement, unreadable text, missing
pages, and automatic psalter replacements remain `needs-human`. Page images,
transcriptions, diffs, and prompts stay under ignored `output/` and must not be
committed.

The reader tries the cited printed label, the same number as a PDF page, a
short corpus-text OCR search, and a feast/slot OCR search in that order. It
stops at the first found result and permits at most three reader calls per key.
Results record the successful `locate_strategy`, the cited label separately,
and full primary and secondary reader objects as `first` and `second`. Any
attestation uses the found page's detected or inferred printed label from the
cache index, never the queue's cited number.

“Attested by codex” means a mechanical, hash-bound statement that the current
corpus entry agrees word-for-word after the documented normalization with the
located printed page image. The provenance ledger stores the corpus hash, source,
printed page, reviewer name, date, and a content-free note pointing to the
cached PNG. It does not mean that Codex supplied wording, resolved a rubric, or
certified another edition.
