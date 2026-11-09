//! Prime.

use calendar::date::is_leap_year;
use calendar::{Category, Date, MoveableDates, Season};
use liturgy::{ElementType, OfficeElement, OfficeHour, OfficeSection, PrayerForm};

use crate::day::Day;
use crate::engine::{append_hour_element, compact_refs, resolve_element};
use crate::hourdef::{HourElement, HourSection};
use crate::major::record_condition_decision;
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
            if section.name == "Martyrology" && elem.reference == MARTYROLOGY_RUBRIC {
                elems.extend(if martyrology { resolve_prime_martyrology(day, t) } else { martyrology_rubric(day, t) });
                continue;
            }
            if elem.kind == "proper-antiphon" && elem.reference == "psalm-antiphon-1" && !crate::psalmody::is_office_of_the_dead(day) {
                elems.push(resolve_prime_psalm_antiphon(day, t, moveable));
                continue;
            }
            append_hour_element(&mut elems, day, "prime", elem, t);
        }
        hour.sections.push(OfficeSection { label: section.label.clone(), collapsible: section.collapsible, bare: false, elements: elems });
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
/// fallback while the per-date corpus is being populated. A feast kept today
/// may have its announcement read before the date (All Souls, Diurnal p. 653);
/// those kept tomorrow are announced in the first place after it.
fn resolve_prime_martyrology(day: &Day, t: &OfficeTexts) -> Vec<OfficeElement> {
    let next = day.date.add_days(1);
    let reference = martyrology_entry_key(next);
    const CONCLUSION: &str = "ordinary/martyrology/conclusion";
    const RESPONSE: &str = "ordinary/martyrology/response";
    let (text, conclusion, response) = (t.get(&reference), t.get(CONCLUSION), t.get(RESPONSE));
    if text.is_empty() || conclusion.is_empty() || response.is_empty() {
        return martyrology_rubric(day, t);
    }
    let mut elements: Vec<OfficeElement> =
        announcements(&day.kept, ANNOUNCEMENT_ON_DAY, t).into_iter().map(|(key, text)| sourced(ElementType::Reading, text, &key)).collect();
    elements.push(sourced(
        ElementType::Heading,
        format!("Martyrology for tomorrow, {} {}", MONTH_NAMES[next.month() as usize - 1], next.day()),
        &reference,
    ));
    for (key, announcement) in announcements(&day.tomorrow, ANNOUNCEMENT, t) {
        elements.push(sourced(ElementType::Reading, announcement, &key));
    }
    elements.extend([
        sourced(ElementType::Reading, text.to_string(), &reference),
        sourced(ElementType::Reading, conclusion.to_string(), CONCLUSION),
        sourced(ElementType::Response, format!("R. {response}"), RESPONSE),
    ]);
    elements
}

/// A feast's announcement read at Prime the day before, after the date
/// ("in the 1st place", as the ordo prints Holy Name's).
const ANNOUNCEMENT: &str = "martyrology-announcement";
/// A feast's announcement read at Prime on its own day, before the date.
const ANNOUNCEMENT_ON_DAY: &str = "martyrology-announcement-on-day";
/// The rubric that, with the Martyrology not read, introduces an on-day
/// announcement in place of the general rubric.
const ANNOUNCEMENT_RUBRIC: &str = "martyrology-rubric";

/// The rubric standing for the Martyrology. A feast kept today whose proper
/// prints its own (All Souls, Diurnal p. 653) keeps that rubric and its
/// announcement, as the book prints them.
fn martyrology_rubric(day: &Day, t: &OfficeTexts) -> Vec<OfficeElement> {
    let found = day.kept.iter().find_map(|feast| {
        crate::proper::feast_proper_ids(feast).into_iter().find_map(|id| {
            let rubric = format!("proper/{id}/{ANNOUNCEMENT_RUBRIC}");
            let announcement = format!("proper/{id}/{ANNOUNCEMENT_ON_DAY}");
            (!t.get(&rubric).is_empty() && !t.get(&announcement).is_empty()).then_some((rubric, announcement))
        })
    });
    match found {
        Some((rubric, announcement)) => vec![
            resolve_element(&HourElement::new("rubric", &rubric), t),
            sourced(ElementType::Reading, t.get(&announcement).to_string(), &announcement),
        ],
        None => vec![resolve_element(&HourElement::new("rubric", MARTYROLOGY_RUBRIC), t)],
    }
}

/// The announcements `feasts` carry under `key`, in the calendar's order: the
/// first of each feast's proper ids that has one.
fn announcements(feasts: &[calendar::FeastRef], key: &str, t: &OfficeTexts) -> Vec<(String, String)> {
    feasts
        .iter()
        .filter_map(|feast| {
            crate::proper::feast_proper_ids(feast).into_iter().find_map(|id| {
                let reference = format!("proper/{id}/{key}");
                let text = t.get(&reference);
                (!text.is_empty()).then(|| (reference.clone(), text.to_string()))
            })
        })
        .collect()
}

fn sourced(kind: ElementType, text: String, key: &str) -> OfficeElement {
    OfficeElement { source_ref: key.to_string(), source_refs: vec![key.to_string()], ..OfficeElement::new(kind, text) }
}

/// The printed day read for `date`. In a leap year the Martyrology keeps
/// February 24 for the bissextile day and reads each printed February 24–28
/// one civil day later (1916 ed., January–June, p. 15), as the calendar moves
/// those feasts.
fn martyrology_entry_key(date: Date) -> String {
    let (month, day) = (date.month(), date.day());
    match (month, day) {
        (2, 24) if is_leap_year(date.year()) => "ordinary/martyrology/02-24-bissextile".to_string(),
        (2, 25..=29) if is_leap_year(date.year()) => format!("ordinary/martyrology/02-{:02}", day - 1),
        _ => format!("ordinary/martyrology/{month:02}-{day:02}"),
    }
}

/// An hour titled `name` for the day's own office.
pub fn new_hour(name: &str, day: &Day) -> OfficeHour {
    let hour_name = name.to_lowercase();
    OfficeHour {
        form: PrayerForm::Private,
        date: day.date,
        hour: name.to_string(),
        title: name.to_string(),
        season: Some(day.season),
        feast: day.celebration.as_deref().map(|c| c.name.clone()).unwrap_or_default(),
        rank: day.celebration.as_deref().map(|c| c.rank),
        color: Some(day.color),
        paschaltide: day.is_paschaltide(&hour_name),
        bare: is_bare(day, &hour_name),
        sections: Vec::new(),
        decisions: Vec::new(),
    }
}

/// Whether the hour is stripped of its furniture (`OfficeHour::bare`): the Triduum's form,
/// which ends at None of Holy Saturday, and the Office of the Dead. Decided by the same
/// predicates that strip the text, so the page cannot disagree with the office.
pub fn is_bare(day: &Day, hour_name: &str) -> bool {
    crate::hourdef::uses_triduum_form(day, hour_name) || crate::psalmody::is_office_of_the_dead(day)
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
    let ferial = day.celebration.as_deref().is_none_or(|c| c.is_category(Category::Feria));
    // The Saturday Office of Our Lady takes the Common of Feasts B.V.M.'s
    // Prime antiphon, "While the King" (Diurnal p. 67*; p. 69*) (#652).
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
            // Leap years read the bissextile day on the 24th, then each
            // printed February day one civil day late (#475).
            (date(2027, 2, 23), "02-24", "February 24"),
            (date(2028, 2, 23), "02-24-bissextile", "February 24"),
            (date(2028, 2, 24), "02-24", "February 25"),
            (date(2028, 2, 28), "02-28", "February 29"),
            (date(2028, 2, 29), "03-01", "March 1"),
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
