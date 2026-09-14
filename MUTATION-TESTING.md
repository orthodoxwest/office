# Mutation testing

Line coverage says a line *ran* during tests. Mutation testing asks whether any
test would have *noticed* if that line were wrong. It alters one expression at a
time — flipping `>=` to `>`, negating a condition, changing `+` to `-` — then
re-runs the tests. If the tests still pass, the mutant "lived": that logic is
executed but unverified.

This matters here because the calendar and office engines are dense conditional
logic implementing rubrics, and a silently wrong boundary produces a plausible
but incorrect office rather than a crash.

## Routine CI: unit-test coverage

PRs and pushes run `make test-coverage` in the existing **Check** job. It runs
all Go unit-test packages once with `-count=1 -covermode=set`, checks per-package
statement coverage, and uploads `output/coverage/unit.out` for 14 days. The job
summary lists covered/total statements and each floor. Golden tests still run
separately; neither their executions nor other packages' tests contribute to a
package's unit-test coverage (`-coverpkg=./...` is deliberately not used).

The floors in `scripts/coverage-floors.json` cover every currently tested
internal package. They start at the measured percentage rounded **down** to
one decimal place on commit `95326da`. These are regression baselines, not
claims of adequate coverage. Raise a floor when tests improve; add a floor when
introducing an internal package, and explain any intentional reduction in the
PR. The checker compares exact statement ratios before rounding for display
and rejects missing packages or malformed reports. Improved coverage in one
package cannot compensate for a regression in another.

Coverage only proves code was exercised. Removing an assertion while keeping
its execution can pass this gate. Behavioral tests, review of changed
assertions, and focused mutation investigations remain necessary to protect
what tests actually verify. For calendar and office changes, describe the
boundary or rubric behavior in the test and demonstrate that the test fails
when that behavior is broken.

```bash
make test-coverage
# Inspect uncovered code in a browser:
go tool cover -html=output/coverage/unit.out
```

## Running mutation investigations

```bash
make mutate                              # models, calendar, office, texts; report only
make mutate MUTATE_PKGS=./internal/calendar/   # one package
make mutate-diff                         # only lines your branch changes vs master
```

Mutation testing is on demand. In Actions, run **Mutation review**, choose a
branch and one of `models`, `calendar`, `office`, or `texts`, then inspect the
`mutation-review` artifact (log and JSON report if completed). It uses one
worker to limit simultaneous runaway mutants, a 20-minute command budget,
and a 25-minute job limit. Large packages may require a longer local run.
A failed or incomplete run is visible as a failed investigation, not a passing
score. Reports are retained for 14 days. There are no scheduled runs or PR bot
comments, and routine CI does not install Gremlins.

Local commands remain available for full-package and changed-line inspection.
Their scores are informational; inspect the individual survivors and timeouts.

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
  it to `MUTATE_DIFF_COEFFICIENT` (5), so bound long local runs with an external timeout if needed.

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

## Why automatic mutation ratchets were retired

The September 14 investigation found:

- [Master run 34900508837](https://github.com/orthodoxwest/office/actions/runs/34900508837)
  completed Check in 3m42s and UX in 4m58s, while office mutation took 19m33s
  (720 killed, 76 lived, 35 uncovered, plus three timeouts). The
  [preceding PR run](https://github.com/orthodoxwest/office/actions/runs/34900369342)
  spent another 16m30s on office mutation. Any Go change triggered every
  full-package ratchet, and PRs also ran a separate changed-line scan.
- Calendar jobs failed after runner shutdown signals, before producing scores.
  [An earlier run](https://github.com/orthodoxwest/office/actions/runs/34855845561)
  also panicked in Gremlins' signal handling. The logs establish interrupted
  execution, not a test-quality regression; they do not establish why the
  runner shut down.
- The changed-line job claimed to never fail the build, but its job timeout
  could still fail CI. Its comment was only updated when survivors existed,
  leaving stale warnings after a clean run. Full-package JSON reports were
  temporary files deleted by the Make target, including on failure.
- Static percentage floors were manually maintained baselines, not an automatic
  comparison against the PR base. Gremlins v0.6.0 excludes timed-out mutants
  from both score denominators, so timing changes can change the scores without
  any test edits. `models`' historic 100% mutation result also did not imply
  full statement coverage: the new baseline measures 42/91 statements (46.15%).

Routine CI now collects coverage during its existing unit-test run, retains the
profile, and cancels superseded PR runs. The `mutate-ratchet` target and mutation
threshold checker have been removed. Mutation testing remains a useful way to
find missing assertions when a developer can review the evidence.

### Other CI bottlenecks

The same run spent 71s on unit tests, 66s on golden tests, and 44s on the
provenance command, sequentially. **Check**, **Golden tests**, and **Text
provenance** now run as independent jobs. All three still fail CI on errors;
golden mismatch comments and the provenance summary remain available. Go build
cache keys include source hashes and restore from the previous source version,
so new compilations can be saved instead of repeatedly restoring the first
cache created for a module version.

UX previously downloaded and compiled Go dependencies on every run (~26s), then
ran 89 behavior tests on one worker (~197s). It now caches Go dependencies and
builds, and runs behavior tests with two workers. Visual comparisons retain one
worker and the pinned browser container. Behavior and visual reports go into
separate artifact directories so the second invocation cannot erase the first
report or its failure traces. No test cases, golden comparisons, or provenance
checks are skipped by these optimizations.

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

Historical full-package results measured before introducing the former CI
ratchets (these are not the current baseline):

| Package | Killed | Lived | Not covered | Timed out | Efficacy | Mutant coverage |
|---|---:|---:|---:|---:|---:|---:|
| `internal/models` | 3 | 0 | 0 | 0 | 100.00% | 100.00% |
| `internal/calendar` | 572 | 79 | 42 | 3 | 87.86% | 93.94% |
| `internal/office` | 488 | 70 | 38 | 3 | 87.46% | 93.62% |
| `internal/texts` | 92 | 22 | 0 | 0 | 80.70% | 100.00% |

## Focused follow-up status

The initial high-value mutation queue is complete. The historical full-package table above
records the original follow-up work; the focused file-level runs below verify
their target clusters but do not replace a current package measurement.

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
