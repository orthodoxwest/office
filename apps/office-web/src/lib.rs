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
pub mod pwa;
pub mod usage;
pub mod web_time;

use std::collections::HashMap;
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
use tools::review::provenance::{ProvenanceStatus, scan_provenance};

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
    review: Review,
    usage: Option<Store>,
}

/// Each corpus entry's provenance, which decides an hour's review notice.
pub(crate) struct Review {
    provenance: HashMap<String, ProvenanceStatus>,
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
    Calendar,
    Root,
}

impl Server {
    /// Loads the engine, templates, and text provenance from `data_dir`.
    pub fn new(data_dir: &Path) -> Result<Server, String> {
        let src = FsData::new(data_dir);
        let engine = Engine::load(&src).map_err(|e| format!("creating office engine: {e}"))?;
        let calendar = CalendarData::load(&src).map_err(|e| format!("loading calendar data: {e}"))?;
        let version = pwa::compute_version(data_dir);
        let pages = Pages::new(pwa::asset_url).map_err(|e| format!("parsing templates: {e}"))?;
        let inventory = scan_provenance(&src).map_err(|e| format!("loading provenance: {e}"))?;
        let provenance = inventory.entries.iter().map(|e| (e.key.clone(), e.status)).collect();
        Ok(Server { engine, cache: YearCache::new(calendar), pages, version, review: Review { provenance }, usage: None })
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

    fn handle(&self, route: Route, method: &Method, uri: &Uri, headers: &HeaderMap, body: BeaconBody) -> Response<Body> {
        let raw_query = uri.query().unwrap_or("");
        let Some(path) = unescape_path(uri.path()) else { return bad_request() };
        let query = Query::parse(raw_query);
        let host = headers.get(header::HOST).and_then(|h| h.to_str().ok()).or_else(|| uri.authority().map(|a| a.as_str())).unwrap_or("");
        let req = Req { method, headers, path: &path, query: &query };
        match route {
            Route::UsageEvent => usage::handle_event(self.usage.as_ref(), method, headers, host, body),
            Route::UsageDashboard => usage::handle_dashboard(self.usage.as_ref(), &self.pages, method, &query),
            Route::Static => pwa::serve_static(&path, query.get("v")),
            Route::ServiceWorker => pwa::service_worker(&self.version),
            Route::Ics => self.ics(&query, headers, host),
            Route::Reminders => self.reminders(&req),
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

/// Serves until the process ends. The current year's calendar page is
/// composed in the background so the first visit to it is fast.
pub fn run(server: Server, addr: &str) -> Result<(), String> {
    let runtime = tokio::runtime::Builder::new_multi_thread().enable_all().build().map_err(|e| e.to_string())?;
    runtime.block_on(async {
        let server = Arc::new(server);
        let warm = Arc::clone(&server);
        tokio::task::spawn_blocking(move || {
            let year = Server::local_year();
            if let Err(e) = warm.cache.months(year, &warm.engine) {
                eprintln!("warn: pre-warming cache for {year}: {e}");
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
        for path in ["/lauds/2026-03-11", "/static/style.css", "/reminders", "/sw.js"] {
            let get = request(Method::GET, path, Body::empty()).await;
            let head = request(Method::HEAD, path, Body::empty()).await;
            assert_eq!(get.status(), StatusCode::OK, "{path}");
            assert_eq!(head.status(), get.status(), "{path}");
            assert_eq!(head.headers().get(header::CONTENT_TYPE), get.headers().get(header::CONTENT_TYPE));
            assert!(axum::body::to_bytes(head.into_body(), usize::MAX).await.unwrap().is_empty());
        }
        let redirect = request(Method::GET, "/calendar?form=priest", Body::empty()).await;
        assert_eq!(redirect.status(), StatusCode::FOUND);
        assert!(redirect.headers()[header::LOCATION].to_str().unwrap().contains("?form=priest#d-"));
        assert!(axum::body::to_bytes(redirect.into_body(), usize::MAX).await.unwrap().is_empty());
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
