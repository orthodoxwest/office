//! The parts of Go's `net/http` and `net/url` behavior the Go server exposes:
//! query parsing, request-path unescaping and canonicalization, cookies, and
//! the plain-text error and redirect responses. Reproducing them keeps the
//! two servers' answers identical, malformed requests included.

use axum::body::Body;
use axum::http::{HeaderMap, HeaderValue, Method, Response, StatusCode, header};

/// A request's query as `r.URL.Query()` reads it.
#[derive(Clone, Debug, Default)]
pub struct Query(Vec<(String, String)>);

impl Query {
    /// Go's `url.ParseQuery`: pairs split on `&`; a pair containing `;` or a
    /// malformed escape is dropped rather than failing the request.
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

    /// The first value for `key`, or "" (`url.Values.Get`).
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

/// Go's `unescape`: `%XX` decodes, a malformed escape fails, and `+` is a
/// space only in a query component.
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

// PORT(inherited): Go keeps undecodable bytes in a query value; they reach
// the page only inside an error message, where Rust shows U+FFFD.
fn query_unescape(s: &str) -> Option<String> {
    unescape(s, true).map(|b| String::from_utf8_lossy(&b).into_owned())
}

/// The decoded request path (`r.URL.Path`), or `None` when an escape is
/// malformed, which Go's server answers before any handler runs.
pub fn unescape_path(raw: &str) -> Option<String> {
    unescape(raw, false).map(|b| String::from_utf8_lossy(&b).into_owned())
}

/// Go's `path.Clean`.
pub fn path_clean(p: &str) -> String {
    if p.is_empty() {
        return ".".into();
    }
    let rooted = p.starts_with('/');
    let mut parts: Vec<&str> = Vec::new();
    for seg in p.split('/') {
        match seg {
            "" | "." => {}
            ".." => {
                if parts.last().is_some_and(|l| *l != "..") {
                    parts.pop();
                } else if !rooted {
                    parts.push("..");
                }
            }
            s => parts.push(s),
        }
    }
    let joined = parts.join("/");
    match (rooted, joined.is_empty()) {
        (true, _) => format!("/{joined}"),
        (false, true) => ".".into(),
        (false, false) => joined,
    }
}

/// Go's `ServeMux` `cleanPath`: `path.Clean`, keeping a trailing slash.
pub fn clean_path(p: &str) -> String {
    if p.is_empty() {
        return "/".into();
    }
    let p = if p.starts_with('/') { p.to_string() } else { format!("/{p}") };
    let mut np = path_clean(&p);
    if p.ends_with('/') && np != "/" {
        np.push('/');
    }
    np
}

/// Go's `shouldEscape` for a URL path.
fn path_char_ok(c: u8) -> bool {
    c.is_ascii_alphanumeric() || matches!(c, b'-' | b'_' | b'.' | b'~' | b'$' | b'&' | b'+' | b',' | b'/' | b':' | b';' | b'=' | b'@')
}

/// Go's `url.URL{Path: p}.EscapedPath()`.
pub fn escape_path(p: &str) -> String {
    let mut out = String::with_capacity(p.len());
    for &c in p.as_bytes() {
        if path_char_ok(c) {
            out.push(c as char);
        } else {
            out.push_str(&format!("%{c:02X}"));
        }
    }
    out
}

/// `url.URL{Path: path, RawQuery: query}.String()` for a rooted path.
pub fn url_string(path: &str, raw_query: &str) -> String {
    let mut s = escape_path(path);
    if !raw_query.is_empty() {
        s.push('?');
        s.push_str(raw_query);
    }
    s
}

fn valid_cookie_value_byte(b: u8) -> bool {
    (0x20..0x7f).contains(&b) && b != b'"' && b != b';' && b != b'\\'
}

fn is_token(s: &str) -> bool {
    !s.is_empty() && s.bytes().all(|c| c.is_ascii_alphanumeric() || b"!#$%&'*+-.^_`|~".contains(&c))
}

/// Go's `r.Cookie(name)`: the first well-formed cookie of that name.
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

/// A header's first value (`Header.Get`), "" when absent or not text.
pub fn header_value<'a>(headers: &'a HeaderMap, name: &str) -> &'a str {
    headers.get(name).and_then(|v| v.to_str().ok()).unwrap_or("")
}

/// Go's `htmlReplacer` in `net/http`.
fn html_escape(s: &str) -> String {
    let mut out = String::with_capacity(s.len());
    for c in s.chars() {
        match c {
            '&' => out.push_str("&amp;"),
            '<' => out.push_str("&lt;"),
            '>' => out.push_str("&gt;"),
            '"' => out.push_str("&#34;"),
            '\'' => out.push_str("&#39;"),
            c => out.push(c),
        }
    }
    out
}

fn status_text(code: StatusCode) -> &'static str {
    code.canonical_reason().unwrap_or("")
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

/// Go's `http.Error`: plain text, `nosniff`, and a trailing newline.
pub fn http_error(msg: &str, code: StatusCode) -> Response<Body> {
    let mut resp = response(code, format!("{msg}\n"));
    set(&mut resp, header::CONTENT_TYPE, "text/plain; charset=utf-8");
    set(&mut resp, header::X_CONTENT_TYPE_OPTIONS, "nosniff");
    resp
}

/// Go's `http.NotFound`.
pub fn not_found() -> Response<Body> {
    http_error("404 page not found", StatusCode::NOT_FOUND)
}

/// Go's answer to a request line it cannot parse.
pub fn bad_request() -> Response<Body> {
    let mut resp = response(StatusCode::BAD_REQUEST, "400 Bad Request");
    set(&mut resp, header::CONTENT_TYPE, "text/plain; charset=utf-8");
    set(&mut resp, header::CONNECTION, "close");
    resp
}

/// Go's `http.Redirect` for a URL without scheme or host.
pub fn redirect(method: &Method, url: &str, code: StatusCode) -> Response<Body> {
    let (path, query) = match url.find('?') {
        Some(i) => (&url[..i], &url[i..]),
        None => (url, ""),
    };
    let mut cleaned = path_clean(path);
    if path.ends_with('/') && !cleaned.ends_with('/') {
        cleaned.push('/');
    }
    let url = format!("{cleaned}{query}");
    let body =
        if method == Method::GET { format!("<a href=\"{}\">{}</a>.\n\n", html_escape(&url), status_text(code)) } else { String::new() };
    let mut resp = response(code, body);
    // Go hex-escapes non-ASCII bytes in the Location header.
    let location: String = url.bytes().map(|b| if b.is_ascii() { (b as char).to_string() } else { format!("%{b:02X}") }).collect();
    set(&mut resp, header::LOCATION, &location);
    if method == Method::GET || method == Method::HEAD {
        set(&mut resp, header::CONTENT_TYPE, "text/html; charset=utf-8");
    }
    resp
}

/// Go's `localRedirect` in the file server: a bare 301.
pub fn local_redirect(new_path: &str, raw_query: &str) -> Response<Body> {
    let location = if raw_query.is_empty() { new_path.to_string() } else { format!("{new_path}?{raw_query}") };
    let mut resp = response(StatusCode::MOVED_PERMANENTLY, Body::empty());
    set(&mut resp, header::LOCATION, &location);
    resp
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn queries_follow_go() {
        let q = Query::parse("a=1&a=2&b=x+y&c=%zz&d;=1&&e&f=%41");
        assert_eq!(q.get("a"), "1");
        assert_eq!(q.get("b"), "x y");
        assert!(!q.has("c"));
        assert!(!q.has("d;"));
        assert!(q.has("e"));
        assert_eq!(q.get("f"), "A");
    }

    #[test]
    fn paths_follow_go() {
        assert_eq!(clean_path("/lauds//2026-01-01"), "/lauds/2026-01-01");
        assert_eq!(clean_path("/calendar/2026/"), "/calendar/2026/");
        assert_eq!(clean_path("/a/../b/./c/"), "/b/c/");
        assert_eq!(clean_path("/.."), "/");
        assert_eq!(path_clean("a/../../b"), "../b");
        assert_eq!(unescape_path("/lauds/2026%2D01%2D01").as_deref(), Some("/lauds/2026-01-01"));
        assert_eq!(unescape_path("/a+b").as_deref(), Some("/a+b"));
        assert!(unescape_path("/a%2").is_none());
        assert_eq!(url_string("/a b/%2D", "x=1"), "/a%20b/%252D?x=1");
    }

    #[test]
    fn cookies_follow_go() {
        let mut h = HeaderMap::new();
        h.append(header::COOKIE, HeaderValue::from_static("a=1; tz=\"America/New_York\"; tz=UTC"));
        assert_eq!(cookie(&h, "tz").as_deref(), Some("America/New_York"));
        let mut h = HeaderMap::new();
        h.append(header::COOKIE, HeaderValue::from_static("tz=a\\b; tz=Europe/Paris"));
        assert_eq!(cookie(&h, "tz").as_deref(), Some("Europe/Paris"));
        assert_eq!(cookie(&h, "none"), None);
    }
}
