//! Commemorations at Lauds and Vespers: heading, antiphon, versicle, invitation, collect.

use calendar::commemoration::lauds_commemorations;
use calendar::traits::{is_day_within_octave, octave_parent_id};
use calendar::{Category, Feast, FeastRef, Rank, Season, Weekday};
use liturgy::{ElementType, OfficeElement};

use crate::conclusion::apply_conclusion;
use crate::concurrence::VespersOwner;
use crate::day::Day;
use crate::engine::{compact_refs, resolve_element};
use crate::hourdef::HourElement;
use crate::proper::{
    advent_date_commemoration_antiphon, feast_proper_ids, feast_proper_name, is_synthesized_feria, lookup_commons_text,
    lookup_section_text, resolve_proper_text, substitute_proper_name,
};
use crate::psalmody::is_office_of_the_dead;
use crate::texts::OfficeTexts;

/// The commemoration elements for each commemorated feast. `more_collects`
/// says a Suffrage or Commemoration of the Cross follows in the same run of
/// collects, so none of these is the last (XXXIII.5).
pub fn add_commemorations(day: &Day, hour_name: &str, t: &OfficeTexts, more_collects: bool) -> Vec<OfficeElement> {
    // All Souls excludes all occurrent feasts.
    if is_office_of_the_dead(day) {
        return Vec::new();
    }
    let comms: Vec<FeastRef> = if hour_name == "lauds" { lauds_commemorations(&day.cal) } else { day.commemorations.clone() };
    // A saint commemorated within the Octave of Pentecost keeps its Common's
    // Paschaltide forms (2026 ordo, 4-5 June: Boniface "Light perpetual",
    // "Daughters"; #631).
    let season = day.saints_season(hour_name);
    let mut elems = Vec::new();
    for (i, comm) in comms.iter().enumerate() {
        let lookup = |reference: &str| -> (String, String) {
            if let Some(key) = octave_commemoration_ref(day, comm, hour_name, reference) {
                let text = t.get(&key);
                if !text.is_empty() {
                    return (text.to_string(), key);
                }
            }
            let found = advent_date_commemoration_antiphon(day, comm, hour_name, reference, t);
            if !found.0.is_empty() {
                return found;
            }
            if is_synthesized_feria(comm) {
                return lookup_feria_commemoration(Some(day), comm, day.season, hour_name, reference, t);
            }
            // An Advent Ember day's collect is "said at all the Hours until
            // Vespers" (Diurnal p. 177): at Vespers it is a feria of its week,
            // with the weekday's antiphon and the Sunday's collect (2025 ordo
            // 17 Dec, "Comm. Fer. ... Col. 170"; 2022 ordo 14 Dec, "Behold the
            // handmaid of the Lord"; #642).
            if hour_name == "vespers"
                && comm.id.starts_with("advent-ember-")
                && let Some(week) = &day.temporal_week_id
            {
                let mut feria = (**comm).clone();
                feria.proper_id = Some(week.clone());
                return lookup_feria_commemoration(Some(day), &feria, day.season, hour_name, reference, t);
            }
            if is_saturday_sunday_commemoration(day, comm, hour_name, reference) {
                return lookup_sunday_first_vespers_commemoration(day, comm, reference, t);
            }
            if hour_name == "vespers" && commemoration_takes_first_vespers(day, comm, reference) {
                return lookup_following_office_commemoration(comm, season, reference, t);
            }
            let found = lookup_commemoration(comm, season, hour_name, reference, t);
            // A vigil or feria without its own antiphon takes the Psalter's
            // for the weekday (General Rubrics VI; Diurnal p. 1*).
            if reference == "commemoration-antiphon"
                && comm.is_category(Category::Feria)
                && (found.1.starts_with("ordinary/") || found.0.starts_with('['))
            {
                let slot = if hour_name == "vespers" { "magnificat-antiphon" } else { "benedictus-antiphon" };
                let key = format!("ordinary/{hour_name}/{slot}-{}", day.civil_weekday_name());
                let text = t.get(&key);
                if !text.is_empty() {
                    return (text.to_string(), key);
                }
            }
            found
        };
        let owned = |mut e: OfficeElement| {
            e.commemoration_owner_id = comm.id.clone();
            e.is_commemoration = true;
            e
        };
        elems.push(owned(OfficeElement::new(ElementType::Heading, format!("Commemoration of {}", comm.commemoration_name()))));

        let slot = |kind: ElementType, text: String, slot_ref: &str, src: &str, refs: Vec<String>| {
            let mut e = OfficeElement::new(kind, text);
            e.slot_ref = slot_ref.to_string();
            e.source_ref = src.to_string();
            e.source_refs = compact_refs(refs);
            owned(e)
        };
        let (ant, ant_src) = lookup("commemoration-antiphon");
        elems.push(slot(ElementType::Antiphon, ant, "commemoration-antiphon", &ant_src, vec![ant_src.clone()]));
        let (vers, vers_src) = lookup("commemoration-versicle");
        elems.push(slot(ElementType::Versicle, vers, "commemoration-versicle", &vers_src, vec![vers_src.clone()]));

        // Each commemoration has its own invitation (XXXIII.3,5).
        elems.push(owned(resolve_element(&HourElement::new("prayer", "shared/leader/let-us-pray"), t)));

        let (mut collect, collect_src) = lookup("commemoration-collect");
        let mut collect_refs = vec![collect_src.clone()];
        if i == comms.len() - 1 && !more_collects {
            (collect, collect_refs) = apply_conclusion(&collect, &collect_src, t);
        }
        elems.push(slot(ElementType::Collect, collect, "commemoration-collect", &collect_src, collect_refs));
    }
    elems
}

/// An optional appointment on the octave parent's proper for the actual
/// Vespers context (Diurnal pp.391,393,418,421; XIV.10–11). Each context is
/// opt-in (#398).
pub fn octave_commemoration_ref(day: &Day, comm: &Feast, hour_name: &str, reference: &str) -> Option<String> {
    let parent = octave_parent_id(comm)?;
    let celebration = day.celebration.as_deref()?;
    if hour_name != "vespers" || (reference != "commemoration-antiphon" && reference != "commemoration-versicle") {
        return None;
    }
    if !celebration.is_category(Category::Sunday) {
        if day.vespers.owner == VespersOwner::IOfFollowing && is_day_within_octave(comm) {
            return Some(format!("proper/{parent}/{reference}-at-first-vespers"));
        }
        return None;
    }
    if day.within_octave_of.as_deref() != Some(parent) {
        return None;
    }
    let context = match day.vespers.owner {
        VespersOwner::IOfFollowing => "sunday-first-vespers".to_string(),
        VespersOwner::IIOfPreceding => {
            let mut c = "sunday-second-vespers".to_string();
            if day.vespers.following_office_octave_of != day.within_octave_of {
                c.push_str("-before-other-office");
            }
            c
        }
        VespersOwner::NotApplicable => return None,
    };
    Some(format!("proper/{parent}/{reference}-{context}"))
}

/// A Sunday commemorated at Saturday Vespers — whether a Saturday feast keeps
/// II Vespers or a feast on the Sunday takes I Vespers — begins with its own
/// I-Vespers antiphon and versicle (XIV.14; Diurnal p. 403). The Sunday
/// office said on Saturday when the Octave Day of the Epiphany is Sunday is
/// outgoing, commemorated with its II-Vespers antiphon (p. 231) (#651).
/// A Sunday anticipated on Saturday has its I Vespers on Friday, with
/// Friday's psalter antiphon (2025 and 2026 ordos, 14 and 6 February: "Fri.
/// Off. ... He hath put down"); commemorated there, it keeps them (needs
/// ruling, #656).
pub fn is_saturday_sunday_commemoration(day: &Day, feast: &Feast, hour_name: &str, reference: &str) -> bool {
    hour_name == "vespers"
        && (reference == "commemoration-antiphon" || reference == "commemoration-versicle")
        && (day.civil_weekday() == Weekday::Saturday || feast.id.ends_with("-anticipated"))
        && feast.is_category(Category::Sunday)
        && day.vespers.incoming_commemoration_ids.contains(&feast.id)
}

/// Resolves the commemorated Sunday's slot exactly as its own I Vespers
/// would: historia, the Sunday's "-first" proper, season, Saturday psalter.
fn lookup_sunday_first_vespers_commemoration(day: &Day, feast: &FeastRef, reference: &str, t: &OfficeTexts) -> (String, String) {
    let mut sunday = day.clone();
    if !sunday.first_vespers {
        sunday.cal.date = day.date.add_days(1);
        sunday.first_vespers = true;
    }
    sunday.cal.celebration = Some(feast.clone());
    let slot = if reference == "commemoration-antiphon" { "magnificat-antiphon" } else { "versicle" };
    resolve_proper_text(&sunday, "vespers", slot, t)
}

/// An incoming office, Memorial, or Saturday-Vespers Sunday begins with
/// its own I-Vespers texts.
pub fn commemoration_takes_first_vespers(day: &Day, comm: &Feast, reference: &str) -> bool {
    if (comm.rank == Rank::Commemoration && comm.companion_of.is_none())
        || (!comm.id.is_empty() && comm.id == day.following_office_commemoration_id)
        || day.vespers.incoming_commemoration_ids.contains(&comm.id)
    {
        return reference == "commemoration-antiphon" || reference == "commemoration-versicle";
    }
    is_saturday_sunday_commemoration(day, comm, "vespers", reference)
}

/// The I-Vespers texts of an incoming office or Memorial (VIII, X).
fn lookup_following_office_commemoration(feast: &Feast, season: Season, reference: &str, t: &OfficeTexts) -> (String, String) {
    if feast.rank == Rank::Commemoration {
        return lookup_commemoration_office(feast, season, "vespers", reference, true, t);
    }
    let candidates: &[&str] = match reference {
        "commemoration-antiphon" => &["magnificat-antiphon-first", "magnificat-antiphon"],
        "commemoration-versicle" => &["versicle-first-vespers", "versicle-vespers"],
        _ => &[],
    };
    // The feast's own Vespers text outranks its Common's I-Vespers one.
    for candidate in candidates {
        for id in feast_proper_ids(feast) {
            let (text, source) = lookup_section_text(&format!("proper/{id}/"), Some(season), "vespers", candidate, t);
            if !text.is_empty() {
                return (substitute_proper_name(&text, &feast_proper_name(feast)), source);
            }
        }
    }
    for candidate in candidates {
        let (text, source) = lookup_commemoration(feast, season, "vespers", candidate, t);
        if !text.is_empty() && !text.starts_with('[') {
            return (text, source);
        }
    }
    lookup_commemoration(feast, season, "vespers", reference, t)
}

/// The synthesized occurring feria: gospel antiphon of the governing week,
/// the little versicle, the Sunday collect.
fn lookup_feria_commemoration(
    day: Option<&Day>,
    feast: &Feast,
    season: Season,
    hour_name: &str,
    reference: &str,
    t: &OfficeTexts,
) -> (String, String) {
    match reference {
        "commemoration-antiphon" => {
            let ant_slot = if hour_name == "vespers" { "magnificat-antiphon" } else { "benedictus-antiphon" };
            if let Some(day) = day {
                // The week's own ferial antiphon, else the Psalter's for the
                // weekday (2026 ordo 10 February: "The Lord" p. 55).
                let weekday = day.civil_weekday_name();
                let proper_ref = feast.proper_id.as_ref().map(|p| format!("proper/{p}/{ant_slot}-{weekday}"));
                for ant_ref in proper_ref.into_iter().chain([format!("ordinary/{hour_name}/{ant_slot}-{weekday}")]) {
                    let text = t.get(&ant_ref);
                    if !text.is_empty() {
                        return (text.to_string(), ant_ref);
                    }
                }
            }
            let ant_ref = format!("ordinary/{hour_name}/{ant_slot}");
            let text = t.get(&ant_ref);
            if !text.is_empty() {
                return (text.to_string(), ant_ref);
            }
        }
        "commemoration-versicle" => {
            let found = lookup_temporal_commemoration_versicle(feast, season, hour_name, t);
            if !found.0.is_empty() {
                return found;
            }
        }
        "commemoration-collect" => {
            if let Some(proper) = &feast.proper_id {
                // The weekday's own collect for the hour (a Lenten feria's
                // Vespers collect: 2026 ordo, 20 Mar., I Vespers of St Benedict,
                // "Comm. Fer. ('Sir I perceive' & Col. 263-4)"), else the Sunday's.
                let weekday = day.map(|d| d.civil_weekday_name());
                let weekday_refs = weekday.iter().flat_map(|w| [format!("collect-{hour_name}-{w}"), format!("collect-{w}")]);
                for collect_ref in weekday_refs.chain(["collect".to_string()]).map(|r| format!("proper/{proper}/{r}")) {
                    let text = t.get(&collect_ref);
                    if !text.is_empty() {
                        return (text.to_string(), collect_ref);
                    }
                }
            }
        }
        _ => {}
    }
    ordinary_or_marker(feast, hour_name, reference, t)
}

fn ordinary_or_marker(feast: &Feast, hour_name: &str, reference: &str, t: &OfficeTexts) -> (String, String) {
    let ordinary_ref = format!("ordinary/{hour_name}/{reference}");
    let text = t.get(&ordinary_ref);
    if !text.is_empty() {
        return (text.to_string(), ordinary_ref);
    }
    (format!("[Commemoration text not found: {reference} for {}]", feast.id), reference.to_string())
}

/// A de Tempore commemoration (Sunday, Ember day, vigil): its own gospel
/// antiphon, the hour's versicle, its proper collect.
fn lookup_temporal_commemoration(feast: &Feast, season: Season, hour_name: &str, reference: &str, t: &OfficeTexts) -> (String, String) {
    let ids = feast_proper_ids(feast);
    for id in &ids {
        let key = format!("proper/{id}/{reference}");
        let text = t.get(&key);
        if !text.is_empty() {
            return (text.to_string(), key);
        }
    }
    match reference {
        "commemoration-antiphon" => {
            let ant_slot = if hour_name == "vespers" { "magnificat-antiphon" } else { "benedictus-antiphon" };
            for id in &ids {
                let key = format!("proper/{id}/{ant_slot}");
                let text = t.get(&key);
                if !text.is_empty() {
                    return (text.to_string(), key);
                }
            }
            let key = format!("ordinary/{hour_name}/{ant_slot}");
            let text = t.get(&key);
            if !text.is_empty() {
                return (text.to_string(), key);
            }
        }
        "commemoration-versicle" => {
            let found = lookup_temporal_commemoration_versicle(feast, season, hour_name, t);
            if !found.0.is_empty() {
                return found;
            }
        }
        "commemoration-collect" => {
            for id in &ids {
                // An hour's own collect (an Ember day's collect-lauds) first.
                let (text, key) = lookup_section_text(&format!("proper/{id}/"), Some(season), hour_name, "collect", t);
                if !text.is_empty() {
                    // A vigil may share its Common's "N." collect (Diurnal p. 7*).
                    return (substitute_proper_name(&text, &feast_proper_name(feast)), key);
                }
            }
        }
        _ => {}
    }
    ordinary_or_marker(feast, hour_name, reference, t)
}

/// The commemorated office's own versicle, else its Psalter (X, p. xxix).
fn lookup_temporal_commemoration_versicle(feast: &Feast, season: Season, hour_name: &str, t: &OfficeTexts) -> (String, String) {
    if !is_synthesized_feria(feast) {
        for id in feast_proper_ids(feast) {
            let found = lookup_section_text(&format!("proper/{id}/"), Some(season), hour_name, "versicle", t);
            if !found.0.is_empty() {
                return found;
            }
        }
    }
    let found = lookup_section_text(&format!("seasonal/{season}/"), Some(season), hour_name, "versicle", t);
    if !found.0.is_empty() {
        return found;
    }
    if feast.is_category(Category::Sunday) {
        let key = format!("ordinary/{hour_name}/versicle-sunday");
        let text = t.get(&key);
        if !text.is_empty() {
            return (text.to_string(), key);
        }
    }
    let key = format!("ordinary/{hour_name}/versicle");
    (t.get(&key).to_string(), key)
}

fn commemoration_fallback_slots(hour_name: &str, reference: &str, first_vespers: bool) -> Vec<&'static str> {
    match reference {
        "commemoration-antiphon" if first_vespers => vec!["magnificat-antiphon-first", "magnificat-antiphon"],
        "commemoration-antiphon" if hour_name == "vespers" => vec!["magnificat-antiphon", "magnificat-antiphon-first"],
        "commemoration-antiphon" => vec!["benedictus-antiphon"],
        "commemoration-versicle" if first_vespers => vec!["versicle-first-vespers", "versicle-vespers"],
        "commemoration-versicle" if hour_name == "vespers" => vec!["versicle-vespers", "versicle-lauds", "versicle"],
        "commemoration-versicle" => vec!["versicle-lauds", "versicle"],
        "commemoration-collect" => vec!["collect"],
        _ => Vec::new(),
    }
}

/// A commemoration text: feast proper, then Commons, then ordinary, with
/// "N." substitution.
pub fn lookup_commemoration(feast: &Feast, season: Season, hour_name: &str, reference: &str, t: &OfficeTexts) -> (String, String) {
    lookup_commemoration_office(feast, season, hour_name, reference, false, t)
}

fn lookup_commemoration_office(
    feast: &Feast,
    season: Season,
    hour_name: &str,
    reference: &str,
    first_vespers: bool,
    t: &OfficeTexts,
) -> (String, String) {
    if is_synthesized_feria(feast) {
        return lookup_feria_commemoration(None, feast, season, hour_name, reference, t);
    }
    if feast.is_category(Category::Sunday) || feast.is_category(Category::Feria) {
        return lookup_temporal_commemoration(feast, season, hour_name, reference, t);
    }
    let proper_name = feast_proper_name(feast);
    let ids = feast_proper_ids(feast);
    for id in &ids {
        let (text, resolved) = lookup_section_text(&format!("proper/{id}/"), Some(season), hour_name, reference, t);
        if !text.is_empty() {
            return (substitute_proper_name(&text, &proper_name), resolved);
        }
    }
    let fallbacks = commemoration_fallback_slots(hour_name, reference, first_vespers);
    for fallback in &fallbacks {
        for id in &ids {
            let (text, resolved) = lookup_section_text(&format!("proper/{id}/"), Some(season), hour_name, fallback, t);
            if !text.is_empty() {
                return (substitute_proper_name(&text, &proper_name), resolved);
            }
        }
    }
    // A Memorial begins with the Common's I-Vespers appointment (VIII, X).
    let common_slots: Vec<&str> = if first_vespers {
        fallbacks.iter().copied().chain([reference]).collect()
    } else {
        [reference].into_iter().chain(fallbacks.iter().copied()).collect()
    };
    for slot in common_slots {
        let (text, resolved) = lookup_commons_text(feast.category, season, hour_name, slot, t);
        if !text.is_empty() {
            return (substitute_proper_name(&text, &proper_name), resolved);
        }
    }
    let ordinary_ref = format!("ordinary/{hour_name}/{reference}");
    let text = t.get(&ordinary_ref);
    if !text.is_empty() {
        return (substitute_proper_name(text, &proper_name), ordinary_ref);
    }
    (format!("[Commemoration text not found: {reference} for {}]", feast.id), reference.to_string())
}
