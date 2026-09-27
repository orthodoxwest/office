#!/usr/bin/env bash
# Compare `office tex` between the Go and Rust CLIs for every hour on each
# sample date (scripts/rust-parity-dates.sh), cycling the prayer form and the
# --chant flag across dates so every variant is exercised. Stdout and the
# exit status are compared; stderr is not (RUST-PORT.md).
# Expects ./office and target/release/office-rs.
set -euo pipefail
go_bin=${GO_OFFICE:-./office}
rs_bin=${RUST_OFFICE:-target/release/office-rs}
dates=${1:-$(scripts/rust-parity-dates.sh)}
variants=("" "--chant" "--form priest" "--chant --form deacon")
checked=0 failed=0 i=0
for date in ${dates//,/ }; do
  # shellcheck disable=SC2206
  flags=(${variants[$((i % ${#variants[@]}))]})
  i=$((i + 1))
  for hour in lauds prime terce sext none vespers compline; do
    checked=$((checked + 1))
    if ! cmp -s <("$go_bin" tex "${flags[@]}" "$hour" "$date" 2>/dev/null; echo "exit $?") <("$rs_bin" tex "${flags[@]}" "$hour" "$date" 2>/dev/null; echo "exit $?"); then
      failed=$((failed + 1))
      echo "parity: office tex ${flags[*]} $hour $date DIFFERS" >&2
      diff -u <("$go_bin" tex "${flags[@]}" "$hour" "$date" 2>/dev/null; echo "exit $?") <("$rs_bin" tex "${flags[@]}" "$hour" "$date" 2>/dev/null; echo "exit $?") | head -20 >&2 || true
    fi
  done
done
if [ "$failed" -gt 0 ]; then
  echo "parity: tex $failed of $checked differ" >&2
  exit 1
fi
echo "parity: tex $checked hour documents identical"
