//! What of `data/` goes into the apps, and in what form. Shared by build.rs, which packs the
//! files, and the tests, which hold the packed files to the data directory.

/// Whether the apps carry this file: not hidden files, the review ledgers, the Markdown notes, the audit's
/// suppressions, or the booklet's own chant scores (the apps' scores, named by corpus key, go in).
pub fn embedded(rel: &str) -> bool {
    let booklet =
        ["psalms/", "canticles/", "hymns/"].iter().any(|dir| rel.strip_prefix("texts/chant/").is_some_and(|r| r.starts_with(dir)));
    !(rel.split('/').any(|name| name.starts_with('.'))
        || rel.starts_with("review/")
        || booklet
        || rel.ends_with(".md")
        || rel == "audit-ok.txt")
}

/// The file as the apps carry it. Every `.txt` parser skips a line whose first non-space
/// character is `#`, so those lines (source notes, scaffold catalogs) are left out: they are over
/// a quarter of the data, and a verification that only touches a comment then leaves the apps'
/// data as it was. Every other byte is kept.
pub fn packed(rel: &str, bytes: Vec<u8>) -> Vec<u8> {
    let text = match std::str::from_utf8(&bytes) {
        Ok(text) if rel.ends_with(".txt") => text,
        _ => return bytes,
    };
    text.split_inclusive('\n').filter(|line| !line.trim_start().starts_with('#')).collect::<String>().into_bytes()
}
