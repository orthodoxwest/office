# Mutation testing

Line coverage says a line *ran* during tests. Mutation testing asks whether any
test would have *noticed* if that line were wrong. It alters one expression at a
time — flipping `>=` to `>`, negating a condition, changing `+` to `-` — then
re-runs the tests. If the tests still pass, the mutant "lived": that logic is
executed but unverified.

This matters here because the calendar and office engines are dense conditional
logic implementing rubrics, and a silently wrong boundary produces a plausible
but incorrect office rather than a crash.

## Running it

```bash
make mutate                              # calendar, office, texts (~4 min per package)
make mutate MUTATE_PKGS=./internal/calendar/   # one package
make mutate-diff                         # only lines your branch changes vs master
```

`make mutate-diff` is what CI runs on every PR. It is **reporting only** — the
thresholds in `.gremlins.yaml` are 0, so it never fails the build. It posts a
comment listing surviving mutants on lines the PR touched.

The tool is [gremlins](https://github.com/go-gremlins/gremlins), pinned in the
Makefile and installed on demand into the configured `GOBIN` or `GOPATH/bin`.

## Three traps

**The timeout coefficient must match the baseline.** Gremlins derives each
mutant's timeout as `coefficient × baseline test run`, and the baseline differs
sharply between the two modes:

- `make mutate` (one package) measures ~1s, so the *default* coefficient yields
  a timeout shorter than a cold compile inside gremlins' temporary module copy.
  Every mutant then reports `TIMED OUT` and gremlins prints
  **`Test efficacy: 100.00%`** — a perfect score from a run that verified
  nothing. The first calendar run failed exactly this way: 638 of 643 mutants
  "timed out". Hence `timeout-coefficient: 30` in `.gremlins.yaml`, where the
  initial corrected run reported the credible 79.4% baseline recorded below.
- `make mutate-diff` measures the *whole suite* (~30s), so that same
  coefficient would mean a ~15-minute timeout per mutant. The target overrides
  it to `MUTATE_DIFF_COEFFICIENT` (5), and the CI job carries a
  `timeout-minutes` bound in case a loop-counter mutant spins.

If you see a wall of `TIMED OUT`, raise the coefficient and ignore the efficacy
figure printed alongside it. Note also that gremlins does not pass `-count=1`,
so a warm test cache shrinks the baseline and therefore the timeout.

**`--diff` only works from the module root.** Passing a package path together
with `--diff` makes gremlins skip every mutant, including the changed ones, and
report `0.00%` with no error. `make mutate-diff` deliberately passes no path.

**The Go build cache must be writable.** Gremlins treats any test exit status 1
as a killed mutant, including failures caused by the execution environment. In
a managed sandbox where the default cache is read-only, this produced a false
`99.84%` calendar result. Point `GOCACHE` at a writable location for the run:

```bash
GOCACHE=/tmp/office-gremlins-cache make mutate MUTATE_PKGS=./internal/calendar/
```

If efficacy changes implausibly without corresponding test changes, run the
package tests with the same environment and inspect infrastructure failures
before trusting the mutation report.

## Reading the results

Not every surviving mutant is a missing test. Roughly a third here are
*equivalent mutants* — the mutated program is semantically identical to the
original, so no test can kill them. The common shape in this codebase:

```go
if aKey[1] != bKey[1] {
    return decision("occurrence:temporal-tiebreak", aKey[1] > bKey[1])
}
```

Mutating `>` to `>=` changes nothing, because the guard already excludes
equality. Same for an `idx < 0` check where `idx == 0` is unreachable. Recognise
these and move on; don't contort a test to kill one.

The mutants worth acting on are the ones where you can state the bug in rubric
terms: "if this boundary were off by one, the suffrage would be wrong on
January 13."

## Original baseline

First full run, retained as the historical ratchet point:

| Package | Mutants | Killed | Lived | Efficacy |
|---|---|---|---|---|
| `internal/calendar` | 643 | 504 | 131 | 79.4% |
| `internal/office` | 588 | 446 | 104 | 81.1% |
| `internal/texts` | 114 | 92 | 22 | 80.7% |

The first follow-up closed the Epiphany-octave suffrage, Easter octave-parent,
Circumcision concurrence, and malformed `DateRule` gaps that were originally
listed here. The feria concurrence finding proved to be unreachable code and
was removed. Do not use an old copy of that list as the current work queue.

Most recent trustworthy full-package results:

| Package | Killed | Lived | Efficacy |
|---|---:|---:|---:|
| `internal/calendar` | 559 | 92 | 85.87% |
| `internal/office` | 450 | 100 | 81.82% |
| `internal/texts` | 92 | 22 | 80.7% |

## Current focus

Counts are triage candidates, not a test quota: equivalent mutants must still
be removed from the queue by reasoning about reachable states.

| Package and area | Surviving mutants | Next question |
|---|---:|---|
| `calendar/builder.go` octave generation | 16 | Do generated day numbers, ranks, and proper-set indices hold at Sunday boundaries? |
| `office/engine.go` uniform-antiphon collapse | 18 | Do groups of two, three, and multiple adjacent psalm sections retain exactly the framing antiphons? |
| `office/proper.go` Greater Antiphons | 11 | Are December 17/23 and first-Vespers date shifts pinned on both sides? |

The anticipated/resumed Sunday arithmetic is no longer in this table. Concrete
years now cover zero, one, and two autumn resumptions; equality at the
anticipation cutoff; Epiphany falling on Sunday; and rejection of a seventh
displaced Sunday. Those tests killed 29 of 32 surviving mutants in the cluster.
Two remaining mutations replace `6-surplus` with a larger cutoff; valid
computed years in the supported 2026–2053 parity range never distinguish that
expression because anticipatable cases land on or below the equality boundary
and all later cases already exceed six. The third changes `len(feasts) > 0` to
`>= 0` before rewriting the final Sunday; every valid computed year produces a
non-empty series, so that boundary is likewise equivalent.
