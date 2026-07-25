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
Makefile and installed on demand into `~/go/bin`.

## Two traps

**The timeout coefficient must match the baseline.** Gremlins derives each
mutant's timeout as `coefficient × baseline test run`, and the baseline differs
sharply between the two modes:

- `make mutate` (one package) measures ~1s, so the *default* coefficient yields
  a timeout shorter than a cold compile inside gremlins' temporary module copy.
  Every mutant then reports `TIMED OUT` and gremlins prints
  **`Test efficacy: 100.00%`** — a perfect score from a run that verified
  nothing. The first calendar run failed exactly this way: 638 of 643 mutants
  "timed out". Hence `timeout-coefficient: 30` in `.gremlins.yaml`, where the
  same run reports its true 79.4%.
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

## Baseline

First full run, for reference when we discuss ratcheting:

| Package | Mutants | Killed | Lived | Efficacy |
|---|---|---|---|---|
| `internal/calendar` | 643 | 504 | 131 | 79.4% |
| `internal/office` | 588 | 446 | 104 | 81.1% |
| `internal/texts` | 114 | 92 | 22 | 80.7% |

Gaps this run surfaced that 88–90% line coverage had hidden:

- `office/preces.go:170` — the Epiphany-octave suffrage window
  (`day >= 7 && day <= 13`). Both boundary mutants lived; no test anywhere
  references January 7 or January 13.
- `calendar/concurrence.go:235` — the Easter Monday/Tuesday octave-parent
  special case. `easter-monday` and `easter-tuesday` appear in no calendar test.
- `calendar/concurrence.go:300–308` — the Circumcision, within-octave, and
  feria first-Vespers commemoration exclusions. `circumcision` is tested only
  for Dec 31 Vespers precedence, never for these branches.
- `calendar/validate.go:176` — nothing asserts that a malformed `DateRule` is
  rejected, though `make validate` is a gate the project relies on.
