//! The Office web server: Axum routes over a shared calendar, corpus,
//! templates, reminder feed, and optional usage store.

mod cache;
mod crawl;
// build.rs trims and stamps the stylesheet with this module; the library
// compiles it only to test it.
#[cfg(test)]
mod css;
mod feasts;
mod handlers;
mod http;
mod ics;
mod moved;
pub mod pwa;
pub mod usage;
pub mod web_time;

use std::path::{Path, PathBuf};
use std::sync::{Arc, OnceLock};

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
use crate::http::{Query, bad_request, redirect, unescape_path};
use crate::usage::{BeaconBody, MAX_BEACON, Store};

/// Everything a request reads, loaded once and shared.
pub struct Server {
    engine: Engine,
    cache: YearCache,
    pages: Pages,
    /// Hashing the binary and the data takes as long as loading the engine,
    /// so it waits until the server listens; see [`Server::version`].
    version: OnceLock<String>,
    data_dir: PathBuf,
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
    About,
    Beta,
    Calendar,
    Feast,
    Robots,
    Sitemap,
    Root,
}

impl Server {
    /// Loads the engine and templates from `data_dir`.
    pub fn new(data_dir: &Path) -> Result<Server, String> {
        let src = FsData::new(data_dir);
        let engine = Engine::load(&src).map_err(|e| format!("creating office engine: {e}"))?;
        let calendar = CalendarData::load(&src).map_err(|e| format!("loading calendar data: {e}"))?;
        let pages = Pages::new(pwa::asset_url)
            .map_err(|e| format!("parsing templates: {e}"))?
            .with_chant(office::chant::offers_chant(&engine.texts));
        Ok(Server {
            engine,
            cache: YearCache::new(calendar),
            pages,
            version: OnceLock::new(),
            data_dir: data_dir.to_path_buf(),
            usage: None,
            canonical: None,
        })
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
        let site = match self.canonical.as_deref() {
            Some(canonical) => format!("https://{canonical}"),
            None => ics::base_url(headers, host),
        };
        let req = Req { method, headers, path: &path, query: &query, site: &site };
        if let Some(canonical) = self.canonical.as_deref()
            && moved::is_elsewhere(host, canonical)
        {
            match route {
                // Clients that already hold these URLs keep using them where they are.
                Route::UsageEvent | Route::Static => {}
                Route::Ics => return self.ics(&query, &format!("https://{canonical}")),
                Route::ServiceWorker => return moved::farewell_worker(),
                Route::UsageDashboard
                | Route::Reminders
                | Route::Privacy
                | Route::About
                | Route::Beta
                | Route::Calendar
                | Route::Feast
                | Route::Robots
                | Route::Sitemap
                | Route::Root => {
                    let target = uri.path_and_query().map(|p| p.as_str()).unwrap_or("/");
                    return moved::forward(canonical, method, headers, target);
                }
            }
        }
        match route {
            Route::UsageEvent => usage::handle_event(self.usage.as_ref(), method, headers, host, self.canonical.as_deref(), body),
            Route::UsageDashboard => usage::handle_dashboard(self.usage.as_ref(), &self.pages, method, &query),
            Route::Static => pwa::serve_static(&path, query.get("v")),
            Route::ServiceWorker => pwa::service_worker(self.version()),
            Route::Ics => self.ics(&query, &ics::base_url(headers, host)),
            Route::Reminders => self.reminders(&req),
            Route::Privacy => self.privacy(&req),
            Route::About => self.about(&req),
            // The Android app's closed-test sign-up, retired now the app is on Google Play.
            Route::Beta => redirect("/", StatusCode::MOVED_PERMANENTLY),
            Route::Calendar => self.calendar(&req),
            Route::Feast => self.feast(&req),
            Route::Robots => crawl::robots(&site),
            Route::Sitemap => crawl::sitemap(&site, Server::local_year(), self.cache.feasts()),
            Route::Root => self.root(&req),
        }
    }

    /// Composes `year`'s calendar months ahead of the first request for them.
    fn warm_year(&self, year: i32) {
        if let Err(e) = self.cache.months(year, &self.engine) {
            eprintln!("warn: pre-warming cache for {year}: {e}");
        }
    }

    /// The build stamp on the service worker and its page cache, computed
    /// on first use. [`run`] computes it in the background as soon as the
    /// server listens; a request that comes first waits for it.
    pub fn version(&self) -> &str {
        self.version.get_or_init(|| pwa::compute_version(&self.data_dir))
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
            .route("/about", endpoint(Route::About))
            .route("/beta", endpoint(Route::Beta))
            .route("/calendar", endpoint(Route::Calendar))
            .route("/calendar/", endpoint(Route::Calendar))
            .route("/calendar/{*path}", endpoint(Route::Calendar))
            .route("/feast", endpoint(Route::Feast))
            .route("/feast/", endpoint(Route::Feast))
            .route("/feast/{*path}", endpoint(Route::Feast))
            .route("/robots.txt", endpoint(Route::Robots))
            .route("/sitemap.xml", endpoint(Route::Sitemap))
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

/// Binds `addr` before the server loads, so that connections arriving while
/// it loads wait in the backlog instead of being refused. An address of the
/// form `:8080` listens on every interface.
pub fn listen(addr: &str) -> Result<std::net::TcpListener, String> {
    let listener = match addr.strip_prefix(':') {
        Some(port) => {
            std::net::TcpListener::bind(format!("[::]:{port}")).or_else(|_| std::net::TcpListener::bind(format!("0.0.0.0:{port}")))
        }
        None => std::net::TcpListener::bind(addr),
    };
    listener.map_err(|e| format!("listen tcp {addr}: {e}"))
}

/// How long after starting the server waits to compose next year's calendar,
/// so that a machine fresh from a deploy spends its CPU on the readers in front
/// of it first.
const NEXT_YEAR_WARM_DELAY: std::time::Duration = std::time::Duration::from_secs(60);

/// Serves on `listener` (from [`listen`]) until the process ends. In the
/// background the build version is hashed and the current year's calendar
/// page composed at once, so the first visit to it is fast (a year takes
/// about half a second, nearly all of it composing each day's Lauds, Prime
/// and Vespers for the ordo's digest). Next year's follows a minute later: it
/// is one tap from the ordo and enters the offline precache each December.
pub fn run(server: Server, listener: std::net::TcpListener) -> Result<(), String> {
    let runtime = tokio::runtime::Builder::new_multi_thread().enable_all().build().map_err(|e| e.to_string())?;
    runtime.block_on(async {
        let server = Arc::new(server);
        let year = Server::local_year();
        let warm = Arc::clone(&server);
        tokio::task::spawn_blocking(move || {
            warm.version();
            warm.warm_year(year);
        });
        let warm = Arc::clone(&server);
        tokio::spawn(async move {
            tokio::time::sleep(NEXT_YEAR_WARM_DELAY).await;
            let _ = tokio::task::spawn_blocking(move || warm.warm_year(year + 1)).await;
        });
        listener.set_nonblocking(true).map_err(|e| format!("listen: {e}"))?;
        let listener = tokio::net::TcpListener::from_std(listener).map_err(|e| format!("listen: {e}"))?;
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
        for path in ["/lauds/2026-03-11", "/static/style.css", "/reminders", "/privacy", "/about", "/sw.js"] {
            let get = request(Method::GET, path, Body::empty()).await;
            let head = request(Method::HEAD, path, Body::empty()).await;
            assert_eq!(get.status(), StatusCode::OK, "{path}");
            assert_eq!(head.status(), get.status(), "{path}");
            assert_eq!(head.headers().get(header::CONTENT_TYPE), get.headers().get(header::CONTENT_TYPE));
            assert!(axum::body::to_bytes(head.into_body(), usize::MAX).await.unwrap().is_empty());
        }
        let beta = request(Method::GET, "/beta", Body::empty()).await;
        assert_eq!(beta.status(), StatusCode::MOVED_PERMANENTLY);
        assert_eq!(beta.headers()[header::LOCATION], "/");
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
    async fn search_engines_find_the_undated_pages_on_the_canonical_host() {
        let mut server = Server::new(Path::new("../../data")).unwrap();
        server.canonical = moved::parse_host("example.org");
        let router = Arc::new(server).router();
        let get = |host: &str, path: &str| {
            let request = Request::builder().uri(path).header(header::HOST, host).body(Body::empty()).unwrap();
            let router = router.clone();
            async move {
                let resp = router.oneshot(request).await.unwrap();
                let status = resp.status();
                (status, String::from_utf8_lossy(&axum::body::to_bytes(resp.into_body(), usize::MAX).await.unwrap()).into_owned())
            }
        };

        let (status, robots) = get("example.org", "/robots.txt").await;
        assert_eq!(status, StatusCode::OK);
        assert!(robots.contains("Disallow: /admin/") && robots.contains("Sitemap: https://example.org/sitemap.xml"), "{robots}");
        let (_, sitemap) = get("example.org", "/sitemap.xml").await;
        for page in ["/", "/lauds", "/compline", "/privacy", "/about"] {
            assert!(sitemap.contains(&format!("<loc>https://example.org{page}</loc>")), "{page}: {sitemap}");
        }
        assert!(!sitemap.contains("<loc>https://example.org/calendar</loc>"), "the bare ordo address only redirects");
        let (status, _) = get("office.fly.dev", "/robots.txt").await;
        assert_eq!(status, StatusCode::MOVED_PERMANENTLY);

        let canonical = |path: &str| format!("rel=\"canonical\" href=\"{}\"", path.replace('/', "&#x2f;"));
        let (_, today) = get("example.org", "/lauds?form=priest").await;
        assert!(today.contains(&canonical("https://example.org/lauds")) && !today.contains("noindex"));
        let (_, dated) = get("example.org", "/lauds?date=2026-12-25").await;
        assert!(dated.contains(&canonical("https://example.org/lauds/2026-12-25")) && dated.contains("noindex"));
        assert!(dated.contains("Lauds for Friday, December 25, 2026: The Nativity of Our Lord."), "the preview names the feast");
        let (_, home) = get("example.org", "/").await;
        assert!(home.contains("og:title\" content=\"Daily Office\"") && !home.contains("noindex"));
        assert!(home.contains("icons&#x2f;share-card.png") && home.contains("summary_large_image"));
        let (status, _) = get("example.org", &pwa::asset_url("icons/share-card.png")).await;
        assert_eq!(status, StatusCode::OK, "the preview card is served");
        let (_, far) = get("example.org", "/calendar/1900").await;
        assert!(far.contains("noindex"));
        let (_, missing) = get("example.org", "/nowhere").await;
        assert!(missing.contains("noindex") && !missing.contains("rel=\"canonical\""));
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
