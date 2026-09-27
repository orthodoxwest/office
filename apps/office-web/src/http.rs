//! HTTP helpers shared by the handlers. Query and cookie interpretation defines how saved links and
//! preferences are read.

use axum::body::Body;
use axum::http::{HeaderMap, HeaderValue, Response, StatusCode, header};

/// A decoded query that preserves parameter order and repeated keys.
#[derive(Clone, Debug, Default)]
pub struct Query(Vec<(String, String)>);

impl Query {
    /// Query pairs split on `&`; a pair containing `;` or a malformed escape is dropped rather than
    /// failing the request.
    pub fn parse(raw: &str) -> Query {
        let mut pairs = Vec::new();
        for pair in raw.split('&') {
            if pair.contains(';') || pair.is_empty() {
                continue;
            }
            let (key, value) = pair.split_once('=').unwrap_or((pair, ""));
            if let (Some(key), Some(value)) = (query_unescape(key), query_unescape(value)) {
                pairs.push((key, value));
            }
        }
        Query(pairs)
    }

    /// The first value for `key`, or "" if absent.
    pub fn get(&self, key: &str) -> &str {
        self.0.iter().find(|(k, _)| k == key).map(|(_, v)| v.as_str()).unwrap_or("")
    }

    pub fn has(&self, key: &str) -> bool {
        self.0.iter().any(|(k, _)| k == key)
    }
}

fn unhex(c: u8) -> Option<u8> {
    match c {
        b'0'..=b'9' => Some(c - b'0'),
        b'a'..=b'f' => Some(c - b'a' + 10),
        b'A'..=b'F' => Some(c - b'A' + 10),
        _ => None,
    }
}

/// `%XX` decodes, a malformed escape fails, and `+` is a space only in a query component.
fn unescape(s: &str, plus_is_space: bool) -> Option<Vec<u8>> {
    let b = s.as_bytes();
    let mut out = Vec::with_capacity(b.len());
    let mut i = 0;
    while i < b.len() {
        match b[i] {
            b'%' => {
                let hi = unhex(*b.get(i + 1)?)?;
                let lo = unhex(*b.get(i + 2)?)?;
                out.push(hi << 4 | lo);
                i += 3;
            }
            b'+' if plus_is_space => {
                out.push(b' ');
                i += 1;
            }
            c => {
                out.push(c);
                i += 1;
            }
        }
    }
    Some(out)
}

// Undecodable bytes in a query value appear as U+FFFD in error messages.
fn query_unescape(s: &str) -> Option<String> {
    unescape(s, true).map(|b| String::from_utf8_lossy(&b).into_owned())
}

/// The decoded request path, or `None` when an escape is malformed.
pub fn unescape_path(raw: &str) -> Option<String> {
    unescape(raw, false).map(|b| String::from_utf8_lossy(&b).into_owned())
}

fn valid_cookie_value_byte(b: u8) -> bool {
    (0x20..0x7f).contains(&b) && b != b'"' && b != b';' && b != b'\\'
}

fn is_token(s: &str) -> bool {
    !s.is_empty() && s.bytes().all(|c| c.is_ascii_alphanumeric() || b"!#$%&'*+-.^_`|~".contains(&c))
}

/// The first well-formed cookie of the requested name.
pub fn cookie(headers: &HeaderMap, name: &str) -> Option<String> {
    for line in headers.get_all(header::COOKIE) {
        let Ok(line) = line.to_str() else { continue };
        for part in line.split(';') {
            let part = part.trim_matches([' ', '\t']);
            if part.is_empty() {
                continue;
            }
            let (n, v) = part.split_once('=').unwrap_or((part, ""));
            let n = n.trim_matches([' ', '\t']);
            if !is_token(n) || n != name {
                continue;
            }
            let v = if v.len() > 1 && v.starts_with('"') && v.ends_with('"') { &v[1..v.len() - 1] } else { v };
            if v.bytes().all(valid_cookie_value_byte) {
                return Some(v.to_string());
            }
        }
    }
    None
}

/// A header's first value, "" when absent or not text.
pub fn header_value<'a>(headers: &'a HeaderMap, name: &str) -> &'a str {
    headers.get(name).and_then(|v| v.to_str().ok()).unwrap_or("")
}

pub fn set(resp: &mut Response<Body>, name: header::HeaderName, value: &str) {
    if let Ok(v) = HeaderValue::from_str(value) {
        resp.headers_mut().insert(name, v);
    }
}

/// A response with the given status and body and no headers yet.
pub fn response(code: StatusCode, body: impl Into<Body>) -> Response<Body> {
    let mut resp = Response::new(body.into());
    *resp.status_mut() = code;
    resp
}

/// An error response: plain text, `nosniff`, and a trailing newline.
pub fn http_error(msg: &str, code: StatusCode) -> Response<Body> {
    let mut resp = response(code, format!("{msg}\n"));
    set(&mut resp, header::CONTENT_TYPE, "text/plain; charset=utf-8");
    set(&mut resp, header::X_CONTENT_TYPE_OPTIONS, "nosniff");
    resp
}

/// A plain-text 404 response.
pub fn not_found() -> Response<Body> {
    http_error("404 page not found", StatusCode::NOT_FOUND)
}

/// A response for a malformed request.
pub fn bad_request() -> Response<Body> {
    let mut resp = response(StatusCode::BAD_REQUEST, "400 Bad Request");
    set(&mut resp, header::CONTENT_TYPE, "text/plain; charset=utf-8");
    set(&mut resp, header::CONNECTION, "close");
    resp
}

/// Redirect to an application URL with Axum's standard response.
pub fn redirect(url: &str, code: StatusCode) -> Response<Body> {
    use axum::response::{IntoResponse, Redirect};
    let mut response = Redirect::temporary(url).into_response();
    *response.status_mut() = code;
    response
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn queries_drop_malformed_pairs() {
        let q = Query::parse("a=1&a=2&b=x+y&c=%zz&d;=1&&e&f=%41");
        assert_eq!(q.get("a"), "1");
        assert_eq!(q.get("b"), "x y");
        assert!(!q.has("c"));
        assert!(!q.has("d;"));
        assert!(q.has("e"));
        assert_eq!(q.get("f"), "A");
    }

    #[test]
    fn paths_decode_percent_escapes() {
        assert_eq!(unescape_path("/lauds/2026%2D01%2D01").as_deref(), Some("/lauds/2026-01-01"));
        assert_eq!(unescape_path("/a+b").as_deref(), Some("/a+b"));
        assert!(unescape_path("/a%2").is_none());
    }

    #[test]
    fn cookies_select_first_valid_match() {
        let mut h = HeaderMap::new();
        h.append(header::COOKIE, HeaderValue::from_static("a=1; tz=\"America/New_York\"; tz=UTC"));
        assert_eq!(cookie(&h, "tz").as_deref(), Some("America/New_York"));
        let mut h = HeaderMap::new();
        h.append(header::COOKIE, HeaderValue::from_static("tz=a\\b; tz=Europe/Paris"));
        assert_eq!(cookie(&h, "tz").as_deref(), Some("Europe/Paris"));
        assert_eq!(cookie(&h, "none"), None);
    }
}
