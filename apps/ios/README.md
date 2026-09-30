# Divine Office for iOS

A SwiftUI app on the same Rust core as the web and the Android app. The engine and the whole
corpus are linked into the app, so it works offline; `render-blocks` lays out each hour as
blocks of styled runs, and `presentation` holds the words every front shares, so the Swift code
never parses corpus text.

This first cut composes one hour, to prove the pipeline end to end. The design pass follows the
Android app's (apps/android/README.md), which carries the web's design into native text.

## Building

On a Mac with Xcode 16, Rust (with the `aarch64-apple-ios` and `aarch64-apple-ios-sim` targets)
and XcodeGen (`brew install xcodegen`):

```bash
make ios          # apps/ios/build-core.sh, then xcodegen generate
open apps/ios/Office.xcodeproj
```

`build-core.sh` builds the Rust core as a static library for iPhone and the simulator, bundles
them as `build/OfficeCore.xcframework`, and generates the UniFFI bindings,
`build/generated/office_mobile.swift`, from the host library as Android's Kotlin is generated.
`project.yml` is the XcodeGen spec; the `.xcodeproj` is generated, not checked in. The app
shares the web's faces with the Android app (`apps/android/app/src/main/res/font/`).

## Continuous integration

`.github/workflows/ios.yml` runs on macOS: it builds the core, generates the project, runs
`OfficeTests` in the simulator, and launches the app at fixed dates
(`-hour lauds -date 2026-03-15`) to take screenshots. They are uploaded as an artifact and
force-pushed, with the build log, to the branch `ci/ios-screenshots/<branch>`, so they can be
fetched with git.

## Signing

Simulator builds need none. A phone needs an Apple Developer account: the preview will go out
through TestFlight once the publishing account exists. Until then, Xcode's free provisioning
runs it on a phone of your own for seven days at a time.
