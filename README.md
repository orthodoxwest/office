# AWRV Benedictine Divine Office

[Pray the Office](https://orthodoxwestbreviary.com): the English Benedictine daily office for
Antiochian Western Rite Vicariate use, with each hour's prayers, psalms and propers assembled on
one page.

Lauds, Prime, Terce, Sext, None, Vespers and Compline are supported; Matins is not yet included.
The calendar uses the Julian paschalion and pre-1962 feast ranks, with AWRV observances and the
Coverdale Psalter. The newest local archdiocesan ordo governs current-year practice.

One Rust engine serves three fronts that stay in step:

- **Web**: the hours, a browsable ordo, an installable offline PWA and a subscribable reminder
  calendar at `/reminders`.
- **Android** ([apps/android](apps/android/README.md)) and **iOS** ([apps/ios](apps/ios/README.md)):
  native apps with the engine and corpus built in, so they work offline.

Each hour offers a **Prayer form**: praying privately (the default, also for clergy alone), or
with others led by a deacon or a priest. A `?form=private|deacon|priest` link overrides the
remembered choice for that visit. Each hour ends with a link to report a problem.

## Run locally

Install Rust 1.94.1 (the version CI uses; the deployment image builds with 1.99.1), a C compiler for bundled
SQLite, and Make. On Windows, use WSL.

```bash
git clone https://github.com/orthodoxwest/office.git
cd office
make serve
```

Open [localhost:8080](http://localhost:8080). Data loads at startup, so restart after editing it.

## Contribute texts or review

Liturgical data is plain text under [data/](data/), so you can propose edits in GitHub's file
editor without installing anything. Cite the source and page or section for each correction.

- [Editing liturgical data](DATA.md): file formats, proper slots, aliases, scaffolding and psalm
  numbering.
- [Reviewing the Office](REVIEWING.md): checking texts and rendered hours, composition checks and
  text provenance.
- [Diurnal transcription and discovery](scripts/DIURNAL-PIPELINE.md): reading scanned pages and
  finding propers hidden by fallbacks.
- [The Martyrology at Prime](MARTYROLOGY.md).

## Common commands

`make help` lists every target. After `make build`:

```bash
./office lauds 2026-09-04                # render an hour (also prime … compline)
./office prime 2026-03-11 --form deacon  # another prayer form
./office ordo 2026                       # annual ordo
./office rubrics 2026                    # per-day rubric and antiphon TSV
./office validate                        # check data files
./office audit                           # missing texts and fallback coverage
./office lint                            # corpus findings
./office corpus show proper/st-andrew/collect
./office review explain lauds 2026-09-04 # why an hour is composed as it is
make project-status YEAR=2026            # proper, assurance and ordo reports
```

## Development

The check suite needs Rust, Make, Python 3 and Node.js 20.19+ (CI uses 22):

```bash
npm ci --prefix .web-tools
make check        # formatting, Clippy, JS lint, tests, data validation, text lint
make test         # Rust and Python tests, including the golden snapshots
make golden       # regenerate snapshots after an intentional change
make parity       # full 2026–2053 calendar and composition snapshot (also in CI)
```

Review changes under [tests/fixtures/golden/](tests/fixtures/golden/) before committing; the
`update-golden` PR label regenerates them in CI. Coverage and mutation testing are in
[MUTATION-TESTING.md](MUTATION-TESTING.md).

Playwright behavior and accessibility tests run separately:

```bash
npm --prefix .web-tools run install:browser
make test-ux
```

Visual snapshots run only in CI's pinned browser container. For an intentional visual change,
inspect the Playwright report and add the `update-ux-snapshots` PR label, which commits the
regenerated snapshots ([workflow](.github/workflows/update-ux-snapshots.yml)).

The native apps build with `make android` and `make ios` (macOS); their READMEs give the
toolchains.

Bundled SQLite dominates a cold build. To share it across worktrees, install sccache and set
`CC = "sccache cc"` under `[env]` in `~/.cargo/config.toml` (not as `rustc-wrapper`, whose keys
include the `target/` path). `make sweep-targets` deletes `target/` in checkouts idle for
`SWEEP_HOURS` (default 24).

## PDF booklets

Half-letter (5.5 × 8.5 inch) booklets, via LuaLaTeX:

```bash
make pdf HOUR=lauds DATE=2026-09-04
make pdf HOUR=compline CHANT=1     # engrave available GABC scores
pdfjam --booklet true --paper letter output/lauds-2026-09-04.pdf   # saddle-stitch imposition
```

This needs LuaLaTeX, GregorioTeX, EB Garamond and Noto Sans Symbols (the symbol-font path assumes
Debian/Ubuntu; see the [template](crates/render-tex/src/preamble.tex)). `./office tex [--chant]
HOUR` emits the TeX alone.

## Deployment

The site runs on Fly.io; maintainers deploy with `fly deploy`. Hosting, the canonical host,
search, and the usage metrics with their privacy rules are in [DEPLOYMENT.md](DEPLOYMENT.md).
