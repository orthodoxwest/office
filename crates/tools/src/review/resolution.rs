//! The resolution inventory: every dynamic proper slot of a sweep with the tier it resolved from,
//! deduplicated.

use std::collections::HashMap;
use std::fmt::Write as _;

use calendar::{CalendarData, DataSource};
use data_format::json::{Json, Obj};
use office::engine::Engine;
use office::psalmody::VESPERS_OF_THE_DEAD_LABEL;
use office::trace::ProperResolutionTrace;

use super::assurance::trace_element;
use super::sweep::{Forms, sweep};

/// One representative resolution: keys and calendar metadata, never text.
#[derive(Clone, Debug)]
pub struct Row {
    pub trace: ProperResolutionTrace,
    pub hour: &'static str,
    pub season: String,
    pub weekday: &'static str,
    pub part: &'static str,
    context_key: String,
    pub date: String,
    pub dates: Vec<String>,
    pub occurrences: usize,
}

pub struct ResolutionInventory {
    pub start_year: i32,
    pub years: i32,
    pub rows: Vec<Row>,
}

fn is_dynamic_resolution_ref(r: &str) -> bool {
    ["proper/", "commons/", "seasonal/", "ordinary/"].iter().any(|p| r.starts_with(p))
}

/// One private composition per date and hour (forms vary only marked ordinary slots after
/// resolution).
pub fn build_resolution_inventory(src: &dyn DataSource, start: i32, years: i32) -> Result<ResolutionInventory, String> {
    if years < 1 {
        return Err("years must be positive".into());
    }
    let engine = Engine::load(src).map_err(|e| format!("creating office engine: {e}"))?;
    let cal = CalendarData::load(src).map_err(|e| format!("building calendar for {start}: {e}"))?;
    let mut by_key: HashMap<String, usize> = HashMap::new();
    let mut rows: Vec<Row> = Vec::new();
    let mut fold = |found: Vec<(String, Row)>| {
        for (key, row) in found {
            match by_key.get(&key) {
                Some(&i) => {
                    let old = &mut rows[i];
                    old.occurrences += 1;
                    if old.dates.last() != Some(&row.date) {
                        old.dates.push(row.date);
                    }
                }
                None => {
                    by_key.insert(key, rows.len());
                    rows.push(row);
                }
            }
        }
    };
    let calendar_error = |y: i32, e: String| format!("building calendar for {y}: {e}");
    sweep(
        &engine,
        &cal,
        start,
        years,
        Forms::Private,
        &calendar_error,
        &|day, hour_name, hour| {
            let mut found = Vec::new();
            let mut part = "principal";
            let season = hour.season.map_or("", |s| s.as_str());
            let weekday = day.date.weekday().name();
            let date = day.date.to_string();
            for section in &hour.sections {
                if section.label == VESPERS_OF_THE_DEAD_LABEL {
                    part = "appended-office-of-the-dead";
                }
                for elem in &section.elements {
                    if elem.slot_ref.is_empty() {
                        continue;
                    }
                    let trace = trace_element(&engine, day, hour_name, elem, part == "appended-office-of-the-dead");
                    if !is_dynamic_resolution_ref(&elem.source_ref) && trace.selected_tier != "not-found" {
                        continue;
                    }
                    // Without a canonical owner there is no safe target for a
                    // feast-specific proposal.
                    if trace.owner_id.is_empty() {
                        continue;
                    }
                    let context_key = [
                        trace.canonical_owner.as_str(),
                        &trace.proper_ids.join("\x1e"),
                        &trace.resolver_hour,
                        &trace.resolver_slot,
                        season,
                        weekday,
                        part,
                    ]
                    .join("\x1f");
                    let key = [
                        trace.owner_id.as_str(),
                        hour_name,
                        if trace.first_vespers { "true" } else { "false" },
                        &trace.slot_ref,
                        &trace.selected_ref,
                        &trace.reason,
                        &context_key,
                    ]
                    .join("\x1f");
                    found.push((
                        key,
                        Row {
                            trace,
                            hour: hour_name,
                            season: season.to_string(),
                            weekday,
                            part,
                            context_key,
                            date: date.clone(),
                            dates: vec![date.clone()],
                            occurrences: 1,
                        },
                    ));
                }
            }
            found
        },
        &mut fold,
    )?;
    rows.sort_by(|a, b| {
        a.trace
            .owner_id
            .cmp(&b.trace.owner_id)
            .then_with(|| a.hour.cmp(b.hour))
            .then_with(|| a.trace.slot_ref.cmp(&b.trace.slot_ref))
            .then_with(|| a.trace.selected_ref.cmp(&b.trace.selected_ref))
            .then_with(|| a.date.cmp(&b.date))
            .then_with(|| a.context_key.cmp(&b.context_key))
    });
    Ok(ResolutionInventory { start_year: start, years, rows })
}

impl ResolutionInventory {
    /// Keeps rows not resolved by the owning proper.
    pub fn filter_fallbacks(&mut self) {
        self.rows.retain(|r| r.trace.selected_tier != "proper");
    }

    /// Occurrences by selected tier.
    pub fn summary(&self) -> String {
        let mut counts: HashMap<&str, usize> = HashMap::new();
        for r in &self.rows {
            *counts.entry(r.trace.selected_tier.as_str()).or_default() += r.occurrences;
        }
        let mut w = format!("resolution inventory: {} rows\n", self.rows.len());
        for tier in [
            "proper",
            "proper-inherited",
            "temporal-week",
            "special",
            "common",
            "seasonal",
            "ordinary-weekday",
            "ordinary",
            "shared",
            "not-found",
        ] {
            if let Some(n) = counts.get(tier).filter(|n| **n != 0) {
                let _ = writeln!(w, "{tier}: {n}");
            }
        }
        w
    }

    /// One tab-separated line per row.
    pub fn tsv(&self) -> String {
        let mut w = String::new();
        for r in &self.rows {
            let t = &r.trace;
            let _ = writeln!(
                w,
                "{}\t{}\t{}\t{}\t{}\t{}\t{}",
                r.date, t.owner_id, r.hour, t.slot_ref, t.selected_tier, t.selected_ref, t.reason
            );
        }
        w
    }

    /// JSON encoding of the inventory.
    pub fn json(&self) -> String {
        let rows = self
            .rows
            .iter()
            .map(|r| {
                let t = &r.trace;
                Obj::new()
                    .str_omitempty("owner_id", &t.owner_id)
                    .str_omitempty("canonical_owner", &t.canonical_owner)
                    .strings_omitempty("proper_ids", &t.proper_ids)
                    .str("hour", r.hour)
                    .bool_omitempty("first_vespers", t.first_vespers)
                    .str("season", &r.season)
                    .str("weekday", r.weekday)
                    .str("part", r.part)
                    .str("requested_slot", &t.requested_slot)
                    .str("slot_ref", &t.slot_ref)
                    .str("resolver_hour", &t.resolver_hour)
                    .str("resolver_slot", &t.resolver_slot)
                    .strings_omitempty("direct_candidates", &t.direct_candidates)
                    .strings_omitempty("direct_existing", &t.direct_existing)
                    .str("selected_ref", &t.selected_ref)
                    .str("selected_tier", &t.selected_tier)
                    .str("reason", &t.reason)
                    .str("date", &r.date)
                    .field("dates", Json::strings(&r.dates))
                    .int("occurrences", r.occurrences as i64)
                    .build()
            })
            .collect();
        let v = Obj::new()
            .int("start_year", i64::from(self.start_year))
            .int("years", i64::from(self.years))
            .field("rows", Json::Arr(rows))
            .build();
        data_format::json::encode_indent(&v)
    }
}
