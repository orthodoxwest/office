//! Civil dates, clock input, time zones, and reminder instants at the server boundary.

use std::sync::OnceLock;

use calendar::Date;
use jiff::Timestamp;
use jiff::tz::{TimeZone, TimeZoneDatabase};

/// Use the bundled time-zone database, including link names. The host's zoneinfo may be partial, so
/// it is not consulted.
fn tzdb() -> &'static TimeZoneDatabase {
    static DB: OnceLock<TimeZoneDatabase> = OnceLock::new();
    DB.get_or_init(TimeZoneDatabase::bundled)
}

/// A zone by its case-sensitive IANA name.
pub fn zone(name: &str) -> Option<TimeZone> {
    let tz = tzdb().get(name).ok()?;
    // Zone names are case-sensitive; jiff itself accepts names with different casing.
    (tz.iana_name() == Some(name)).then_some(tz)
}

/// `""` and `"UTC"` select UTC, `"Local"` selects the host zone, and other names are looked up
/// exactly as spelled.
pub fn load_location(name: &str) -> Option<TimeZone> {
    match name {
        "" | "UTC" => return Some(TimeZone::UTC),
        "Local" => return Some(TimeZone::system()),
        _ => {}
    }
    if name.contains("..") || name.starts_with('/') || name.starts_with('\\') {
        return None;
    }
    zone(name)
}

/// The host time zone.
pub fn local() -> TimeZone {
    TimeZone::system()
}

/// The civil date and hour of now in `tz`.
pub fn now_in(tz: &TimeZone) -> (Date, i8) {
    let z = Timestamp::now().to_zoned(tz.clone());
    (Date::new(i32::from(z.year()), i32::from(z.month()), i32::from(z.day())), z.hour())
}

/// One or two digits, exactly two when `fixed`.
fn getnum(s: &[u8], fixed: bool) -> Option<(u32, &[u8])> {
    let d0 = s.first().filter(|c| c.is_ascii_digit())?;
    match s.get(1).filter(|c| c.is_ascii_digit()) {
        None if fixed => None,
        None => Some((u32::from(d0 - b'0'), &s[1..])),
        Some(d1) => Some((u32::from(d0 - b'0') * 10 + u32::from(d1 - b'0'), &s[2..])),
    }
}

fn days_in(month: u32, year: i32) -> u32 {
    match month {
        2 if calendar::date::is_leap_year(year) => 29,
        2 => 28,
        4 | 6 | 9 | 11 => 30,
        _ => 31,
    }
}

/// Parses a four-digit year, two-digit month and day, and a day that exists in that month.
pub fn parse_date(s: &str) -> Option<Date> {
    let b = s.as_bytes();
    if b.len() < 4 || !b[..4].iter().all(u8::is_ascii_digit) {
        return None;
    }
    let year = b[..4].iter().fold(0i32, |n, d| n * 10 + i32::from(d - b'0'));
    let rest = b[4..].strip_prefix(b"-")?;
    let (month, rest) = getnum(rest, true)?;
    let rest = rest.strip_prefix(b"-")?;
    let (day, rest) = getnum(rest, true)?;
    if !rest.is_empty() || !(1..=12).contains(&month) || day < 1 || day > days_in(month, year) {
        return None;
    }
    Some(Date::new(year, month as i32, day as i32))
}

/// Parses an hour (one or two digits) and a two-digit minute, separated by a colon.
pub fn parse_clock(s: &str) -> Option<(u32, u32)> {
    let (hour, rest) = getnum(s.as_bytes(), false)?;
    let rest = rest.strip_prefix(b":")?;
    let (minute, rest) = getnum(rest, true)?;
    (rest.is_empty() && hour < 24 && minute < 60).then_some((hour, minute))
}

/// At least four year digits, with the sign outside them.
fn format_year(y: i32) -> String {
    if y < 0 { format!("-{:04}", -i64::from(y)) } else { format!("{y:04}") }
}

/// `Format("2006-01-02")`.
pub fn date_slug(d: Date) -> String {
    format!("{}-{:02}-{:02}", format_year(d.year()), d.month(), d.day())
}

const MONTHS: [&str; 12] =
    ["January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"];

pub fn month_name(d: Date) -> &'static str {
    MONTHS[d.month() as usize - 1]
}

/// `Format("Monday, January 2, 2006")`.
pub fn long_date(d: Date) -> String {
    format!("{}, {} {}, {}", d.weekday().name(), month_name(d), d.day(), format_year(d.year()))
}

/// Resolves a reminder wall time by reading it as UTC, finding the zone offset there, and
/// correcting once using the offset at the resulting instant. At DST gaps and overlaps the choice
/// depends on the zone: New York takes the earlier instant, while London takes the later one. These
/// choices keep subscribed reminder times stable.
pub fn wall_time(tz: &TimeZone, date: Date, hour: u32, minute: u32) -> Timestamp {
    let epoch = Date::new(1970, 1, 1);
    let unix = i64::from(date.days_since(epoch)) * 86_400 + i64::from(hour) * 3600 + i64::from(minute) * 60;
    let offset_at = |secs: i64| Timestamp::from_second(secs).map(|t| i64::from(tz.to_offset(t).seconds())).unwrap_or(0);
    let first = offset_at(unix);
    let second = if first == 0 { 0 } else { offset_at(unix - first) };
    Timestamp::from_second(unix - second).unwrap_or(Timestamp::UNIX_EPOCH)
}

/// A UTC timestamp in iCalendar format (`YYYYMMDDTHHMMSSZ`).
pub fn ics_stamp(t: Timestamp) -> String {
    t.to_zoned(TimeZone::UTC).strftime("%Y%m%dT%H%M%SZ").to_string()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn dates_require_valid_ymd() {
        assert_eq!(parse_date("2026-03-11"), Some(Date::new(2026, 3, 11)));
        assert_eq!(parse_date("2024-02-29"), Some(Date::new(2024, 2, 29)));
        assert_eq!(parse_date("2026-02-29"), None);
        assert_eq!(parse_date("+202-01-01"), None);
        assert_eq!(parse_date("-001-01-01"), None);
        assert_eq!(parse_date("2026-1-01"), None);
        assert_eq!(parse_date("2026-01-01x"), None);
        assert_eq!(parse_date("20260101"), None);
        assert_eq!(parse_date("2026-13-01"), None);
        assert_eq!(parse_date("2026-00-01"), None);
        assert_eq!(parse_date("0000-01-01"), Some(Date::new(0, 1, 1)));
    }

    #[test]
    fn clocks_require_valid_hour_and_minute() {
        assert_eq!(parse_clock("06:45"), Some((6, 45)));
        assert_eq!(parse_clock("6:45"), Some((6, 45)));
        assert_eq!(parse_clock("24:00"), None);
        assert_eq!(parse_clock("6:5"), None);
        assert_eq!(parse_clock("12:60"), None);
        assert_eq!(parse_clock("123:45"), None);
    }

    #[test]
    fn dates_use_padded_years() {
        assert_eq!(date_slug(Date::new(-1, 1, 1)), "-0001-01-01");
        assert_eq!(date_slug(Date::new(202, 1, 1)), "0202-01-01");
        assert_eq!(date_slug(Date::new(10000, 1, 1)), "10000-01-01");
        assert_eq!(long_date(Date::new(2026, 3, 11)), "Wednesday, March 11, 2026");
    }

    #[test]
    fn zones_require_exact_names() {
        assert!(load_location("America/New_York").is_some());
        assert!(load_location("america/new_york").is_none());
        assert!(load_location("US/Eastern").is_some());
        assert!(load_location("Nowhere/City").is_none());
        assert!(load_location("../etc/passwd").is_none());
        assert_eq!(load_location(""), Some(TimeZone::UTC));
    }

    #[test]
    fn reminder_instants_at_dst_transitions() {
        let ny = load_location("America/New_York").unwrap();
        // The spring-forward gap resolves 02:30 to 01:30 EST: 06:30Z.
        assert_eq!(ics_stamp(wall_time(&ny, Date::new(2026, 3, 8), 2, 30)), "20260308T063000Z");
        // The repeated hour in November resolves to EDT: 05:30Z.
        assert_eq!(ics_stamp(wall_time(&ny, Date::new(2026, 11, 1), 1, 30)), "20261101T053000Z");
        assert_eq!(ics_stamp(wall_time(&ny, Date::new(2026, 7, 1), 6, 45)), "20260701T104500Z");
        let london = load_location("Europe/London").unwrap();
        assert_eq!(ics_stamp(wall_time(&london, Date::new(2026, 3, 29), 1, 30)), "20260329T013000Z");
        assert_eq!(ics_stamp(wall_time(&london, Date::new(2026, 10, 25), 1, 30)), "20261025T013000Z");
    }
}
