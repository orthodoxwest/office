use calendar::{Category, Rank, Season};
use liturgy::{ElementType, OfficeElement};

use crate::commemoration::add_commemorations;
use crate::conclusion::apply_conclusion;
use crate::engine::resolve_hour_element;
use crate::hourdef::{HourElement, HourSection};
use crate::major::collect_follows;
use crate::testutil::{arc, celebrating, date, day, feast, live_texts, texts};
use crate::texts::OfficeTexts;

fn conclusion_corpus() -> OfficeTexts {
    let mut t = texts(&[
        ("shared/formulas/collect-conclusion-through", "Through Jesus Christ thy Son, our Lord.\nR. Amen."),
        ("shared/formulas/collect-conclusion-through-same", "Through the same Jesus Christ thy Son, our Lord.\nR. Amen."),
        ("shared/formulas/collect-conclusion-who-liveth", "Who with thee liveth and reigneth.\nR. Amen."),
        (
            "shared/formulas/collect-conclusion-through-spirit",
            "Through Jesus Christ thy Son, our Lord, in the unity of the same Holy Ghost.\nR. Amen.",
        ),
    ]);
    t.corpus.set_collect_conclusion_form("proper/a/collect", "through-same");
    t.corpus.set_collect_conclusion_form("proper/b/collect", "who-liveth");
    t.corpus.set_collect_conclusion_form("proper/c/collect", "through-spirit");
    t
}

#[test]
fn uses_recorded_form() {
    let t = conclusion_corpus();
    for (source, want) in [
        ("proper/a/collect", "shared/formulas/collect-conclusion-through-same"),
        ("proper/b/collect", "shared/formulas/collect-conclusion-who-liveth"),
        ("proper/c/collect", "shared/formulas/collect-conclusion-through-spirit"),
        ("proper/z/collect", "shared/formulas/collect-conclusion-through"),
        ("commons/martyr/collect", "shared/formulas/collect-conclusion-through"),
    ] {
        let (_, refs) = apply_conclusion("Body.", source, &t);
        assert_eq!(refs, [source, want], "{source}");
    }
}

#[test]
fn missing_formula_leaves_the_collect_unconcluded() {
    let mut t = texts(&[]);
    t.corpus.set_collect_conclusion_form("proper/a/collect", "nonexistent-form");
    assert_eq!(apply_conclusion("Body.", "proper/a/collect", &t), ("Body.".into(), vec!["proper/a/collect".into()]));
    assert_eq!(apply_conclusion("Body.", "proper/z/collect", &t), ("Body.".into(), vec!["proper/z/collect".into()]));
}

#[test]
fn appends_and_separates() {
    let t = conclusion_corpus();
    let (text, refs) = apply_conclusion("O God, who didst.", "proper/a/collect", &t);
    assert!(text.starts_with("O God, who didst.\nThrough the same Jesus Christ"), "{text}");
    assert_eq!(refs.len(), 2);
    let (text, refs) = apply_conclusion("[Proper text not found: collect]", "proper/a/collect", &t);
    assert!(!text.contains("Through") && refs.len() == 1);
    assert_eq!(apply_conclusion("", "proper/a/collect", &t).0, "");
    let (text, _) = apply_conclusion("O God, who didst.\n\n", "proper/z/collect", &t);
    assert!(text.contains("didst.\nThrough Jesus Christ") && !text.contains("didst.\n\n\n"), "{text:?}");
}

#[test]
fn corpus_christi_vespers_collect_inherits_conclusion() {
    let t = live_texts();
    const CONCLUSION: &str = "shared/formulas/collect-conclusion-who-livest";
    for first in [true, false] {
        let mut d = celebrating(date(2026, 6, 11), Season::Pentecost, feast("corpus-christi", Some(Category::Lord)));
        d.first_vespers = first;
        let collect = resolve_hour_element(&d, "vespers", &HourElement::new("proper-collect", "collect"), t);
        assert_eq!(collect.source_ref, "proper/corpus-christi/collect-vespers", "first={first}");
        assert!(collect.text.ends_with(&format!("\n{}", t.get(CONCLUSION))), "first={first}");
        assert!(collect.source_refs.iter().any(|r| r == CONCLUSION), "first={first}");
    }
}

fn sequencing_corpus() -> OfficeTexts {
    texts(&[
        ("shared/formulas/collect-conclusion-through", "CONCLUSION.\nR. Amen."),
        ("commons/confessor/benedictus-antiphon", "Ant. N."),
        ("commons/confessor/versicle-lauds", "V. N.\nR. N."),
        ("commons/confessor/collect", "Collect of N."),
    ])
}

fn commemorated_day(n: usize) -> crate::Day {
    let mut d = day(date(2026, 7, 26), Season::Lent);
    d.commemorations = (0..n)
        .map(|i| {
            let mut f = feast(&format!("comm-{i}"), Some(Category::Confessor));
            f.name = format!("Commemorated Saint {i}");
            f.rank = Rank::Simple;
            f.proper_name = Some(format!("Saint{i}"));
            arc(f)
        })
        .collect();
    d
}

fn collects(elems: &[OfficeElement]) -> Vec<&OfficeElement> {
    elems.iter().filter(|e| e.kind == ElementType::Collect).collect()
}

#[test]
fn commemorations_conclude_only_the_last() {
    let t = sequencing_corpus();
    for n in [1, 2, 3, 5] {
        let elems = add_commemorations(&commemorated_day(n), "lauds", &t, false);
        let got = collects(&elems);
        assert_eq!(got.len(), n);
        for (i, c) in got.iter().enumerate() {
            let last = i == n - 1;
            assert_eq!(c.text.contains("CONCLUSION."), last, "collect {i} of {n}: {:?}", c.text);
            assert_eq!(c.source_refs.iter().any(|r| r.starts_with("shared/formulas/collect-conclusion-")), last);
        }
        // A following Suffrage or Cross carries the run's last conclusion.
        let elems = add_commemorations(&commemorated_day(n), "lauds", &t, true);
        assert!(collects(&elems).iter().all(|c| !c.text.contains("CONCLUSION.")));
    }
    assert!(add_commemorations(&commemorated_day(0), "lauds", &t, false).is_empty());
}

#[test]
fn collect_follows_table() {
    let section = |name: &str, kinds: &[&str]| {
        let mut s = HourSection::new(name);
        s.elements = kinds.iter().map(|k| HourElement::new(k, "")).collect();
        s
    };
    let sections = [
        section("Collect", &["proper-collect"]),
        section("Commemorations", &["commemorations"]),
        section("Suffrage", &["antiphon", "versicle", "collect"]),
        section("CrossCommemoration", &["antiphon", "versicle", "collect"]),
        section("Closing", &["blessing", "versicle"]),
        section("Marian", &["marian", "collect"]),
    ];
    for (included, want) in [
        ([true, true, true, false, true, true], true),
        ([true, true, false, true, true, true], true),
        ([true, true, false, false, true, true], false),
    ] {
        assert_eq!(collect_follows(&sections, &included, 1), want, "{included:?}");
    }
}
