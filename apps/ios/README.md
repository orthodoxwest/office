# Divine Office for iOS

A SwiftUI app on the same Rust core as the web and the Android app. The engine and the whole
corpus are linked into the app, so it works offline; `render-blocks` lays out each hour as
blocks of styled runs, and `presentation` holds the words every front shares, so the Swift code
never parses corpus text.

The app follows the Android app (apps/android/README.md), which carries the web's design into
native text, screen for screen:

- **Home**: the day's frontispiece, the invitation to pray, and the hours by period. On an iPad
  it stands in the niche with the chapel light, as the web's desktop.
- **The hours**: the colour band and framed title, the web's spacing, and the Lauds, Prime and
  Vespers preparation open. Psalms get their gilded initials, antiphons their hanging sigils,
  and hymns a centred column.
- **The ordo**: the month (the wide table on an iPad), each day's office digest, and the year's
  Tabula Temporaria.
- **Reminders**: local notifications with the bell. Each names its hour and the day's feast,
  and offers "Remind me in 10 min".
- **Chrome**: the Nave and Apse themes, the web's three text sizes on top of Dynamic Type, and
  the hand-set date picker.
- **Motion**, as the Android app's: pushes and pops are the stack's own slide and edge swipe; a
  page of the same kind (the next hour, another day or month) changes in place, keeping the
  page shown until the next is composed and then moving along Material's shared axis.
  Disclosures unfold, the menu drops in, the picker's months slide and follow a swipe, and a
  new theme or text size eases in. Reduce Motion turns the slides to fades. A pressed control
  dims; a choice (a setting, a prayer form, a day) gives a selection tick, a checkbox a light
  tap, and opening a page nothing.

The Office's text is set with TextKit (`Prose.swift`), not SwiftUI's `Text`:

- every line box is the web's line height, with its half-leading;
- hanging indents and tab-set gutters hold verse numbers and ℣/℟;
- an initial runs two lines deep beside the text, or stands raised when the text is short.

iOS keeps at most 64 pending notifications for an app, so reminders are scheduled as far ahead
as fit, up to four weeks. Every visit, and a background refresh about twice a day, move the
schedule on. The bell is `Office/Resources/bell.caf`, baked with Android's
(`apps/android/tools/bake-bell.py`).

## Usage counts

As the Android app does (apps/android/README.md, "Usage counts"), the app is counted in the
web's daily usage report: `Usage.swift` posts the Rust core's beacon when a page is shown, how
it is read changes, or the app comes forward, once a day each, under a random identifier it
replaces every reporting day. Only Release builds report (`OfficeCountsUsage`, set per
configuration in `project.yml`); Debug builds, and so the tests and the simulator screenshots,
never do. `UsageTests` checks what counts, the daily identifier, and the retry.

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
`OfficeTests` in the simulator, and launches the app at fixed pages to take screenshots.
Each page is shot on an iPhone and an iPad, in both themes.

The screenshots come from launch arguments:

- `-page hour -hour lauds -date 2026-03-15` opens a page at a date; `-page` also takes
  `home`, `ordo`, `year` or `reminders`;
- `-today` fixes today;
- `-theme apse` chooses the theme, and `-settings YES` opens the wide header's Settings;
- `-anchor hymn` or `-anchor psalm` scrolls an hour to its first hymn or psalm.

The screenshots are uploaded as an artifact. They are also force-pushed, with the build log, to
the branch `ci/ios-screenshots/<branch>`, so they can be fetched with git.

## Signing

Simulator builds need none. A phone needs an Apple Developer account: the preview will go out
through TestFlight once the publishing account exists. Until then, Xcode's free provisioning
runs it on a phone of your own for seven days at a time.
