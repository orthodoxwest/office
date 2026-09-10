# AWRV Benedictine Divine Office

[Pray the Office](https://office.fly.dev) — the English Benedictine daily
office for Antiochian Western Rite Vicariate use, with each hour's prayers,
psalms, and propers assembled on one page.

Lauds, Prime, Terce, Sext, None, Vespers, and Compline are supported; Matins
is not yet included. The calendar uses the Julian paschalion and pre-1962
feast ranks, with AWRV observances and the Coverdale Psalter. The newest local
archdiocesan ordo governs current-year practice.

The web app includes a browsable calendar, an installable PWA with cached
pages available offline, and a subscribable prayer-reminder calendar at
`/reminders`. Each hour has a collapsed Assurance disclosure for source and
composition metadata. Corpus verification is ongoing; see
[REVIEWING.md](REVIEWING.md) for how to help.

Each hour has a **Prayer form** selector near the date control: **Praying privately**,
**With others, led by a deacon**, or **With others, led by a priest**. Private
is the default for anyone praying alone, including clergy. The ordained
choices are for following or leading choir prayer; choir without clergy is not
modeled yet. Explicit choices are remembered on the device and can be changed
offline. A `?form=deacon` (or `private` / `priest`) link temporarily overrides
the saved choice.

## Run locally

Install Go at the version required by [go.mod](go.mod) (currently 1.26.3 or
later) and Make. On Windows, use WSL.

```bash
git clone https://github.com/orthodoxwest/office.git
cd office
make serve
```

Open [localhost:8080](http://localhost:8080). `make serve` builds the binary
and starts the server. Data is loaded at startup, so restart the server after
editing it. To run an already-built binary, use `./office serve`.

## Contribute texts or review

Liturgical data is plain text under [data/](data/). You can propose edits
through GitHub's file editor without installing the app. Include the source
and page or section for a correction.

- [Editing liturgical data](DATA.md): file formats, proper slots, aliases,
  scaffolding, and psalm numbering.
- [Reviewing the Office](REVIEWING.md): checking texts and rendered hours,
  recording findings, and tracking source and structural coverage.
- [Diurnal transcription and discovery](scripts/DIURNAL-PIPELINE.md): reading
  scanned pages, comparing the corpus, and finding propers hidden by fallbacks.

The page-image workflow starts with `make pages`, then `make transcribe` or
`make discover`. Transcription and discovery prepare prompts by default;
`APPLY=1` enables readers and application through the workflow's checks.
Keep source PDFs outside the repository and generated artifacts under ignored
`output/`.

Text attestations and structural hour signoffs are separate. The provenance
queue prioritizes individual corpus entries; the structural review plan
covers composition rules. Their commands and attestation semantics are
documented in [REVIEWING.md](REVIEWING.md).

## Common commands

Run commands from the repository root after `make build`. Use `make help`
for the full list of build and maintenance targets.

```bash
./office lauds 2026-09-04          # render an hour (also prime … compline)
./office prime 2026-03-11 --form deacon # choir form (also available for tex)
./office ordo 2026                # annual ordo
./office rubrics 2026             # per-day rubric and antiphon TSV
./office validate                # check data files
./office audit                   # report missing texts and fallback coverage
./office lint                    # mechanical and advisory corpus findings
./office corpus show proper/st-andrew/collect
./office review provenance       # source coverage, flat + usage-weighted
./office review provenance-queue # prioritize text review
./office review explain lauds 2026-09-04
make project-status YEAR=2026     # proper, assurance, and ordo reports
```

## Development

The full check suite needs Go, Make, Python 3, `staticcheck`, Node.js 20.19+
and npm. CI uses Node.js 22. Install the pinned tooling:

```bash
go install honnef.co/go/tools/cmd/staticcheck@v0.7.0
npm ci --prefix .web-tools
make install-hooks               # optional: pre-push runs make check
make check
```

Ensure the Go binary directory (`go env GOBIN`, or `$(go env GOPATH)/bin`
when unset) is on your `PATH`.

```bash
make test                        # Go and Python tests, including golden files
make check                       # formatting, analysis, tests, validation, lint
make diurnal-test                # page/transcription/discovery tests; no providers
make golden                      # update expected output after intentional changes
make verify-psalms                # compare the psalter with its reference witness
```

Review golden-file changes under [internal/e2e/testdata/golden/](internal/e2e/testdata/golden/)
before committing. They cover rendered hours, parity, and assurance coverage.

Playwright behavior and accessibility tests run separately from `make check`:

```bash
npm --prefix .web-tools run install:browser
make test-ux
```

Visual snapshots run in CI's pinned browser container. For intentional visual
changes, inspect the Playwright report and apply the `update-ux-snapshots`
PR label to regenerate them. See the [CI workflow](.github/workflows/ci.yml)
and [snapshot workflow](.github/workflows/update-ux-snapshots.yml) for details.

## PDF booklets

Generate half-letter (5.5 × 8.5 inch) booklets with LuaLaTeX:

```bash
make pdf HOUR=compline
make pdf HOUR=lauds DATE=2026-09-04
make pdf HOUR=compline CHANT=1     # engrave available GABC scores
```

Output goes under `output/`. The template requires LuaLaTeX, GregorioTeX,
EB Garamond, and Noto Sans Symbols. Its symbol-font path currently assumes
the Debian/Ubuntu font layout; see [the TeX template](internal/output/tex.go).
`make pdf` enables shell escape for chant compilation. Elements without
scores use formatted text.

For TeX output alone, use `./office tex compline` or
`./office tex --chant compline`. Optional saddle-stitch imposition with
`pdfjam`:

```bash
pdfjam --booklet true --paper letter output/compline-2026-09-04.pdf
```

## Deployment

The app runs on Fly.io; configuration is in [fly.toml](fly.toml).
The [Dockerfile](Dockerfile) builds a static Go binary and copies it with
`data/` into a `scratch` image.

Maintainers with Fly access can deploy with:

```bash
fly deploy
```

### Usage metrics

The unlinked, unauthenticated `/admin/usage` report shows daily unique browsers
and unique browsers opening each office, the ordo, and the reminder feed, plus
how those browsers rendered the page, with 7/30/90/366-day views. It is public
to anyone who knows the URL, marked `noindex`, and excluded from the
service-worker cache. Reporting days use America/New_York, including DST.
Counts describe visible pages, not completed prayers. A browser opening several
hours counts once overall and once in each hour column; likewise the Ordo
column counts a viewed calendar page, and the Reminders column counts a
*generated* feed link (the subscribe or copy action on `/reminders`, not merely
loading that page) — both also count once toward the overall total. Home
contributes to the overall total but has no column of its own. Preloads do not
count; offline use and browsers without JavaScript are missed. Separate
devices, blocked cookies, and cleared cookies can inflate the approximate user
count.

Two further dimensions ride along on the same beacon, describing how a page
was rendered rather than which page it was, to show where design work pays
off: **Nave vs Apse** — the appearance actually on screen, whether chosen or
inherited from the device — and **Desktop vs Mobile** — a viewport under the
700px layout breakpoint or a touch-primary pointer, so tablets and a phone in
landscape count as mobile, as does a narrowed desktop window. Each is counted
like any other scope, once per browser per day, and stored under a qualified
`family:value` name (`appearance:apse`, `screen:mobile`) rather than the bare
value. A value therefore belongs to exactly one family, two families may each
offer a `default`, and a family retired later cannot be silently continued by
a different one that reuses one of its value names. A key or value that has
been written is never redefined: add a new key, so an old series ends where
its meaning ended instead of changing mid-flight. The report draws each pair as
a day-by-day mix band with its period share beside it: a single figure for the
window cannot tell a settled 60/40 from a migration that passed through it,
and the longer the window the more movement it would hide. Both compare with
each other, not with the daily totals; a day nobody reported leaves a gap in
the band rather than being drawn as an even split, and a single day's share
swings easily at these volumes, so the period figure is the steadier one.

A reader who switches appearance or rotates a phone during the day can count
on both sides, so a pair can exceed the daily total; a browser still serving a
cached `app.js` from before this existed reports neither, so a pair can also
fall short of it.

Office pages also report `prayer-form:private|deacon|priest`, including a
selection changed during an engaged visit. This measures the prayers being
read, not the reader's ordination. The report shows browser-days for each
form and daily counts; someone using two forms on the same day counts in both,
while the overall visitor and hour counts remain deduplicated. This dimension
is ignored on Home, Ordo, and Reminders, and no offline history is queued.

Only the scope has to be understood server-side. The service worker keeps
`app.js` across deploys, so both directions of skew are ordinary: a client
from before dimensions existed posts the bare scope, and a client cached from
a build newer than the server posts tokens the server has never heard of.
Either way the page still counts and the unreadable tokens are dropped, rather
than un-refreshed browsers dropping out of the report.

Three filters keep scraping from becoming usage. The site stays freely
crawlable — there is no robots.txt and the dated archive is deliberately
open — but a crawl must not read as a congregation, because a crawler
presents a fresh cookie jar per render and would otherwise mint one "unique
browser" for every URL it touches:

- **Only a current page counts.** `data-usage-when` carries the page's day
  (or the ordo's year), and the beacon refuses anything outside today ±1 day
  (±1 year for the ordo). The dated archive is unbounded; today's surface is
  about ten pages, so that is the ceiling on what any crawl can contribute,
  however deep it goes.
- **Engagement is required.** Nothing is reported until someone touches the
  page or leaves it visible for eight seconds. Dwell accrues only while the
  page is visible, so a background tab never qualifies and a scraper that
  renders and moves on reports nothing.
- **Self-identifying crawlers are dropped** server-side by user agent
  (`usage.IsBot`), answered `204` with no cookie and no count. The bare word
  "bot" is not enough to be dropped — phone agents such as "Cubot Note 20"
  embed it — so the check wants the conventional `<name>bot/<version>` form
  or an explicit token.

Honest limits: a stealth scraper with a browser user agent that dwells or
clicks on today's pages still counts, and a real reader browsing only the
archive is missed. Both are bounded and deliberate.

Fly configuration expects one replica and a volume named `office_usage` in
`iad`, mounted at `/usage`. Create it before deploying:

```bash
fly volumes create office_usage --region iad --size 1 --app office
```

`OFFICE_USAGE_DB=/usage/usage.sqlite` enables metrics. For local use:

```bash
mkdir -p output/usage
OFFICE_USAGE_DB="$PWD/output/usage/usage.sqlite" go run ./cmd/server serve
```

Unset `OFFICE_USAGE_DB` to disable storage (the event endpoint is a no-op and
the report returns 404). If SQLite cannot open, the server logs a warning and
continues serving offices without metrics. Runtime storage failures return
503 only on the metrics endpoints. The parent directory must already exist.

The first-party `office-usage` cookie is a random 128-bit identifier, lasts
30 days, and is HttpOnly and SameSite=Strict (Secure over HTTPS). Its path is
limited to `/api/usage`. SQLite stores only a different hash each day, an
office category, and a reporting date; no raw cookie, IP address, user agent,
URL, or precise visit timestamp is stored. Deduplication rows older than the
current day and previous two days are deleted on the next event or report
read. Aggregate counts are retained indefinitely. Existing volume snapshots
can retain older database contents until those snapshots expire.

SQLite uses WAL mode and a single connection. Keep the database and its WAL
files together on the volume; use SQLite-aware backups or volume snapshots,
not a copy of the live database file alone. Multiple app replicas would have
independent counts and require a different storage arrangement.
