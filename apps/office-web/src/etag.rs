//! Conditional requests for the pages: a page is a function of the build, its address, the
//! reader's cookies and the hour it is where the reader is (home's invitation turns with the
//! hour, the ordo and the hours mark today), and of nothing else. Its entity tag names those,
//! so a reader or the service worker asking again within the hour is told "not modified"
//! without the page being composed or sent.

use std::hash::{DefaultHasher, Hash, Hasher};

use axum::body::Body;
use axum::http::{HeaderMap, Response, StatusCode, header};

use crate::http::{response, set};

/// The page's entity tag. `now` is the reader's date and hour (`web_time::now_in`) and
/// `server_year` the server's own, which sets the years the date fields offer.
pub fn page_tag(version: &str, site: &str, target: &str, headers: &HeaderMap, now: (calendar::Date, i8), server_year: i32) -> String {
    let mut h = DefaultHasher::new();
    (site, target, server_year).hash(&mut h);
    for cookies in headers.get_all(header::COOKIE) {
        cookies.as_bytes().hash(&mut h);
    }
    (now.0.year(), now.0.month(), now.0.day(), now.1).hash(&mut h);
    format!("W/\"{version}-{:016x}\"", h.finish())
}

/// Whether the request's `If-None-Match` names `tag`, compared weakly.
pub fn matches(headers: &HeaderMap, tag: &str) -> bool {
    let opaque = |t: &str| t.trim().trim_start_matches("W/").to_string();
    headers
        .get_all(header::IF_NONE_MATCH)
        .iter()
        .filter_map(|v| v.to_str().ok())
        .flat_map(|v| v.split(','))
        .any(|t| opaque(t) == opaque(tag))
}

/// "Not modified", carrying the tag and the pages' revalidation rule and `Vary`.
pub fn not_modified(tag: &str) -> Response<Body> {
    let mut resp = response(StatusCode::NOT_MODIFIED, Body::empty());
    set(&mut resp, header::ETAG, tag);
    set(&mut resp, header::CACHE_CONTROL, "no-cache");
    set(&mut resp, header::VARY, "Cookie");
    resp
}
