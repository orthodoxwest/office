//! Fasting and abstinence rules (`penitential.txt`).

use std::collections::HashMap;

use crate::date::{Date, Weekday};
use crate::loader::Section;
use crate::model::CalendarDay;
use data_format::atoi;
use data_format::quote;

/// One `[rule]` of `penitential.txt`. Later rules override earlier ones.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct PenitentialRule {
    pub id: String,
    pub from: String,
    pub to: String,
    /// Empty means every weekday.
    pub weekdays: Vec<Weekday>,
    pub fast: Option<bool>,
    pub abstinence: Option<bool>,
}

/// Parses booleans: 1/0 and true/false in lowercase, uppercase, or title case.
fn parse_bool(s: &str) -> Option<bool> {
    match s {
        "1" | "t" | "T" | "TRUE" | "true" | "True" => Some(true),
        "0" | "f" | "F" | "FALSE" | "false" | "False" => Some(false),
        _ => None,
    }
}

const KNOWN_KEYS: [&str; 5] = ["From", "To", "Weekdays", "Fast", "Abstinence"];

pub(crate) fn section_to_penitential_rule(m: &Section, source_file: &str) -> Result<PenitentialRule, String> {
    let id = m.id.clone();
    if id.is_empty() {
        return Err(format!("{source_file}: rule missing ID"));
    }
    let qid = quote(&id);
    let from = m.get("From").unwrap_or("").to_string();
    let to = m.get("To").unwrap_or("").to_string();
    if from.is_empty() || to.is_empty() {
        return Err(format!("{source_file}: rule {qid} must have From and To"));
    }
    let mut rule = PenitentialRule { id, from, to, weekdays: Vec::new(), fast: None, abstinence: None };

    let raw = m.get("Weekdays").unwrap_or("");
    if !raw.is_empty() {
        rule.weekdays = parse_weekdays(raw).map_err(|e| format!("{source_file}: rule {qid}: {e}"))?;
    }
    for (key, slot) in [("Fast", &mut rule.fast), ("Abstinence", &mut rule.abstinence)] {
        if let Some(raw) = m.get(key) {
            *slot = Some(parse_bool(raw).ok_or_else(|| format!("{source_file}: rule {qid}: invalid {key} value {}", quote(raw)))?);
        }
    }
    if rule.fast.is_none() && rule.abstinence.is_none() {
        return Err(format!("{source_file}: rule {qid} must set Fast and/or Abstinence"));
    }
    // Report the first unknown key in byte order.
    if let Some(key) = m.values.keys().find(|k| !KNOWN_KEYS.contains(&k.as_str())) {
        return Err(format!("{source_file}: rule {qid}: unrecognized key {}", quote(key)));
    }
    Ok(rule)
}

fn parse_weekdays(raw: &str) -> Result<Vec<Weekday>, String> {
    let mut days = Vec::new();
    for part in raw.split(',') {
        let name = part.to_lowercase();
        let name = name.trim();
        if name.is_empty() {
            continue;
        }
        let Some(day) = Weekday::ALL.into_iter().find(|d| d.name().eq_ignore_ascii_case(name)) else {
            return Err(format!("invalid weekday {}", quote(part)));
        };
        if !days.contains(&day) {
            days.push(day);
        }
    }
    if days.is_empty() {
        return Err("no valid weekdays configured".to_string());
    }
    Ok(days)
}

/// Splits `anchor@+N`. `date:` anchors never take an offset.
fn split_anchor_offset(raw: &str) -> Result<(&str, i32), String> {
    if raw.starts_with("date:") {
        return Ok((raw, 0));
    }
    // ^(.*)@([+-]\d+)$
    let Some(at) = raw.rfind('@') else { return Ok((raw, 0)) };
    let suffix = &raw[at + 1..];
    let digits = suffix.strip_prefix(['+', '-']).unwrap_or("");
    if suffix.len() == digits.len() || digits.is_empty() || !digits.bytes().all(|b| b.is_ascii_digit()) {
        return Ok((raw, 0));
    }
    let offset = atoi(suffix).ok().and_then(|n| i32::try_from(n).ok()).ok_or_else(|| format!("invalid offset in {}", quote(raw)))?;
    Ok((&raw[..at], offset))
}

fn resolve_anchor(raw: &str, year: i32, feast_dates: &HashMap<String, Date>) -> Result<Date, String> {
    let (anchor, offset) = split_anchor_offset(raw)?;
    let date = if let Some(id) = anchor.strip_prefix("feast:") {
        *feast_dates.get(id).ok_or_else(|| format!("unknown feast anchor {}", quote(id)))?
    } else if let Some(md) = anchor.strip_prefix("date:") {
        let parts: Vec<&str> = md.split('-').collect();
        if parts.len() != 2 {
            return Err(format!("invalid date anchor {}", quote(raw)));
        }
        let month = atoi(parts[0]).map_err(|_| format!("invalid month in {}", quote(raw)))?;
        let day = atoi(parts[1]).map_err(|_| format!("invalid day in {}", quote(raw)))?;
        // Out-of-range dates are normalized by carrying into the neighboring month or year.
        Date::new(year, month.clamp(-100_000, 100_000) as i32, day.clamp(-10_000_000, 10_000_000) as i32)
    } else {
        return Err(format!("invalid anchor {}", quote(raw)));
    };
    Ok(date.add_days(offset))
}

/// Applies the rules to a civil year of days, then the two fixed disciplines:
/// no fast on Sunday, and every fast day is also a day of abstinence.
pub(crate) fn apply_penitential_rules(
    days: &mut [CalendarDay],
    rules: &[PenitentialRule],
    feast_dates: &HashMap<String, Date>,
) -> Result<(), String> {
    let Some(first) = days.first() else { return Ok(()) };
    let year = first.date.year();
    let start_of_year = first.date;
    for rule in rules {
        let qid = quote(&rule.id);
        let start = resolve_anchor(&rule.from, year, feast_dates).map_err(|e| format!("rule {qid}: {e}"))?;
        let end = resolve_anchor(&rule.to, year, feast_dates).map_err(|e| format!("rule {qid}: {e}"))?;
        if end < start {
            return Err(format!("rule {qid}: To precedes From"));
        }
        let mut current = start;
        while current <= end {
            if rule.weekdays.is_empty() || rule.weekdays.contains(&current.weekday()) {
                let idx = current.days_since(start_of_year);
                if idx >= 0 && (idx as usize) < days.len() {
                    let day = &mut days[idx as usize];
                    if let Some(fast) = rule.fast {
                        day.penitential.fast = fast;
                    }
                    if let Some(abstinence) = rule.abstinence {
                        day.penitential.abstinence = abstinence;
                    }
                }
            }
            current = current.add_days(1);
        }
    }
    for day in days.iter_mut() {
        if day.date.weekday() == Weekday::Sunday {
            day.penitential.fast = false;
        }
        if day.penitential.fast {
            day.penitential.abstinence = true;
        }
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn anchors_and_weekdays() {
        assert_eq!(split_anchor_offset("feast:easter-sunday@+5"), Ok(("feast:easter-sunday", 5)));
        assert_eq!(split_anchor_offset("feast:a@b@-2"), Ok(("feast:a@b", -2)));
        assert_eq!(split_anchor_offset("feast:x@5"), Ok(("feast:x@5", 0)));
        assert_eq!(split_anchor_offset("date:01-01@+1"), Ok(("date:01-01@+1", 0)));
        let dates = HashMap::from([("x".to_string(), Date::new(2026, 4, 12))]);
        assert_eq!(resolve_anchor("feast:x@-1", 2026, &dates), Ok(Date::new(2026, 4, 11)));
        assert_eq!(resolve_anchor("date:02-30", 2026, &dates), Ok(Date::new(2026, 3, 2)));
        assert_eq!(resolve_anchor("feast:y", 2026, &dates), Err("unknown feast anchor \"y\"".to_string()));
        assert_eq!(parse_weekdays("Monday, friday,"), Ok(vec![Weekday::Monday, Weekday::Friday]));
        assert_eq!(parse_weekdays("funday"), Err("invalid weekday \"funday\"".to_string()));
        assert_eq!(parse_weekdays(" , "), Err("no valid weekdays configured".to_string()));
    }
}
