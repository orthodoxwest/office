//! The Phase 2 gate for validation: the Rust validators must write Go's
//! report for every case in internal/e2e/testdata/broken-corpora (see
//! internal/e2e/broken_corpora_test.go, which checks Go against the same
//! expected.txt files).

use std::path::Path;

use tools::fs::FsData;
use tools::validate::{validate_calendar, validate_texts};

const CASES: &str = "../../internal/e2e/testdata/broken-corpora";

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
    assert!(names.len() >= 10, "found {} cases", names.len());
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
fn live_data_is_valid() {
    let src = FsData::new("../../data");
    assert_eq!(validate_calendar(&src), Vec::<String>::new());
    assert_eq!(validate_texts(&src), Vec::<String>::new());
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
