#!/usr/bin/env bash
# Compare one command's output between the Go and Rust CLIs:
#   scripts/rust-parity-cmd.sh ordo 2026
# Expects ./office and target/release/office-rs; shows a unified diff on a mismatch.
set -euo pipefail
go_bin=${GO_OFFICE:-./office}
rs_bin=${RUST_OFFICE:-target/release/office-rs}
if cmp -s <("$go_bin" "$@") <("$rs_bin" "$@"); then
  echo "parity: office $* identical"
  exit 0
fi
echo "parity: office $* DIFFERS" >&2
diff -u <("$go_bin" "$@") <("$rs_bin" "$@") | head -40 >&2 || true
exit 1
