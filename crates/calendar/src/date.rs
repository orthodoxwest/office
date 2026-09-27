//! A proleptic Gregorian civil date with no time of day and no time zone. Weekdays are numbered
//! from Sunday = 0. Out-of-range months and days normalize by carrying into the next unit (February
//! 30 is March 2 in a common year).

use std::fmt;

/// Day of the week, numbered from Sunday = 0.
#[derive(Clone, Copy, Debug, PartialEq, Eq, PartialOrd, Ord, Hash)]
pub enum Weekday {
    Sunday,
    Monday,
    Tuesday,
    Wednesday,
    Thursday,
    Friday,
    Saturday,
}

impl Weekday {
    pub const ALL: [Weekday; 7] =
        [Weekday::Sunday, Weekday::Monday, Weekday::Tuesday, Weekday::Wednesday, Weekday::Thursday, Weekday::Friday, Weekday::Saturday];

    /// Days since Sunday.
    pub fn number(self) -> i32 {
        match self {
            Weekday::Sunday => 0,
            Weekday::Monday => 1,
            Weekday::Tuesday => 2,
            Weekday::Wednesday => 3,
            Weekday::Thursday => 4,
            Weekday::Friday => 5,
            Weekday::Saturday => 6,
        }
    }

    pub fn from_number(n: i32) -> Weekday {
        Weekday::ALL[n.rem_euclid(7) as usize]
    }

    /// The English weekday name.
    pub fn name(self) -> &'static str {
        match self {
            Weekday::Sunday => "Sunday",
            Weekday::Monday => "Monday",
            Weekday::Tuesday => "Tuesday",
            Weekday::Wednesday => "Wednesday",
            Weekday::Thursday => "Thursday",
            Weekday::Friday => "Friday",
            Weekday::Saturday => "Saturday",
        }
    }
}

impl fmt::Display for Weekday {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(self.name())
    }
}

/// A civil date, stored as days since 1970-01-01.
#[derive(Clone, Copy, Debug, PartialEq, Eq, PartialOrd, Ord, Hash)]
pub struct Date {
    days: i64,
}

// Howard Hinnant's days_from_civil / civil_from_days.
fn days_from_civil(y: i64, m: i64, d: i64) -> i64 {
    let y = if m <= 2 { y - 1 } else { y };
    let era = y.div_euclid(400);
    let yoe = y - era * 400;
    let mp = (m + 9) % 12;
    let doy = (153 * mp + 2) / 5 + d - 1;
    let doe = yoe * 365 + yoe / 4 - yoe / 100 + doy;
    era * 146097 + doe - 719468
}

fn civil_from_days(z: i64) -> (i64, u32, u32) {
    let z = z + 719468;
    let era = z.div_euclid(146097);
    let doe = z - era * 146097;
    let yoe = (doe - doe / 1460 + doe / 36524 - doe / 146096) / 365;
    let y = yoe + era * 400;
    let doy = doe - (365 * yoe + yoe / 4 - yoe / 100);
    let mp = (5 * doy + 2) / 153;
    let d = (doy - (153 * mp + 2) / 5 + 1) as u32;
    let m = if mp < 10 { mp + 3 } else { mp - 9 } as u32;
    (if m <= 2 { y + 1 } else { y }, m, d)
}

impl Date {
    /// Builds a date with normalization: a month outside 1–12 carries into the year, and a day
    /// outside the month carries into the neighboring months.
    pub fn new(year: i32, month: i32, day: i32) -> Date {
        let m0 = i64::from(month) - 1;
        let y = i64::from(year) + m0.div_euclid(12);
        let m = m0.rem_euclid(12) + 1;
        Date { days: days_from_civil(y, m, 1) + i64::from(day) - 1 }
    }

    /// Parses `YYYY-MM-DD`, rejecting dates that do not exist.
    pub fn parse(s: &str) -> Option<Date> {
        let b = s.as_bytes();
        if b.len() != 10 || b[4] != b'-' || b[7] != b'-' {
            return None;
        }
        let digits = |r: std::ops::Range<usize>| -> Option<i32> {
            let part = &s[r];
            if !part.bytes().all(|c| c.is_ascii_digit()) {
                return None;
            }
            part.parse().ok()
        };
        let (y, m, d) = (digits(0..4)?, digits(5..7)?, digits(8..10)?);
        let date = Date::new(y, m, d);
        (date.year() == y && date.month() as i32 == m && date.day() as i32 == d).then_some(date)
    }

    pub fn year(self) -> i32 {
        civil_from_days(self.days).0 as i32
    }

    pub fn month(self) -> u32 {
        civil_from_days(self.days).1
    }

    pub fn day(self) -> u32 {
        civil_from_days(self.days).2
    }

    pub fn weekday(self) -> Weekday {
        // 1970-01-01 was a Thursday.
        Weekday::from_number((self.days + 4).rem_euclid(7) as i32)
    }

    /// Day of the year, 1-based.
    pub fn ordinal(self) -> u32 {
        (self.days - Date::new(self.year(), 1, 1).days + 1) as u32
    }

    pub fn add_days(self, n: i32) -> Date {
        Date { days: self.days + i64::from(n) }
    }

    /// Whole days from `earlier` to `self` (`self.Sub(earlier) / 24h`).
    pub fn days_since(self, earlier: Date) -> i32 {
        (self.days - earlier.days) as i32
    }
}

impl fmt::Display for Date {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let (y, m, d) = civil_from_days(self.days);
        write!(f, "{y:04}-{m:02}-{d:02}")
    }
}

pub fn is_leap_year(year: i32) -> bool {
    if year % 400 == 0 {
        return true;
    }
    if year % 100 == 0 {
        return false;
    }
    year % 4 == 0
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn round_trips_and_weekdays() {
        let d = Date::new(2026, 4, 12);
        assert_eq!(d.to_string(), "2026-04-12");
        assert_eq!(d.weekday(), Weekday::Sunday);
        assert_eq!(Date::new(1970, 1, 1).weekday(), Weekday::Thursday);
        assert_eq!(Date::new(2000, 12, 31).ordinal(), 366);
        for n in -1000..1000 {
            let d = Date::new(2026, 1, 1).add_days(n * 37);
            assert_eq!(Date::new(d.year(), d.month() as i32, d.day() as i32), d);
        }
    }

    #[test]
    fn normalizes_out_of_range_dates() {
        assert_eq!(Date::new(2026, 2, 30).to_string(), "2026-03-02");
        assert_eq!(Date::new(2026, 2, 29).to_string(), "2026-03-01");
        assert_eq!(Date::new(2026, 13, 1).to_string(), "2027-01-01");
        assert_eq!(Date::new(2026, 0, 1).to_string(), "2025-12-01");
        assert_eq!(Date::new(2026, 1, 0).to_string(), "2025-12-31");
    }

    #[test]
    fn parse_rejects_impossible_dates() {
        assert_eq!(Date::parse("2024-02-29"), Some(Date::new(2024, 2, 29)));
        assert_eq!(Date::parse("2026-02-29"), None);
        assert_eq!(Date::parse("2026-2-01"), None);
    }
}
