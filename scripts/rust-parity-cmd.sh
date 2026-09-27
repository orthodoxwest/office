#!/usr/bin/env bash
# Compare one command between the Go and Rust CLIs:
#   scripts/rust-parity-cmd.sh ordo 2026
# Stdout and the exit status must match; stderr is not compared, since error
# wording is not part of the parity contract (RUST-PORT.md). Expects output/office-go
# and target/release/office; shows a unified diff on a mismatch.
set -euo pipefail
go_bin=${GO_OFFICE:-output/office-go}
rs_bin=${RUST_OFFICE:-target/release/office}
out=$(mktemp -d)
trap 'rm -rf "${out:?}"' EXIT
go_rc=0
rs_rc=0
"$go_bin" "$@" > "$out/go" 2> "$out/go.err" || go_rc=$?
"$rs_bin" "$@" > "$out/rs" 2> "$out/rs.err" || rs_rc=$?
if [ "$go_rc" = "$rs_rc" ] && cmp -s "$out/go" "$out/rs"; then
  echo "parity: office $* identical (exit $go_rc)"
  exit 0
fi
echo "parity: office $* DIFFERS (exit go $go_rc, rust $rs_rc)" >&2
diff -u "$out/go" "$out/rs" | head -40 >&2 || true
head -5 "$out/go.err" "$out/rs.err" >&2 || true
exit 1
