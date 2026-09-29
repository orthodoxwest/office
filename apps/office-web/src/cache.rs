//! Per-year calendar days and ordo summaries, composed a month at a time: a small window of
//! recently used years, not an archive.

use std::collections::HashMap;
use std::sync::{Arc, Mutex, OnceLock};

use calendar::{CalendarData, MoveableDates};
use office::Engine;
use office::day::Day;
use render_html::view::MonthData;

/// One year's office days, with each month's ordo rows built on first use.
pub struct YearEntry {
    pub days: Vec<Day>,
    pub moveable: MoveableDates,
    months: [OnceLock<Arc<MonthData>>; 12],
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
    // Feast data is loaded once at startup and stays fixed for the lifetime of this cache.
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
        let entry = Arc::new(YearEntry { days, moveable: MoveableDates::compute(year), months: Default::default() });
        if order.len() == MAX_CACHED_YEARS {
            let oldest = order.remove(0);
            entries.remove(&oldest);
        }
        entries.insert(year, Arc::clone(&entry));
        order.push(year);
        Ok(entry)
    }

    /// A month's ordo rows (`month` is 1-based), composed once outside the
    /// cache lock: readers of the same month share the work, other requests
    /// proceed, and a month page never waits on the rest of its year.
    pub fn month(&self, year: i32, month: u32, engine: &Engine) -> Result<Arc<MonthData>, String> {
        let entry = self.get(year)?;
        Ok(entry.month(month, engine))
    }

    /// Every month of the year, for the whole-year page.
    pub fn months(&self, year: i32, engine: &Engine) -> Result<Vec<Arc<MonthData>>, String> {
        let entry = self.get(year)?;
        Ok((1..=12).map(|m| entry.month(m, engine)).collect())
    }
}

impl YearEntry {
    fn month(&self, month: u32, engine: &Engine) -> Arc<MonthData> {
        Arc::clone(self.months[month as usize - 1].get_or_init(|| {
            let start = self.days.iter().position(|d| d.date.month() == month).unwrap_or(self.days.len());
            let len = self.days[start..].iter().take_while(|d| d.date.month() == month).count();
            Arc::new(crate::handlers::build_month(&self.days[start..start + len], engine, &self.moveable))
        }))
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::test_server;

    fn cache() -> YearCache {
        YearCache::new(CalendarData::load(&tools::fs::FsData::new("../../data")).unwrap())
    }

    #[test]
    fn calendar_cache_shares_composed_months() {
        let cache = cache();
        let engine = &test_server().engine;
        let results: Vec<Arc<MonthData>> = std::thread::scope(|s| {
            let handles: Vec<_> = (0..6).map(|_| s.spawn(|| cache.month(2026, 9, engine).unwrap())).collect();
            handles.into_iter().map(|h| h.join().unwrap()).collect()
        });
        for month in &results {
            assert!(Arc::ptr_eq(month, &results[0]), "concurrent requests did not reuse the composed month");
        }
        let entry = cache.get(2026).unwrap();
        let september: Vec<Day> = entry.days.iter().filter(|d| d.date.month() == 9).cloned().collect();
        assert_eq!(*results[0], crate::handlers::build_month(&september, engine, &entry.moveable));
        assert_eq!((results[0].name.as_str(), results[0].days.len()), ("September", 30));
        let year = cache.months(2026, engine).unwrap();
        assert_eq!(year.iter().map(|m| m.days.len()).sum::<usize>(), 365);
        assert!(Arc::ptr_eq(&year[8], &results[0]), "the whole year reuses composed months");
        assert_eq!(year[0].name, "January");
        assert_eq!(year[11].days.last().map(|d| d.day_num), Some(31));
    }

    #[test]
    fn eviction_preserves_active_readers() {
        let cache = cache();
        let first = cache.get(2026).unwrap();
        for year in 2027..=2026 + MAX_CACHED_YEARS as i32 {
            cache.get(year).unwrap();
        }
        {
            let entries = cache.entries.lock().unwrap();
            assert_eq!(entries.years.len(), MAX_CACHED_YEARS);
            assert!(!entries.years.contains_key(&2026), "oldest year was not evicted");
        }
        assert!(first.days.len() == 365 && first.days[0].date.year() == 2026, "eviction invalidated an active reader");
        let again = cache.get(2026).unwrap();
        assert!(!Arc::ptr_eq(&again, &first));
        assert_eq!(again.days.len(), first.days.len());
        assert!(again.days.iter().zip(&first.days).all(|(a, b)| a.date == b.date && a.celebration_id() == b.celebration_id()));
    }

    #[test]
    fn bounds_retention_and_keeps_recent_years() {
        let cache = cache();
        let load = |year: i32| {
            let e = cache.get(year).unwrap();
            assert!(e.days.len() >= 365 && e.days[0].date.year() == year && e.moveable.easter.year() == year, "wrong calendar for {year}");
            e
        };
        let first = load(2026);
        let second = load(2027);
        for year in 2028..2026 + MAX_CACHED_YEARS as i32 {
            load(year);
        }
        assert!(Arc::ptr_eq(&load(2026), &first), "cached year was rebuilt");
        load(2026 + MAX_CACHED_YEARS as i32);
        assert!(Arc::ptr_eq(&load(2026), &first), "recently used year was evicted");
        assert!(!Arc::ptr_eq(&load(2027), &second), "least recently used year was retained beyond the limit");
        // Eviction drops only the cache's reference.
        assert_eq!(second.days[0].date.year(), 2027);
    }
}
