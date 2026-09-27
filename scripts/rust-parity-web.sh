#!/usr/bin/env bash
# Phase 5 gate: run the Go and Rust web servers side by side and crawl both
# with the same requests (scripts/rust-parity-web.py). Each server gets its
# own copy of a usage database seeded with a year of counts, so the usage
# report has history to draw. Expects output/office-go and target/release/office.
set -euo pipefail
go_bin=${GO_OFFICE:-output/office-go}
rs_bin=${RUST_OFFICE:-target/release/office}
go_port=${GO_PORT:-18180}
rs_port=${RUST_PORT:-18181}
dates=${1:-$(scripts/rust-parity-dates.sh)}

work=$(mktemp -d)
go_pid="" rs_pid=""
cleanup() {
  [ -n "$go_pid" ] && kill "$go_pid" 2>/dev/null
  [ -n "$rs_pid" ] && kill "$rs_pid" 2>/dev/null
  rm -rf "${work:?}"
}
trap cleanup EXIT

python3 - "$work/usage.db" <<'EOF'
import datetime, random, sqlite3, sys, zoneinfo
db = sqlite3.connect(sys.argv[1])
db.executescript("""
CREATE TABLE totals (day TEXT NOT NULL, scope TEXT NOT NULL, users INTEGER NOT NULL, PRIMARY KEY(day, scope));
CREATE TABLE seen (day TEXT NOT NULL, browser BLOB NOT NULL, scope TEXT NOT NULL, PRIMARY KEY(day, browser, scope));
""")
rng = random.Random(5)
today = datetime.datetime.now(zoneinfo.ZoneInfo("America/New_York")).date()
scopes = ["lauds", "prime", "terce", "sext", "none", "vespers", "compline", "ordo", "reminders",
          "appearance:nave", "appearance:apse", "screen:desktop", "screen:mobile",
          "prayer-form:private", "prayer-form:deacon", "prayer-form:priest", "retired:scope"]
for i in range(1, 400):
    if rng.random() < 0.1:
        continue
    day = (today - datetime.timedelta(days=i)).isoformat()
    site = rng.randint(1, 60)
    db.execute("INSERT INTO totals VALUES (?, 'site', ?)", (day, site))
    for scope in scopes:
        if rng.random() < 0.8:
            db.execute("INSERT INTO totals VALUES (?, ?, ?)", (day, scope, rng.randint(0, site)))
db.commit()
EOF
cp "$work/usage.db" "$work/go.db"
cp "$work/usage.db" "$work/rs.db"

OFFICE_USAGE_DB="$work/go.db" "$go_bin" serve ":$go_port" 2>"$work/go.log" &
go_pid=$!
OFFICE_USAGE_DB="$work/rs.db" "$rs_bin" serve ":$rs_port" 2>"$work/rs.log" &
rs_pid=$!

ready() { curl -s -o /dev/null "http://127.0.0.1:$1/sw.js"; }
for _ in $(seq 1 120); do
  if ready "$go_port" && ready "$rs_port"; then break; fi
  if ! kill -0 "$go_pid" 2>/dev/null || ! kill -0 "$rs_pid" 2>/dev/null; then break; fi
  python3 -c 'import time; time.sleep(0.5)'
done
if ! ready "$go_port" || ! ready "$rs_port"; then
  echo "parity: web servers did not start" >&2
  cat "$work/go.log" "$work/rs.log" >&2
  exit 1
fi

python3 scripts/rust-parity-web.py "http://127.0.0.1:$go_port" "http://127.0.0.1:$rs_port" "$dates"
