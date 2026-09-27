# Rust coverage and mutation testing

Coverage and mutation testing are optional local diagnostics for a specific
question, such as an untested error path or a suspected calendar boundary gap.
Use source-cited rubric assertions, synthetic boundary tests, regression snapshots
and browser checks for routine validation. There are no coverage-percentage
targets or planned coverage floors.

## Coverage

Use coverage when locating code that the current tests do not exercise. It shows
which lines ran, not whether the assertions protect the intended behavior.
Reports stay locally under `output/coverage/`.

```sh
rustup component add llvm-tools-preview
cargo install cargo-llvm-cov --version 0.8.7 --locked
make test-coverage
cargo llvm-cov report --html --output-dir output/coverage/html
```

## Mutation review

Install [cargo-mutants](https://mutants.rs/) 27.1.0, then select a crate:

```sh
cargo install cargo-mutants --version 27.1.0 --locked
make mutate MUTATE_PKG=calendar
make mutate-diff MUTATE_PKG=office MUTATE_DIFF_BASE=master
```

Run mutation testing when investigating assertion quality in a selected crate.
The local commands use one worker and a 60-second per-test timeout, with results
under `output/mutation/`. Mutants are tested with their crate's tests. Use
`--test-workspace` when investigating coverage supplied by consumers in other
crates. Neither coverage nor mutation testing runs in GitHub Actions.

Inspect missed mutants and timeouts individually. A timeout is not evidence
that an assertion caught a defect, and incomplete runs establish no score.
Keep tests about meaningful behavior and boundaries, not implementation spelling.
Ignored output directories are excluded from scratch copies; source PDFs and
page-image caches must never enter a mutation artifact or Git.
