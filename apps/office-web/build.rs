//! Embed the web assets at build time, keyed by their public static path,
//! each with a stamp of its content for cache-busting URLs.

use std::collections::HashMap;
use std::fmt::Write as _;
use std::path::{Path, PathBuf};

use sha2::{Digest, Sha256};

#[path = "src/css.rs"]
mod css;

fn walk(dir: &Path, rel: &str, files: &mut Vec<(String, PathBuf)>) {
    let mut entries: Vec<_> = std::fs::read_dir(dir).expect("reading static dir").map(|e| e.expect("static entry")).collect();
    entries.sort_by_key(|e| e.file_name());
    for entry in entries {
        let name = entry.file_name().into_string().expect("UTF-8 static file name");
        if name.starts_with('.') || name.starts_with('_') {
            continue;
        }
        let path = entry.path();
        let child = format!("{rel}/{name}");
        if path.is_dir() {
            walk(&path, &child, files);
        } else {
            files.push((child, path));
        }
    }
}

/// Twelve hex digits of the SHA-256, like the build version.
fn stamp(bytes: &[u8]) -> String {
    Sha256::digest(bytes).iter().map(|b| format!("{b:02x}")).collect::<String>()[..12].to_string()
}

fn main() {
    let manifest = PathBuf::from(std::env::var("CARGO_MANIFEST_DIR").expect("CARGO_MANIFEST_DIR"));
    let root = manifest.join("static").canonicalize().expect("apps/office-web/static");
    let out_dir = PathBuf::from(std::env::var("OUT_DIR").expect("OUT_DIR"));
    println!("cargo:rerun-if-changed={}", root.display());
    println!("cargo:rerun-if-changed={}", manifest.join("src/css.rs").display());
    let mut files = Vec::new();
    walk(&root, "static", &mut files);

    // Stylesheets name other assets by stamped URL, so everything else is
    // stamped first; no stylesheet references another.
    let mut stamps: HashMap<String, String> = HashMap::new();
    for (name, path) in files.iter().filter(|(n, _)| !n.ends_with(".css")) {
        stamps.insert(name.clone(), stamp(&std::fs::read(path).expect("reading static file")));
    }
    let mut embedded: Vec<(String, PathBuf, String)> = Vec::new();
    for (name, path) in &files {
        if !name.ends_with(".css") {
            embedded.push((name.clone(), path.clone(), stamps[name].clone()));
            continue;
        }
        let src = std::fs::read_to_string(path).expect("reading stylesheet");
        let dir = name.rsplit_once('/').map(|(d, _)| d).unwrap_or("");
        let css = css::stamp_urls(&css::strip_comments(&src), |rel| stamps.get(&format!("{dir}/{rel}")).cloned())
            .unwrap_or_else(|e| panic!("{name}: {e}"));
        let dest = out_dir.join(name.replace('/', "__"));
        std::fs::write(&dest, &css).expect("writing processed stylesheet");
        embedded.push((name.clone(), dest, stamp(css.as_bytes())));
    }

    let mut out = String::from("/// The embedded files, by path, with their content stamps.\npub static FILES: &[(&str, &[u8], &str)] = &[\n");
    for (name, path, stamp) in &embedded {
        writeln!(out, "    ({name:?}, include_bytes!({:?}), {stamp:?}),", path.display().to_string()).expect("write");
    }
    out.push_str("];\n");
    std::fs::write(out_dir.join("static_files.rs"), out).expect("writing static_files.rs");
}
