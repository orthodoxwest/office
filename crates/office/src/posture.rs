//! Congregational posture cues in the psalmody of Lauds and Vespers, as the
//! parish booklets print them: all sit after the mediant of a unit's first
//! verse, stand at the mediant of the verse before its doxology, bow at
//! "Glory be" and stand upright at "As it was". A unit is the run of psalms
//! said under one doxology, so a psalm said on without Glory be is cued only
//! where the unit opens or closes. The Gospel canticles are sung standing and
//! take only the bow. The Benedicite closes with its own doxology verse ("Let
//! us bless the Father"), which takes the bow in place of the Gloria.
//!
//! The Office of the Dead ("Rest eternal") and the Triduum (no doxology) are
//! not cued: the booklets give no pattern for them.

use corpus::lines::{PsalmItem, parse_psalm};
use liturgy::{ElementType, OfficeElement, OfficeHour, Posture, PostureAnchor, PostureCue};

use crate::Day;
use crate::psalmody::{is_office_of_the_dead, says_psalm_doxology};

const GOSPEL_CANTICLES: [&str; 2] = ["canticles/benedictus", "canticles/magnificat"];
const IN_TEXT_DOXOLOGY: &str = "Let us bless the Father";

/// Marks the posture cues on the psalmody of Lauds and Vespers.
pub fn mark_postures(hour: &mut OfficeHour, day: &Day, hour_name: &str) {
    if hour_name != "lauds" && hour_name != "vespers" {
        return;
    }
    let in_text_doxology = says_psalm_doxology(day, hour_name) && !is_office_of_the_dead(day);
    for section in &mut hour.sections {
        let elems = &mut section.elements;
        let mut i = 0;
        while i < elems.len() {
            if !elems[i].kind.is_psalmody() {
                i += 1;
                continue;
            }
            let start = i;
            while i < elems.len() && elems[i].kind.is_psalmody() {
                i += 1;
            }
            let end = i;
            let followed_by_gloria =
                end < elems.len() && elems[end].kind == ElementType::PsalmDoxology && elems[end].text.trim_start().starts_with("Glory be");
            if followed_by_gloria {
                if !is_gospel_canticle(&elems[start]) {
                    mark_unit(&mut elems[start..end], None);
                }
                let lines = elems[end].text.trim().lines().count();
                let doxology = &mut elems[end].postures;
                doxology.push(PostureCue::new(Posture::Bow, PostureAnchor::BeforeVerse(0)));
                if lines > 1 {
                    doxology.push(PostureCue::new(Posture::StandUpright, PostureAnchor::BeforeVerse(1)));
                }
            } else if in_text_doxology && let Some(at) = own_doxology_verse(&elems[end - 1]) {
                mark_unit(&mut elems[start..end], Some(at));
            }
        }
    }
}

fn is_gospel_canticle(elem: &OfficeElement) -> bool {
    elem.kind == ElementType::Canticle && GOSPEL_CANTICLES.contains(&elem.source_ref.as_str())
}

/// The first half of each verse.
fn verse_openings(elem: &OfficeElement) -> Vec<String> {
    let items = parse_psalm(&elem.text).items;
    items
        .into_iter()
        .filter_map(|item| match item {
            PsalmItem::Verse { first, .. } => Some(first),
            PsalmItem::Section { .. } | PsalmItem::Gloria { .. } => None,
        })
        .collect()
}

fn verse_count(elem: &OfficeElement) -> usize {
    verse_openings(elem).len()
}

/// The index of a canticle's own doxology verse, when it has one after its
/// first verse.
fn own_doxology_verse(elem: &OfficeElement) -> Option<usize> {
    if elem.kind != ElementType::Canticle {
        return None;
    }
    verse_openings(elem).iter().skip(1).position(|first| first.starts_with(IN_TEXT_DOXOLOGY)).map(|n| n + 1)
}

/// Cues a unit: sit in its first verse, stand in the verse before the
/// doxology, and, for a canticle's own doxology verse, bow into it and stand
/// upright for the verse after. A unit of one verse has no time to sit.
fn mark_unit(unit: &mut [OfficeElement], own_doxology: Option<usize>) {
    let Some(last) = unit.len().checked_sub(1) else { return };
    let last_verses = verse_count(&unit[last]);
    let stand_verse = match own_doxology {
        Some(n) => n - 1,
        None if last_verses > 0 => last_verses - 1,
        None => return,
    };
    if verse_count(&unit[0]) == 0 || (last == 0 && stand_verse == 0) {
        return;
    }
    unit[0].postures.push(PostureCue::new(Posture::Sit, PostureAnchor::AfterMediant(0)));
    let closing = &mut unit[last].postures;
    closing.push(PostureCue::new(Posture::Stand, PostureAnchor::AfterMediant(stand_verse)));
    if let Some(n) = own_doxology {
        closing.push(PostureCue::new(Posture::Bow, PostureAnchor::BeforeVerse(n)));
        if n + 1 < last_verses {
            closing.push(PostureCue::new(Posture::StandUpright, PostureAnchor::BeforeVerse(n + 1)));
        }
    }
}

#[cfg(test)]
mod tests {
    use liturgy::{OfficeSection, PrayerForm};

    use super::*;

    const GLORIA: &str =
        "Glory be to the Father, and to the Son, * and to the Holy Ghost;\nAs it was in the beginning, * world without end. Amen.";

    fn el(kind: ElementType, text: &str, source: &str) -> OfficeElement {
        OfficeElement { source_ref: source.into(), ..OfficeElement::new(kind, text) }
    }

    fn psalm(verses: usize) -> OfficeElement {
        let text: Vec<String> = (1..=verses).map(|n| format!("{n}. Verse {n} * second half.")).collect();
        el(ElementType::Psalm, &text.join("\n"), "psalms/x")
    }

    fn marked(elements: Vec<OfficeElement>) -> Vec<Vec<PostureCue>> {
        let mut hour = OfficeHour {
            form: PrayerForm::Private,
            date: calendar::Date::new(2026, 10, 5),
            hour: "Lauds".into(),
            title: String::new(),
            season: None,
            feast: String::new(),
            color: None,
            sections: vec![OfficeSection { label: String::new(), collapsible: false, elements }],
            decisions: Vec::new(),
        };
        let day = crate::testutil::day(calendar::Date::new(2026, 10, 5), calendar::Season::Pentecost);
        mark_postures(&mut hour, &day, "lauds");
        hour.sections.remove(0).elements.into_iter().map(|e| e.postures).collect()
    }

    fn cue(posture: Posture, at: PostureAnchor) -> PostureCue {
        PostureCue::new(posture, at)
    }

    use PostureAnchor::{AfterMediant, BeforeVerse};

    #[test]
    fn single_psalm_sits_stands_and_bows() {
        let got = marked(vec![
            el(ElementType::Antiphon, "Ant.", "a"),
            psalm(6),
            el(ElementType::PsalmDoxology, GLORIA, "ordinary/shared/gloria-patri"),
            el(ElementType::Antiphon, "Ant.", "a"),
        ]);
        assert_eq!(
            got,
            vec![
                vec![],
                vec![cue(Posture::Sit, AfterMediant(0)), cue(Posture::Stand, AfterMediant(5))],
                vec![cue(Posture::Bow, BeforeVerse(0)), cue(Posture::StandUpright, BeforeVerse(1))],
                vec![],
            ]
        );
    }

    #[test]
    fn joined_psalms_are_cued_once() {
        let got = marked(vec![psalm(14), psalm(9), psalm(6), el(ElementType::PsalmDoxology, GLORIA, "g")]);
        assert_eq!(got[0], vec![cue(Posture::Sit, AfterMediant(0))]);
        assert_eq!(got[1], vec![]);
        assert_eq!(got[2], vec![cue(Posture::Stand, AfterMediant(5))]);
        // A two-verse psalm closing a unit stands in its first verse.
        let got = marked(vec![psalm(7), psalm(2), el(ElementType::PsalmDoxology, GLORIA, "g")]);
        assert_eq!(got[1], vec![cue(Posture::Stand, AfterMediant(1))]);
    }

    #[test]
    fn a_one_verse_unit_only_bows() {
        let got = marked(vec![psalm(1), el(ElementType::PsalmDoxology, GLORIA, "g")]);
        assert_eq!(got[0], vec![]);
        assert_eq!(got[1].len(), 2);
    }

    #[test]
    fn gospel_canticles_only_bow() {
        let got = marked(vec![
            el(ElementType::Canticle, "1. Blessed * be.\n2. For he * hath.", "canticles/benedictus"),
            el(ElementType::PsalmDoxology, GLORIA, "g"),
        ]);
        assert_eq!(got, vec![vec![], vec![cue(Posture::Bow, BeforeVerse(0)), cue(Posture::StandUpright, BeforeVerse(1))]]);
    }

    #[test]
    fn benedicite_bows_at_its_own_doxology() {
        let text = "Benedicite\n\nO all ye Works * bless ye.\n2 O ye Angels * bless ye.\n3 O Ananias * praise him.\n\
                    4 Let us bless the Father, and the Son, and the Holy Ghost: * praise him.\n5 Blessed art thou * for ever.";
        let got = marked(vec![el(ElementType::Canticle, text, "canticles/benedicite"), el(ElementType::Antiphon, "Ant.", "a")]);
        assert_eq!(
            got[0],
            vec![
                cue(Posture::Sit, AfterMediant(0)),
                cue(Posture::Stand, AfterMediant(2)),
                cue(Posture::Bow, BeforeVerse(3)),
                cue(Posture::StandUpright, BeforeVerse(4)),
            ]
        );
    }

    #[test]
    fn no_cues_without_the_gloria() {
        // The Triduum omits the doxology; the Office of the Dead says Rest eternal.
        assert_eq!(marked(vec![psalm(6), el(ElementType::Antiphon, "Ant.", "a")]), vec![vec![], vec![]]);
        let rest = el(ElementType::PsalmDoxology, "Rest eternal * grant unto them.\nAnd let * light.", "shared/formulas/rest-eternal");
        assert_eq!(marked(vec![psalm(6), rest]), vec![vec![], vec![]]);
    }
}
