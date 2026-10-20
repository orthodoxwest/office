//! Prime.

use calendar::date::is_leap_year;
use calendar::{Category, Date, MoveableDates, Season, Weekday};
use liturgy::{ElementType, OfficeElement, OfficeHour, OfficeSection, PrayerForm};

use crate::day::Day;
use crate::engine::{append_hour_element, compact_refs, resolve_element};
use crate::hourdef::{HourElement, HourSection};
use crate::major::record_condition_decision;
use crate::proper::{advent_ferial_psalm_antiphon_ref, lookup_feast_proper_text, resolve_proper_text};
use crate::texts::OfficeTexts;

/// What Prime gives after its closing versicles (Diurnal p. 10), a reader's
/// setting: the Martyrology's rubric (the default), the next day's
/// Martyrology read, or the Capitular Office said out of Choir with the
/// Martyrology read within it.
#[derive(Clone, Copy, Debug, Default, PartialEq, Eq, Hash)]
pub enum AfterPrime {
    #[default]
    Rubric,
    Martyrology,
    ChapterOffice,
}

impl AfterPrime {
    pub const ALL: [AfterPrime; 3] = [AfterPrime::Rubric, AfterPrime::Martyrology, AfterPrime::ChapterOffice];

    /// The setting's stored value: "off", "on" (the Martyrology, the value the
    /// earlier On/Off setting kept) or "chapter".
    pub fn as_str(self) -> &'static str {
        match self {
            AfterPrime::Rubric => "off",
            AfterPrime::Martyrology => "on",
            AfterPrime::ChapterOffice => "chapter",
        }
    }

    /// A stored value; anything unknown is the default.
    pub fn parse(value: &str) -> AfterPrime {
        AfterPrime::ALL.into_iter().find(|a| a.as_str() == value).unwrap_or_default()
    }

    pub fn reads_martyrology(self) -> bool {
        self != AfterPrime::Rubric
    }
}

/// Composes Prime. `after_prime` is the reader's Settings choice: the
/// Martyrology substitutes the next day's reviewed entry for the static
/// rubric, and the Capitular Office adds its sections (`Chapter-Office…`) and
/// takes the place of *May the souls of the faithful*, which closes Prime only
/// when it is not said (p. 10) and ends its Blessing instead (p. 13).
pub fn compose_prime(
    day: &Day,
    sections: &[HourSection],
    t: &OfficeTexts,
    moveable: Option<&MoveableDates>,
    after_prime: AfterPrime,
) -> OfficeHour {
    let chapter_office = after_prime == AfterPrime::ChapterOffice;
    let mut hour = new_hour("Prime", day);
    for section in sections {
        if section.name.starts_with(CHAPTER_OFFICE) && !chapter_office {
            continue;
        }
        if !section.condition.is_empty() {
            let included = section.condition_holds(day, moveable, t);
            record_condition_decision(&mut hour, &section.condition, included, &section.name);
            if !included {
                continue;
            }
        }
        let mut elems = Vec::new();
        for elem in &section.elements {
            if after_prime.reads_martyrology() && section.name == "Martyrology" && elem.reference == MARTYROLOGY_RUBRIC {
                elems.extend(resolve_prime_martyrology(day, t));
                continue;
            }
            if chapter_office && section.name == "Closing" && elem.reference == FAITHFUL_DEPARTED {
                continue;
            }
            if elem.reference == CHAPTER_OFFICE_LESSON {
                elems.push(resolve_chapter_office_lesson(day, t, moveable));
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

/// The hour definition's Capitular Office sections share this prefix.
const CHAPTER_OFFICE: &str = "Chapter-Office";
const FAITHFUL_DEPARTED: &str = "shared/formulas/faithful-departed";
/// The Short Lesson's slot in the hour definition; the day chooses the text.
const CHAPTER_OFFICE_LESSON: &str = "ordinary/chapter-office/short-lesson-per-annum";

/// Whether a composed Prime says the Capitular Office.
pub fn says_chapter_office(hour: &OfficeHour) -> bool {
    hour.sections.iter().flat_map(|s| &s.elements).any(|e| e.source_ref.starts_with("ordinary/chapter-office/"))
}

/// The Short Lesson said out of Choir (Diurnal pp. 11–12). The Sunday and
/// Ferial Office takes the season's lesson. The Office of a feast or octave
/// (Our Lady's Saturday among them), the Vigils of Christmas, the Epiphany
/// and Pentecost, the Sundays within the Octaves of Christmas, the Epiphany,
/// the Ascension and Corpus Christi, and the Friday after the Octave of the
/// Ascension take None's chapter, with *But thou, O Lord*. The book's seasons
/// leave out only Christmas Eve to January 13, whose Sundays and ferias fall
/// within those octaves or keep Christmastide's feasts; a feria left there
/// takes None's chapter too.
fn resolve_chapter_office_lesson(day: &Day, t: &OfficeTexts, moveable: Option<&MoveableDates>) -> OfficeElement {
    let computed;
    let m = match moveable {
        Some(m) => m,
        None => {
            computed = MoveableDates::compute(day.date.year());
            &computed
        }
    };
    let sunday = day.celebration.as_deref().is_some_and(|c| c.is_category(Category::Sunday));
    let sunday_in_octave =
        sunday && day.within_octave_of.as_deref().is_some_and(|o| matches!(o, "christmas" | "epiphany" | "ascension" | "corpus-christi"));
    let own_lesson = (day.is_ferial() || sunday)
        && !sunday_in_octave
        && !day.celebration.as_deref().is_some_and(calendar::traits::is_privileged_vigil)
        && !day.celebration_is("friday-after-ascension-octave");
    let seasonal = own_lesson.then(|| chapter_office_season(day.date, m)).flatten();
    if let Some(season) = seasonal {
        let key = format!("ordinary/chapter-office/short-lesson-{season}");
        let mut e = resolve_element(&HourElement::new("chapter", &key), t);
        e.slot_ref = CHAPTER_OFFICE_LESSON.to_string();
        return e;
    }
    const ENDING: &str = "ordinary/chapter-office/short-lesson-ending";
    let mut e = crate::engine::resolve_hour_element(day, "none", &HourElement::new("proper-chapter", "chapter"), t);
    let body = e.text.trim_end();
    let body = body.strip_suffix("R. Thanks be to God.").unwrap_or(body).trim_end();
    e.text = format!("{body}\n{}", t.get(ENDING).trim());
    e.slot_ref = CHAPTER_OFFICE_LESSON.to_string();
    e.source_refs = compact_refs(e.source_refs.iter().cloned().chain([ENDING.to_string()]).collect());
    e
}

/// The season whose Short Lesson the Sunday and Ferial Office takes on `date`
/// (Diurnal pp. 11–12), named as its corpus key ends.
fn chapter_office_season(date: Date, m: &MoveableDates) -> Option<&'static str> {
    let christmas_eve = Date::new(date.year(), 12, 24);
    if date >= m.advent1 && date < christmas_eve {
        Some("advent")
    } else if date >= m.lent1 && date < m.passion_sunday {
        Some("lent")
    } else if date >= m.passion_sunday && date < m.holy_thursday {
        Some("passiontide")
    } else if date >= m.easter && date < m.ascension {
        Some("paschaltide")
    } else if (date >= Date::new(date.year(), 1, 14) && date < m.lent1)
        || (date > m.trinity_sunday && date < m.corpus_christi)
        || (date > m.corpus_christi.add_days(7) && date < m.advent1)
    {
        Some("per-annum")
    } else {
        None
    }
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
    let reference = martyrology_entry_key(next);
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
    let mut elements = vec![sourced(
        ElementType::Heading,
        format!("Martyrology for tomorrow, {} {}", MONTH_NAMES[next.month() as usize - 1], next.day()),
        &reference,
    )];
    // Announcements the AWRV keeps on this date come first.
    for key in martyrology_announcement_keys(next) {
        let announcement = t.get(&key);
        if !announcement.is_empty() {
            elements.push(sourced(ElementType::Reading, announcement.to_string(), &key));
        }
    }
    elements.extend([
        sourced(ElementType::Reading, text.to_string(), &reference),
        sourced(ElementType::Reading, conclusion.to_string(), CONCLUSION),
        sourced(ElementType::Response, format!("R. {response}"), RESPONSE),
    ]);
    elements
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

/// Moveable announcements and vigils read on `date`, in order. Holy Name
/// follows the calendar's rule (the Sunday of January 2–5, else January 2);
/// every ordo since 2021 has it read "in the 1st place". A vigil is read on
/// the day the calendar keeps it: printed the day before its feast, moved to
/// Saturday from Sunday (except the Epiphany's, which keeps January 5), and,
/// for St Matthias in a leap year, to February 24 (1916 ed., p. 14).
fn martyrology_announcement_keys(date: Date) -> Vec<String> {
    let mut keys = Vec::new();
    let jan2 = Date::new(date.year(), 1, 2);
    let holy_name = (0..=3).map(|i| jan2.add_days(i)).find(|d| d.weekday() == Weekday::Sunday).unwrap_or(jan2);
    if date == holy_name {
        keys.push("ordinary/martyrology/holy-name".to_string());
    }
    // In a leap year the printed 23rd moves to the 24th, so nothing is read
    // from it on the 23rd.
    let printed_vigil = |d: Date| match (d.month(), d.day()) {
        (2, 23) if is_leap_year(d.year()) => None,
        (2, day @ 24..=29) if is_leap_year(d.year()) => Some(format!("ordinary/martyrology/vigil-02-{:02}", day - 1)),
        (month, day) => Some(format!("ordinary/martyrology/vigil-{month:02}-{day:02}")),
    };
    const KEPT_ON_SUNDAY: [&str; 1] = ["ordinary/martyrology/vigil-01-05"];
    if let Some(own) = printed_vigil(date)
        && (date.weekday() != Weekday::Sunday || KEPT_ON_SUNDAY.contains(&own.as_str()))
    {
        keys.push(own);
    }
    if date.weekday() == Weekday::Saturday
        && let Some(sunday) = printed_vigil(date.add_days(1))
        && !KEPT_ON_SUNDAY.contains(&sunday.as_str())
    {
        keys.push(sunday);
    }
    keys
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
        rank: day.celebration.as_deref().map(|c| c.rank),
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
            let h = compose_prime(&d, &sections, &t, Some(&MoveableDates::compute(current.year())), AfterPrime::Martyrology);
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
            let plain = compose_prime(&d, &sections, &t, None, AfterPrime::Rubric);
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
            let h = compose_prime(&d, &sections, &t, Some(&MoveableDates::compute(2026)), AfterPrime::ChapterOffice);
            assert!(!h.sections.iter().flat_map(|s| &s.elements).any(|e| e.source_ref.contains("martyrology")));
        }
    }
}
