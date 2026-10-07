# AWRV Benedictine Divine Office

Rust web application that renders the hours of the Benedictine Office, as used by the AWRV.
Cargo builds the server and CLI.

## Architecture

```
crates/
  data-format/             Corpus/ledger CSV, report JSON, quoting and parsing contracts
  calendar/                Computus, feast loading, occurrence, octaves, fasting; no file access
  corpus/                  Text loading, aliases, sidecars and shared line grammar
  liturgy/                 Document model, element kinds, prayer forms, voice spans and posture cues
  office/                  Hour composition, concurrence, scopes, summaries and tracing
  ordo/                    Text calendar and rubrics TSV
  render-text/             Plain-text office rendering
  render-tex/              LuaLaTeX booklet; caller supplies GABC lookup
  render-html/             HTML rendering, view models and minijinja templates
  render-blocks/           Platform-neutral blocks of styled runs for the native apps; tested
                           word-for-word against render-html
  presentation/            Words every front shares: day, season and date names, the current-hour
                           schedule (app.js mirrors it), home's invitation, the report-issue link,
                           the usage beacon's vocabulary (server, app.js and native apps)
  tools/                   Filesystem access, validation, audit, review, corpus edits and scaffolds
apps/
  cli/                     office command dispatch, dump stream, diff and digest
  office-web/              Axum routes, usage SQLite store, reminders, embedded static/ assets
                           http.rs holds saved-link query/cookie parsing; web_time.rs handles
                           civil dates, time zones and reminder instants
  mobile-ffi/              UniFFI bindings for the native apps; embeds data/ at build time
  android/                 Kotlin/Compose app; Gradle drives cargo-ndk and binding generation
                           (see apps/android/README.md; `make android`, `make android-screenshots`)
  ios/                     SwiftUI app; build-core.sh builds the XCFramework and Swift bindings,
                           XcodeGen the project (see apps/ios/README.md; `make ios`, macOS only)
tests/fixtures/            Rendered-hour, ordo, audit, assurance and 28-year snapshots;
                           broken corpora for validation boundary tests
tools/
  genicons.py              App icon (web, iOS, Android) generator; requires tools/requirements.txt
  genornaments.py          Painted ornament masks (crosses, sun, moon, quatrefoil, tailpiece, the
                           Apse vault tile and the Nave powdering cell) for the web
  genplaster.py            Texture generator; source photograph stays in ../resources/
  genarch.py               Home frontispiece's pointed head: clip-path polygons in style.css, and
                           the arcs' figures for the native apps (Arch.kt, Arch.swift)
data/
  feasts/                  Feast definitions (INI-like format)
  texts/                   Liturgical texts
  office/                  Hour structure definitions (one file per hour)
  audit-ok.txt             Feasts that intentionally use ordinary/common texts (suppress audit warnings)
  latin-incipits.txt       Latin incipit printed beside each psalm/canticle label. Our psalter is
                           Coverdale = HEBREW numbering; incipits come from the Vulgate, which runs
                           one behind — the file header records the join/split mapping. Sits outside
                           data/texts/ (like collect-conclusions.txt) so it stays out of the corpus
                           provenance, zero-occurrence and Latin-lint sweeps.
  review/provenance.csv    Source/page attestations; citations only, never book contents
  review/prescreen.csv     Read-through suspicion flags bound to text versions (see REVIEWING.md)
  review/assurance-baseline.json  Intentional verified-text floor
  texts/chant/             GABC chant score files (psalms/, canticles/, hymns/)
scripts/
  DIURNAL-PIPELINE.md      Supported scanned-diurnal ingestion workflow and attestation semantics
  diurnal-pages.py         Render/index page images; OCR only locates pages
  diurnal-transcribe.py    Read provenance-queue entries from pages and apply agreed wording
  diurnal-discover.py      Find printed propers behind runtime fallbacks
  golden.py               Check/regenerate snapshots using the Rust CLI
  verify-psalms.py         Read-only comparison with the official BCP Psalter
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

## Composition checks

Use the source-requirement checklist in REVIEWING.md. Record the cited rubric,
expected behavior, representative/boundary dates and prayer forms, and linked
tests or open questions. `review plan` provides sample dates from observed
engine behavior; it does not measure correctness or completion. Whole-page
signoffs and their coverage ledger are retired. Keep known incomplete Triduum
work separate from an assessment of the rest of the year.

## Text provenance

Texts seeded from Divinum Officium carry a `# SOURCE: divinum-officium <file> [<section>] — check against diurnal` comment inside the section. Grep for `SOURCE: divinum-officium` to find texts awaiting verification against the printed diurnal; delete the comment once verified. Comment lines (`#`) inside INI text sections are stripped by the corpus loader and never render. `# TODO(diurnal):` comments mark refs that DO could not supply at all.

Every corpus entry must have a current `verified` attestation in `data/review/provenance.csv`; `make validate` fails otherwise (a stale hash counts as unverified). Adding or editing a text means attesting it (`./office review attest`, `--replace` for an edit) in the same PR. The web server does not load provenance.

## Git workflow

All changes must go through a pull request — do not push directly to `master`.

1. Create a feature branch: `git checkout -b your-branch-name`
2. Make changes and commit with clear messages
3. Push the branch: `git push -u origin your-branch-name`
4. Open a PR against `master` and ensure CI passes before merging

### The `update-golden` label

Adding `update-golden` to a PR makes CI merge `master` into the branch, run `make golden`,
verify the merged tree (`cargo test --workspace --locked` + `make validate`), and push the result. Use it both
when an intentional data/logic change moved the golden files and when the PR is blocked on
merge conflicts.

Conflicts are auto-settled **only** inside `tests/fixtures/golden/`. Those files are
whole-corpus rollups — `parity-snapshot.json` digests an entire year per row and
`assurance-report.md` is a table of global counts — so two PRs fixing unrelated corpus entries
always collide there, and the correct merged value is neither side's: it has to be recomputed.
A conflict anywhere else aborts the merge untouched and the job comments with the paths that
need a human. In particular `data/review/provenance.csv` (sorted by key, so it usually
auto-merges) and `data/review/assurance-baseline.json` (the intentional coverage floor) are
never resolved automatically.

## Issue labeling

Repo labels `bug`, `needs ruling`, and `data validation` together cover nearly every issue worth filing here. Apply based on where the defect actually lives, not the symptom:

- **`bug`** — the Rust code (composers, resolvers, formatters) produces output that contradicts a rubric or spec we already agree on. The fix is a code change. E.g. `concurrence_winner` picking the wrong feast per XIII.10, Preces firing on the wrong days.
- **`data validation`** — the code is correct but a text/data file is missing, wrong, or a placeholder (missing propers, wrong antiphon corpus, `SOURCE: divinum-officium` text never checked against the diurnal). The fix is editing `data/`, not the Rust engine.
- **`needs ruling`** — the ordo/rubrics are ambiguous, contradictory, or silent, and only clergy can settle it. File the question, then make the app give the most likely answer and keep the issue open until clergy rule. The newest ordo breaks ties between ordos unless its line is a carried-over template (copied from another year or from the neighbouring days) or it contradicts itself; then the rubric or Diurnal decides. Keep each guess easy to flip: name the issue in the code or data comment beside the rule and in one test, and add a `data/review/ordo-triage.csv` row wherever the guess contradicts an ordo line.

These aren't mutually exclusive — an issue can need a ruling *and* turn into a bug/data-validation fix once the ruling lands (see #13, #15). Use the other labels (`enhancement`, `question`, `documentation`, `duplicate`, `invalid`, `wontfix`, `good first issue`, `help wanted`, `update-golden`) only when none of the three above fit.

## Dev commands

```bash
make build       # Build binary
make test        # Run all tests (includes golden)
make lint        # Run Clippy
make fmt         # Reformat Rust source
make fmt-check   # Check Rust formatting
make check       # Formatting, Clippy, JS lint, tests, data validation and text lint
make serve       # Start web server on :8080
make ordo        # Print text ordo (Tabula Temporaria header + per-hour stanzas) for current year (YEAR=2026)
./office rubrics YEAR  # Per-day TSV of composed rubric flags + Ben/Mag antiphons (for ordo cross-checks)
make validate    # Validate data files (fails on any unverified or stale corpus entry)
make audit       # Report placeholder texts, missing propers + composition sweep (./office audit -year N)
make scaffold-propers  # Ensure proper files exist with commented key catalogs (never overwrites live sections)
make lint-texts  # Lint text corpus: mechanical findings fail, advisory printed
make pages       # Render and index diurnal/supplement pages under ignored output/
make transcribe  # Prepare page-reading prompts; APPLY=1 enables readers and gated application
make discover    # Prepare proper-discovery dossiers; APPLY=1 enables readers and gated application
make review-manifest  # Inventory distinct rendered compositions as CSV for current year (START=2026 YEARS=1)
make review-provenance # Report generated corpus source coverage
make review-provenance-queue # Rank atomic text review by dependency fan-out (suspect tier first)
make review-zero-occurrences # List unrendered corpus entries with classification heuristics
make review-suspects  # Only pre-flagged/lint-flagged texts — the findings-sprint list
make review-plan      # Sample observed engine behavior (default 28y); no completion score
make review-assurance # Check text-provenance floor and print summary
./office review explain HOUR DATE # JSON dependencies and rule decisions
./office review attest --source SOURCE --page PAGE KEY REVIEWER # Record verified text
./office review flag --severity high --reason WHY KEY # Record a prescreen suspicion
make tex         # Emit .tex booklet (HOUR=lauds DATE=2026-03-11; DATE defaults to today)
make pdf         # Generate PDF via lualatex (HOUR=compline; DATE defaults to today)
make mutate      # Mutation-test a Rust crate (MUTATE_PKG=calendar) — see MUTATION-TESTING.md
make mutate-diff # Mutation-test only lines changed vs master (local review)
make test-coverage # Optional local Rust line-coverage diagnostic
make golden      # Regenerate golden test files after intentional changes
make rust-check  # Rust workspace: cargo fmt --check, clippy -D warnings, tests
make test-ux      # Playwright browser suites
make parity      # Check all snapshots, including the full 2026–2053 digest
make android     # Sideloadable Android preview APK (needs Android SDK/NDK, cargo-ndk)
make android-screenshots # Render Android screens on the JVM (Robolectric) for review
make ios         # iOS core (XCFramework + Swift bindings) and Xcode project (macOS, Xcode, XcodeGen)
make clean       # Remove artifacts
```

## PDF booklet pipeline

`crates/render-tex/` — TeX document renderer

Produces a complete LuaLaTeX document (half-letter 5.5"×8.5") from a composed `OfficeHour`. Uses the same composed document model as the plain-text and HTML renderers.

- CLI: `./office tex HOUR [YYYY-MM-DD]` — date defaults to today
- Makefile: `make pdf HOUR=compline` (chains `./office tex` → `lualatex`)
- Font: EB Garamond (`fonts-ebgaramond`). Cross ✠ via Noto Sans Symbols Black (`fonts-noto-extra`).
- Layout: two-sided (inner margin alternates for booklet imposition), running heads via `fancyhdr`, headings and psalm/hymn labels kept with their text via `needspace`, PDF title/subject via `hyperref`. Display text goes through the same `corpus::typography::typeset` as the web (curly quotes, en-dash verse ranges).
- Initials: `\initial` in the preamble mirrors the web's initial system (small-caps opening word; dropped, raised or elevated by the lines the opening takes; per-letter optical profiles from `style.css`, eased for print on a few letters). Recalibrate against a specimen of every letter when changing the initial face.
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

Keys: Name, Rank, Color, Category, DateRule (moveable), Month/Day (fixed), HasOctave, OctaveClass (Rubrics VII.3 groups; inherited by generated days), OctaveDays/OctaveDay2–8 (ordo names for generated octave days), PrimaryOfOurLord (Diurnal rank table; §X Double exception), Secondary (Diurnal rank table; X.1(c) primary before secondary), CommemorationClass (named XIV.14 ferial exceptions), HasVigil, IsVigil with VigilOf, OctaveOf (octave continued by a day whose ID does not name it), CompanionOf, Source, Notes.
