//! Lauds psalmody: the weekday psalter on lesser Doubles and Simples, the festal weekday canticle,
//! and declared psalmody.

use calendar::{Category, Rank, Season, Weekday};
use data_format::quote;

use crate::day::Day;
use crate::preces::SATURDAY_OFFICE_BVM_ID;
use crate::proper::{feast_proper_ids, first_text, lookup_seasonal_text, lookup_section_text};
use crate::psalmody::{PsalmodyItem, VESPERS_PSALMODY_REF, parse_psalmody_declaration, resolve_vespers_psalmody, select_psalmody_items};
use crate::texts::OfficeTexts;

pub const LAUDS_PSALMODY_REF: &str = "lauds-psalmody";
pub const LAUDS_LAUDATE_PSALMODY_REF: &str = "lauds-laudate-psalmody";

/// Lesser Doubles and Simples without proper psalm antiphons keep the weekday
/// psalter (Diurnal pp.44,46).
pub fn uses_weekday_lauds_psalmody(day: &Day, t: &OfficeTexts) -> bool {
    let Some(feast) = day.celebration.as_deref() else { return false };
    if feast.id == SATURDAY_OFFICE_BVM_ID || feast.is_category(Category::Feria) {
        return false;
    }
    let eligible = feast.rank == Rank::Double
        || feast.rank == Rank::Simple
        || (feast.is_category(Category::Sunday) && day.civil_weekday() == Weekday::Saturday);
    if !eligible {
        return false;
    }
    let season = day.saints_season("lauds");
    for id in feast_proper_ids(feast) {
        let mut prefixes = vec![format!("proper/{id}/")];
        if season == Season::Easter {
            prefixes.insert(0, format!("proper/{id}-paschal/"));
        }
        for prefix in prefixes {
            if !t.get(&format!("{prefix}{LAUDS_PSALMODY_REF}")).is_empty() {
                return false;
            }
            let (text, _) = lookup_section_text(&prefix, Some(season), "lauds", "psalm-antiphon-1", t);
            if !text.is_empty() && !corpus::is_omitted(&text) {
                return false;
            }
        }
    }
    true
}

pub fn uses_festal_weekday_lauds_canticle(day: &Day, t: &OfficeTexts) -> bool {
    if !uses_weekday_lauds_psalmody(day, t) || day.civil_weekday() == Weekday::Sunday {
        return false;
    }
    let f = day.celebration.as_deref().expect("weekday Lauds psalmody implies a celebration");
    f.rank == Rank::Double || f.id.contains("octave-day") || f.is_category(Category::Sunday)
}

/// The weekday psalter's antiphon at Lauds itself.
pub fn weekday_lauds_antiphon(day: &Day, reference: &str, t: &OfficeTexts) -> (String, String) {
    let weekday = day.civil_weekday_name();
    let canticle_slot = if day.civil_weekday() == Weekday::Saturday { "psalm-antiphon-3" } else { "psalm-antiphon-4" };
    if reference == canticle_slot && uses_festal_weekday_lauds_canticle(day, t) {
        let mut key = format!("ordinary/lauds/festal-canticle-antiphon-{weekday}");
        if day.season == Season::Easter {
            key.push_str("-easter");
        }
        return (t.get(&key).to_string(), key);
    }
    if day.season == Season::Easter {
        return lookup_seasonal_text(day, "lauds", reference, t);
    }
    let key = format!("ordinary/lauds/{reference}-{weekday}");
    (t.get(&key).to_string(), key)
}

/// A proper's declared Lauds psalmody (or its Laudate).
pub fn lookup_lauds_psalmody(day: &Day, reference: &str, t: &OfficeTexts) -> (String, String) {
    let Some(c) = day.celebration.as_deref() else { return (String::new(), String::new()) };
    for id in feast_proper_ids(c) {
        if day.season == Season::Easter {
            let found = first_text(t, &format!("proper/{id}-paschal/"), &[reference]);
            if !found.0.is_empty() {
                return found;
            }
        }
        let found = first_text(t, &format!("proper/{id}/"), &[reference]);
        if !found.0.is_empty() {
            return found;
        }
    }
    (String::new(), String::new())
}

pub fn uses_declared_lauds_psalmody(day: &Day, t: &OfficeTexts) -> bool {
    let (body, _) = lookup_lauds_psalmody(day, LAUDS_PSALMODY_REF, t);
    !body.is_empty() && body != "festal"
}

pub fn valid_hour_psalmody_ref(hour: &str, reference: &str) -> bool {
    (hour == "vespers" && reference == VESPERS_PSALMODY_REF)
        || (hour == "lauds" && (reference == LAUDS_PSALMODY_REF || reference == LAUDS_LAUDATE_PSALMODY_REF))
}

/// The psalmody named by a `proper-psalmody` element.
pub fn resolve_hour_psalmody(day: &Day, hour: &str, reference: &str, t: &OfficeTexts) -> Result<Vec<PsalmodyItem>, String> {
    if !valid_hour_psalmody_ref(hour, reference) {
        return Err(format!("unsupported {hour} psalmody ref {}", quote(reference)));
    }
    if hour == "vespers" {
        return resolve_vespers_psalmody(day, t).map(|(items, _)| items);
    }
    let (body, source) = lookup_lauds_psalmody(day, reference, t);
    let (items, ferial) = parse_psalmody_declaration(&body).map_err(|e| format!("invalid Lauds psalmody {}: {e}", quote(reference)))?;
    if ferial {
        return Err(format!("lauds psalmody {} does not support ferial markers", quote(&source)));
    }
    select_psalmody_items(items, day.date)
}
