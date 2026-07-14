# Reviewing the Office

This guide is for volunteers checking the rendered hours against the printed
books: the diurnal and the archdiocese supplement. You do not need to know
anything about the code or about git — you need the books, a web browser, and
your assigned rows from the review checklist.

## How review is organized

There are two complementary review tracks:

1. **Text provenance** certifies an individual corpus entry once against a
   named source and page/section locator.
2. **Structural review** checks that the engine selected and assembled the
   right elements. A generated set-cover plan selects representative pages
   that exercise every known condition branch, occurrence/concurrence rule,
   and fallback tier.

This distinction prevents a wording check from being repeated merely because
the same text appears in several calendar contexts.

The unit of review is **one hour of one celebration**, not one calendar date.
The same composition recurs year after year, so checking "Trinity Sunday
Lauds" once covers every year it recurs. Some celebrations have more than one
variant (a year with a commemoration attached, a feast falling inside an
octave) — each variant is its own row in the checklist with its own link.

Each row in the checklist has:

- a **link** — the exact page to open (e.g. `/lauds/2026-06-07`)
- a **priority** — A (Sundays and 1st/2nd class feasts), B (greater doubles
  and doubles), C (everything else). Work top down.
- a **context** note — commemorations or octave the page should reflect.

The link's hour and date identify the page for sign-off. If its contents later
change, the sign-off is automatically marked stale and the page returns to the
queue; reviewers do not need to copy or manage a version identifier.

## What to look for

Open the linked page side by side with the books and check, in this order:

### 1. Missing propers

The most common seeded error: the app silently falls back to a generic text
where the diurnal or supplement gives a specific one. Warning signs:

- A major feast whose psalm antiphons look like the ordinary Sunday or
  weekday psalter.
- A hymn that is the ordinary weekday hymn rather than the feast's own
  (check the first line against the book).
- A chapter, versicle, or Benedictus/Magnificat antiphon that reads like a
  general default while the book has one proper to the day.

If the book has something more specific than what the page shows, that is a
finding — even if what the page shows is not "wrong" in itself.

### 2. Incorrect translations

The project was seeded from Divinum Officium, whose translations sometimes
differ from our diocesan books. Compare **word for word**, not just gist:

- Collects especially — small differences in wording are still findings.
- Watch for modern pronouns (you/your) where the books use thou/thee/thy,
  and for mixed registers within a single text.
- Psalm texts should match the Coverdale psalter as printed.

### 3. Logic and rubric errors

Check the **structure** of the hour, not just the texts:

- Are the right psalms appointed for this day and hour?
- On special days (Sundays coinciding with feasts, days within octaves,
  vigils, penitential seasons): does the page add, omit, and substitute what
  the rubrics direct? E.g. preces said or omitted, proper doxologies,
  I Vespers belonging to the following feast.
- Anything present that should be absent, or absent that should be present.

## Reporting what you find

Every hour page has a **"Report a problem"** link at the bottom. It opens a
GitHub issue already filled in with the page, date, and celebration — tick
the category, then write two things:

1. **What the books say** — quote it, and cite the diurnal or supplement
   page number if you can.
2. **What the app shows** — paste the text from the page.

If a page is fully correct, that is just as valuable: tell your coordinator
which linked hour and date you checked so it can be signed off.

## For the maintainer

```bash
make review-manifest > manifest.csv   # regenerate the checklist (START=2026 YEARS=1)
make review-status                    # coverage report: current / stale / unreviewed
make review-provenance                # generated text-provenance coverage
make review-provenance-queue > provenance-queue.csv  # highest-leverage texts first
make review-plan > review-plan.csv    # minimal structural checklist
make review-assurance                 # release coverage gates and summary
./office review explain lauds 2026-06-07  # one page's assurance JSON
./office review sign lauds 2026-06-07 REVIEWER [note...] # record a sign-off
```

Explicit text attestations live in `data/review/provenance.csv`. The file
records only citations, review metadata, the corpus key, and internal version
metadata; it does not copy or embed source-book contents. If an entry later
changes, its attestation automatically becomes stale without requiring
reviewers to handle version identifiers.

Prefer the safe CLI to manual CSV editing:

```bash
./office review attest --source "Printed Diurnal" --page 123 \
  --locator "Proper of Example" --note "word-for-word" \
  proper/example/collect reviewer
```

The command resolves the corpus key to its current content, validates every
field, and rewrites the ledger atomically; use `--replace` only when
deliberately superseding an existing attestation.

### Release assurance

`./office review assurance` fails when modeled structural features are
uncovered, modeled coverage drops below its intentional floor, or verified
text coverage falls below its floor. It reports stale attestations separately
so reviewers can see what changed. The floor lives in
`data/review/assurance-baseline.json`; update it only as an intentional,
reviewable change:

```bash
./office review assurance --update-baseline
```

Each web hour also has a collapsed **Assurance** disclosure. It shows the same
dependency states, fallback tiers, and stable rule identifiers without
revealing local paths or source contents. Unverified dependency rows link to a
prefilled review issue for that exact corpus key. `needs-review` means a source
lead or explicit review task exists but still needs verification;
`source-unknown` means provenance research must happen first.

Sign-offs live in `data/review/signoffs.txt`. The CLI binds each sign-off to
the exact page contents automatically, so any later edit makes the unit show
up as **stale** in `review-status` until re-reviewed. Sign-offs are committed
to git like any other data change.

### Pairing against local draft office books

When the local `../resources/` directory contains the full Lauds and Vespers
DOCX books, generate page-aware source comparisons with:

```bash
make review-sources
```

The command extracts into `output/source-reconcile/`, which is gitignored. It
does not edit the corpus, copy source text into tracked files, or record human
attestations. Start with `output/source-reconcile/README.md`, then work through
the small Markdown files under `batches/`. Candidate IDs can be printed again
without rebuilding:

```bash
scripts/source-reconcile.py show SR-0001-01234567
```

Keep local decisions from returning to later batches with the ignored scratch
ledger:

```bash
scripts/source-reconcile.py decide retain SR-0001-01234567 --note "current text is intentional"
scripts/source-reconcile.py decide defer SR-0002-89abcdef --note "needs seasonal variants"
make review-sources
```

Use `applied` after a corpus change has been validated, `manual` when a source
lead has become a hand-entry task, and `pending` to remove a prior decision.
Both the extracted excerpts and `decisions.csv` remain under the gitignored
`output/source-reconcile/` tree.

For each candidate, compare the page-aware source extraction with the current
corpus and decide to retain it, replace it from the source, edit it manually,
or defer it. Apply and validate accepted changes in small batches. Record a
`review attest` entry only after a person has actually checked the wording;
the reconciliation script deliberately never marks its own matches verified.

### Annual cadence

The checklist deliberately covers only the current year — the archdiocese
ordo for future years is not yet published, and most compositions recur, so
coverage accumulates: sign-offs are date-independent, and when the manifest
is regenerated each January the recurring units arrive already reviewed.
Only that year's genuinely new variants (different commemoration and octave
patterns) appear as unreviewed, so the annual ask shrinks over time.

Calendar resolution itself (which feast wins each day, transfers,
commemorations) is checked separately by diffing `make ordo YEAR=20XX`
against the published ordo when it arrives — volunteers reviewing texts
against the diurnal and supplement do not need the ordo.
