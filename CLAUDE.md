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
    templates/             Embedded HTML templates (layout, home, hour, calendar)
    static/                Embedded CSS, PWA manifest, icons, service worker source (sw.js)
  e2e/                     End-to-end golden-file tests
    golden_test.go         TestHourGolden + TestOrdoGolden
    testdata/golden/       Checked-in golden files (regenerate with make golden)
  audit/                   Data completeness audit
    audit.go               Placeholder scanner + missing-propers reporter
    sweep.go               Composition sweep: not-found markers + ordinary fallbacks on Double+ days
    lint.go                Text-corpus lints (mechanical fail make check; advisory for triage)
  review/                  Human review coverage tracking
    review.go              Manifest sweep: dedupe composed hours by content hash into review units
    signoff.go             Sign-off file (data/review/signoffs.txt) + current/stale/unreviewed classification
tools/
  genicons/                Generates checked-in PWA icon PNGs from the favicon cross design
data/
  feasts/                  Feast definitions (INI-like format)
  seasons.txt              Season definitions
  texts/                   Liturgical texts
  office/                  Hour structure definitions (one file per hour)
  audit-ok.txt             Feasts that intentionally use ordinary/common texts (suppress audit warnings)
  review/signoffs.txt      Human review sign-offs (hash-keyed; see REVIEWING.md)
  texts/chant/             GABC chant score files (psalms/, canticles/, hymns/)
scripts/
  seed-divinum.go          Seed propers/commons from a local Divinum Officium checkout
                           (go run scripts/seed-divinum.go -do <path> [-feast id] [-write])
```

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
make ordo        # Print text ordo for current year (YEAR=2026)
make validate    # Validate data files
make audit       # Report placeholder texts, missing propers + composition sweep (./office audit -year N)
make lint-texts  # Lint text corpus: mechanical findings fail, advisory printed
make review-manifest  # Print human-review checklist CSV for current year (START=2026 YEARS=1)
make review-status    # Report review coverage vs data/review/signoffs.txt
make tex         # Emit .tex booklet (HOUR=lauds DATE=2026-03-11; DATE defaults to today)
make pdf         # Generate PDF via lualatex (HOUR=compline; DATE defaults to today)
make golden      # Regenerate golden test files after intentional changes
make clean       # Remove artifacts
```

## PDF booklet pipeline

`internal/output/tex.go` — `FormatOfficeHourTeX(*OfficeHour, dataDir string) string`

Produces a complete LuaLaTeX document (half-letter 5.5"×8.5") from a composed `OfficeHour`. Mirrors `FormatOfficeHour` in `office.go` but emits LaTeX instead of plain text.

- CLI: `./office tex HOUR [YYYY-MM-DD]` — date defaults to today
- Makefile: `make pdf HOUR=compline` (chains `./office tex` → `lualatex`)
- Font: EB Garamond (`fonts-ebgaramond`). Cross ✠ via Noto Sans Symbols Black (`fonts-noto-extra`).
- gregoriotex: loaded via `\IfFileExists` — skipped silently if not installed.
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

Keys: Name, Rank, Color, Category, DateRule (moveable), Month/Day (fixed), HasOctave, HasVigil, Source, Notes.
