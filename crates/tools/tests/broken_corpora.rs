//! Deliberately broken corpora protect validation diagnostics and boundaries.

use std::path::Path;
use tools::fs::FsData;
use tools::review::provenance;
use tools::validate::{validate_calendar, validate_texts};

const CASES: &str = "../../tests/fixtures/broken-corpora";

fn report(dir: &str) -> String {
    let src = FsData::new(dir);
    let mut out = String::new();
    let mut layer = |name: &str, errs: Vec<String>| {
        out.push_str(&format!("== {name}\n"));
        for e in errs {
            out.push_str(&e.replace(dir, "$DATA"));
            out.push('\n');
        }
    };
    if Path::new(dir).join("feasts").is_dir() {
        layer("calendar", validate_calendar(&src));
    }
    if Path::new(dir).join("texts").is_dir() {
        layer("texts", validate_texts(&src));
    }
    if Path::new(dir).join("office").is_dir() {
        layer("office", office::validate::validate_hour_definitions(&src));
    }
    if Path::new(dir).join("review").is_dir() {
        let errs = match provenance::scan_provenance(&src) {
            Err(e) => vec![format!("review provenance: {e}")],
            Ok(_) => Vec::new(),
        };
        layer("review", errs);
    }
    out
}

#[test]
fn broken_corpora_match_diagnostics() {
    let mut names: Vec<String> = std::fs::read_dir(CASES)
        .unwrap()
        .map(|e| e.unwrap())
        .filter(|e| e.path().is_dir())
        .map(|e| e.file_name().into_string().unwrap())
        .collect();
    names.sort();
    assert!(names.len() >= 14, "found {} cases", names.len());
    let mut failures = Vec::new();
    for name in &names {
        let dir = format!("{CASES}/{name}");
        let want = std::fs::read_to_string(format!("{dir}/expected.txt")).unwrap();
        let got = report(&dir);
        if got != want {
            failures.push(format!("{name}\n--- got\n{got}--- want\n{want}"));
        }
    }
    assert!(failures.is_empty(), "{}", failures.join("\n"));
}

#[test]
fn appended_dead_office_keeps_its_resolution_boundary_without_a_rubric() {
    for date in [calendar::Date::new(2026, 11, 1), calendar::Date::new(2025, 11, 2)] {
        let explanation =
            tools::review::explain::explain_composition(&FsData::new("../../data"), "vespers", date, liturgy::PrayerForm::Private).unwrap();
        let json: serde_json::Value = serde_json::from_str(&explanation).unwrap();
        let resolutions = json.get("resolutions").unwrap().as_array().unwrap();
        let mut dead_count = 0;
        let mut principal_count = 0;
        for row in resolutions {
            if row.get("selected_ref").and_then(|v| v.as_str()).unwrap_or("").starts_with("proper/all-souls/") {
                dead_count += 1;
                assert_eq!(row.get("canonical_owner").and_then(|v| v.as_str()), Some("all-souls"));
                assert_eq!(row.get("selected_tier").and_then(|v| v.as_str()), Some("proper"));
            } else if row.get("canonical_owner").and_then(|v| v.as_str()).is_some_and(|owner| !owner.is_empty() && owner != "all-souls") {
                principal_count += 1;
            }
        }
        assert!(dead_count > 0);
        assert!(principal_count > 0);
    }
}
