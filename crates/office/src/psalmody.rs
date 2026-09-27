//! Psalmody declarations and the Office of the Dead. Ported from Go's
//! `psalmody.go`.

use std::collections::{BTreeSet, HashMap};

use calendar::{Category, Color, Date, Rank, Season};
use compat::{quote, scan_lines};
use liturgy::OfficeElement;

use crate::concurrence::{VespersDesignation, VespersOwner};
use crate::day::Day;
use crate::hourdef::{HourElement, uses_triduum_form};
use crate::proper::{feast_proper_ids, first_text, is_synthesized_feria, lookup_section_text};
use crate::texts::OfficeTexts;

pub const VESPERS_PSALMODY_REF: &str = "vespers-psalmody";
pub const DEFAULT_VESPERS_PSALMODY_KEY: &str = "ordinary/vespers/festal-psalmody";
const FERIAL: &str = "ferial";
pub const FERIAL_WITH_WEEKDAY_ANTIPHONS: &str = "ferial weekday-antiphons";

/// One row of a declaration: an antiphon slot, its text key, the psalm, and
/// optional fixed dates ("MM-DD").
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct PsalmodyItem {
    pub slot: String,
    pub antiphon: String,
    pub psalm: String,
    pub dates: BTreeSet<String>,
}

fn valid_month_day(s: &str) -> bool {
    let b = s.as_bytes();
    if b.len() != 5 || b[2] != b'-' || !b[..2].iter().chain(&b[3..]).all(u8::is_ascii_digit) {
        return false;
    }
    let (m, d) = (s[..2].parse::<i32>().unwrap_or(0), s[3..].parse::<i32>().unwrap_or(0));
    let date = Date::new(2000, m, d);
    (1..=12).contains(&m) && d >= 1 && date.month() as i32 == m && date.day() as i32 == d
}

/// Parses one declaration; `Ok((items, true))` for the "ferial" stop markers.
pub fn parse_psalmody_declaration(body: &str) -> Result<(Vec<PsalmodyItem>, bool), String> {
    let body = body.trim();
    if body == FERIAL || body == FERIAL_WITH_WEEKDAY_ANTIPHONS {
        return Ok((Vec::new(), true));
    }
    if body.is_empty() {
        return Err("declaration is empty".to_string());
    }
    #[derive(Default)]
    struct Seen {
        unconditional: bool,
        dates: BTreeSet<String>,
    }
    let mut items = Vec::new();
    let mut seen: HashMap<String, Seen> = HashMap::new();
    for (i, raw) in scan_lines(body).enumerate() {
        let n = i + 1;
        let line = raw.trim();
        if line.is_empty() {
            continue;
        }
        let (antiphon, rhs) = match line.split_once('=') {
            Some((a, r)) if !a.trim().is_empty() && !r.trim().is_empty() => (a.trim(), r.trim()),
            _ => return Err(format!("line {n}: expected <antiphon-key> = <psalm-key>")),
        };
        let fields: Vec<&str> = rhs.split_whitespace().collect();
        if fields.is_empty() || fields.len() > 3 || fields[0].contains('=') {
            return Err(format!("line {n}: expected <antiphon-key> = <psalm-key> [dates=MM-DD,...] [antiphon=<corpus-key>]"));
        }
        let psalm = fields[0];
        let mut antiphon_ref = antiphon.to_string();
        let mut dates: Option<BTreeSet<String>> = None;
        for option in &fields[1..] {
            if let Some(value) = option.strip_prefix("dates=") {
                if dates.is_some() {
                    return Err(format!("line {n}: duplicate dates option"));
                }
                if value.is_empty() {
                    return Err(format!("line {n}: expected a non-empty dates=MM-DD list"));
                }
                let mut set = BTreeSet::new();
                for date in value.split(',') {
                    if !valid_month_day(date) {
                        return Err(format!("line {n}: invalid declaration date {} (expected MM-DD)", quote(date)));
                    }
                    if !set.insert(date.to_string()) {
                        return Err(format!("line {n}: duplicate declaration date {}", quote(date)));
                    }
                }
                dates = Some(set);
            } else if let Some(value) = option.strip_prefix("antiphon=") {
                if antiphon_ref != antiphon {
                    return Err(format!("line {n}: duplicate antiphon option"));
                }
                antiphon_ref = value.to_string();
                if antiphon_ref.is_empty() {
                    return Err(format!("line {n}: antiphon option requires a corpus key"));
                }
            } else {
                return Err(format!("line {n}: unknown declaration option {}", quote(option)));
            }
        }
        if antiphon.contains([' ', '\t']) || psalm.contains([' ', '\t']) || antiphon_ref.contains([' ', '\t']) {
            return Err(format!("line {n}: corpus keys may not contain whitespace"));
        }
        let dates = dates.unwrap_or_default();
        let exists = seen.contains_key(antiphon);
        let entry = seen.entry(antiphon.to_string()).or_default();
        if dates.is_empty() {
            if exists {
                return Err(format!("line {n}: duplicate antiphon key {}", quote(antiphon)));
            }
            entry.unconditional = true;
        } else {
            if entry.unconditional {
                return Err(format!("line {n}: conditional alternative overlaps unconditional antiphon key {}", quote(antiphon)));
            }
            for date in &dates {
                if !entry.dates.insert(date.clone()) {
                    return Err(format!("line {n}: antiphon key {} repeats date {date}", quote(antiphon)));
                }
            }
        }
        items.push(PsalmodyItem { slot: antiphon.to_string(), antiphon: antiphon_ref, psalm: psalm.to_string(), dates });
    }
    if items.is_empty() {
        return Err("declaration has no psalms".to_string());
    }
    Ok((items, false))
}

/// The rows that apply on `date`: each slot must have exactly one.
pub fn select_psalmody_items(items: Vec<PsalmodyItem>, date: Date) -> Result<Vec<PsalmodyItem>, String> {
    let month_day = format!("{:02}-{:02}", date.month(), date.day());
    let mut expected: Vec<String> = Vec::new();
    let mut counts: HashMap<String, usize> = HashMap::new();
    let mut selected = Vec::new();
    for item in items {
        if !expected.contains(&item.slot) {
            expected.push(item.slot.clone());
        }
        if !item.dates.is_empty() && !item.dates.contains(&month_day) {
            continue;
        }
        *counts.entry(item.slot.clone()).or_default() += 1;
        selected.push(item);
    }
    // PORT(inherited): Go reports the first failing slot in map order; this
    // reports the first in declaration order.
    for slot in &expected {
        match counts.get(slot).copied().unwrap_or(0) {
            0 => return Err(format!("antiphon key {} has no alternative for date {month_day}", quote(slot))),
            1 => {}
            _ => return Err(format!("antiphon key {} has multiple alternatives for date {month_day}", quote(slot))),
        }
    }
    Ok(selected)
}

fn vespers_psalmody_candidates(day: &Day) -> Vec<String> {
    if day.first_vespers {
        vec![format!("{VESPERS_PSALMODY_REF}-first"), VESPERS_PSALMODY_REF.to_string()]
    } else {
        vec![VESPERS_PSALMODY_REF.to_string()]
    }
}

fn has_feast_proper_vespers_psalm_antiphons(day: &Day, t: &OfficeTexts) -> bool {
    let Some(c) = day.celebration.as_deref() else { return false };
    for feast_id in feast_proper_ids(c) {
        if day.season == Season::Easter
            && !lookup_section_text(&format!("proper/{feast_id}-paschal/"), None, "vespers", "psalm-antiphon-1", t).0.is_empty()
        {
            return true;
        }
        if !lookup_section_text(&format!("proper/{feast_id}/"), Some(day.season), "vespers", "psalm-antiphon-1", t).0.is_empty() {
            return true;
        }
    }
    false
}

/// The Vespers psalmody declaration body and its source key.
pub fn lookup_vespers_psalmody(day: &Day, t: &OfficeTexts) -> (String, String) {
    let Some(c) = day.celebration.as_deref() else { return (String::new(), String::new()) };
    if is_synthesized_feria(c) {
        return (String::new(), String::new());
    }
    let refs = vespers_psalmody_candidates(day);
    for feast_id in feast_proper_ids(c) {
        if day.season == Season::Easter {
            let found = first_text(t, &format!("proper/{feast_id}-paschal/"), &refs);
            if !found.0.is_empty() {
                return found;
            }
        }
        let found = first_text(t, &format!("proper/{feast_id}/"), &refs);
        if !found.0.is_empty() {
            return found;
        }
    }
    // A plain Double or a Simple without its own psalmody takes the weekday
    // psalter (General Rubrics XX.1, XXV.4, III.1), outside octaves.
    if matches!(c.rank, Rank::Double | Rank::Simple) && day.within_octave_of.is_none() && !has_feast_proper_vespers_psalm_antiphons(day, t)
    {
        return (FERIAL_WITH_WEEKDAY_ANTIPHONS.to_string(), "rubric/plain-double-ferial-vespers".to_string());
    }
    if let Some(category) = c.category {
        if day.season == Season::Easter {
            let found = first_text(t, &format!("commons/{category}-paschal/"), &refs);
            if !found.0.is_empty() {
                return found;
            }
        }
        let found = first_text(t, &format!("commons/{category}/"), &refs);
        if !found.0.is_empty() {
            return found;
        }
    }
    match c.category {
        None | Some(Category::Feria | Category::Sunday) => (String::new(), String::new()),
        Some(_) => (t.get(DEFAULT_VESPERS_PSALMODY_KEY).to_string(), DEFAULT_VESPERS_PSALMODY_KEY.to_string()),
    }
}

/// The festal Vespers psalmody, empty for the weekday psalter.
pub fn resolve_vespers_psalmody(day: &Day, t: &OfficeTexts) -> Result<(Vec<PsalmodyItem>, String), String> {
    let (body, source) = lookup_vespers_psalmody(day, t);
    if body.is_empty() {
        if !source.is_empty() {
            return Err(format!("vespers psalmody declaration {} not found", quote(&source)));
        }
        return Ok((Vec::new(), source));
    }
    let (items, ferial) =
        parse_psalmody_declaration(&body).map_err(|e| format!("invalid vespers psalmody declaration {}: {e}", quote(&source)))?;
    if ferial {
        return Ok((Vec::new(), source));
    }
    // Fixed-date appointments follow the civil evening.
    let date = if day.first_vespers { day.date.add_days(-1) } else { day.date };
    let items = select_psalmody_items(items, date).map_err(|e| format!("invalid vespers psalmody declaration {}: {e}", quote(&source)))?;
    Ok((items, source))
}

/// Whether the day's office is the Office of the Dead.
pub fn is_office_of_the_dead(day: &Day) -> bool {
    day.celebration_is("all-souls")
}

/// The heading of Vespers of the Dead appended on the eve of All Souls.
pub const VESPERS_OF_THE_DEAD_LABEL: &str = "Vespers of the Dead";

/// The synthetic office of Vespers and Compline of the Dead on the eve of All
/// Souls: black, without the civil day's commemorations.
pub fn dead_office_day(day: &Day) -> Day {
    let Some(feast) = day.vespers.appended_feast.clone() else { return day.clone() };
    let mut out = day.clone();
    out.celebration = Some(feast.clone());
    out.color = Color::Black;
    out.commemorations = Vec::new();
    out.feria_commemoration = None;
    out.first_vespers = false;
    out.following_office_commemoration_id = String::new();
    out.vespers = VespersDesignation {
        owner: VespersOwner::IIOfPreceding,
        feast: Some(feast.clone()),
        color: Some(Color::Black),
        season: Some(day.season),
        within_octave_of: None,
        rule: "vespers:appended-office-of-the-dead".to_string(),
        decisions: Vec::new(),
        commemorations: Vec::new(),
        following_office_commemoration_id: None,
        following_office_octave_of: None,
        psalmody_from_preceding: false,
        appended_office_of_the_dead: true,
        appended_feast: Some(feast),
    };
    out
}

pub const DOXOLOGY_GLORIA_PATRI: &str = "ordinary/shared/gloria-patri";
pub const DOXOLOGY_REST_ETERNAL: &str = "shared/formulas/rest-eternal";
/// The Ref by which an hour definition defers the doxology to the office.
pub const DOXOLOGY_REF_PER_OFFICE: &str = "office";

/// What concludes each psalm: "Rest eternal" in the Office of the Dead.
pub fn psalm_doxology_ref(day: &Day) -> &'static str {
    if is_office_of_the_dead(day) { DOXOLOGY_REST_ETERNAL } else { DOXOLOGY_GLORIA_PATRI }
}

/// Nothing concludes the psalms during the Triduum (Diurnal pp. 311, 314).
pub fn says_psalm_doxology(day: &Day, hour_name: &str) -> bool {
    !uses_triduum_form(day, hour_name)
}

pub fn uses_festal_vespers_psalmody(day: &Day, t: &OfficeTexts) -> bool {
    resolve_vespers_psalmody(day, t).is_ok_and(|(items, _)| !items.is_empty())
}

pub fn uses_weekday_vespers_antiphons(day: &Day, t: &OfficeTexts) -> bool {
    lookup_vespers_psalmody(day, t).0.trim() == FERIAL_WITH_WEEKDAY_ANTIPHONS
}

/// Antiphon, psalm, doxology, antiphon for each row.
pub fn compose_resolved_psalmody(day: &Day, hour_name: &str, items: &[PsalmodyItem], t: &OfficeTexts) -> Vec<OfficeElement> {
    let mut elems = Vec::with_capacity(items.len() * 4);
    for item in items {
        let antiphon = crate::engine::resolve_hour_element(day, hour_name, &HourElement::new("proper-antiphon", &item.antiphon), t);
        let kind = if item.psalm.starts_with("canticles/") { "canticle" } else { "psalm" };
        elems.push(antiphon.clone());
        elems.push(crate::engine::resolve_element(&HourElement::new(kind, &item.psalm), t));
        if says_psalm_doxology(day, hour_name) {
            elems.push(crate::engine::resolve_element(&HourElement::new("gloria-patri", psalm_doxology_ref(day)), t));
        }
        elems.push(antiphon);
    }
    elems
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn declarations() {
        let (items, ferial) = parse_psalmody_declaration("psalm-antiphon-1 = psalms/110\npsalm-antiphon-2 = psalms/130 dates=12-25,12-27 antiphon=x/y\npsalm-antiphon-2 = psalms/131 dates=12-26\n").unwrap();
        assert!(!ferial);
        assert_eq!(items.len(), 3);
        assert_eq!(items[1].antiphon, "x/y");
        assert_eq!(parse_psalmody_declaration("ferial weekday-antiphons").unwrap(), (Vec::new(), true));
        let selected = select_psalmody_items(items.clone(), Date::new(2026, 12, 26)).unwrap();
        assert_eq!(selected.iter().map(|i| i.psalm.as_str()).collect::<Vec<_>>(), ["psalms/110", "psalms/131"]);
        assert_eq!(
            select_psalmody_items(items, Date::new(2026, 12, 28)).unwrap_err(),
            "antiphon key \"psalm-antiphon-2\" has no alternative for date 12-28"
        );
    }

    #[test]
    fn declaration_errors() {
        for (body, want) in [
            ("", "declaration is empty"),
            ("a", "line 1: expected <antiphon-key> = <psalm-key>"),
            ("a = b c d e", "line 1: expected <antiphon-key> = <psalm-key> [dates=MM-DD,...] [antiphon=<corpus-key>]"),
            ("a = b dates=02-30", "line 1: invalid declaration date \"02-30\" (expected MM-DD)"),
            ("a = b dates=2-3", "line 1: invalid declaration date \"2-3\" (expected MM-DD)"),
            ("a = b dates=01-01,01-01", "line 1: duplicate declaration date \"01-01\""),
            ("a = b dates=01-01 dates=01-02", "line 1: duplicate dates option"),
            ("a = b antiphon=x antiphon=y", "line 1: duplicate antiphon option"),
            ("a = b bogus=1", "line 1: unknown declaration option \"bogus=1\""),
            ("a = b\na = c", "line 2: duplicate antiphon key \"a\""),
            ("a = b\na = c dates=01-01", "line 2: conditional alternative overlaps unconditional antiphon key \"a\""),
            ("a = b dates=01-01\na = c dates=01-01", "line 2: antiphon key \"a\" repeats date 01-01"),
        ] {
            assert_eq!(parse_psalmody_declaration(body).unwrap_err(), want, "{body}");
        }
    }
}
