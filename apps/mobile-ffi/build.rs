//! Embeds the engine's data files: everything under `data/` except the review ledgers, Markdown
//! notes, and chant scores. Entries are listed depth first, each directory's names in byte order,
//! the order `DataSource::walk` promises.

use std::fmt::Write as _;
use std::path::{Path, PathBuf};

fn embedded(rel: &str) -> bool {
    !(rel.starts_with("review/") || rel.starts_with("texts/chant/") || rel.ends_with(".md"))
}

fn walk(dir: &Path, rel: &str, files: &mut Vec<(String, PathBuf)>) {
    let mut entries: Vec<_> = std::fs::read_dir(dir).expect("reading data dir").map(|e| e.expect("data entry")).collect();
    entries.sort_by_key(|e| e.file_name());
    for entry in entries {
        let name = entry.file_name().into_string().expect("UTF-8 data file name");
        if name.starts_with('.') {
            continue;
        }
        let child = if rel.is_empty() { name } else { format!("{rel}/{name}") };
        let path = entry.path();
        if path.is_dir() {
            walk(&path, &child, files);
        } else if embedded(&child) {
            files.push((child, path));
        }
    }
}

fn main() {
    let data = Path::new(env!("CARGO_MANIFEST_DIR")).join("../../data").canonicalize().expect("data dir");
    println!("cargo:rerun-if-changed={}", data.display());
    let mut files = Vec::new();
    walk(&data, "", &mut files);
    let mut src = String::from("pub static FILES: &[(&str, &[u8])] = &[\n");
    for (rel, path) in &files {
        writeln!(src, "    ({rel:?}, include_bytes!({:?})),", path.display().to_string()).unwrap();
    }
    src.push_str("];\n");
    let out = PathBuf::from(std::env::var("OUT_DIR").expect("OUT_DIR")).join("data_files.rs");
    std::fs::write(out, src).expect("writing data_files.rs");
}
