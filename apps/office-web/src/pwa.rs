//! Static assets, the service worker, and the build version that stamps
//! them. Ported from Go's `web/pwa.go`; the file server reproduces Go's
//! `http.FileServer` over the embedded directory.

use std::io::Read as _;
use std::path::Path;

use axum::body::Body;
use axum::http::{Response, StatusCode, header};
use sha2::{Digest, Sha256};

use crate::gonet::{http_error, local_redirect, path_clean, response, set, url_string};

include!(concat!(env!("OUT_DIR"), "/static_files.rs"));

/// An embedded file's bytes, by path under the embedding directory.
pub fn file(name: &str) -> Option<&'static [u8]> {
    FILES.iter().find(|(n, _)| *n == name).map(|(_, b)| *b)
}

fn is_dir(name: &str) -> bool {
    DIRS.contains(&name)
}

/// A deterministic build identifier: the SHA-256 of the server binary and
/// every data file (path and contents, in Go's `fs.WalkDir` order), cut to
/// 12 hex digits. It changes exactly when a deploy can change a page.
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
            // A symlinked directory opens but does not read, as in Go.
            if f.read_to_end(&mut buf).is_ok() {
                h.update(&buf);
            }
        }
    }
}

/// `mime.TypeByExtension` for the embedded files' extensions, with the two
/// Go registers at startup; anything else is sniffed as Go would.
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

/// Go's `path.Base`.
fn path_base(p: &str) -> &str {
    if p.is_empty() {
        return ".";
    }
    let trimmed = p.trim_end_matches('/');
    if trimmed.is_empty() {
        return "/";
    }
    trimmed.rsplit('/').next().unwrap_or(trimmed)
}

/// Serves `/static/…`: immutable for a year when the URL carries the build
/// stamp (`?v=`), revalidated otherwise.
pub fn serve_static(url_path: &str, raw_query: &str, versioned: bool) -> Response<Body> {
    let cache = if versioned { "public, max-age=31536000, immutable" } else { "no-cache" };
    let mut resp = serve_file(url_path, raw_query);
    // Go's error path drops the success-only cache header.
    if resp.status() != StatusCode::NOT_FOUND && resp.status() != StatusCode::INTERNAL_SERVER_ERROR {
        set(&mut resp, header::CACHE_CONTROL, cache);
    }
    resp
}

fn serve_file(url_path: &str, raw_query: &str) -> Response<Body> {
    if url_path.ends_with("/index.html") {
        return local_redirect("./", raw_query);
    }
    let name = path_clean(url_path);
    let fs_name = name.trim_start_matches('/');
    let dir = is_dir(fs_name);
    let content = file(fs_name);
    if !dir && content.is_none() {
        return http_error("404 page not found", StatusCode::NOT_FOUND);
    }
    if dir && !url_path.ends_with('/') {
        return local_redirect(&format!("{}/", path_base(url_path)), raw_query);
    }
    if !dir && url_path.ends_with('/') {
        let base = path_base(url_path);
        if base == "/" || base == "." {
            return http_error("http: attempting to traverse a non-directory", StatusCode::INTERNAL_SERVER_ERROR);
        }
        return local_redirect(&format!("../{base}"), raw_query);
    }
    match content {
        Some(body) => {
            let mut resp = response(StatusCode::OK, body);
            set(&mut resp, header::CONTENT_TYPE, content_type(fs_name, body));
            set(&mut resp, header::ACCEPT_RANGES, "bytes");
            resp
        }
        None => dir_list(fs_name),
    }
}

/// Go's `dirList`: the entries sorted by name, directories with a slash.
fn dir_list(dir: &str) -> Response<Body> {
    let prefix = format!("{dir}/");
    let mut names: Vec<String> = FILES
        .iter()
        .map(|(n, _)| *n)
        .chain(DIRS.iter().copied())
        .filter_map(|n| n.strip_prefix(&prefix))
        .filter(|rest| !rest.is_empty() && !rest.contains('/'))
        .map(|rest| if is_dir(&format!("{prefix}{rest}")) { format!("{rest}/") } else { rest.to_string() })
        .collect();
    names.sort_by(|a, b| a.trim_end_matches('/').cmp(b.trim_end_matches('/')));
    let mut body = String::from("<!doctype html>\n<meta name=\"viewport\" content=\"width=device-width\">\n<pre>\n");
    for name in names {
        let href = url_string(&name, "");
        let text = name.replace('&', "&amp;").replace('<', "&lt;").replace('>', "&gt;").replace('"', "&#34;").replace('\'', "&#39;");
        body.push_str(&format!("<a href=\"{href}\">{text}</a>\n"));
    }
    body.push_str("</pre>\n");
    let mut resp = response(StatusCode::OK, body);
    set(&mut resp, header::CONTENT_TYPE, "text/html; charset=utf-8");
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
        assert!(is_dir("static/icons"));
        assert_eq!(path_base("/static/icons"), "icons");
        assert_eq!(path_base("/static/style.css/"), "style.css");
    }

    // Ported from Go's `TestComputeVersionDeterministicAndDataSensitive`.
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
    fn compute_version_walks_in_go_order() {
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
    fn serves_files_as_go_does() {
        let r = serve_static("/static/style.css", "v=1", true);
        assert_eq!(r.status(), StatusCode::OK);
        assert_eq!(r.headers()[header::CONTENT_TYPE], "text/css; charset=utf-8");
        assert_eq!(r.headers()[header::CACHE_CONTROL], "public, max-age=31536000, immutable");
        let r = serve_static("/static/missing.css", "", false);
        assert_eq!(r.status(), StatusCode::NOT_FOUND);
        assert!(r.headers().get(header::CACHE_CONTROL).is_none());
        let r = serve_static("/static/icons", "a=b", false);
        assert_eq!(r.status(), StatusCode::MOVED_PERMANENTLY);
        assert_eq!(r.headers()[header::LOCATION], "icons/?a=b");
        let r = serve_static("/static/style.css/", "", false);
        assert_eq!(r.headers()[header::LOCATION], "../style.css");
        let r = serve_static("/static/index.html", "", false);
        assert_eq!(r.headers()[header::LOCATION], "./");
    }
}
