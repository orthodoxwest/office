//! The Office web server. Ported from Go's `internal/web` and
//! `internal/usage`: the same routes, pages, static assets, reminder feed,
//! and usage store, answering every request as the Go server does (Phase 5
//! of RUST-PORT.md). Axum only carries requests; routing reproduces Go's
//! `http.ServeMux` so path canonicalization and fallbacks match too.

mod cache;
pub mod gonet;
pub mod gotime;
mod handlers;
mod ics;
pub mod pwa;
pub mod usage;

use std::collections::{BTreeMap, HashMap};
use std::path::Path;
use std::sync::Arc;

use axum::Router;
use axum::body::Body;
use axum::extract::{Request, State};
use axum::http::{HeaderMap, Method, Response, StatusCode, Uri, header};
use calendar::CalendarData;
use office::Engine;
use render_html::Pages;
use tools::fs::FsData;
use tools::review::prescreen::{Suspicion, suspicion_by_key};
use tools::review::provenance::{ProvenanceStatus, scan_provenance};

use crate::cache::YearCache;
use crate::gonet::{Query, bad_request, clean_path, redirect, unescape_path, url_string};
use crate::handlers::Req;
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

/// The review metadata an hour page discloses: each corpus entry's
/// provenance and its suspicions.
#[derive(Default)]
pub(crate) struct Review {
    provenance: HashMap<String, ProvenanceStatus>,
    suspicions: BTreeMap<String, Vec<Suspicion>>,
}

/// The registered patterns of Go's mux, by what they serve.
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

/// A path's segments as Go's routing tree compares them: split on the
/// escaped slashes, each segment unescaped.
fn segments(escaped: &str) -> Vec<String> {
    escaped.trim_start_matches('/').split('/').map(|s| unescape_path(s).unwrap_or_else(|| s.to_string())).collect()
}

fn route(escaped: &str) -> Route {
    let segs = segments(escaped);
    let segs: Vec<&str> = segs.iter().map(String::as_str).collect();
    match segs.as_slice() {
        ["api", "usage"] => Route::UsageEvent,
        ["admin", "usage"] => Route::UsageDashboard,
        ["sw.js"] => Route::ServiceWorker,
        ["office.ics"] => Route::Ics,
        ["reminders"] => Route::Reminders,
        ["calendar", ..] => Route::Calendar,
        ["static", _, ..] => Route::Static,
        _ => Route::Root,
    }
}

/// Whether a registered pattern matches the path exactly, which decides
/// Go's "/tree" → "/tree/" redirect.
fn exact(escaped: &str) -> bool {
    let segs = segments(escaped);
    let segs: Vec<&str> = segs.iter().map(String::as_str).collect();
    matches!(
        segs.as_slice(),
        ["api", "usage"]
            | ["admin", "usage"]
            | ["sw.js"]
            | ["office.ics"]
            | ["reminders"]
            | ["calendar"]
            | ["calendar", ""]
            | ["static", ""]
            | [""]
    )
}

impl Server {
    /// Loads the engine, templates, and review metadata from `data_dir`.
    pub fn new(data_dir: &Path) -> Result<Server, String> {
        let src = FsData::new(data_dir);
        let engine = Engine::load(&src).map_err(|e| format!("creating office engine: {e}"))?;
        let calendar = CalendarData::load(&src).map_err(|e| format!("loading calendar data: {e}"))?;
        let version = pwa::compute_version(data_dir);
        let pages = Pages::new(&version).map_err(|e| format!("parsing templates: {e}"))?;
        let inventory = scan_provenance(&src).map_err(|e| format!("loading provenance: {e}"))?;
        let suspicions = suspicion_by_key(&src, &inventory).map_err(|e| format!("loading review suspicions: {e}"))?;
        let provenance = inventory.entries.iter().map(|e| (e.key.clone(), e.status)).collect();
        Ok(Server { engine, cache: YearCache::new(calendar), pages, version, review: Review { provenance, suspicions }, usage: None })
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

    /// Answers one request as Go's mux and handlers would.
    pub fn handle(&self, method: &Method, uri: &Uri, headers: &HeaderMap, body: BeaconBody) -> Response<Body> {
        let raw_path = uri.path();
        let raw_query = uri.query().unwrap_or("");
        // Go rejects a malformed escape while parsing the request line.
        let Some(path) = unescape_path(raw_path) else { return bad_request() };
        let cleaned = clean_path(raw_path);
        if method != Method::CONNECT {
            if !exact(&cleaned) && !cleaned.ends_with('/') && exact(&format!("{cleaned}/")) {
                return redirect(method, &url_string(&format!("{}/", clean_path(&path)), raw_query), StatusCode::TEMPORARY_REDIRECT);
            }
            if cleaned != raw_path {
                return redirect(method, &url_string(&cleaned, raw_query), StatusCode::TEMPORARY_REDIRECT);
            }
        }
        let query = Query::parse(raw_query);
        let host = headers.get(header::HOST).and_then(|h| h.to_str().ok()).or_else(|| uri.authority().map(|a| a.as_str())).unwrap_or("");
        let req = Req { method, headers, path: &path, query: &query };
        match route(&cleaned) {
            Route::UsageEvent => usage::handle_event(self.usage.as_ref(), method, headers, host, body),
            Route::UsageDashboard => usage::handle_dashboard(self.usage.as_ref(), &self.pages, method, &query),
            Route::Static => pwa::serve_static(&path, raw_query, !query.get("v").is_empty()),
            Route::ServiceWorker => pwa::service_worker(&self.version),
            Route::Ics => self.ics(&query, headers, host),
            Route::Reminders => self.reminders(&req),
            Route::Calendar => self.calendar(&req),
            Route::Root => self.root(&req),
        }
    }

    /// The build stamp on static URLs and the service worker.
    pub fn version(&self) -> &str {
        &self.version
    }
}

async fn entry(State(server): State<Arc<Server>>, req: Request) -> Response<Body> {
    let (parts, body) = req.into_parts();
    // Only the beacon reads a body, and never more than it allows.
    let beacon = if route(&clean_path(parts.uri.path())) == Route::UsageEvent && parts.method == Method::POST {
        axum::body::to_bytes(body, MAX_BEACON).await.ok().map(|b| b.to_vec())
    } else {
        Some(Vec::new())
    };
    let task = tokio::task::spawn_blocking(move || server.handle(&parts.method, &parts.uri, &parts.headers, beacon));
    match task.await {
        Ok(resp) => resp,
        Err(e) => gonet::http_error(&e.to_string(), StatusCode::INTERNAL_SERVER_ERROR),
    }
}

/// Go's `ListenAndServe(":8080")` listens on every interface.
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
        let app = Router::new().fallback(entry).with_state(server);
        axum::serve(listener, app).await.map_err(|e| format!("server error: {e}"))
    })
}

/// One server over the live data, shared by the tests.
#[cfg(test)]
pub(crate) fn test_server() -> &'static Server {
    static SERVER: std::sync::OnceLock<Server> = std::sync::OnceLock::new();
    SERVER.get_or_init(|| Server::new(Path::new("../../data")).unwrap())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn routes_follow_go_mux() {
        assert_eq!(route("/api/usage"), Route::UsageEvent);
        assert_eq!(route("/api/usage/"), Route::Root);
        assert_eq!(route("/static/style.css"), Route::Static);
        assert_eq!(route("/static/"), Route::Static);
        assert_eq!(route("/static"), Route::Root);
        assert_eq!(route("/%73tatic/style.css"), Route::Static);
        assert_eq!(route("/static%2Fstyle.css"), Route::Root);
        assert_eq!(route("/calendar"), Route::Calendar);
        assert_eq!(route("/calendar/2026"), Route::Calendar);
        assert_eq!(route("/reminders/"), Route::Root);
        assert!(exact("/static/"));
        assert!(!exact("/static"));
        assert!(exact("/"));
    }
}
