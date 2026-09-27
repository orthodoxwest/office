//! Embeds `internal/web/static` as Go's `//go:embed static` does: every file
//! under it except names beginning with `.` or `_`, keyed by its path from
//! the embedding directory ("static/app.js").

use std::fmt::Write as _;
use std::path::{Path, PathBuf};

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

fn main() {
    let manifest = PathBuf::from(std::env::var("CARGO_MANIFEST_DIR").expect("CARGO_MANIFEST_DIR"));
    let root = manifest.join("../../internal/web/static").canonicalize().expect("internal/web/static");
    println!("cargo:rerun-if-changed={}", root.display());
    let mut files = Vec::new();
    walk(&root, "static", &mut files);
    let mut out = String::from("/// The embedded files, by path.\npub static FILES: &[(&str, &[u8])] = &[\n");
    for (name, path) in &files {
        writeln!(out, "    ({name:?}, include_bytes!({:?})),", path.display().to_string()).expect("write");
    }
    out.push_str("];\n");
    let dest = PathBuf::from(std::env::var("OUT_DIR").expect("OUT_DIR")).join("static_files.rs");
    std::fs::write(dest, out).expect("writing static_files.rs");
}
