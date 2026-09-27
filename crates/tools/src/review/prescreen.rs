//! The prescreen ledger (read-through suspicions bound to a text version)
//! and the per-key suspicion map it forms with the advisory lints. Ported
//! from Go's `review/prescreen.go`.

use std::collections::BTreeMap;
use std::path::Path;

use calendar::DataSource;
use compat::{csv, quote};

use super::provenance::{ProvenanceInventory, ProvenanceStatus, read_ledger, scan_provenance};

pub const PRESCREEN_FILE: &str = "review/prescreen.csv";
const PRESCREEN_HEADER: [&str; 6] = ["key", "content_hash", "severity", "reason", "flagged", "issue"];

/// One durable read-through finding.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct PrescreenFlag {
    pub key: String,
    pub content_hash: String,
    /// "high" or "medium".
    pub severity: String,
    pub reason: String,
    /// The batch identifier, e.g. "2026-07".
    pub flagged: String,
    pub issue: String,
}

impl PrescreenFlag {
    fn row(&self) -> [&str; 6] {
        [&self.key, &self.content_hash, &self.severity, &self.reason, &self.flagged, &self.issue]
    }
}

/// One reason to prioritize an entry for book time.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Suspicion {
    /// "prescreen:high", "prescreen:medium", or "lint:<class>".
    pub label: String,
    /// The flagged text has changed since: the fix still needs a book check.
    pub addressed: bool,
    pub reason: String,
}

impl std::fmt::Display for Suspicion {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str(&self.label)?;
        if self.addressed {
            f.write_str(" (addressed)")?;
        }
        if !self.reason.is_empty() {
            write!(f, " — {}", self.reason)?;
        }
        Ok(())
    }
}

pub fn load_prescreen(src: &dyn DataSource) -> Result<Vec<PrescreenFlag>, String> {
    let path = src.display_path(PRESCREEN_FILE);
    let Some(rows) = read_ledger(src, PRESCREEN_FILE)? else { return Ok(Vec::new()) };
    let mut out = Vec::new();
    let mut seen: BTreeMap<String, usize> = BTreeMap::new();
    for (i, row) in rows.into_iter().enumerate() {
        let n = i + 1;
        if i == 0 {
            if row != PRESCREEN_HEADER {
                return Err(format!("{path}: unexpected header"));
            }
            continue;
        }
        if row.first().is_none_or(|k| k.trim().is_empty()) {
            continue;
        }
        if row.len() != PRESCREEN_HEADER.len() {
            return Err(format!("{path} row {n}: want {} columns, got {}", PRESCREEN_HEADER.len(), row.len()));
        }
        let [key, content_hash, severity, reason, flagged, issue]: [String; 6] = row.try_into().expect("six columns");
        let flag = PrescreenFlag { key, content_hash, severity, reason, flagged, issue };
        validate_flag(&flag).map_err(|e| format!("{path} row {n}: {e}"))?;
        if let Some(prev) = seen.get(&flag.key) {
            return Err(format!("{path} row {n}: duplicate flag for {} (also row {prev})", quote(&flag.key)));
        }
        seen.insert(flag.key.clone(), n);
        out.push(flag);
    }
    Ok(out)
}

fn validate_flag(f: &PrescreenFlag) -> Result<(), String> {
    let key = quote(&f.key);
    if f.key.is_empty() || f.content_hash.is_empty() || f.reason.is_empty() || f.flagged.is_empty() {
        return Err(format!("flag for {key} needs key, content_hash, reason, and flagged"));
    }
    if f.severity != "high" && f.severity != "medium" {
        return Err(format!("flag for {key} has invalid severity {}", quote(&f.severity)));
    }
    for (field, value) in [("key", &f.key), ("reason", &f.reason), ("flagged", &f.flagged), ("issue", &f.issue)] {
        if value.contains(['\r', '\n']) {
            return Err(format!("{field} of flag for {key} may not contain a newline"));
        }
    }
    Ok(())
}

/// Advisory lint classes that suggest the wording itself is wrong.
const SUSPECT_LINT_CLASSES: [&str; 4] = ["truncated", "unpointed-antiphon", "near-duplicate", "latin"];

/// Prescreen flags and suspect advisory lints per key. A verified entry
/// carries none; an edit since flagging marks a flag addressed.
pub fn suspicion_by_key(src: &dyn DataSource, inv: &ProvenanceInventory) -> Result<BTreeMap<String, Vec<Suspicion>>, String> {
    let flags = load_prescreen(src)?;
    let by_key = inv.by_key();
    let mut out: BTreeMap<String, Vec<Suspicion>> = BTreeMap::new();
    for f in flags {
        let entry = by_key.get(f.key.as_str()).ok_or_else(|| format!("prescreen flag references unknown corpus key {}", quote(&f.key)))?;
        if entry.status == ProvenanceStatus::Verified {
            continue;
        }
        out.entry(f.key.clone()).or_default().push(Suspicion {
            label: format!("prescreen:{}", f.severity),
            addressed: f.content_hash != entry.content_hash,
            reason: f.reason,
        });
    }
    let lints = crate::audit::lint::lint(src)?;
    for finding in lints.advisory {
        if !SUSPECT_LINT_CLASSES.contains(&finding.class) {
            continue;
        }
        let Some(entry) = by_key.get(finding.key.as_str()) else { continue };
        if entry.status == ProvenanceStatus::Verified {
            continue;
        }
        out.entry(finding.key).or_default().push(Suspicion {
            label: format!("lint:{}", finding.class),
            addressed: false,
            reason: finding.detail,
        });
    }
    for list in out.values_mut() {
        list.sort_by_key(suspicion_rank);
    }
    Ok(out)
}

fn suspicion_rank(s: &Suspicion) -> u8 {
    match s.label.as_str() {
        "prescreen:high" => 0,
        "prescreen:medium" => 1,
        _ => 2,
    }
}

/// Go's `RecordPrescreenFlag`: binds the flag to the entry's current text
/// and rewrites the ledger atomically.
pub fn record_prescreen_flag(src: &dyn DataSource, dir: &Path, mut flag: PrescreenFlag, replace: bool) -> Result<PrescreenFlag, String> {
    flag.key = flag.key.trim().to_string();
    flag.reason = flag.reason.trim().to_string();
    flag.flagged = flag.flagged.trim().to_string();
    flag.issue = flag.issue.trim().to_string();
    let inv = scan_provenance(src)?;
    let entry = inv
        .by_key()
        .get(flag.key.as_str())
        .map(|e| e.content_hash.clone())
        .ok_or_else(|| format!("unknown corpus key {}", quote(&flag.key)))?;
    flag.content_hash = entry;
    validate_flag(&flag)?;
    let existing = load_prescreen(src)?;
    let found = existing.iter().any(|f| f.key == flag.key);
    if found && !replace {
        return Err(format!("entry {} already has a prescreen flag; use --replace to replace it", quote(&flag.key)));
    }
    let mut kept: Vec<PrescreenFlag> = existing.into_iter().filter(|f| f.key != flag.key).collect();
    kept.push(flag.clone());
    kept.sort_by(|a, b| a.key.cmp(&b.key));
    write_prescreen(dir, &kept)?;
    Ok(flag)
}

/// Removes any flag for `key` once an attestation supersedes it.
pub fn prune_prescreen_flag(src: &dyn DataSource, dir: &Path, key: &str) -> Result<bool, String> {
    let flags = load_prescreen(src)?;
    let kept: Vec<PrescreenFlag> = flags.iter().filter(|f| f.key != key).cloned().collect();
    if kept.len() == flags.len() {
        return Ok(false);
    }
    write_prescreen(dir, &kept)?;
    Ok(true)
}

fn write_prescreen(dir: &Path, flags: &[PrescreenFlag]) -> Result<(), String> {
    let mut out = String::new();
    csv::write_record(&mut out, &PRESCREEN_HEADER);
    for f in flags {
        csv::write_record(&mut out, &f.row());
    }
    crate::fs::write_atomic(&dir.join(PRESCREEN_FILE), out.as_bytes())
}
