use std::path::Path;

use calendar::DataSource;
use tools::fs::FsData;

use super::*;

fn repo_data() -> FsData {
    FsData::new(Path::new(env!("CARGO_MANIFEST_DIR")).join("../../data"))
}

#[test]
fn the_embedded_corpus_is_the_data_directory() {
    let fs = repo_data();
    for dir in ["texts", "feasts", "office"] {
        let embedded =
            |rel: &str| !(rel.starts_with("chant/") || rel.ends_with(".md") || rel.rsplit('/').next().is_some_and(|n| n.starts_with('.')));
        let want: Vec<_> = fs.walk(dir).unwrap().into_iter().filter(|(rel, _)| embedded(rel)).collect();
        let got = EmbeddedData.walk(dir).unwrap();
        let paths = |files: &[(String, Vec<u8>)]| files.iter().map(|(rel, _)| rel.clone()).collect::<Vec<_>>();
        assert_eq!(paths(&got), paths(&want), "{dir}");
        assert!(got == want, "{dir}: contents differ");
    }
    for file in ["appointment-scopes.json", "collect-conclusions.txt", "latin-incipits.txt", "penitential.txt"] {
        assert_eq!(EmbeddedData.read(file).unwrap(), fs.read(file).unwrap(), "{file}");
    }
    assert_eq!(EmbeddedData.read("review/provenance.csv").unwrap(), None);
}

#[test]
fn composes_what_the_web_composes() {
    let core = OfficeCore::new().expect("embedded engine");
    let fs = repo_data();
    let engine = Engine::load(&fs).unwrap();
    let days = tools::year::load_office_days(&fs, 2026).unwrap();
    let moveable = MoveableDates::compute(2026);
    for (month, day) in [(1, 6), (4, 3), (4, 5), (9, 30), (12, 25)] {
        for hour in hour_names() {
            let view = core.compose(hour.clone(), 2026, month, day, "private".into()).unwrap();
            let date = Date::new(2026, month, day);
            let want = engine.compose_hour(&hour, &days[date.ordinal() as usize - 1], &moveable, PrayerForm::Private).unwrap();
            let want_sections: Vec<SectionView> = render_blocks::hour_sections(&want).into_iter().map(SectionView::from).collect();
            assert_eq!(format!("{:?}", view.sections), format!("{want_sections:?}"), "{hour} 2026-{month}-{day}");
            assert_eq!(view.title, want.title);
        }
    }
}

#[test]
fn describes_the_day() {
    let core = OfficeCore::new().unwrap();
    let view = core.compose("vespers".into(), 2026, 12, 25, "priest".into()).unwrap();
    assert_eq!(view.date_label, "Friday, December 25, 2026");
    assert_eq!(view.color, "white");
    assert!(!view.feast.is_empty());
}

#[test]
fn rejects_bad_requests() {
    let core = OfficeCore::new().unwrap();
    assert!(core.compose("matins".into(), 2026, 1, 1, "private".into()).is_err());
    assert!(core.compose("lauds".into(), 2026, 2, 30, "private".into()).is_err());
    assert!(core.compose("lauds".into(), 2026, 1, 1, "bishop".into()).is_err());
}

#[test]
fn current_office_follows_the_web_schedule() {
    let at = |h| {
        let c = current_office(h);
        (c.hour, c.day_offset)
    };
    assert_eq!(at(0), ("compline".to_string(), -1));
    assert_eq!(at(1), ("compline".to_string(), -1));
    assert_eq!(at(2), ("lauds".to_string(), 0));
    assert_eq!(at(12), ("sext".to_string(), 0));
    assert_eq!(at(19), ("vespers".to_string(), 0));
    assert_eq!(at(23), ("compline".to_string(), 0));
}
