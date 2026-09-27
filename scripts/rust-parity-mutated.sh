#!/usr/bin/env bash
# Compare validate, lint, and audit between the Go and Rust CLIs on a copy of
# data/ broken in known ways. The live data is clean, so most finding paths
# (placeholders, missing propers, flat antiphons, lint classes, orphan chant
# scores, not-found markers, ordinary fallbacks) are only reached this way.
# Expects ./office and target/release/office-rs.
set -euo pipefail
go_bin=$(realpath "${GO_OFFICE:-./office}")
rs_bin=$(realpath "${RUST_OFFICE:-target/release/office-rs}")
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
cp -r data "$work/data"
# Both CLIs find data/ beside their executable.
cp "$go_bin" "$work/office"
cp "$rs_bin" "$work/office-rs"
python3 - "$work/data" <<'PY'
import glob, os, re, sys
os.chdir(sys.argv[1])
open('audit-ok.txt', 'w').write('# trimmed\nst-joseph *\n* hymn\nst-mark collect benedictus-antiphon\n')
targets = set()
for f in glob.glob('texts/**/*.txt', recursive=True):
    for m in re.finditer(r'@use\s+proper/([^/\s]+)/', open(f).read()):
        targets.add(m.group(1))
for i, f in enumerate(sorted(glob.glob('texts/proper/*.txt'))):
    pid = os.path.basename(f)[:-4]
    if i % 7 == 0 and pid not in targets and not pid.endswith('-paschal'):
        os.remove(f)
kept = []
for line in open('collect-conclusions.txt').read().split('\n'):
    m = re.search(r'proper/([^/\s]+)/', line)
    if m and not line.lstrip().startswith('#') and not os.path.exists('texts/proper/%s.txt' % m.group(1)):
        continue
    kept.append(line)
open('collect-conclusions.txt', 'w').write('\n'.join(kept))
p = 'texts/ordinary/lauds.txt'
s = open(p).read().replace('\n[chapter]\n', '\n[chapter]\nPlaceholder chapter\ttext `x` \\n ** Domine deus  end \n\n[chapter-was]\n', 1)
open(p, 'w').write(s)
for i, f in enumerate(sorted(glob.glob('texts/proper/*.txt'))[:40]):
    s = open(f).read()
    if i % 5 == 0: s = s.replace('the ', 'the  ', 1)
    if i % 5 == 1: s = s.replace('.', '. ', 1)
    if i % 5 == 2: s = s.replace("'", '`', 1)
    if i % 5 == 3: s += '\n[latin-x]\nDomine deus, sancti nobis\n'
    if i % 5 == 4 and '[psalm-antiphon-1]' not in s:
        s += ''.join('\n[psalm-antiphon-%d]\nPer omnia saecula\n' % k for k in range(1, 6))
    open(f, 'w').write(s)
os.makedirs('texts/chant/canticles', exist_ok=True)
open('texts/chant/canticles/nothing.gabc', 'w').write('x\n')
open('texts/chant/psalms/999.gabc', 'w').write('y\n')
PY
failed=0
for cmd in validate lint "audit -year 2026" "audit -year 2031"; do
  # shellcheck disable=SC2086
  if cmp -s <(cd "$work" && ./office $cmd 2>&1; echo "exit $?") <(cd "$work" && ./office-rs $cmd 2>&1; echo "exit $?"); then
    echo "parity: mutated data: office $cmd identical"
  else
    failed=1
    echo "parity: mutated data: office $cmd DIFFERS" >&2
    # shellcheck disable=SC2086
    diff -u <(cd "$work" && ./office $cmd 2>&1) <(cd "$work" && ./office-rs $cmd 2>&1) | head -40 >&2 || true
  fi
done
exit $failed
