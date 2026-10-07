//! Static assets, their content stamps, the service worker, and the build
//! version that names its cache. Only named embedded assets are public;
//! directories and filesystem redirects are not application routes.

use std::io::Read as _;
use std::path::Path;

use axum::body::Body;
use axum::http::{Response, StatusCode, header};
use sha2::{Digest, Sha256};

use crate::http::{http_error, response, set};

include!(concat!(env!("OUT_DIR"), "/static_files.rs"));

/// An embedded file's bytes, by path under the embedding directory.
pub fn file(name: &str) -> Option<&'static [u8]> {
    FILES.iter().find(|(n, ..)| *n == name).map(|(_, b, _)| *b)
}

/// An embedded file's content stamp, the `v` its URLs carry. A file keeps
/// its stamp, and so its cached copy, across deploys that leave it alone.
pub fn stamp(name: &str) -> Option<&'static str> {
    FILES.iter().find(|(n, ..)| *n == name).map(|(.., s)| *s)
}

/// The URL of an asset under /static/, stamped with its content.
pub fn asset_url(name: &str) -> String {
    let name = name.strip_prefix('/').unwrap_or(name);
    render_html::links::static_url(name, stamp(&format!("static/{name}")).unwrap_or(""))
}

/// A deterministic build identifier: the SHA-256 of the server binary and every data file (path and
/// contents, depth first with each directory sorted by name), cut to 12 hex digits. It changes
/// exactly when a deploy can change a page.
pub fn compute_version(data_dir: &Path) -> String {
    let mut h = Sha256::new();
    if let Ok(exe) = std::env::current_exe() {
        hash_file(&exe, &mut h);
    }
    walk_data(data_dir, "", &mut h);
    let digest = h.finalize();
    digest.iter().map(|b| format!("{b:02x}")).collect::<String>()[..12].to_string()
}

/// Streams a file into the hash rather than holding it in memory; the
/// server binary alone is tens of megabytes.
fn hash_file(path: &Path, h: &mut Sha256) {
    let Ok(mut f) = std::fs::File::open(path) else { return };
    let mut buf = [0u8; 64 * 1024];
    // Reading a symlinked directory as a file fails before any bytes.
    while let Ok(n @ 1..) = f.read(&mut buf) {
        h.update(&buf[..n]);
    }
}

/// Visits each directory's entries sorted by name, descending into a
/// directory where it sorts, and does not follow symlinked ones.
fn walk_data(dir: &Path, rel: &str, h: &mut Sha256) {
    let Ok(entries) = std::fs::read_dir(dir) else { return };
    let mut entries: Vec<_> = entries.filter_map(Result::ok).collect();
    entries.sort_by_key(|e| e.file_name());
    for entry in entries {
        let name = entry.file_name().to_string_lossy().into_owned();
        let path = if rel.is_empty() { name } else { format!("{rel}/{name}") };
        let is_dir = entry.file_type().map(|t| t.is_dir()).unwrap_or(false);
        if is_dir {
            walk_data(&entry.path(), &path, h);
            continue;
        }
        h.update(path.as_bytes());
        hash_file(&entry.path(), h);
    }
}

/// Content types for embedded assets; unrecognized extensions fall back to inspecting the content.
fn content_type(name: &str, body: &[u8]) -> &'static str {
    let ext = name.rsplit_once('.').map(|(_, e)| e).unwrap_or("");
    match ext {
        "css" => "text/css; charset=utf-8",
        "js" | "mjs" => "text/javascript; charset=utf-8",
        "svg" => "image/svg+xml",
        "png" => "image/png",
        "jpg" | "jpeg" => "image/jpeg",
        "json" => "application/json",
        "html" | "htm" => "text/html; charset=utf-8",
        "txt" => "text/plain; charset=utf-8",
        "webmanifest" => "application/manifest+json",
        "woff2" => "font/woff2",
        _ if std::str::from_utf8(&body[..body.len().min(512)]).is_ok() => "text/plain; charset=utf-8",
        _ => "application/octet-stream",
    }
}

/// Serves an exact embedded asset: immutable for a year under its current
/// stamp, revalidated when unstamped. A stamp from another build (a page
/// cached before a deploy, or a machine mid-rollout) still gets this build's
/// bytes but must not be stored, or they would sit under a URL that promises
/// other content. No host filesystem access or directory listings.
pub fn serve_static(url_path: &str, v: &str) -> Response<Body> {
    let Some(name) = url_path.strip_prefix("/static/") else {
        return http_error("404 page not found", StatusCode::NOT_FOUND);
    };
    let Some(&(_, body, current)) = FILES.iter().find(|(n, ..)| n.strip_prefix("static/") == Some(name)) else {
        return http_error("404 page not found", StatusCode::NOT_FOUND);
    };
    let cache = match v {
        "" => "no-cache",
        v if v == current => "public, max-age=31536000, immutable",
        _ => "no-store",
    };
    let mut resp = response(StatusCode::OK, body);
    set(&mut resp, header::CONTENT_TYPE, content_type(name, body));
    set(&mut resp, header::CACHE_CONTROL, cache);
    resp
}

/// The service worker at the site root, stamped with the build version so
/// every deploy installs a fresh worker, and given every asset's stamp so it
/// precaches the URLs pages request.
pub fn service_worker(version: &str) -> Response<Body> {
    let Some(src) = file("static/sw.js") else {
        return http_error("service worker unavailable", StatusCode::INTERNAL_SERVER_ERROR);
    };
    let stamps: Vec<String> = FILES.iter().map(|(n, _, s)| format!("\"/{n}\": \"{s}\"")).collect();
    let body = String::from_utf8_lossy(src).replace("__VERSION__", version).replace("/*__ASSET_STAMPS__*/", &stamps.join(", "));
    let mut resp = response(StatusCode::OK, body);
    set(&mut resp, header::CONTENT_TYPE, "text/javascript; charset=utf-8");
    set(&mut resp, header::CACHE_CONTROL, "no-cache");
    resp
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn service_worker_learns_every_asset_stamp() {
        let resp = service_worker("build1");
        let body = String::from_utf8(
            tokio::runtime::Builder::new_current_thread()
                .build()
                .unwrap()
                .block_on(axum::body::to_bytes(resp.into_body(), usize::MAX))
                .unwrap()
                .to_vec(),
        )
        .unwrap();
        assert!(body.contains(r#"var VERSION = "build1";"#));
        assert!(!body.contains("__ASSET_STAMPS__"));
        let font = stamp("static/fonts/junicode-regular.woff2").unwrap();
        assert!(body.contains(&format!(r#""/static/fonts/junicode-regular.woff2": "{font}""#)));
    }

    #[test]
    fn compute_version_is_deterministic_and_data_sensitive() {
        let dir = std::env::temp_dir().join(format!("office-version-{}", std::process::id()));
        std::fs::create_dir_all(&dir).unwrap();
        std::fs::write(dir.join("a.txt"), "one").unwrap();
        let v1 = compute_version(&dir);
        assert_eq!(v1, compute_version(&dir));
        assert_eq!(v1.len(), 12);
        std::fs::write(dir.join("a.txt"), "two").unwrap();
        assert_ne!(compute_version(&dir), v1);
        std::fs::remove_dir_all(&dir).unwrap();
    }

    #[test]
    fn assets_keep_mime_types_and_stamped_cache_policy() {
        let current = stamp("static/style.css").unwrap();
        let r = serve_static("/static/style.css", current);
        assert_eq!(r.status(), StatusCode::OK);
        assert_eq!(r.headers()[header::CONTENT_TYPE], "text/css; charset=utf-8");
        assert_eq!(r.headers()[header::CACHE_CONTROL], "public, max-age=31536000, immutable");
        let r = serve_static("/static/style.css", "");
        assert_eq!(r.headers()[header::CACHE_CONTROL], "no-cache");
        let r = serve_static("/static/style.css", "0123456789ab");
        assert_eq!(r.status(), StatusCode::OK);
        assert_eq!(r.headers()[header::CACHE_CONTROL], "no-store", "another build's stamp is never cached");
        for path in ["/static/missing.css", "/static/icons", "/static/style.css/", "/static/index.html"] {
            let r = serve_static(path, "");
            assert_eq!(r.status(), StatusCode::NOT_FOUND);
            assert!(!r.headers().contains_key(header::CACHE_CONTROL));
        }
    }
}
