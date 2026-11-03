//! The summer windows of the Sunday Lauds hymn and of the Saturday Vespers hymn's tune.

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

/// The Saturday nearest October 1.
fn saturday_nearest_october1(year: i32) -> Date {
    let oct1 = Date::new(year, 10, 1);
    let ahead = (6 - oct1.weekday().number() + 7) % 7;
    if ahead <= 3 { oct1.add_days(ahead) } else { oct1.add_days(ahead - 7) }
}

/// Saturday Vespers' *O lux beata Trinitas* takes its summer tune "from the Saturday before the
/// III Sunday after Pentecost until the Saturday nearest the first day of October", and its
/// winter tune from that Saturday (Vespers for Saturdays & Major Feasts, pp. 218–219).
pub fn saturday_vespers_hymn_is_summer(date: Date) -> bool {
    let m = MoveableDates::compute(date.year());
    date >= m.trinity_sunday.add_days(13) && date < saturday_nearest_october1(date.year())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn saturday_vespers_summer_runs_to_the_saturday_nearest_october_1() {
        // 2025: 1 October is a Wednesday, so the nearest Saturday is the 4th.
        assert!(saturday_vespers_hymn_is_summer(Date::new(2025, 9, 27)));
        assert!(!saturday_vespers_hymn_is_summer(Date::new(2025, 10, 4)));
        // 2026: Trinity falls on 7 June (Julian Pascha 12 April); 1 October is a Thursday.
        assert!(!saturday_vespers_hymn_is_summer(Date::new(2026, 6, 13)));
        assert!(saturday_vespers_hymn_is_summer(Date::new(2026, 6, 20)));
        assert!(saturday_vespers_hymn_is_summer(Date::new(2026, 9, 26)));
        assert!(!saturday_vespers_hymn_is_summer(Date::new(2026, 10, 3)));
        // When 1 October is a Sunday, Monday or Tuesday, the nearest Saturday falls in September.
        assert!(saturday_vespers_hymn_is_summer(Date::new(2028, 9, 23)));
        assert!(!saturday_vespers_hymn_is_summer(Date::new(2028, 9, 30)));
        assert!(saturday_vespers_hymn_is_summer(Date::new(2030, 9, 21)));
        assert!(!saturday_vespers_hymn_is_summer(Date::new(2030, 9, 28)));
        // 2033: 1 October is itself a Saturday, and winter.
        assert!(saturday_vespers_hymn_is_summer(Date::new(2033, 9, 24)));
        assert!(!saturday_vespers_hymn_is_summer(Date::new(2033, 10, 1)));
    }
}
