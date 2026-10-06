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

## Design standard

Every interface in this project aims at immaculate craftsmanship, with as much beauty as
possible without distracting from a reverent experience. The app is the web's design set in
native text, not a restyling of it (`.claude/skills/web-ui-design/SKILL.md`):

- **Pages.** Home (the day's frontispiece, the "Pray the hours" inscription band, the
  invitation, the hour directory by period, "Change date"), each hour (colour band, the gold
  hairline of progress through the prayer, framed title, "Change date" and "Prayer form", the
  office, its continuation and report link), and
  the ordo's month (year and month navigation, day rows with colour rails, ranks,
  commemorations and office details), and the year's frontispiece (the Tabula Temporaria, each
  date leading to its day in the ordo). The site menu carries the day's hours, the Ordo, and
  the Theme (Default / Nave / Apse) and Text (A A A) rows. The date picker's title turns its
  days into the year's months, as the web's does.
- **Wide screens.** From the web's breakpoint (701dp: a tablet, or a phone on its side) the app
  takes the web's desktop composition: the header's links inline, ending in Settings for the
  theme and text size; home's frontispiece set in the niche, with its pointed head, stone
  moulding and day-coloured trim, centred in the room with the chapel's light on it
  (`Niche.kt`); the ordo as the desktop table; the
  Tabula's figures in one line and its tables side by side. `WideScreenshotTest` renders them
  at 1280×900, beside the web's desktop snapshots, and at a tablet's and a landscape phone's
  sizes.
- **Hymns.** Each hymn's stanzas are centred in a column the width of its longest line, as the
  web's fit-content `.hymn-verses`, so the rag is balanced rather than flush left
  (`HymnScreenshotTest`).
- **Tokens.** `Theme.kt` holds the web's colour tokens for Nave and Apse and the seasonal
  ornament retints; `TokensTest` fails when style.css changes a token the app has not followed.
  Text size scales the whole page, as the web scales its root (93% / 100% / 110%).
- **Measures.** Type sizes, line heights, gutters and the spacing between kinds of block were
  measured from the rendered web pages at a phone's width (one CSS px to one dp); compare
  `make android-screenshots` output with `.web-tools/tests/visual.spec.js-snapshots/`.
- **Materials.** The plaster wall is baked from the web's own photograph and blend layers
  (`tools/bake-plaster.py`; each theme's wall averages exactly its `--bg`). Ornaments are drawn
  from the web templates' SVG paths, and the Apse vault from its star tile.
- **Reading.** Navigation stays in the page, not over it; the screen stays awake while an hour
  is open, like the web's wake lock.
- **Keeping the place.** Each visit on the way back keeps its own scroll position and open
  sections, and the way back itself is saved state: Back returns to where the reader was, and
  so does a return after Android has closed the app in the background (`PlaceTest`).
- **The chosen theme throughout.** Nave or Apse colours the window from the first frame, the
  status and navigation bar icons, and (Android 13+) the next launch screen, whatever the
  phone's own light or dark (`WindowThemeTest`).
- **Motion.** Quiet and brief, never decorative. A page comes in only once its content is
  composed, so nothing flashes; it moves along Material's shared axis, deeper or later from
  the end and back or earlier from the start, or fades through the wall where there is no
  order (`MotionTest`). The back gesture draws the page behind in as it is made. Disclosures
  unfold from their controls and, once open, scroll into view; the menu drops in; the date
  picker's months slide, and a swipe turns them; a new theme or text size dissolves in
  (`Dissolve.kt`). A pressed control dims, as on iOS. Remove animations shows each at once.
- **Touch.** A light tick under a choice (a setting, a prayer form, a day) and a toggle's tick
  on a checkbox; none for a page opened.
- **Shortcuts.** Long-pressing the icon offers Lauds, Vespers, Compline and the Ordo, drawn in
  the web's hairline glyphs. They open the day they are tapped; Compline in the small hours is
  the day before's, as "Pray now" reckons it (`ShortcutsTest`).

## Home-screen widget

`OfficeWidget` sets out the day as home's frontispiece does: the date, a double gold rule with
the day's liturgical colour as its lozenge, the feast, and the invitation to the hour now,
which opens that hour; the day opens home. On Android 12+ a widget one row high becomes a
strip. Under the Default theme its colours are night-aware resources, so it follows the phone
as it changes; Nave or Apse sets them. Each refresh sets a non-waking alarm for the next change
of hour or midnight (`Widgets.nextChange`), so a sleeping phone is never woken for it; clock and
time-zone changes, updates, the theme menu and app visits refresh it too. `WidgetTest` checks
its words and refreshes, and renders it for review.

## Accessibility

- **The office, spoken.** Each block of an hour is one stop for TalkBack, in words: ℣ and ℟
  said as "Versicle" and "Response", the pointing marks (* and †) turned to the pauses they
  mark, verse numbers left to the eye, ✠ said as "sign of the cross", and posture cues read as
  asides ("(Sit.)"). `SpokenTest` checks every hour of three days.
- **Controls.** Every control has its role (button, checkbox, radio) and state (selected,
  expanded or collapsed); glyph-only controls ("‹", "↑") are named; headings are marked; an
  ordo day is one stop that says its date, feast, colour and observances
  (`AccessibilityTest`).
- **Large text.** Rows, gutters and labels grow with Android's font size rather than clip or
  break a word; the 200% captures (`font-200-*.png`) are rendered with the others.

The two reasons for a native app are fully offline use and **reminder notifications of a
higher quality than the web can give**.

## Reminders

The Reminders page (from the menu, as on the web) offers the web's hours, times, days and
lead times (`presentation::REMINDER_DEFAULTS`). Nothing fires until the reader turns
reminders on. Each notification names the office and the day it keeps ("Vespers", "III
Sunday in Lent"), in the same words as the web's calendar feed (`presentation::reminder_summary`),
and a tap opens that hour.

- **Scheduling.** `ReminderScheduler` keeps the next eight days registered with the alarm
  service, so every weekday is in the window beyond today and a weekly reminder always has its
  next one waiting. Each alarm carries its notification's words, so it posts at once without
  loading the engine. Every reboot, clock or time-zone change, app update and app visit syncs
  again; a firing syncs only once the last alarm scheduled is within a day, so most reminders
  wake nothing but the notification. A sync replaces alarms rather than adding to them.
- **On the minute.** Exact alarms that fire through Doze when the reader allows "Alarms &
  reminders" (Android 14 asks); otherwise the system's nearest time, still through Doze, and
  the page says how to allow the exact minute.
- **Permission.** Turning reminders on asks for notifications (Android 13+). If they are
  refused, reminders stay scheduled and the page links to the setting.
- **The bell.** Reminders ring one soft stroke of a tubular bell (`res/raw/bell.ogg`), on
  their own channel, so the phone's settings can still change or silence it. The recording is
  from the Versilian Community Sample Library, dedicated to the public domain (CC0 1.0);
  `tools/bake-bell.py` names the file and how it was trimmed.
- **Remind me in 10 min.** A reminder's one action puts it away and rings it again, with the
  same words, ten minutes later. The action is worded as an action, so it is not read as the
  office's own countdown.
- **The time, not an age.** A reminder's title gives the office's clock time ("Prime · 7:00
  AM", following the phone's 12/24-hour setting) rather than Android's relative "30m", which
  drifts and does not say whether the office is past or to come.
- **Clears itself.** A reminder leaves the shade an hour after its office begins (one fired late
  still shows for ten minutes), so an old Prime does not linger into the afternoon.

`ReminderTest` checks the schedule the alarm service holds, cancellation, the bell's channel,
the ten-minute wait, and the posted notification, its clock time and when it clears.

## Usage counts

The app is counted in the web's daily usage report (the top-level README, "Usage metrics"), in
the same words: `Usage.kt` posts one beacon, built by the Rust core (`usage_beacon`), when a
page is shown, its theme or prayer form changes, or the reader comes back. Home and the reminders
page count toward the day's readers, an hour and the ordo in their own columns, and turning
reminders on as the web counts a generated feed link; only today's pages (a day either side)
count. Each page counts once a day. In place of the web's cookie the app sends a random
identifier it replaces every reporting day (America/New_York), so nothing it sends ties one day
to the next. A beacon is best effort: it is never queued offline, retried later, or shown.

This is the app's only use of the network (the `INTERNET` permission). Only `preview` and
`release` builds report (`BuildConfig.COUNT_USAGE`); `debug`, and so every test and screenshot,
never does. `UsageTest` checks what counts, the daily identifier, and the retry.

## Trying it on a phone

Every push to `master` that touches the app, the engine, or `data/` publishes a new
**Android preview** release (tagged `android-preview-<version>`), marked latest. On an Android
phone (8.0 or newer), open

<https://github.com/orthodoxwest/office/releases/latest/download/office-preview.apk>

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
what the prerelease ships). Both use the `.preview` application ID. `preview` and `release` are
shrunk by R8; JNA binds the generated bindings by name, so `app/proguard-rules.pro` keeps JNA
and `org.orthodoxwest.office.core` whole. The screenshot and reminder tests run against `debug`,
so a change to those rules wants a look at a `preview` build on a phone.

## Screenshots without a device

`ScreenshotTest` renders real hours from the Rust core with Robolectric's native graphics
and Roborazzi, on the JVM, with no emulator. The tall captures show most of an hour in one
image, which makes layout review (by a person or by Claude in a cloud session) practical.
CI uploads them as the `android-screenshots` artifact.

## Fonts

`res/font/` holds EB Garamond Regular and Italic and the ✠ glyph from Noto Sans Symbols,
converted from the web's WOFF2 core subsets (`apps/office-web/static/fonts/`), under the SIL
Open Font License (`FONTS-OFL-1.1.txt`). There is deliberately no Bold: EB Garamond 12's Bold
is an unfinished 128-glyph face without small caps or ℣/℟, so under Android's Bold text setting
Compose synthesizes weight from Regular instead (`ScreenshotTest.boldTextSetting`).
