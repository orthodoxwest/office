//! Building a year of office days from a data source: the calendar, then the
//! Office-only resolution (Vespers, the Marian antiphon) of each day.

use calendar::{CalendarData, DataSource, build_calendar};
use office::day::Day;
use office::resolve_office_days;

/// Every office day of `year` from already loaded calendar data.
pub fn office_days(data: &CalendarData, year: i32) -> Result<Vec<Day>, String> {
    let cal = build_calendar(year, data)?;
    let office = resolve_office_days(&cal);
    Ok(cal.days.into_iter().zip(office).map(|(c, o)| Day::new(c, o)).collect())
}

/// Go's `calendar.BuildCalendar(year, dataDir)` plus the office days.
pub fn load_office_days(src: &dyn DataSource, year: i32) -> Result<Vec<Day>, String> {
    office_days(&CalendarData::load(src)?, year)
}
