# Rust port: completed cutover

The application, CLI, deployment image, golden updater and CI now run without
Go. The cutover was merged in #456; reverting that PR and redeploying remains
the rollback procedure. No tagged rollback build or nightly parity job is needed.

The removal follow-up relocates web assets to `apps/office-web/static/` and
fixtures to `tests/fixtures/`. It retires the Go implementation, Go-only
coverage/Gremlins tooling, old import scripts and differential harness.
`make golden` now uses the Rust CLI; ordinary PR CI checks the full 2026–2053
calendar/composition digest against the retained baseline. Rendered hours,
ordos, audit and assurance snapshots retain their original bytes.

Rust tests retain the corpus validation fixtures, cited composition requirements,
calendar/office structural sweeps, synthetic precedence and commemoration cases,
concurrence and proper-resolution tables, antiphon grouping, collect chains,
and Martyrology rollover boundaries. The suites consolidate overlapping Go
cases rather than preserving a one-to-one test-function inventory. Python
retains the shared static JS/CSS contracts; Playwright retains browser behavior
and visual baselines. Rust line coverage replaces the Go statement report;
new per-crate floors need a measured Rust baseline, not copied Go percentages.

The read-only psalm verifier and deterministic asset generators now use Python.
The old fetch/split/Divinum seed programs are retired: corpus ingestion uses
`scripts/DIURNAL-PIPELINE.md`. The generators do not change existing assets.

## Remaining cleanup

- Retain CSV, corpus parsing, query/cookie, template escaping and reminder DST
  behavior until focused contract tests justify any changes. They determine
  accepted data, saved links, rendered UI or reminder instants.
- Replace empty-string sentinels with domain types where absence is meaningful;
  review `PORT(inherited)` annotations individually rather than reproducing
  defects merely because the reference did.
- Keep strengthening synthetic tests as rubric rules change. The large snapshot
  detects changed output; it does not establish that every inherited rule is right.
- Consider renderer/parser simplification and mobile adapters independently of
  the server cutover. Core crates remain free of filesystem, network and clock IO
  and cross-compile for iOS and Android in CI.

The completed phase-by-phase migration record and retired comparison commands
are preserved in Git history before the Go removal.
