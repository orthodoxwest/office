//! `office-rs dump`: the canonical record stream the Go and Rust engines are
//! compared on (RUST-PORT.md, "Dump format"). The Go generator in
//! `internal/dump/records.go` is the reference field list.

use std::io::Write;

use calendar::{CalendarData, CalendarDay, Date, Decision, Feast, MoveableDates, Tabula, build_calendar};
use office::texts::OfficeTexts;
use serde_json::{Value, json};

use crate::args::{Flags, split_list};
use tools::fs::FsData;

pub const FORMAT: &str = "office-dump/2";

const GROUP_CORPUS: &str = "corpus";
const GROUP_CALENDAR: &str = "calendar";
const GROUP_OFFICE: &str = "office";
const GROUP_HOURS: &str = "hours";
const GROUPS: [&str; 4] = [GROUP_CORPUS, GROUP_CALENDAR, GROUP_OFFICE, GROUP_HOURS];
const HOUR_NAMES: [&str; 7] = ["lauds", "prime", "terce", "sext", "none", "vespers", "compline"];
const PRAYER_FORMS: [&str; 3] = ["private", "deacon", "priest"];

/// Groups this engine can produce so far.
const PORTED_GROUPS: [&str; 3] = [GROUP_CORPUS, GROUP_CALENDAR, GROUP_OFFICE];

const USAGE: &str = "usage: office-rs dump -start YEAR [-years N] [-hours LIST] [-forms LIST] [-groups LIST]
       office-rs dump -dates YYYY-MM-DD,... [-hours LIST] [-forms LIST] [-groups LIST]";

/// A slice of the record stream: whole civil years or individual dates.
#[derive(Clone, Debug, Default)]
pub struct Selection {
    pub start_year: i32,
    pub years: i32,
    pub dates: Vec<Date>,
    pub hours: Vec<String>,
    pub forms: Vec<String>,
    pub groups: Vec<String>,
}

/// The members of `order` that appear in `chosen`, in `order`'s sequence, or
/// all of `order` when nothing was chosen.
fn canonical_subset(what: &str, chosen: &[String], order: &[&str]) -> Result<Vec<String>, String> {
    if chosen.is_empty() {
        return Ok(order.iter().map(|s| s.to_string()).collect());
    }
    if let Some(c) = chosen.iter().find(|c| !order.contains(&c.as_str())) {
        return Err(format!("unknown {what} {} (want one of [{}])", compat::quote(c), order.join(" ")));
    }
    Ok(order.iter().filter(|o| chosen.iter().any(|c| c == *o)).map(|s| s.to_string()).collect())
}

impl Selection {
    fn normalize(mut self) -> Result<Selection, String> {
        if !self.dates.is_empty() {
            if self.start_year != 0 || self.years != 0 {
                return Err("select either dates or a year window, not both".to_string());
            }
            self.dates.sort();
            self.dates.dedup();
        } else if self.start_year < 1 || self.years < 1 {
            return Err("select dates or a year window (start year and at least one year)".to_string());
        }
        self.hours = canonical_subset("hour", &self.hours, &HOUR_NAMES)?;
        self.groups = canonical_subset("record group", &self.groups, &GROUPS)?;
        self.forms = canonical_subset("prayer form", &self.forms, &PRAYER_FORMS)?;
        Ok(self)
    }

    fn has(&self, group: &str) -> bool {
        self.groups.iter().any(|g| g == group)
    }

    fn meta(&self) -> Value {
        let (start, years, dates) = if self.dates.is_empty() {
            (json!(self.start_year), json!(self.years), Value::Null)
        } else {
            (Value::Null, Value::Null, json!(self.dates.iter().map(Date::to_string).collect::<Vec<_>>()))
        };
        json!({
            "kind": "meta",
            "format": FORMAT,
            "selection": {
                "start_year": start, "years": years, "dates": dates,
                "hours": self.hours, "forms": self.forms, "groups": self.groups,
            },
        })
    }

    /// Civil years with the dates to emit (`None` = every day).
    fn plan(&self) -> Vec<(i32, Option<Vec<Date>>)> {
        if self.dates.is_empty() {
            return (self.start_year..self.start_year + self.years).map(|y| (y, None)).collect();
        }
        let mut plans: Vec<(i32, Option<Vec<Date>>)> = Vec::new();
        for &d in &self.dates {
            match plans.last_mut() {
                Some((y, Some(dates))) if *y == d.year() => dates.push(d),
                _ => plans.push((d.year(), Some(vec![d]))),
            }
        }
        plans
    }
}

pub fn cmd_dump(data: &FsData, args: &[String], out: &mut dyn Write) -> Result<(), String> {
    let flags = Flags::parse(args, &["start", "years", "dates", "hours", "forms", "groups"]).map_err(|e| format!("{e}\n{USAGE}"))?;
    if !flags.rest.is_empty() {
        return Err(USAGE.to_string());
    }
    let start = flags.int("start", 0)?;
    let years = flags.int("years", 1)?;
    let mut sel = Selection {
        hours: split_list(flags.str("hours")),
        forms: split_list(flags.str("forms")),
        groups: split_list(flags.str("groups")),
        ..Selection::default()
    };
    let dates = flags.str("dates");
    if !dates.is_empty() {
        if start != 0 {
            return Err("use -dates or -start, not both".to_string());
        }
        for value in split_list(dates) {
            sel.dates.push(Date::parse(&value).ok_or_else(|| format!("invalid date (use YYYY-MM-DD): {value}"))?);
        }
    } else {
        sel.start_year = i32::try_from(start).map_err(|_| "start year out of range".to_string())?;
        sel.years = i32::try_from(years).map_err(|_| "years out of range".to_string())?;
    }
    let sel = sel.normalize()?;
    if let Some(g) = sel.groups.iter().find(|g| !PORTED_GROUPS.contains(&g.as_str())) {
        return Err(format!("the Rust engine does not produce the {g} record group yet (use -groups {})", PORTED_GROUPS.join(",")));
    }
    let texts =
        if sel.has(GROUP_CORPUS) { Some(office::texts::load_texts(data).map_err(|e| format!("loading text corpus: {e}"))?) } else { None };
    let data = CalendarData::load(data)?;
    generate(&sel, &data, texts.as_ref(), &mut |record| {
        let line = marshal(&record).map_err(|e| format!("{}: {e}", describe_record(&record)))?;
        out.write_all(line.as_bytes()).and_then(|()| out.write_all(b"\n")).map_err(|e| e.to_string())
    })
}

fn describe_record(r: &Value) -> String {
    let mut parts = vec![r["kind"].as_str().unwrap_or("?").to_string()];
    for k in ["year", "date", "hour", "form"] {
        match &r[k] {
            Value::Null => {}
            Value::String(s) => parts.push(s.clone()),
            v => parts.push(v.to_string()),
        }
    }
    parts.join(" ")
}

/// Emits every selected record in canonical order.
pub fn generate(
    sel: &Selection,
    data: &CalendarData,
    texts: Option<&OfficeTexts>,
    emit: &mut dyn FnMut(Value) -> Result<(), String>,
) -> Result<(), String> {
    emit(sel.meta())?;
    if sel.has(GROUP_CORPUS) {
        let texts = texts.ok_or("the corpus group needs the loaded texts")?;
        for record in corpus_records(texts) {
            emit(record)?;
        }
    }
    for (year, dates) in sel.plan() {
        let cal = build_calendar(year, data).map_err(|e| format!("building calendar for {year}: {e}"))?;
        if sel.has(GROUP_CALENDAR) {
            emit(calendar_year_record(year, &Tabula::compute(year), &MoveableDates::compute(year)))?;
        }
        let office_days = if sel.has(GROUP_OFFICE) { office::resolve_office_days(&cal) } else { Vec::new() };
        let indices: Vec<usize> = match &dates {
            None => (0..cal.days.len()).collect(),
            Some(dates) => dates
                .iter()
                .map(|d| {
                    let i = d.ordinal() as usize - 1;
                    match cal.days.get(i) {
                        Some(day) if day.date == *d => Ok(i),
                        Some(day) => Err(format!("calendar for {year} returned {} at {d}", day.date)),
                        None => Err(format!("{d} is outside the {year} calendar")),
                    }
                })
                .collect::<Result<_, _>>()?,
        };
        for i in indices {
            if sel.has(GROUP_CALENDAR) {
                emit(calendar_day_record(&cal.days[i]))?;
            }
            if sel.has(GROUP_OFFICE) {
                emit(office_day_record(&cal.days[i], &office_days[i]))?;
            }
        }
    }
    Ok(())
}

/// Go's zero-value mapping: `None` is null.
fn opt(s: &Option<String>) -> Value {
    s.as_ref().map_or(Value::Null, |s| Value::String(s.clone()))
}

fn day_str(d: Date) -> Value {
    Value::String(d.to_string())
}

fn feast(f: Option<&Feast>) -> Value {
    let Some(f) = f else { return Value::Null };
    json!({
        "id": f.id,
        "name": f.name,
        "rank": f.rank.as_str(),
        "color": f.color.as_str(),
        "category": f.category.map(|c| c.as_str()),
        "proper_name": opt(&f.proper_name),
        "proper_id": opt(&f.proper_id),
        "date_rule": opt(&f.date_rule),
        "month": f.fixed.map(|md| md.month),
        "day": f.fixed.map(|md| md.day),
        "has_octave": f.has_octave,
        "has_vigil": f.has_vigil,
        "octave_class": f.octave_class.as_str(),
        "commemoration_class": f.commemoration_class.as_str(),
        "is_privileged_octave_day": f.is_privileged_octave_day,
        "is_vigil": f.is_vigil,
        "vigil_of": opt(&f.vigil_of),
        "companion_of": opt(&f.companion_of),
        "primary_of_our_lord": f.primary_of_our_lord,
        "only_with": opt(&f.only_with),
        "skip_roman_leap_shift": f.skip_roman_leap_shift,
        "source": opt(&f.source),
    })
}

fn feasts(list: &[calendar::FeastRef]) -> Value {
    Value::Array(list.iter().map(|f| feast(Some(f))).collect())
}

fn decisions(list: &[Decision]) -> Value {
    Value::Array(list.iter().map(|d| json!({"rule": d.rule, "outcome": d.outcome, "detail": opt(&d.detail)})).collect())
}

/// Every resolvable corpus key in byte order with its directive and resolved
/// body, then the appointment scopes in file order.
fn corpus_records(texts: &OfficeTexts) -> Vec<Value> {
    let c = &texts.corpus;
    let text = |s: Option<&str>| s.map_or(Value::Null, |s| if s.is_empty() { Value::Null } else { Value::String(s.to_string()) });
    let mut out = Vec::new();
    for key in c.references() {
        let body = c.get(key);
        let (directive, target) = match c.alias_target(key) {
            Some(t) => (Some("use"), Some(t)),
            None if corpus::is_omitted(body) => (Some("omit"), None),
            None => (None, None),
        };
        out.push(json!({
            "kind": "corpus_entry",
            "key": key,
            "directive": directive,
            "use_target": target,
            "canonical": c.canonical_ref(key),
            "body": text(Some(body)),
            "collect_conclusion": text(c.collect_conclusion_form(key)),
            "incipit": text(c.incipit(key)),
        }));
    }
    for s in texts.scopes.as_ref().map_or(&[][..], |s| s.list()) {
        out.push(json!({
            "kind": "appointment_scope",
            "id": s.id,
            "source": text(Some(&s.source)),
            "season": s.season.as_str(),
            "hours": s.hours,
            "slots": s.slots,
            "require_ferial": s.require_ferial,
            "exclude_weekdays": s.exclude_weekdays,
            "from_easter": s.from_easter,
            "until_easter": s.until_easter,
        }));
    }
    out
}

fn calendar_year_record(year: i32, t: &Tabula, m: &MoveableDates) -> Value {
    let ember =
        |s: &calendar::computus::EmberSet| json!({"wednesday": day_str(s.wed), "friday": day_str(s.fri), "saturday": day_str(s.sat)});
    json!({
        "kind": "calendar_year",
        "year": year,
        "tabula": {
            "golden_number": t.golden_number,
            "dominical_letter": t.dominical_letter.to_string(),
            "sundays_after_epiphany": t.sundays_after_epiphany,
            "sundays_after_pentecost": t.sundays_after_pentecost,
            "ember_days": {"spring": ember(&t.spring), "summer": ember(&t.summer), "autumn": ember(&t.autumn), "winter": ember(&t.winter)},
        },
        "moveable_dates": {
            "septuagesima": day_str(m.septuagesima), "sexagesima": day_str(m.sexagesima), "quinquagesima": day_str(m.quinquagesima),
            "ash_wednesday": day_str(m.ash_wednesday),
            "lent_1": day_str(m.lent1), "lent_2": day_str(m.lent2), "lent_3": day_str(m.lent3), "lent_4": day_str(m.lent4),
            "passion_sunday": day_str(m.passion_sunday), "palm_sunday": day_str(m.palm_sunday),
            "holy_monday": day_str(m.holy_monday), "holy_tuesday": day_str(m.holy_tuesday), "holy_wednesday": day_str(m.holy_wednesday),
            "holy_thursday": day_str(m.holy_thursday), "good_friday": day_str(m.good_friday), "holy_saturday": day_str(m.holy_saturday),
            "easter": day_str(m.easter), "easter_monday": day_str(m.easter_monday), "easter_tuesday": day_str(m.easter_tuesday),
            "low_sunday": day_str(m.low_sunday), "ascension": day_str(m.ascension), "pentecost": day_str(m.pentecost),
            "trinity_sunday": day_str(m.trinity_sunday), "corpus_christi": day_str(m.corpus_christi),
            "advent_1": day_str(m.advent1), "advent_2": day_str(m.advent2), "advent_3": day_str(m.advent3), "advent_4": day_str(m.advent4),
        },
    })
}

/// The observance of the day every product shares.
fn calendar_day_record(d: &CalendarDay) -> Value {
    json!({
        "kind": "calendar_day",
        "date": day_str(d.date),
        "season": d.season.as_str(),
        "tempora": opt(&d.tempora),
        "celebration": feast(d.celebration.as_deref()),
        "commemorations": feasts(&d.commemorations),
        "feria_commemoration": feast(d.feria_commemoration.as_deref()),
        "color": d.color.as_str(),
        "notes": opt(&d.notes),
        "resolution_rule": d.resolution_rule,
        "occurrence_decisions": decisions(&d.occurrence_decisions),
        "temporal_week_id": opt(&d.temporal_week_id),
        "within_octave_of": opt(&d.within_octave_of),
        "penitential": {"fast": d.penitential.fast, "abstinence": d.penitential.abstinence},
    })
}

/// The Office-only resolution of the day: concurrence and the Marian antiphon.
fn office_day_record(d: &CalendarDay, o: &office::OfficeDay) -> Value {
    let v = &o.vespers;
    json!({
        "kind": "office_day",
        "date": day_str(d.date),
        "marian_antiphon": o.marian_antiphon,
        "vespers": {
            "owner": v.owner.as_str(),
            "feast": feast(v.feast.as_deref()),
            "color": v.color.map(|c| c.as_str()),
            "season": v.season.map(|s| s.as_str()),
            "within_octave_of": opt(&v.within_octave_of),
            "rule": v.rule,
            "decisions": decisions(&v.decisions),
            "commemorations": feasts(&v.commemorations),
            "following_office_commemoration_id": opt(&v.following_office_commemoration_id),
            "following_office_octave_of": opt(&v.following_office_octave_of),
            "psalmody_from_preceding": v.psalmody_from_preceding,
            "appended_office_of_the_dead": v.appended_office_of_the_dead,
            "appended_feast": feast(v.appended_feast.as_deref()),
        },
    })
}

/// Rejects empty strings (absent values are null), reporting a JSON Pointer.
fn check(v: &Value, path: &mut Vec<String>) -> Result<(), String> {
    match v {
        Value::String(s) if s.is_empty() => Err(format!("/{}: empty string (absent values are null)", path.join("/"))),
        Value::Array(items) => items.iter().enumerate().try_for_each(|(i, item)| {
            path.push(i.to_string());
            let r = check(item, path);
            path.pop();
            r
        }),
        Value::Object(map) => map.iter().try_for_each(|(k, item)| {
            path.push(k.replace('~', "~0").replace('/', "~1"));
            let r = check(item, path);
            path.pop();
            r
        }),
        Value::Null | Value::Bool(_) | Value::Number(_) | Value::String(_) => Ok(()),
    }
}

/// The canonical compact encoding (RFC 8785 for this value domain).
pub fn marshal(v: &Value) -> Result<String, String> {
    check(v, &mut Vec::new())?;
    Ok(serde_json::to_string(v).expect("serializing a Value cannot fail"))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn canonical_encoding_matches_go() {
        let v = json!({"b": "<&>\u{2028}", "a": [1, null, true], "c": "\u{1}\t\"\\"});
        assert_eq!(marshal(&v).unwrap(), r#"{"a":[1,null,true],"b":"<&>\u{2028}","c":"\u0001\t\"\\"}"#.replace("\\u{2028}", "\u{2028}"));
        assert_eq!(marshal(&json!({"x": [{"y": ""}]})).unwrap_err(), "/x/0/y: empty string (absent values are null)");
    }

    #[test]
    fn selection_is_canonical() {
        let sel = Selection { start_year: 2026, years: 1, hours: vec!["vespers".into(), "lauds".into()], ..Selection::default() }
            .normalize()
            .unwrap();
        assert_eq!(sel.hours, ["lauds", "vespers"]);
        assert_eq!(sel.forms, PRAYER_FORMS);
        let err = Selection { start_year: 2026, years: 1, groups: vec!["x".into()], ..Selection::default() }.normalize().unwrap_err();
        assert_eq!(err, "unknown record group \"x\" (want one of [corpus calendar office hours])");
    }
}
