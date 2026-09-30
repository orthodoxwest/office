# Android app

A native Kotlin/Compose reader for the Office, built on the same Rust engine as the web app.

```
data/ ──► apps/mobile-ffi (embeds data/, UniFFI API) ──► liboffice_mobile.so + Kotlin bindings
              │
              └─ crates/render-blocks: a composed hour as blocks of styled runs
apps/android/app: Compose UI that sets those blocks as text
```

The app never parses liturgical text itself. `render-blocks` applies the same line grammar
as the HTML renderer (pointed verses, ℣/℟ sigils, hymn stanzas, secret prayers, rubric
spans), and its tests check that every hour of a year carries exactly the web's words. The
corpus is compiled into the library, so the app works offline and each build prays the
corpus it was built from.

## Trying it on a phone

Every push to `master` that touches the app, the engine, or `data/` republishes the
**android-preview** prerelease. On an Android phone (8.0 or newer), open

<https://github.com/orthodoxwest/office/releases/download/android-preview/office-preview.apk>

and allow your browser to install apps when asked. Play Protect may warn about an unknown
developer; choose *Install anyway*. Later previews install over earlier ones and keep your
settings.

The preview's application ID is `org.orthodoxwest.office.preview`, so it will sit beside
the eventual store app rather than replace it. It is signed with `app/preview.keystore`, a
deliberately public key (like Android's debug key) whose only job is letting every
preview, from CI or a laptop, update the last. Store releases will use Play App Signing.

## Building locally

Needs: the Rust toolchain with the Android targets, `cargo-ndk`, JDK 21, and the Android
SDK (platform 36) with NDK 28.2.13676358.

```bash
rustup target add aarch64-linux-android armv7-linux-androideabi x86_64-linux-android
cargo install cargo-ndk
sdkmanager "platforms;android-36" "ndk;28.2.13676358"
export ANDROID_HOME=~/android-sdk     # or put sdk.dir in apps/android/local.properties

make android               # → apps/android/app/build/outputs/apk/preview/app-preview.apk
make android-screenshots   # → apps/android/app/build/screenshots/*.png
```

Gradle drives Cargo: `preBuild` runs `cargo ndk` for arm64-v8a, armeabi-v7a, and x86_64,
builds the host library, and generates the Kotlin bindings into `app/build/generated/uniffi`.
Nothing generated is checked in.

Build types: `debug` (debuggable, for Android Studio) and `preview` (optimized, non-debuggable,
what the prerelease ships). Both use the `.preview` application ID.

## Screenshots without a device

`ScreenshotTest` renders real hours from the Rust core with Robolectric's native graphics
and Roborazzi, on the JVM, with no emulator. The tall captures show most of an hour in one
image, which makes layout review (by a person or by Claude in a cloud session) practical.
CI uploads them as the `android-screenshots` artifact.

## Fonts

`res/font/` holds EB Garamond and the ✠ glyph from Noto Sans Symbols, converted from the
web's WOFF2 core subsets (`apps/office-web/static/fonts/`), under the SIL Open Font License
(`FONTS-OFL-1.1.txt`).
