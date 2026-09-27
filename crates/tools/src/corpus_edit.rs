//! `corpus show` and `corpus put`: read one corpus body, or replace or
//! activate one section in place, with its source citation. Ported from Go's
//! `cli/corpus.go`.

use std::path::{Path, PathBuf};

use compat::quote;

const NAMESPACES: [&str; 7] = ["proper", "commons", "seasonal", "ordinary", "shared", "psalms", "canticles"];

struct Location {
    path: PathBuf,
    section: String,
    plain: bool,
}

/// One line with its byte span and line ending.
struct Line<'a> {
    start: usize,
    end: usize,
    text: &'a str,
    eol: &'a str,
}

/// `^[a-z0-9][a-z0-9-]*$`
fn valid_segment(s: &str) -> bool {
    let b = s.as_bytes();
    !b.is_empty()
        && (b[0].is_ascii_lowercase() || b[0].is_ascii_digit())
        && b.iter().all(|c| c.is_ascii_lowercase() || c.is_ascii_digit() || *c == b'-')
}

/// `^\[([a-z0-9-]+)\]\s*$` on an already-trimmed line.
fn live_section(s: &str) -> Option<&str> {
    let inner = s.strip_prefix('[')?.trim_end_matches(go_space).strip_suffix(']')?;
    (!inner.is_empty() && inner.bytes().all(|c| c.is_ascii_lowercase() || c.is_ascii_digit() || c == b'-')).then_some(inner)
}

/// `^#\s*\[([a-z0-9-]+)\]\s*$`
fn comment_section(s: &str) -> Option<&str> {
    live_section(s.strip_prefix('#')?.trim_start_matches(go_space))
}

/// Go's regexp `\s`: ASCII whitespace.
fn go_space(c: char) -> bool {
    matches!(c, '\t' | '\n' | '\x0c' | '\r' | ' ')
}

fn io_err(op: &str, path: &Path, e: &std::io::Error) -> String {
    crate::fs::io_error(op, path, e)
}

fn resolve(dir: &Path, key: &str) -> Result<Location, String> {
    if key.contains('\\') || key.starts_with('/') {
        return Err(format!("invalid corpus key {}", quote(key)));
    }
    let parts: Vec<&str> = key.split('/').collect();
    if parts.len() < 2 || !NAMESPACES.contains(&parts[0]) {
        return Err(format!("unsupported corpus key {}", quote(key)));
    }
    if !parts.iter().all(|p| valid_segment(p)) {
        return Err(format!("invalid corpus key {}", quote(key)));
    }
    let texts = dir.join("texts");
    let mut plain = texts.clone();
    for p in &parts {
        plain.push(p);
    }
    let plain = PathBuf::from(format!("{}.txt", plain.display()));
    match std::fs::metadata(&plain) {
        Ok(m) if !m.is_dir() => {
            let content = std::fs::read(&plain).map_err(|e| format!("reading corpus file: {}", io_err("open", &plain, &e)))?;
            if !contains_live_section(&String::from_utf8_lossy(&content)) {
                return Ok(Location { path: plain, section: String::new(), plain: true });
            }
        }
        Ok(_) => {}
        Err(e) if e.kind() == std::io::ErrorKind::NotFound => {}
        Err(e) => return Err(format!("stat corpus file: {}", io_err("stat", &plain, &e))),
    }
    let section = parts[parts.len() - 1].to_string();
    let mut path = texts;
    for p in &parts[..parts.len() - 1] {
        path.push(p);
    }
    let path = PathBuf::from(format!("{}.txt", path.display()));
    match std::fs::metadata(&path) {
        Err(e) if e.kind() == std::io::ErrorKind::NotFound => Err(format!("corpus file for {} does not exist", quote(key))),
        Err(e) => Err(format!("stat corpus file: {}", io_err("stat", &path, &e))),
        Ok(m) if m.is_dir() => Err(format!("corpus file for {} is a directory", quote(key))),
        Ok(_) => Ok(Location { path, section, plain: false }),
    }
}

fn contains_live_section(content: &str) -> bool {
    split_lines(content).iter().any(|l| live_section(l.text.trim()).is_some())
}

fn split_lines(content: &str) -> Vec<Line<'_>> {
    let mut lines = Vec::new();
    let mut start = 0;
    while start < content.len() {
        let end = content[start..].find('\n').map_or(content.len(), |i| start + i + 1);
        let raw = &content[start..end];
        let (text, eol) = match raw.strip_suffix('\n') {
            Some(t) => match t.strip_suffix('\r') {
                Some(t) => (t, "\r\n"),
                None => (t, "\n"),
            },
            None => (raw, ""),
        };
        lines.push(Line { start, end, text, eol });
        start = end;
    }
    lines
}

fn strip_comments(body: &str) -> String {
    let kept: Vec<&str> = body.split('\n').map(|l| l.strip_suffix('\r').unwrap_or(l)).filter(|l| !l.trim().starts_with('#')).collect();
    kept.join("\n").trim().to_string()
}

/// The section's header line, where its body ends, and whether the header
/// is a commented scaffold.
fn section_bounds<'a>(content: &'a str, section: &str) -> Result<(Line<'a>, usize, bool), String> {
    let lines = split_lines(content);
    let mut found: Option<(usize, bool)> = None;
    for (i, line) in lines.iter().enumerate() {
        let trimmed = line.text.trim();
        let (name, commented) = match live_section(trimmed) {
            Some(n) => (n, false),
            None => match comment_section(trimmed) {
                Some(n) => (n, true),
                None => continue,
            },
        };
        if name != section {
            continue;
        }
        if found.is_some() {
            return Err(format!("duplicate corpus section [{section}]"));
        }
        found = Some((i, commented));
    }
    let Some((index, commented)) = found else { return Err(format!("corpus section [{section}] does not exist")) };
    let body_end = lines[index + 1..]
        .iter()
        .find(|l| {
            let t = l.text.trim();
            live_section(t).is_some() || comment_section(t).is_some()
        })
        .map_or(content.len(), |l| l.start);
    let mut lines = lines;
    Ok((lines.swap_remove(index), body_end, commented))
}

/// `corpus show KEY`: the live body, comments stripped.
pub fn read_corpus_body(dir: &Path, key: &str) -> Result<String, String> {
    let loc = resolve(dir, key)?;
    let content = std::fs::read(&loc.path).map_err(|e| format!("reading corpus file: {}", io_err("open", &loc.path, &e)))?;
    let content = String::from_utf8_lossy(&content);
    if loc.plain {
        let body = strip_comments(&content);
        if body.is_empty() {
            return Err(format!("corpus key {} has no live body", quote(key)));
        }
        return Ok(body);
    }
    let (header, body_end, commented) = section_bounds(&content, &loc.section)?;
    if commented {
        return Err(format!("corpus key {} is only a commented scaffold", quote(key)));
    }
    let body = strip_comments(&content[header.end..body_end]);
    if body.is_empty() {
        return Err(format!("corpus key {} has an empty body", quote(key)));
    }
    Ok(body)
}

fn normalize_source(source: &str) -> Result<String, String> {
    let s = source.trim();
    let s = s.strip_prefix('#').unwrap_or(s).trim();
    let s = if s.to_uppercase().starts_with("SOURCE:") { s.get("SOURCE:".len()..).unwrap_or("").trim() } else { s };
    if s.is_empty() || s.contains(['\r', '\n']) {
        return Err("source citation must be one non-empty line".into());
    }
    Ok(s.to_string())
}

fn replacement_body(body: &str, source: &str, eol: &str) -> Result<String, String> {
    let body = body.replace("\r\n", "\n");
    let body = body.trim();
    if body.is_empty() {
        return Err("replacement body must not be empty".into());
    }
    let mut lines = Vec::new();
    for line in body.split('\n') {
        let trimmed = line.trim();
        if trimmed.to_uppercase().starts_with("# SOURCE:") {
            continue;
        }
        if live_section(trimmed).is_some() {
            return Err("replacement body must not contain a corpus section header".into());
        }
        lines.push(line);
    }
    let body = lines.join("\n");
    let body = body.trim();
    if body.is_empty() {
        return Err("replacement body must contain liturgical text".into());
    }
    let source = normalize_source(source)?;
    let eol = if eol.is_empty() { "\n" } else { eol };
    Ok(format!("# SOURCE: {source}{eol}{}{eol}{eol}", body.replace('\n', eol)))
}

/// `corpus put KEY --file BODY --source SOURCE`: replaces a live section, or
/// activates a commented scaffold, keeping the file's line endings and mode.
pub fn put_corpus_body(dir: &Path, key: &str, body: &str, source: &str) -> Result<(), String> {
    let loc = resolve(dir, key)?;
    let content = std::fs::read(&loc.path).map_err(|e| format!("reading corpus file: {}", io_err("open", &loc.path, &e)))?;
    let content = String::from_utf8_lossy(&content).into_owned();
    let updated = if loc.plain {
        replacement_body(body, source, "\n")?
    } else {
        let (header, body_end, commented) = section_bounds(&content, &loc.section)?;
        let eol = if header.eol.is_empty() { "\n" } else { header.eol };
        let replacement = replacement_body(body, source, eol)?;
        let mut header_text = content[header.start..header.end].to_string();
        if commented {
            header_text = format!("[{}]{eol}", loc.section);
        } else if header.eol.is_empty() {
            header_text.push_str(eol);
        }
        format!("{}{header_text}{replacement}{}", &content[..header.start], &content[body_end..])
    };
    let meta = std::fs::metadata(&loc.path).map_err(|e| format!("stat corpus file: {}", io_err("stat", &loc.path, &e)))?;
    let parent = loc.path.parent().unwrap_or(Path::new("."));
    let tmp = parent.join(format!(".corpus-put-{}", std::process::id()));
    let result = std::fs::write(&tmp, &updated)
        .map_err(|e| format!("writing temporary corpus file: {e}"))
        .and_then(|()| std::fs::set_permissions(&tmp, meta.permissions()).map_err(|e| format!("setting temporary corpus permissions: {e}")))
        .and_then(|()| std::fs::rename(&tmp, &loc.path).map_err(|e| format!("writing corpus file: {e}")));
    if result.is_err() {
        let _ = std::fs::remove_file(&tmp);
    }
    result
}
