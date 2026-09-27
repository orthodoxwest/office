# Rust coverage and mutation testing

Coverage shows which code ran. Mutation testing checks whether assertions detect
changed behavior. Neither substitutes for cited rubric expectations or review.

## Coverage

The **Rust coverage** CI job uses `cargo-llvm-cov` 0.8.7 and uploads LCOV output
for 14 days. This measures Rust line coverage across the workspace. The retired
Go package statement floors are not comparable; set new per-crate floors only
after measuring the Rust suite. The normal Rust, golden, validation and browser
checks remain required regardless of coverage.

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

The manually dispatched **Mutation review** workflow takes `calendar`, `office`,
`corpus` or `liturgy`. It uses one worker, a 60-second per-test timeout and a
20-minute overall budget; evidence is uploaded from `output/mutation/` for 14
days. Mutants are tested with their crate's tests. Use `--test-workspace` locally
when investigating coverage supplied by consumers in other crates.

Inspect missed mutants and timeouts individually. A timeout is not evidence
that an assertion caught a defect, and incomplete runs establish no score.
Keep tests about meaningful behavior and boundaries, not implementation spelling.
Ignored output directories are excluded from scratch copies; source PDFs and
page-image caches must never enter a mutation artifact or Git.
