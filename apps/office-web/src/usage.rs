//! Approximate daily browser counts, not request logs, and the two
//! endpoints that write and read them. Ported from Go's `usage/store.go`,
//! `usage/bots.go`, and `web/usage.go`; the SQLite schema is shared, so
//! either server can open the other's database.

use std::collections::BTreeMap;
use std::io::Read as _;
use std::sync::Mutex;
use std::time::Duration;

use axum::body::Body;
use axum::http::{HeaderMap, Method, Response, StatusCode, header};
use calendar::Date;
use render_html::usage::{Dimension, HOURS, UsageDay};
use rusqlite::{Connection, params};
use sha2::{Digest, Sha256};

use crate::gonet::{Query, cookie, header_value, http_error, not_found, response, set};

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

/// Go's `strings.ToLower`: one rune for one rune.
fn go_lower(s: &str) -> String {
    s.chars()
        .map(|c| {
            let mut lower = c.to_lowercase();
            match (lower.next(), lower.next()) {
                (Some(l), None) => l,
                // Only U+0130 lowers to two runes; Go's simple mapping is 'i'.
                _ => 'i',
            }
        })
        .collect()
}

/// Whether a user agent is a crawler or scripted client. Beyond the tokens
/// it catches the "<name>bot/<version>" form, but not a bare "bot", which
/// phone models carry ("Cubot"). An empty agent is a scripted client.
pub fn is_bot(agent: &str) -> bool {
    let agent = go_lower(agent.trim());
    agent.is_empty() || agent.contains("bot/") || BOT_TOKENS.iter().any(|t| agent.contains(t))
}

fn eastern() -> jiff::tz::TimeZone {
    crate::gotime::zone("America/New_York").unwrap_or(jiff::tz::TimeZone::UTC)
}

/// The reporting day of now: a civil date in America/New_York.
fn eastern_today() -> Date {
    crate::gotime::now_in(&eastern()).0
}

/// The SQLite store, one connection as in Go.
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
    pub fn record(&self, browser: &str, scope: &str, dimensions: &[String]) -> Result<(), String> {
        if !valid_scope(scope) || dimensions.iter().any(|d| dimension_key(d).is_none()) {
            return Err("invalid usage event".into());
        }
        let today = eastern_today();
        let day = crate::gotime::date_slug(today);
        let hash = Sha256::digest(format!("{day}\x00{browser}").as_bytes());
        let mut conn = self.conn.lock().map_err(|e| e.to_string())?;
        let tx = conn.transaction().map_err(|e| e.to_string())?;
        tx.execute("DELETE FROM seen WHERE day < ?", [crate::gotime::date_slug(today.add_days(-2))]).map_err(|e| e.to_string())?;
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

    /// A zero-filled window of `days`, newest first.
    pub fn daily(&self, days: usize) -> Result<Vec<UsageDay>, String> {
        if !(1..=366).contains(&days) {
            return Err("invalid day window".into());
        }
        let today = eastern_today();
        let conn = self.conn.lock().map_err(|e| e.to_string())?;
        conn.execute("DELETE FROM seen WHERE day < ?", [crate::gotime::date_slug(today.add_days(-2))]).map_err(|e| e.to_string())?;
        let mut result: Vec<UsageDay> =
            (0..days).map(|i| UsageDay { day: crate::gotime::date_slug(today.add_days(-(i as i32))), ..UsageDay::default() }).collect();
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

/// Go's `url.Parse(origin)` far enough to compare its scheme and host;
/// `None` where Go's parse fails.
fn origin_parts(origin: &str) -> Option<(String, String)> {
    let origin = origin.split('#').next().unwrap_or("");
    let mut scheme = "";
    let mut rest = origin;
    for (i, c) in origin.bytes().enumerate() {
        match c {
            b'a'..=b'z' | b'A'..=b'Z' => {}
            b'0'..=b'9' | b'+' | b'-' | b'.' if i > 0 => {}
            b':' if i == 0 => return None,
            b':' => {
                scheme = &origin[..i];
                rest = &origin[i + 1..];
                break;
            }
            _ => break,
        }
    }
    let scheme = scheme.to_ascii_lowercase();
    let rest = rest.split('?').next().unwrap_or("");
    let Some(authority) = rest.strip_prefix("//").filter(|_| !scheme.is_empty()) else {
        return Some((scheme, String::new()));
    };
    let authority = authority.split('/').next().unwrap_or("");
    let host = authority.rsplit_once('@').map(|(_, h)| h).unwrap_or(authority);
    if !host.starts_with('[')
        && let Some((_, port)) = host.rsplit_once(':')
        && !port.bytes().all(|c| c.is_ascii_digit())
    {
        return None;
    }
    let host_ok = host.bytes().all(|c| c >= 0x80 || c.is_ascii_alphanumeric() || b"-_.~!$&'()*+,;=:[]<>\"".contains(&c));
    host_ok.then(|| (scheme, host.to_string()))
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
    if !origin.is_empty() {
        match origin_parts(origin) {
            Some((scheme, h)) if h == host && (scheme == "https" || scheme == "http") => {}
            _ => return with_usage_headers(http_error("Forbidden", StatusCode::FORBIDDEN)),
        }
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
    if store.record(&id, &event.scope, &event.dimensions).is_err() {
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
        match compat::atoi(raw) {
            Ok(n) if [7, 30, 90, 365].contains(&n) => days = n,
            _ => return with_usage_headers(http_error("Choose 7, 30, 90 or 365 days", StatusCode::BAD_REQUEST)),
        }
    }
    let Ok(rows) = store.daily(days as usize) else {
        return with_usage_headers(http_error("Usage temporarily unavailable", StatusCode::SERVICE_UNAVAILABLE));
    };
    let data = render_html::usage::usage_data(rows, days, &DIMENSIONS);
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
    fn events_parse_as_go_parses_them() {
        assert_eq!(parse_event("lauds appearance:apse screen:mobile prayer-form:priest").unwrap().dimensions.len(), 3);
        let e = parse_event("ordo prayer-form:priest appearance:nave appearance:apse future:x").unwrap();
        assert_eq!(e.dimensions, vec!["appearance:nave".to_string()]);
        assert!(parse_event("matins").is_none());
        assert!(parse_event("").is_none());
        assert!(parse_event(" lauds").is_none());
    }

    #[test]
    fn bots() {
        assert!(is_bot(""));
        assert!(is_bot("Mozilla/5.0 (compatible; Googlebot/2.1)"));
        assert!(is_bot("somethingbot/1.0"));
        assert!(!is_bot("Mozilla/5.0 (Linux; Android 10; Cubot Note 20)"));
        assert!(is_bot("curl/8.0"));
    }

    #[test]
    fn origins() {
        assert_eq!(origin_parts("https://office.example:8443"), Some(("https".into(), "office.example:8443".into())));
        assert_eq!(origin_parts("HTTP://a.b"), Some(("http".into(), "a.b".into())));
        assert_eq!(origin_parts("null"), Some((String::new(), String::new())));
        assert_eq!(origin_parts("https://a.b:x"), None);
    }

    #[test]
    fn store_round_trip() {
        let dir = std::env::temp_dir().join(format!("office-usage-{}", std::process::id()));
        std::fs::create_dir_all(&dir).unwrap();
        let path = dir.join("usage.db");
        let store = Store::open(path.to_str().unwrap()).unwrap();
        store.record("a", "lauds", &["screen:mobile".into()]).unwrap();
        store.record("a", "lauds", &[]).unwrap();
        store.record("b", "ordo", &[]).unwrap();
        let rows = store.daily(7).unwrap();
        assert_eq!(rows.len(), 7);
        assert_eq!(rows[0].users, 2);
        assert_eq!(rows[0].hours[0], 1);
        assert_eq!(rows[0].ordo, 1);
        assert_eq!(rows[0].dimensions.get("screen:mobile"), Some(&1));
        std::fs::remove_dir_all(&dir).unwrap();
    }
}
