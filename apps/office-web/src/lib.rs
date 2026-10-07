//! The Office web server: Axum routes over a shared calendar, corpus,
//! templates, reminder feed, and optional usage store.

mod cache;
// build.rs trims and stamps the stylesheet with this module; the library
// compiles it only to test it.
#[cfg(test)]
mod css;
mod handlers;
mod http;
mod ics;
mod moved;
pub mod pwa;
pub mod usage;
pub mod web_time;

use std::path::Path;
use std::sync::Arc;

use axum::Router;
use axum::body::Body;
use axum::extract::Request;
use axum::http::{HeaderMap, Method, Response, StatusCode, Uri, header};
use axum::routing::any;
use calendar::CalendarData;
use office::Engine;
use render_html::Pages;
use tools::fs::FsData;

use crate::cache::YearCache;
use crate::handlers::Req;
use crate::http::{Query, bad_request, unescape_path};
use crate::usage::{BeaconBody, MAX_BEACON, Store};

/// Everything a request reads, loaded once and shared.
pub struct Server {
    engine: Engine,
    cache: YearCache,
    pages: Pages,
    version: String,
    usage: Option<Store>,
    /// The host every other host forwards to; see [`moved`].
    canonical: Option<String>,
}

/// Application endpoints; Axum owns path matching.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
enum Route {
    UsageEvent,
    UsageDashboard,
    Static,
    ServiceWorker,
    Ics,
    Reminders,
    Privacy,
    Calendar,
    Root,
}

impl Server {
    /// Loads the engine and templates from `data_dir`.
    pub fn new(data_dir: &Path) -> Result<Server, String> {
        let src = FsData::new(data_dir);
        let engine = Engine::load(&src).map_err(|e| format!("creating office engine: {e}"))?;
        let calendar = CalendarData::load(&src).map_err(|e| format!("loading calendar data: {e}"))?;
        let version = pwa::compute_version(data_dir);
        let pages = Pages::new(pwa::asset_url).map_err(|e| format!("parsing templates: {e}"))?;
        Ok(Server { engine, cache: YearCache::new(calendar), pages, version, usage: None, canonical: None })
    }

    /// Opens the usage database named by `OFFICE_USAGE_DB`, if any. A
    /// database that will not open disables metrics, never the site.
    pub fn open_usage_from_env(&mut self) {
        let Ok(path) = std::env::var("OFFICE_USAGE_DB") else { return };
        if path.is_empty() {
            return;
        }
        match Store::open(&path) {
            Ok(store) => self.usage = Some(store),
            Err(e) => eprintln!("warn: usage metrics disabled: {e}"),
        }
    }

    /// Reads `OFFICE_CANONICAL_HOST`: when set, requests for any other host forward there.
    pub fn canonical_host_from_env(&mut self) {
        let raw = std::env::var("OFFICE_CANONICAL_HOST").unwrap_or_default();
        if raw.trim().is_empty() {
            return;
        }
        match moved::parse_host(&raw) {
            Some(host) => self.canonical = Some(host),
            None => eprintln!("warn: OFFICE_CANONICAL_HOST is not a host name; not forwarding"),
        }
    }

    fn handle(&self, route: Route, method: &Method, uri: &Uri, headers: &HeaderMap, body: BeaconBody) -> Response<Body> {
        let raw_query = uri.query().unwrap_or("");
        let Some(path) = unescape_path(uri.path()) else { return bad_request() };
        let query = Query::parse(raw_query);
        let host = headers.get(header::HOST).and_then(|h| h.to_str().ok()).or_else(|| uri.authority().map(|a| a.as_str())).unwrap_or("");
        let req = Req { method, headers, path: &path, query: &query };
        if let Some(canonical) = self.canonical.as_deref()
            && moved::is_elsewhere(host, canonical)
        {
            match route {
                // Clients that already hold these URLs keep using them where they are.
                Route::UsageEvent | Route::Static => {}
                Route::Ics => return self.ics(&query, &format!("https://{canonical}")),
                Route::ServiceWorker => return moved::farewell_worker(),
                Route::UsageDashboard | Route::Reminders | Route::Privacy | Route::Calendar | Route::Root => {
                    let target = uri.path_and_query().map(|p| p.as_str()).unwrap_or("/");
                    return moved::forward(canonical, method, headers, target);
                }
            }
        }
        match route {
            Route::UsageEvent => usage::handle_event(self.usage.as_ref(), method, headers, host, self.canonical.as_deref(), body),
            Route::UsageDashboard => usage::handle_dashboard(self.usage.as_ref(), &self.pages, method, &query),
            Route::Static => pwa::serve_static(&path, query.get("v")),
            Route::ServiceWorker => pwa::service_worker(&self.version),
            Route::Ics => self.ics(&query, &ics::base_url(headers, host)),
            Route::Reminders => self.reminders(&req),
            Route::Privacy => self.privacy(&req),
            Route::Calendar => self.calendar(&req),
            Route::Root => self.root(&req),
        }
    }

    /// The build stamp on the service worker and its page cache.
    pub fn version(&self) -> &str {
        &self.version
    }
}

/// Construct the same router for production and request-level tests.
impl Server {
    pub fn router(self: Arc<Self>) -> Router {
        let endpoint = |route| {
            let server = Arc::clone(&self);
            any(move |req: Request| entry(Arc::clone(&server), route, req))
        };
        Router::new()
            .route("/api/usage", endpoint(Route::UsageEvent))
            .route("/admin/usage", endpoint(Route::UsageDashboard))
            .route("/static", endpoint(Route::Static))
            .route("/static/", endpoint(Route::Static))
            .route("/static/{*path}", endpoint(Route::Static))
            .route("/sw.js", endpoint(Route::ServiceWorker))
            .route("/office.ics", endpoint(Route::Ics))
            .route("/reminders", endpoint(Route::Reminders))
            .route("/privacy", endpoint(Route::Privacy))
            .route("/calendar", endpoint(Route::Calendar))
            .route("/calendar/", endpoint(Route::Calendar))
            .route("/calendar/{*path}", endpoint(Route::Calendar))
            .fallback(move |req: Request| entry(Arc::clone(&self), Route::Root, req))
    }
}

async fn entry(server: Arc<Server>, route: Route, req: Request) -> Response<Body> {
    let (parts, body) = req.into_parts();
    // Only the beacon reads a body, and never more than it allows.
    let beacon = if route == Route::UsageEvent && parts.method == Method::POST {
        axum::body::to_bytes(body, MAX_BEACON).await.ok().map(|b| b.to_vec())
    } else {
        Some(Vec::new())
    };
    let task = tokio::task::spawn_blocking(move || server.handle(route, &parts.method, &parts.uri, &parts.headers, beacon));
    match task.await {
        Ok(resp) => resp,
        Err(e) => {
            eprintln!("request failed: {e}");
            http::http_error("Internal server error", StatusCode::INTERNAL_SERVER_ERROR)
        }
    }
}

/// An address of the form `:8080` listens on every interface.
async fn listen(addr: &str) -> Result<tokio::net::TcpListener, String> {
    if let Some(port) = addr.strip_prefix(':') {
        if let Ok(l) = tokio::net::TcpListener::bind(format!("[::]:{port}")).await {
            return Ok(l);
        }
        return tokio::net::TcpListener::bind(format!("0.0.0.0:{port}")).await.map_err(|e| format!("listen tcp {addr}: {e}"));
    }
    tokio::net::TcpListener::bind(addr).await.map_err(|e| format!("listen tcp {addr}: {e}"))
}

/// Serves until the process ends. The current year's calendar page, then
/// the next year's, are composed in the background so the first visit to
/// either is fast: composing a year takes about half a second, and the next
/// year is one tap from the ordo and enters the offline precache each
/// December.
pub fn run(server: Server, addr: &str) -> Result<(), String> {
    let runtime = tokio::runtime::Builder::new_multi_thread().enable_all().build().map_err(|e| e.to_string())?;
    runtime.block_on(async {
        let server = Arc::new(server);
        let warm = Arc::clone(&server);
        tokio::task::spawn_blocking(move || {
            let year = Server::local_year();
            for year in [year, year + 1] {
                if let Err(e) = warm.cache.months(year, &warm.engine) {
                    eprintln!("warn: pre-warming cache for {year}: {e}");
                }
            }
        });
        let listener = listen(addr).await?;
        let app = server.router();
        axum::serve(listener, app).await.map_err(|e| format!("server error: {e}"))
    })
}

/// One server over the live data, shared by the tests.
#[cfg(test)]
pub(crate) fn test_server() -> Arc<Server> {
    static SERVER: std::sync::OnceLock<Arc<Server>> = std::sync::OnceLock::new();
    Arc::clone(SERVER.get_or_init(|| Arc::new(Server::new(Path::new("../../data")).unwrap())))
}

#[cfg(test)]
mod routing_tests {
    use super::*;
    use tower::ServiceExt;

    async fn request(method: Method, path: &str, body: impl Into<Body>) -> Response<Body> {
        let request = Request::builder()
            .method(method)
            .uri(path)
            .header("x-office-usage", "1")
            .header("user-agent", "Mozilla/5.0")
            .body(body.into())
            .unwrap();
        test_server().router().oneshot(request).await.unwrap()
    }

    #[tokio::test]
    async fn routes_serve_pages_assets_and_head() {
        for path in ["/lauds/2026-03-11", "/static/style.css", "/reminders", "/privacy", "/sw.js"] {
            let get = request(Method::GET, path, Body::empty()).await;
            let head = request(Method::HEAD, path, Body::empty()).await;
            assert_eq!(get.status(), StatusCode::OK, "{path}");
            assert_eq!(head.status(), get.status(), "{path}");
            assert_eq!(head.headers().get(header::CONTENT_TYPE), get.headers().get(header::CONTENT_TYPE));
            assert!(axum::body::to_bytes(head.into_body(), usize::MAX).await.unwrap().is_empty());
        }
        let redirect = request(Method::GET, "/calendar?form=priest", Body::empty()).await;
        assert_eq!(redirect.status(), StatusCode::FOUND);
        let location = redirect.headers()[header::LOCATION].to_str().unwrap();
        assert!(location.starts_with("/calendar/") && location.contains("?form=priest#d-"), "{location}");
        // Today's month, anchored at today: /calendar/YYYY/MM?form=priest#d-YYYY-MM-DD.
        let (month, day) = (&location["/calendar/".len().."/calendar/YYYY/MM".len()], &location[location.len() - 10..]);
        assert_eq!(month.replace('/', "-"), day[..7]);
        assert!(axum::body::to_bytes(redirect.into_body(), usize::MAX).await.unwrap().is_empty());
    }

    #[tokio::test]
    async fn other_hosts_forward_to_the_canonical_host() {
        let mut server = Server::new(Path::new("../../data")).unwrap();
        server.canonical = moved::parse_host("example.org");
        let router = Arc::new(server).router();
        let send = |method: Method, host: &str, path: &str, navigate: bool| {
            let mut request = Request::builder().method(method).uri(path).header(header::HOST, host);
            if navigate {
                request = request.header("sec-fetch-mode", "navigate");
            }
            router.clone().oneshot(request.body(Body::empty()).unwrap())
        };
        let body = |resp: Response<Body>| async {
            String::from_utf8(axum::body::to_bytes(resp.into_body(), usize::MAX).await.unwrap().to_vec()).unwrap()
        };

        let home = send(Method::GET, "example.org", "/lauds/2026-03-11", true).await.unwrap();
        assert_eq!(home.status(), StatusCode::OK);
        assert!(body(home).await.contains("office-carry"), "the canonical host imports carried settings");

        let old = send(Method::GET, "office.fly.dev", "/lauds/2026-03-11?form=priest", false).await.unwrap();
        assert_eq!(old.status(), StatusCode::MOVED_PERMANENTLY);
        assert_eq!(old.headers()[header::LOCATION], "https://example.org/lauds/2026-03-11?form=priest");
        let nav = send(Method::GET, "office.fly.dev", "/", true).await.unwrap();
        assert_eq!(nav.status(), StatusCode::OK);
        assert!(body(nav).await.contains("location.replace(\"https://example.org\""));

        // Native builds post here without following redirects; feeds and assets stay put.
        let beacon = send(Method::POST, "office.fly.dev", "/api/usage", false).await.unwrap();
        assert!(!beacon.status().is_redirection(), "{}", beacon.status());
        let asset = send(Method::GET, "office.fly.dev", &pwa::asset_url("style.css"), false).await.unwrap();
        assert_eq!(asset.status(), StatusCode::OK);
        let feed = send(Method::GET, "office.fly.dev", "/office.ics?lauds=06:00&tz=America/New_York", false).await.unwrap();
        assert_eq!(feed.status(), StatusCode::OK);
        let feed = body(feed).await;
        assert!(feed.contains("https://example.org/") && !feed.contains("office.fly.dev"), "feed links point at the canonical host");

        let worker = body(send(Method::GET, "office.fly.dev", "/sw.js", false).await.unwrap()).await;
        assert!(worker.contains("unregister") && !worker.contains("PRECACHE_DAYS"));
        let worker = body(send(Method::GET, "example.org", "/sw.js", false).await.unwrap()).await;
        assert!(worker.contains("PRECACHE_DAYS"));
    }

    #[tokio::test]
    async fn framework_rejects_noncanonical_paths() {
        for path in ["/lauds//2026-03-11", "/./lauds/2026-03-11", "/lauds/../prime/2026-03-11", "/%73tatic/app.js", "/%73w.js"] {
            assert_eq!(request(Method::GET, path, Body::empty()).await.status(), StatusCode::NOT_FOUND, "{path}");
        }
        for path in
            ["/static", "/static/", "/static/fonts", "/static/fonts/", "/static/style.css/", "/static/../data/review/provenance.csv"]
        {
            assert_eq!(request(Method::GET, path, Body::empty()).await.status(), StatusCode::NOT_FOUND, "{path}");
        }
        let asset = request(Method::GET, &pwa::asset_url("style.css"), Body::empty()).await;
        assert_eq!(asset.headers()[header::CACHE_CONTROL], "public, max-age=31536000, immutable");
        assert!(!asset.headers().contains_key(header::ACCEPT_RANGES));
        for path in ["/lauds/2026-03-11%", "/lauds/2026-03-11%zz"] {
            assert_eq!(request(Method::GET, path, Body::empty()).await.status(), StatusCode::BAD_REQUEST);
        }
    }

    #[tokio::test]
    async fn beacons_enforce_limits_and_methods() {
        let get = request(Method::GET, "/api/usage", Body::empty()).await;
        assert_eq!(get.status(), StatusCode::METHOD_NOT_ALLOWED);
        assert_eq!(get.headers()[header::ALLOW], "POST");
        assert_eq!(request(Method::POST, "/api/usage", "lauds").await.status(), StatusCode::NO_CONTENT);
        assert_eq!(request(Method::POST, "/api/usage", "x".repeat(MAX_BEACON + 1)).await.status(), StatusCode::BAD_REQUEST);
    }
}
