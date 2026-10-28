#!/bin/sh
# Builds neuma's browser module at the rev Cargo.toml pins, with only what the web page uses
# (GABC and the Google Fonts EB Garamond's metrics), into apps/office-web/static/neuma.mjs.
# The page lays chant scores out with it at the column's width (static/chant.js). Run it
# (`make neuma-web`) whenever the neuma pin moves; office-web's tests fail until the module's
# first line names the pinned rev.
set -eu

root=$(cd "$(dirname "$0")/.." && pwd)
rev=$(sed -n 's/^neuma = {.*rev = "\([0-9a-f]*\)".*/\1/p' "$root/Cargo.toml")
[ -n "$rev" ] || { echo "no neuma rev in Cargo.toml" >&2; exit 1; }
work="$root/target/neuma-web"
src="$work/neuma"

if [ ! -d "$src/.git" ]; then
  git clone --quiet https://github.com/orthodoxwest/neuma "$src"
fi
git -C "$src" fetch --quiet origin "$rev" 2>/dev/null || git -C "$src" fetch --quiet origin
git -C "$src" checkout --quiet --detach "$rev"

(cd "$src" && CARGO_TARGET_DIR="$src/target" cargo build --locked -p neuma-wasm --target wasm32-unknown-unknown --profile wasm \
  --no-default-features --features font-google)
# neuma's CI optimizes with this binaryen; the module is about 10% larger without it.
if [ ! -x "$work/node_modules/binaryen/bin/wasm-opt" ]; then
  npm install --silent --no-save --prefix "$work" binaryen@132.0.0
fi
node "$src/crates/neuma-wasm/build.mjs" "$work/node_modules/binaryen/bin/wasm-opt"

out="$root/apps/office-web/static/neuma.mjs"
{
  echo "// neuma $rev (GABC, the Google EB Garamond's metrics and a page-supplied table); scripts/build-neuma-web.sh"
  cat "$src/crates/neuma-wasm/dist/neuma.mjs"
} > "$out"
echo "$out: $(wc -c < "$out") bytes"
