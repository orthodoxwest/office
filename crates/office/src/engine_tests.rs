use super::*;
use crate::testutil::{arc, celebrating, date, day, feast};
use calendar::{Category, Rank, Season};
use liturgy::{ElementType::*, OfficeSection};

fn hour(sections: &[&[(ElementType, &str)]]) -> OfficeHour {
    OfficeHour {
        form: PrayerForm::Private,
        date: date(2026, 3, 11),
        hour: "lauds".into(),
        title: String::new(),
        season: None,
        feast: String::new(),
        color: None,
        decisions: vec![],
        sections: sections
            .iter()
            .map(|elements| OfficeSection {
                label: String::new(),
                collapsible: false,
                elements: elements.iter().map(|(kind, text)| OfficeElement::new(*kind, *text)).collect(),
            })
            .collect(),
    }
}

#[test]
fn uniform_antiphons_keep_framing_pairs_and_section_boundaries() {
    for (input, expected) in [
        (vec![(Antiphon, "A"), (Psalm, "1"), (Antiphon, "A")], vec![(Antiphon, "A"), (Psalm, "1"), (Antiphon, "A")]),
        (
            vec![(Antiphon, "A"), (Psalm, "1"), (Antiphon, "A"), (Psalm, "2"), (Antiphon, "A")],
            vec![(Antiphon, "A"), (Psalm, "1"), (Psalm, "2"), (Antiphon, "A")],
        ),
        (
            vec![(Antiphon, "A"), (Psalm, "A"), (Rubric, "A"), (Heading, "A"), (Antiphon, "A")],
            vec![(Antiphon, "A"), (Psalm, "A"), (Rubric, "A"), (Heading, "A"), (Antiphon, "A")],
        ),
        (
            vec![
                (Antiphon, "A"),
                (Psalm, "1"),
                (Antiphon, "A"),
                (Psalm, "2"),
                (Antiphon, "A"),
                (Antiphon, "B"),
                (Psalm, "3"),
                (Antiphon, "B"),
                (Psalm, "4"),
                (Antiphon, "B"),
                (Antiphon, "C"),
            ],
            vec![
                (Antiphon, "A"),
                (Psalm, "1"),
                (Psalm, "2"),
                (Antiphon, "A"),
                (Antiphon, "B"),
                (Psalm, "3"),
                (Psalm, "4"),
                (Antiphon, "B"),
                (Antiphon, "C"),
            ],
        ),
    ] {
        let mut h = hour(&[&input]);
        collapse_uniform_antiphons(&mut h);
        assert_eq!(h.sections, hour(&[&expected]).sections);
    }
    let a = &[(Rubric, "Before"), (Antiphon, "A"), (Psalm, "1"), (Antiphon, "A")][..];
    let b = &[(Antiphon, "A"), (Canticle, "C"), (Antiphon, "A"), (Rubric, "After")][..];
    let mut h = hour(&[a, b]);
    collapse_uniform_antiphons(&mut h);
    assert_eq!(h.sections, hour(&[&a[..3], &b[1..]]).sections);
    let mut h = hour(&[a, &[(Rubric, "ceremony")], b]);
    let original = h.sections.clone();
    collapse_uniform_antiphons(&mut h);
    assert_eq!(h.sections, original);
}

#[test]
fn doxology_classification_respects_preceding_element_and_section() {
    let mut h = hour(&[
        &[(Doxology, ""), (Psalm, ""), (Doxology, ""), (Canticle, ""), (Doxology, ""), (Versicle, ""), (Doxology, ""), (Psalm, "")],
        &[(Doxology, "")],
    ]);
    mark_psalm_doxologies(&mut h);
    let kinds: Vec<_> = h.sections[0].elements.iter().map(|e| e.kind).collect();
    assert_eq!(kinds, [Doxology, Psalm, PsalmDoxology, Canticle, PsalmDoxology, Versicle, Doxology, Psalm]);
    assert_eq!(h.sections[1].elements[0].kind, Doxology);
}

#[test]
fn announcements_follow_hour_rank_and_evening_owner() {
    let feria = day(date(2026, 3, 18), Season::Lent);
    let mut f = feast("st-joseph", Some(Category::Confessor));
    f.rank = Rank::Double1stClass;
    let double = celebrating(feria.date, Season::Lent, f.clone());
    let semi = celebrating(feria.date, Season::Lent, feast("sunday", Some(Category::Sunday)));
    for name in HOUR_NAMES {
        assert_eq!(antiphons_doubled(&double, name), matches!(name, "lauds" | "vespers"));
        assert!(!antiphons_doubled(&semi, name));
        assert!(!antiphons_doubled(&feria, name));
    }
    let mut evening = feria.clone();
    evening.vespers.owner = crate::VespersOwner::IOfFollowing;
    evening.vespers.feast = Some(arc(f));
    assert!(antiphons_doubled(&evening, "vespers"));
    assert!(!antiphons_doubled(&evening, "lauds"));
    let input = &[
        (Antiphon, "Alleluia"),
        (Antiphon, "A"),
        (Psalm, "p"),
        (PsalmDoxology, "g"),
        (Antiphon, "A"),
        (Heading, "Commemoration"),
        (Antiphon, "B"),
        (Versicle, "v"),
    ][..];
    for (day, name, announced) in [(&feria, "lauds", true), (&double, "lauds", false), (&double, "prime", true)] {
        let mut h = hour(&[input]);
        mark_announced_antiphons(&mut h, day, name);
        assert_eq!(
            h.sections[0].elements.iter().enumerate().filter(|(_, e)| e.announce).map(|(i, _)| i).collect::<Vec<_>>(),
            if announced { vec![1] } else { vec![] }
        );
    }
    let mut h = hour(&[input, input]);
    h.sections[1].label = VESPERS_OF_THE_DEAD_LABEL.into();
    mark_announced_antiphons(&mut h, &feria, "vespers");
    assert!(h.sections[0].elements[1].announce);
    assert!(h.sections[1].elements.iter().all(|e| !e.announce));
}

#[test]
fn unresolved_markers_are_detected_in_rendered_text() {
    assert_eq!(unresolved_marker("[Text not found: ordinary/lauds/collect]"), Some("[Text not found: ordinary/lauds/collect]"));
    assert_eq!(
        unresolved_marker("Ant. [Commemoration text not found: commemoration-antiphon for st-x] V."),
        Some("[Commemoration text not found: commemoration-antiphon for st-x]")
    );
    assert!(unresolved_marker("[Little Hours versicle not found: proper/x/short-responsory]").is_some());
    // Canticle section markup and ordinary brackets are not markers.
    assert_eq!(unresolved_marker("[section: Benedicite] O all ye Works of the Lord"), None);
    assert_eq!(unresolved_marker("(which he had promised afore) [sic]"), None);
}
