#!/usr/bin/env bash
# Builds the Rust core for the iOS app (macOS with Xcode only):
#
#   build/OfficeCore.xcframework   the engine and embedded corpus, for iPhone and the simulator
#   build/generated/office_mobile.swift   the UniFFI bindings, compiled into the app
#
# The Swift is generated from the host library, as Android's Kotlin is: the bindings are the
# same for every target. Run from anywhere; `xcodegen generate` then reads build/ from here.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
root="$(cd "$here/../.." && pwd)"
out="$here/build"
targets=(aarch64-apple-ios aarch64-apple-ios-sim)
export IPHONEOS_DEPLOYMENT_TARGET=17.0

cd "$root"
rm -rf "$out"
mkdir -p "$out/generated" "$out/include"

# The static library, for each target. The crate's own crate-type serves Android's JNI loader;
# iOS links the same library statically.
for t in "${targets[@]}"; do
  cargo rustc --locked --release -p mobile-ffi --lib --target "$t" --crate-type staticlib
done

# The bindings, from the host build.
cargo build --locked --release -p mobile-ffi --lib
cargo run --locked --release -p mobile-ffi --features bindgen --bin uniffi-bindgen -- \
  generate --library target/release/liboffice_mobile.dylib --language swift --out-dir "$out/bindings"
mv "$out/bindings/office_mobile.swift" "$out/generated/"
mv "$out/bindings/office_mobileFFI.h" "$out/include/"
# Xcode finds a static XCFramework's module by this name, beside its header.
mv "$out/bindings/office_mobileFFI.modulemap" "$out/include/module.modulemap"
rm -r "$out/bindings"

args=()
for t in "${targets[@]}"; do
  args+=(-library "target/$t/release/liboffice_mobile.a" -headers "$out/include")
done
xcodebuild -create-xcframework "${args[@]}" -output "$out/OfficeCore.xcframework"
echo "Built $out/OfficeCore.xcframework and $out/generated/office_mobile.swift"
