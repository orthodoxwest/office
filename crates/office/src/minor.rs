//! Terce, Sext, and None.

use calendar::{MoveableDates, Season};
use liturgy::{ElementType, OfficeElement, OfficeHour, OfficeSection};

use crate::day::Day;
use crate::engine::{append_hour_element, append_resolved, compact_refs};
use crate::hourdef::HourSection;
use crate::major::record_condition_decision;
use crate::prime::new_hour;
use crate::proper::resolve_proper_text;
use crate::texts::OfficeTexts;

pub fn compose_minor_hour(
    name: &str,
    day: &Day,
    sections: &[HourSection],
    t: &OfficeTexts,
    moveable: Option<&MoveableDates>,
) -> OfficeHour {
    let hour_name = name.to_lowercase();
    let mut hour = new_hour(name, day);
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
            if elem.kind == "proper-versicle" && elem.reference == "versicle" {
                append_resolved(&mut elems, resolve_minor_hour_versicle(day, &hour_name, t));
                continue;
            }
            append_hour_element(&mut elems, day, &hour_name, elem, t);
        }
        hour.sections.push(OfficeSection { label: section.label.clone(), collapsible: section.collapsible, elements: elems });
    }
    hour
}

fn versicle(text: String, source: &str) -> OfficeElement {
    let mut e = OfficeElement::new(ElementType::Versicle, text);
    e.slot_ref = "versicle".to_string();
    e.source_ref = source.to_string();
    e.source_refs = compact_refs(vec![source.to_string()]);
    e
}

/// The Little Hours take a simple versicle after the chapter; responsories
/// stored in Responsory Breve form are reduced to their opening pair.
fn resolve_minor_hour_versicle(day: &Day, hour_name: &str, t: &OfficeTexts) -> OfficeElement {
    let (responsory, responsory_ref) = resolve_proper_text(day, hour_name, "short-responsory", t);
    if corpus::is_omitted(&responsory) {
        return versicle(responsory, &responsory_ref);
    }
    let (text, r) = resolve_proper_text(day, hour_name, "versicle", t);
    if minor_text_tier(&r) <= minor_text_tier(&responsory_ref) {
        return versicle(decorate_minor_hour_versicle(day, &text), &r);
    }
    let text = match short_responsory_versicle(&responsory) {
        Some(v) => decorate_minor_hour_versicle(day, &v),
        None => format!("[Little Hours versicle not found: {responsory_ref}]"),
    };
    versicle(text, &responsory_ref)
}

fn minor_text_tier(r: &str) -> usize {
    ["proper/", "commons/", "seasonal/", "ordinary/"].iter().position(|p| r.starts_with(p)).unwrap_or(4)
}

fn decorate_minor_hour_versicle(day: &Day, text: &str) -> String {
    if day.season != Season::Easter {
        return text.to_string();
    }
    text.split('\n')
        .map(|line| {
            let trimmed = line.trim();
            if !trimmed.starts_with("V. ") && !trimmed.starts_with("R. ") {
                return line.to_string();
            }
            format!("{}{}, alleluia.", &trimmed[..3], strip_trailing_alleluias(trimmed[3..].trim()))
        })
        .collect::<Vec<_>>()
        .join("\n")
}

fn strip_trailing_alleluias(text: &str) -> String {
    let mut text = text.trim().strip_suffix('.').unwrap_or(text.trim()).trim().to_string();
    loop {
        let lower = text.to_lowercase();
        if lower.ends_with(", alleluia") {
            text = text[..text.len() - ", alleluia".len()].trim().to_string();
        } else if lower.ends_with(" alleluia") {
            text = text[..text.len() - " alleluia".len()].trim().to_string();
        } else {
            return text.trim_end_matches([' ', ',', ';', ':']).to_string();
        }
    }
}

fn short_responsory_versicle(responsory: &str) -> Option<String> {
    let (mut versicle, mut response) = (String::new(), String::new());
    for line in responsory.split('\n') {
        let line = line.trim();
        if versicle.is_empty()
            && let Some(rest) = line.strip_prefix("R. ")
        {
            versicle = rest.replace('*', "").trim().to_string();
        } else if !versicle.is_empty()
            && let Some(rest) = line.strip_prefix("V. ")
        {
            response = rest.trim().to_string();
        }
        if !versicle.is_empty() && !response.is_empty() {
            break;
        }
    }
    if versicle.is_empty() || response.is_empty() {
        return None;
    }
    Some(format!("V. {}\nR. {response}", versicle.split_whitespace().collect::<Vec<_>>().join(" ")))
}
