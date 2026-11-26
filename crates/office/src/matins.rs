//! Matins, the night office. Composed so far only in its Office of the Dead
//! form (Monastic Diurnal pp. 81*–99*), said in the app on All Souls' Day with
//! all three Nocturns. The hour stays out of [`liturgy::HOUR_NAMES`], which
//! the fronts publish, until the fronts publish it; the CLI composes it.

use calendar::MoveableDates;
use liturgy::{ElementType, INVITATORY_ANTIPHON, OfficeElement, OfficeHour, OfficeSection};

use crate::day::Day;
use crate::engine::{append_hour_element, append_resolved, extract_chapter_ref, format_label};
use crate::hourdef::{HourElement, HourSection};
use crate::major::record_condition_decision;
use crate::prime::new_hour;
use crate::texts::OfficeTexts;

/// The Office of the Dead's own definition, chosen as Holy Saturday's Vespers is.
pub const DEAD_MATINS_DEFINITION: &str = "matins-dead";

/// Psalm 95 in the strophes between which the invitatory's antiphon is said
/// (Monastic Diurnal pp. 82*–83*).
pub const INVITATORY_PSALM: &str = "ordinary/matins/invitatory-psalm";

/// What the Office of the Dead says after the invitatory psalm in place of
/// the Glory be, as the Diurnal prints it there (p. 83*). The ordinary
/// Matins, when it comes, ends the psalm with the Glory be.
pub const INVITATORY_DOXOLOGY_DEAD: &str = "shared/formulas/rest-eternal-invitatory";

/// The definition Matins is composed from on this day, if it is composed.
pub fn matins_definition(day: &Day) -> Option<&'static str> {
    crate::psalmody::is_office_of_the_dead(day).then_some(DEAD_MATINS_DEFINITION)
}

pub fn compose_matins(day: &Day, sections: &[HourSection], t: &OfficeTexts, moveable: Option<&MoveableDates>) -> OfficeHour {
    let mut hour = new_hour("Matins", day);
    for section in sections {
        if !section.condition.is_empty() {
            let included = section.condition_holds(day, moveable, t);
            record_condition_decision(&mut hour, &section.condition, included, &section.name);
            if !included {
                continue;
            }
        }
        let mut elems = Vec::new();
        for elem in &section.elements {
            match elem.kind.as_str() {
                "invitatory" => append_resolved(&mut elems, invitatory(elem, t)),
                "lesson" => append_resolved(&mut elems, lesson(elem, t)),
                _ => append_hour_element(&mut elems, day, "matins", elem, t),
            }
        }
        hour.sections.push(OfficeSection { label: section.label.clone(), collapsible: section.collapsible, bare: false, elements: elems });
    }
    hour
}

/// The text of `reference`, or the marker that fails the composition.
fn text_of(t: &OfficeTexts, reference: &str) -> String {
    match t.get(reference) {
        "" => format!("[Text not found: {reference}]"),
        text => text.to_string(),
    }
}

/// The invitatory: its antiphon said twice, then said whole and in its second
/// half by turns after the strophes of Psalm 95, and after the doxology half
/// and then whole (Monastic Diurnal pp. 82*–83*). The `Ref` names the antiphon.
fn invitatory(elem: &HourElement, t: &OfficeTexts) -> OfficeElement {
    let whole = text_of(t, &elem.reference);
    let half = whole.split_once(" * ").map_or(whole.as_str(), |(_, second)| second).trim().to_string();
    let psalm = text_of(t, INVITATORY_PSALM);
    let strophes: Vec<&str> = psalm.split("\n\n").map(str::trim).filter(|s| !s.is_empty()).collect();
    let doxology = text_of(t, INVITATORY_DOXOLOGY_DEAD);
    let ant = |s: &str| format!("{INVITATORY_ANTIPHON}{s}");
    let mut lines = vec![ant(&whole), ant(&whole)];
    for (i, strophe) in strophes.iter().enumerate() {
        lines.push(strophe.to_string());
        lines.push(ant(if i % 2 == 0 { &whole } else { &half }));
    }
    lines.push(doxology.trim().to_string());
    lines.push(ant(&half));
    lines.push(ant(&whole));
    let mut e = OfficeElement::new(ElementType::Invitatory, lines.join("\n"));
    e.label = format_label("psalm", "psalms/095");
    e.incipit = t.incipit("psalms/095").unwrap_or("").to_string();
    e.slot_ref = "invitatory".to_string();
    e.source_ref = elem.reference.clone();
    e.source_refs = vec![elem.reference.clone(), INVITATORY_PSALM.to_string(), INVITATORY_DOXOLOGY_DEAD.to_string()];
    e
}

/// A lesson: its heading from the key's numeral ("lesson-iv" is Lesson IV),
/// its reference from the text's leading `!` line, as a chapter's.
fn lesson(elem: &HourElement, t: &OfficeTexts) -> OfficeElement {
    let (reference, body) = extract_chapter_ref(&text_of(t, &elem.reference));
    let slot = elem.reference.rsplit('/').next().unwrap_or("");
    let mut e = OfficeElement::new(ElementType::Lesson, body);
    e.label = match slot.strip_prefix("lesson-") {
        Some(numeral) => format!("Lesson {}", numeral.to_uppercase()),
        None => "Lesson".to_string(),
    };
    e.incipit = reference;
    e.slot_ref = slot.to_string();
    e.source_ref = elem.reference.clone();
    e.source_refs = vec![elem.reference.clone()];
    e
}

#[cfg(test)]
#[path = "matins_tests.rs"]
mod tests;
