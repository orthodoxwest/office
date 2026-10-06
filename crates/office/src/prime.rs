//! Prime.

use calendar::{Category, MoveableDates, Season};
use liturgy::{ElementType, OfficeElement, OfficeHour, OfficeSection, PrayerForm};

use crate::day::Day;
use crate::engine::{append_hour_element, compact_refs, resolve_element};
use crate::hourdef::{HourElement, HourSection};
use crate::major::record_condition_decision;
use crate::preces::SATURDAY_OFFICE_BVM_ID;
use crate::proper::{advent_ferial_psalm_antiphon_ref, lookup_feast_proper_text, resolve_proper_text};
use crate::texts::OfficeTexts;

/// Composes Prime. `martyrology` substitutes the next day's reviewed
/// Martyrology entry for the static rubric; readers turn it on in Settings,
/// and it is off by default.
pub fn compose_prime(
    day: &Day,
    sections: &[HourSection],
    t: &OfficeTexts,
    moveable: Option<&MoveableDates>,
    martyrology: bool,
) -> OfficeHour {
    let mut hour = new_hour("Prime", day);
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
            if martyrology && section.name == "Martyrology" && elem.reference == MARTYROLOGY_RUBRIC {
                elems.extend(resolve_prime_martyrology(day, t));
                continue;
            }
            if elem.kind == "proper-antiphon" && elem.reference == "psalm-antiphon-1" && !crate::psalmody::is_office_of_the_dead(day) {
                elems.push(resolve_prime_psalm_antiphon(day, t, moveable));
                continue;
            }
            append_hour_element(&mut elems, day, "prime", elem, t);
        }
        hour.sections.push(OfficeSection { label: section.label.clone(), collapsible: section.collapsible, elements: elems });
    }
    hour
}

const MARTYROLOGY_RUBRIC: &str = "ordinary/prime/martyrology-rubric";

/// Whether a composed Prime reads a day's Martyrology entry, rather than
/// the rubric that stands in for it.
pub fn reads_martyrology(hour: &OfficeHour) -> bool {
    hour.sections.iter().flat_map(|s| &s.elements).any(|e| e.source_ref.starts_with("ordinary/martyrology/"))
}

const MONTH_NAMES: [&str; 12] =
    ["January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"];

/// The next day's reviewed Martyrology entry. The static rubric remains the
/// fallback while the per-date corpus is being populated.
fn resolve_prime_martyrology(day: &Day, t: &OfficeTexts) -> Vec<OfficeElement> {
    let next = day.date.add_days(1);
    let reference = format!("ordinary/martyrology/{:02}-{:02}", next.month(), next.day());
    const CONCLUSION: &str = "ordinary/martyrology/conclusion";
    const RESPONSE: &str = "ordinary/martyrology/response";
    let (text, conclusion, response) = (t.get(&reference), t.get(CONCLUSION), t.get(RESPONSE));
    if text.is_empty() || conclusion.is_empty() || response.is_empty() {
        return vec![resolve_element(&HourElement::new("rubric", MARTYROLOGY_RUBRIC), t)];
    }
    let sourced = |kind: ElementType, text: String, key: &str| OfficeElement {
        source_ref: key.to_string(),
        source_refs: vec![key.to_string()],
        ..OfficeElement::new(kind, text)
    };
    vec![
        sourced(
            ElementType::Heading,
            format!("Martyrology for tomorrow, {} {}", MONTH_NAMES[next.month() as usize - 1], next.day()),
            &reference,
        ),
        sourced(ElementType::Reading, text.to_string(), &reference),
        sourced(ElementType::Reading, conclusion.to_string(), CONCLUSION),
        sourced(ElementType::Response, format!("R. {response}"), RESPONSE),
    ]
}

/// An hour titled `name` for the day's own office.
pub fn new_hour(name: &str, day: &Day) -> OfficeHour {
    OfficeHour {
        form: PrayerForm::Private,
        date: day.date,
        hour: name.to_string(),
        title: name.to_string(),
        season: Some(day.season),
        feast: day.celebration.as_deref().map(|c| c.name.clone()).unwrap_or_default(),
        color: Some(day.color),
        sections: Vec::new(),
        decisions: Vec::new(),
    }
}

/// Prime's antiphon rubric: feasts and Sundays take the first Lauds
/// antiphon unless Prime has its own; ferias take the seasonal exceptions,
/// then the weekday psalter.
fn resolve_prime_psalm_antiphon(day: &Day, t: &OfficeTexts, moveable: Option<&MoveableDates>) -> OfficeElement {
    const SLOT: &str = "psalm-antiphon-1";
    // Passion and Palm Sundays print their own Prime antiphons.
    let (text, key) = lookup_feast_proper_text(day, "prime", SLOT, t);
    if !text.is_empty() && is_prime_antiphon_ref(&key, day.season) {
        return prime_element(SLOT, &key, text);
    }
    if day.celebration_is(SATURDAY_OFFICE_BVM_ID) {
        let key = "proper/saturday-office-bvm/saturday-psalm-antiphon-1";
        return prime_element(SLOT, key, t.get(key).to_string());
    }
    let ferial = day.celebration.as_deref().is_none_or(|c| c.is_category(Category::Feria));
    if !ferial {
        let (mut text, mut key) = resolve_proper_text(day, "lauds", SLOT, t);
        // The ordinary Sunday psalter has its own threefold Alleluia (p. 83).
        if key == "ordinary/lauds/psalm-antiphon-1-sunday" {
            key = "ordinary/prime/psalm-antiphon-1-sunday".to_string();
            text = t.get(&key).to_string();
        }
        return prime_element(SLOT, &key, text);
    }
    let computed;
    let m = match moveable {
        Some(m) => m,
        None => {
            computed = MoveableDates::compute(day.date.year());
            &computed
        }
    };
    let weekday = day.civil_weekday_name();
    let mut key = format!("ordinary/prime/{SLOT}-{weekday}");
    match day.season {
        Season::Advent => {
            if let Some(advent) = advent_ferial_psalm_antiphon_ref(day, "prime", 1) {
                key = advent;
            }
        }
        Season::Lent => {
            if day.date >= m.lent1.add_days(1) {
                key = format!("seasonal/lent/{SLOT}-prime");
            }
        }
        Season::Passiontide => {
            if day.date >= m.holy_monday {
                let (text, proper_key) = resolve_proper_text(day, "lauds", SLOT, t);
                return prime_element(SLOT, &proper_key, text);
            }
            key = format!("seasonal/passiontide/{SLOT}-prime");
        }
        Season::Easter => {
            if day.date >= m.low_sunday.add_days(1) && day.date < m.ascension {
                key = format!("seasonal/easter/{SLOT}-prime");
            }
        }
        Season::Christmas | Season::Epiphany | Season::Septuagesima | Season::Pentecost => {}
    }
    let text = t.get(&key).to_string();
    prime_element(SLOT, &key, text)
}

pub(crate) fn is_prime_antiphon_ref(reference: &str, season: Season) -> bool {
    reference.ends_with("-prime") || reference.ends_with(&format!("-prime-{season}"))
}

fn prime_element(slot: &str, key: &str, text: String) -> OfficeElement {
    let text = if text.is_empty() { format!("[Prime psalm antiphon not found: {key}]") } else { text };
    let mut e = OfficeElement::new(ElementType::Antiphon, text);
    e.slot_ref = slot.to_string();
    e.source_ref = key.to_string();
    e.source_refs = compact_refs(vec![key.to_string()]);
    e
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::testutil::{celebrating, date, day, feast, texts};
    use calendar::Rank;

    #[test]
    fn martyrology_uses_next_civil_date_and_keeps_missing_day_rubric() {
        let sections = crate::hourdef::parse_hour_definition("prime", include_str!("../../../data/office/prime.txt")).unwrap();
        for (current, key, title) in [
            (date(2026, 9, 7), "09-08", "September 8"),
            (date(2026, 12, 31), "01-01", "January 1"),
            (date(2027, 2, 28), "03-01", "March 1"),
            (date(2028, 2, 28), "02-29", "February 29"),
            (date(2026, 3, 7), "03-08", "March 8"),
        ] {
            let reference = format!("ordinary/martyrology/{key}");
            let t = texts(&[
                (MARTYROLOGY_RUBRIC, "Read the Martyrology."),
                (&reference, "Reviewed entry."),
                ("ordinary/martyrology/conclusion", "Many other holy ones."),
                ("ordinary/martyrology/response", "Thanks be to God."),
            ]);
            let d = day(current, Season::Lent);
            let h = compose_prime(&d, &sections, &t, Some(&MoveableDates::compute(current.year())), true);
            let elements: Vec<_> = h
                .sections
                .iter()
                .flat_map(|s| &s.elements)
                .filter(|e| e.source_ref.starts_with("ordinary/martyrology/") || e.kind == ElementType::Heading)
                .collect();
            assert_eq!(elements.len(), 4);
            assert_eq!(elements[0].text, format!("Martyrology for tomorrow, {title}"));
            assert_eq!(elements[1].kind, ElementType::Reading);
            assert_eq!(elements[1].text, "Reviewed entry.");
            assert_eq!(elements[1].source_ref, reference);
            assert_eq!(elements[2].text, "Many other holy ones.");
            assert_eq!(elements[3].kind, ElementType::Response);
            assert_eq!(elements[3].text, "R. Thanks be to God.");
            let plain = compose_prime(&d, &sections, &t, None, false);
            assert!(!plain.sections.iter().flat_map(|s| &s.elements).any(|e| e.kind == ElementType::Reading));
        }
        let t = texts(&[(MARTYROLOGY_RUBRIC, "Read the Martyrology.")]);
        let d = day(date(2026, 12, 31), Season::Christmas);
        let fallback = resolve_prime_martyrology(&d, &t);
        assert_eq!(fallback.len(), 1);
        assert_eq!(fallback[0].kind, ElementType::Rubric);
        assert_eq!(fallback[0].text, "Read the Martyrology.");
        for id in ["holy-thursday", "good-friday", "holy-saturday"] {
            let mut f = feast(id, None);
            f.rank = Rank::Double1stClass;
            let d = celebrating(date(2026, 4, 9), Season::Passiontide, f);
            let h = compose_prime(&d, &sections, &t, Some(&MoveableDates::compute(2026)), true);
            assert!(!h.sections.iter().flat_map(|s| &s.elements).any(|e| e.source_ref.contains("martyrology")));
        }
    }
}
