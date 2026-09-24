# Mutation testing

Line coverage says a line *ran*. Mutation testing asks whether any test would
*notice* if it were wrong: it alters one expression at a time (`>=` to `>`,
negating a condition, `+` to `-`) and re-runs the tests. A mutant that still
passes "lived" — that logic is executed but unverified. The calendar and office
engines are dense rubric conditionals where an off-by-one produces a plausible
but wrong office rather than a crash, so this matters here.

## Routine CI: unit-test coverage

The **Check** job runs `make test-coverage`: every Go unit-test package once
(`-count=1 -covermode=set`), with per-package statement floors from
`scripts/coverage-floors.json`. The profile is uploaded as
`output/coverage/unit.out` for 14 days. `-coverpkg=./...` is deliberately not
used, so golden tests and other packages' tests don't count toward a package.

Floors are regression baselines, not claims of adequacy. Raise one when tests
improve, add one for each new internal package, and explain any reduction in
the PR. Packages are checked independently, so gains in one cannot offset a
regression in another.

Coverage only proves code ran; deleting an assertion keeps coverage intact. For
calendar and office changes, state the boundary or rubric behavior in the test
and show that it fails when the behavior breaks.

```bash
make test-coverage
go tool cover -html=output/coverage/unit.out   # browse uncovered code
```

## Running mutation investigations

Mutation testing is on demand only; routine CI does not run it.

```bash
make mutate                                    # models, calendar, office, texts
make mutate MUTATE_PKGS=./internal/calendar/   # one package
make mutate-diff                               # only lines changed vs master
```

In Actions, **Mutation review** takes a branch and one of `models`, `calendar`,
`office`, or `texts`, and uploads a `mutation-review` artifact (log and JSON
report) for 14 days. It uses one worker and a 20-minute budget; large packages
may need a longer local run. An incomplete run shows as failed, not as a score.

Scores are informational: inspect individual survivors and timeouts. The tool
is [gremlins](https://github.com/go-gremlins/gremlins), pinned in the Makefile
and installed on demand into `GOBIN` or `GOPATH/bin`.

## Three traps

**The timeout coefficient must match the baseline.** Each mutant's timeout is
`coefficient × baseline test run`.

- `make mutate` measures one package (~1s), so the default coefficient is
  shorter than a cold compile. Every mutant then reports `TIMED OUT` and
  gremlins prints **`Test efficacy: 100.00%`** for a run that verified nothing.
  Hence `timeout-coefficient: 30` in `.gremlins.yaml`.
- `make mutate-diff` measures the whole suite (~30s), where 30 would mean
  ~15 minutes per mutant, so it uses `MUTATE_DIFF_COEFFICIENT` (5).

A wall of `TIMED OUT` means raise the coefficient and ignore the efficacy
figure. Gremlins doesn't pass `-count=1`, so a warm test cache also shrinks the
timeout.

**`--diff` only works from the module root.** With a package path, gremlins
skips every mutant and reports `0.00%` without error. `make mutate-diff`
passes no path.

**The Go build cache must be writable.** Gremlins counts any test exit status 1
as a kill, including environment failures; a read-only sandbox cache once
produced a false 99.84%. Point `GOCACHE` somewhere writable:

```bash
GOCACHE=/tmp/office-gremlins-cache make mutate MUTATE_PKGS=./internal/calendar/
```

If efficacy moves without test changes, run the package tests in the same
environment before trusting the report.

## Reading the results

Roughly a third of survivors are *equivalent mutants* that no test can kill:

```go
if aKey[1] != bKey[1] {
    return decision("occurrence:temporal-tiebreak", aKey[1] > bKey[1])
}
```

Mutating `>` to `>=` changes nothing because the guard already excludes
equality. Recognise these and move on; don't contort a test to kill one. The
mutants worth acting on are those you can state as a rubric bug: "if this
boundary were off by one, the suffrage would be wrong on January 13."

Known equivalent survivors, so they aren't re-investigated:

- Anticipated/resumed Sundays: replacing `6-surplus` with a larger cutoff, and
  `len(feasts) > 0` → `>= 0` before rewriting the final Sunday. No valid year in
  the 2026–2053 parity range distinguishes them.
- Uniform-antiphon collapse (`engine.go`): `lo < len(ants)` → `<=` and
  `len(drop) > 0` → `>= 0` each add a no-op pass.
- Loop-control mutations that stop a loop advancing (the collapse loop,
  octave proper-set numbering) time out rather than survive.
