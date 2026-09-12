# Reviewing the Office

This guide covers checking corpus wording against the printed books and
checking composed hours against the Diurnal, supplements, rubrics, and current
archdiocesan ordo. Findings need a source citation and a reproducible example.

## Composition review

Start from a requirement in the sources, then inspect what the app renders.
A correct page does not certify a rule on other dates. Engine-generated feature
counts and sample-page counts are not a measure of liturgical completeness.
The former structural signoff workflow and its coverage score are retired.

Keep a short checklist in the relevant audit issue or PR. For each check, record:

- **Requirement and source:** the appointment or rubric being checked, with
  edition and page/section. Paraphrase the requirement; keep scans and extracted
  book contents outside Git, and generated evidence under ignored `output/`.
- **Cases and expected behavior:** civil date, hour, prayer form, calendar scope,
  expected selections/omissions/order, and relevant boundary or collision cases.
- **Result:** observed behavior and a link to the regression test, confirmed
  defect, or unresolved source/clergy question. Name the reviewer, including
  when the review was performed by an agent.

These are useful starting areas, not a completed or exhaustive rubric inventory:

| Area | Examples to check |
|---|---|
| Psalmody | Weekday, Sunday, and festal appointments at the affected hours |
| Seasonal changes | Preces, suffrages, Marian antiphons, doxologies, and boundary dates |
| Text selection | Proper/common/seasonal/ordinary appointments, hour-specific limits, redirects |
| Prayer forms | Private, deacon, and priest openings, greetings, confession, and endings |
| Calendar interactions | Occurrence, transfers, I/II Vespers, commemorations, octaves, and vigils |

Keep the known incomplete Triduum composition work separate from an assessment
of the rest of the year. A narrow passing check does not close an entire area.
Turn confirmed defects into repairs with direct appointment or boundary tests.
Regression snapshots detect changes; they do not independently establish that
an existing appointment is correct.

Use the newest local archdiocesan ordo for current practice. If it conflicts
with the Diurnal, normative rubrics, or older ordos, document the conflict for
clergy and leave the affected entries unapplied. A difference from the ordo
may be an app defect, a printed error, or an unresolved source conflict.

### Optional sample dates

`review plan` selects representative pages from decisions and source tiers
already observed in the engine, weighted by their frequency in the sweep.
It helps find examples to inspect. It cannot discover an unmodeled rubric or
prove that other appointments and combinations of rules are correct.

The CSV includes the page URL, priority, context, `primary_year`,
`sampled_features`, and `feature_exposure`. Features listed on a row are those
not represented by earlier samples. Exposure sums their date-hour-form
occurrences; the same composition can contribute to several features, so it
is not a count of distinct affected offices or review progress. Primary-year
examples are preferred; later dates provide examples absent from that year,
and cannot be verified against an unpublished future ordo.

The default sample uses selected engine decisions and proper-slot resolution
tiers. Descriptive context and pure weekday psalmody gates are excluded, so
check weekday appointments explicitly when a source requirement calls for them.
`-include-sources` also samples corpus keys; it does not attest their wording.
Samples never shrink because a page was reviewed. Record the specific checked
requirement in tests and issues rather than signing off a whole page.

### Reporting a problem

Every hour page has a **"Report a problem"** link. Describe what the cited
source requires and what the app shows, with the date, hour, and prayer form.
Distinguish incorrect wording from incorrect selection or assembly. A proper
may exist in the corpus yet be selected for the wrong day or hour; a generic
fallback may hide a missing proper.

## Text provenance

Text provenance records verification of an individual corpus entry against a
named source and page/section locator. It does not certify where that text is
appointed. Compare wording against the printed page, following the
[page-image workflow](scripts/DIURNAL-PIPELINE.md) for scanned sources.
The provenance queue ranks entries by usage and suspicion; a wording check
need not be repeated for every date on which the entry appears.

## For the maintainer

Keep generated inventories beneath ignored `output/`:

```bash
mkdir -p output/review
make -s review-manifest > output/review/manifest.csv   # distinct rendered compositions
make review-provenance                # source verification, flat and usage-weighted
make -s review-provenance-queue > output/review/provenance-queue.csv
make -s review-zero-occurrences START=2026 YEARS=30 > output/review/zero-occurrences.csv
make -s review-suspects > output/review/suspects.csv
make -s review-plan > output/review/samples.csv       # optional examples, default 28 years
./office review plan -start 2026 -years 1 -summary
make review-assurance                 # text-provenance floor and summary
./office review explain lauds 2026-06-07 --form private
```

`review sign`, `review status`, and `make review-status` are retired. The
historical ledger is available in Git history. Corpus `review attest`, the
composition explanation, resolution inventory, and date-sensitive parity
snapshot remain available.

Explicit text attestations live in `data/review/provenance.csv`. The file
records only citations, review metadata, the corpus key, and internal version
metadata; it does not copy or embed source-book contents. If an entry later
changes, its attestation automatically becomes stale without requiring
reviewers to handle version identifiers.

Some corpus sections carry
`# SOURCE: … — agent-proposed, not attested`. Those came through
the retired packet-apply pipeline from a gated diurnal witness. Treat them as
current wording awaiting a human check, not as a finished verification. A later
`review attest` is what flips the key to `verified`. Do not delete the hedge
until that attestation lands.

Prefer the safe CLI to manual CSV editing:

```bash
./office review attest --source "Printed Diurnal" --page 123 \
  --locator "Proper of Example" --note "word-for-word" \
  proper/example/collect reviewer
```

The command resolves the corpus key to its current content, validates every
field, and rewrites the ledger atomically; use `--replace` only when
deliberately superseding an existing attestation.

### Prescreen flags: sending book time where findings are likely

The provenance queue ranks texts by exposure (how many pages a verification
would cover). Exposure alone sends volunteers to hundreds of probably-fine
texts first, so a second signal feeds the queue: **suspicion**. Entries with
any suspicion flag form the queue's top tier, and `make review-suspects`
prints only that tier — a short list where a book check is likely to yield a
finding rather than a quick confirm. Suspicion comes from two places:

1. **The prescreen ledger** (`data/review/prescreen.csv`) — durable
   read-through findings ("this collect ends mid-sentence") recorded once and
   tracked until resolved. Record one with:

   ```bash
   ./office review flag --severity high \
     --reason "collect ends mid-sentence; no termination formula" \
     proper/example/collect
   ```

   The command binds the flag to the entry's current content version. A flag
   resolves when the text is **attested** (verified word for word):
   `review attest` prunes the flag's row from the ledger, and its history
   stays in git. If the text is merely **edited** after flagging, the flag
   shows as `(addressed)` — the fix still needs its book check — until an
   attestation lands. The ledger records suspicions only, never source-book
   contents.

2. **Advisory corpus lints** (`./office lint`) — mechanical heuristics
   (truncated text, unpointed antiphons, near-duplicate pairs, leftover
   Latin) recomputed from the corpus on every run, so they clear themselves
   when the text is fixed.

Both kinds appear in the queue's `flags` column and in each hour page's
Assurance disclosure, so a reviewer on any page is pointed at the exact
element most likely to be wrong. Record read-through findings directly in
the ledger, keyed by corpus entry — narrative prescreen write-ups rot as
items get fixed, with no way to tell which findings still apply.

Entries not selected anywhere in a sweep are classification work, not text
attestation work. `review zero-occurrences` groups those entries by mechanical
key-shape heuristics and joins the durable judgments in
`data/review/zero-occurrences.csv`. The ledger dispositions are
`shadowed-fallback`, `displaced`, `dormant-policy`, `suppressed`, `dead`,
`defect`, and `unclassified`; a defect row must cite its issue. Rows are bound
to the entry's content hash, so an edit makes the judgment stale. The standard
provenance queue labels these rows `classified-zero` or
`zero-needs-classification` and keeps current classified zeroes in a separate
final tier. The assurance summary counts pending provenance only for rendered
entries and reports classified and unclassified zeroes separately.

### Usage-weighted provenance: a status-update metric

The headline count in `make review-provenance` weights every corpus entry
equally — a once-a-year collect and a daily-recited psalm each count as one
entry. That undersells practical coverage: common texts (ordinaries,
psalter, frequently-used propers) tend to get verified first, so the entries
still outstanding skew toward rarely-rendered ones.

To give a second, more honest number, `review provenance` also composes
every hour of every day across a sweep (default: the current year; override
with `START`/`YEARS`) and weights each rendered corpus entry by how many
times it was actually prayed. It prints both the flat percentage among
*rendered* entries and the usage-weighted percentage — verified renders over
total renders — which answers "how much of what we actually pray each year
is verified" and is typically well above the flat corpus-wide count.

### Release assurance

`./office review assurance` reports text provenance and fails if the verified
text count falls below the configured floor. It reports stale attestations
separately. Structural feature counts and page signoffs do not participate.
The report inventories dependencies from every composed date-hour form in the
configured sweep, without using the optional sample plan.

The floor lives in `data/review/assurance-baseline.json`; a zero floor enforces
no minimum verified count. Update it only as an intentional, reviewable change:

```bash
./office review assurance --update-baseline
```

Calendar, rendered content, selected sources, and decision traces remain
protected by the date-sensitive parity snapshot described below.

Each web hour also has a collapsed **Assurance** disclosure. It shows the same
dependency states, fallback tiers, and stable rule identifiers without
revealing local paths or source contents. Unverified dependency rows link to a
prefilled review issue for that exact corpus key. `needs-review` means a source
lead or explicit review task exists but still needs verification;
`source-unknown` means provenance research must happen first.

### Date-sensitive parity and commemoration dedupe

`make parity` verifies the checked-in 2026–2053 snapshot of calendar state,
rendered hour content, selected source references, and decision traces. Its
`commemoration_merges` array remains deliberately readable rather than hidden
inside a digest because fuzzy name containment deserves direct review.

The complete inventory was human-reviewed on 2026-07-17:

- 957 suppressions resolve to 69 unique kept/dropped pairs;
- 61 pairs are duplicate records or title variants of the same observance;
- 8 pairs are parent-feast/octave aliases on collision dates;
- every dropped item resolves to the corresponding item retained on the final
  occurrence or Vespers surface; and
- no distinct observances, unmatched items, or ambiguous pairs were found.

This evidence does not justify replacing the current dedupe with a typed
commemoration pipeline or a new canonical-identity layer. If the readable
inventory changes, review each new or altered pair before accepting the golden
update; a false merge should be corrected narrowly and its behavior pinned in
a dedicated test.

### Ingesting scanned diurnal pages

Use the [page-image workflow](scripts/DIURNAL-PIPELINE.md) to transcribe entries
from the provenance queue or discover printed propers hidden behind fallbacks:

```bash
make pages
make transcribe KEYS=proper/st-athanasius/collect
make discover FEASTS=st-stephen-hungary
```

Transcription and discovery prepare prompts without invoking readers or
changing the corpus by default. After inspecting the prompts and cache, add
`APPLY=1` to enable readers and application through `office corpus put` and
`office review attest`. The pipeline documentation describes the comparison
thresholds, independent second readings, and meaning of a Codex attestation.
Unresolved rows remain `needs-human`; OCR only locates pages.

The former intake, source-reconciliation, agent scheduler, and packet-apply
tools have been retired. Existing local artifacts and decisions under ignored
`output/` remain historical evidence, and existing provenance records retain
their meaning.

### Annual cadence

When a new archdiocesan ordo arrives, compare that year's appointments and
revisit affected source requirements and unresolved questions. Use the ordo
verification skill for a reproducible comparison and discrepancy triage.
Existing regression tests preserve checked examples; they do not automatically
certify appointments in a new ordo. Multi-year samples can help expose calendar
interactions for examination against the applicable rubrics.

### Clergy-facing project status

Generate the repeatable high-level report with:

```bash
make project-status YEAR=2026
```

This rebuilds the app, extracts `../resources/2026-ordo.pdf`, composes the
annual ordo and rubrics, runs the proper and provenance audits, and optionally
queries GitHub for open `needs ruling` issues. It writes three ignored working
artifacts under `output/status/`: a Markdown report suitable for forwarding,
a JSON snapshot for automation, and `ordo-findings-2026.csv` as the complete
date/aspect discrepancy queue. If GitHub is unavailable, every local metric is
still generated and the ruling count is marked unavailable; use
`scripts/project-status.py --year 2026 --offline` to request that behavior.

Ordo cause classifications are deliberately not inferred from symptoms.
Durable rules live in `data/review/ordo-triage.csv` with these categories:
`translation-mismatch`, `data-gap`, `engine-bug`, `open-question`,
`suspected-reference-error`, and `reference-error`. The `year`, `aspect`, and
`date` fields accept shell-style wildcards, while a more-specific rule wins
over a broad one. Use wildcard rules only for a genuinely uniform cluster;
after diagnosing a finding, prefer an exact row with a GitHub issue number
and a short reason. Anything
not covered by the ledger remains visibly `untriaged` and is never silently
guessed. In particular, a mismatched canticle-antiphon incipit remains
untriaged until review distinguishes a translation difference from selection
of the wrong antiphon.

Use `suspected-reference-error` with `provisional` confidence for a proposed
printed error. Cite the source conflict and relevant discussion, and keep it
in the strict difference count. Adjudicated parity credits only
`reference-error` findings with `confirmed` confidence. An issue comment
posted by an agent on someone's behalf does not by itself establish personal
clergy endorsement. A confirmed `open-question` row confirms the existence
of the question, not an answer to it. Keep exceptions and unanswered aspects
as separate rows; a Vespers label decision does not settle an antiphon on the
same date.

The report also derives non-adjudicative diagnostic clusters from the current
findings: Vespers share and aspect totals, ownership and commemoration
direction, co-occurring symptoms, repeated generated incipits, multi-word
reference incipits that occur later in the generated text, monthly hotspots,
and same-date/aspect recurrence in the immediately preceding local ordo when
that PDF is available. These clusters are written into the JSON snapshot as
well as the Markdown report. They help order the queue but never assign a
cause or turn a wording/boundary difference into an agreement.

The report keeps several percentages separate:

- **known proper-slot coverage** counts the six feast-proper lookup slots
  audited by `office audit`; accepted common/seasonal fallbacks count as
  covered, and acknowledged exclusions in `data/audit-ok.txt` leave the
  denominator;
- **rendered completeness** comes from composing every hour for the year;
- **text source verification** is the explicit attestation rate from the
  provenance inventory;
- **strict ordo parity** gives equal weight to each comparable date/aspect
  assertion, with exact commemoration sets counted once per office and date.

This makes the headline stable without conflating “the page renders,” “we
have no known proper gap,” “the wording was checked against a book,” and “the
calendar matches the annual ordo.”

### Prayer forms

Review links include `?form=private|deacon|priest` so the selected prayers and
source metadata remain reproducible regardless of a device's saved preference.
Use the matching `--form` option for `office review explain`, an hour command,
or `office tex`. Omitting it selects Private.

The manifest, provenance queue, and sample plan sweep every distinct
composition across the three forms. Identical Deacon and Priest compositions
share one inventory unit; the parity snapshot checks all three explicitly.
Dynamic proper-resolution inventory remains a calendar sweep: prayer forms
only replace marked ordinary slots, after proper resolution.

The ordinary greetings and choir confession use the Monastic Diurnal's
Prime (pp. 7–9), closing versicles (p. 43), and Compline (pp. 147–148).
The All Souls Compline rubric (p. 643) retains Confession and Absolution while
omitting the opening blessing, lesson, Our help, and Lord's Prayer.
The priest-led form uses the longer choir confession and its exchanges, with
“grant you … your sins” in the final absolution from the parish Compline draft
(Compline Alex edit 10-24 v.08.docx, p. 5). The draft differs here from the
older Diurnal's “grant us … our sins.” Its opening rubric also reserves “Sir,
ask a blessing” for a priest; private and deacon-led forms use “Lord, grant a
blessing.”

The current application scope assigns the shared confession and “our sins”
prayers to everyone together when led by a deacon. This mapping follows the
feature's priest-only confession scope; the printed Out of Choir rubric does
not explicitly name deacons. Speaker metadata labels every turn, including
Amens, without changing the underlying source wording.

The greeting audit checks both clergy and private formulas. “O Lord, hear my
prayer” also occurs as a fixed preces response before the variable greeting:
at Prime (Diurnal p. 9), Compline, and in the Office of the Dead. Those
occurrences remain in every form. In private prayer, the substituted greeting
is omitted when the preceding prayer already ends with that same pair
(diurnal-tutorial.pdf, p. 8, note 21). This covers Prime and Compline with
preces, and Lauds, Vespers, and Compline of the Dead. Later greetings remain;
Psalm and antiphon occurrences are independent of the leader. The older combined collect-intro entries are no
longer used by the hour definitions.
