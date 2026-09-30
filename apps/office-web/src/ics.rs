//! The `/office.ics` reminder feed: stateless, configured entirely by its query.

use axum::body::Body;
use axum::http::{HeaderMap, Response, StatusCode, header};
use jiff::tz::TimeZone;
use jiff::{SignedDuration, Timestamp};
use presentation::{date_slug, reminder_description, reminder_summary};

use crate::Server;
use crate::http::{Query, header_value, http_error, response, set};
use crate::web_time::{ics_stamp, load_location, parse_clock, wall_time};

/// The hours in liturgical order, so a day's events are in sequence
/// whatever the query's order.
const HOUR_ORDER: [&str; 7] = ["lauds", "prime", "terce", "sext", "none", "vespers", "compline"];

const DEFAULT_HORIZON_DAYS: i64 = 60;
const MAX_HORIZON_DAYS: i64 = 366;

/// The reminder schedule encoded in a subscription URL.
struct IcsConfig {
    hours: Vec<(&'static str, u32, u32)>,
    /// Indexed by weekday, Sunday first.
    days: [bool; 7],
    /// Minutes before the start; `None` disables the alarm.
    alarm: Option<i64>,
    tz: TimeZone,
    horizon: i64,
}

fn day_number(name: &str) -> Option<usize> {
    ["sun", "mon", "tue", "wed", "thu", "fri", "sat"].iter().position(|d| *d == name)
}

/// Three-letter day names or ranges ("mon-fri,sun"); a range may wrap the
/// week's end ("sat-sun").
fn parse_days(spec: &str) -> Result<[bool; 7], String> {
    if spec.is_empty() {
        return Ok([true; 7]);
    }
    let mut days = [false; 7];
    for token in spec.split(',') {
        let token = token.trim().to_lowercase();
        if let Some((from, to)) = token.split_once('-') {
            let (Some(start), Some(end)) = (day_number(from), day_number(to)) else {
                return Err(format!("invalid day range {}", data_format::quote(&token)));
            };
            let mut d = start;
            loop {
                days[d] = true;
                if d == end {
                    break;
                }
                d = (d + 1) % 7;
            }
            continue;
        }
        let Some(d) = day_number(&token) else {
            return Err(format!("invalid day {}", data_format::quote(&token)));
        };
        days[d] = true;
    }
    Ok(days)
}

fn parse_config(q: &Query) -> Result<IcsConfig, String> {
    let mut hours = Vec::new();
    for name in HOUR_ORDER {
        let v = q.get(name);
        if v.is_empty() {
            continue;
        }
        let Some((hh, mm)) = parse_clock(v) else {
            return Err(format!("invalid time {} for {name} — use HH:MM (24-hour)", data_format::quote(v)));
        };
        hours.push((name, hh, mm));
    }
    if hours.is_empty() {
        return Err("no hours configured — set at least one, e.g. ?lauds=06:45".into());
    }
    let days = parse_days(q.get("days"))?;
    let mut alarm = Some(10);
    let v = q.get("alarm");
    if v == "none" {
        alarm = None;
    } else if !v.is_empty() {
        match data_format::atoi(v) {
            Ok(n) if (0..=24 * 60).contains(&n) => alarm = Some(n),
            _ => return Err(format!("invalid alarm {} — minutes before the hour, or \"none\"", data_format::quote(v))),
        }
    }
    let mut tz = TimeZone::UTC;
    let v = q.get("tz");
    if !v.is_empty() {
        tz = load_location(v)
            .ok_or_else(|| format!("unknown timezone {} — use an IANA name like America/New_York", data_format::quote(v)))?;
    }
    let mut horizon = DEFAULT_HORIZON_DAYS;
    let v = q.get("horizon");
    if !v.is_empty() {
        match data_format::atoi(v) {
            Ok(n) if (1..=MAX_HORIZON_DAYS).contains(&n) => horizon = n,
            _ => return Err(format!("invalid horizon {} — days from 1 to {MAX_HORIZON_DAYS}", data_format::quote(v))),
        }
    }
    Ok(IcsConfig { hours, days, alarm, tz, horizon })
}

/// A text value escaped per RFC 5545 §3.3.11.
fn escape_ics(s: &str) -> String {
    s.replace('\\', "\\\\").replace(';', "\\;").replace(',', "\\,").replace('\n', "\\n")
}

/// Folds a content line at 75 octets, continued with CRLF and a space,
/// breaking only between characters.
fn fold_line(out: &mut String, line: &str) {
    let mut line = line;
    let mut limit = 75;
    while line.len() > limit {
        let mut cut = limit;
        while cut > 0 && !line.is_char_boundary(cut) {
            cut -= 1;
        }
        out.push_str(&line[..cut]);
        out.push_str("\r\n ");
        line = &line[cut..];
        // The continuation space counts toward the limit.
        limit = 74;
    }
    out.push_str(line);
    out.push_str("\r\n");
}

impl Server {
    fn build_ics(&self, cfg: &IcsConfig, base_url: &str, now: Timestamp) -> Result<String, String> {
        let mut out = String::new();
        let mut write = |line: &str| fold_line(&mut out, line);
        for line in [
            "BEGIN:VCALENDAR",
            "VERSION:2.0",
            "PRODID:-//AWRV Divine Office//office//EN",
            "CALSCALE:GREGORIAN",
            "METHOD:PUBLISH",
            "X-WR-CALNAME:Divine Office",
            "X-WR-CALDESC:Hours of the Benedictine Office",
            "REFRESH-INTERVAL;VALUE=DURATION:P1D",
            "X-PUBLISHED-TTL:P1D",
        ] {
            write(line);
        }
        let dtstamp = ics_stamp(now);
        // Civil dates, so a missing local clock time cannot move a date.
        let local = now.to_zoned(cfg.tz.clone());
        let start = calendar::Date::new(i32::from(local.year()), i32::from(local.month()), i32::from(local.day()));
        for i in 0..cfg.horizon {
            let date = start.add_days(i as i32);
            if !cfg.days[date.weekday().number() as usize] {
                continue;
            }
            let entry = self.cache.get(date.year())?;
            let Some(day) = entry.days.get(date.ordinal() as usize - 1) else { continue };
            let slug = date_slug(date);
            let desc = reminder_description(day);
            for &(name, hh, mm) in &cfg.hours {
                let begin = wall_time(&cfg.tz, date, hh, mm);
                let end = begin.checked_add(SignedDuration::from_mins(15)).map_err(|e| e.to_string())?;
                let summary = reminder_summary(name, day);
                write("BEGIN:VEVENT");
                write(&format!("UID:{name}-{slug}@awrv-office"));
                write(&format!("DTSTAMP:{dtstamp}"));
                write(&format!("DTSTART:{}", ics_stamp(begin)));
                write(&format!("DTEND:{}", ics_stamp(end)));
                write(&format!("SUMMARY:{}", escape_ics(&summary)));
                write(&format!("DESCRIPTION:{}", escape_ics(&desc)));
                write(&format!("URL:{base_url}/{name}/{slug}"));
                if let Some(alarm) = cfg.alarm {
                    write("BEGIN:VALARM");
                    write("ACTION:DISPLAY");
                    write(&format!("DESCRIPTION:{}", escape_ics(&summary)));
                    write(&if alarm == 0 { "TRIGGER:PT0M".to_string() } else { format!("TRIGGER:-PT{alarm}M") });
                    write("END:VALARM");
                }
                write("END:VEVENT");
            }
        }
        write("END:VCALENDAR");
        Ok(out)
    }

    /// `/office.ics`.
    pub fn ics(&self, query: &Query, headers: &HeaderMap, host: &str) -> Response<Body> {
        let cfg = match parse_config(query) {
            Ok(c) => c,
            Err(e) => return http_error(&e, StatusCode::BAD_REQUEST),
        };
        // The external base URL, honoring the proxy's X-Forwarded-Proto.
        let scheme = match header_value(headers, "x-forwarded-proto") {
            "" => "http",
            s => s,
        };
        let body = match self.build_ics(&cfg, &format!("{scheme}://{host}"), Timestamp::now()) {
            Ok(b) => b,
            Err(e) => return http_error(&format!("error building calendar: {e}"), StatusCode::INTERNAL_SERVER_ERROR),
        };
        let mut resp = response(StatusCode::OK, body);
        set(&mut resp, header::CONTENT_TYPE, "text/calendar; charset=utf-8");
        set(&mut resp, header::CACHE_CONTROL, "no-cache");
        resp
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::test_server;

    fn config(q: &str) -> Result<IcsConfig, String> {
        parse_config(&Query::parse(q))
    }

    fn at(tz: &TimeZone, y: i16, m: i8, d: i8, h: i8, min: i8) -> Timestamp {
        jiff::civil::date(y, m, d).at(h, min, 0, 0).to_zoned(tz.clone()).unwrap().timestamp()
    }

    fn ics(q: &str, now: Timestamp) -> String {
        test_server().build_ics(&config(q).unwrap(), "https://office.example", now).unwrap()
    }

    #[test]
    fn parse_days_cases() {
        assert_eq!(parse_days("").unwrap(), [true; 7]);
        let wk = parse_days("mon-fri").unwrap();
        assert!(!wk[0] && !wk[6] && wk[1] && wk[5], "{wk:?}");
        let wrap = parse_days("sat-sun").unwrap();
        assert!(wrap[6] && wrap[0] && !wrap[1], "{wrap:?}");
        let mixed = parse_days("mon-wed,sat").unwrap();
        assert!(mixed[2] && mixed[6] && !mixed[5], "{mixed:?}");
        assert!(parse_days("monday").is_err());
        assert!(parse_days(" Mon , wed").unwrap()[3]);
        assert!(parse_days("mon-xyz").is_err());
        assert!(parse_days("mon,").is_err());
    }

    #[test]
    fn config_validation() {
        for bad in [
            "",
            "lauds=6am",
            "lauds=06:45&days=xx",
            "lauds=06:45&alarm=-5",
            "lauds=06:45&alarm=abc",
            "lauds=06:45&tz=Nowhere/Nowhere",
            "lauds=06:45&horizon=0",
            "lauds=06:45&horizon=9999",
        ] {
            assert!(config(bad).is_err(), "expected an error for {bad:?}");
        }
        let cfg = config("vespers=18:00&lauds=06:45&alarm=none&tz=America/New_York&days=mon-fri&horizon=30").unwrap();
        let names: Vec<&str> = cfg.hours.iter().map(|h| h.0).collect();
        assert_eq!(names, ["lauds", "vespers"], "hours in canonical order");
        assert_eq!(cfg.alarm, None);
        assert_eq!(cfg.tz.iana_name(), Some("America/New_York"));
        assert_eq!(cfg.horizon, 30);
    }

    #[test]
    fn build_ics() {
        let ny = crate::web_time::zone("America/New_York").unwrap();
        // Christmas 2026 (a Friday) falls inside the horizon.
        let body = ics("lauds=06:45&tz=America/New_York&horizon=7&alarm=15", at(&ny, 2026, 12, 20, 12, 0));
        assert!(body.starts_with("BEGIN:VCALENDAR\r\n") && body.ends_with("END:VCALENDAR\r\n"));
        assert_eq!(body.matches("BEGIN:VEVENT").count(), 7);
        for want in [
            "SUMMARY:Lauds — Nativity of Our Lord Jesus Christ",
            // 06:45 EST is 11:45 UTC.
            "DTSTART:20261225T114500Z",
            "UID:lauds-2026-12-25@awrv-office",
            "TRIGGER:-PT15M",
            "URL:https://office.example/lauds/2026-12-25",
        ] {
            assert!(body.contains(want), "missing {want:?}");
        }
        for (i, line) in body.split("\r\n").enumerate() {
            assert!(line.len() <= 75, "line {} exceeds 75 octets: {line:?}", i + 1);
        }
    }

    #[test]
    fn day_filter_and_no_alarm() {
        // 2026-06-08 is a Monday.
        let body = ics("vespers=18:00&days=sun&alarm=none&horizon=14", at(&TimeZone::UTC, 2026, 6, 8, 12, 0));
        assert_eq!(body.matches("BEGIN:VEVENT").count(), 2);
        assert!(!body.contains("BEGIN:VALARM"));
        assert!(body.contains("DTSTART:20260614T180000Z"));
    }

    #[test]
    fn spans_year_boundary() {
        let body = ics("compline=21:00&horizon=10", at(&TimeZone::UTC, 2026, 12, 28, 12, 0));
        assert!(body.contains("UID:compline-2027-01-03@awrv-office"));
        assert_eq!(body.matches("BEGIN:VEVENT").count(), 10);
    }

    #[test]
    fn escapes_text() {
        assert_eq!(escape_ics("a;b,c\\d\ne"), r"a\;b\,c\\d\ne");
    }

    #[test]
    fn folds_lines() {
        for line in [String::new(), "a".repeat(75), "a".repeat(150), "a".repeat(225), "a".repeat(74) + &"é".repeat(100), "🕯".repeat(80)]
        {
            let mut folded = String::new();
            fold_line(&mut folded, &line);
            for physical in folded.split("\r\n") {
                assert!(physical.len() <= 75, "physical line has {} bytes", physical.len());
            }
            assert_eq!(folded.strip_suffix("\r\n").unwrap().replace("\r\n ", ""), line, "unfolding changed the line");
        }
    }

    /// Santiago's September 6 begins at 01:00. The civil dates must not
    /// repeat September 5 or skip the 6th.
    #[test]
    fn dates_across_midnight_dst() {
        let cfg = config("lauds=06:45&tz=America/Santiago&horizon=3").unwrap();
        let now = at(&cfg.tz, 2026, 9, 5, 0, 30);
        let body = test_server().build_ics(&cfg, "https://office.example", now).unwrap();
        for date in ["2026-09-05", "2026-09-06", "2026-09-07"] {
            assert_eq!(body.matches(&format!("UID:lauds-{date}@awrv-office\r\n")).count(), 1, "{date}");
        }
        for start in ["20260905T104500Z", "20260906T094500Z", "20260907T094500Z"] {
            assert!(body.contains(&format!("DTSTART:{start}\r\n")), "missing {start}");
        }
    }
}
