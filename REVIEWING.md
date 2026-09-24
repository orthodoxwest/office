# Reviewing the Office

Two kinds of review: checking corpus **wording** against the printed books, and
checking **composition** (which texts appear where) against the Diurnal,
supplements, rubrics, and current archdiocesan ordo. Every finding needs a
source citation and a reproducible example.

The newest local ordo governs current practice. If it conflicts with the
Diurnal, the normative rubrics, or older ordos, document the conflict for
clergy and leave the affected entries alone. A difference from the ordo may be
an app defect, a printed error, or an unresolved source conflict.

## Composition review

Start from a requirement in the sources, then check what the app renders. A
correct page says nothing about other dates, and engine feature or sample
counts are not a measure of completeness.

Keep a short checklist in the relevant issue or PR. For each check record:

- **Requirement and source:** the rubric or appointment, with edition and
  page/section. Paraphrase; keep scans and extracted text out of Git and
  generated evidence under ignored `output/`.
- **Cases:** civil date, hour, prayer form, calendar scope, and the expected
  selections, omissions, and order, including boundary and collision cases.
- **Result:** observed behavior and a link to the regression test, defect, or
  open source/clergy question. Name the reviewer, including agents.

Starting areas (not an exhaustive inventory):

| Area | Examples to check |
|---|---|
| Psalmody | Weekday, Sunday, and festal appointments at the affected hours |
| Seasonal changes | Preces, suffrages, Marian antiphons, doxologies, and boundary dates |
| Text selection | Proper/common/seasonal/ordinary appointments, hour-specific limits, redirects |
| Prayer forms | Private, deacon, and priest openings, greetings, confession, and endings |
| Calendar interactions | Occurrence, transfers, I/II Vespers, commemorations, octaves, and vigils |

Keep the known-incomplete Triduum work separate from the rest of the year. Turn
confirmed defects into repairs with direct appointment or boundary tests.
Golden snapshots detect change; they don't show that an existing appointment
is correct.

### Executable composition requirements

`data/review/composition-requirements.json` holds source-backed appointment
cases; `TestCompositionRequirements` checks the composed slot, source, optional
wording, omission, and relative order. Cases name a commemoration's owner
explicitly so a correct principal-office text can't hide a wrong
commemoration. Add a citation plus representative and boundary cases with each
repair, and never regenerate expectations from current output.

```bash
go test ./internal/e2e -run 'TestCompositionRequirements|TestSundayCommemoration|TestCalendarCompositionRequirements'
```

The calendar tests compose all seven hours on every date of 2026, 2027, and
2032, including unnamed ferias; all run in `make check`. See also the
[composition audit](data/review/composition-audit.md) (requirements checked so
far and remaining passes) and [seasonal appointment scopes](data/APPOINTMENT-SCOPES.md).

The ingestion resolution inventory deliberately excludes unnamed owners (an
unnamed feria is never a feast-proper target), so a composition audit must
compose those hours directly or use `review explain`.

### Optional sample dates

`review plan` picks representative pages from engine decisions and source
tiers already observed in a sweep, weighted by frequency. It finds examples to
inspect; it cannot find an unmodeled rubric. Its CSV gives the page URL,
priority, context, `primary_year`, `sampled_features` (features not covered by
earlier rows), and `feature_exposure` (their summed date-hour-form occurrences,
which overlap and are not a progress measure). Later-year rows cover cases
absent from the primary year and can't be checked against an unpublished ordo.

Weekday psalmody gates are not sampled, so check those explicitly.
`-include-sources` also samples corpus keys without attesting them. Samples
never shrink as pages are reviewed; record the checked requirement in tests
and issues rather than signing off a page.

### Reporting a problem

Every hour page has a **Report a problem** link. Give the date, hour, prayer
form, what the source requires, and what the app shows. Distinguish wrong
wording from wrong selection: a proper can exist but be chosen on the wrong
day, and a generic fallback can hide a missing proper.

## Text provenance

A text attestation records that one corpus entry matches a named source and
page; it says nothing about where the entry is appointed. For scanned sources
use the [page-image workflow](scripts/DIURNAL-PIPELINE.md). One check covers
every date the entry appears on.

```bash
mkdir -p output/review
make -s review-manifest > output/review/manifest.csv   # distinct rendered compositions
make review-provenance                                 # source coverage, flat and usage-weighted
make -s review-provenance-queue > output/review/provenance-queue.csv
make -s review-zero-occurrences START=2026 YEARS=30 > output/review/zero-occurrences.csv
make -s review-suspects > output/review/suspects.csv
make -s review-plan > output/review/samples.csv       # optional examples, default 28 years
make review-assurance                                  # text-provenance floor and summary
./office review explain lauds 2026-06-07 --form private
```

Attestations live in `data/review/provenance.csv`: citations, reviewer, corpus
key, and a content hash, never book text. Editing an entry makes its
attestation stale automatically. Record one with:

```bash
./office review attest --source "Printed Diurnal" --page 123 \
  --locator "Proper of Example" --note "word-for-word" \
  proper/example/collect reviewer
```

The command validates every field and rewrites the ledger atomically; pass
`--replace` only to supersede an existing attestation deliberately.

Sections marked `# SOURCE: … — agent-proposed, not attested` came from a
retired pipeline. Treat them as awaiting a human check and keep the hedge
until `review attest` marks the key `verified`.

### Prescreen flags

The provenance queue ranks by exposure (pages covered per check), which alone
would send reviewers to hundreds of probably-fine texts. **Suspicion** forms a
top tier, and `make review-suspects` prints only that tier. It comes from:

1. **The prescreen ledger** (`data/review/prescreen.csv`): read-through
   findings bound to the entry's current content.

   ```bash
   ./office review flag --severity high \
     --reason "collect ends mid-sentence; no termination formula" \
     proper/example/collect
   ```

   Attesting the entry removes the flag. Editing it only marks the flag
   `(addressed)` until the fix is attested.

2. **Advisory lints** (`./office lint`): truncation, unpointed antiphons,
   near-duplicates, leftover Latin; recomputed each run.

Both show in the queue's `flags` column and on each hour's Assurance
disclosure. Record findings in the ledger, keyed by entry, not in narrative
write-ups that go stale.

### Zero-occurrence entries

Entries never rendered in a sweep need classification, not attestation.
`review zero-occurrences` groups them by key shape and joins the judgments in
`data/review/zero-occurrences.csv`: `shadowed-fallback`, `displaced`,
`dormant-policy`, `suppressed`, `dead`, `defect` (must cite its issue), or
`unclassified`. Rows are hash-bound, so an edit makes them stale. The
provenance queue puts classified zeroes in a final tier, and the assurance
summary counts them apart from rendered entries.

### Coverage figures

`make review-provenance` prints two figures. The flat count weights every
rendered entry equally. The usage-weighted count composes every hour in the
sweep (default: current year; override with `START`/`YEARS`) and reports
verified renders over total renders — how much of what is actually prayed is
verified. Common texts are verified first, so it runs well above the flat
figure.

`./office review assurance` fails if the verified-text count drops below the
floor in `data/review/assurance-baseline.json` (zero enforces nothing), and
reports stale attestations separately. Raise the floor deliberately with
`./office review assurance --update-baseline`.

Each hour's collapsed **Assurance** disclosure shows the same dependency
states, fallback tiers, and rule IDs without local paths or source text, and
links unverified rows to a prefilled issue. `needs-review` means a source lead
exists; `source-unknown` means provenance research comes first.

## Date-sensitive parity

`make parity` checks the 2026–2053 snapshot of calendar state, rendered
content, selected sources, and decision traces. Its `commemoration_merges`
array stays readable rather than digested, because fuzzy name matching
deserves direct review. The full inventory (69 kept/dropped pairs: duplicates,
title variants, and octave aliases) was human-reviewed on 2026-07-17 with no
false merges. When it changes, review each new pair before accepting the
golden update, and fix any false merge narrowly with a dedicated test.

## Ingesting scanned diurnal pages

See [scripts/DIURNAL-PIPELINE.md](scripts/DIURNAL-PIPELINE.md):

```bash
make pages
make transcribe KEYS=proper/st-athanasius/collect
make discover FEASTS=st-stephen-hungary
```

Both prepare prompts only; add `APPLY=1` after inspecting them. Unresolved rows
stay `needs-human`.

## Annual cadence

When a new ordo arrives, compare the year with the `/ordo-verify` skill and
revisit affected requirements and open questions. Existing regression tests
don't certify a new ordo.

## Project status

```bash
make project-status YEAR=2026
```

Rebuilds the app, extracts `../resources/2026-ordo.pdf`, composes the ordo and
rubrics, runs the proper and provenance audits, and (optionally) counts open
`needs ruling` issues. It writes to ignored `output/status/`: a forwardable
Markdown report, a JSON snapshot, and `ordo-findings-2026.csv` listing every
date/aspect discrepancy. Offline, or in a worktree, run
`scripts/project-status.py --year 2026 --resources /path/to/resources --offline`.

The report keeps these figures separate:

- **known proper-slot coverage:** the six feast-proper slots audited by
  `office audit`; accepted fallbacks count as covered and `data/audit-ok.txt`
  exclusions leave the denominator;
- **rendered completeness:** composing every hour of the year;
- **text source verification:** the attestation rate;
- **strict ordo parity:** each comparable date/aspect weighted equally, with a
  commemoration set counted once per office and date.

It also derives diagnostic clusters (Vespers shares, commemoration direction,
co-occurring symptoms, repeated incipits, monthly hotspots, recurrence in the
previous year's ordo) to help order the queue. They never assign a cause.

### Ordo triage

Causes are never inferred from symptoms. `data/review/ordo-triage.csv` assigns
`translation-mismatch`, `data-gap`, `engine-bug`, `open-question`,
`suspected-reference-error`, or `reference-error`. `year`, `aspect`, and `date`
accept shell wildcards, and the more specific rule wins; prefer an exact row
with an issue number once a finding is diagnosed. Anything unmatched stays
`untriaged`. A mismatched canticle-antiphon incipit stays untriaged until
review separates a translation difference from a wrong antiphon.

A proposed printed error is `suspected-reference-error` with `provisional`
confidence and still counts as a difference; only `reference-error` with
`confirmed` confidence is credited. An agent's comment on someone's behalf is
not clergy endorsement. A confirmed `open-question` confirms the question, not
an answer. Use separate rows per aspect: a Vespers label ruling doesn't settle
an antiphon on the same date.

### Repair backlog

`data/review/repair-backlog.csv` is a curated list of concrete open problems,
included in the project-status report. It is not a defect count or a
completeness score. Add a row once a specific discrepancy or source establishes
work: the problem, dated examples (hour, form, owner, boundary), expected
behavior, citation, linked findings or issue, and the next action or blocker —
distinguishing a confirmed repair from diagnosis or a clergy question. Use
`scope=triduum` for Triduum rows and `ordinary-year` otherwise.

Rows don't close when a finding disappears; remove a row in the PR that fixes
it, keeping its regression tests. Speculative fallback candidates stay in the
resolution inventory and discovery reports until a concrete problem is found.

## Prayer forms

Review links carry `?form=private|deacon|priest` so they reproduce regardless
of a device's saved choice; CLI commands take `--form` (default private). The
manifest, provenance queue, and sample plan sweep all three forms, sharing one
unit where Deacon and Priest compose identically; the parity snapshot checks
all three. Forms only replace marked ordinary slots after proper resolution.

Sources:

- Greetings and choir confession: Monastic Diurnal Prime (pp. 7–9), closing
  versicles (p. 43), and Compline (pp. 147–148). All Souls Compline (p. 643)
  keeps Confession and Absolution but omits the opening blessing, lesson, Our
  help, and Lord's Prayer.
- The priest-led form uses the longer choir confession, with "grant you … your
  sins" in the absolution from the parish Compline draft (Compline Alex edit
  10-24 v.08.docx, p. 5), where the older Diurnal has "grant us … our sins".
  That draft reserves "Sir, ask a blessing" for a priest; private and
  deacon-led forms use "Lord, grant a blessing."
- Deacon-led prayer assigns the shared confession and "our sins" prayers to
  everyone, following the feature's priest-only confession scope; the printed
  Out of Choir rubric doesn't name deacons. Speaker labels mark every turn,
  including Amens, without changing wording.
- "O Lord, hear my prayer" also occurs as a fixed preces response at Prime
  (p. 9), Compline, and the Office of the Dead, and stays in every form. In
  private prayer the substituted greeting is omitted when the preceding prayer
  already ends with that pair (diurnal-tutorial.pdf, p. 8, note 21): Prime and
  Compline with preces, and Lauds, Vespers, and Compline of the Dead.
