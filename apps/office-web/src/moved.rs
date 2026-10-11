//! Forwarding from an old address to the canonical one (`OFFICE_CANONICAL_HOST`).
//!
//! Off the canonical host, pages forward to the same path and query there, but what other
//! clients already hold keeps working where it is: native builds post usage beacons to their
//! compiled-in endpoint without following redirects, and subscribed calendars keep polling their
//! feed URL. The old origin's service worker is replaced by one that clears its caches and
//! unregisters, so its visitors reach the forward instead of cached pages.

use axum::body::Body;
use axum::http::{HeaderMap, Method, Response, StatusCode, header};

use crate::http::{header_value, redirect, response, set};

/// The `localStorage` keys a browser navigation carries to the canonical origin; the layout's
/// head script imports them from the `#office-carry=` fragment.
pub const CARRIED_KEYS: [&str; 4] = ["office-theme", "office-text-size", "office-prayer-form", "office-martyrology"];

/// A canonical host as configured: a bare host name with an optional port.
pub fn parse_host(raw: &str) -> Option<String> {
    let host = raw.trim().to_ascii_lowercase();
    let valid = !host.is_empty() && host.bytes().all(|b| b.is_ascii_alphanumeric() || b"-.:".contains(&b));
    valid.then_some(host)
}

/// Whether a request for `host` is off the canonical host. A request with no Host is left alone.
pub fn is_elsewhere(host: &str, canonical: &str) -> bool {
    !host.is_empty() && !host.eq_ignore_ascii_case(canonical)
}

/// The same path and query on the canonical host. A browser navigation gets a page that carries
/// the reader's settings across in the fragment; anything else is redirected permanently, with
/// 308 keeping a non-GET method and body.
pub fn forward(canonical: &str, method: &Method, headers: &HeaderMap, path_and_query: &str) -> Response<Body> {
    let target = format!("https://{canonical}{path_and_query}");
    if method != Method::GET && method != Method::HEAD {
        return redirect(&target, StatusCode::PERMANENT_REDIRECT);
    }
    if header_value(headers, "sec-fetch-mode") != "navigate" {
        return redirect(&target, StatusCode::MOVED_PERMANENTLY);
    }
    let keys = CARRIED_KEYS.map(|k| format!("\"{k}\"")).join(", ");
    let link = escape(&target);
    let body = format!(
        r##"<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<meta name="robots" content="noindex">
<link rel="canonical" href="{link}">
<title>Daily Office has moved</title>
<noscript><meta http-equiv="refresh" content="0; url={link}"></noscript>
<script>
  (function () {{
    var carried = new URLSearchParams();
    try {{
      [{keys}].forEach(function (key) {{
        var value = localStorage.getItem(key);
        if (value !== null) carried.set(key, value);
      }});
    }} catch (e) {{}}
    var fragment = carried.toString() ? "#office-carry=" + encodeURIComponent(carried.toString()) : location.hash;
    location.replace("https://{canonical}" + location.pathname + location.search + fragment);
  }})();
</script>
</head>
<body>
<p>The Daily Office has moved to <a href="{link}">{canonical}</a>.</p>
</body>
</html>
"##
    );
    let mut resp = response(StatusCode::OK, body);
    set(&mut resp, header::CONTENT_TYPE, "text/html; charset=utf-8");
    set(&mut resp, header::CACHE_CONTROL, "no-store");
    resp
}

/// The service worker served off the canonical host: it takes over from the old worker at once,
/// deletes every cache, unregisters, and reloads its pages so they reach the forward.
pub fn farewell_worker() -> Response<Body> {
    let body = r#"/* This address has moved: retire the old worker and its caches. */
self.addEventListener("install", function () { self.skipWaiting(); });
self.addEventListener("activate", function (event) {
  event.waitUntil((async function () {
    var keys = await caches.keys();
    await Promise.all(keys.map(function (key) { return caches.delete(key); }));
    await self.registration.unregister();
    var pages = await self.clients.matchAll({ type: "window" });
    pages.forEach(function (page) { page.navigate(page.url); });
  })());
});
"#;
    let mut resp = response(StatusCode::OK, body);
    set(&mut resp, header::CONTENT_TYPE, "text/javascript; charset=utf-8");
    set(&mut resp, header::CACHE_CONTROL, "no-cache");
    resp
}

fn escape(s: &str) -> String {
    s.replace('&', "&amp;").replace('"', "&quot;").replace('<', "&lt;").replace('>', "&gt;")
}

#[cfg(test)]
mod tests {
    use super::*;

    fn body(resp: Response<Body>) -> String {
        let bytes = tokio::runtime::Builder::new_current_thread()
            .build()
            .unwrap()
            .block_on(axum::body::to_bytes(resp.into_body(), usize::MAX))
            .unwrap();
        String::from_utf8(bytes.to_vec()).unwrap()
    }

    #[test]
    fn canonical_hosts_are_bare_names() {
        assert_eq!(parse_host(" Example.ORG "), Some("example.org".into()));
        assert_eq!(parse_host("localhost:8080"), Some("localhost:8080".into()));
        for raw in ["", "https://example.org", "a.org/x", "a.org\"<"] {
            assert_eq!(parse_host(raw), None, "{raw}");
        }
        assert!(is_elsewhere("office.fly.dev", "example.org"));
        assert!(!is_elsewhere("Example.ORG", "example.org"));
        assert!(!is_elsewhere("", "example.org"));
    }

    #[test]
    fn navigations_carry_settings_and_others_redirect() {
        let mut nav = HeaderMap::new();
        nav.insert("sec-fetch-mode", "navigate".parse().unwrap());
        let page = forward("example.org", &Method::GET, &nav, "/lauds/2026-10-06?form=priest&x=\"<");
        assert_eq!(page.status(), StatusCode::OK);
        let html = body(page);
        assert!(html.contains(r#"href="https://example.org/lauds/2026-10-06?form=priest&amp;x=&quot;&lt;""#), "{html}");
        for key in CARRIED_KEYS {
            assert!(html.contains(&format!("\"{key}\"")), "{key}");
        }

        let get = forward("example.org", &Method::GET, &HeaderMap::new(), "/calendar/2026?form=deacon");
        assert_eq!(get.status(), StatusCode::MOVED_PERMANENTLY);
        assert_eq!(get.headers()[header::LOCATION], "https://example.org/calendar/2026?form=deacon");

        let post = forward("example.org", &Method::POST, &nav, "/reminders");
        assert_eq!(post.status(), StatusCode::PERMANENT_REDIRECT);
    }
}
