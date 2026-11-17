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
- **The ordo**: the month under its pinned heading (the wide table on an iPad), each day's
  office details (its commemorations and office digest), and the year's Tabula Temporaria.
- **Reminders**: local notifications with the bell. Each names its hour and time ("Vespers ·
  6:00 PM", as Android's) and the day's feast, offers "Remind me in 10 min", and clears an hour
  after its office begins.
- **Icon shortcuts**: long-pressing the app's icon offers Lauds, Vespers, Compline and this
  month's ordo, with the web's glyphs, as Android's launcher shortcuts do (`Shortcuts.swift`).
- **Keeping the place**: the way back is kept with the scene for a return after iOS has closed
  the app in the background, and so is the reader's place in an hour, which opens there by the
  core's rule, the same on every front (`reopensAtPlace`: within 45 minutes, and not once
  prayed). A month of the ordo notes the day chosen in it, so Back or a return opens at that day.
- **Chrome**: the Nave and Apse themes, the web's three text sizes on top of Dynamic Type, and
  the hand-set date picker.
- **The crown**: the consecration cross at the head of home's arch turns the look over, unannounced,
  as the web's does (presentation::crown has the rule and the timetable). The change plays as dusk
  or dawn (`CrownPlay.swift`): act one is drawn in a layer of the window over the app's views, and
  at the turn the page is photographed and let go of, over the live new theme, through the front
  in the head and then the arch's own shape. Every figure it reads is in `CrownPlan.swift`.
- **Motion**, as the Android app's: pushes and pops are the stack's own slide and edge swipe; a
  page of the same kind (the next hour, another day or month) changes in place, keeping the
  page shown until the next is composed and then moving along Material's shared axis.
  Disclosures unfold; home's "Change date" turns the invitation and the hours under the band
  into the calendar in their place, in one motion, as the web's and Android's: where the head's
  spare height holds the difference the head gives it up in step, and otherwise the arch keeps
  its height and the page rises with the turn to the card's foot. Another prayer form, or the
  Martyrology turned on or off, changes the hour where it stands, keeping the reader's place and
  open sections. The menu drops in, the picker's months slide and follow a swipe, and a
  new theme or text size eases in. Reduce Motion turns the slides to fades. A pressed control
  dims; a choice (a setting, a prayer form, a day) gives a selection tick, a checkbox a light
  tap, and opening a page nothing.

The Office's text is set with TextKit (`Prose.swift`), not SwiftUI's `Text`:

- every line box is the web's line height, with its half-leading;
- hanging indents and tab-set gutters hold verse numbers and ℣/℟;
- an initial runs two lines deep beside the text, or stands raised when the text is short.

iOS keeps at most 64 pending notifications for an app, so reminders are scheduled as far ahead
as fit, up to four weeks. Every visit, each reminder answered or shown while the app is open, a
change of time zone or clock, and a background refresh about twice a day, move the schedule on. The bell is `Office/Resources/bell.caf`, baked with Android's
(`apps/android/tools/bake-bell.py`).

## Usage counts

As the Android app does (apps/android/README.md, "Usage counts"), the app is counted in the
web's daily usage report: `Usage.swift` posts the Rust core's beacon when a page is shown, how
it is read changes, or the app comes forward, once a day each, under a random identifier it
replaces every reporting day. Only Release builds report (`OfficeCountsUsage`, set per
configuration in `project.yml`); Debug builds, and so the tests and the simulator screenshots,
never do. Nor does a copy with a sandbox receipt (TestFlight's, and the one App Review runs for
each submission) or a simulator's (`Usage.storeInstall`): only App Store installs are readers.
`UsageTests` checks what counts, the daily identifier, the week's and month's first day, and the
retry.

## Building

On a Mac with Xcode 16, Rust (with the `aarch64-apple-ios` and `aarch64-apple-ios-sim` targets)
and XcodeGen (`brew install xcodegen`):

```bash
make ios          # apps/ios/build-core.sh, then xcodegen generate
open apps/ios/Office.xcodeproj
```

`build-core.sh` builds the Rust core as a static library for iPhone and the simulator (or only
the targets `IOS_TARGETS` names, as CI's simulator jobs do), bundles
them as `build/OfficeCore.xcframework`, and generates the UniFFI bindings,
`build/generated/office_mobile.swift`, from the host library as Android's Kotlin is generated.
The same library carries neuma's chant engraving; its bindings (`build/generated/Neuma.swift`)
build as their own `Neuma` module, imported only by `Score.swift`, since its `Page` and `Item`
would otherwise meet the app's own names.
`project.yml` is the XcodeGen spec; the `.xcodeproj` is generated, not checked in. The app
shares the web's faces with the Android app (`apps/android/app/src/main/res/font/`).

## Continuous integration

`.github/workflows/ios.yml` runs on macOS: it builds the core, generates the project, runs
`OfficeTests` in the simulator, and launches the app at fixed pages to take screenshots
(`screenshots.sh`). Each page is shot on an iPhone and an iPad, in both themes.

The screenshots come from launch arguments:

- `-page hour -hour lauds -date 2026-03-15` opens a page at a date; `-page` also takes
  `home`, `ordo`, `year`, `reminders` or `about`, and `find` (with `-query benedict`) or `feast`
  (with `-feast st-benedict`);
- `-today` fixes today, and `-clock 9` the hour of the clock (home's invitation, About's hours);
- `-theme apse` chooses the theme, `-settings YES` opens the wide header's Settings, and
  `-menu YES` the phone's menu;
- `-day YES` brings the ordo to the date's day, under its pinned month heading;
- `-anchor hymn`, `-anchor psalm` or `-anchor score` scrolls an hour to its first hymn, psalm or
  chant notation (with `-chant YES`).

The screenshots are uploaded as an artifact. They are also force-pushed, with the build log, to
the branch `ci/ios-screenshots/<branch>`, so they can be fetched with git.

### Screenshot baselines

The screenshots are compared with baselines checked in at `snapshots/`, as the web's browser
snapshots are (`scripts/ios-snapshots.py`). Each shot is brought down to the device's points (a
third of an iPhone's pixels, half an iPad's) before it is compared and kept, which keeps the
folder near the web's in size and lets antialiasing average out. A pixel differs when a channel
moves by more than 24 levels of 255, and a shot differs when more than 0.05% of its pixels do
(the web's `maxDiffPixelRatio`), which a changed word still exceeds. A pull request whose shots
differ, are new or are gone fails; the run's summary names them, and its `ios-snapshot-diffs`
artifact shows each beside its baseline with the changed pixels in red; the summary and the
pull request's comment also say when nothing was compared. The job runs on any change to what
the shots are built from (the app, the core, the crates, `data/`, the shared faces and plaster),
so a text or engine change that moves a page is caught on its own pull request. On `master` a
difference is only a warning, since master cannot take the label.

The app is installed once on each simulator, and every shot starts from its data cleared back to
what the install left (defaults through cfprefsd, then the container), so no earlier shot's kept
reading place, way back or settings reaches it; at a fixed day, today and hour (`-date`, `-today`,
`-clock 9`), in New York's time zone, in English, at the default text size, with the status bar's
time fixed; and once the app says the page asked for is drawn (`ScreenshotReady.swift` writes a
marker to its temporary directory once the page's content is on screen and scrolled to its anchor;
three minutes from launch without it fails the run) and has come to rest (two frames a second apart
alike). A blank shot, one colour below the status bar as a page not yet drawn is, fails the
comparison and is never offered as a baseline, nor fetched as one. The baselines record the Xcode,
simulator runtime and devices they were taken with (`snapshots/environment.json`), and later runs
take the same ones. When the runner image no longer has them, the shots are taken on its newest and
not compared, with a warning, until the baselines are taken again. Shots that cannot be made stable
are listed with their reason in `EXCLUDED` in `scripts/ios-snapshots.py` (none so far). The App
Store sizes are never compared.

When a change to the shots is intended, add the **update-ios-snapshots** label to the pull
request (`.github/workflows/update-ios-snapshots.yml`). It reuses the set the pull request's
last iOS run would check in, or builds the app and takes the screenshots again on macOS when
there is none, validates the files, and pushes them to the branch; a shot that still matches
keeps its baseline's bytes. The label is the browser's `update-ux-snapshots`' twin rather than
the same label, so that a browser update need not wait on a macOS build. Its commit is pushed
with the workflow's own token, which starts no workflow, so the new head has no iOS result until
the next push to the branch runs the job again. A fork's pull request cannot take the label (its
branch is not this repository's), and its iOS run publishes no branch; its baselines are taken
on a branch here. Without the label, `python3 scripts/ios-snapshots.py fetch BRANCH` copies a
branch's last run's set from `ci/ios-screenshots/BRANCH` into `snapshots/`, to commit by hand.

A manual run with **store** ticked takes the same pages at the sizes App Store Connect asks for,
on a 6.9-inch iPhone and a 13-inch iPad, and publishes them to `ci/ios-screenshots/<branch>-store`.
Its `store/` folder holds the listing's sets ready to upload (`scripts/ios-snapshots.py store`):
`iphone-6.9/`, `iphone-6.3/` (the size App Store Connect's 6.3-inch slot takes) and `ipad-13/`, numbered
in the listing's order and without an alpha channel, which App Store Connect refuses.

## Signing

Simulator builds need none. Xcode's free provisioning runs the app on a phone of your own, from
a Mac, for seven days at a time; everyone else gets it through TestFlight.

## TestFlight

`.github/workflows/testflight.yml` archives the app for release on every pull request that
touches it, unsigned, with the current Xcode. On `master` it also signs the archive, uploads it
to App Store Connect, and gives it to the TestFlight external group **Testers**. A tester
installs Apple's TestFlight app and opens the group's public link once; each later build
reaches them there, as the Android preview's link always serves the newest APK.

The build number counts commits, as the Android version code does. The version is chosen
before each upload: `MARKETING_VERSION` in `project.yml`, unless App Store Connect already has
that version (or a higher one) submitted, in review, waiting to be released or on sale, in which
case the upload takes one patch above the highest such version (`scripts/appstore.py version`).
App Store Connect closes a version to new builds once Apple approves it, and a build uploaded
while its version is in review could never be released, so the version moves on as soon as a
release is submitted: nobody edits `project.yml` after a release. It never goes back below a
version TestFlight already has builds under. `project.yml` stays the floor, so raising its
minor or major version (1.0.x to 1.1.0) is how to start a new line; pull requests and unsigned
builds use it as it stands. If App Store Connect cannot be read, the upload goes ahead under
`project.yml`'s version with a warning.

So each App Store submission starts a new TestFlight version, and Apple's beta review checks
the first build of every version (usually within a day) before external testers can install it.
Builds uploaded while that review runs reach internal testers only; the first merge after it
clears reaches the **Testers** group, and later builds of the same version normally go through
without review. The group is a convenience for anyone who wants builds early: nothing in the
App Store submission below depends on it, nor on beta review, which gates nothing any more. Builds lapse after 90 days, and a re-run for a commit already uploaded
is refused at the upload.

Until the secrets below exist, `master` builds unsigned and uploads nothing. None of the setup
needs a Mac: the certificate is made with `openssl`, the rest on the web.

1. **The app.** Under the Apple Developer account, add the App ID
   `org.orthodoxwest.office` (Certificates, Identifiers & Profiles → Identifiers; no
   capabilities). In App Store Connect, add a new iOS app with that bundle ID. Its store name
   must be unique across the App Store, so it may need to differ from the home-screen name
   (`CFBundleDisplayName`, "Daily Office").
2. **The distribution certificate.** Make a key and signing request, upload the request as an
   *Apple Distribution* certificate (Certificates → +), download `distribution.cer`, and pack
   both as a `.p12`. OpenSSL 3 needs `-legacy`, or the Mac runner's keychain cannot read it.

   ```bash
   openssl req -new -newkey rsa:2048 -nodes -keyout dist.key -out dist.csr \
     -subj "/emailAddress=you@example.org/CN=Your Name/C=US"
   openssl x509 -inform der -in distribution.cer -out dist.pem
   openssl pkcs12 -export -legacy -inkey dist.key -in dist.pem -out dist.p12
   ```
3. **The profile.** Profiles → + → *App Store Connect*, for the App ID and certificate above;
   download it.
4. **The API key.** App Store Connect → Users and Access → Integrations → Team Keys, with the
   App Manager role. Download the `.p8` (offered only once) and note its Key ID and the
   Issuer ID above the list.
5. **The secrets** (repository Settings → Secrets and variables → Actions):

   | Secret | Value |
   | --- | --- |
   | `IOS_DIST_CERT_P12` | `base64 -w0 dist.p12` |
   | `IOS_DIST_CERT_PASSWORD` | the `.p12`'s export password |
   | `IOS_APPSTORE_PROFILE` | `base64 -w0` of the downloaded `.mobileprovision` |
   | `ASC_KEY_ID` | the key's ID |
   | `ASC_ISSUER_ID` | the Issuer ID |
   | `ASC_KEY` | the `.p8` file's contents |

   The team ID is read from the profile. Keep `dist.key` and the `.p8` out of the repository.
6. **TestFlight.** In the app's TestFlight tab, fill in Test Information: the Beta App
   Description, a feedback email, and the beta review contact's name, phone and email. Beta
   review refuses a build without them. Run the workflow from the Actions tab on `master`, or
   push to `master`. App Store Connect offers external groups only once a build exists, so after
   the first upload add an external group named `Testers`, add that build to it by hand, and
   turn on its public link; later builds go to the group by themselves. Once Apple has
   reviewed the first build, the link installs it.

Only `master` uploads: a run from another branch archives unsigned, since a branch's commit
count is a build number master reaches later. If a run fails after the upload (processing, or
handing the build to the group), a re-run is refused at the upload; give the build to the group
in App Store Connect, or with `fastlane pilot distribute --build_number N`.

The certificate and profile last a year. Renew both the same way and replace the three
secrets; the API key does not expire.

## The App Store

Before the first release, by hand in App Store Connect:

- **The listing** (the app's App Store tab): subtitle, description, keywords, the Reference
  category, a support URL, and the web's privacy page as the privacy policy
  (`https://orthodoxwestbreviary.com/privacy`).
- **App Privacy.** One data type: *Usage Data → Product Interaction*, used for *Analytics*, not
  linked to the user, not used for tracking (the daily usage count, above). It must agree with
  `PrivacyInfo.xcprivacy`, the privacy manifest every upload carries.
- **Age rating**: no to every question, which gives 4+.
- **Screenshots**: a manual run of the iOS workflow with **store** ticked (above).
- **Pricing and availability**: free, in the countries chosen.

Each later version carries all of these over from the one before; change them in App Store
Connect when they need it.

### Submitting for review

The **Submit to the App Store** workflow (`.github/workflows/appstore-promote.yml`,
`scripts/appstore.py`) sends a TestFlight build to App Review through the App Store Connect API,
without rebuilding, as the Android app's promotion moves a Closed testing build to production.

- **Every day** at 15:00 UTC: unless a version is already with Apple or waiting to be
  released, the newest valid build that was uploaded at least 24 hours before and has a higher
  version than the App Store's is submitted. The day is a soak, in which master's CI for the same
  merge can show a problem; there are no regular TestFlight testers,
  so TestFlight's beta review and the **Testers** group play no part, and what catches a problem
  after that is App Review and readers' reports, during the phased release.
  The workflow creates the App Store version for the build's version string (or fills in one
  being prepared), attaches the build, sets *What's New*, chooses *Automatically release this
  version* and Apple's seven-day phased release, and submits it. Apple's review usually takes a
  day or two; once approved the version goes out by itself, to a growing share of automatic
  updaters over seven days (anyone can download it at once). While a version is with Apple or
  waiting to be released nothing more is submitted, so merges in the meantime go out together in
  the next release. A version set to be released by hand (1.0.0 was) waits in *Pending Developer
  Release* until someone presses *Release this version* in App Store Connect, and holds back the
  next one until then.
- **What's New** is `apps/android/whatsnew.txt` as of the build's commit, the notes Google Play
  shows, or, while that is empty, "Corrections to the texts and improvements to the app." Edit
  it in App Store Connect before Apple's review starts if a release deserves more.
- **Releasing now**: a manual run from the Actions tab submits the newest valid build, of any
  age, or the build number given, with the notes given or the file's;
  *Only say what would be submitted* reads App Store Connect and changes nothing.
- **Pausing**: disable the workflow in the Actions tab. A version in review can be taken back
  with *Remove from Review* in App Store Connect; the next run then fills in that version again
  with the newest build a day old and resubmits it, so disable the workflow first to keep it out.
  The phased release can be paused (up to 30 days) or finished at once on the version's page.
- **A run that stopped** partway (App Store Connect slow or refusing) is finished by the next
  run: it reuses the version and draft submission it made, and sends them. A draft submission
  that also holds something else (an in-app event, say) is left alone until it is sent or
  emptied in App Store Connect.
- **A rejection** stops the daily submissions until a person acts, since Apple's reasons need
  reading first. Uploads moved past the rejected version's number when it was submitted, so the
  fix comes in a build with a higher version. Once that build is uploaded, a manual run
  (the build number, or empty for the newest) renumbers the rejected version to the build's
  version, attaches it, marks Apple's issue resolved and resubmits; daily runs then carry on.
  By hand in App Store Connect the same is: on the rejected version's page change the version
  number to the new build's, choose that build under *Build*, save, and *Resubmit to App
  Review* (or reply in App Review's messages if the rejection was a misunderstanding).
- **Export compliance** needs no answer: `ITSAppUsesNonExemptEncryption` is false in the
  Info.plist (`project.yml`), so every build already says the app uses no non-exempt encryption.

It uses the TestFlight API key (`ASC_KEY_ID`, `ASC_ISSUER_ID`, `ASC_KEY`, above), whose App
Manager role may submit; without it runs warn and submit nothing. Scheduled runs use the
`appstore-auto` environment, which GitHub creates on first use with no reviewer; manual runs use
`appstore`. Create `appstore` (Settings → Environments) with a required reviewer, and give both
environments the deployment branch rule *master*, so only master's copy of the workflow can
submit.
