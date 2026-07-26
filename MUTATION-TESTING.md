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
make mutate                              # models, calendar, office, texts; report only
make mutate-ratchet                      # enforce models/calendar/office floors
make mutate-ratchet MUTATE_RATCHET=models     # enforce one package's floors
make mutate MUTATE_PKGS=./internal/calendar/   # one package
make mutate-diff                         # only lines your branch changes vs master
```

CI runs full-package ratchets for `models`, `calendar`, and `office` in parallel
on every PR and push to `master`. It also runs `make mutate-diff` on every PR.
The changed-line run is **reporting only** — the global thresholds in
`.gremlins.yaml` are 0 — and posts a comment listing surviving mutants on lines
the PR touched.

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

## Ratchet gates

Both mutation efficacy and mutant coverage are gated. Efficacy measures whether
tests notice changes to logic they execute; mutant coverage measures how much
mutable logic the tests execute at all. Gating only efficacy would allow new
untested logic to be classified as `NOT COVERED` without lowering the score.

| Package | Minimum efficacy | Minimum mutant coverage |
|---|---:|---:|
| `internal/models` | 100% | 100% |
| `internal/calendar` | 86% | 92% |
| `internal/office` | 86% | 92% |

For the three mutable predicates currently in `models`, this requires every
mutant to be both covered and killed. Calendar and office start roughly one
percentage point below their fresh baselines to avoid making small timing
variations into CI flakes.

The package paths and floors live in the Makefile's `MUTATE_RATCHETS` entries.
Raise them after tests improve the full-package result. The Make target checks
Gremlins' JSON output with `scripts/check_mutation_threshold.py`; Gremlins
v0.6.0 does not reliably apply its nested threshold CLI flags. Do not put these
floors in `.gremlins.yaml`: its global values also apply to the reporting-only,
changed-line subset, whose score is not comparable to a whole package.

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

Most recent trustworthy full-package results, measured before introducing the
CI ratchets:

| Package | Killed | Lived | Not covered | Timed out | Efficacy | Mutant coverage |
|---|---:|---:|---:|---:|---:|---:|
| `internal/models` | 3 | 0 | 0 | 0 | 100.00% | 100.00% |
| `internal/calendar` | 572 | 79 | 42 | 3 | 87.86% | 93.94% |
| `internal/office` | 488 | 70 | 38 | 3 | 87.46% | 93.62% |
| `internal/texts` | 92 | 22 | 0 | 0 | 80.70% | 100.00% |

## Focused follow-up status

The initial high-value mutation queue is complete. The full-package table above
remains the most recent directly measured package baseline; the focused
file-level runs below verify their target clusters but do not replace a full
package measurement.

For anticipated/resumed Sunday arithmetic, concrete years now cover zero, one,
and two autumn resumptions; equality at the anticipation cutoff; Epiphany
falling on Sunday; and rejection of a seventh displaced Sunday. Those tests
killed 29 of 32 surviving mutants in the cluster.
Two remaining mutations replace `6-surplus` with a larger cutoff; valid
computed years in the supported 2026–2053 parity range never distinguish that
expression because anticipatable cases land on or below the equality boundary
and all later cases already exceed six. The third changes `len(feasts) > 0` to
`>= 0` before rewriting the final Sunday; every valid computed year produces a
non-empty series, so that boundary is likewise equivalent.

The uniform-antiphon collapse is also complete. Direct tests now cover the
two-versus-three threshold, adjacent psalm-bearing sections, non-psalm section
boundaries, multiple maximal text groups, a later singleton group, element-type
filtering, and both psalms and canticles. In a focused `engine.go` mutation run,
these cases killed 18 previously living mutants: 16 in the collapse loop and
both in its psalmody classifier. The two remaining live mutations change
`lo < len(ants)` to `<=` for an extra no-op loop iteration and
`len(drop) > 0` to `>= 0` for an extra no-op filtering pass. Both preserve the
result. Three loop-control mutations time out because they prevent their loops
from advancing; they are not silent survivors.

Direct octave-generation tests now cover the Easter Monday/Tuesday/Low Sunday
and Pentecost/Trinity exclusions; special and generic octave names; a Sunday
immediately after the parent feast; non-Sunday proper-set numbering; fixed and
moveable dates; and intermediate versus octave-day ranks. They killed all 16
previously living mutants in that cluster. Mutating the proper-set loop from
incrementing to decrementing still times out, as expected for a loop that no
longer advances toward its bound.

Greater Antiphon tests now pin December 17 and 23, reject December 16 and 24,
shift I Vespers to the preceding civil evening, reject the wrong hour, season,
and month, and verify that the date-fixed antiphon replaces Sunday, feria, and
commons texts while yielding to a feast's own proper. They killed all 11
previously living mutants in that cluster.
