# Rust port

The Office's calendar and composition engine is moving from Go to Rust. The
port is test-driven: the Go engine stays the reference until the Rust engine
reproduces its output for every date, hour, and prayer form in the sweep, and
only then is Go removed.

**Status:** Phases 0–3 are complete. `office-rs dump` is byte-identical to Go:
the corpus, calendar, and office groups for 1900–2199, and every hour in
every prayer form for 2026–2053 (the parity snapshot). The Rust validators
reproduce Go's reports on every broken test corpus, and the text renderer
reproduces every hour golden. Phase 4 is under way: the ordo, the rubrics
TSV, and the TeX booklet are byte-identical. Nothing user-facing changes until
the cutover in Phase 6.

## Why Rust

The engine will serve more than the Office web app:

- **Matins**, as an extension of the Office.
- **The vicariate calendar**: a JSON feed of the observance of the day,
  commemorations, fasting, and links to the hours, embedded in a site we don't
  host.
- **Daily Mass propers** for clergy: a separate frontend over the same
  calendar and shared texts.
- **Automated ordo PDFs**, built from the calendar, the Office, and the Mass.
- **Native mobile apps** for the Office in Swift and Kotlin.

The native apps decide the language. Rust is the standard way to share one core
between Swift and Kotlin (UniFFI), and it serves the server, the CLI, and the
JSON feed natively.

The second reason is modeling. Rust enums with exhaustive matching turn an
unhandled case into a compile error. In the Go engine today, the calendar,
office, models, and texts packages have 33 `default:` branches across 55
`switch` statements and 90 comparisons against string literals, 22 of them
hard-coded feast IDs. `Feast` carries 6 boolean flags whose combinations
nothing constrains, and the calendar day holds its celebration and Vespers
owner as nullable pointers. Each is a decision the type system can't see. Making them explicit makes missed cases visible and
turns them into questions for a ruling.

Types don't make rubrics right. The oracle below, the ordo cross-checks, and
clergy review still carry correctness.

**Unchanged by the port:** the `data/` format and content, the diurnal
ingestion and review scripts, the Playwright UX suite, and the browser assets
(CSS, `app.js`, `sw.js`).

## Target structure

One repository and one Cargo workspace. During the port the workspace sits
beside the Go code at the repository root, so both implementations are
compared at the same commit against the same `data/`.

```
crates/
  calendar    computus, moveable dates, seasons, feast catalog, occurrence,
              temporal week, fasting, Tabula            shared by every product
  corpus      text file format, loader, @use/@omit, keys (no Office slot names)
  liturgy     document model: sections, elements, speaker roles, rubric spans;
              what every composer produces and every renderer consumes
  office      concurrence/Vespers, Marian antiphons, historia weeks, slot
              fallbacks, appointment scopes, hour composers; Matins later
  mass        (later) Mass rules and propers composer
  ordo        ordo document from calendar + office + mass; rubrics TSV
  daily       the vicariate JSON feed: versioned schema, contract tests
  render-html, render-tex, render-text    depend only on `liturgy`
  tools       validate, audit, lint, review, scaffold, over any corpus
apps/
  cli, office-web, missal-web, api    separate binaries, or one server that
                                      mounts them all; decide at deploy time
  ffi         UniFFI types for Swift and Kotlin
  ios/, android/                      later
```

Where today's Go packages land:

| Go package | Rust home |
|---|---|
| `calendar` | `calendar`, except `concurrence.go` and `historia.go`, which move to `office` |
| `models` | split by owner: calendar types to `calendar`, the composed hour to `liturgy`, Office types to `office` |
| `texts` | `corpus` (format, loader, directives, line grammar); appointment scopes and slot rules to `office` |
| `office` | `office` |
| `output` | `render-text`, `render-tex`, `ordo` |
| `render` | `render-html`, with assurance metadata passed in rather than imported from `review` |
| `web`, `usage` | `apps/office-web` |
| `cli` | `apps/cli` |
| `audit`, `review`, `scaffold` | `tools` |
| `dump`, `e2e` | the differential harness and per-crate tests |

### Rules

- `calendar`, `corpus`, `liturgy`, `office`, and `mass` do no file, network,
  or clock access; they take a loaded corpus and a date. CI builds them for
  `aarch64-apple-ios` and `aarch64-linux-android` from the first week, so a
  dependency that won't cross-compile never gets in.
- The published contracts are the `daily` JSON schema and the `ffi` types.
  Both are versioned and contract-tested; everything behind them may change.
- Catch-all match arms are denied on liturgical enums in the core crates
  (`clippy::wildcard_enum_match_arm`). Adding `Hour::Matins` should break the
  build at every place that has to consider it.
- An open rubrical question is a value, such as `Outcome::Unruled { issue: 42 }`,
  that appears in the decision trace and the ordo, not a branch that quietly
  picks something.
- Renderers lay out the document model; they don't parse text. After the
  cutover, verse, stanza, and block parsing moves from the renderers into the
  core, so the HTML, TeX, Swift, and Kotlin renderers can't drift apart.

## The oracle

`office dump` writes the observable state of the engine as a stream of
canonical JSON records. The Rust engine must write the same bytes.
`office dump diff` compares two streams and reports the first differences by
JSON Pointer. `office dump digest` fingerprints a stream into the parity
snapshot that CI checks.

```bash
./office dump -start 2026 -years 1 > go.jsonl        # one civil year, ~310 MB, ~4 s
./office dump -dates 2026-04-12,2026-11-02 -hours lauds,vespers -forms private
./office dump -start 1900 -years 300 -groups calendar,office   # calendar only, ~7 s
./office dump diff go.jsonl rust.jsonl
./office dump digest rust.jsonl                      # compare with the parity golden
```

Streams can be piped rather than stored:
`./office dump diff <(./office dump -start 2026 -years 28) <(office-rs dump -start 2026 -years 28)`.

The Go generator in `internal/dump/records.go` is the reference field list.
This section states the rules a second implementation needs.

### Dump format (`office-dump/2`)

**Encoding.** One record per line, each a JSON object followed by `\n`.

- Object keys are sorted by byte. There is no whitespace.
- Strings are UTF-8. Only `"`, `\`, and control characters are escaped:
  `\b \t \n \f \r` in short form, other controls as lowercase `\u00xx`.
  Everything else is literal, including `<`, `>`, `&`, and U+2028.
- Numbers are integers only.

For this value domain the encoding is RFC 8785, and it is exactly what
`serde_json::to_string` produces for a `serde_json::Value`, whose map is sorted
as long as serde_json's `preserve_order` feature stays off.

**Values.**

- Keys are `snake_case`. Enumerated values are the kebab-case strings of the
  data files (`double-1st-class`, `i-of-following`). Dates are `YYYY-MM-DD`.
- Every field is always present. Absent values are `null`, never `""`: the
  encoder rejects empty strings, so `None` and `Some("")` can't be confused.
  Go's zero values (`""`, `0` for an unset month or day) become `null`.
  Empty lists are `[]`.

**Records.** Each has a `kind`. They appear in this order:

1. `meta`: format name and the normalized selection.
2. The corpus, which depends on `data/` alone: a `corpus_entry` for every
   resolvable key in byte order (its `@use` or `@omit` directive, the direct
   `use_target`, the `canonical` key, the resolved `body`, the collect
   conclusion form, and the Latin incipit), then an `appointment_scope` for
   each scope in file order.
3. Per civil year, `calendar_year`: the Tabula and every moveable date.
4. Per date, `calendar_day`: the observance every product shares. It holds
   season, tempora, celebration, commemorations, the feria commemoration,
   color, notes, the occurrence rule and trace, temporal week, octave, and
   fasting.
5. Per date, `office_day`: what only the Office resolves. It holds the
   Vespers designation (owner, feast, color, commemorations, the split at the
   Chapter, the appended Office of the Dead) and the Marian antiphon.
6. Per date, `hour` for each hour (Lauds through Compline) and prayer form
   (private, deacon, priest): title, color, sections, elements, and the
   decision trace.

`calendar_day` and `office_day` partition Go's `CalendarDay`. The two fields
that exist only on the synthetic I Vespers day (`FirstVespers`,
`FollowingOfficeCommemorationID`) are rejected if a calendar-built day carries
them. `Feast.Notes` is left out: it is documentation and never reaches output.

Selections: `-start`/`-years` or `-dates`, narrowed by `-hours`, `-forms`, and
`-groups` (`corpus`, `calendar`, `office`, `hours`). The stream order depends only on
what is selected, not on how it is spelled.

### Parity snapshot (`office-parity/2`)

`internal/e2e/testdata/golden/parity-snapshot.json` is the digest of the
2026–2053 dump. Each digest is a SHA-256 over canonical lines:

| Digest | Lines |
|---|---|
| `corpus` | the `corpus_entry` and `appointment_scope` records |
| `calendar` (per year) | the `calendar_year` and `calendar_day` records |
| `office` (per year) | the `office_day` records |
| `content` (per year, hour, form) | the date plus hour label, title, season, feast, color, section labels, and each element's type, label, incipit, rubric, text, and spoken voice text |
| `presentation` | the date plus each element's display text, announce flag, leader slot, rubric spans, and voice roles |
| `sources` | the date plus each element's slot, source refs, and commemoration owner |
| `decisions` | the date plus the hour's decision trace |

Nesting is kept, so an element's position is part of every hour digest. Every
hour field belongs to exactly one digest; digesting fails on an unassigned
field. The snapshot also lists every fuzzy-name commemoration merge, because
those decisions deserve human review.

The snapshot is written in canonical indented form (`serde_json::to_string_pretty`
plus a final newline). Building it takes about 50 seconds on 12 cores, one
worker per year.

## Phases

Each phase ends at a gate. A gate is met when the Rust and Go outputs are
identical, not when the tests look plausible.

| Phase | Scope | Gate |
|---|---|---|
| 0 | The oracle, in Go (this document's first PR) | Parity snapshot computed from the dump; all goldens unchanged |
| 1 | Workspace, toolchain, CI; `calendar` crate | `calendar` group identical for 1900–2199; Tabula identical |
| 2 | `corpus` crate, line grammar, `validate` | `corpus` group identical (see below); validation errors identical on broken test corpora |
| 3 | `office` composer, one hour at a time: Compline, the minor hours, Prime, Lauds, Vespers | `office` and `hours` groups identical for 2026–2053 in every form; text goldens byte-identical |
| 4 | `ordo`, renderers, `tools` | ordo text, rubrics TSV, and TeX byte-identical; audit, lint, and assurance reports identical; review subcommands still in use ported |
| 5 | `apps/office-web`: routes, templates, static assets, reminder feed, usage store | a crawl of both servers matches after HTML normalization; the Playwright suite, visual snapshots included, passes unchanged |
| 6 | Cutover | a shadow period on the live service; a tagged Go build kept for rollback; then Go deleted |
| 7 | After cutover | the inherited-from-Go list sorted into "right", "bug", and "needs ruling"; the model tightened; text parsing moved into the core; the data layout for the Missal; then Matins, the Mass, and mobile |

PR CI compares a sample: every golden date plus a fixed set of boundary
dates. A nightly job compares the full window.

### Rules while both engines exist

- **Data changes flow as usual.** Both engines read the same `data/`, and CI
  compares them against each other at the same commit, never against frozen
  dumps.
- **Engine changes land in Go first**, with a test; Rust follows. Pause engine
  changes in a package while it is being ported.
- **When the engines disagree**, decide which is right. If Go is wrong, fix Go
  first so the oracle stays trustworthy, then Rust.
- **Port faithfully.** Where Rust makes a Go default explicit, tag it
  `// PORT(inherited): <what Go did>` and keep Go's behavior. Behavior changes
  wait for Phase 7; mixing them into the port makes translation bugs
  indistinguishable from fixes.
- **Port the Go unit tests with each crate.** They encode edge cases the black
  box can't reach.

## Go-to-Rust traps

- **Strings.** Go slices strings by byte and silently splits a character;
  Rust panics. `models.AntiphonAnnouncement` does `incipit[len(incipit)-1:]`,
  which would panic on an antiphon ending in ’. Use char-based APIs. The dump
  encoder rejects invalid UTF-8, and the full 2026–2053 sweep produces none,
  so Go never emits a broken character today.
- **Regexes.** Go's `regexp` and Rust's `regex` are both RE2-style, but check
  Unicode classes (`\w`, `\b`, `(?i)`) in each of the 16 patterns.
- **Sorting.** Every Go sort with a custom comparator is now stable
  (`sort.SliceStable`), matching Rust's `sort_by`. Don't use `sort_unstable*`
  where ties are possible.
- **Map order.** Go randomizes map iteration and so does Rust's `HashMap`.
  Sort before output or use `BTreeMap`.
- **JSON.** Go's `encoding/json` escapes `<`, `>`, `&`, and U+2028. Anything
  compared across implementations goes through the canonical encoder.
- **Dates.** Go uses `time.Time` at UTC midnight, with Sunday as weekday 0.
  Use a civil-date type with no time zones in the core. The engine does no
  month arithmetic with `AddDate`, so its normalization rules don't matter.
- **Zero values.** Go's `""`, `0`, and `nil` all mean absent. The dump maps
  them to `null`, which is `Option` in Rust.

## Phase 0 checklist

- [x] `office dump` with the shared calendar day, Office day, and hour records
- [x] `office dump diff` and `office dump digest`
- [x] Parity snapshot recomputed from the dump, digested one year per worker
      (the sweep went from 67 to 52 seconds)
- [x] Stable sorts throughout; a total order for the review manifest;
      fixed-order field validation instead of map iteration
- [x] Phase 2 prerequisites: a `corpus` record group (every key with its
      resolved body and directive) and broken test corpora with expected
      `validate` output (`internal/e2e/testdata/broken-corpora/`, one data
      directory per case, the calendar and texts reports in `expected.txt`
      with the data directory written `$DATA`). The validators now report in
      a fixed order instead of Go map order.

## Phase 1 checklist

- [x] Cargo workspace at the repository root (`crates/calendar`, `apps/cli`),
      toolchain pinned in CI, `rustfmt.toml`
- [x] `calendar` crate: civil date type, computus and moveable dates, seasons,
      Tabula, feast and penitential loaders (through a `DataSource`, no file
      access), occurrence, XIV.14 commemoration order, the year builder
- [x] `office-rs dump` for the `calendar` group (`-start`/`-years`, `-dates`,
      `-hours`, `-forms`, `-groups`)
- [x] Gate: `make rust-parity` — `calendar` group identical for 1900–2199,
      Tabula included
- [x] CI: `make rust-check` (fmt, clippy `-D warnings`, tests), iOS and
      Android cross-builds of the core crates, and the parity gate
- [ ] Port the remaining Go calendar unit tests (a first set is ported; the
      data-backed builder assertions are also covered by the parity gate)

## Phase 2 checklist

- [x] `corpus` crate: the `data/texts/` format (INI and plain files, comment
      stripping, Go `filepath.Walk` order), `@use` and `@omit`, the collect
      conclusion and Latin incipit sidecars, and the line grammar
      (`parse_psalm`, `parse_block`, `parse_hymn`); Go's tests ported
- [x] Appointment scopes in the `office` crate (they name hours and slots),
      loaded with the corpus by `office::texts::load_texts`
- [x] `tools` crate: the filesystem `DataSource` and the calendar and texts
      validators
- [x] `compat` crate: Go-compatible `%q`, `bufio.Scanner` lines, and `Atoi`,
      so messages match byte for byte; removed after the cutover
- [x] Gate: the `corpus` group is identical (`make rust-parity`), and
      `cargo test -p tools` reproduces every `expected.txt` under
      `internal/e2e/testdata/broken-corpora/`

Known gap, by design: JSON syntax errors in `appointment-scopes.json` carry
serde_json's wording rather than encoding/json's. Semantic errors match.

## Phase 3 checklist

- [x] `office` crate: Vespers concurrence (`concurrence.go`), the Marian
      antiphon, and the historia weeks; Go's concurrence unit tests ported
- [x] `office_day` record identical for 1900–2199 (`make rust-parity`)
- [x] `liturgy` crate: the document model (element types, voice and rubric
      spans, `OfficeHour`, `PrayerForm`)
- [x] Hour composers: Compline, the minor hours, Prime, Lauds, Vespers (with
      Vespers of the Dead), proper resolution, psalmody declarations,
      commemorations, preces and suffrage, conclusions, and the prayer-form
      pass; `office::Engine` loads the corpus and hour definitions
- [x] `render-text` crate: Go's `FormatOfficeHour`
- [x] Gate: `hours` group identical for 2026–2053 in every form
      (`make rust-parity-full` digests the Rust dump and diffs it with
      `parity-snapshot.json`; nightly in CI), a sample of 127 dates in PR CI
      (`make rust-parity`: every golden date plus boundary dates), and every
      hour golden byte-identical (`cargo test -p render-text`)
- [ ] Port the Go `office` unit tests (the black-box gate covers the sweep;
      the unit tests reach cases it cannot)
- [ ] The Martyrology preview at Prime (unpublished; not in the dump)
- [ ] Composition tracing (`TraceProperResolution`), used by the review tools
      in Phase 4

HTML goldens are intentionally absent. Phase 5 crawls the Go and Rust servers
and compares them directly; checked-in HTML would churn with every UI change
until then.

## Phase 4 checklist

- [x] `ordo` crate: the text ordo (Tabula Temporaria and per-hour stanzas)
      and the rubrics TSV; `office-rs ordo` and `office-rs rubrics` are
      byte-identical to Go (`make rust-parity`: 2026, 2027, 2038; the ordo
      goldens in `cargo test -p ordo`)
- [x] `render-tex` crate: Go's `FormatOfficeHourTeX`; the caller supplies the
      GABC score lookup, so the renderer does no file access. `office-rs tex`
      is byte-identical for every hour on the 127 sample dates, cycling the
      prayer forms and `--chant` (`scripts/rust-parity-tex.sh`)
- [x] `validate`: the hour definitions (`office::validate`), the provenance
      and zero-occurrence ledgers (`tools::review`), and Go's `encoding/csv`
      (`compat::csv`, fuzzed against Go). The broken corpora gain `office`
      and `review` layers and five cases; the Rust reports match on all 15
      (`cargo test -p tools`), and `make rust-parity` runs `validate` on the
      live data
- [ ] `audit` (placeholders, missing propers, the composition sweep)
- [ ] `lint` (the text-corpus lints)
- [ ] Assurance and the review subcommands still in use
