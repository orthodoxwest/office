//! Approximate daily browser counts, not request logs, and the two endpoints that write and read
//! them.

use std::collections::BTreeMap;
use std::io::Read as _;
use std::sync::Mutex;
use std::time::Duration;

use axum::body::Body;
use axum::http::{HeaderMap, Method, Response, StatusCode, Uri, header};
use calendar::Date;
use render_html::usage::{Dimension, HOURS, UsageDay};
use rusqlite::{Connection, params};
use sha2::{Digest, Sha256};

use crate::http::{Query, cookie, header_value, http_error, not_found, response, set};

/// Single-purpose scopes counted beside the hours: the ordo (calendar) page
/// and a generated reminder-feed link.
const EXTRA_SCOPES: [&str; 2] = ["ordo", "reminders"];

/// Families of mutually exclusive values describing how a page was
/// rendered. Stored as "<key>:<value>"; a written key or value is never
/// redefined.
pub const DIMENSIONS: [Dimension<'static>; 3] = [
    Dimension { key: "appearance", values: &["nave", "apse"] },
    Dimension { key: "screen", values: &["desktop", "mobile"] },
    Dimension { key: "prayer-form", values: &["private", "deacon", "priest"] },
];

/// The family a stored dimension scope belongs to.
fn dimension_key(scope: &str) -> Option<&'static str> {
    DIMENSIONS
        .iter()
        .find(|d| d.values.iter().any(|v| scope.strip_prefix(d.key).and_then(|r| r.strip_prefix(':')) == Some(v)))
        .map(|d| d.key)
}

pub fn valid_scope(scope: &str) -> bool {
    scope == "site" || HOURS.contains(&scope) || EXTRA_SCOPES.contains(&scope)
}

/// One beacon: the page scope and the dimensions reported about it.
#[derive(Debug, PartialEq, Eq)]
pub struct Event {
    pub scope: String,
    pub dimensions: Vec<String>,
}

/// Reads a beacon body ("lauds appearance:apse screen:mobile"). Only the
/// scope must be understood; unknown or repeated dimension tokens are
/// dropped, so clients from either side of a deploy still count.
pub fn parse_event(body: &str) -> Option<Event> {
    let mut fields = body.split(' ');
    let scope = fields.next().unwrap_or("");
    if !valid_scope(scope) {
        return None;
    }
    let mut event = Event { scope: scope.to_string(), dimensions: Vec::new() };
    let mut seen = Vec::new();
    for field in fields {
        let Some(key) = dimension_key(field) else { continue };
        if seen.contains(&key) || (key == "prayer-form" && !HOURS.contains(&scope)) {
            continue;
        }
        seen.push(key);
        event.dimensions.push(field.to_string());
    }
    Some(event)
}

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
        let day = crate::web_time::date_slug(today);
        let hash = Sha256::digest(format!("{day}\x00{browser}").as_bytes());
        let mut conn = self.conn.lock().map_err(|e| e.to_string())?;
        let tx = conn.transaction().map_err(|e| e.to_string())?;
        tx.execute("DELETE FROM seen WHERE day < ?", [crate::web_time::date_slug(today.add_days(-2))]).map_err(|e| e.to_string())?;
        let mut scopes = vec!["site"];
        if scope != "site" {
            scopes.push(scope);
        }
        scopes.extend(dimensions.iter().map(String::as_str));
        for v in scopes {
            let n =
                tx.execute("INSERT OR IGNORE INTO seen VALUES (?, ?, ?)", params![day, hash.as_slice(), v]).map_err(|e| e.to_string())?;
            if n == 0 {
                continue;
            }
            tx.execute("INSERT INTO totals VALUES (?, ?, 1)\nON CONFLICT(day, scope) DO UPDATE SET users = users + 1", params![day, v])
                .map_err(|e| e.to_string())?;
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
        if !(1..=366).contains(&days) {
            return Err("invalid day window".into());
        }
        let today = eastern_day(now);
        let conn = self.conn.lock().map_err(|e| e.to_string())?;
        conn.execute("DELETE FROM seen WHERE day < ?", [crate::web_time::date_slug(today.add_days(-2))]).map_err(|e| e.to_string())?;
        let mut result: Vec<UsageDay> =
            (0..days).map(|i| UsageDay { day: crate::web_time::date_slug(today.add_days(-(i as i32))), ..UsageDay::default() }).collect();
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

/// The beacon body, or `None` when it exceeded the 96-byte limit.
pub type BeaconBody = Option<Vec<u8>>;

pub const MAX_BEACON: usize = 96;

/// `POST /api/usage`: one page view's beacon.
pub fn handle_event(store: Option<&Store>, method: &Method, headers: &HeaderMap, host: &str, body: BeaconBody) -> Response<Body> {
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
        return with_usage_headers(response(StatusCode::NO_CONTENT, Body::empty()));
    };
    const COOKIE: &str = "office-usage";
    let mut resp = response(StatusCode::NO_CONTENT, Body::empty());
    let id = match cookie(headers, COOKIE).filter(|id| valid_browser_id(id)) {
        Some(id) => id,
        None => {
            let Some(id) = random_id() else {
                return with_usage_headers(response(StatusCode::SERVICE_UNAVAILABLE, Body::empty()));
            };
            let secure = if header_value(headers, "x-forwarded-proto") == "https" { "; Secure" } else { "" };
            let value = format!("{COOKIE}={id}; Path=/api/usage; Max-Age=2592000; HttpOnly{secure}; SameSite=Strict");
            set(&mut resp, header::SET_COOKIE, &value);
            id
        }
    };
    if store.record(jiff::Timestamp::now(), &id, &event.scope, &event.dimensions).is_err() {
        let mut failed = response(StatusCode::SERVICE_UNAVAILABLE, Body::empty());
        if let Some(c) = resp.headers().get(header::SET_COOKIE) {
            failed.headers_mut().insert(header::SET_COOKIE, c.clone());
        }
        return with_usage_headers(failed);
    }
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
            Ok(n) if [7, 30, 90, 365].contains(&n) => days = n,
            _ => return with_usage_headers(http_error("Choose 7, 30, 90 or 365 days", StatusCode::BAD_REQUEST)),
        }
    }
    let (Ok(rows), Ok(since)) = (store.daily(jiff::Timestamp::now(), days as usize), store.first_day()) else {
        return with_usage_headers(http_error("Usage temporarily unavailable", StatusCode::SERVICE_UNAVAILABLE));
    };
    let data = render_html::usage::usage_data(rows, days, since.as_deref(), &DIMENSIONS);
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
    fn events_validate_scopes_and_dimensions() {
        assert_eq!(parse_event("lauds appearance:apse screen:mobile prayer-form:priest").unwrap().dimensions.len(), 3);
        let e = parse_event("ordo prayer-form:priest appearance:nave appearance:apse future:x").unwrap();
        assert_eq!(e.dimensions, vec!["appearance:nave".to_string()]);
        assert!(parse_event("matins").is_none());
        assert!(parse_event("").is_none());
        assert!(parse_event(" lauds").is_none());
    }

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
            let response = handle_event(None, &Method::POST, &headers, host, Some(b"lauds".to_vec()));
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
            assert_ne!(crate::web_time::date_slug(eastern_day(now)), utc_day, "used UTC instead of Eastern: {value}");
        }
    }

    // Every stored family has a report breakdown.
    #[test]
    fn trend_dimensions_cover_the_vocabulary() {
        use render_html::usage::TREND_LABELS;
        assert_eq!(TREND_LABELS.len() + 1, DIMENSIONS.len());
        for (key, labels) in TREND_LABELS {
            let dimension = DIMENSIONS.iter().find(|d| d.key == key).unwrap_or_else(|| panic!("unmapped {key}"));
            assert_eq!(dimension.values.len(), labels.len(), "{key}");
            assert!(labels.iter().all(|l| !l.is_empty()), "{key}");
        }
    }

    #[test]
    fn dimension_vocabulary_is_unambiguous() {
        // Counts live under "<key>:<value>" forever: keys and values stay
        // distinct and colon-free, and no page scope looks qualified.
        let mut keys = Vec::new();
        let mut scopes = Vec::new();
        for d in &DIMENSIONS {
            assert!(!d.key.is_empty() && !d.key.contains(':') && !keys.contains(&d.key), "key {:?}", d.key);
            keys.push(d.key);
            assert_ne!(d.values[0], d.values[1], "{} has one value twice", d.key);
            for value in d.values {
                assert!(!value.is_empty() && !value.contains(':'), "value {value:?}");
                let scope = format!("{}:{value}", d.key);
                assert!(!scopes.contains(&scope) && !valid_scope(&scope), "scope {scope:?} collides");
                assert_eq!(dimension_key(&scope), Some(d.key));
                scopes.push(scope);
            }
        }
        for scope in ["site", "ordo", "reminders"].iter().chain(HOURS.iter()) {
            assert!(!scope.contains(':'));
        }
    }

    #[test]
    fn beacon_dimensions_parse_and_count() {
        for (body, scope, want) in [
            ("lauds appearance:apse screen:mobile", "lauds", vec!["appearance:apse", "screen:mobile"]),
            ("site appearance:nave screen:desktop", "site", vec!["appearance:nave", "screen:desktop"]),
            // A client from before dimensions existed.
            ("vespers", "vespers", vec![]),
            // Unreadable tokens are dropped; a bare value names no family.
            ("ordo chant:gabc appearance:apse", "ordo", vec!["appearance:apse"]),
            ("ordo apse", "ordo", vec![]),
            // One value per family wins.
            ("prime appearance:nave appearance:apse screen:mobile", "prime", vec!["appearance:nave", "screen:mobile"]),
        ] {
            let event = parse_event(body).unwrap_or_else(|| panic!("{body:?} rejected"));
            assert_eq!((event.scope.as_str(), event.dimensions), (scope, dims(&want)), "{body:?}");
        }
        for body in ["", "matins", "matins appearance:nave", "appearance:nave", " lauds"] {
            assert!(parse_event(body).is_none(), "{body:?} accepted");
        }

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
