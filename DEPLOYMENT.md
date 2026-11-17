# Deployment and usage metrics

[Back to the README](README.md)

## Deployment

The app runs on Fly.io ([fly.toml](fly.toml)). The [Dockerfile](Dockerfile)
builds a Rust binary with bundled SQLite and time-zone data, then copies it
and `data/` into a Debian slim runtime image. CI smoke-tests that image.
Maintainers with Fly access deploy with `fly deploy`. To roll back a change,
revert its PR, merge the revert, and deploy it.

**Changing address.** `OFFICE_CANONICAL_HOST` (a bare host such as
`example.org`) makes every other host forward there: browser navigations get a
page that carries the reader's settings across, and other requests a 301 (308
for non-GET). The usage beacon, `/office.ics` and `/static/*` keep answering on
the old host, since native builds post beacons without following redirects and
subscribed calendars keep polling their feed URL; the feed's links name the
canonical host. The old host's `/sw.js` becomes a worker that clears its caches
and unregisters. Unset, every host is served as itself.

**Search and link previews.** `/robots.txt` and `/sitemap.xml` point crawlers at
the undated pages (today's day and hours, this year's ordo, reminders, privacy)
on the canonical host. Every page names its canonical URL without `?form=` and
carries Open Graph tags for link previews; dated days and hours, ordo years
other than this one and the next, and error pages are `noindex`.

## Usage metrics

`/admin/usage` is an unlinked, unauthenticated, `noindex` report (excluded from
the service-worker cache) of daily readers over 7, 30, 90 or 365 days (a window
longer than the record is offered once, as "Since" its first day). Days are
America/New_York. A reader is one browser or app installation on one day. The
page leads with a sentence: the last seven completed days' average against the
seven before, called "about level" when they differ by under a fifth, which at
a few dozen readers a day is chance. Under it are the same figure over four
weeks, returning readers a day, new readers in the week, and today so far;
then, once clients report them, the readers of the last calendar week and
month. The chart splits each day's bar into returning and new and draws a
trailing seven-day average. "Where they read" sets each client's week against
the one before; "What they open" gives each hour, the ordo and reminders as a
share of the period's readers, and how many hours a reader opens; "Settings in
use" gives one share per setting, with every family's series over time behind a
disclosure (weekly by default; a part week stands apart at either end).

**Counts that were not readers.** The store keeps what it was sent; the report
reads it through `usage::CORRECTIONS`. A day test devices swamped (`test_days`)
is drawn and listed but left out of every average, share and scale. A client's
counts before the day it reached readers (`released`) are taken out of readers,
new readers and the clients; the hours those devices opened cannot be told
apart and remain. The page lists whatever its window leaves out.

**What counts.** Visible pages, not completed prayers. A browser counts once
overall and once in each hour it opens. Ordo counts a viewed calendar page;
Reminders counts a *generated* feed link, not a visit to `/reminders`. Home
counts toward the total only. Preloads don't count; offline use and no-JS
browsers are missed; multiple devices or cleared cookies inflate the count.

**Rendering dimensions.** The same beacon reports `appearance:nave|apse` (as
rendered, chosen or inherited) and `screen:desktop|mobile` (mobile means under
the 700px breakpoint or a touch-primary pointer). Office pages also report
`prayer-form:private|deacon|priest`, which measures the prayers read, not the
reader's ordination. `visit:first|returning` says whether this browser or installation first opened the Office
today: each keeps that day on the device (`office-first-counted` in localStorage, `usage-first`
in the apps) and sends only the word, so nothing links one day to the next. Browsers that used
the Office before it existed count as returning when they hold a setting or the service
worker; apps, when they had reported before. Safari erases a page's storage after seven days
without a visit, so the server also sets two flags of its own: `office-seen=1` (400 days, renewed on
every beacon) and, beside it the first time, `office-new=1`, which lapses at New York's midnight.
A browser that brings the first without the second was counted on an earlier day, and counts as
returning whatever its page says. Both hold only `1`, so no date is sent. An iOS Home Screen web app has its own
cookies as well as its own storage, so installing one still counts as new once. Neither app reports until its installation is two hours old: Play's
pre-launch phones and the devices Apple checks each TestFlight upload on are fresh installs that
are gone within the hour, and each counted as a new reader. The iOS app also stays silent in a
copy with a sandbox receipt, which is TestFlight's and the one App Review runs for each submission, and
in a simulator (`Usage.storeInstall`). `client:browser|pwa|android|ios` separates a browser tab,
the web app installed to a home screen, and the native apps. The server also counts a
first-day visit under `new-client:<client>`, so new readers split by client; no beacon sends it.

`week:first|again` and `month:first|again` say whether today is the first reporting day this
browser or installation is counted in its calendar week (Monday to Sunday) and month, so a week's
`week:first` counts add up to that week's readers, and a month's likewise, with nothing sent that
links one day to another. Each front keeps the last two reporting days one of its beacons was
counted (`office-counted` and `office-counted-before` in localStorage, `usage-counted` and
`usage-counted-before` in the apps), moves them on only when a beacon has arrived, and sends the
two words (`presentation::usage::periods`; app.js mirrors it). Every beacon of a first day says
`first`. Older clients say nothing, so the report shows how many of the period's readers reported
and a week leads only once every day of it was reported. Safari's seven-day erasure cannot
recount a reader within a week, since seven days away always ends in another week.

`offices:1`…`offices:7` is derived by the server, like `new-client`: as a reader first opens an
hour, they move from one count to the next, so each sits under the number of hours opened so far
that day. The dedupe rows already say which hours a day's hash has opened; nothing new is kept.
Prime, on a day
with a Martyrology reading, also reports `martyrology:shown|hidden`: whether the
reader's Settings choice (off by default) read it or left the rubric, which
measures the reading's exposure (see MARTYROLOGY.md). An hour with a scored hymn or Marian
antiphon reports `chant:on|off`: whether the reader's Chant setting (off by default) showed the
notation or the words alone. Each counts once per browser per day, so a reader who
switches mid-day counts on both sides and a pair can exceed the daily total;
clients with an older cached `app.js` report none, so a pair can also fall
short. The report draws each pair as a day-by-day mix band, since one period
share can't distinguish a steady split from a migration.

Values are stored as `family:value` and are never redefined: add a new key so
an old series ends where its meaning ended. The vocabulary lives in
`presentation::usage`, shared by the server's parser and the apps' beacons
(app.js mirrors it). The server only has to understand the scope; unknown
tokens from newer or older cached clients are dropped and the page still counts.
A body over `MAX_BEACON` (192 bytes) is refused whole, so the longest beacon each front writes is
tested against it with room to spare (`beacons_fit_the_limit`). The browser sends its beacon
`keepalive`, so the tap that opens an hour from home, which is also what first engages the page,
does not lose the reply's cookie and count a second browser on the next page.

**The native apps.** Release builds of the Android and iOS apps post the same
beacon to `https://orthodoxwestbreviary.com/api/usage` (`presentation::usage::ENDPOINT`)
until the server names another: every counted beacon's reply carries
`Office-Usage-Endpoint: https://<canonical host>/api/usage`, and the apps keep it
and post there from then on (only a plain HTTPS host is accepted). Builds from
before the move post to `https://office.fly.dev/api/usage`, which keeps
answering, and follow the header the same way. The browser's beacon is relative, so it follows the site by itself.
Apps post when a page is shown: home and the reminders page count toward the total, each
hour and the ordo in their columns, and turning reminders on counts as
Reminders. The same current-page window applies (`current_day`, `current_year`);
there is no engagement gate, since nothing crawls an app. Instead of a cookie the
server sets, each app sends a random ID it replaces every reporting day, so
nothing it sends ties one day to the next. Beacons are best effort: never queued
offline or retried later. Debug builds (and so the apps' tests and CI
screenshots) never report.

**Scraping is welcome but never counted.** `robots.txt` leaves every page open, but a
crawler presents a fresh cookie jar per page and would otherwise mint a
"browser" per URL. Four filters prevent that:

- Only current pages count: the beacon rejects dates outside today ±1 day (±1
  year for the ordo), capping any crawl at about ten pages.
- Nothing reports until the page is touched or visible for eight seconds.
- Self-identifying crawlers (`usage::is_bot`: `<name>bot/<version>` or an
  explicit token, not merely the substring "bot") get `204` with no cookie.
- The Android app does not report from the devices where Play's pre-launch
  report installs and crawls every uploaded build: an installation reports
  only once it is two hours old, and those devices are wiped sooner.

A stealth scraper that dwells on today's pages still counts, and a reader
browsing only the archive is missed.

**Storage.** `OFFICE_USAGE_DB` names the SQLite file; unset, the event
endpoint is a no-op and the report 404s. If SQLite cannot open, the server
logs a warning and serves without metrics; runtime storage errors return 503
only on the metrics endpoints. The parent directory must exist.

```bash
# Production: one replica, volume office_usage in iad mounted at /usage
fly volumes create office_usage --region iad --size 1 --app office
# Local
mkdir -p output/usage
OFFICE_USAGE_DB="$PWD/output/usage/usage.sqlite" ./office serve
```

**Privacy.** The first-party `office-usage` cookie is a random 128-bit ID
(until New York's midnight, HttpOnly, SameSite=Strict, Secure over HTTPS, path
`/api/usage`; a month-long one from before is cut short on its next beacon); the
apps send their own daily ID in its place, so no front sends an identifier on two days. `office-seen` and `office-new` (same attributes) hold only `1`
and settle `visit:first` as above; the apps get none of these cookies.
SQLite stores only a per-day hash, an office category, and a date — no raw
cookie, IP, user agent, URL, or timestamp. Dedupe rows older than the day
before yesterday are deleted on the next event or report read; aggregate counts are kept
indefinitely (volume snapshots may retain older data until they expire).

SQLite runs in WAL mode on one connection: back up with SQLite-aware tools or
volume snapshots, not by copying the database file alone. Multiple replicas
would keep independent counts.
