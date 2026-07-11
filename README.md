# AWRV Benedictine Divine Office

Web application that renders the complete text of the Benedictine Divine Office — no jumping around in a physical breviary required.

**Live:** https://office.fly.dev

## Hours covered

Lauds, Prime, Terce, Sext, None, Vespers, Compline (no Matins yet).

## Liturgical specifics

- **Calendar:** Julian paschalion, pre-1962 feast ranking
- **Psalter:** Coverdale (Book of Common Prayer)
- **Language:** English only
- **AWRV additions:** post-schism Eastern saints (St. Raphael of Brooklyn, St. Innocent of Alaska, St. Tikhon of Moscow, St. Herman of Alaska)

---

## Contributing

### Editing data files (no programming required)

The liturgical texts live in the `data/` directory as plain text files. You can edit them directly on GitHub without installing anything.

**To fix a text or fill in a missing proper:**

1. Find the file you want to edit in `data/texts/proper/` (one file per feast) or `data/texts/commons/` (shared texts by category).
2. Click the pencil icon on GitHub to edit the file in your browser.
3. Make your changes, then click **Propose changes** to open a pull request.

**To find what's missing:** look at the live site and note any feasts that show placeholder text, or ask a maintainer to run `make audit` (see below).

See the [Data files](#data-files) section for the file format and valid keys.

### Running locally (no Go experience required)

You need two things: Go and Make.

**Install Go:**
- Download from [go.dev/dl](https://go.dev/dl/) — pick the installer for your OS (Windows, macOS, or Linux).
- Verify: open a terminal and run `go version`. You should see `go1.21` or later.

**Install Make:**
- macOS: comes with Xcode Command Line Tools (`xcode-select --install`)
- Linux: `sudo apt install make` (Debian/Ubuntu) or `sudo dnf install make` (Fedora)
- Windows: use [WSL](https://learn.microsoft.com/en-us/windows/wsl/install) and then `sudo apt install make`

**Clone and run:**

```bash
git clone https://github.com/orthodoxwest/office.git
cd office
make build   # builds the ./office binary
make serve   # starts the server at http://localhost:8080
```

Open http://localhost:8080 in your browser. The server loads all data files at startup, so after editing a file you need to restart it (`Ctrl-C`, then `make serve` again). You do not need to run `make build` between restarts unless you changed Go code.

---

## Running locally (for developers)

Requires Go 1.21+.

```bash
make build   # build ./office binary
make serve   # start server on http://localhost:8080
```

The server finds `data/` relative to the binary or the working directory.

## CLI commands

```bash
./office serve [addr]              # web server (default :8080)
./office ordo YEAR                 # print text ordo for a year
./office lauds YYYY-MM-DD          # print Lauds text for a date
./office vespers YYYY-MM-DD        # (same for prime, terce, sext, none, compline)
./office tex HOUR [YYYY-MM-DD]     # emit LaTeX booklet for HOUR (date defaults to today)
./office validate                  # validate all data files
./office audit                     # report placeholder texts and missing propers
make verify-psalms                  # machine-check all Coverdale Psalm files
./office review provenance         # generated source/provenance coverage summary
./office review provenance-queue   # dependency-weighted atomic text review queue
./office review attest [flags] KEY REVIEWER # verify one text against its source
./office review explain HOUR DATE  # JSON dependencies and decisions for one hour
./office review plan               # minimal structural-review checklist CSV
./office review sign HOUR DATE REVIEWER # sign off one reviewed hour page
./office review assurance          # release assurance gates and summary
```

## PDF booklets

The `tex` subcommand produces a `.tex` file formatted as a half-letter (5.5"×8.5") booklet, suitable for printing handouts (e.g. Compline for overnight guests, Sunday Lauds for visitors).

```bash
# Quickest path — generate and compile in one step:
make pdf HOUR=compline              # today's date
make pdf HOUR=lauds DATE=2026-03-15

# Or step by step:
./office tex compline > compline.tex
lualatex compline.tex

# Booklet imposition for saddle-stitched printing:
pdfjam --booklet true --paper letter compline.pdf
```

**Requirements:** lualatex, EB Garamond (`fonts-ebgaramond`), Noto Sans Symbols (`fonts-noto-extra`). gregoriotex is optional — loaded automatically if installed, skipped otherwise. GABC chant scores (when added) live in `data/texts/chant/`.

## Development

```bash
make test     # run all tests (includes golden file tests)
make check    # fmt + vet + lint + test + validate
make golden   # regenerate golden files after intentional changes
make audit    # show data completeness report
make review-provenance # generated corpus provenance counts
make review-provenance-queue # prioritize atomic text verification
make review-plan       # minimal structural-review checklist
make review-assurance  # release assurance gates
```

Golden files live in `internal/e2e/testdata/golden/`. Alongside representative
rendered hours, `assurance-report.md` records the current review counts and
sorted structural feature inventory so coverage changes appear directly in a
PR diff. Run `make golden` after changing office composition, text output, or
assurance coverage, then review the diff before committing.

### Assurance and review planning

Text verification and structural verification are tracked separately:

- `./office review provenance` derives non-stale counts from every corpus entry
  and its adjacent `# SOURCE:` / `# TODO(diurnal):` annotations. Add `-csv` for
  the complete inventory.
- `data/review/provenance.csv` records explicit source attestations. A
  `verified` row must name its source, a page or section locator, reviewer, and
  review date. It stores citations and internal content-version metadata, not
  source-book contents.
- `./office review explain lauds 2026-06-07` emits a JSON assurance manifest
  containing the corpus dependencies, provenance status, condition branches,
  exact occurrence/commemoration decisions, color resolution, transfers, and
  calendar/concurrence rule identifiers behind that hour.
- `./office review provenance-queue -start 2026 -years 1` ranks every atomic
  corpus entry by distinct composition fan-out, priority-A use, principal-hour
  use, and total occurrences. Verified entries are excluded unless
  `-include-verified` is supplied.
- `./office review plan -start 2026 -years 1` uses greedy set cover to select a
  small checklist exercising every structural decision and fallback tier. Text
  entries are verified independently through provenance. Add
  `-include-sources` only when a checklist that also renders every used corpus
  key is desired.

Record a completed source check without editing CSV manually:

```bash
./office review attest --source "Printed Diurnal" --page 123 \
  --locator "Proper of Example" --note "word-for-word" \
  proper/example/collect reviewer
```

The command binds the attestation to the current corpus text automatically so
later changes make it stale. An existing attestation requires `--replace`.

Record a completed structural review by the hour and date that was checked:

```bash
./office review sign lauds 2026-06-07 reviewer
```

The command resolves the page's internal version identity automatically; no
identifier needs to be copied from a checklist.

`./office review assurance` runs the representative multi-year structural
plan, validates provenance, and enforces the reviewable floors in
`data/review/assurance-baseline.json`. CI publishes its source-content-free
Markdown summary. After an intentional increase in verified coverage or
modeled rules, refresh the floor with `--update-baseline` and review the JSON
diff.

On rendered hour pages, a collapsed **Assurance** disclosure shows provenance
counts, corpus dependency keys, fallback tiers, and composition rule IDs.
Unverified texts are either `needs-review` when a source lead or explicit
review task exists, or `source-unknown` when provenance research must come
first.
It deliberately omits local paths, source-book links, and source contents;
expanding it is optional and does not alter the prayer view or review state.

---

## Data files

Liturgical data lives in `data/`. The engine reads these at startup — no recompile needed when editing them.

```
data/
  feasts/          feast definitions (sanctoral, temporal, AWRV-specific)
  seasons.txt      season date rules
  texts/
    psalms/        Coverdale Psalter, one file per psalm (Hebrew numbering)
    canticles/     Benedictus, Magnificat, Nunc Dimittis, etc.
    hymns/         hymn texts
    ordinary/      fixed prayers, versicles, Marian antiphons
      session.txt  session opening/closing prayers (Aperi Domine, Sacrosanctae)
    proper/        feast-specific antiphons and collects (one file per feast)
    commons/       texts by category (apostle, martyr, confessor, etc.)
    seasonal/      season-specific overrides
  office/          hour structure definitions
```

### File format

A simple INI-like format: `[section]` headers, `Key = value` lines, `#` comments, blank lines for readability. No indentation sensitivity, no quoting, no multiline values.

**Feast definition** (`data/feasts/sanctoral.txt`):

```ini
[st-andrew]
Name     = Saint Andrew, Apostle
Rank     = double-2nd-class
Color    = red
Category = apostle
Month    = 11
Day      = 30
```

Valid `Rank` values: `double-1st-class`, `double-2nd-class`, `greater-double`, `double`, `semi-double`, `privileged-feria`, `simple`, `commemoration`

Valid `Color` values: `white`, `red`, `violet`, `rose`, `black`

Valid `Category` values: `lord`, `blessed-virgin`, `apostle`, `evangelist`, `martyr`, `bishop-martyr`, `virgin-martyr`, `confessor`, `confessor-bishop`, `confessor-doctor`, `holy-woman`, `angel`, `sunday`, `feria`

Optional keys: `HasOctave = true`, `HasVigil = true`, `ProperName = Andrew` (saint's given name, substituted for `N.` in common texts), `DateRule` (for moveable feasts instead of `Month`/`Day`).

**Feast proper** (`data/texts/proper/st-andrew.txt`):

Each `[section]` key corresponds to a liturgical text slot. A feast file need only include the slots it actually has — the engine falls back to the common (by `Category`) or ordinary for anything omitted.

In prose sections such as collects, chapters, and prayers, a single newline is a soft source wrap and the web renderer lets the paragraph reflow. A blank line starts a new paragraph. Hymn and psalm renderers preserve their verse structure automatically.

```ini
[psalm-antiphon]
The Lord saw Peter and Andrew, * and He called them.

[benedictus-antiphon]
There followed the Lord two brethren, Peter and Andrew.

[magnificat-antiphon]
O Lord, Thou hast caused them that persecuted the just to be swallowed up in hell,
* but to the just Thou hast thyself shown the way on the tree of the cross.

[collect]
O Lord, we humbly beseech thy Majesty: that even as Thou didst give thy blessed
Apostle Andrew to thy Church to be a teacher and ruler on earth, so, now that he is
with thee, he may continually make intercession for us.
```

Valid sections for feast propers:

| Section | Used in |
|---------|---------|
| `[psalm-antiphon]` | Psalm antiphon at Lauds |
| `[benedictus-antiphon]` | Benedictus antiphon at Lauds |
| `[magnificat-antiphon]` | Magnificat antiphon at Vespers |
| `[collect]` | Collect at Lauds and Vespers |
| `[commemoration-antiphon]` | Antiphon for commemoration |
| `[commemoration-versicle]` | Versicle/response for commemoration |
| `[hymn-lauds]` | Hymn slug for Lauds (overrides common) |
| `[hymn-vespers]` | Hymn slug for Vespers (overrides common) |

**Common texts** (`data/texts/commons/confessor.txt`) — used when a feast has no proper of its own:

```ini
[collect]
O God, Who, year by year, dost gladden us by the solemn feast-day of thy blessed
confessor N., mercifully grant unto all who keep his birthday, grace to follow after
the pattern of his godly conversation.
```

`N.` is a placeholder substituted with the feast's `ProperName` field at runtime (e.g. `ProperName = Nicholas` → "blessed confessor Nicholas").

### Adding missing propers

Run `make audit` to see which feasts still need proper texts. For each feast listed, create or edit the corresponding file in `data/texts/proper/`. The filename must match the feast's `[section-id]` in the feasts file (e.g. feast `[st-andrew]` → `data/texts/proper/st-andrew.txt`).

If a feast should intentionally fall back to ordinary/common texts (e.g. a feria or a minor feast without a unique proper), add its ID to `data/audit-ok.txt` to suppress the warning:

```
# data/audit-ok.txt
st-raphael-of-brooklyn *    # suppress all warnings for this feast
some-martyr commemoration-antiphon    # suppress only this one slot
```

### Psalm numbering

Hour and proper definitions use **Vulgate** psalm numbers (liturgical standard). Psalm files use **Hebrew** numbers (Coverdale standard). The engine maps between them automatically. Psalms 1–9 and 147–150 are identical in both schemes; psalms 10–146 differ by one (e.g. Vulgate Psalm 31 = Hebrew Psalm 32).

### Psalm text verification

Run `make verify-psalms` to compare every file in `data/texts/psalms/` with the [Church of England's official 1662 BCP Psalter](https://www.churchofengland.org/prayer-and-worship/worship-texts-and-resources/book-common-prayer/psalter). The check covers wording, punctuation, verse numbering, and chant separators; local `*` separators are retained as the project's representation. Historical readings are checked against the [official 1662 Book of Common Prayer PDF](https://www.churchofengland.org/sites/default/files/2019-10/the-book-of-common-prayer-1662.pdf) where the online transcription differs.

---

## Deployment

Deployed on [Fly.io](https://fly.io). Configuration is in `fly.toml`.

```bash
fly deploy   # deploy to fly.io (requires flyctl and access)
```

The Dockerfile does a multi-stage build: compiles the binary, then copies it with the `data/` directory into a minimal Debian image.
