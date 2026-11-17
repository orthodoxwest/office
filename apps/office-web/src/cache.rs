//! Per-year calendar days and ordo summaries, composed a month at a time: a small window of
//! recently used years, not an archive.

use std::collections::HashMap;
use std::sync::{Arc, Mutex, OnceLock};

use calendar::{CalendarData, MoveableDates, build_calendar};
use office::day::Day;
use office::{Engine, resolve_office_days};
use presentation::feast::{FeastIndex, YearIndex};
use presentation::rails::{Rail, octave_rails};
use render_html::view::MonthData;

/// One year's office days, with each month's ordo rows built on first use.
pub struct YearEntry {
    pub days: Vec<Day>,
    /// Each day's rail, the octaves bound (`presentation::rails`).
    pub rails: Vec<Rail>,
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

impl Entries {
    /// The cached year, moved to the back of the order of use.
    fn used(&mut self, year: i32) -> Option<Arc<YearEntry>> {
        let e = Arc::clone(self.years.get(&year)?);
        self.order.retain(|y| *y != year);
        self.order.push(year);
        Some(e)
    }
}

pub struct YearCache {
    data: CalendarData,
    entries: Mutex<Entries>,
    feasts: FeastIndex,
    /// Each year's feasts and how they are kept, for search and the feast pages: the calendar
    /// alone, with no hour composed.
    keepings: Mutex<HashMap<i32, Arc<YearIndex>>>,
}

/// The feast pages reach this many years ahead and search two; a reader browsing far years
/// starts the window afresh rather than growing it.
const MAX_KEEPING_YEARS: usize = 12;

impl YearCache {
    pub fn new(data: CalendarData) -> YearCache {
        let feasts = FeastIndex::new(&data);
        YearCache { data, entries: Mutex::new(Entries::default()), feasts, keepings: Mutex::new(HashMap::new()) }
    }

    /// Every feast a reader can look up.
    pub fn feasts(&self) -> &FeastIndex {
        &self.feasts
    }

    /// How every feast is kept in `year`.
    pub fn keeping(&self, year: i32) -> Result<Arc<YearIndex>, String> {
        if let Some(y) = self.keepings.lock().map_err(|e| e.to_string())?.get(&year) {
            return Ok(Arc::clone(y));
        }
        // Built outside the lock: another year's reader does not wait on this one.
        let built = Arc::new(YearIndex::new(year, build_calendar(year, &self.data)?.days));
        let mut keepings = self.keepings.lock().map_err(|e| e.to_string())?;
        if keepings.len() >= MAX_KEEPING_YEARS {
            keepings.clear();
        }
        Ok(Arc::clone(keepings.entry(year).or_insert(built)))
    }

    /// The year's days, building them if the year is not cached. A year in
    /// use moves to the back so browsing older years does not evict it.
    // Feast data is loaded once at startup and stays fixed for the lifetime of this cache.
    ///
    /// A missing year is built outside the lock, as `keeping` builds: building takes a
    /// fraction of a second on a laptop and seconds on the server's shared CPU, and every
    /// hour, ordo and feast page asks this cache for its year, so building under the lock
    /// held every reader of every year until it finished. Two readers of the same new year
    /// may both build it; the first to finish is kept.
    pub fn get(&self, year: i32) -> Result<Arc<YearEntry>, String> {
        if let Some(e) = self.cached(year)? {
            return Ok(e);
        }
        let cal = build_calendar(year, &self.data)?;
        let rails = octave_rails(&cal, &self.data.feasts);
        let office = resolve_office_days(&cal);
        let days = cal.days.into_iter().zip(office).map(|(c, o)| Day::new(c, o)).collect();
        let entry = Arc::new(YearEntry { days, rails, moveable: MoveableDates::compute(year), months: Default::default() });
        let mut guard = self.entries.lock().map_err(|e| e.to_string())?;
        if let Some(e) = guard.used(year) {
            return Ok(e);
        }
        let Entries { years: entries, order } = &mut *guard;
        if order.len() == MAX_CACHED_YEARS {
            let oldest = order.remove(0);
            entries.remove(&oldest);
        }
        entries.insert(year, Arc::clone(&entry));
        order.push(year);
        Ok(entry)
    }

    fn cached(&self, year: i32) -> Result<Option<Arc<YearEntry>>, String> {
        Ok(self.entries.lock().map_err(|e| e.to_string())?.used(year))
    }

    /// A month's ordo rows (`month` is 1-based), composed once outside the
    /// cache lock: readers of the same month share the work, other requests
    /// proceed, and a month page never waits on the rest of its year.
    pub fn month(&self, year: i32, month: u32, engine: &Engine) -> Result<Arc<MonthData>, String> {
        let entry = self.get(year)?;
        Ok(entry.month(month, engine, &self.feasts))
    }

    /// Every month of the year, for the whole-year page.
    pub fn months(&self, year: i32, engine: &Engine) -> Result<Vec<Arc<MonthData>>, String> {
        let entry = self.get(year)?;
        Ok((1..=12).map(|m| entry.month(m, engine, &self.feasts)).collect())
    }
}

impl YearEntry {
    fn month(&self, month: u32, engine: &Engine, feasts: &FeastIndex) -> Arc<MonthData> {
        Arc::clone(self.months[month as usize - 1].get_or_init(|| {
            let start = self.days.iter().position(|d| d.date.month() == month).unwrap_or(self.days.len());
            let len = self.days[start..].iter().take_while(|d| d.date.month() == month).count();
            let range = start..start + len;
            Arc::new(crate::handlers::build_month(&self.days[range.clone()], &self.rails[range], engine, &self.moveable, feasts))
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
        let start = entry.days.iter().position(|d| d.date.month() == 9).unwrap();
        let september = start..start + 30;
        assert_eq!(
            *results[0],
            crate::handlers::build_month(&entry.days[september.clone()], &entry.rails[september], engine, &entry.moveable, cache.feasts())
        );
        assert_eq!((results[0].name.as_str(), results[0].days.len()), ("September", 30));
        let year = cache.months(2026, engine).unwrap();
        assert_eq!(year.iter().map(|m| m.days.len()).sum::<usize>(), 365);
        assert!(Arc::ptr_eq(&year[8], &results[0]), "the whole year reuses composed months");
        assert_eq!(year[0].name, "January");
        assert_eq!(year[11].days.last().map(|d| d.day_num), Some(31));
    }

    #[test]
    fn a_cached_year_is_not_held_while_another_builds() {
        let cache = cache();
        cache.get(2026).unwrap();
        let (cached_at, built_at) = std::thread::scope(|s| {
            let builder = s.spawn(|| {
                for year in 2101..2104 {
                    cache.get(year).unwrap();
                }
                std::time::Instant::now()
            });
            std::thread::sleep(std::time::Duration::from_millis(20));
            cache.get(2026).unwrap();
            (std::time::Instant::now(), builder.join().unwrap())
        });
        assert!(cached_at < built_at, "a reader of a cached year waited for other years to be built");
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
