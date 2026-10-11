//! The zero-occurrence ledger: durable judgments on corpus entries a sweep never selects.

use std::collections::BTreeMap;

use calendar::DataSource;
use data_format::quote;

use super::provenance::{ProvenanceInventory, read_ledger};

pub const ZERO_OCCURRENCE_FILE: &str = "review/zero-occurrences.csv";
const ZERO_OCCURRENCE_HEADER: [&str; 5] = ["key", "content_hash", "disposition", "reason", "issue"];

/// Why an entry is intentionally absent from a sweep. `Unclassified` keeps
/// uncertainty visible instead of guessing.
#[derive(Clone, Copy, Debug, PartialEq, Eq, Hash, PartialOrd, Ord)]
pub enum ZeroDisposition {
    ShadowedFallback,
    Displaced,
    DormantPolicy,
    Suppressed,
    Dead,
    Defect,
    Unclassified,
}

impl ZeroDisposition {
    pub const ALL: [ZeroDisposition; 7] = [
        ZeroDisposition::ShadowedFallback,
        ZeroDisposition::Displaced,
        ZeroDisposition::DormantPolicy,
        ZeroDisposition::Suppressed,
        ZeroDisposition::Dead,
        ZeroDisposition::Defect,
        ZeroDisposition::Unclassified,
    ];

    pub fn as_str(self) -> &'static str {
        match self {
            ZeroDisposition::ShadowedFallback => "shadowed-fallback",
            ZeroDisposition::Displaced => "displaced",
            ZeroDisposition::DormantPolicy => "dormant-policy",
            ZeroDisposition::Suppressed => "suppressed",
            ZeroDisposition::Dead => "dead",
            ZeroDisposition::Defect => "defect",
            ZeroDisposition::Unclassified => "unclassified",
        }
    }

    pub fn parse(s: &str) -> Option<ZeroDisposition> {
        ZeroDisposition::ALL.into_iter().find(|d| d.as_str() == s)
    }
}

/// One judgment bound to the content it was made on; a stale one stays
/// reportable but no longer applies.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct ZeroClassification {
    pub key: String,
    pub content_hash: String,
    pub disposition: ZeroDisposition,
    pub reason: String,
    pub issue: String,
    pub stale: bool,
}

impl ZeroClassification {
    pub fn current(&self) -> bool {
        !self.stale
    }

    pub fn classified(&self) -> bool {
        self.current() && self.disposition != ZeroDisposition::Unclassified
    }
}

/// Reads and validates the ledger. A missing file is an empty ledger; edits
/// to an entry's text mark its row stale.
pub fn load_zero_classifications(
    src: &dyn DataSource,
    inventory: &ProvenanceInventory,
) -> Result<BTreeMap<String, ZeroClassification>, String> {
    let path = src.display_path(ZERO_OCCURRENCE_FILE);
    let Some(rows) = read_ledger(src, ZERO_OCCURRENCE_FILE)? else { return Ok(BTreeMap::new()) };
    let by_key = inventory.by_key();
    let mut out = BTreeMap::new();
    let mut seen: BTreeMap<String, usize> = BTreeMap::new();
    for (i, row) in rows.into_iter().enumerate() {
        let n = i + 1;
        if i == 0 {
            if row != ZERO_OCCURRENCE_HEADER {
                return Err(format!("{path}: unexpected header"));
            }
            continue;
        }
        if row.first().is_none_or(|k| k.trim().is_empty()) {
            continue;
        }
        if row.len() != ZERO_OCCURRENCE_HEADER.len() {
            return Err(format!("{path} row {n}: want {} columns, got {}", ZERO_OCCURRENCE_HEADER.len(), row.len()));
        }
        let [key, content_hash, disposition, reason, issue]: [String; 5] = row.try_into().expect("five columns");
        let key_q = quote(&key);
        if key.is_empty() || content_hash.is_empty() || reason.is_empty() {
            return Err(format!("{path} row {n}: classification for {key_q} needs key, content_hash, and reason"));
        }
        let Some(disposition) = ZeroDisposition::parse(&disposition) else {
            return Err(format!("{path} row {n}: classification for {key_q} has invalid disposition {}", quote(&disposition)));
        };
        if disposition == ZeroDisposition::Defect && issue.trim().is_empty() {
            return Err(format!("{path} row {n}: defect classification for {key_q} needs an issue reference"));
        }
        for (field, value) in [("key", &key), ("reason", &reason), ("issue", &issue)] {
            if value.contains(['\r', '\n']) {
                return Err(format!("{path} row {n}: {field} of classification for {key_q} may not contain a newline"));
            }
        }
        if let Some(previous) = seen.get(&key) {
            return Err(format!("{path} row {n}: duplicate classification for {key_q} (also row {previous})"));
        }
        seen.insert(key.clone(), n);
        let Some(entry) = by_key.get(key.as_str()) else {
            return Err(format!("zero-occurrence classification references unknown corpus key {key_q}"));
        };
        let stale = content_hash != entry.content_hash;
        out.insert(key.clone(), ZeroClassification { key, content_hash, disposition, reason, issue, stale });
    }
    Ok(out)
}

/// A mechanical key-shape category for organizing manual classification.
#[derive(Clone, Copy, Debug, PartialEq, Eq, Hash, PartialOrd, Ord)]
pub enum ZeroHeuristic {
    GenericPsalmAntiphon,
    IndexedPsalmAntiphon,
    CommemorationSlot,
    WeekdayVariant,
    FirstVespersVariant,
    Other,
}

impl ZeroHeuristic {
    /// In rank order.
    pub const ALL: [ZeroHeuristic; 6] = [
        ZeroHeuristic::GenericPsalmAntiphon,
        ZeroHeuristic::IndexedPsalmAntiphon,
        ZeroHeuristic::CommemorationSlot,
        ZeroHeuristic::WeekdayVariant,
        ZeroHeuristic::FirstVespersVariant,
        ZeroHeuristic::Other,
    ];

    pub fn as_str(self) -> &'static str {
        match self {
            ZeroHeuristic::GenericPsalmAntiphon => "generic-psalm-antiphon",
            ZeroHeuristic::IndexedPsalmAntiphon => "indexed-psalm-antiphon",
            ZeroHeuristic::CommemorationSlot => "commemoration-slot",
            ZeroHeuristic::WeekdayVariant => "weekday-variant",
            ZeroHeuristic::FirstVespersVariant => "first-vespers-variant",
            ZeroHeuristic::Other => "other",
        }
    }

    pub fn detect(key: &str) -> ZeroHeuristic {
        let section = key.rsplit('/').next().unwrap_or(key);
        let indexed = section.strip_prefix("psalm-antiphon-").is_some_and(|n| !n.is_empty() && n.bytes().all(|c| c.is_ascii_digit()));
        let weekday =
            ["monday", "tuesday", "wednesday", "thursday", "friday", "saturday"].iter().any(|w| section.ends_with(&format!("-{w}")));
        if section == "psalm-antiphon" {
            ZeroHeuristic::GenericPsalmAntiphon
        } else if indexed {
            ZeroHeuristic::IndexedPsalmAntiphon
        } else if section.starts_with("commemoration-") {
            ZeroHeuristic::CommemorationSlot
        } else if weekday {
            ZeroHeuristic::WeekdayVariant
        } else if section.contains("first-vespers") || section.ends_with("-first") {
            ZeroHeuristic::FirstVespersVariant
        } else {
            ZeroHeuristic::Other
        }
    }
}
