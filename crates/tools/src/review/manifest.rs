//! The review manifest: every distinct rendered composition of a sweep, as a reviewer checklist.

use std::collections::HashMap;

use calendar::{CalendarData, DataSource, Date, Rank, Season};
use data_format::csv;
use liturgy::PrayerForm;
use office::engine::Engine;

use super::sweep::{Forms, sweep};
use super::{base_url, celebration_name, celebration_rank, context_note, hash_hour, hour_order, hour_tier, priority, unit_key};

/// One distinct rendered composition of one hour.
#[derive(Clone, Debug)]
pub struct Unit {
    pub form: PrayerForm,
    pub hash: String,
    pub hour: &'static str,
    pub unit_key: String,
    pub name: String,
    pub rank: Option<Rank>,
    pub season: Season,
    /// The earliest date this composition occurs.
    pub date: Date,
    pub occurrences: usize,
    pub context: String,
}

impl Unit {
    pub fn priority(&self) -> &'static str {
        priority(self.rank, self.date)
    }
}

pub struct Manifest {
    pub start_year: i32,
    pub years: i32,
    pub units: Vec<Unit>,
}

fn form_index(f: PrayerForm) -> usize {
    PrayerForm::ALL.iter().position(|x| *x == f).unwrap_or(0)
}

/// Sweeps `[start, start+years)` into deduplicated review units.
pub fn build_manifest(src: &dyn DataSource, start: i32, years: i32) -> Result<Manifest, String> {
    let engine = Engine::load(src).map_err(|e| format!("creating office engine: {e}"))?;
    let cal = CalendarData::load(src).map_err(|e| format!("building calendar for {start}: {e}"))?;
    let mut by_hash: HashMap<String, usize> = HashMap::new();
    let mut units: Vec<Unit> = Vec::new();
    let mut fold = |u: Unit| match by_hash.get(&u.hash) {
        Some(&i) => units[i].occurrences += 1,
        None => {
            by_hash.insert(u.hash.clone(), units.len());
            units.push(u);
        }
    };
    let calendar_error = |y: i32, e: String| format!("building calendar for {y}: {e}");
    sweep(
        &engine,
        &cal,
        start,
        years,
        Forms::All,
        &calendar_error,
        &|day, hour_name, hour| Unit {
            hash: hash_hour(hour),
            form: hour.form,
            hour: hour_name,
            unit_key: unit_key(day, hour_name),
            name: celebration_name(day),
            rank: celebration_rank(day, hour_name),
            season: day.season,
            date: day.date,
            occurrences: 1,
            context: context_note(day, hour_name),
        },
        &mut fold,
    )?;
    units.sort_by(|a, b| {
        a.priority()
            .cmp(b.priority())
            .then_with(|| hour_tier(a.hour).cmp(&hour_tier(b.hour)))
            // Recurring compositions repay review effort fastest.
            .then_with(|| b.occurrences.cmp(&a.occurrences))
            .then_with(|| b.rank.map_or(0, Rank::weight).cmp(&a.rank.map_or(0, Rank::weight)))
            .then_with(|| a.unit_key.cmp(&b.unit_key))
            .then_with(|| hour_order(a.hour).cmp(&hour_order(b.hour)))
            .then_with(|| a.date.cmp(&b.date))
            .then_with(|| form_index(a.form).cmp(&form_index(b.form)))
            .then_with(|| a.hash.cmp(&b.hash))
    });
    Ok(Manifest { start_year: start, years, units })
}

/// The reviewer checklist, one row per unit.
pub fn manifest_csv(m: &Manifest, base: &str) -> String {
    let base = base_url(base);
    let mut out = String::new();
    csv::write_record(
        &mut out,
        &["priority", "hour", "date", "unit_key", "celebration", "rank", "season", "context", "occurrences", "url"],
    );
    for u in &m.units {
        let date = u.date.to_string();
        let occurrences = u.occurrences.to_string();
        let url = format!("{base}/{}/{date}?form={}", u.hour, u.form.as_str());
        csv::write_record(
            &mut out,
            &[
                u.priority(),
                u.hour,
                &date,
                &u.unit_key,
                &u.name,
                u.rank.map_or("", Rank::abbrev),
                u.season.as_str(),
                &u.context,
                &occurrences,
                &url,
            ],
        );
    }
    out
}
