//! The shared composer of Lauds and Vespers.

use calendar::{Decision, MoveableDates};
use liturgy::{OfficeHour, OfficeSection, PrayerForm};

use crate::commemoration::add_commemorations;
use crate::day::Day;
use crate::engine::append_hour_element;
use crate::hourdef::HourSection;
use crate::lauds_psalmody::resolve_hour_psalmody;
use crate::psalmody::compose_resolved_psalmody;
use crate::texts::OfficeTexts;

pub struct MajorHourOptions {
    pub hour_name: &'static str,
    pub title: &'static str,
    /// The office that owns the hour (Vespers: the owner of the evening).
    pub office_day: fn(&Day) -> Day,
    /// The office supplying the psalmody, when split at the Chapter.
    pub psalmody_day: fn(&Day) -> Day,
}

impl MajorHourOptions {
    pub fn lauds() -> MajorHourOptions {
        MajorHourOptions { hour_name: "lauds", title: "Lauds", office_day: Day::clone, psalmody_day: Day::clone }
    }
}

pub fn compose_major_hour(
    day: &Day,
    sections: &[HourSection],
    t: &OfficeTexts,
    moveable: Option<&MoveableDates>,
    opts: &MajorHourOptions,
) -> Result<OfficeHour, String> {
    let office_day = (opts.office_day)(day);
    let psalmody_day = (opts.psalmody_day)(day);
    let mut hour = OfficeHour {
        form: PrayerForm::Private,
        date: day.date,
        hour: opts.title.to_string(),
        title: opts.title.to_string(),
        season: Some(office_day.season),
        feast: office_day.celebration.as_deref().map(|c| c.name.clone()).unwrap_or_default(),
        color: Some(office_day.color),
        sections: Vec::new(),
        decisions: Vec::new(),
    };
    // Which sections are said must be known up front: whether a
    // commemoration's collect is the last of the run depends on them.
    let included: Vec<bool> = sections.iter().map(|s| s.condition.is_empty() || s.condition_holds(&office_day, moveable, t)).collect();
    for (i, section) in sections.iter().enumerate() {
        if !section.condition.is_empty() {
            record_condition_decision(&mut hour, &section.condition, included[i], &section.name);
            if !included[i] {
                continue;
            }
        }
        let mut elems = Vec::new();
        for elem in &section.elements {
            match elem.kind.as_str() {
                "commemorations" => {
                    let more = collect_follows(sections, &included, i);
                    elems.extend(add_commemorations(&office_day, opts.hour_name, t, more));
                }
                "proper-psalmody" => {
                    let items = resolve_hour_psalmody(&psalmody_day, opts.hour_name, &elem.reference, t)?;
                    elems.extend(compose_resolved_psalmody(&psalmody_day, opts.hour_name, &items, t));
                }
                _ => append_hour_element(&mut elems, &office_day, opts.hour_name, elem, t),
            }
        }
        hour.sections.push(OfficeSection { label: section.label.clone(), collapsible: section.collapsible, elements: elems });
    }
    Ok(hour)
}

/// Whether a further collect of the hour's run follows section `after`: the
/// run ends at "The Lord be with you" (XXXIII.3,5; Diurnal p. 144).
pub(crate) fn collect_follows(sections: &[HourSection], included: &[bool], after: usize) -> bool {
    for (i, section) in sections.iter().enumerate().skip(after + 1) {
        if !included[i] {
            continue;
        }
        if section.has_element_type(&["blessing"]) || section.has_element_type(&["officiant-greeting"]) {
            return false;
        }
        if section.has_element_type(&["collect", "proper-collect"]) {
            return true;
        }
    }
    false
}

pub fn record_condition_decision(hour: &mut OfficeHour, condition: &str, included: bool, section: &str) {
    hour.decisions.push(Decision::new(format!("condition:{condition}"), if included { "included" } else { "omitted" }, section));
}
