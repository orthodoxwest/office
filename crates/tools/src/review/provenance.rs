//! Structured per-entry source inventory and attestations. Ported from Go's
//! `review/provenance.go`.

use std::collections::BTreeMap;
use std::fmt::Write as _;
use std::sync::LazyLock;

use calendar::{DataSource, Date};
use compat::{csv, quote, scan_lines};
use regex::Regex;
use sha2::{Digest, Sha256};

pub const PROVENANCE_FILE: &str = "review/provenance.csv";
pub const PROVENANCE_HEADER: [&str; 9] = ["key", "content_hash", "source", "locator", "page", "status", "reviewer", "reviewed_on", "notes"];

/// The strongest available evidence for a corpus entry. Only an explicit
/// attestation produces `Verified`.
#[derive(Clone, Copy, Debug, PartialEq, Eq, Hash, PartialOrd, Ord)]
pub enum ProvenanceStatus {
    SourceUnknown,
    NeedsReview,
    Verified,
}

impl ProvenanceStatus {
    pub fn as_str(self) -> &'static str {
        match self {
            ProvenanceStatus::SourceUnknown => "source-unknown",
            ProvenanceStatus::NeedsReview => "needs-review",
            ProvenanceStatus::Verified => "verified",
        }
    }

    /// Maps the ledger spelling, retired states included.
    fn normalized(status: &str) -> ProvenanceStatus {
        match status {
            "verified" => ProvenanceStatus::Verified,
            "needs-review" | "documented" => ProvenanceStatus::NeedsReview,
            _ => ProvenanceStatus::SourceUnknown,
        }
    }
}

/// One source claim attached to a corpus entry.
#[derive(Clone, Debug, Default, PartialEq, Eq)]
pub struct SourceCitation {
    pub kind: String,
    pub source: String,
    pub locator: String,
    pub page: String,
    pub note: String,
    pub line: usize,
}

/// The generated assurance record for one corpus key.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct EntryProvenance {
    pub key: String,
    pub file: String,
    pub section: String,
    pub line: usize,
    pub content_hash: String,
    pub status: ProvenanceStatus,
    pub reviewer: String,
    pub reviewed_on: String,
    pub notes: String,
    pub stale: bool,
    pub sources: Vec<SourceCitation>,
    pub todos: Vec<String>,
}

/// A sorted snapshot of corpus provenance.
#[derive(Clone, Debug, Default)]
pub struct ProvenanceInventory {
    pub entries: Vec<EntryProvenance>,
}

impl ProvenanceInventory {
    pub fn by_key(&self) -> BTreeMap<&str, &EntryProvenance> {
        self.entries.iter().map(|e| (e.key.as_str(), e)).collect()
    }
}

/// One row of `data/review/provenance.csv`.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Attestation {
    pub key: String,
    pub content_hash: String,
    pub source: String,
    pub locator: String,
    pub page: String,
    pub status: String,
    pub reviewer: String,
    pub reviewed_on: String,
    pub notes: String,
}

impl Attestation {
    pub fn row(&self) -> [&str; 9] {
        [
            &self.key,
            &self.content_hash,
            &self.source,
            &self.locator,
            &self.page,
            &self.status,
            &self.reviewer,
            &self.reviewed_on,
            &self.notes,
        ]
    }
}

// Go's \b and \s are ASCII.
static PAGE_RE: LazyLock<Regex> =
    LazyLock::new(|| Regex::new(r"(?i)(?-u:\b)p(?:age)?\.?[\t\n\f\r ]*([0-9]+(?:[-–][0-9]+)?)").expect("valid regex"));
static PDF_RE: LazyLock<Regex> = LazyLock::new(|| Regex::new(r"^([^\t\n\f\r ]+\.pdf)[\t\n\f\r ]*(.*)$").expect("valid regex"));

/// The first 12 hex digits of the body's SHA-256.
pub fn content_hash(body: &str) -> String {
    let sum = Sha256::digest(body.as_bytes());
    let mut hex = String::with_capacity(12);
    for b in &sum[..6] {
        let _ = write!(hex, "{b:02x}");
    }
    hex
}

/// Combines the `# SOURCE:` comments beside corpus sections with the
/// attestations in `data/review/provenance.csv`.
pub fn scan_provenance(src: &dyn DataSource) -> Result<ProvenanceInventory, String> {
    let texts = office::texts::load_texts(src)?;
    let mut entries: BTreeMap<String, EntryProvenance> = texts
        .entries()
        .into_iter()
        .map(|(key, body)| {
            (
                key.to_string(),
                EntryProvenance {
                    key: key.to_string(),
                    file: String::new(),
                    section: String::new(),
                    line: 0,
                    content_hash: content_hash(body),
                    status: ProvenanceStatus::SourceUnknown,
                    reviewer: String::new(),
                    reviewed_on: String::new(),
                    notes: String::new(),
                    stale: false,
                    sources: Vec::new(),
                    todos: Vec::new(),
                },
            )
        })
        .collect();

    for (rel, bytes) in src.walk("texts").map_err(|e| format!("scanning text provenance: {e}"))? {
        if !rel.rsplit('/').next().is_some_and(|n| n.ends_with(".txt")) {
            continue;
        }
        scan_provenance_file(&rel, &String::from_utf8_lossy(&bytes), &mut entries);
    }
    for e in entries.values_mut() {
        e.status = if e.sources.is_empty() && e.todos.is_empty() { ProvenanceStatus::SourceUnknown } else { ProvenanceStatus::NeedsReview };
    }

    for a in load_attestations(src)? {
        let e = entries.get_mut(&a.key).ok_or_else(|| format!("provenance attestation references unknown corpus key {}", quote(&a.key)))?;
        if !a.source.is_empty() {
            e.sources.push(SourceCitation {
                kind: "attestation".into(),
                source: a.source.clone(),
                locator: a.locator.clone(),
                page: a.page.clone(),
                note: a.notes.clone(),
                line: 0,
            });
        }
        if !a.content_hash.is_empty() && a.content_hash != e.content_hash {
            e.status = ProvenanceStatus::NeedsReview;
            e.stale = true;
            e.notes = join_nonempty(&[&e.notes, &a.notes, "stale attestation for an earlier text version"]);
        } else if !a.status.is_empty() {
            e.status = ProvenanceStatus::normalized(&a.status);
            e.notes = a.notes.clone();
        }
        e.reviewer = a.reviewer;
        e.reviewed_on = a.reviewed_on;
    }
    Ok(ProvenanceInventory { entries: entries.into_values().collect() })
}

fn scan_provenance_file(rel: &str, content: &str, entries: &mut BTreeMap<String, EntryProvenance>) {
    let (dir, base) = rel.rsplit_once('/').unwrap_or((".", rel));
    let stem = base.strip_suffix(".txt").unwrap_or(base);
    let plain_key = rel.strip_suffix(".txt").unwrap_or(rel);
    let mut current_key = plain_key.to_string();
    let mut has_sections = false;
    for (i, raw) in scan_lines(content).enumerate() {
        let line_num = i + 1;
        let trimmed = raw.trim();
        if let Some(section) = provenance_section(trimmed) {
            has_sections = true;
            current_key = if dir == "." { format!("{stem}/{section}") } else { format!("{dir}/{stem}/{section}") };
            if let Some(e) = entries.get_mut(&current_key) {
                e.file = rel.to_string();
                e.section = section.to_string();
                e.line = line_num;
            }
            continue;
        }
        if let Some(after) = trimmed.strip_prefix("# SOURCE:")
            && let Some(e) = entries.get_mut(&current_key)
        {
            e.sources.push(parse_citation(after.trim(), line_num));
        }
        if trimmed.contains("TODO(diurnal)")
            && let Some(e) = entries.get_mut(&current_key)
        {
            e.todos.push(trimmed.strip_prefix('#').unwrap_or(trimmed).trim().to_string());
        }
    }
    if !has_sections && let Some(e) = entries.get_mut(plain_key) {
        e.file = rel.to_string();
        e.line = 1;
    }
}

fn provenance_section(line: &str) -> Option<&str> {
    let inner = line.strip_prefix('[')?.strip_suffix(']')?;
    (!inner.is_empty() && !inner.contains([' ', ':', '\t'])).then_some(inner)
}

fn parse_citation(raw: &str, line: usize) -> SourceCitation {
    let (cite, note) = raw.split_once(" — ").unwrap_or((raw, ""));
    let mut c = SourceCitation { source: cite.trim().to_string(), note: note.trim().to_string(), line, ..SourceCitation::default() };
    if let Some(locator) = cite.strip_prefix("divinum-officium") {
        c.kind = "divinum-officium".into();
        c.source = "divinum-officium".into();
        c.locator = locator.trim().to_string();
    } else if let Some(m) = PDF_RE.captures(cite) {
        c.kind = "local-pdf".into();
        c.source = m[1].to_string();
        c.locator = m[2].trim().to_string();
    } else {
        c.kind = "other".into();
    }
    if let Some(m) = PAGE_RE.captures(raw) {
        c.page = m[1].to_string();
    }
    c
}

/// Reads a review ledger: `Ok(None)` when absent, Go's CSV errors otherwise.
pub fn read_ledger(src: &dyn DataSource, rel: &str) -> Result<Option<Vec<Vec<String>>>, String> {
    let Some(content) = src.read(rel)? else { return Ok(None) };
    csv::read_all(&content).map(Some).map_err(|e| format!("reading {}: {e}", src.display_path(rel)))
}

pub fn load_attestations(src: &dyn DataSource) -> Result<Vec<Attestation>, String> {
    let path = src.display_path(PROVENANCE_FILE);
    let Some(rows) = read_ledger(src, PROVENANCE_FILE)? else { return Ok(Vec::new()) };
    let mut out = Vec::new();
    for (i, row) in rows.into_iter().enumerate() {
        if i == 0 {
            if row != PROVENANCE_HEADER {
                return Err(format!("{path}: unexpected header"));
            }
            continue;
        }
        if row.first().is_none_or(|k| k.trim().is_empty()) {
            continue;
        }
        if row.len() != 9 {
            return Err(format!("{path} row {}: want 9 columns, got {}", i + 1, row.len()));
        }
        let mut f = row.into_iter();
        let mut next = || f.next().unwrap_or_default();
        let a = Attestation {
            key: next(),
            content_hash: next(),
            source: next(),
            locator: next(),
            page: next(),
            status: next(),
            reviewer: next(),
            reviewed_on: next(),
            notes: next(),
        };
        validate_attestation(&a).map_err(|e| format!("{path} row {}: {e}", i + 1))?;
        out.push(a);
    }
    Ok(out)
}

fn validate_attestation(a: &Attestation) -> Result<(), String> {
    let key = quote(&a.key);
    match a.status.as_str() {
        "needs-review" | "documented" | "source-unknown" | "undocumented" => Ok(()),
        "verified" => {
            if a.content_hash.is_empty() {
                return Err(format!("verified entry {key} needs content_hash"));
            }
            if a.source.is_empty() || (a.locator.is_empty() && a.page.is_empty()) {
                return Err(format!("verified entry {key} needs a source and locator or page"));
            }
            if a.reviewer.is_empty() || a.reviewed_on.is_empty() {
                return Err(format!("verified entry {key} needs reviewer and reviewed_on"));
            }
            if Date::parse(&a.reviewed_on).is_none() {
                return Err(format!("verified entry {key} has invalid reviewed_on {}", quote(&a.reviewed_on)));
            }
            Ok(())
        }
        other => Err(format!("entry {key} has invalid status {}", quote(other))),
    }
}

/// Generated, non-stale corpus assurance counts.
pub fn provenance_summary(p: &ProvenanceInventory) -> String {
    let mut counts: BTreeMap<ProvenanceStatus, usize> = BTreeMap::new();
    let (mut with_page, mut stale) = (0, 0);
    for e in &p.entries {
        *counts.entry(e.status).or_default() += 1;
        if e.stale {
            stale += 1;
        }
        if e.sources.iter().any(|s| !s.page.is_empty()) {
            with_page += 1;
        }
    }
    let mut out = format!("=== Corpus provenance: {} entries ===\n", p.entries.len());
    for status in [ProvenanceStatus::Verified, ProvenanceStatus::NeedsReview, ProvenanceStatus::SourceUnknown] {
        let _ = writeln!(out, "  {:<14} {:5}", status.as_str(), counts.get(&status).copied().unwrap_or(0));
    }
    let _ = writeln!(out, "  {:<14} {with_page:5}", "page-located");
    let _ = writeln!(out, "  {:<14} {stale:5}", "stale");
    out
}

/// One row per entry and source claim; entries without a source still get a
/// row so missing provenance stays machine-visible.
pub fn provenance_csv(p: &ProvenanceInventory) -> String {
    let mut out = String::new();
    csv::write_record(
        &mut out,
        &[
            "key",
            "file",
            "section",
            "line",
            "status",
            "source_kind",
            "source",
            "locator",
            "page",
            "source_line",
            "reviewer",
            "reviewed_on",
            "notes",
        ],
    );
    let none = [SourceCitation::default()];
    for e in &p.entries {
        let sources: &[SourceCitation] = if e.sources.is_empty() { &none } else { &e.sources };
        for s in sources {
            let mut parts: Vec<&str> = vec![&s.note, &e.notes];
            parts.extend(e.todos.iter().map(String::as_str));
            let notes = join_nonempty(&parts);
            let source_line = if s.line > 0 { s.line.to_string() } else { String::new() };
            let line = e.line.to_string();
            csv::write_record(
                &mut out,
                &[
                    &e.key,
                    &e.file,
                    &e.section,
                    &line,
                    e.status.as_str(),
                    &s.kind,
                    &s.source,
                    &s.locator,
                    &s.page,
                    &source_line,
                    &e.reviewer,
                    &e.reviewed_on,
                    &notes,
                ],
            );
        }
    }
    out
}

pub fn join_nonempty(values: &[&str]) -> String {
    values.iter().map(|v| v.trim()).filter(|v| !v.is_empty()).collect::<Vec<_>>().join(" | ")
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn citations() {
        let c = parse_citation("divinum-officium Sancti/01-06.txt [Ant 1] — check against diurnal p. 12", 4);
        assert_eq!(
            (c.kind.as_str(), c.source.as_str(), c.locator.as_str(), c.page.as_str()),
            ("divinum-officium", "divinum-officium", "Sancti/01-06.txt [Ant 1]", "12")
        );
        assert_eq!(c.note, "check against diurnal p. 12");
        let c = parse_citation("diurnal.pdf page 311–312", 1);
        assert_eq!(
            (c.kind.as_str(), c.source.as_str(), c.locator.as_str(), c.page.as_str()),
            ("local-pdf", "diurnal.pdf", "page 311–312", "311–312")
        );
        let c = parse_citation("Monastic Diurnal, P.44", 1);
        assert_eq!((c.kind.as_str(), c.page.as_str()), ("other", "44"));
        assert_eq!(parse_citation("step12", 1).page, "");
    }

    #[test]
    fn hashes_and_sections() {
        assert_eq!(content_hash(""), "e3b0c44298fc");
        assert_eq!(provenance_section("[psalm-antiphon-1]"), Some("psalm-antiphon-1"));
        assert_eq!(provenance_section("[a b]"), None);
        assert_eq!(provenance_section("[]"), None);
        assert_eq!(join_nonempty(&[" a ", "", "b"]), "a | b");
    }
}
