//! Compline.

use calendar::MoveableDates;
use liturgy::{OfficeHour, OfficeSection};

use crate::day::Day;
use crate::engine::append_hour_element;
use crate::hourdef::HourSection;
use crate::major::record_condition_decision;
use crate::prime::new_hour;
use crate::psalmody::dead_office_day;
use crate::texts::OfficeTexts;
use crate::vespers::vespers_office_day;

pub fn compose_compline(day: &Day, sections: &[HourSection], t: &OfficeTexts, moveable: Option<&MoveableDates>) -> OfficeHour {
    let office_day = compline_office_day(day);
    let mut hour = new_hour("Compline", &office_day);
    hour.date = day.date;
    for section in sections {
        let section_day =
            if day.celebration_is("holy-saturday") && section.condition.contains("feast-holy-saturday") { day } else { &office_day };
        if !section.condition.is_empty() {
            let included = section.condition_holds(section_day, moveable, t);
            record_condition_decision(&mut hour, &section.condition, included, &section.name);
            if !included {
                continue;
            }
        }
        let mut elems = Vec::new();
        for elem in &section.elements {
            append_hour_element(&mut elems, section_day, "compline", elem, t);
        }
        hour.sections.push(OfficeSection { label: section.label.clone(), collapsible: section.collapsible, elements: elems });
    }
    hour
}

/// The office that owns the evening: the Dead on the eve of All Souls,
/// otherwise the owner of Vespers.
pub fn compline_office_day(day: &Day) -> Day {
    if day.vespers.appended_office_of_the_dead { dead_office_day(day) } else { vespers_office_day(day) }
}

/// The display label of a Marian antiphon key.
pub fn marian_label(key: &str) -> &'static str {
    match key {
        "alma-redemptoris-advent" | "alma-redemptoris-christmas" => "Alma Redemptoris Mater",
        "ave-regina-caelorum" => "Ave Regina Caelorum",
        "regina-caeli" => "Regina Caeli",
        "salve-regina" => "Salve Regina",
        _ => "",
    }
}
