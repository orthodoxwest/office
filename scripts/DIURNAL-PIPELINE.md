# Diurnal page transcription pipeline

This is a narrow, page-first review loop. It maps corpus keys to cited printed
pages, renders those pages into a content-bound cache under ignored `output/`,
asks a vision reader for literal transcription, and compares that witness with
the live corpus. OCR is retained only for locating pages; it is never accepted
as wording. The older intake/reconcile/agent tools remain available but are not
part of this loop.

The cache records the PDF SHA-256 and render DPI. It stores one PNG per PDF page
and an index containing plain and layout OCR plus detected printed labels
(roman front matter, arabic body pages, and starred appendix pages). A missing
label is inferred only inside a bounded, consecutive run of the same numbering
series.

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

For page diagnostics, the underlying helpers are also useful directly:

```bash
python3 scripts/diurnal-pages.py locate monastic-diurnal 595
python3 scripts/diurnal-pages.py locate monastic-diurnal xxvi
python3 scripts/diurnal-pages.py find monastic-diurnal "blessed Athanasius"
```

## Classification and application

- `exact`: the trimmed transcription and corpus strings are identical.
- `near`: comparison normalization makes them equal, or their normalized
  similarity is at least 0.985. Normalization covers whitespace, quote forms,
  ligatures, soft and line-end hyphens, terminal punctuation, response sigils,
  and case; it does not supply words.
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

“Attested by codex” means a mechanical, hash-bound statement that the current
corpus entry agrees word-for-word after the documented normalization with the
cited printed page image. The provenance ledger stores the corpus hash, source,
printed page, reviewer name, date, and a content-free note pointing to the
cached PNG. It does not mean that Codex supplied wording, resolved a rubric, or
certified another edition.
