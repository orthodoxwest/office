//! Approximate daily browser counts, not request logs, and the two endpoints that write and read
//! them.

use std::collections::BTreeMap;
use std::io::Read as _;
use std::sync::Mutex;
use std::time::Duration;

use axum::body::Body;
use axum::http::{HeaderMap, Method, Response, StatusCode, Uri, header};
use calendar::Date;
use presentation::usage::{
    DIMENSIONS, ENDPOINT_HEADER, HOURS, advertised_endpoint, dimension_key, endpoint_for, offices_scope, parse_event, valid_scope,
};
use render_html::usage::UsageDay;
use rusqlite::{Connection, params};
use sha2::{Digest, Sha256};

use crate::http::{Query, cookie, header_value, http_error, not_found, response, set};

/// Substrings of crawler and scripted-client user agents, lowercase.
const BOT_TOKENS: &[&str] = &[
    // Search, ads, and social preview fetchers.
    "googlebot",
    "google-extended",
    "google-inspectiontool",
    "adsbot",
    "bingbot",
    "bingpreview",
    "msnbot",
    "slurp",
    "duckduckbot",
    "baiduspider",
    "yandex",
    "sogou",
    "exabot",
    "seznambot",
    "qwantify",
    "petalbot",
    "applebot",
    "facebookexternalhit",
    "facebot",
    "twitterbot",
    "linkedinbot",
    "slackbot",
    "discordbot",
    "telegrambot",
    "whatsapp",
    "pinterest",
    // Archives and SEO/marketing crawlers.
    "ia_archiver",
    "archive.org_bot",
    "ahrefs",
    "semrush",
    "mj12bot",
    "dotbot",
    "dataforseo",
    "blexbot",
    "serpstat",
    "screaming frog",
    "sitebulb",
    // AI and dataset crawlers.
    "gptbot",
    "chatgpt-user",
    "oai-searchbot",
    "claudebot",
    "claude-web",
    "anthropic-ai",
    "ccbot",
    "perplexitybot",
    "bytespider",
    "amazonbot",
    "meta-externalagent",
    "diffbot",
    "cohere-ai",
    "timpibot",
    "youbot",
    // Automation, libraries, and monitoring — never a praying human.
    "headlesschrome",
    "phantomjs",
    "puppeteer",
    "playwright",
    "selenium",
    "scrapy",
    "python-requests",
    "python-urllib",
    "aiohttp",
    "httpx",
    "go-http-client",
    "okhttp",
    "libwww-perl",
    "java/",
    "apache-httpclient",
    "curl/",
    "wget/",
    "lighthouse",
    "pagespeed",
    "uptimerobot",
    "pingdom",
    "newrelicpinger",
    "statuscake",
    "site24x7",
    "prerender",
    // Generic self-descriptions.
    "crawler",
    "crawling",
    "spider",
    "scraper",
    "feedfetcher",
    "monitoring",
];

/// Unicode simple lowercase: one character maps to one character.
fn simple_lowercase(s: &str) -> String {
    s.chars()
        .map(|c| {
            let mut lower = c.to_lowercase();
            match (lower.next(), lower.next()) {
                (Some(l), None) => l,
                // Only U+0130 expands under full lowercase; its simple mapping is 'i'.
                _ => 'i',
            }
        })
        .collect()
}

/// Whether a user agent is a crawler or scripted client. Beyond the tokens
/// it catches the "<name>bot/<version>" form, but not a bare "bot", which
/// phone models carry ("Cubot"). An empty agent is a scripted client.
pub fn is_bot(agent: &str) -> bool {
    let agent = simple_lowercase(agent.trim());
    agent.is_empty() || agent.contains("bot/") || BOT_TOKENS.iter().any(|t| agent.contains(t))
}

fn eastern() -> jiff::tz::TimeZone {
    crate::web_time::zone("America/New_York").unwrap_or(jiff::tz::TimeZone::UTC)
}

/// The reporting day of an instant: its civil date in America/New_York.
fn eastern_day(now: jiff::Timestamp) -> Date {
    let z = now.to_zoned(eastern());
    Date::new(i32::from(z.year()), i32::from(z.month()), i32::from(z.day()))
}

/// The SQLite store, using a single connection.
pub struct Store {
    conn: Mutex<Connection>,
}

/// The longest window `daily` reads: the report's longest, and the days before it that its summary reads.
const MAX_DAYS: usize = 365 + render_html::usage::LOOKBACK;

impl Store {
    pub fn open(path: &str) -> Result<Store, String> {
        let conn = Connection::open(path).map_err(|e| e.to_string())?;
        conn.busy_timeout(Duration::from_millis(1000)).map_err(|e| e.to_string())?;
        conn.query_row("PRAGMA journal_mode=WAL", [], |_| Ok(())).map_err(|e| e.to_string())?;
        conn.execute_batch(
            "PRAGMA secure_delete=ON;
CREATE TABLE IF NOT EXISTS totals (
 day TEXT NOT NULL, scope TEXT NOT NULL, users INTEGER NOT NULL,
 PRIMARY KEY(day, scope)
);
CREATE TABLE IF NOT EXISTS seen (
 day TEXT NOT NULL, browser BLOB NOT NULL, scope TEXT NOT NULL,
 PRIMARY KEY(day, browser, scope)
);",
        )
        .map_err(|e| e.to_string())?;
        Ok(Store { conn: Mutex::new(conn) })
    }

    /// Counts a browser once per day for the site, the page scope, and each
    /// dimension. Only a per-day hash of the cookie is stored.
    pub fn record(&self, now: jiff::Timestamp, browser: &str, scope: &str, dimensions: &[String]) -> Result<(), String> {
        if !valid_scope(scope) || dimensions.iter().any(|d| dimension_key(d).is_none()) {
            return Err("invalid usage event".into());
        }
        let today = eastern_day(now);
        let day = presentation::date_slug(today);
        let hash = Sha256::digest(format!("{day}\x00{browser}").as_bytes());
        let mut conn = self.conn.lock().map_err(|e| e.to_string())?;
        let tx = conn.transaction().map_err(|e| e.to_string())?;
        tx.execute("DELETE FROM seen WHERE day < ?", [presentation::date_slug(today.add_days(-2))]).map_err(|e| e.to_string())?;
        let mut scopes = vec!["site"];
        if scope != "site" {
            scopes.push(scope);
        }
        scopes.extend(dimensions.iter().map(String::as_str));
        let new_client = presentation::usage::new_client(dimensions);
        scopes.extend(new_client.as_deref());
        let add = |scope: &str, by: i64| {
            tx.execute(
                "INSERT INTO totals VALUES (?1, ?2, ?3)\nON CONFLICT(day, scope) DO UPDATE SET users = users + ?3",
                params![day, scope, by],
            )
            .map(drop)
            .map_err(|e| e.to_string())
        };
        let mut opened = false;
        let reports = dimensions.iter().any(|d| d.starts_with("version:"));
        let reported = HOURS.contains(&scope).then(|| format!("reported:{scope}"));
        for v in scopes {
            let n =
                tx.execute("INSERT OR IGNORE INTO seen VALUES (?, ?, ?)", params![day, hash.as_slice(), v]).map_err(|e| e.to_string())?;
            if n == 0 {
                continue;
            }
            add(v, 1)?;
            opened |= HOURS.contains(&v);
        }
        // An hour opened by a client that says whether it was opened at its time and prayed: the
        // share's denominator. Counted once, whichever of the hour's beacons first said it.
        if let Some(reported) = reported.filter(|_| reports)
            && tx
                .execute("INSERT OR IGNORE INTO seen VALUES (?, ?, ?)", params![day, hash.as_slice(), reported])
                .map_err(|e| e.to_string())?
                > 0
        {
            add(&reported, 1)?;
        }
        // A newly opened hour moves the reader from one `offices` count to the next, so each
        // reader sits under the number of hours opened so far today.
        if opened {
            let hours = HOURS.map(|h| format!("'{h}'")).join(", ");
            let n: i64 = tx
                .query_row(
                    &format!("SELECT COUNT(*) FROM seen WHERE day = ? AND browser = ? AND scope IN ({hours})"),
                    params![day, hash.as_slice()],
                    |r| r.get(0),
                )
                .map_err(|e| e.to_string())?;
            // A reader whose visit today is their first is counted again among the new.
            let new: i64 = tx
                .query_row(
                    "SELECT COUNT(*) FROM seen WHERE day = ? AND browser = ? AND scope = 'visit:first'",
                    params![day, hash.as_slice()],
                    |r| r.get(0),
                )
                .map_err(|e| e.to_string())?;
            for prefix in if new > 0 { &["", "new-"][..] } else { &[""][..] } {
                // Only a reader counted there is moved: on the day the server begins keeping
                // these, an hour opened before it began left no count to take from.
                if let Some(before) = offices_scope(n as usize - 1) {
                    tx.execute(
                        "UPDATE totals SET users = users - 1 WHERE day = ? AND scope = ? AND users > 0",
                        params![day, format!("{prefix}{before}")],
                    )
                    .map_err(|e| e.to_string())?;
                }
                if let Some(now) = offices_scope(n as usize) {
                    add(&format!("{prefix}{now}"), 1)?;
                }
            }
        }
        tx.commit().map_err(|e| e.to_string())
    }

    /// The first reporting day with any recorded visit. Earlier days predate
    /// collection, so they are absent rather than quiet.
    pub fn first_day(&self) -> Result<Option<String>, String> {
        let conn = self.conn.lock().map_err(|e| e.to_string())?;
        conn.query_row("SELECT MIN(day) FROM totals WHERE scope = 'site'", [], |r| r.get(0)).map_err(|e| e.to_string())
    }

    /// A zero-filled window of `days`, newest first.
    pub fn daily(&self, now: jiff::Timestamp, days: usize) -> Result<Vec<UsageDay>, String> {
        if !(1..=MAX_DAYS).contains(&days) {
            return Err("invalid day window".into());
        }
        let today = eastern_day(now);
        let conn = self.conn.lock().map_err(|e| e.to_string())?;
        conn.execute("DELETE FROM seen WHERE day < ?", [presentation::date_slug(today.add_days(-2))]).map_err(|e| e.to_string())?;
        let mut result: Vec<UsageDay> =
            (0..days).map(|i| UsageDay { day: presentation::date_slug(today.add_days(-(i as i32))), ..UsageDay::default() }).collect();
        let indices: BTreeMap<String, usize> = result.iter().enumerate().map(|(i, r)| (r.day.clone(), i)).collect();
        let mut stmt = conn.prepare("SELECT day, scope, users FROM totals WHERE day >= ? AND day <= ?").map_err(|e| e.to_string())?;
        let rows = stmt
            .query_map([&result[days - 1].day, &result[0].day], |r| {
                Ok((r.get::<_, String>(0)?, r.get::<_, String>(1)?, r.get::<_, i64>(2)?))
            })
            .map_err(|e| e.to_string())?;
        for row in rows {
            let (day, scope, n) = row.map_err(|e| e.to_string())?;
            let Some(&i) = indices.get(&day) else { continue };
            let r = &mut result[i];
            match scope.as_str() {
                "site" => r.users = n,
                "ordo" => r.ordo = n,
                "reminders" => r.reminders = n,
                s if dimension_key(s).is_some() => {
                    r.dimensions.insert(scope, n);
                }
                s => {
                    if let Some(h) = HOURS.iter().position(|h| *h == s) {
                        r.hours[h] = n;
                    }
                }
            }
        }
        Ok(result)
    }
}

/// Counts in the record that were not readers. The store keeps what it was sent; the report
/// reads it through these.
const CORRECTIONS: render_html::usage::Corrections = render_html::usage::Corrections {
    // Play's pre-launch report installed the first uploads on its test phones, which opened every
    // page, before an installation had to be two hours old to report (Usage.kt's SETTLING).
    test_days: &[
        ("2026-10-11", "Google Play's test phones crawled the Android app's first upload: 75 of the day's 105"),
        ("2026-10-12", "Google Play's test phones were still crawling the Android app: 18 of the day's 40"),
    ],
    // The iOS app reached the App Store on 16 November 2026. Until then its only installs were
    // TestFlight's and App Review's, which stop reporting with the first build that carries
    // Usage.swift's storeInstall.
    released: &[(
        "ios",
        "2026-11-16",
        "it reached the App Store that day, and every earlier install was Apple’s review or a TestFlight test",
    )],
};

fn usage_headers(resp: &mut Response<Body>) {
    set(resp, header::CACHE_CONTROL, "no-store");
    set(resp, header::HeaderName::from_static("x-robots-tag"), "noindex, nofollow");
}

fn with_usage_headers(mut resp: Response<Body>) -> Response<Body> {
    usage_headers(&mut resp);
    resp
}

/// Accepts an HTTP(S) origin for this host, including its explicit port.
fn origin_matches_host(origin: &str, host: &str) -> bool {
    let Ok(uri) = origin.parse::<Uri>() else { return false };
    let Some(authority) = uri.authority() else { return false };
    // An Origin is a scheme and authority, not a full URL. Uri discards
    // fragments and accepts userinfo, so reject those explicitly.
    matches!(uri.scheme_str(), Some("http" | "https"))
        && !origin.contains(['#', '@'])
        && uri.path() == "/"
        && uri.query().is_none()
        && (authority.as_str() == authority.host() || authority.port_u16().is_some())
        && authority.as_str().eq_ignore_ascii_case(host)
}

fn random_id() -> Option<String> {
    let mut random = [0u8; 16];
    std::fs::File::open("/dev/urandom").and_then(|mut f| f.read_exact(&mut random)).ok()?;
    Some(random.iter().map(|b| format!("{b:02x}")).collect())
}

fn valid_browser_id(id: &str) -> bool {
    id.len() == 32 && id.bytes().all(|c| c.is_ascii_hexdigit())
}

pub use presentation::usage::MAX_BEACON;

/// The beacon body, or `None` when it exceeded [`MAX_BEACON`].
pub type BeaconBody = Option<Vec<u8>>;

/// Names the canonical host's endpoint on a counted beacon, so the native apps, which post to a
/// fixed address, move to the new one (see `presentation::usage::ENDPOINT_HEADER`).
fn advertise(resp: &mut Response<Body>, canonical: Option<&str>) {
    if let Some(endpoint) = canonical.and_then(|c| advertised_endpoint(&endpoint_for(c))) {
        set(resp, header::HeaderName::from_static(ENDPOINT_HEADER), &endpoint);
    }
}

/// Two flags, never a date, keep a browser's first day where Safari keeps it. The page's own mark
/// (`office-first-counted`) lives in script storage, which Safari erases after seven days without
/// a visit; cookies the server sets survive that. `office-seen` says the server has counted this
/// browser before (renewed on every beacon, so it outlasts its 400 days for a reader who keeps
/// coming); `office-new`, set beside it the first time, lapses at the reporting day's midnight. A
/// browser seen before and no longer new is returning whatever its page says. Each holds only
/// "1", the same for every browser, so nothing sent links one day to another.
const SEEN_COOKIE: &str = "office-seen";
const NEW_COOKIE: &str = "office-new";

/// A beacon's dimensions with its first visit made returning when the browser was counted on an
/// earlier reporting day: seen before, and its new flag lapsed.
fn settle_visit(mut dimensions: Vec<String>, seen: bool, new: bool) -> Vec<String> {
    if seen && !new && dimensions.iter().any(|d| d == "visit:first") {
        for d in &mut dimensions {
            if d == "visit:first" {
                *d = "visit:returning".to_string();
            }
        }
        // The page reckoned its tenure from the first day it lost: it is older than it says,
        // by how much the server cannot tell.
        dimensions.retain(|d| !d.starts_with("tenure:"));
    }
    dimensions
}

/// A week's and a month's flags, kept as the first day's are and for the same reason: the page's
/// own marks of the days it was counted (`office-counted`) are erased by Safari after a week
/// away, which would open a month twice, and are not written at all when the page is left before
/// a beacon's reply. Each is (the beacon's family, the flag that lasts the period, the flag that
/// lasts the day the period was opened). A browser holding a period's flag, and not on the day it
/// was given, has been counted in that period whatever its page says. Each holds only "1".
const PERIOD_COOKIES: [(&str, &str, &str); 2] =
    [("week", "office-week", "office-week-first"), ("month", "office-month", "office-month-first")];

/// A beacon's dimensions with a period's `first` made `again` when the browser was counted in
/// the period on an earlier day: it holds the period's flag, and its opening day's has lapsed.
fn settle_periods(mut dimensions: Vec<String>, held: [(bool, bool); 2]) -> Vec<String> {
    for ((key, ..), (period, opened_today)) in PERIOD_COOKIES.iter().zip(held) {
        if period && !opened_today {
            for d in &mut dimensions {
                if d.strip_suffix(":first") == Some(key) {
                    *d = format!("{key}:again");
                }
            }
            // The week before was told on the week's first day, if it was counted.
            if *key == "week" {
                dimensions.retain(|d| !d.starts_with("last-week:"));
            }
        }
    }
    dimensions
}

/// Seconds from `now` to the start of the reporting day `days` after today's, New York's midnight.
fn until_day(now: jiff::Timestamp, days: i64) -> i64 {
    let z = now.to_zoned(eastern());
    let next = z.date().checked_add(jiff::Span::new().days(days)).ok().and_then(|d| d.to_zoned(eastern()).ok());
    next.map_or(86_400 * days, |n| (n.timestamp().as_second() - now.as_second()).max(1))
}

/// Seconds from `now` to the next reporting day's start.
fn until_midnight(now: jiff::Timestamp) -> i64 {
    until_day(now, 1)
}

/// Seconds from `now` to the end of the reporting week (Monday's start) or month.
fn until_period_end(now: jiff::Timestamp, key: &str) -> i64 {
    let today = now.to_zoned(eastern()).date();
    let days = match key {
        "week" => 7 - i64::from(today.weekday().to_monday_zero_offset()),
        _ => i64::from(today.days_in_month() - today.day()) + 1,
    };
    until_day(now, days)
}

/// `POST /api/usage`: one page view's beacon. `canonical` is the site's canonical host, if set.
pub fn handle_event(
    store: Option<&Store>,
    method: &Method,
    headers: &HeaderMap,
    host: &str,
    canonical: Option<&str>,
    body: BeaconBody,
) -> Response<Body> {
    event_at(jiff::Timestamp::now(), store, method, headers, host, canonical, body)
}

fn event_at(
    now: jiff::Timestamp,
    store: Option<&Store>,
    method: &Method,
    headers: &HeaderMap,
    host: &str,
    canonical: Option<&str>,
    body: BeaconBody,
) -> Response<Body> {
    if method != Method::POST {
        let mut resp = with_usage_headers(http_error("Method not allowed", StatusCode::METHOD_NOT_ALLOWED));
        set(&mut resp, header::ALLOW, "POST");
        return resp;
    }
    // A custom header forces a CORS preflight, which this endpoint never
    // grants, and so rules out cross-origin form posts.
    if header_value(headers, "x-office-usage") != "1" || header_value(headers, "sec-fetch-site") == "cross-site" {
        return with_usage_headers(http_error("Forbidden", StatusCode::FORBIDDEN));
    }
    let origin = header_value(headers, "origin");
    if !origin.is_empty() && !origin_matches_host(origin, host) {
        return with_usage_headers(http_error("Forbidden", StatusCode::FORBIDDEN));
    }
    // Scraping is welcome; it just never counts.
    if is_bot(header_value(headers, "user-agent")) {
        return with_usage_headers(response(StatusCode::NO_CONTENT, Body::empty()));
    }
    let Some(body) = body else {
        let mut resp = with_usage_headers(http_error("Invalid office", StatusCode::BAD_REQUEST));
        set(&mut resp, header::CONNECTION, "close");
        return resp;
    };
    let Some(event) = parse_event(&String::from_utf8_lossy(&body)) else {
        return with_usage_headers(http_error("Invalid office", StatusCode::BAD_REQUEST));
    };
    let Some(store) = store else {
        let mut resp = with_usage_headers(response(StatusCode::NO_CONTENT, Body::empty()));
        advertise(&mut resp, canonical);
        return resp;
    };
    const COOKIE: &str = "office-usage";
    let mut resp = response(StatusCode::NO_CONTENT, Body::empty());
    let secure = if header_value(headers, "x-forwarded-proto") == "https" { "; Secure" } else { "" };
    let seen = cookie(headers, SEEN_COOKIE).as_deref() == Some("1");
    let new = cookie(headers, NEW_COOKIE).as_deref() == Some("1");
    let flag = |name: &str| cookie(headers, name).as_deref() == Some("1");
    let periods = PERIOD_COOKIES.map(|(_, period, opened)| (flag(period), flag(opened)));
    let dimensions = settle_periods(settle_visit(event.dimensions, seen, new), periods);
    // Only a browser sends an Origin; the apps keep their own first day and take no cookies.
    if !origin.is_empty() {
        let mut flags = vec![format!("{SEEN_COOKIE}=1; Path=/api/usage; Max-Age=34560000; HttpOnly{secure}; SameSite=Strict")];
        if !seen {
            flags.push(format!("{NEW_COOKIE}=1; Path=/api/usage; Max-Age={}; HttpOnly{secure}; SameSite=Strict", until_midnight(now)));
        }
        // A period's flags are given with the first beacon that speaks of it, so a page too old
        // to say is not later taken for one already counted.
        for ((key, period, opened), (held, _)) in PERIOD_COOKIES.iter().zip(periods) {
            if !held && dimensions.iter().any(|d| d.split_once(':').is_some_and(|(family, _)| family == *key)) {
                for (name, age) in [(period, until_period_end(now, key)), (opened, until_midnight(now))] {
                    flags.push(format!("{name}=1; Path=/api/usage; Max-Age={age}; HttpOnly{secure}; SameSite=Strict"));
                }
            }
        }
        for flag in flags {
            if let Ok(v) = header::HeaderValue::from_str(&flag) {
                resp.headers_mut().append(header::SET_COOKIE, v);
            }
        }
    }
    // The identifier lasts the reporting day and no longer, as the apps' does: it is set, or an
    // older month-long one cut short, to lapse at New York's midnight, so a browser sends
    // nothing that links one day to another.
    let held = cookie(headers, COOKIE).filter(|id| valid_browser_id(id));
    let id = match held.clone().or_else(random_id) {
        Some(id) => id,
        None => return with_usage_headers(response(StatusCode::SERVICE_UNAVAILABLE, Body::empty())),
    };
    if held.is_none() || !origin.is_empty() {
        let value = format!("{COOKIE}={id}; Path=/api/usage; Max-Age={}; HttpOnly{secure}; SameSite=Strict", until_midnight(now));
        if let Ok(v) = header::HeaderValue::from_str(&value) {
            resp.headers_mut().append(header::SET_COOKIE, v);
        }
    }
    if store.record(now, &id, &event.scope, &dimensions).is_err() {
        let mut failed = response(StatusCode::SERVICE_UNAVAILABLE, Body::empty());
        // A period's flags say the browser was counted in it, and it was not.
        let period = |c: &header::HeaderValue| PERIOD_COOKIES.iter().any(|(_, name, _)| c.as_bytes().starts_with(name.as_bytes()));
        for c in resp.headers().get_all(header::SET_COOKIE).iter().filter(|c| !period(c)) {
            failed.headers_mut().append(header::SET_COOKIE, c.clone());
        }
        return with_usage_headers(failed);
    }
    advertise(&mut resp, canonical);
    with_usage_headers(resp)
}

/// `GET /admin/usage`: the daily report.
pub fn handle_dashboard(store: Option<&Store>, pages: &render_html::Pages, method: &Method, query: &Query) -> Response<Body> {
    if method != Method::GET && method != Method::HEAD {
        let mut resp = with_usage_headers(http_error("Method not allowed", StatusCode::METHOD_NOT_ALLOWED));
        set(&mut resp, header::ALLOW, "GET, HEAD");
        return resp;
    }
    let Some(store) = store else { return with_usage_headers(not_found()) };
    let mut days = 30;
    let raw = query.get("days");
    if !raw.is_empty() {
        match data_format::atoi(raw) {
            Ok(n) if render_html::usage::WINDOWS.contains(&n) => days = n,
            _ => return with_usage_headers(http_error("Choose 7, 30, 90 or 365 days", StatusCode::BAD_REQUEST)),
        }
    }
    let lookback = days as usize + render_html::usage::LOOKBACK;
    let (Ok(rows), Ok(since)) = (store.daily(jiff::Timestamp::now(), lookback), store.first_day()) else {
        return with_usage_headers(http_error("Usage temporarily unavailable", StatusCode::SERVICE_UNAVAILABLE));
    };
    let data = render_html::usage::usage_data(rows, days, since.as_deref(), &DIMENSIONS, &CORRECTIONS);
    let Ok(body) = pages.usage(&data) else {
        return with_usage_headers(http_error("Unable to render usage", StatusCode::INTERNAL_SERVER_ERROR));
    };
    let mut resp = response(StatusCode::OK, body);
    set(&mut resp, header::CONTENT_TYPE, "text/html; charset=utf-8");
    with_usage_headers(resp)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn beacons_require_matching_http_origins() {
        for (host, origin, allowed) in [
            ("office.example", "https://office.example", true),
            ("office.example", "HTTP://office.example", true),
            ("OFFICE.EXAMPLE", "https://office.example", true),
            ("localhost:18159", "http://localhost:18159", true),
            ("office.example:8443", "https://office.example:8443", true),
            ("[::1]:18159", "http://[::1]:18159", true),
            ("office.example", "", true), // Non-browser clients may omit Origin.
            ("office.example", "null", false),
            ("office.example", "https://other.example", false),
            ("office.example", "https://office.example.evil.test", false),
            ("office.example:8443", "https://office.example:9443", false),
            ("office.example", "ftp://office.example", false),
            ("office.example", "//office.example", false),
            ("office.example", "https://user@office.example", false),
            ("office.example", "https://office.example/path", false),
            ("office.example", "https://office.example?query", false),
            ("office.example", "https://office.example#fragment", false),
            ("office.example:x", "https://office.example:x", false),
            ("office.example:99999", "https://office.example:99999", false),
            ("office.example", "https://office.example https://other.example", false),
        ] {
            let mut headers = HeaderMap::new();
            headers.insert("x-office-usage", "1".parse().unwrap());
            headers.insert(header::ORIGIN, origin.parse().unwrap());
            let response = handle_event(None, &Method::POST, &headers, host, None, Some(b"lauds".to_vec()));
            let expected = if allowed { StatusCode::NO_CONTENT } else { StatusCode::FORBIDDEN };
            assert_eq!(response.status(), expected, "host={host}, origin={origin}");
        }
    }

    #[test]
    fn bots_and_people() {
        for agent in [
            "",
            "   ",
            "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
            "Mozilla/5.0 (compatible; bingbot/2.0; +http://www.bing.com/bingbot.htm)",
            "Mozilla/5.0 (compatible; AhrefsBot/7.0; +http://ahrefs.com/robot/)",
            "Mozilla/5.0 (compatible; SemrushBot/7~bl; +http://www.semrush.com/bot.html)",
            "GPTBot/1.1 (+https://openai.com/gptbot)",
            "Mozilla/5.0 (compatible; ClaudeBot/1.0; +claudebot@anthropic.com)",
            "Mozilla/5.0 (compatible; PerplexityBot/1.0)",
            "Mozilla/5.0 AppleWebKit/537.36 (KHTML, like Gecko) HeadlessChrome/141.0.0.0 Safari/537.36",
            "python-requests/2.32.3",
            "curl/8.7.1",
            "Wget/1.21.4",
            "Go-http-client/2.0",
            "Scrapy/2.11 (+https://scrapy.org)",
            "facebookexternalhit/1.1",
            "Mozilla/5.0 (compatible; YandexBot/3.0)",
            // An unknown crawler in the "<name>bot/<version>" form.
            "Mozilla/5.0 (compatible; NewfangledBot/0.3; +https://example.test)",
        ] {
            assert!(is_bot(agent), "is_bot({agent:?})");
        }
        // Several phone makers put "bot" inside a model name.
        for agent in [
            "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36",
            "Mozilla/5.0 (iPhone; CPU iPhone OS 18_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.1 Mobile/15E148 Safari/604.1",
            "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:133.0) Gecko/20100101 Firefox/133.0",
            "Mozilla/5.0 (Linux; Android 13; Cubot Note 20) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Mobile Safari/537.36",
            "Mozilla/5.0 (Linux; Android 10; CUBOT_X30) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Mobile Safari/537.36",
            "Mozilla/5.0 (Linux; Android 14; Abbot One) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Mobile Safari/537.36",
            "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.6 Safari/605.1.15",
        ] {
            assert!(!is_bot(agent), "is_bot({agent:?})");
        }
    }

    struct TempDb(std::path::PathBuf);

    impl TempDb {
        fn new(name: &str) -> TempDb {
            let dir = std::env::temp_dir().join(format!("office-usage-{name}-{}", std::process::id()));
            std::fs::create_dir_all(&dir).unwrap();
            TempDb(dir.join("usage.sqlite"))
        }

        fn open(&self) -> Store {
            Store::open(self.0.to_str().unwrap()).unwrap()
        }
    }

    impl Drop for TempDb {
        fn drop(&mut self) {
            if let Some(dir) = self.0.parent() {
                let _ = std::fs::remove_dir_all(dir);
            }
        }
    }

    fn eastern_at(y: i16, m: i8, d: i8, h: i8, min: i8) -> jiff::Timestamp {
        jiff::civil::date(y, m, d).at(h, min, 0, 0).to_zoned(eastern()).unwrap().timestamp()
    }

    fn count(store: &Store, sql: &str) -> i64 {
        store.conn.lock().unwrap().query_row(sql, [], |r| r.get(0)).unwrap()
    }

    fn dims(v: &[&str]) -> Vec<String> {
        v.iter().map(|s| s.to_string()).collect()
    }

    #[test]
    fn daily_deduplication_and_persistence() {
        let db = TempDb::new("dedupe");
        let store = db.open();
        let now = eastern_at(2026, 9, 4, 23, 59);
        assert_eq!(store.first_day().unwrap(), None, "empty store has no first day");
        std::thread::scope(|s| {
            for _ in 0..12 {
                s.spawn(|| store.record(now, "browser-a", "lauds", &[]).unwrap());
            }
        });
        for (id, scope) in
            [("browser-a", "vespers"), ("browser-b", "lauds"), ("browser-c", "site"), ("browser-d", "ordo"), ("browser-e", "reminders")]
        {
            store.record(now, id, scope, &[]).unwrap();
        }
        drop(store);
        let store = db.open();
        let rows = store.daily(now, 7).unwrap();
        assert!(
            rows[0].users == 5
                && rows[0].hours[0] == 2
                && rows[0].hours[5] == 1
                && rows[0].ordo == 1
                && rows[0].reminders == 1
                && rows[1].users == 0,
            "counts: {rows:?}"
        );
        let tomorrow = now.checked_add(jiff::SignedDuration::from_mins(2)).unwrap();
        store.record(tomorrow, "browser-a", "lauds", &[]).unwrap();
        assert_eq!(store.first_day().unwrap().as_deref(), Some("2026-09-04"), "first recorded day");
        let rows = store.daily(tomorrow, 7).unwrap();
        assert!(rows[0].day == "2026-09-05" && rows[0].users == 1 && rows[1].users == 5, "midnight counts: {rows:?}");
        assert_eq!(
            count(&store, "SELECT COUNT(DISTINCT browser) FROM seen WHERE scope='vespers' OR day='2026-09-05'"),
            2,
            "daily hashes were reused"
        );
        let future = eastern_at(2026, 9, 8, 23, 59);
        let rows = store.daily(future, 7).unwrap();
        assert_eq!(count(&store, "SELECT COUNT(*) FROM seen"), 0, "retention kept identifiers");
        assert_eq!(rows[4].users, 5, "retention lost totals");
    }

    #[test]
    fn reporting_day_across_dst() {
        for (value, utc_day) in [("2026-03-08T04:59:00Z", "2026-03-08"), ("2026-11-01T03:59:00Z", "2026-11-01")] {
            let now: jiff::Timestamp = value.parse().unwrap();
            assert_ne!(presentation::date_slug(eastern_day(now)), utc_day, "used UTC instead of Eastern: {value}");
        }
    }

    // Every stored family has a report breakdown.
    #[test]
    fn trend_dimensions_cover_the_vocabulary() {
        use render_html::usage::{HOUR_FAMILIES, OFFICES_FOLD, TREND_LABELS};
        assert_eq!(TREND_LABELS.len() + HOUR_FAMILIES.len(), DIMENSIONS.len());
        for key in HOUR_FAMILIES {
            assert!(TREND_LABELS.iter().all(|(k, ..)| *k != key) && DIMENSIONS.iter().any(|d| d.key == key), "{key}");
        }
        for (key, _, labels) in TREND_LABELS {
            let dimension = DIMENSIONS.iter().find(|d| d.key == key).unwrap_or_else(|| panic!("unmapped {key}"));
            // The hours opened in a day are drawn with the larger counts as one series.
            let series = if key.ends_with("offices") { OFFICES_FOLD } else { dimension.values.len() };
            assert_eq!(series, labels.len(), "{key}");
            assert!(labels.iter().all(|l| !l.is_empty()), "{key}");
        }
    }

    // Each reader sits under the number of hours opened so far that day, whatever else they open.
    #[test]
    fn hours_opened_are_counted_per_reader() {
        let db = TempDb::new("offices");
        let store = db.open();
        let now = eastern_at(2026, 9, 4, 9, 0);
        for (browser, scope) in [
            ("a", "site"),
            ("a", "lauds"),
            ("a", "lauds"),
            ("a", "ordo"),
            ("a", "prime"),
            ("a", "vespers"),
            ("b", "compline"),
            ("c", "site"),
            ("d", "reminders"),
        ] {
            store.record(now, browser, scope, &[]).unwrap();
        }
        let row = &store.daily(now, 1).unwrap()[0];
        let offices: Vec<(&str, i64)> =
            row.dimensions.iter().filter(|(k, _)| k.starts_with("offices:")).map(|(k, v)| (k.as_str(), *v)).collect();
        assert_eq!(offices, [("offices:1", 1), ("offices:2", 0), ("offices:3", 1)]);
        assert_eq!(row.users, 4);
        // The next day starts again from nothing.
        let tomorrow = eastern_at(2026, 9, 5, 9, 0);
        store.record(tomorrow, "a", "lauds", &[]).unwrap();
        assert_eq!(store.daily(tomorrow, 1).unwrap()[0].dimensions.get("offices:1"), Some(&1));
        // A beacon cannot claim a count of its own.
        assert!(parse_event("lauds offices:7").unwrap().dimensions.is_empty());
    }

    // The hours a reporting client opened are the denominator of those it opened at their time
    // and prayed; a new reader's hours are counted again among the new.
    #[test]
    fn reporting_readers_hours_and_new_readers_hours() {
        let db = TempDb::new("habits");
        let store = db.open();
        let now = eastern_at(2026, 11, 18, 9, 0);
        for (browser, body) in [
            ("a", "lauds visit:first version:2 timely:lauds"),
            ("a", "lauds visit:first version:2 timely:lauds prayed:lauds"),
            ("a", "prime visit:first version:2"),
            ("b", "lauds visit:returning"),
            ("c", "vespers visit:returning version:2 prayed:vespers offline:yes"),
            ("c", "site visit:returning version:2"),
        ] {
            let event = parse_event(body).unwrap();
            store.record(now, browser, &event.scope, &event.dimensions).unwrap();
        }
        let row = &store.daily(now, 1).unwrap()[0];
        let got = |scope: &str| row.dimensions.get(scope).copied().unwrap_or(0);
        assert_eq!((row.hours[0], got("reported:lauds"), got("timely:lauds"), got("prayed:lauds")), (2, 1, 1, 1));
        assert_eq!((got("reported:prime"), got("reported:vespers"), got("prayed:vespers"), got("offline:yes")), (1, 1, 1, 1));
        assert_eq!((got("offices:1"), got("offices:2")), (2, 1));
        assert_eq!((got("new-offices:1"), got("new-offices:2")), (0, 1));
        // What the server learns from its own flags overrules a page that lost its marks.
        let dims = |v: &[&str]| v.iter().map(|s| s.to_string()).collect::<Vec<_>>();
        assert_eq!(
            settle_visit(dims(&["visit:first", "tenure:week", "client:browser"]), true, false),
            dims(&["visit:returning", "client:browser"])
        );
        assert_eq!(settle_visit(dims(&["visit:first", "tenure:week"]), true, true), dims(&["visit:first", "tenure:week"]));
        assert_eq!(settle_visit(dims(&["visit:returning", "tenure:month"]), true, false), dims(&["visit:returning", "tenure:month"]));
        assert_eq!(
            settle_periods(dims(&["week:first", "last-week:2-3", "month:first"]), [(true, false), (false, false)]),
            dims(&["week:again", "month:first"])
        );
        assert_eq!(
            settle_periods(dims(&["week:first", "last-week:2-3"]), [(true, true), (true, false)]),
            dims(&["week:first", "last-week:2-3"])
        );
    }

    // A week's first days add up to its readers: each browser says "first" on one day of the week.
    #[test]
    fn a_weeks_first_days_add_up_to_its_readers() {
        let db = TempDb::new("weeks");
        let store = db.open();
        // Monday 16 to Wednesday 18 November 2026.
        for (day, browser, token) in [
            (16, "a", "week:first"),
            (16, "a", "week:first"),
            (16, "b", "week:first"),
            (17, "a", "week:again"),
            (17, "c", "week:first"),
            (18, "b", "week:again"),
            (18, "c", "week:again"),
        ] {
            let event = parse_event(&format!("site {token} month:again")).unwrap();
            store.record(eastern_at(2026, 11, day, 9, 0), browser, &event.scope, &event.dimensions).unwrap();
        }
        let rows = store.daily(eastern_at(2026, 11, 18, 10, 0), 3).unwrap();
        let first = |i: usize| rows[i].dimensions.get("week:first").copied().unwrap_or(0);
        assert_eq!([first(2), first(1), first(0)], [2, 1, 0]);
        assert_eq!(rows.iter().map(|r| r.users).sum::<i64>(), 6, "reader-days");
    }

    #[test]
    fn dimensions_count_once_per_browser_per_day() {
        let db = TempDb::new("dimensions");
        let store = db.open();
        let now = eastern_at(2026, 9, 4, 9, 0);
        // One reader praying two hours counts once in each dimension.
        for hour in ["lauds", "vespers"] {
            store.record(now, "browser-a", hour, &dims(&["appearance:apse", "screen:mobile"])).unwrap();
        }
        store.record(now, "browser-b", "lauds", &dims(&["appearance:nave", "screen:desktop"])).unwrap();
        // A browser without the new app.js still counts overall.
        store.record(now, "browser-c", "lauds", &[]).unwrap();
        for bad in ["narthex:vault", "apse"] {
            assert!(store.record(now, "browser-d", "lauds", &dims(&[bad])).is_err(), "accepted {bad:?}");
        }
        let rows = store.daily(now, 7).unwrap();
        let d = |r: &UsageDay, k: &str| r.dimensions.get(k).copied().unwrap_or(0);
        assert!(rows[0].users == 3 && rows[0].hours[0] == 3, "totals: {:?}", rows[0]);
        for k in ["appearance:apse", "screen:mobile", "appearance:nave", "screen:desktop"] {
            assert_eq!(d(&rows[0], k), 1, "{k}");
        }
        assert_eq!(d(&rows[1], "appearance:nave"), 0);
        // The silent client is in the total but in neither appearance.
        assert!(d(&rows[0], "appearance:nave") + d(&rows[0], "appearance:apse") < rows[0].users);
        // Switching appearance during the day counts on both sides.
        store.record(eastern_at(2026, 9, 4, 10, 0), "browser-a", "compline", &dims(&["appearance:nave", "screen:mobile"])).unwrap();
        let rows = store.daily(now, 7).unwrap();
        assert!(
            rows[0].users == 3
                && d(&rows[0], "appearance:nave") == 2
                && d(&rows[0], "appearance:apse") == 1
                && d(&rows[0], "screen:mobile") == 1,
            "appearance switch: {:?}",
            rows[0]
        );
    }

    // The native apps post as non-browser clients: no Origin, their own daily identifier as the
    // cookie, and an agent naming the app.
    #[test]
    fn app_beacons_count_under_their_own_identifier() {
        use presentation::usage::{App, Habits, app_beacon};
        let db = TempDb::new("apps");
        let store = db.open();
        for (id, agent, app) in [
            ("0123456789abcdef0123456789abcdef", "DivineOffice/0.1.1234 (Android 15)", App::Android),
            ("fedcba9876543210fedcba9876543210", "DivineOffice/0.1.0 (iOS 18.1)", App::Ios),
        ] {
            assert!(!is_bot(agent), "{agent}");
            let mut headers = HeaderMap::new();
            headers.insert("x-office-usage", "1".parse().unwrap());
            headers.insert(header::USER_AGENT, agent.parse().unwrap());
            headers.insert(header::COOKIE, format!("office-usage={id}").parse().unwrap());
            let body = app_beacon(
                "lauds",
                app,
                false,
                "priest",
                None,
                Some(app == App::Ios),
                Some(true),
                Some(true),
                Some(false),
                Some(&Habits::default()),
            )
            .unwrap()
            .into_bytes();
            let response = handle_event(Some(&store), &Method::POST, &headers, "office.fly.dev", None, Some(body.clone()));
            assert_eq!(response.status(), StatusCode::NO_CONTENT, "{agent}");
            assert!(response.headers().get(header::SET_COOKIE).is_none(), "{agent} was given a cookie");
            assert!(response.headers().get(ENDPOINT_HEADER).is_none(), "{agent} told to move with nowhere to go");
            // Once the site has a canonical host, the old one tells each app where to report.
            let moved = handle_event(Some(&store), &Method::POST, &headers, "office.fly.dev", Some("example.org"), Some(body));
            assert_eq!(moved.status(), StatusCode::NO_CONTENT, "{agent}");
            assert_eq!(moved.headers()[ENDPOINT_HEADER], "https://example.org/api/usage", "{agent}");
        }
        let sum = |scope: &str| count(&store, &format!("SELECT COALESCE(SUM(users), 0) FROM totals WHERE scope = '{scope}'"));
        for (scope, n) in [
            ("site", 2),
            ("lauds", 2),
            ("client:android", 1),
            ("client:ios", 1),
            ("screen:mobile", 2),
            ("prayer-form:priest", 2),
            ("visit:first", 2),
            ("chant:on", 1),
            ("chant:off", 1),
            ("new-client:android", 1),
            ("new-client:ios", 1),
        ] {
            assert_eq!(sum(scope), n, "{scope}");
        }
    }

    // Safari erases a page's storage after seven days without a visit, so a weekly reader's page
    // says it is new again; two server-set flags remember that the browser was counted before.
    #[test]
    fn a_browser_counted_on_an_earlier_day_is_returning_whatever_its_page_says() {
        let db = TempDb::new("seen");
        let store = db.open();
        // 9 p.m. in New York: three hours to the reporting day's end.
        let now = eastern_at(2026, 11, 11, 21, 0);
        let post = |cookies: &str, body: &str| {
            let mut headers = HeaderMap::new();
            headers.insert("x-office-usage", "1".parse().unwrap());
            headers.insert(header::ORIGIN, "https://office.example".parse().unwrap());
            headers.insert("x-forwarded-proto", "https".parse().unwrap());
            headers.insert(header::USER_AGENT, "Mozilla/5.0 (iPhone; CPU iPhone OS 18_1 like Mac OS X) Safari/604.1".parse().unwrap());
            if !cookies.is_empty() {
                headers.insert(header::COOKIE, cookies.parse().unwrap());
            }
            event_at(now, Some(&store), &Method::POST, &headers, "office.example", None, Some(body.as_bytes().to_vec()))
        };
        let flags = |r: &Response<Body>| {
            r.headers()
                .get_all(header::SET_COOKIE)
                .iter()
                .map(|v| v.to_str().unwrap().to_string())
                .filter(|c| !c.starts_with("office-usage="))
                .collect::<Vec<_>>()
        };
        let seen = "office-seen=1; Path=/api/usage; Max-Age=34560000; HttpOnly; Secure; SameSite=Strict";
        // A browser the server has not met is given both flags, the new one until midnight.
        let fresh = post("", "lauds visit:first client:browser");
        assert_eq!(flags(&fresh), [seen, "office-new=1; Path=/api/usage; Max-Age=10800; HttpOnly; Secure; SameSite=Strict"]);
        // Its identifier lasts the reporting day and no longer, as the flag does.
        let identifier = |r: &Response<Body>| {
            r.headers().get_all(header::SET_COOKIE).iter().map(|v| v.to_str().unwrap().to_string()).find(|c| c.starts_with("office-usage="))
        };
        let minted = identifier(&fresh).expect("no identifier set");
        assert!(minted.ends_with("; Path=/api/usage; Max-Age=10800; HttpOnly; Secure; SameSite=Strict"), "{minted}");
        // Later the same day it is still new.
        let later = post("office-usage=fedcba9876543210fedcba9876543210; office-seen=1; office-new=1", "site visit:first client:pwa");
        assert_eq!(flags(&later), [seen]);
        // A month-long identifier from before is cut short at midnight, and kept until then.
        assert_eq!(
            identifier(&later).as_deref(),
            Some("office-usage=fedcba9876543210fedcba9876543210; Path=/api/usage; Max-Age=10800; HttpOnly; Secure; SameSite=Strict")
        );
        // Seen on an earlier day, its storage since erased: returning, and the seen flag renewed.
        let erased = post("office-usage=0123456789abcdef0123456789abcdef; office-seen=1", "vespers visit:first client:browser");
        assert_eq!(flags(&erased), [seen]);
        // A garbled flag is no flag.
        let garbled = post("office-usage=00112233445566778899aabbccddeeff; office-seen=2026-11-02", "site visit:first client:browser");
        assert_eq!(flags(&garbled).len(), 2);
        let sum = |scope: &str| count(&store, &format!("SELECT COALESCE(SUM(users), 0) FROM totals WHERE scope = '{scope}'"));
        for (scope, n) in [("site", 4), ("visit:first", 3), ("visit:returning", 1), ("new-client:browser", 2), ("new-client:pwa", 1)] {
            assert_eq!(sum(scope), n, "{scope}");
        }
    }

    // The page's own marks of the days it was counted are lost when Safari erases its storage, or
    // never written when the page is left before a beacon's reply; the server's flags hold.
    #[test]
    fn a_browser_counted_in_a_week_or_month_is_not_first_again_whatever_its_page_says() {
        let db = TempDb::new("periods");
        let store = db.open();
        // Wednesday 11 November 2026, 9 p.m. in New York: the week ends in four days and three
        // hours, the month in nineteen days and three hours.
        let post = |day: i8, cookies: &str, body: &str| {
            let mut headers = HeaderMap::new();
            headers.insert("x-office-usage", "1".parse().unwrap());
            headers.insert(header::ORIGIN, "https://office.example".parse().unwrap());
            headers.insert(header::USER_AGENT, "Mozilla/5.0 (iPhone; CPU iPhone OS 18_1 like Mac OS X) Safari/604.1".parse().unwrap());
            headers.insert(header::COOKIE, format!("office-seen=1{cookies}").parse().unwrap());
            let now = eastern_at(2026, 11, day, 21, 0);
            event_at(now, Some(&store), &Method::POST, &headers, "office.example", None, Some(body.as_bytes().to_vec()))
        };
        let flags = |r: &Response<Body>| {
            r.headers()
                .get_all(header::SET_COOKIE)
                .iter()
                .map(|v| v.to_str().unwrap().to_string())
                .filter(|c| c.starts_with("office-week") || c.starts_with("office-month"))
                .collect::<Vec<_>>()
        };
        let id = "; office-usage=0123456789abcdef0123456789abcdef";
        let fresh = post(11, id, "site week:first month:first");
        assert_eq!(
            flags(&fresh),
            [
                "office-week=1; Path=/api/usage; Max-Age=356400; HttpOnly; SameSite=Strict",
                "office-week-first=1; Path=/api/usage; Max-Age=10800; HttpOnly; SameSite=Strict",
                "office-month=1; Path=/api/usage; Max-Age=1652400; HttpOnly; SameSite=Strict",
                "office-month-first=1; Path=/api/usage; Max-Age=10800; HttpOnly; SameSite=Strict",
            ]
        );
        // Later the same day its page still says first, and it is: nothing is set again.
        let all = "; office-week=1; office-week-first=1; office-month=1; office-month-first=1";
        assert!(flags(&post(11, &format!("{id}{all}"), "lauds week:first month:first")).is_empty());
        // The next day, its marks never written: again in both.
        assert!(flags(&post(12, "; office-week=1; office-month=1", "site week:first month:first")).is_empty());
        // The next week, its storage erased: the week opens, and its flags are given; the month does not.
        let next = post(20, "; office-month=1", "site week:first month:first");
        assert_eq!(flags(&next).len(), 2, "{:?}", flags(&next));
        // A page too old to say is given no flag, so it is not taken for counted when it updates.
        assert!(flags(&post(21, "", "site client:browser")).is_empty());
        let sum = |scope: &str| count(&store, &format!("SELECT COALESCE(SUM(users), 0) FROM totals WHERE scope = '{scope}'"));
        for (scope, n) in [("week:first", 2), ("week:again", 1), ("month:first", 1), ("month:again", 2)] {
            assert_eq!(sum(scope), n, "{scope}");
        }
        assert_eq!(until_period_end(eastern_at(2026, 11, 15, 23, 0), "week"), 3600, "Sunday night");
        assert_eq!(until_period_end(eastern_at(2026, 11, 16, 0, 0), "week"), 7 * 86_400, "Monday");
        assert_eq!(until_period_end(eastern_at(2026, 11, 30, 23, 0), "month"), 3600);
        assert_eq!(until_period_end(eastern_at(2026, 12, 1, 0, 0), "month"), 31 * 86_400);
    }

    // On the day the server begins keeping hours per reader, an hour opened before it began left
    // no count to move: nothing goes below zero, and another reader's count is the worst it takes.
    #[test]
    fn hours_opened_before_the_count_began_take_nothing_below_zero() {
        let db = TempDb::new("offices-first-day");
        let store = db.open();
        let now = eastern_at(2026, 9, 4, 9, 0);
        store.record(now, "a", "lauds", &[]).unwrap();
        store.conn.lock().unwrap().execute("DELETE FROM totals WHERE scope LIKE 'offices:%'", []).unwrap();
        store.record(now, "a", "vespers", &[]).unwrap();
        let row = &store.daily(now, 1).unwrap()[0];
        let offices: Vec<(&str, i64)> =
            row.dimensions.iter().filter(|(k, _)| k.starts_with("offices:")).map(|(k, v)| (k.as_str(), *v)).collect();
        assert_eq!(offices, [("offices:2", 1)]);
        // The report's longest window and the days its summary reads fit the store's limit.
        let longest = *render_html::usage::WINDOWS.iter().max().unwrap() as usize + render_html::usage::LOOKBACK;
        assert_eq!(store.daily(now, longest).unwrap().len(), longest);
        assert!(store.daily(now, longest + 1).is_err());
    }

    #[test]
    fn the_new_flag_lapses_at_new_york_midnight() {
        assert_eq!(until_midnight(eastern_at(2026, 11, 11, 23, 59)), 60);
        assert_eq!(until_midnight(eastern_at(2026, 11, 11, 0, 0)), 86_400);
        // The night the clocks go back is 25 hours long.
        assert_eq!(until_midnight(eastern_at(2026, 11, 1, 0, 0)), 90_000);
        let dims = |v: &[&str]| v.iter().map(|s| s.to_string()).collect::<Vec<_>>();
        let first = dims(&["visit:first", "client:browser"]);
        assert_eq!(settle_visit(first.clone(), true, false), dims(&["visit:returning", "client:browser"]));
        assert_eq!(settle_visit(first.clone(), true, true), first);
        assert_eq!(settle_visit(first.clone(), false, false), first);
    }

    #[test]
    fn prayer_form_dimension_counts_forms_without_inflating_visits() {
        let db = TempDb::new("forms");
        let store = db.open();
        let now: jiff::Timestamp = "2026-09-10T12:00:00Z".parse().unwrap();
        for body in [
            "lauds prayer-form:private",
            "lauds prayer-form:private",
            "lauds prayer-form:priest",
            "vespers prayer-form:deacon",
            "site prayer-form:private",
            "lauds prayer-form:unknown",
        ] {
            let event = parse_event(body).unwrap();
            store.record(now, "one-browser", &event.scope, &event.dimensions).unwrap();
        }
        let row = &store.daily(now, 1).unwrap()[0];
        assert!(row.users == 1 && row.hours[0] == 1 && row.hours[5] == 1, "inflated visits: {row:?}");
        for form in ["private", "deacon", "priest"] {
            assert_eq!(row.dimensions.get(&format!("prayer-form:{form}")), Some(&1), "{form}");
        }
        for scope in ["site", "ordo", "reminders"] {
            assert!(parse_event(&format!("{scope} prayer-form:priest")).unwrap().dimensions.is_empty(), "form counted without an office");
        }
    }
}
