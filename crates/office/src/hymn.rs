//! The Sunday Lauds hymn's summer window. Ported from Go's `hymn.go`.

use calendar::{Date, MoveableDates};

/// The Sunday nearest October 1.
fn sunday_nearest_october1(year: i32) -> Date {
    let oct1 = Date::new(year, 10, 1);
    let wd = oct1.weekday().number();
    if wd <= 3 { oct1.add_days(-wd) } else { oct1.add_days(7 - wd) }
}

/// The summer hymn runs from the II Sunday after Trinity until the Sunday
/// nearest October 1.
pub fn sunday_lauds_hymn_is_summer(date: Date) -> bool {
    let m = MoveableDates::compute(date.year());
    date >= m.trinity_sunday.add_days(14) && date < sunday_nearest_october1(date.year())
}
