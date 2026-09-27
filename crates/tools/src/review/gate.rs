//! The text-provenance assurance report, its reviewable baseline, and the
//! CI gate. Ported from Go's `review/assurance_gate.go`.

use std::collections::HashSet;
use std::fmt::Write as _;
use std::path::Path;

use calendar::{CalendarData, DataSource};
use office::engine::Engine;

use super::assurance::hour_dependencies;
use super::provenance::{ProvenanceStatus, scan_provenance};
use super::sweep::{Forms, sweep};
use super::zero_occurrence::load_zero_classifications;

pub const ASSURANCE_BASELINE_FILE: &str = "review/assurance-baseline.json";

/// The intentional verified-text floor.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct AssuranceBaseline {
    pub start_year: i32,
    pub years: i32,
    pub verified_minimum: i64,
}

/// Source-content-free release facts.
#[derive(Clone, Debug, Default, PartialEq, Eq)]
pub struct AssuranceReport {
    pub start_year: i32,
    pub years: i32,
    pub candidate_count: usize,
    pub verified: i64,
    pub needs_review: usize,
    pub source_unknown: usize,
    pub classified_zeroes: usize,
    pub unclassified_zeroes: usize,
    pub stale_zero_classes: usize,
    pub stale_attestations: usize,
}

pub fn build_assurance_report(src: &dyn DataSource, start: i32, years: i32) -> Result<AssuranceReport, String> {
    let (rendered, count) = rendered_dependencies(src, start, years)?;
    let provenance = scan_provenance(src)?;
    let zero = load_zero_classifications(src, &provenance)?;
    let mut r = AssuranceReport { start_year: start, years, candidate_count: count, ..AssuranceReport::default() };
    for entry in &provenance.entries {
        if entry.status == ProvenanceStatus::Verified {
            r.verified += 1;
        } else if rendered.contains(&entry.key) {
            if entry.status == ProvenanceStatus::NeedsReview {
                r.needs_review += 1;
            } else {
                r.source_unknown += 1;
            }
        } else {
            match zero.get(&entry.key) {
                Some(z) if z.classified() => r.classified_zeroes += 1,
                z => {
                    r.unclassified_zeroes += 1;
                    if z.is_some_and(|z| z.stale) {
                        r.stale_zero_classes += 1;
                    }
                }
            }
        }
        if entry.stale {
            r.stale_attestations += 1;
        }
    }
    Ok(r)
}

/// Every key any composed date-hour form renders, and how many forms.
fn rendered_dependencies(src: &dyn DataSource, start: i32, years: i32) -> Result<(HashSet<String>, usize), String> {
    if years < 1 {
        return Err("years must be at least 1".into());
    }
    let engine = Engine::load(src)?;
    let cal = CalendarData::load(src)?;
    let mut rendered = HashSet::new();
    let mut count = 0;
    let mut fold = |deps: Vec<String>| {
        count += 1;
        rendered.extend(deps);
    };
    sweep(&engine, &cal, start, years, Forms::All, &|_, e| e, &|_, _, hour| hour_dependencies(hour), &mut fold)?;
    Ok((rendered, count))
}

/// Reads the floor. Go decodes it with `encoding/json`; the file is a flat
/// object of three integers.
pub fn load_assurance_baseline(src: &dyn DataSource) -> Result<AssuranceBaseline, String> {
    let path = src.display_path(ASSURANCE_BASELINE_FILE);
    let body = src.read(ASSURANCE_BASELINE_FILE)?.ok_or_else(|| format!("{path} does not exist"))?;
    let v: serde_json::Value = serde_json::from_str(&body).map_err(|e| format!("reading {path}: {e}"))?;
    let int = |name: &str| v.get(name).and_then(serde_json::Value::as_i64).unwrap_or(0);
    let baseline =
        AssuranceBaseline { start_year: int("start_year") as i32, years: int("years") as i32, verified_minimum: int("verified_minimum") };
    if baseline.years < 1 {
        return Err(format!("{path}: years must be at least 1"));
    }
    Ok(baseline)
}

/// Gate failures: only a drop below the verified floor fails.
pub fn evaluate_assurance(r: &AssuranceReport, b: &AssuranceBaseline) -> Vec<String> {
    if r.verified < b.verified_minimum {
        return vec![format!("verified provenance decreased: got {}, baseline requires {}", r.verified, b.verified_minimum)];
    }
    Vec::new()
}

/// Plain text or a Markdown CI summary.
pub fn assurance_summary(r: &AssuranceReport, failures: &[String], markdown: bool) -> String {
    let mut w = String::new();
    let end = r.start_year + r.years - 1;
    if markdown {
        w.push_str("## Text provenance assurance\n\n");
        w.push_str("Source verification does not establish correct appointments or complete office structure.\n\n");
        w.push_str("| Measure | Count |\n|---|---:|\n");
        let _ = writeln!(w, "| Distinct date-hour forms ({}–{end}) | {} |", r.start_year, r.candidate_count);
        let _ = writeln!(w, "| Verified text entries | {} |", r.verified);
        let _ = writeln!(w, "| Rendered text entries needing review | {} |", r.needs_review);
        let _ = writeln!(w, "| Rendered text entries with unknown source | {} |", r.source_unknown);
        let _ = writeln!(w, "| Classified zero-occurrence entries | {} |", r.classified_zeroes);
        let _ = writeln!(w, "| Zeroes needing classification | {} |", r.unclassified_zeroes);
        let _ = writeln!(w, "| Stale zero-occurrence classifications | {} |", r.stale_zero_classes);
        let _ = writeln!(w, "| Stale attestations | {} |", r.stale_attestations);
    } else {
        let _ = writeln!(w, "=== Text provenance assurance: {}-{end} ===", r.start_year);
        w.push_str("Source verification does not establish correct appointments or complete office structure.\n");
        let _ = writeln!(w, "  distinct date-hour forms: {}", r.candidate_count);
        let _ = writeln!(w, "  verified:             {}", r.verified);
        let _ = writeln!(w, "  rendered needs review:{:5}", r.needs_review);
        let _ = writeln!(w, "  rendered unknown:     {}", r.source_unknown);
        let _ = writeln!(w, "  classified zeroes:    {}", r.classified_zeroes);
        let _ = writeln!(w, "  unclassified zeroes:  {}", r.unclassified_zeroes);
        let _ = writeln!(w, "  stale zero classes:   {}", r.stale_zero_classes);
        let _ = writeln!(w, "  stale attestations:   {}", r.stale_attestations);
    }
    if !failures.is_empty() {
        if markdown {
            w.push_str("\n### Gate failures\n");
        }
        for f in failures {
            let _ = writeln!(w, "- {f}");
        }
    }
    w
}

/// Resets the floor to the current report, as Go's `json.MarshalIndent`
/// writes it.
pub fn update_assurance_baseline(dir: &Path, r: &AssuranceReport) -> Result<(), String> {
    let body = format!("{{\n  \"start_year\": {},\n  \"years\": {},\n  \"verified_minimum\": {}\n}}\n", r.start_year, r.years, r.verified);
    let dir = dir.join("review");
    if !dir.is_dir() {
        return Err(format!("{} does not exist", dir.display()));
    }
    crate::fs::write_atomic(&dir.join("assurance-baseline.json"), body.as_bytes())
}
