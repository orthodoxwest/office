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
  hairline of progress through the prayer, framed title, its date opening the date picker,
  "Prayer form", the
  office, its continuation and report link), and
  the ordo's month (year and month navigation, the month's heading pinned while its days scroll,
  day rows with colour rails, ranks, and one "Office details" disclosure holding the day's
  commemorations and its hours), and the year's frontispiece (the Tabula Temporaria, each
  date leading to its day in the ordo), and About the Office (`AboutScreen.kt`: the core's
  `about()` words, the seven hours as home's table of periods; home offers it as "Introduction"
  beside Change date in a reader's first week). The site menu carries the day's hours, the Ordo,
  Reminders and About (left out of a wide hour's header, as on the web), and the Theme
  (Default / Nave / Apse), Text (A A A) and Martyrology (Off / On) rows, and Chant (Off / On)
  once a hymn or Marian antiphon has a score: neuma (its bindings in `org.orthodoxwest.neuma`,
  from the same library) engraves the score and `Score.kt` draws it. The date picker's title
  turns its
  days into the year's months, as the web's does.
- **Wide screens.** From the web's breakpoint (701dp: a tablet, or a phone on its side) the app
  takes the web's desktop composition: the header's links inline, ending in Settings for the
  theme and text size; home's frontispiece set in one lancet niche for the screen's
  orientation, its depth told by light on the reveal by day and the phone's apse sky by night,
  every hour glossed (one to a line from 1000dp high), the whole at one computed scale
  (`nicheZoom`, the web's `--niche-zoom`), centred in the room with the chapel's light on it
  (`Niche.kt`); the ordo as the desktop table; the
  Tabula's figures in one line and its tables side by side; from 1000dp "Blessing." hangs into
  the margin as "All:" does, where below it stands above its words. `WideScreenshotTest` renders them
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
  (`tools/bake-plaster.py`; each theme's wall averages exactly its `--bg`). The ordo's pinned
  month heading carries a copy of the wall lined up with it, as the web's fixed attachment does. Ornaments are drawn
  from the web templates' SVG paths, and the Apse vault from its star tile.
- **Reading.** Navigation stays in the page, not over it; the screen stays awake while an hour
  is open, like the web's wake lock. The status bar is put away while the app is open; a swipe
  down from the top brings it back for a moment.
- **Keeping the place.** Each visit on the way back keeps its own scroll position and open
  sections, and the way back itself is saved state: Back returns to where the reader was, and
  so does a return after Android has closed the app in the background (`PlaceTest`). An hour
  the app was closed on opens at the reader's place only by the core's rule, the same on every
  front (`reopensAtPlace`: within 45 minutes, and not once prayed); otherwise at its top.
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

The app is counted in the web's daily usage report (DEPLOYMENT.md, "Usage metrics"), in
the same words: `Usage.kt` posts one beacon, built by the Rust core (`usage_beacon`), when a
page is shown, its theme or prayer form changes, or the reader comes back. Home and the reminders
page count toward the day's readers, an hour and the ordo in their own columns, and turning
reminders on as the web counts a generated feed link; only today's pages (a day either side)
count. Each page counts once a day. In place of the web's cookie the app sends a random
identifier it replaces every reporting day (America/New_York), so nothing it sends ties one day
to the next. A beacon is best effort: it is never queued offline, retried later, or shown.

This is the app's only use of the network (the `INTERNET` permission). Only `preview` and
`release` builds report (`BuildConfig.COUNT_USAGE`); `debug`, and so every test and screenshot,
never does. Nor does a phone in Play's pre-launch report, which installs each uploaded build
fresh on several test devices and crawls it; with an upload on every merge they would otherwise
outnumber the testers. Nothing on those phones reliably marks them (not all carry Test Lab's
`firebase.test.lab` system setting, which still rules out those that do, and the Play Store
installs the build), but each is wiped within minutes, so an installation reports only once it is
two hours old (`Usage.SETTLING`). A reader's first hours on a new install go uncounted; an update
keeps the install time. `UsageTest` checks what counts, the daily identifier, the retry, and the
test-device exclusion.

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

## Store bundles

Google Play takes a signed `.aab`. The `release` build signs with the upload key named by the
environment; the key never enters the repository, and Play App Signing re-signs each upload
with the app signing key Google holds.

```bash
export OFFICE_UPLOAD_KEYSTORE=~/keys/office-upload.jks
export OFFICE_UPLOAD_STORE_PASSWORD=...
export OFFICE_UPLOAD_KEY_PASSWORD=...
export OFFICE_UPLOAD_KEY_ALIAS=upload   # the default

make android-bundle        # → apps/android/app/build/outputs/bundle/release/app-release.aab
```

Without the key, `bundleRelease` stops before building. A manual run of the Android workflow
builds the same bundle when the repository has the `ANDROID_UPLOAD_KEYSTORE_BASE64`,
`ANDROID_UPLOAD_STORE_PASSWORD` and `ANDROID_UPLOAD_KEY_PASSWORD` secrets, and attaches it as
the `office-release-aab` artifact. The version code is the commit count, so build from a full
clone of `master`.

### Publishing from CI

The **Publish to Play** workflow (`.github/workflows/play.yml`) builds the bundle from `master`
and uploads it to Google Play.

- **Every merge to `master` that changes the app** goes to Internal testing and Closed testing
  at once, with `apps/android/whatsnew.txt` as the release notes. Internal testing skips Google's
  review and reaches the maintainers within minutes, so a bad build shows there first; Closed
  testing reaches testers once the review passes (usually hours), and a newer merge replaces a
  release still in review.
- **The release notes** ("What's new") are only ever the text of `apps/android/whatsnew.txt`
  (lines starting with `#` dropped), never anything from the merge. The same text goes with
  every build until it is edited, and an empty file uploads builds with no notes. Edit it in the
  pull request whose merge should carry it; a change to that file alone uploads nothing. A text
  over Play's 500 characters stops the run before it builds.
- **The public** gets a build when the **Promote to production** workflow moves it there (below),
  or when you promote it in Play Console (*Closed testing → Promote release → Production*); either
  way it is the exact build testers have been running. A manual run of Publish to Play can also upload to any
  track (`internal`, `alpha` for Closed testing, or `production`) with a staged rollout and
  notes, but only for a commit that isn't already in Play: each build's version code is its
  commit count, and Play takes each version code once.

It needs:

- the three upload-key secrets above;
- `PLAY_SERVICE_ACCOUNT_JSON`: the JSON key of a Google Cloud service account that has been
  invited under Play Console's *Users and permissions* with release rights for this app. Until
  it is set, merges skip the upload;
- a `play` environment (Settings → Environments) with you as required reviewer, so every
  manual run waits for approval. Merges run in `play-internal`, which GitHub creates on first
  use and which needs no reviewer.

Google's API can't create the app or take its first bundle, so the very first upload is
done by hand in Play Console.

### Promoting to production

The **Promote to production** workflow (`.github/workflows/play-promote.yml`,
`scripts/play-promote.py`) moves a build from Closed testing to production through the Play API,
by its version code, without rebuilding. It carries the release notes the build had on Closed
testing.

- **Every day** at 15:00 UTC: a staged rollout the workflow started two days before is
  finished (all users). Otherwise the build on Closed testing, once it has been there 24 hours and
  if it is newer than production's, goes to 20% of users. So a merge reaches a fifth of the public
  1 to 2 days after testers get it, and everyone two days later. Google's review of each new
  release (usually hours) comes out of the 20% stage. A build's time on Closed testing is when
  its Publish to Play run finished, and only the build on Closed testing now is promoted: while
  merges land less than a day apart, promotion waits for a quiet day.
- **The staged release's name** in Play Console ("0.1.120 staged 2026-11-12T15:00Z") records when
  it started; that is how the next runs know its age.
- **Halting** a rollout in Play Console (*Production → Halt rollout*), or starting one there,
  stops the daily promotions until you resume, finish or replace it (a manual run can do any of
  these by releasing a build).
- **A manual run** from the Actions tab releases Closed testing's newest build (or the version code
  given) now, to the share of users asked for; *Only say what would be released* reads Play and
  changes nothing.
- **Google reviews** every new production release. With managed publishing on in Play Console,
  approved releases also wait for you to press Publish there, so leave it off for the daily
  promotion to run unattended. If Play refuses the commit asking for `changesNotSentForReview`
  (it can after a rejection), send the release for review by hand in Play Console once.

It needs `PLAY_SERVICE_ACCOUNT_JSON` as a repository secret (or on both environments); without it
runs warn and promote nothing. Scheduled runs use the `play-auto` environment, which GitHub creates
on first use with no reviewer; manual runs use `play`, with its required reviewer. Give both
environments the deployment branch rule *master* (Settings → Environments), so only master's
copy of the workflow can promote.

### Closed testers

Closed testing's tester list is the Google Group `daily-office-testers` (*Anyone can join*).
The app is public on Google Play
(`play.google.com/store/apps/details?id=org.orthodoxwest.office`), so the website no longer
recruits testers: its old sign-up page, `/beta`, now redirects home.

### Listing screenshots

`StoreScreenshotTest` renders the listing's screenshots into `app/build/screenshots/store/`
(`make android-screenshots`; CI's `android-screenshots` artifact carries them), all 9:16
portrait: the phone at 1080×1920 and the 7-inch and 10-inch tablets at 1224×2176 and 1800×3200.
Files are numbered in upload order. The pages and the day follow the App Store's set in
`.github/workflows/ios.yml`, so both stores show the same screens; change them together. Re-shoot
after a visible change and replace them under *Main store listing*.

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
Nothing generated is checked in. The APK carries those three ABIs and no others (`abiFilters`),
though JNA ships its own native library for more. The corpus goes into the library compressed
(`apps/mobile-ffi/build.rs`) and is inflated once when the engine loads.

Build types: `debug` (debuggable, for Android Studio) and `preview` (optimized, non-debuggable,
what the prerelease ships). Both use the `.preview` application ID. `preview` and `release` are
shrunk by R8; JNA binds the generated bindings by name, so `app/proguard-rules.pro` keeps JNA
and `org.orthodoxwest.office.core` and `org.orthodoxwest.neuma` whole. The screenshot and reminder tests run against `debug`,
so a change to those rules wants a look at a `preview` build on a phone.

## Screenshots without a device

`ScreenshotTest` renders real hours from the Rust core with Robolectric's native graphics
and Roborazzi, on the JVM, with no emulator. The tall captures show most of an hour in one
image, which makes layout review (by a person or by Claude in a cloud session) practical.
CI uploads them as the `android-screenshots` artifact.

## Fonts

`res/font/` holds EB Garamond Regular and Italic and the ✠ glyph from Noto Sans Symbols,
under the SIL Open Font License (`FONTS-OFL-1.1.txt`). The Garamond files are the web's own
instances (`tools/gengaramond.py`: the variable fonts pinned a step heavier than Regular, cut to
Latin), so the apps set the very glyphs the web does. There is deliberately no Bold: the app
sets nothing bold, so under Android's Bold text setting Compose synthesizes weight from Regular
and small caps, sigils and ligatures stay (`ScreenshotTest.boldTextSetting`).
