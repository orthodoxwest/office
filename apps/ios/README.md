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

Simulator builds need none. Xcode's free provisioning runs the app on a phone of your own, from
a Mac, for seven days at a time; everyone else gets it through TestFlight.

## TestFlight

`.github/workflows/testflight.yml` archives the app for release on every pull request that
touches it, unsigned, with the current Xcode. On `master` it also signs the archive, uploads it
to App Store Connect, and gives it to the TestFlight external group **Testers**. A tester
installs Apple's TestFlight app and opens the group's public link once; each later build
reaches them there, as the Android preview's link always serves the newest APK.

The version is `MARKETING_VERSION` in `project.yml`; the build number counts commits, as the
Android version code does. Apple reviews the first build of each version (usually within a
day) before external testers can install it, and normally lets later builds of the same
version through without, so only a release should raise `MARKETING_VERSION`. Builds lapse after
90 days, and a re-run for a commit already uploaded is refused at the upload.

Until the secrets below exist, `master` builds unsigned and uploads nothing. None of the setup
needs a Mac: the certificate is made with `openssl`, the rest on the web.

1. **The app.** Under the organization's Apple Developer account, add the App ID
   `org.orthodoxwest.office` (Certificates, Identifiers & Profiles → Identifiers; no
   capabilities). In App Store Connect, add a new iOS app with that bundle ID. Its store name
   must be unique across the App Store, so it may need to differ from the home-screen name
   (`CFBundleDisplayName`, "Divine Office").
2. **The distribution certificate.** Make a key and signing request, upload the request as an
   *Apple Distribution* certificate (Certificates → +), download `distribution.cer`, and pack
   both as a `.p12`. OpenSSL 3 needs `-legacy`, or the Mac runner's keychain cannot read it.

   ```bash
   openssl req -new -newkey rsa:2048 -nodes -keyout dist.key -out dist.csr \
     -subj "/emailAddress=you@example.org/CN=Organization Name/C=US"
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
6. **TestFlight.** In the app's TestFlight tab, fill in Test Information (the beta review's
   contact and a feedback email), add an external group named `Testers`, and turn on its
   public link. Run the workflow from the Actions tab, or push to `master`; once Apple has
   reviewed the first build, the link installs it.

The certificate and profile last a year. Renew both the same way and replace the three
secrets; the API key does not expire.
