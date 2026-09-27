//! Deliberately broken corpora protect validation diagnostics and boundaries.

use std::path::Path;
use std::sync::LazyLock;

use regex::Regex;

use tools::fs::FsData;
use tools::review::{provenance, zero_occurrence};
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
            Ok(inventory) => match zero_occurrence::load_zero_classifications(&src, &inventory) {
                Err(e) => vec![format!("review zero occurrences: {e}")],
                Ok(_) => Vec::new(),
            },
        };
        layer("review", errs);
    }
    out
}

#[test]
fn broken_corpora_match_go() {
    let mut names: Vec<String> = std::fs::read_dir(CASES)
        .unwrap()
        .map(|e| e.unwrap())
        .filter(|e| e.path().is_dir())
        .map(|e| e.file_name().into_string().unwrap())
        .collect();
    names.sort();
    assert!(names.len() >= 15, "found {} cases", names.len());
    let mut failures = Vec::new();
    for name in &names {
        let dir = format!("{CASES}/{name}");
        let want = std::fs::read_to_string(format!("{dir}/expected.txt")).unwrap();
        let got = report(&dir);
        if !same_findings(&got, &want) {
            failures.push(format!("{name}\n--- got\n{got}--- want\n{want}"));
        }
    }
    assert!(failures.is_empty(), "{}", failures.join("\n"));
}

/// The shapes of Go library error text that can end a finding.
static GO_LIBRARY_DETAIL: LazyLock<Regex> =
    LazyLock::new(|| Regex::new(r"^(strconv\.\w+: |open \S+: |record on line \d+|parse error on line \d+)").expect("valid regex"));

/// Whether `got` states `want`'s findings: every line identical, except
/// that a Go library error tail after a shared `…: ` prefix may be worded
/// differently (but not left out).
fn same_findings(got: &str, want: &str) -> bool {
    let (got, want): (Vec<&str>, Vec<&str>) = (got.lines().collect(), want.lines().collect());
    got.len() == want.len() && got.iter().zip(&want).all(|(g, w)| g == w || same_prefix_before_library_detail(g, w))
}

fn same_prefix_before_library_detail(got: &str, want: &str) -> bool {
    want.match_indices(": ").any(|(i, _)| {
        let prefix = &want[..i + 2];
        GO_LIBRARY_DETAIL.is_match(&want[i + 2..]) && got.len() > prefix.len() && got.starts_with(prefix)
    })
}

#[test]
fn library_detail_tolerance_is_narrow() {
    let want = "x.txt: feast \"m\": invalid Month: strconv.Atoi: parsing \"March\": invalid syntax";
    assert!(same_findings("x.txt: feast \"m\": invalid Month: \"March\" is not an integer", want));
    // The finding itself must still match, and the detail must be present.
    assert!(!same_findings("x.txt: feast \"m\": invalid Day: \"March\" is not an integer", want));
    assert!(!same_findings("x.txt: feast \"m\": invalid Month: ", want));
    // Our own wording is never tolerated.
    assert!(!same_findings("Duplicate feast ID: 'st-y'", "Duplicate feast ID: 'st-x'"));
}

#[test]
fn live_data_is_valid() {
    let src = FsData::new("../../data");
    assert_eq!(validate_calendar(&src), Vec::<String>::new());
    assert_eq!(validate_texts(&src), Vec::<String>::new());
    assert_eq!(office::validate::validate_hour_definitions(&src), Vec::<String>::new());
    let inventory = provenance::scan_provenance(&src).unwrap();
    zero_occurrence::load_zero_classifications(&src, &inventory).unwrap();
}

#[test]
fn assumption_week_commemoration_routing() {
    let texts = office::texts::load_texts(&FsData::new("../../data")).unwrap();
    for (reference, want) in [
        ("proper/st-eusebius-august/commemoration-antiphon-vespers", "I will liken him"),
        ("proper/st-helen/commemoration-antiphon-vespers", "The kingdom of heaven"),
        ("proper/st-helen/commemoration-antiphon-lauds", "Give her"),
        ("proper/st-agapitus/commemoration-antiphon-vespers", "This is a Martyr"),
        ("proper/st-agapitus/commemoration-antiphon-lauds", "The very hairs"),
    ] {
        assert!(texts.corpus.get(reference).starts_with(want), "{reference} = {:?}", texts.corpus.get(reference));
    }
    assert!(texts.scopes.is_some_and(|s| !s.list().is_empty()));
}
