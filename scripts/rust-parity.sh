#!/usr/bin/env bash
# Compare the Go and Rust engines' dump streams (RUST-PORT.md, "The oracle").
#
#   scripts/rust-parity.sh GROUPS -start YEAR -years N    e.g. calendar -start 1900 -years 300
#   scripts/rust-parity.sh GROUPS -dates D1,D2,...
#
# Expects output/office-go (make go-build) and target/release/office (cargo build
# --release). Streams are piped, never stored; on a mismatch `office dump diff`
# reports the first differences by JSON Pointer.
set -euo pipefail

groups=$1
shift
go_bin=${GO_OFFICE:-output/office-go}
rs_bin=${RUST_OFFICE:-target/release/office}

if cmp -s <("$go_bin" dump -groups "$groups" "$@") <("$rs_bin" dump -groups "$groups" "$@"); then
  echo "parity: ${groups} identical ($*)"
  exit 0
fi
echo "parity: ${groups} DIFFER ($*)" >&2
"$go_bin" dump diff <("$go_bin" dump -groups "$groups" "$@") <("$rs_bin" dump -groups "$groups" "$@") >&2 || true
exit 1
