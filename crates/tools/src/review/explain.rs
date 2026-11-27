//! Composition explanations (`office review explain`).

use std::collections::{BTreeSet, HashSet};

use calendar::{DataSource, Date, Decision};
use data_format::json::{Json, Obj};
use liturgy::{OfficeElement, OfficeHour, PrayerForm};
use office::day::Day;
use office::engine::Engine;
use office::psalmody::{VESPERS_OF_THE_DEAD_LABEL, dead_office_day};
use office::trace::ProperResolutionTrace;

use super::provenance::{ProvenanceStatus, SourceCitation, scan_provenance};
use super::{celebration_name, unit_key};

/// The sorted, unique corpus keys behind a rendered hour.
pub fn hour_dependencies(hour: &OfficeHour) -> Vec<String> {
    let mut refs = BTreeSet::new();
    for elem in hour.sections.iter().flat_map(|s| &s.elements) {
        if elem.source_refs.is_empty() {
            if !elem.source_ref.is_empty() {
                refs.insert(elem.source_ref.clone());
            }
        } else {
            refs.extend(elem.source_refs.iter().filter(|r| !r.is_empty()).cloned());
        }
    }
    refs.into_iter().collect()
}

/// Trace events without repeats, in first-seen order.
pub fn dedupe_decisions(decisions: &[Decision]) -> Vec<&Decision> {
    let mut seen = HashSet::new();
    decisions.iter().filter(|d| seen.insert((d.rule.as_str(), d.outcome.as_str(), d.detail.as_deref().unwrap_or("")))).collect()
}

/// Traces a rendered slot, as a commemoration when it is one.
pub fn trace_element(engine: &Engine, day: &Day, hour_name: &str, elem: &OfficeElement, appended_dead: bool) -> ProperResolutionTrace {
    let dead;
    let day = if appended_dead {
        dead = dead_office_day(day);
        &dead
    } else {
        day
    };
    if elem.is_commemoration {
        return engine.trace_commemoration_resolution(day, hour_name, &elem.slot_ref, &elem.source_ref, &elem.commemoration_owner_id);
    }
    engine.trace_proper_resolution(day, hour_name, &elem.slot_ref, &elem.source_ref)
}

fn is_resolution_source(r: &str) -> bool {
    ["proper/", "commons/", "seasonal/", "ordinary/"].iter().any(|p| r.starts_with(p))
}

/// One hour joined to its provenance, as the `review explain` JSON.
pub fn explain_composition(src: &dyn DataSource, hour_name: &str, date: Date, form: PrayerForm) -> Result<String, String> {
    let days = crate::year::load_office_days(src, date.year())?;
    let day = days.get(date.ordinal() as usize - 1).ok_or_else(|| format!("date out of range: {date}"))?;
    let engine = Engine::load(src)?;
    let hour = engine.compose_hour(hour_name, day, &calendar::MoveableDates::compute(date.year()), form)?;
    let inventory = scan_provenance(src)?;
    let by_key = inventory.by_key();

    let decisions = dedupe_decisions(&hour.decisions)
        .into_iter()
        .map(|d| {
            Obj::new().str("rule", &d.rule).str("outcome", &d.outcome).str_omitempty("detail", d.detail.as_deref().unwrap_or("")).build()
        })
        .collect();
    let dependencies: Vec<Json> = hour_dependencies(&hour)
        .iter()
        .map(|r| match by_key.get(r.as_str()) {
            None => Obj::new().str("key", r).str("status", ProvenanceStatus::SourceUnknown.as_str()).build(),
            Some(e) => Obj::new()
                .str("key", r)
                .str("status", e.status.as_str())
                .str_omitempty("file", &e.file)
                .str_omitempty("section", &e.section)
                .field_omitempty_arr("sources", e.sources.iter().map(citation_json).collect())
                .build(),
        })
        .collect();
    let mut resolutions = Vec::new();
    let mut appended_dead = false;
    for section in &hour.sections {
        appended_dead |= section.label == VESPERS_OF_THE_DEAD_LABEL;
        for elem in &section.elements {
            if elem.slot_ref.is_empty() {
                continue;
            }
            let t = trace_element(&engine, day, hour_name, elem, appended_dead);
            if !is_resolution_source(&elem.source_ref) && t.selected_tier != "not-found" {
                continue;
            }
            resolutions.push(
                Obj::new()
                    .str("requested_slot", &t.requested_slot)
                    .str("resolver_hour", &t.resolver_hour)
                    .str("resolver_slot", &t.resolver_slot)
                    .str_omitempty("canonical_owner", &t.canonical_owner)
                    .strings_omitempty("proper_ids", &t.proper_ids)
                    .strings_omitempty("direct_candidates", &t.direct_candidates)
                    .strings_omitempty("direct_existing", &t.direct_existing)
                    .str("selected_ref", &t.selected_ref)
                    .str("selected_tier", &t.selected_tier)
                    .str("reason", &t.reason)
                    .bool_omitempty("first_vespers", t.first_vespers)
                    .build(),
            );
        }
    }
    let v = Obj::new()
        .str("form", hour.form.as_str())
        .str("date", &date.to_string())
        .str("hour", hour_name)
        .str("unit_key", &unit_key(day, hour_name))
        .str("celebration", &celebration_name(day))
        .str("season", hour.season.map_or("", |s| s.as_str()))
        .str("color", hour.color.map_or("", |c| c.as_str()))
        .field("decisions", Json::Arr(decisions))
        .field("dependencies", if dependencies.is_empty() { Json::Null } else { Json::Arr(dependencies) })
        .field_omitempty_arr("resolutions", resolutions)
        .build();
    Ok(data_format::json::encode_indent(&v))
}

fn citation_json(s: &SourceCitation) -> Json {
    let mut o = Obj::new()
        .str("kind", &s.kind)
        .str("source", &s.source)
        .str_omitempty("locator", &s.locator)
        .str_omitempty("page", &s.page)
        .str_omitempty("note", &s.note);
    if s.line != 0 {
        o = o.int("line", s.line as i64);
    }
    o.build()
}

trait ObjExt {
    fn field_omitempty_arr(self, name: &str, items: Vec<Json>) -> Obj;
}

impl ObjExt for Obj {
    fn field_omitempty_arr(self, name: &str, items: Vec<Json>) -> Obj {
        if items.is_empty() { self } else { self.field(name, Json::Arr(items)) }
    }
}
