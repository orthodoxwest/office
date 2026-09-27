//! Vespers: the owner of the evening, the psalmody day, and Vespers of the
//! Dead appended on the eve of All Souls. Ported from Go's `vespers.go`.

use calendar::{Color, Decision, MoveableDates};
use liturgy::{OfficeHour, OfficeSection};

use crate::concurrence::VespersOwner;
use crate::day::Day;
use crate::engine::resolve_element;
use crate::hourdef::{HourElement, HourSection};
use crate::major::{MajorHourOptions, compose_major_hour};
use crate::psalmody::{VESPERS_OF_THE_DEAD_LABEL, dead_office_day, resolve_vespers_psalmody};
use crate::texts::OfficeTexts;

/// The civil day selects the abbreviated Holy Saturday form.
pub fn is_holy_saturday_vespers(day: &Day) -> bool {
    day.celebration_is("holy-saturday")
}

fn vespers_options() -> MajorHourOptions {
    MajorHourOptions { hour_name: "vespers", title: "Vespers", office_day: vespers_office_day, psalmody_day: vespers_psalmody_day }
}

pub fn compose_vespers(
    day: &Day,
    sections: &[HourSection],
    t: &OfficeTexts,
    moveable: Option<&MoveableDates>,
) -> Result<OfficeHour, String> {
    resolve_vespers_psalmody(&vespers_psalmody_day(day), t)?;
    let mut hour = compose_major_hour(day, sections, t, moveable, &vespers_options())?;
    if day.celebration_is("holy-thursday") {
        hour.color = Some(Color::Violet); // 2026 ordo p. 53
    }
    if day.vespers.appended_office_of_the_dead {
        append_vespers_of_the_dead(&mut hour, day, sections, t, moveable)?;
    }
    Ok(hour)
}

fn append_vespers_of_the_dead(
    hour: &mut OfficeHour,
    day: &Day,
    sections: &[HourSection],
    t: &OfficeTexts,
    moveable: Option<&MoveableDates>,
) -> Result<(), String> {
    let dead = compose_major_hour(&dead_office_day(day), sections, t, moveable, &vespers_options())?;
    let rubric_ref = if day.celebration_is("all-saints") {
        "shared/formulas/appended-vespers-of-the-dead-rubric"
    } else {
        "shared/formulas/appended-vespers-of-the-dead-rubric-optional"
    };
    let rubric = resolve_element(&HourElement::new("rubric", rubric_ref), t);
    hour.sections.push(OfficeSection { label: VESPERS_OF_THE_DEAD_LABEL.to_string(), collapsible: false, elements: vec![rubric] });
    hour.sections.extend(dead.sections);
    hour.decisions.extend(dead.decisions);
    hour.decisions.push(Decision::new("vespers:appended-office-of-the-dead", "included", "all-souls"));
    Ok(())
}

/// The office supplying the psalms: the outgoing office when the incoming
/// Simple begins only at the Chapter (General Rubrics III).
pub fn vespers_psalmody_day(day: &Day) -> Day {
    if !day.vespers.psalmody_from_preceding {
        return vespers_office_day(day);
    }
    let mut d = day.clone();
    d.commemorations = Vec::new();
    d
}

/// The office that owns the evening's Vespers.
pub fn vespers_office_day(day: &Day) -> Day {
    let mut office_day = day.clone();
    let v = &day.vespers;
    let Some(feast) = v.feast.clone().filter(|_| v.owner != VespersOwner::NotApplicable) else {
        // No adjacent celebration owns Vespers: today's office with
        // tomorrow's occurrence commemorations (XIV.9). Rogation and
        // September Ember propers end at None (XIII.18).
        office_day.commemorations = v.commemorations.clone();
        if matches!(
            day.celebration_id(),
            Some("rogation-monday" | "september-ember-wednesday" | "september-ember-friday" | "september-ember-saturday")
        ) {
            office_day.celebration = None;
            office_day.color = day.season.color();
        }
        return office_day;
    };
    office_day.celebration = Some(feast);
    // PORT(inherited): an owned evening always carries a colour.
    office_day.color = v.color.unwrap_or(day.color);
    if let Some(s) = v.season {
        office_day.season = s;
    }
    match v.owner {
        VespersOwner::IOfFollowing => {
            office_day.date = day.date.add_days(1);
            office_day.commemorations = v.commemorations.clone();
            office_day.tempora = None;
            office_day.within_octave_of = v.within_octave_of.clone();
            office_day.first_vespers = true;
        }
        VespersOwner::IIOfPreceding => {
            office_day.commemorations = v.commemorations.clone();
            office_day.following_office_commemoration_id = v.following_office_commemoration_id.clone().unwrap_or_default();
        }
        VespersOwner::NotApplicable => {}
    }
    office_day
}
