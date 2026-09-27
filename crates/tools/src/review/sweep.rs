//! The review sweep: compose every hour of every day of a window of years, in every prayer form,
//! keeping one witness per distinct composition. Years compose in parallel; each composition is
//! mapped to a small record on its worker and the records are folded in deterministic order (year,
//! day, hour, form), so the reports stay deterministic and memory stays bounded by the years in
//! flight.

use std::collections::{BTreeMap, HashSet};
use std::sync::atomic::{AtomicUsize, Ordering};

use calendar::{CalendarData, MoveableDates};
use liturgy::{OfficeHour, PrayerForm};
use office::day::Day;
use office::engine::{Engine, HOUR_NAMES};

use super::hash_hour;

/// Composes one hour in every form, keeping the first of each distinct
/// composition, so identical deacon and priest forms count once.
pub fn compose_review_forms(engine: &Engine, hour_name: &str, day: &Day, moveable: &MoveableDates) -> Result<Vec<OfficeHour>, String> {
    let mut out = Vec::new();
    let mut seen = HashSet::new();
    for form in PrayerForm::ALL {
        let hour = engine.compose_hour(hour_name, day, moveable, form)?;
        if seen.insert(hash_hour(&hour)) {
            out.push(hour);
        }
    }
    Ok(out)
}

/// Which compositions a sweep visits.
#[derive(Clone, Copy, PartialEq, Eq)]
pub enum Forms {
    /// Every distinct form (`compose_review_forms`).
    All,
    /// Private prayer only.
    Private,
}

/// Maps every composition in `[start, start+years)` and folds the records in sweep order, each year
/// as soon as the years before it are folded. `calendar_error` formats a failure to build a year;
/// the first failure in sweep order is returned.
#[allow(clippy::too_many_arguments)]
pub fn sweep<T: Send>(
    engine: &Engine,
    cal: &CalendarData,
    start: i32,
    years: i32,
    forms: Forms,
    calendar_error: &(dyn Fn(i32, String) -> String + Sync),
    map: &(dyn Fn(&Day, &'static str, &OfficeHour) -> T + Sync),
    fold: &mut dyn FnMut(T),
) -> Result<(), String> {
    let count = usize::try_from(years.max(0)).unwrap_or(0);
    let next = AtomicUsize::new(0);
    let workers = std::thread::available_parallelism().map_or(1, |n| n.get()).min(count.max(1));
    std::thread::scope(|scope| {
        let (tx, rx) = std::sync::mpsc::channel::<(usize, Result<Vec<T>, String>)>();
        for _ in 0..workers {
            let tx = tx.clone();
            let next = &next;
            scope.spawn(move || {
                loop {
                    let i = next.fetch_add(1, Ordering::Relaxed);
                    if i >= count {
                        return;
                    }
                    let result = sweep_year(engine, cal, start + i as i32, forms, calendar_error, map);
                    let failed = result.is_err();
                    if tx.send((i, result)).is_err() || failed {
                        // Later years are moot once one fails.
                        next.store(count, Ordering::Relaxed);
                        return;
                    }
                }
            });
        }
        drop(tx);
        let mut pending: BTreeMap<usize, Result<Vec<T>, String>> = BTreeMap::new();
        let mut want = 0;
        for (i, result) in rx {
            pending.insert(i, result);
            while let Some(result) = pending.remove(&want) {
                for record in result? {
                    fold(record);
                }
                want += 1;
            }
        }
        if want < count {
            // A worker stopped early after an earlier year failed; that
            // failure is the first in order unless a still earlier year is
            // missing, which cannot happen since years are claimed in order.
            return Err(pending.into_values().find_map(Result::err).unwrap_or_else(|| "sweep stopped early".to_string()));
        }
        Ok(())
    })
}

fn sweep_year<T>(
    engine: &Engine,
    cal: &CalendarData,
    year: i32,
    forms: Forms,
    calendar_error: &(dyn Fn(i32, String) -> String + Sync),
    map: &(dyn Fn(&Day, &'static str, &OfficeHour) -> T + Sync),
) -> Result<Vec<T>, String> {
    let days = crate::year::office_days(cal, year).map_err(|e| calendar_error(year, e))?;
    let moveable = MoveableDates::compute(year);
    let mut out = Vec::new();
    for day in &days {
        for hour_name in HOUR_NAMES {
            let composed = match forms {
                Forms::All => compose_review_forms(engine, hour_name, day, &moveable),
                Forms::Private => engine.compose_hour(hour_name, day, &moveable, PrayerForm::Private).map(|h| vec![h]),
            }
            .map_err(|e| format!("composing {hour_name} for {}: {e}", day.date))?;
            for hour in &composed {
                out.push(map(day, hour_name, hour));
            }
        }
    }
    Ok(out)
}
