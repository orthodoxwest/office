#!/usr/bin/env bash
# Prints the comma-separated sample dates PR CI compares hour by hour
# (RUST-PORT.md, "Phases"): every date with a checked-in hour golden, plus
# fixed boundary dates the goldens do not reach.
set -euo pipefail

boundary=(
  2025-12-31 2026-01-01 2026-01-05 2026-02-01 2026-02-02 2026-02-07
  2026-03-24 2026-04-01 2026-04-02 2026-04-03 2026-04-04 2026-04-18
  2026-05-13 2026-05-14 2026-05-23 2026-05-30 2026-06-05 2026-10-31
  2026-11-01 2026-11-02 2026-12-17 2026-12-21 2026-12-23 2026-12-24
  2026-12-31 2027-01-01 2027-11-01 2027-11-02
  2028-02-24 2028-02-28 2028-02-29 2028-03-01 2029-11-01 2029-11-02
  2030-04-20 2031-12-25 2035-04-29 2036-01-06 2038-02-27 2053-12-31
)
golden=$(ls internal/e2e/testdata/golden | sed -nE 's/^[a-z]+-([0-9]{4}-[0-9]{2}-[0-9]{2})\.txt$/\1/p')
printf '%s\n' "${boundary[@]}" $golden | sort -u | paste -sd, -
