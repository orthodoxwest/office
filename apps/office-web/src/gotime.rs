//! Go's `time` behavior at the server's edges: the layouts it parses and
//! prints, `time.LoadLocation`, and `time.Date` in a zone.

use std::sync::OnceLock;

use calendar::Date;
use jiff::Timestamp;
use jiff::tz::{TimeZone, TimeZoneDatabase};

/// The zones Go's server can load: the Go build embeds the full tz
/// database (`time/tzdata`), link names included, and jiff bundles the
/// same. A host's zoneinfo may be partial (Debian moved the legacy links
/// to `tzdata-legacy`), so it is not consulted.
fn tzdb() -> &'static TimeZoneDatabase {
    static DB: OnceLock<TimeZoneDatabase> = OnceLock::new();
    DB.get_or_init(TimeZoneDatabase::bundled)
}

/// A zone by its IANA name, as `time.LoadLocation` finds it.
pub fn zone(name: &str) -> Option<TimeZone> {
    let tz = tzdb().get(name).ok()?;
    // PORT(inherited): Go reads zone files by exact name; jiff's lookup
    // ignores case, so "america/new_york" must still fail.
    (tz.iana_name() == Some(name)).then_some(tz)
}

/// Go's `time.LoadLocation`: "" and "UTC" are UTC, "Local" is the host
/// zone, and other names are looked up exactly as spelled.
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

/// The host zone, Go's `time.Local`.
pub fn local() -> TimeZone {
    TimeZone::system()
}

/// The civil date and hour of now in `tz`.
pub fn now_in(tz: &TimeZone) -> (Date, i8) {
    let z = Timestamp::now().to_zoned(tz.clone());
    (Date::new(i32::from(z.year()), i32::from(z.month()), i32::from(z.day())), z.hour())
}

/// Go's `getnum`: one or two digits, exactly two when `fixed`.
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

/// Go's `time.Parse("2006-01-02", s)`: a four-digit year, two-digit month
/// and day, and a day that exists in that month.
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

/// Go's `time.Parse("15:04", s)`: the hour and minute.
pub fn parse_clock(s: &str) -> Option<(u32, u32)> {
    let (hour, rest) = getnum(s.as_bytes(), false)?;
    let rest = rest.strip_prefix(b":")?;
    let (minute, rest) = getnum(rest, true)?;
    (rest.is_empty() && hour < 24 && minute < 60).then_some((hour, minute))
}

/// Go's `2006` year field: at least four digits, the sign outside them.
fn go_year(y: i32) -> String {
    if y < 0 { format!("-{:04}", -i64::from(y)) } else { format!("{y:04}") }
}

/// `Format("2006-01-02")`.
pub fn date_slug(d: Date) -> String {
    format!("{}-{:02}-{:02}", go_year(d.year()), d.month(), d.day())
}

const MONTHS: [&str; 12] =
    ["January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"];

pub fn month_name(d: Date) -> &'static str {
    MONTHS[d.month() as usize - 1]
}

/// `Format("Monday, January 2, 2006")`.
pub fn long_date(d: Date) -> String {
    format!("{}, {} {}, {}", d.weekday().name(), month_name(d), d.day(), go_year(d.year()))
}

/// Go's `time.Date(y, m, d, hh, mm, 0, 0, loc)`: the offset at the wall
/// time read as UTC, corrected once by the offset at the instant that gives.
/// A wall time skipped by a spring-forward change lands an hour early
/// (02:30 becomes 01:30 standard time); a repeated one takes its first
/// occurrence.
pub fn wall_time(tz: &TimeZone, date: Date, hour: u32, minute: u32) -> Timestamp {
    let epoch = Date::new(1970, 1, 1);
    let unix = i64::from(date.days_since(epoch)) * 86_400 + i64::from(hour) * 3600 + i64::from(minute) * 60;
    let offset_at = |secs: i64| Timestamp::from_second(secs).map(|t| i64::from(tz.to_offset(t).seconds())).unwrap_or(0);
    let first = offset_at(unix);
    let second = if first == 0 { 0 } else { offset_at(unix - first) };
    Timestamp::from_second(unix - second).unwrap_or(Timestamp::UNIX_EPOCH)
}

/// `t.UTC().Format("20060102T150405Z")`.
pub fn ics_stamp(t: Timestamp) -> String {
    t.to_zoned(TimeZone::UTC).strftime("%Y%m%dT%H%M%SZ").to_string()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn dates_parse_as_go_parses_them() {
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
    fn clocks_parse_as_go_parses_them() {
        assert_eq!(parse_clock("06:45"), Some((6, 45)));
        assert_eq!(parse_clock("6:45"), Some((6, 45)));
        assert_eq!(parse_clock("24:00"), None);
        assert_eq!(parse_clock("6:5"), None);
        assert_eq!(parse_clock("12:60"), None);
        assert_eq!(parse_clock("123:45"), None);
    }

    #[test]
    fn dates_print_as_go_prints_them() {
        assert_eq!(date_slug(Date::new(-1, 1, 1)), "-0001-01-01");
        assert_eq!(date_slug(Date::new(202, 1, 1)), "0202-01-01");
        assert_eq!(date_slug(Date::new(10000, 1, 1)), "10000-01-01");
        assert_eq!(long_date(Date::new(2026, 3, 11)), "Wednesday, March 11, 2026");
    }

    #[test]
    fn zones_load_as_go_loads_them() {
        assert!(load_location("America/New_York").is_some());
        assert!(load_location("america/new_york").is_none());
        assert!(load_location("US/Eastern").is_some());
        assert!(load_location("Nowhere/City").is_none());
        assert!(load_location("../etc/passwd").is_none());
        assert_eq!(load_location(""), Some(TimeZone::UTC));
    }

    #[test]
    fn wall_times_resolve_as_go_resolves_them() {
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
