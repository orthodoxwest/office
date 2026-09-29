//! The composition sweep: compose every hour of every day of a year (a missing corpus entry fails
//! composition) and report ordinary-tier fallbacks on Double-or-above days.

use std::collections::HashMap;
use std::fmt::Write as _;

use calendar::{DataSource, Date, Feast, MoveableDates, Rank};
use liturgy::PrayerForm;
use office::day::Day;
use office::engine::{Engine, HOUR_NAMES};

use super::{load_suppress_file, trim_index_suffix};

/// Slot bases that can carry a feast's own text at Lauds or Vespers; other
/// slots are fixed texts that rightly come from the ordinary every day.
const PROPERIZABLE_SLOTS: [&str; 8] =
    ["psalm-antiphon", "benedictus-antiphon", "magnificat-antiphon", "hymn", "chapter", "versicle", "short-responsory", "collect"];

/// A Lauds or Vespers slot of a Double-or-above day that rendered from the
/// ordinary.
#[derive(Clone, Debug)]
pub struct OrdinaryFallback {
    pub feast_id: String,
    pub name: String,
    pub rank: Rank,
    pub hour: &'static str,
    pub slot: String,
    pub source_ref: String,
    pub first_date: Date,
    pub count: usize,
}

pub struct SweepReport {
    pub year: i32,
    pub ordinary_fallbacks: Vec<OrdinaryFallback>,
}

/// Findings are deduplicated across the year and suppressible per feast and slot in
/// `data/audit-ok.txt`.
pub fn sweep_year(src: &dyn DataSource, year: i32) -> Result<SweepReport, String> {
    let engine = Engine::load(src).map_err(|e| format!("creating office engine: {e}"))?;
    let days = crate::year::load_office_days(src, year).map_err(|e| format!("building calendar for {year}: {e}"))?;
    let suppress = load_suppress_file(src).map_err(|e| format!("loading audit-ok.txt: {e}"))?;
    let moveable = MoveableDates::compute(year);

    let mut fallbacks: HashMap<(String, &'static str, String, String), OrdinaryFallback> = HashMap::new();
    for day in &days {
        for hour_name in HOUR_NAMES {
            let hour = engine
                .compose_hour(hour_name, day, &moveable, PrayerForm::Private)
                .map_err(|e| format!("composing {hour_name} for {}: {e}", day.date))?;
            let feast = sweep_feast(day, hour_name);
            let check_fallbacks =
                matches!(hour_name, "lauds" | "vespers") && feast.is_some_and(|f| f.rank.weight() >= Rank::Double.weight());

            for el in hour.sections.iter().flat_map(|s| &s.elements) {
                let Some(feast) = feast.filter(|_| check_fallbacks && !el.slot_ref.is_empty()) else { continue };
                let base = trim_index_suffix(&el.slot_ref);
                if !PROPERIZABLE_SLOTS.contains(&base) || !el.source_ref.starts_with("ordinary/") {
                    continue;
                }
                let suppressed = |id: &str| suppress.get(id).is_some_and(|s| s.contains(&el.slot_ref) || s.contains(base));
                if suppress.get(&feast.id).is_some_and(|s| s.contains("*")) || suppressed(&feast.id) || suppressed("*") {
                    continue;
                }
                fallbacks
                    .entry((feast.id.clone(), hour_name, el.slot_ref.clone(), el.source_ref.clone()))
                    .and_modify(|f| f.count += 1)
                    .or_insert_with(|| OrdinaryFallback {
                        feast_id: feast.id.clone(),
                        name: feast.name.clone(),
                        rank: feast.rank,
                        hour: hour_name,
                        slot: el.slot_ref.clone(),
                        source_ref: el.source_ref.clone(),
                        first_date: day.date,
                        count: 1,
                    });
            }
        }
    }

    let mut ordinary_fallbacks: Vec<OrdinaryFallback> = fallbacks.into_values().collect();
    ordinary_fallbacks.sort_by(|a, b| {
        b.rank
            .weight()
            .cmp(&a.rank.weight())
            .then_with(|| a.feast_id.cmp(&b.feast_id))
            .then_with(|| a.hour.cmp(b.hour))
            .then_with(|| a.slot.cmp(&b.slot))
            .then_with(|| a.source_ref.cmp(&b.source_ref))
            .then(a.first_date.cmp(&b.first_date))
    });
    Ok(SweepReport { year, ordinary_fallbacks })
}

/// The celebration that owns the hour: the evening may belong to the
/// following day's feast.
fn sweep_feast<'a>(day: &'a Day, hour_name: &str) -> Option<&'a Feast> {
    if hour_name == "vespers"
        && let Some(f) = &day.vespers.feast
    {
        return Some(f);
    }
    day.celebration.as_deref()
}

pub fn format_sweep(r: &SweepReport) -> String {
    let mut w = String::new();
    let _ = writeln!(w, "=== Sweep {}: ordinary fallbacks on Double+ days: {} slot(s) ===", r.year, r.ordinary_fallbacks.len());
    if !r.ordinary_fallbacks.is_empty() {
        w.push_str("Lauds/Vespers slots that rendered ordinary texts on a Double-or-above day —\n");
        w.push_str("check the diurnal for a proper; add to data/audit-ok.txt if intentional.\n");
        let mut last_feast = "";
        for f in &r.ordinary_fallbacks {
            if f.feast_id != last_feast {
                let _ = writeln!(w, "  [{}] {} ({})", f.rank.abbrev(), f.name, f.feast_id);
                last_feast = &f.feast_id;
            }
            let _ = writeln!(w, "    {:<8} {:<20} → {} ({} day(s), first {})", f.hour, f.slot, f.source_ref, f.count, f.first_date);
        }
    }
    w.push('\n');
    w
}
