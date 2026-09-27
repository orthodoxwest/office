#!/usr/bin/env bash
# Compare the review subcommands between the Go and Rust CLIs: the read-only
# reports on the live data (one-year sweeps, to keep CI fast; the nightly job
# checks the 28-year assurance report against its golden), and the ledger
# writers (flag, attest, assurance -update-baseline) on two copies of data/,
# comparing their output and the resulting trees.
# Stdout and exit statuses are compared; stderr is not (RUST-PORT.md).
# Expects ./office and target/release/office-rs.
set -euo pipefail
go_bin=$(realpath "${GO_OFFICE:-./office}")
rs_bin=$(realpath "${RUST_OFFICE:-target/release/office-rs}")
year=2026
for cmd in \
  "review provenance -csv" \
  "review provenance -start $year" \
  "review explain lauds 2026-03-11" \
  "review explain vespers 2026-12-24 --form priest" \
  "review explain compline 2026-04-04" \
  "review manifest -start $year" \
  "review provenance-queue -start $year" \
  "review provenance-queue -start $year -summary -include-verified" \
  "review provenance-queue -start $year -suspect-only" \
  "review zero-occurrences -start $year" \
  "review zero-occurrences -start $year -summary" \
  "review resolution-inventory -start $year -years 1" \
  "review resolution-inventory -start $year -years 1 -json -fallback-only" \
  "review resolution-inventory -start $year -years 1 -summary" \
  "review plan -start $year -years 2 -include-sources"; do
  # shellcheck disable=SC2086
  scripts/rust-parity-cmd.sh $cmd
done

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
for impl in go rs; do
  mkdir -p "$work/$impl"
  cp -r data "$work/$impl/data"
  printf '{\n  "start_year": %s,\n  "years": 1,\n  "verified_minimum": 999999\n}\n' "$year" > "$work/$impl/data/review/assurance-baseline.json"
done
cp "$go_bin" "$work/go/office"
cp "$rs_bin" "$work/rs/office"
keys=$("$go_bin" review provenance -csv | awk -F, 'NR > 1 && $5 == "needs-review" { print $1 }' | sed -n '3p;9p')
k1=$(echo "$keys" | head -1)
k2=$(echo "$keys" | tail -1)
run() {
  for impl in go rs; do
    (cd "$work/$impl" && { ./office "$@" 2>/dev/null || echo "exit $?"; } >> log.txt)
  done
}
run review flag --reason "suspect wording, it's \"odd\"" --severity medium --flagged 2026-09 "$k1"
run review flag --reason again "$k1"
run review flag --reason again --replace --issue 42 "$k1"
run review flag --reason x --severity low "$k2"
run review attest --source "Monastic Diurnal" --page 12 --date 2026-09-01 "$k1" "B. W."
run review attest --source "Monastic Diurnal" --page 12 --date 2026-09-01 "$k1" "B. W."
run review attest --source "Monastic Diurnal" --locator "Lauds, Monday" --note "checked, with comma" --date 2026-09-02 --replace "$k1" reviewer
run review attest --source D "$k2" r
run review attest --source D --page 3 --date 2026-02-30 "$k2" r
run review attest --source D --page 3 --date 2026-02-28 nope/key r
run review flag --reason after --flagged 2026-09 "$k2"
run review assurance
run review assurance -update-baseline -markdown
if diff -u "$work/go/log.txt" "$work/rs/log.txt" >&2 && diff -r "$work/go/data" "$work/rs/data" >&2; then
  echo "parity: review ledger writers identical"
else
  echo "parity: review ledger writers DIFFER" >&2
  exit 1
fi
