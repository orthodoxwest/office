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
