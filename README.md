# AWRV Benedictine Divine Office

[Pray the Office](https://office.fly.dev) — the English Benedictine daily
office for Antiochian Western Rite Vicariate use, with each hour's prayers,
psalms, and propers assembled on one page.

Lauds, Prime, Terce, Sext, None, Vespers, and Compline are supported; Matins
is not yet included. The calendar uses the Julian paschalion and pre-1962
feast ranks, with AWRV observances and the Coverdale Psalter. The newest local
archdiocesan ordo governs current-year practice.

The web app includes a browsable calendar, an installable offline PWA, and a
subscribable reminder calendar at `/reminders`. Each hour has a collapsed
Assurance disclosure with source and composition metadata. Corpus
verification is ongoing; see [REVIEWING.md](REVIEWING.md) to help.

Each hour's **Prayer form** selector offers **Praying privately** (the default,
including for clergy praying alone), **With others, led by a deacon**, or
**With others, led by a priest**. Choir without clergy is not modeled yet. The
choice is remembered on the device; a `?form=private|deacon|priest` link
overrides it for that visit.

## Run locally

Install Go at the version required by [go.mod](go.mod) (currently 1.26.3 or
later) and Make. On Windows, use WSL.

```bash
git clone https://github.com/orthodoxwest/office.git
cd office
make serve
```

Open [localhost:8080](http://localhost:8080). `make serve` builds and starts
the server (`./office serve` runs an existing binary). Data loads at startup,
so restart after editing it.

## Contribute texts or review

Liturgical data is plain text under [data/](data/), so you can propose edits
in GitHub's file editor without installing anything. Cite the source and page
or section for each correction.

- [Editing liturgical data](DATA.md): file formats, proper slots, aliases,
  scaffolding, and psalm numbering.
- [Reviewing the Office](REVIEWING.md): checking texts and rendered hours,
  recording source-backed composition checks, and tracking text provenance.
- [Diurnal transcription and discovery](scripts/DIURNAL-PIPELINE.md): reading
  scanned pages, comparing the corpus, and finding propers hidden by fallbacks.


## Common commands

Run from the repository root after `make build`; `make help` lists every target.

```bash
./office lauds 2026-09-04          # render an hour (also prime … compline)
./office prime 2026-03-11 --form deacon # deacon-led form (also available for tex)
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

Review changes under [internal/e2e/testdata/golden/](internal/e2e/testdata/golden/)
(rendered hours, date-sensitive parity, text provenance) before committing.

Playwright behavior and accessibility tests run separately from `make check`:

```bash
npm --prefix .web-tools run install:browser
make test-ux
```

Visual snapshots run only in CI's pinned browser container. For intentional
visual changes, inspect the Playwright report and add the `update-ux-snapshots`
PR label to regenerate them ([workflow](.github/workflows/update-ux-snapshots.yml)).

## PDF booklets

Generate half-letter (5.5 × 8.5 inch) booklets with LuaLaTeX:

```bash
make pdf HOUR=compline
make pdf HOUR=lauds DATE=2026-09-04
make pdf HOUR=compline CHANT=1     # engrave available GABC scores
```

Output goes under `output/`. Requires LuaLaTeX, GregorioTeX, EB Garamond,
and Noto Sans Symbols; the symbol-font path assumes the Debian/Ubuntu layout
([template](internal/output/tex.go)). `make pdf` enables shell escape for
chant; elements without scores render as text. `./office tex [--chant] HOUR`
emits the TeX alone. For saddle-stitch imposition:

```bash
pdfjam --booklet true --paper letter output/compline-2026-09-04.pdf
```

## Deployment

The app runs on Fly.io ([fly.toml](fly.toml)). The [Dockerfile](Dockerfile)
copies a static Go binary and `data/` into a `scratch` image. Maintainers with
Fly access deploy with `fly deploy`.

### Usage metrics

`/admin/usage` is an unlinked, unauthenticated, `noindex` report (excluded from
the service-worker cache) of daily unique browsers, overall and per office,
ordo, and reminder feed, over 7/30/90/366 days. Days are America/New_York.

**What counts.** Visible pages, not completed prayers. A browser counts once
overall and once in each hour it opens. Ordo counts a viewed calendar page;
Reminders counts a *generated* feed link, not a visit to `/reminders`. Home
counts toward the total only. Preloads don't count; offline use and no-JS
browsers are missed; multiple devices or cleared cookies inflate the count.

**Rendering dimensions.** The same beacon reports `appearance:nave|apse` (as
rendered, chosen or inherited) and `screen:desktop|mobile` (mobile means under
the 700px breakpoint or a touch-primary pointer). Office pages also report
`prayer-form:private|deacon|priest`, which measures the prayers read, not the
reader's ordination. Each counts once per browser per day, so a reader who
switches mid-day counts on both sides and a pair can exceed the daily total;
clients with an older cached `app.js` report none, so a pair can also fall
short. The report draws each pair as a day-by-day mix band, since one period
share can't distinguish a steady split from a migration.

Values are stored as `family:value` and are never redefined: add a new key so
an old series ends where its meaning ended. The server only has to understand
the scope; unknown tokens from newer or older cached clients are dropped and
the page still counts.

**Scraping is welcome but never counted.** There is no robots.txt, but a
crawler presents a fresh cookie jar per page and would otherwise mint a
"browser" per URL. Three filters prevent that:

- Only current pages count: the beacon rejects dates outside today ±1 day (±1
  year for the ordo), capping any crawl at about ten pages.
- Nothing reports until the page is touched or visible for eight seconds.
- Self-identifying crawlers (`usage.IsBot`: `<name>bot/<version>` or an
  explicit token, not merely the substring "bot") get `204` with no cookie.

A stealth scraper that dwells on today's pages still counts, and a reader
browsing only the archive is missed.

**Storage.** `OFFICE_USAGE_DB` names the SQLite file; unset, the event
endpoint is a no-op and the report 404s. If SQLite cannot open, the server
logs a warning and serves without metrics; runtime storage errors return 503
only on the metrics endpoints. The parent directory must exist.

```bash
# Production: one replica, volume office_usage in iad mounted at /usage
fly volumes create office_usage --region iad --size 1 --app office
# Local
mkdir -p output/usage
OFFICE_USAGE_DB="$PWD/output/usage/usage.sqlite" go run ./cmd/server serve
```

**Privacy.** The first-party `office-usage` cookie is a random 128-bit ID
(30 days, HttpOnly, SameSite=Strict, Secure over HTTPS, path `/api/usage`).
SQLite stores only a per-day hash, an office category, and a date — no raw
cookie, IP, user agent, URL, or timestamp. Dedupe rows older than the day
before yesterday are deleted on the next event or report read; aggregate counts are kept
indefinitely (volume snapshots may retain older data until they expire).

SQLite runs in WAL mode on one connection: back up with SQLite-aware tools or
volume snapshots, not by copying the database file alone. Multiple replicas
would keep independent counts.
