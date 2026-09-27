#!/usr/bin/env bash
# Compare the data-editing commands (scaffold propers, corpus show/put)
# between the Go and Rust CLIs: each runs the same sequence, error paths
# included, on its own copy of data/, and the output and trees are diffed.
# Stdout and exit statuses are compared; stderr is not (RUST-PORT.md).
# Expects ./office and target/release/office-rs.
set -euo pipefail
go_bin=$(realpath "${GO_OFFICE:-./office}")
rs_bin=$(realpath "${RUST_OFFICE:-target/release/office-rs}")
work=$(mktemp -d)
trap 'rm -rf "${work:?}"' EXIT
scaffolded=$(grep -l '^# \[hymn-terce\]' data/texts/proper/*.txt | head -1 | xargs basename)
scaffolded=${scaffolded%.txt}
for impl in go rs; do
  mkdir -p "$work/$impl"
  cp -r data "$work/$impl/data"
  rm "${work:?}/$impl/data/texts/proper/st-ambrose.txt"
  python3 - "$work/$impl" <<'PY'
import re, sys
root = sys.argv[1]
p = root + '/data/texts/proper/st-lucy.txt'
s = open(p).read()
for key in ('hymn-lauds', 'chapter-prime', 'versicle-none'):
    s = re.sub(r'\n# \[%s\]\n[^\n]*\n' % key, '\n', s)
open(p, 'w').write(s)
open(root + '/body.txt', 'w').write('# SOURCE: old\nO God, who didst * make this test,\ngrant us grace. Amen.\r\n')
PY
done
cp "$go_bin" "$work/go/office"
cp "$rs_bin" "$work/rs/office"
run() {
  for impl in go rs; do
    (cd "$work/$impl" && { ./office "$@" 2>/dev/null || echo "exit $?"; } >> log.txt)
  done
}
run scaffold propers -dry-run
run scaffold propers -check
run scaffold propers -check -dry-run
run scaffold propers -dry-run -feast st-lucy
run scaffold propers -dry-run -feast nope
run scaffold propers -dry-run -include-commemorations
run scaffold bogus
run corpus show psalms/051
run corpus show proper/st-agatha/collect
run corpus show "proper/$scaffolded/hymn-terce"
run corpus show proper/st-lucy/nothing
run corpus show proper/nobody/collect
run corpus show bad/key
run corpus show proper/St-X/collect
run corpus put proper/st-lucy/collect --file body.txt --source "# SOURCE: Monastic Diurnal p. 5"
run corpus put proper/st-agatha/collect --file body.txt --source "source: Monastic Diurnal p. 6"
run corpus put "proper/$scaffolded/hymn-terce" --file body.txt --source Diurnal
run corpus put psalms/051 --file body.txt --source Coverdale
run corpus put proper/st-lucy/collect --file missing.txt --source x
run corpus put proper/st-lucy/collect --file body.txt --source ""
run corpus show proper/st-agatha/collect
run corpus show "proper/$scaffolded/hymn-terce"
run scaffold propers -include-commemorations
run scaffold propers -check
if diff -u "$work/go/log.txt" "$work/rs/log.txt" >&2 && diff -r "$work/go/data" "$work/rs/data" >&2; then
  echo "parity: scaffold and corpus editing identical ($(grep -c . "$work/go/log.txt") output lines)"
else
  echo "parity: scaffold and corpus editing DIFFER" >&2
  exit 1
fi
