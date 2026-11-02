//! Embeds the engine's data files: those `pack::embedded` keeps, in the form `pack::packed` gives.
//! Entries are listed depth first, each directory's names in byte order, the order
//! `DataSource::walk` promises. Each file is its own DEFLATE stream, so an edit to one file
//! changes only that file's bytes in the library and an app update stays the size of the edit;
//! the library inflates them once when it loads the engine (data.rs).

use std::fmt::Write as _;
use std::path::{Path, PathBuf};

#[path = "src/pack.rs"]
mod pack;

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
        } else if pack::embedded(&child) {
            files.push((child, path));
        }
    }
}

fn main() {
    println!("cargo:rerun-if-changed=src/pack.rs");
    let data = Path::new(env!("CARGO_MANIFEST_DIR")).join("../../data").canonicalize().expect("data dir");
    println!("cargo:rerun-if-changed={}", data.display());
    let mut files = Vec::new();
    walk(&data, "", &mut files);
    let mut blob = Vec::new();
    let mut src = String::from(
        "/// Each file's path, the byte range of its DEFLATE stream, and its inflated length.\npub static FILES: &[(&str, usize, usize, usize)] = &[\n",
    );
    for (rel, path) in &files {
        let bytes = pack::packed(rel, std::fs::read(path).expect("reading data file"));
        let start = blob.len();
        blob.extend(miniz_oxide::deflate::compress_to_vec(&bytes, 10));
        writeln!(src, "    ({rel:?}, {start}, {}, {}),", blob.len(), bytes.len()).expect("writing to a String cannot fail");
    }
    src.push_str("];\n");
    src.push_str("/// Every file's DEFLATE stream, end to end.\npub static DEFLATED: &[u8] = include_bytes!(concat!(env!(\"OUT_DIR\"), \"/data.deflate\"));\n");
    let out = PathBuf::from(std::env::var("OUT_DIR").expect("OUT_DIR"));
    std::fs::write(out.join("data.deflate"), blob).expect("writing data.deflate");
    std::fs::write(out.join("data_files.rs"), src).expect("writing data_files.rs");
}
