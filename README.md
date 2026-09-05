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
./office ordo 2026                # annual ordo
./office rubrics 2026             # per-day rubric and antiphon TSV
./office validate                # check data files
./office audit                   # report missing texts and fallback coverage
./office lint                    # mechanical and advisory corpus findings
./office corpus show proper/st-andrew/collect
./office review provenance       # source coverage
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
