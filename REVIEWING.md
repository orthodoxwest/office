# Reviewing the Office

This guide is for volunteers checking the rendered hours against the printed
books: the diurnal and the archdiocese supplement. You do not need to know
anything about the code or about git — you need the books, a web browser, and
your assigned rows from the review checklist.

## How review is organized

The unit of review is **one hour of one celebration**, not one calendar date.
The same composition recurs year after year, so checking "Trinity Sunday
Lauds" once covers every year it recurs. Some celebrations have more than one
variant (a year with a commemoration attached, a feast falling inside an
octave) — each variant is its own row in the checklist with its own link.

Each row in the checklist has:

- a **link** — the exact page to open (e.g. `/lauds/2026-06-07`)
- a **priority** — A (Sundays and 1st/2nd class feasts), B (greater doubles
  and doubles), C (everything else). Work top down.
- a **hash** — a short code identifying exactly what you reviewed. When you
  finish a row, report the hash back so coverage can be recorded. If the
  texts later change, your sign-off is automatically marked stale and the
  unit returns to the queue.
- a **context** note — commemorations or octave the page should reflect.

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

If a page is fully correct, that is just as valuable: report the row's hash
back through your coordinator so it can be signed off.

## For the maintainer

```bash
make review-manifest > manifest.csv   # regenerate the checklist (START=2026 YEARS=1)
make review-status                    # coverage report: current / stale / unreviewed
./office review sign HASH REVIEWER [note...]   # record a sign-off
```

Sign-offs live in `data/review/signoffs.txt` and are keyed by content hash:
any edit to the texts behind a signed-off unit orphans the hash and the unit
shows up as **stale** in `review-status` until re-reviewed. Sign-offs are
committed to git like any other data change.

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
