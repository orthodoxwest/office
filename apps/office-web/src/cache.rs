//! Per-year calendar days and calendar-page summaries. Ported from Go's
//! `web/cache.go`: a small window of recently used years, not an archive.

use std::collections::HashMap;
use std::sync::{Arc, Mutex, OnceLock};

use calendar::{CalendarData, MoveableDates};
use office::Engine;
use office::day::Day;
use render_html::view::MonthData;

/// One year's office days, with its calendar page built on first use.
pub struct YearEntry {
    pub days: Vec<Day>,
    pub moveable: MoveableDates,
    months: OnceLock<Arc<Vec<MonthData>>>,
}

const MAX_CACHED_YEARS: usize = 8;

/// The cached years and their order of use, least recent first.
#[derive(Default)]
struct Entries {
    years: HashMap<i32, Arc<YearEntry>>,
    order: Vec<i32>,
}

pub struct YearCache {
    data: CalendarData,
    entries: Mutex<Entries>,
}

impl YearCache {
    pub fn new(data: CalendarData) -> YearCache {
        YearCache { data, entries: Mutex::new(Entries::default()) }
    }

    /// The year's days, building them if the year is not cached. A year in
    /// use moves to the back so browsing older years does not evict it.
    // PORT(inherited): Go rereads the feast files for every year it builds;
    // Rust loads them once at startup, as the data directory is fixed for a
    // deploy.
    pub fn get(&self, year: i32) -> Result<Arc<YearEntry>, String> {
        let mut guard = self.entries.lock().map_err(|e| e.to_string())?;
        let Entries { years: entries, order } = &mut *guard;
        if let Some(e) = entries.get(&year) {
            let e = Arc::clone(e);
            order.retain(|y| *y != year);
            order.push(year);
            return Ok(e);
        }
        let days = tools::year::office_days(&self.data, year)?;
        let entry = Arc::new(YearEntry { days, moveable: MoveableDates::compute(year), months: OnceLock::new() });
        if order.len() == MAX_CACHED_YEARS {
            let oldest = order.remove(0);
            entries.remove(&oldest);
        }
        entries.insert(year, Arc::clone(&entry));
        order.push(year);
        Ok(entry)
    }

    /// The year's calendar page rows, composed once outside the cache lock:
    /// readers of the same year share the work, other requests proceed.
    pub fn months(&self, year: i32, engine: &Engine) -> Result<Arc<Vec<MonthData>>, String> {
        let entry = self.get(year)?;
        Ok(Arc::clone(entry.months.get_or_init(|| Arc::new(crate::handlers::build_month_data(&entry.days, engine, &entry.moveable)))))
    }
}
