//! Observational tracing of dynamic proper resolution, for the review tools. Composition never
//! calls this: the trace records how a source the composer already selected relates to the day's
//! proper IDs.

use calendar::traits::octave_parent_id;
use calendar::{Category, Feast, Rank, Season};

use crate::commemoration::{is_incoming_sunday_commemoration, octave_commemoration_ref};
use crate::day::Day;
use crate::engine::Engine;
use crate::prime::is_prime_antiphon_ref;
use crate::proper::{feast_proper_ids, hour_ref_candidates, ref_candidates, season_ref_candidates};
use crate::texts::OfficeTexts;
use crate::vespers::vespers_office_day;

/// Marks a commemoration trace whose owner is not among the day's
/// commemorations, which an empty owner alone cannot distinguish from an
/// unowned feria.
pub const UNKNOWN_COMMEMORATION_OWNER_REASON: &str = "unknown-commemoration-owner";

/// How one dynamic slot was resolved. Empty strings and vectors are absent.
#[derive(Clone, Debug, Default, PartialEq, Eq)]
pub struct ProperResolutionTrace {
    /// The slot the composer asked for; `slot_ref` is its compatibility alias.
    pub requested_slot: String,
    pub slot_ref: String,
    pub resolver_hour: String,
    pub resolver_slot: String,
    pub owner_id: String,
    pub canonical_owner: String,
    pub proper_ids: Vec<String>,
    pub direct_candidates: Vec<String>,
    pub direct_existing: Vec<String>,
    pub selected_ref: String,
    pub selected_tier: String,
    pub reason: String,
    pub first_vespers: bool,
}

impl Engine {
    /// Traces a principal slot of the hour; Vespers uses the office day of
    /// the evening.
    pub fn trace_proper_resolution(&self, day: &Day, hour_name: &str, reference: &str, selected: &str) -> ProperResolutionTrace {
        if hour_name == "vespers" {
            return trace(&vespers_office_day(day), hour_name, reference, selected, &self.texts);
        }
        trace(day, hour_name, reference, selected, &self.texts)
    }

    /// Traces a generated commemoration slot. An empty owner never falls
    /// back to the principal celebration.
    pub fn trace_commemoration_resolution(
        &self,
        day: &Day,
        hour_name: &str,
        reference: &str,
        selected: &str,
        owner_id: &str,
    ) -> ProperResolutionTrace {
        let office_day;
        let day = if hour_name == "vespers" {
            office_day = vespers_office_day(day);
            &office_day
        } else {
            day
        };
        let (mut owner_day, found) = commemoration_owner_day(day, owner_id);
        // The office day's first_vespers describes the incoming celebration,
        // never the commemoration, so it is re-derived for the owner.
        owner_day.first_vespers =
            found && hour_name == "vespers" && commemoration_takes_first_vespers(day, owner_day.celebration.as_deref(), reference);
        let mut t = trace(&owner_day, hour_name, reference, selected, &self.texts);
        if found
            && let Some(owner) = owner_day.celebration.as_deref()
            && let Some(key) = octave_commemoration_ref(day, owner, hour_name, reference)
            && key == selected
        {
            let prefix = format!("proper/{}/", octave_parent_id(owner).unwrap_or(""));
            t.resolver_slot = key.strip_prefix(&prefix).unwrap_or(&key).to_string();
            t.direct_existing = if self.texts.has(&key) { vec![key.clone()] } else { Vec::new() };
            t.direct_candidates = vec![key];
            t.reason = "octave-commemoration-context".into();
        }
        if !found {
            t.reason = UNKNOWN_COMMEMORATION_OWNER_REASON.into();
        }
        t
    }
}

/// The day re-owned by a commemoration, and whether the owner was found.
fn commemoration_owner_day(day: &Day, owner_id: &str) -> (Day, bool) {
    let mut owner_day = day.clone();
    owner_day.cal.celebration = None;
    if owner_id.is_empty() {
        return (owner_day, false);
    }
    if let Some(f) = day.commemorations.iter().find(|f| f.id == owner_id) {
        owner_day.cal.celebration = Some(f.clone());
        return (owner_day, true);
    }
    if let Some(f) = day.feria_commemoration.as_ref().filter(|f| f.id == owner_id) {
        owner_day.cal.celebration = Some(f.clone());
        return (owner_day, true);
    }
    (owner_day, false)
}

/// Mirrors the composer: an incoming office, Memorial, or Sunday at Saturday
/// Vespers begins with its own I-Vespers texts.
fn commemoration_takes_first_vespers(day: &Day, comm: Option<&Feast>, reference: &str) -> bool {
    let Some(comm) = comm else { return false };
    if (comm.rank == Rank::Commemoration && comm.companion_of.is_none())
        || (!comm.id.is_empty() && comm.id == day.following_office_commemoration_id)
        || day.vespers.incoming_commemoration_ids.contains(&comm.id)
    {
        return reference == "commemoration-antiphon" || reference == "commemoration-versicle";
    }
    is_incoming_sunday_commemoration(day, comm, "vespers", reference)
}

fn trace(day: &Day, hour_name: &str, reference: &str, selected: &str, t: &OfficeTexts) -> ProperResolutionTrace {
    let (resolver_hour, resolver_slot) = resolution_coordinates(day, hour_name, reference, selected);
    let mut tr = ProperResolutionTrace {
        requested_slot: reference.to_string(),
        slot_ref: reference.to_string(),
        resolver_hour: resolver_hour.to_string(),
        resolver_slot: resolver_slot.to_string(),
        ..ProperResolutionTrace::default()
    };
    if let Some(c) = day.celebration.as_deref() {
        tr.owner_id = c.id.clone();
        tr.canonical_owner = c.id.clone();
        tr.proper_ids = feast_proper_ids(c);
    }
    tr.first_vespers = hour_name == "vespers" && day.first_vespers;
    tr.direct_candidates = direct_proper_candidates(day, resolver_hour, resolver_slot, &tr.proper_ids);
    tr.direct_existing = tr.direct_candidates.iter().filter(|k| t.has(k)).cloned().collect();
    tr.selected_ref = selected.to_string();
    tr.selected_tier =
        if selected.is_empty() || !t.has(selected) { "not-found".into() } else { resolution_tier(selected, &tr.proper_ids, day).into() };
    tr.reason = resolution_reason(day, hour_name, reference, selected).into();
    tr
}

/// The production lookup coordinates, where a composer displays a slot in
/// one hour but resolves it through another.
fn resolution_coordinates<'a>(day: &Day, hour_name: &'a str, reference: &'a str, selected: &str) -> (&'a str, &'a str) {
    if reference == "collect" && matches!(hour_name, "terce" | "sext" | "none") {
        return ("lauds", "collect");
    }
    if hour_name == "prime" && reference == "psalm-antiphon-1" {
        if selected.starts_with("ordinary/prime/") || is_prime_antiphon_ref(selected, day.season) {
            return (hour_name, reference);
        }
        let festal = day.celebration.as_deref().is_some_and(|c| c.category != Some(Category::Feria));
        if festal || selected.starts_with("proper/") || selected.starts_with("commons/") || selected.starts_with("ordinary/lauds/") {
            return ("lauds", reference);
        }
    }
    (hour_name, reference)
}

fn direct_proper_candidates(day: &Day, hour_name: &str, reference: &str, proper_ids: &[String]) -> Vec<String> {
    let mut refs = vec![reference.to_string()];
    if hour_name == "vespers" && day.first_vespers && !reference.ends_with("-first") {
        refs.insert(0, format!("{reference}-first"));
    }
    let mut out: Vec<String> = Vec::new();
    let mut add = |key: String| {
        if !out.contains(&key) {
            out.push(key);
        }
    };
    for id in proper_ids {
        for attempt in &refs {
            let hour_refs = hour_ref_candidates(hour_name, attempt);
            let bare_refs = ref_candidates(attempt);
            let mut candidates = season_ref_candidates(&hour_refs, Some(day.season));
            candidates.extend(hour_refs);
            candidates.extend(season_ref_candidates(&bare_refs, Some(day.season)));
            candidates.extend(bare_refs);
            for candidate in candidates {
                if day.season == Season::Easter {
                    add(format!("proper/{id}-paschal/{candidate}"));
                }
                add(format!("proper/{id}/{candidate}"));
            }
        }
    }
    out
}

fn resolution_tier(selected: &str, proper_ids: &[String], day: &Day) -> &'static str {
    if proper_ids.iter().any(|id| selected.starts_with(&format!("proper/{id}/")) || selected.starts_with(&format!("proper/{id}-paschal/")))
    {
        return "proper";
    }
    if let Some(week) = &day.temporal_week_id
        && selected.starts_with(&format!("proper/{week}/"))
    {
        return "temporal-week";
    }
    if selected.starts_with("proper/historia-") {
        "special"
    } else if selected.starts_with("proper/") {
        "proper-inherited"
    } else if selected.starts_with("commons/") {
        "common"
    } else if selected.starts_with("seasonal/") {
        "seasonal"
    } else if selected.starts_with("ordinary/shared/") {
        "shared"
    } else if selected.starts_with("ordinary/") {
        let weekday = ["monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"]
            .iter()
            .any(|w| selected.contains(&format!("-{w}")));
        if weekday { "ordinary-weekday" } else { "ordinary" }
    } else {
        "not-found"
    }
}

fn resolution_reason(day: &Day, hour_name: &str, reference: &str, selected: &str) -> &'static str {
    if hour_name == "lauds" && reference.starts_with("psalm-antiphon-") && selected.contains("festal-canticle-antiphon-") {
        "festal-weekday-canticle"
    } else if selected.starts_with("seasonal/advent/benedictus-antiphon-december-") {
        "date-fixed-benedictus"
    } else if selected.starts_with("seasonal/advent/") && selected.contains("-december-") {
        "greater-antiphon"
    } else if selected.starts_with("proper/historia-") {
        "historia-first-vespers"
    } else if day.temporal_week_id.as_ref().is_some_and(|w| selected.starts_with(&format!("proper/{w}/"))) {
        "weekday-temporal"
    } else if selected.contains("-paschal/") {
        "paschal-proper"
    } else if hour_name == "vespers" && day.first_vespers {
        "first-vespers"
    } else {
        "normal"
    }
}
