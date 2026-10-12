//! The ordo-relevant digest of a composed hour: preces, suffrage, commemorations, and the
//! gospel-canticle antiphon. Shared by the ordo, the calendar view, and `office rubrics`,
//! and by the web's and the native apps' ordo rows through `ordo_day` (which
//! `presentation::day_name` names).

use calendar::{Color, MoveableDates};
use liturgy::{ElementType, OfficeHour, PrayerForm};

use crate::concurrence::VespersOwner;
use crate::day::Day;
use crate::engine::Engine;

/// One commemoration: its name and its antiphon's incipit.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct CommSummary {
    pub name: String,
    pub incipit: String,
}

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct HourSummary {
    pub color: Option<Color>,
    /// The Benedictus/Magnificat antiphon as an incipit.
    pub gospel_ant: String,
    /// The whole antiphon on one line.
    pub gospel_ant_full: String,
    pub preces: bool,
    pub suffrage: bool,
    pub comms: Vec<CommSummary>,
}

pub fn summarize_hour(hour: &OfficeHour) -> HourSummary {
    let mut s = HourSummary {
        color: hour.color,
        gospel_ant: String::new(),
        gospel_ant_full: String::new(),
        preces: false,
        suffrage: false,
        comms: Vec::new(),
    };
    for sec in &hour.sections {
        if sec.label.contains("Suffrage") {
            s.suffrage = true;
        }
        for (i, el) in sec.elements.iter().enumerate() {
            if el.kind == ElementType::Preces {
                s.preces = true;
            } else if el.kind == ElementType::Heading
                && let Some(name) = el.text.strip_prefix("Commemoration of ")
            {
                let mut c = CommSummary { name: name.to_string(), incipit: String::new() };
                for next in &sec.elements[i + 1..] {
                    if next.kind == ElementType::Antiphon {
                        c.incipit = incipit(&next.text);
                        break;
                    }
                    if next.kind == ElementType::Heading {
                        break;
                    }
                }
                s.comms.push(c);
            } else if s.gospel_ant.is_empty() && (el.slot_ref == "benedictus-antiphon" || el.slot_ref == "magnificat-antiphon") {
                s.gospel_ant = incipit(&el.text);
                s.gospel_ant_full = el.text.replace('\n', " ");
            } else if s.gospel_ant.is_empty()
                && el.kind == ElementType::Canticle
                && (el.source_ref == "canticles/benedictus" || el.source_ref == "canticles/magnificat")
                && i > 0
                && sec.elements[i - 1].kind == ElementType::Antiphon
            {
                let antiphon = &sec.elements[i - 1].text;
                s.gospel_ant = incipit(antiphon);
                s.gospel_ant_full = antiphon.replace('\n', " ");
            }
        }
    }
    s
}

/// One day of the ordo: the calendar's facts and the digest of its composed
/// Lauds, Hours (Prime stands for the minor hours' shared preces), and Vespers.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct OrdoDay {
    /// The rank's abbreviation ("2cl") and full name; both empty without a celebration.
    pub rank: String,
    pub rank_full: String,
    pub color: Color,
    pub fast: bool,
    pub abstinence: bool,
    pub commemorations: Vec<String>,
    pub lauds: Option<HourSummary>,
    pub hours_preces: bool,
    pub vespers: Option<HourSummary>,
    /// "II Vespers of preceding", "I Vespers of …", or empty.
    pub vespers_note: String,
}

pub fn ordo_day(day: &Day, engine: &Engine, moveable: &MoveableDates) -> OrdoDay {
    let summarize = |hour: &str| engine.compose_hour(hour, day, moveable, PrayerForm::Private).ok().map(|h| summarize_hour(&h));
    let (rank, rank_full) = match &day.celebration {
        Some(c) => (c.rank.abbrev().to_string(), c.rank.display_name().to_string()),
        None => (String::new(), String::new()),
    };
    OrdoDay {
        rank,
        rank_full,
        color: day.color,
        fast: day.penitential.fast,
        abstinence: day.penitential.abstinence,
        // Named as the hours' headings name them ("St Paul", not "Commemoration of St Paul"),
        // so a day's commemoration can be matched to the one its Lauds or Vespers carries.
        commemorations: day.commemorations.iter().map(|c| c.commemoration_name().to_string()).collect(),
        lauds: summarize("lauds"),
        hours_preces: summarize("prime").is_some_and(|h| h.preces),
        vespers: summarize("vespers"),
        vespers_note: match day.vespers.owner {
            VespersOwner::IIOfPreceding => "II Vespers of preceding".to_string(),
            VespersOwner::IOfFollowing => day.vespers.feast.as_ref().map(|f| format!("I Vespers of {}", f.name)).unwrap_or_default(),
            VespersOwner::NotApplicable => String::new(),
        },
    }
}

/// An antiphon's opening: through the mediant, at most nine words, without
/// trailing punctuation.
pub fn incipit(text: &str) -> String {
    let s = text.replace('\n', " ");
    let mut s = s.trim();
    if s.is_empty() {
        return String::new();
    }
    if let Some(i) = s.find('*')
        && i > 0
    {
        s = s[..i].trim();
    }
    let mut words: Vec<&str> = s.split_whitespace().collect();
    let truncated = words.len() > 9;
    words.truncate(9);
    let mut out = words.join(" ").trim_end_matches([' ', ',', ';', ':', '.']).to_string();
    if truncated {
        out.push('…');
    }
    out
}

#[cfg(test)]
mod tests {
    use calendar::{CalendarData, MoveableDates, build_calendar};

    use super::*;
    use crate::{resolve_office_days, testutil::TestData};

    #[test]
    fn a_days_commemorations_are_named_as_its_hours_name_them() {
        let data = TestData("../../data".into());
        let engine = Engine::load(&data).unwrap();
        let cal = build_calendar(2026, &CalendarData::load(&data).unwrap()).unwrap();
        let offices = resolve_office_days(&cal);
        let moveable = MoveableDates::compute(2026);
        let (c, o) = cal.days.into_iter().zip(offices).find(|(c, _)| c.date.to_string() == "2026-01-18").unwrap();
        let ordo = ordo_day(&Day::new(c, o), &engine, &moveable);
        // The Second Sunday after Epiphany commemorates St Paul (a feast named "Commemoration of
        // St Paul") at Lauds; the day's list and the hour's heading must agree on the name.
        let lauds: Vec<_> = ordo.lauds.unwrap().comms.into_iter().map(|c| c.name).collect();
        assert!(ordo.commemorations.iter().any(|c| c == "St Paul"), "{:?}", ordo.commemorations);
        assert!(ordo.commemorations.iter().all(|c| !c.starts_with("Commemoration of ")));
        assert!(lauds.iter().any(|c| c == "St Paul"), "{lauds:?}");
    }
}
