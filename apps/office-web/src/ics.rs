//! The `/office.ics` reminder feed: stateless, configured entirely by its
//! query. Ported from Go's `web/ics.go`.

use axum::body::Body;
use axum::http::{HeaderMap, Response, StatusCode, header};
use jiff::tz::TimeZone;
use jiff::{SignedDuration, Timestamp};
use render_html::links::title_case;

use crate::Server;
use crate::gonet::{Query, header_value, http_error, response, set};
use crate::gotime::{date_slug, ics_stamp, load_location, parse_clock, wall_time};

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
                return Err(format!("invalid day range {}", compat::quote(&token)));
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
            return Err(format!("invalid day {}", compat::quote(&token)));
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
            return Err(format!("invalid time {} for {name} — use HH:MM (24-hour)", compat::quote(v)));
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
        match compat::atoi(v) {
            Ok(n) if (0..=24 * 60).contains(&n) => alarm = Some(n),
            _ => return Err(format!("invalid alarm {} — minutes before the hour, or \"none\"", compat::quote(v))),
        }
    }
    let mut tz = TimeZone::UTC;
    let v = q.get("tz");
    if !v.is_empty() {
        tz = load_location(v).ok_or_else(|| format!("unknown timezone {} — use an IANA name like America/New_York", compat::quote(v)))?;
    }
    let mut horizon = DEFAULT_HORIZON_DAYS;
    let v = q.get("horizon");
    if !v.is_empty() {
        match compat::atoi(v) {
            Ok(n) if (1..=MAX_HORIZON_DAYS).contains(&n) => horizon = n,
            _ => return Err(format!("invalid horizon {} — days from 1 to {MAX_HORIZON_DAYS}", compat::quote(v))),
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
            let feast = crate::handlers::celebration_name(day);
            let mut parts = Vec::new();
            if let Some(c) = &day.celebration {
                parts.push(c.rank.display_name().to_string());
            }
            parts.push(title_case(day.season.as_str()));
            parts.push(day.color.as_str().to_string());
            parts.extend(day.commemorations.iter().map(|c| format!("Comm. {}", c.name)));
            let desc = parts.join(" · ");
            for &(name, hh, mm) in &cfg.hours {
                let begin = wall_time(&cfg.tz, date, hh, mm);
                let end = begin.checked_add(SignedDuration::from_mins(15)).map_err(|e| e.to_string())?;
                let summary = format!("{} — {feast}", title_case(name));
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

    #[test]
    fn days_parse_as_go_parses_them() {
        assert_eq!(parse_days("").unwrap(), [true; 7]);
        assert_eq!(parse_days("sat-mon").unwrap(), [true, true, false, false, false, false, true]);
        assert_eq!(parse_days(" Mon , wed").unwrap(), [false, true, false, true, false, false, false]);
        assert!(parse_days("mon-xyz").is_err());
        assert!(parse_days("mon,").is_err());
    }

    #[test]
    fn lines_fold_at_75_octets() {
        let mut out = String::new();
        let line = "X".repeat(74) + "é" + &"Y".repeat(80);
        fold_line(&mut out, &line);
        let lines: Vec<&str> = out.split("\r\n").collect();
        assert_eq!(lines[0].len(), 74);
        assert!(lines.iter().all(|l| l.len() <= 75));
    }
}
