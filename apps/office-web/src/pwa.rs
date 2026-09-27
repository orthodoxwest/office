//! Static assets, the service worker, and the build version that stamps
//! them. Only named embedded assets are public; directories and filesystem
//! redirects are not application routes.

use std::io::Read as _;
use std::path::Path;

use axum::body::Body;
use axum::http::{Response, StatusCode, header};
use sha2::{Digest, Sha256};

use crate::http::{http_error, response, set};

include!(concat!(env!("OUT_DIR"), "/static_files.rs"));

/// An embedded file's bytes, by path under the embedding directory.
pub fn file(name: &str) -> Option<&'static [u8]> {
    FILES.iter().find(|(n, _)| *n == name).map(|(_, b)| *b)
}

/// A deterministic build identifier: the SHA-256 of the server binary and every data file (path and
/// contents, depth first with each directory sorted by name), cut to 12 hex digits. It changes
/// exactly when a deploy can change a page.
pub fn compute_version(data_dir: &Path) -> String {
    let mut h = Sha256::new();
    if let Ok(exe) = std::env::current_exe()
        && let Ok(bytes) = std::fs::read(exe)
    {
        h.update(&bytes);
    }
    walk_data(data_dir, "", &mut h);
    let digest = h.finalize();
    digest.iter().map(|b| format!("{b:02x}")).collect::<String>()[..12].to_string()
}

/// `fs.WalkDir` visits each directory's entries sorted by name, descending
/// into a directory where it sorts, and does not follow symlinked ones.
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
        if let Ok(mut f) = std::fs::File::open(entry.path()) {
            let mut buf = Vec::new();
            // Reading a symlinked directory as a file fails.
            if f.read_to_end(&mut buf).is_ok() {
                h.update(&buf);
            }
        }
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

/// Serves an exact embedded asset: immutable for a year with a build stamp,
/// revalidated otherwise. No host filesystem access or directory listings.
pub fn serve_static(url_path: &str, versioned: bool) -> Response<Body> {
    let Some(name) = url_path.strip_prefix("/static/") else {
        return http_error("404 page not found", StatusCode::NOT_FOUND);
    };
    let Some(body) = file(&format!("static/{name}")) else {
        return http_error("404 page not found", StatusCode::NOT_FOUND);
    };
    let mut resp = response(StatusCode::OK, body);
    set(&mut resp, header::CONTENT_TYPE, content_type(name, body));
    set(&mut resp, header::CACHE_CONTROL, if versioned { "public, max-age=31536000, immutable" } else { "no-cache" });
    resp
}

/// The service worker at the site root, stamped with the build version so
/// every deploy installs a fresh worker.
pub fn service_worker(version: &str) -> Response<Body> {
    let Some(src) = file("static/sw.js") else {
        return http_error("service worker unavailable", StatusCode::INTERNAL_SERVER_ERROR);
    };
    let body = String::from_utf8_lossy(src).replace("__VERSION__", version);
    let mut resp = response(StatusCode::OK, body);
    set(&mut resp, header::CONTENT_TYPE, "text/javascript; charset=utf-8");
    set(&mut resp, header::CACHE_CONTROL, "no-cache");
    resp
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn embeds_the_static_directory() {
        assert!(file("static/style.css").is_some());
        assert!(file("static/fonts/eb-garamond-regular.woff2").is_some());
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

    /// `fs.WalkDir` descends into a directory where its name sorts, so "a/"
    /// is hashed before "a.txt" although "a.txt" < "a/x" as a full path.
    #[test]
    fn compute_version_walks_in_sorted_depth_first_order() {
        let dir = std::env::temp_dir().join(format!("office-walk-{}", std::process::id()));
        std::fs::create_dir_all(dir.join("a")).unwrap();
        std::fs::write(dir.join("a/x"), "1").unwrap();
        std::fs::write(dir.join("a.txt"), "2").unwrap();
        let mut h = Sha256::new();
        h.update(b"a/x1a.txt2");
        let want: String = h.finalize().iter().map(|b| format!("{b:02x}")).collect();
        let mut walked = Sha256::new();
        walk_data(&dir, "", &mut walked);
        let got: String = walked.finalize().iter().map(|b| format!("{b:02x}")).collect();
        assert_eq!(got, want);
        std::fs::remove_dir_all(&dir).unwrap();
    }

    #[test]
    fn assets_keep_mime_types_and_versioned_cache_policy() {
        let r = serve_static("/static/style.css", true);
        assert_eq!(r.status(), StatusCode::OK);
        assert_eq!(r.headers()[header::CONTENT_TYPE], "text/css; charset=utf-8");
        assert_eq!(r.headers()[header::CACHE_CONTROL], "public, max-age=31536000, immutable");
        let r = serve_static("/static/style.css", false);
        assert_eq!(r.headers()[header::CACHE_CONTROL], "no-cache");
        for path in ["/static/missing.css", "/static/icons", "/static/style.css/", "/static/index.html"] {
            let r = serve_static(path, false);
            assert_eq!(r.status(), StatusCode::NOT_FOUND);
            assert!(!r.headers().contains_key(header::CACHE_CONTROL));
        }
    }
}
