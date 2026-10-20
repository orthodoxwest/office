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
the service-worker cache) of daily unique browsers, overall and per office,
ordo, and reminder feed, over 7/30/90/365 days. Days are America/New_York.
Its headline is the last seven completed days' average against the seven
before, and the chart draws a trailing seven-day average, so a launch isn't
averaged in with the quiet months before it.

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
worker; apps, when they had reported before. `client:browser|pwa|android|ios` separates a browser tab,
the web app installed to a home screen, and the native apps. The server also counts a
first-day visit under `new-client:<client>`, so new readers split by client; no beacon sends it.
Prime, on a day
with a Martyrology reading, also reports `martyrology:shown|hidden|chapter`: whether the
reader's After Prime choice (Nothing by default) read it, left the rubric, or said the
Capitular Office (which reads it), which measures the reading's exposure (see MARTYROLOGY.md). Each counts once per browser per day, so a reader who
switches mid-day counts on both sides and a pair can exceed the daily total;
clients with an older cached `app.js` report none, so a pair can also fall
short. The report draws each pair as a day-by-day mix band, since one period
share can't distinguish a steady split from a migration.

Values are stored as `family:value` and are never redefined: add a new key so
an old series ends where its meaning ended. The vocabulary lives in
`presentation::usage`, shared by the server's parser and the apps' beacons
(app.js mirrors it). The server only has to understand the scope; unknown
tokens from newer or older cached clients are dropped and the page still counts.

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
(30 days, HttpOnly, SameSite=Strict, Secure over HTTPS, path `/api/usage`); the
apps send their own daily ID in its place.
SQLite stores only a per-day hash, an office category, and a date — no raw
cookie, IP, user agent, URL, or timestamp. Dedupe rows older than the day
before yesterday are deleted on the next event or report read; aggregate counts are kept
indefinitely (volume snapshots may retain older data until they expire).

SQLite runs in WAL mode on one connection: back up with SQLite-aware tools or
volume snapshots, not by copying the database file alone. Multiple replicas
would keep independent counts.
