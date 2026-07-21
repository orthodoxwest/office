# AWRV Benedictine Divine Office

Go web application that renders the hours of the Benedictine Office, as used by the AWRV

## Architecture

```
cmd/server/main.go        CLI entry point (ordo, validate, tex, serve subcommands)
internal/
  models/                  Shared types (Feast, CalendarDay, Rank, Color, Season, OfficeHour)
  calendar/                Calendar engine (ported from Python reference at ../calendar/)
    paschalion.go          Julian Easter calculation
    moveable.go            All moveable dates derived from Easter/Christmas
    seasons.go             Season determination
    loader.go              INI-like data file parser
    builder.go             Main pipeline: feasts → dates → resolve → CalendarDay list
    occurrence.go          Conflict resolution (which feast wins each day)
    validate.go            Three-layer data validation
  output/                  Output formatters
    ordo.go                Text ordo formatter
    office.go              Plain-text office hour formatter
    tex.go                 LaTeX booklet formatter (half-letter, lualatex)
  texts/                   Text corpus loader
  push/                    Web Push (RFC 8291) notification support
    push.go                Manager: VAPID config, Subscribe/Unsubscribe, encrypted Send
    config.go              Env-driven VAPID config + subscription store path
    filestore.go           JSON-file subscription Store (single instance + volume)
  office/                  Office composition engine
    engine.go              Engine: loads corpus, dispatches to hour composers
    hourdef.go             Hour definition file parser
    lauds.go               Lauds composer
    vespers.go             Vespers composer
    prime.go               Prime composer
    compline.go            Compline composer
    minor.go               Minor hours composer (Terce, Sext, None)
    proper.go              Proper antiphon resolution
    marian.go              Marian antiphon selection
    preces.go              Preces condition logic
  web/                     HTTP server
    server.go              Server struct, route registration, embedded templates
    handlers.go            Handlers: home, hour, calendar
    cache.go               Per-year CalendarDay + MoveableDates cache
    pwa.go                 PWA support: /sw.js handler + build-version hash (binary + data dir)
    ics.go                 /office.ics reminder feed (stateless, query-param config) + /reminders page
    webpush.go             Web Push endpoints: VAPID public key, subscribe/unsubscribe, test send
    templates/             Embedded HTML templates (layout, home, hour, calendar)
    static/                Embedded CSS, PWA manifest, icons, service worker source (sw.js)
  e2e/                     End-to-end golden-file tests
    golden_test.go         Rendered-hour, ordo, audit, and assurance golden tests
    testdata/golden/       Checked-in output/review snapshots (regenerate with make golden)
  audit/                   Data completeness audit
    audit.go               Placeholder scanner + missing-propers reporter
    sweep.go               Composition sweep: not-found markers + ordinary fallbacks on Double+ days
    lint.go                Text-corpus lints (mechanical fail make check; advisory for triage)
  review/                  Human review coverage tracking
    review.go              Manifest sweep: dedupe identical composed hours into review units
    signoff.go             Sign-off file (data/review/signoffs.txt) + current/stale/unreviewed classification
    provenance.go          Structured per-entry source inventory and attestations
    provenance_queue.go    Dependency-weighted atomic text review ordering (suspect tier first)
    prescreen.go           Durable prescreen-flag ledger + suspicion map (flags ∪ advisory lints)
    assurance.go           Composition explanations and minimal structural-review planning
    assurance_gate.go      Release assurance baseline, gates, and CI summary
tools/
  genicons/                Generates checked-in PWA icon PNGs from the favicon cross design
data/
  feasts/                  Feast definitions (INI-like format)
  texts/                   Liturgical texts
  office/                  Hour structure definitions (one file per hour)
  audit-ok.txt             Feasts that intentionally use ordinary/common texts (suppress audit warnings)
  review/signoffs.txt      Human review sign-offs with internal version binding (see REVIEWING.md)
  review/provenance.csv    Source/page attestations; citations only, never book contents
  review/prescreen.csv     Read-through suspicion flags bound to text versions (see REVIEWING.md)
  review/assurance-baseline.json  Intentional verified/structural coverage floors
  texts/chant/             GABC chant score files (psalms/, canticles/, hymns/)
scripts/
  seed-divinum.go          Seed propers/commons from a local Divinum Officium checkout
                           (go run scripts/seed-divinum.go -do <path> [-feast id] [-write])
  ordo-compare.py          Diff app output against a parish ordo PDF (see /ordo-verify skill)
```

## Reference materials & verification

`../resources/` (sibling of this repo) holds archdiocesan ordo PDFs (2017–2026) and rubrics
documents. **The newest-year ordo is the authority for what this app should produce** — it
reflects current archdiocesan policy, which is revised over time. A feast, rank, or discipline
that held steady across older ordos and then differs in the newest year may be a deliberate
revision, so don't assume a typo just because it changed — but typos happen every year too, so
flag the discrepancy for confirmation rather than silently picking a side. Older ordos stay
useful for cross-checking anything unchanged, and the temporal cycle (paschalion, moveable
dates) is valid in all years. (Computus figures — Golden Number, Dominical Letter, moveable
dates, Ember days — are arithmetic, so a discrepancy there is a genuine error in whichever ordo,
not policy.)
`diurnal-rubrics.pdf` is the normative rubric text (preces §XXXVII, suffrage §XXXVIII). Where
the ordo and the rubrics disagree, file an issue for the priest rather than picking a side.

Use the `/ordo-verify` skill (`.claude/skills/ordo-verify/`) to machine-diff the app against
an ordo PDF: `office rubrics YEAR` emits per-day composition flags, `office ordo YEAR` emits
the Tabula Temporaria (computus figures, moveable feasts, Ember days) plus per-hour stanzas,
and `scripts/ordo-compare.py` diffs headlines, preces/suffrage/commemorations, Ben/Mag antiphon
incipits, colors, Vespers precedence, and moveable dates. Known divergence clusters are tracked
in GitHub issues (#9–#13, #42 need rulings; #15–#17, #20, #40, #41 are engine/data work).

## Text provenance

Texts seeded from Divinum Officium carry a `# SOURCE: divinum-officium <file> [<section>] — check against diurnal` comment inside the section. Grep for `SOURCE: divinum-officium` to find texts awaiting verification against the printed diurnal; delete the comment once verified. Comment lines (`#`) inside INI text sections are stripped by the corpus loader and never render. `# TODO(diurnal):` comments mark refs that DO could not supply at all.

## Git workflow

All changes must go through a pull request — do not push directly to `master`.

1. Create a feature branch: `git checkout -b your-branch-name`
2. Make changes and commit with clear messages
3. Push the branch: `git push -u origin your-branch-name`
4. Open a PR against `master` and ensure CI passes before merging

## Issue labeling

Repo labels `bug`, `needs ruling`, and `data validation` together cover nearly every issue worth filing here. Apply based on where the defect actually lives, not the symptom:

- **`bug`** — the Go code (composers, resolvers, formatters) produces output that contradicts a rubric or spec we already agree on. The fix is a code change. E.g. `concurrenceWinner` picking the wrong feast per XIII.10, Preces firing on the wrong days.
- **`data validation`** — the code is correct but a text/data file is missing, wrong, or a placeholder (missing propers, wrong antiphon corpus, `SOURCE: divinum-officium` text never checked against the diurnal). The fix is editing `data/`, not `internal/`.
- **`needs ruling`** — the ordo/rubrics are ambiguous, contradictory, or silent, and a decision from clergy is required before any fix can be written. Don't guess an implementation here; file the question and wait for a ruling.

These aren't mutually exclusive — an issue can need a ruling *and* turn into a bug/data-validation fix once the ruling lands (see #13, #15). Use the other labels (`enhancement`, `question`, `documentation`, `duplicate`, `invalid`, `wontfix`, `good first issue`, `help wanted`, `update-golden`) only when none of the three above fit.

## Dev commands

```bash
make build       # Build binary
make test        # Run all tests (includes golden)
make vet         # Run go vet
make fmt         # Check formatting
make check       # fmt + vet + lint + test + validate + lint-texts
make serve       # Start web server on :8080
make ordo        # Print text ordo (Tabula Temporaria header + per-hour stanzas) for current year (YEAR=2026)
./office rubrics YEAR  # Per-day TSV of composed rubric flags + Ben/Mag antiphons (for ordo cross-checks)
make validate    # Validate data files
make audit       # Report placeholder texts, missing propers + composition sweep (./office audit -year N)
make lint-texts  # Lint text corpus: mechanical findings fail, advisory printed
make review-manifest  # Print human-review checklist CSV for current year (START=2026 YEARS=1)
make review-status    # Report review coverage vs data/review/signoffs.txt
make review-provenance # Report generated corpus source coverage
make review-provenance-queue # Rank atomic text review by dependency fan-out (suspect tier first)
make review-zero-occurrences # List unrendered corpus entries with classification heuristics
make review-suspects  # Only pre-flagged/lint-flagged texts — the findings-sprint list
make review-plan      # Print minimal structural-review checklist CSV
make review-assurance # Run release assurance gates and summary
./office review explain HOUR DATE # JSON dependencies and rule decisions
./office review attest --source SOURCE --page PAGE KEY REVIEWER # Record verified text
./office review flag --severity high --reason WHY KEY # Record a prescreen suspicion
./office review sign HOUR DATE REVIEWER # Record structural sign-off

Hour pages expose assurance metadata in a collapsed disclosure. Keep it
source-content-free: corpus keys, provenance states, fallback tiers, rule IDs,
and review links are allowed; local paths and inaccessible PDF links are not.
make tex         # Emit .tex booklet (HOUR=lauds DATE=2026-03-11; DATE defaults to today)
make pdf         # Generate PDF via lualatex (HOUR=compline; DATE defaults to today)
make golden      # Regenerate golden test files after intentional changes
make clean       # Remove artifacts
```

## Web Push notifications

`internal/push/` + `internal/web/webpush.go` add opt-in Web Push reminders. Push is
the one stateful corner of an otherwise stateless server (contrast the `office.ics`
feed, whose whole schedule lives in the URL), so subscriptions sit behind a `Store`
interface (default: a JSON file, meant for a single instance on a persistent volume).

Push is **disabled unless VAPID keys are set** — the server logs that it's off and runs
normally, and `/reminders` hides the push UI (the client only reveals it when
`/push/vapid-public-key` returns 200). To enable:

```bash
./office vapid            # generate a VAPID keypair as env-var exports
# set OFFICE_VAPID_PUBLIC_KEY / _PRIVATE_KEY / _SUBJECT in the server env
# (private key is a secret — use `fly secrets set`, not source control)
# OFFICE_PUSH_STORE overrides the subscription file path (default push-subscriptions.json)
```

Endpoints: `GET /push/vapid-public-key`, `POST /push/{subscribe,unsubscribe,test}`. The
service worker's `push`/`notificationclick` handlers (in `static/sw.js`) render the
`push.Payload` JSON and route a tap to the hour page. This change ships subscription
plumbing plus a manual test-send; the always-on scheduler that fires each hour's
reminder on a stored `Schedule` is intentionally a follow-up (the Schedule is already
captured and stored).

## PDF booklet pipeline

`internal/output/tex.go` — `FormatOfficeHourTeX(*OfficeHour, dataDir string) string`

Produces a complete LuaLaTeX document (half-letter 5.5"×8.5") from a composed `OfficeHour`. Mirrors `FormatOfficeHour` in `office.go` but emits LaTeX instead of plain text.

- CLI: `./office tex HOUR [YYYY-MM-DD]` — date defaults to today
- Makefile: `make pdf HOUR=compline` (chains `./office tex` → `lualatex`)
- Font: EB Garamond (`fonts-ebgaramond`). Cross ✠ via Noto Sans Symbols Black (`fonts-noto-extra`).
- gregoriotex: required (TeX Live `gregorio` package) — the preamble loads it unconditionally; it supplies the ℣/℟ glyphs (`\Vbar`/`\Rbar`) even in non-chant booklets.
- GABC chant files: `data/texts/chant/{psalms,canticles,hymns}/{slug}.gabc`. When present, element renders as `\gregorioscore{}` instead of formatted text. Psalm slugs zero-padded to 3 digits (e.g. `psalms/067.gabc`).

## Liturgical specifics

- **Calendar**: Julian paschalion, pre-1962 ranking system
- **Hours**: Lauds, Prime, Terce, Sext, None, Vespers, Compline (no Matins)
- **Psalter**: Coverdale (Book of Common Prayer)
- **English only**
- **AWRV additions**: post-schism Eastern saints (St. Raphael of Brooklyn, St. Innocent of Alaska, St. Tikhon of Moscow, St. Herman of Alaska)

## Data format

INI-like text format for feast/season definitions:

```
[feast-id]
Name = Feast Name
Rank = double-1st-class
Color = white
Category = lord
Month = 12
Day = 25
```

Keys: Name, Rank, Color, Category, DateRule (moveable), Month/Day (fixed), HasOctave, HasVigil, IsVigil with VigilOf, IsApostolicCompanion, Source, Notes.
