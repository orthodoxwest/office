//! Appointment scopes (`data/appointment-scopes.json`): when a seasonal fallback text is eligible.

use std::collections::HashMap;

use calendar::{Date, Season, Weekday};
use corpus::Corpus;
use serde_json::Value;

/// Limits a seasonal fallback lookup. Easter offsets are civil days: `from`
/// inclusive, `until` exclusive; `None` is open.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct AppointmentScope {
    pub id: String,
    pub source: String,
    pub season: Season,
    pub hours: Vec<String>,
    pub slots: Vec<String>,
    pub require_ferial: bool,
    pub exclude_weekdays: Vec<String>,
    pub from_easter: Option<i64>,
    pub until_easter: Option<i64>,
    excluded: [bool; 7],
}

impl AppointmentScope {
    /// Uses the office date for bounds and the composer's civil weekday for
    /// weekday predicates.
    pub fn allows(&self, date: Date, easter: Date, weekday: Weekday, ferial: bool) -> bool {
        let offset = |n: i64| easter.add_days(n as i32);
        (!self.require_ferial || ferial)
            && !self.excluded[weekday.number() as usize]
            && self.from_easter.is_none_or(|n| date >= offset(n))
            && self.until_easter.is_none_or(|n| date < offset(n))
    }
}

/// The validated scopes, indexed by season and hour.
#[derive(Clone, Debug, Default)]
pub struct AppointmentScopes {
    list: Vec<AppointmentScope>,
    index: HashMap<(Season, String), Vec<usize>>,
}

fn slot_matches(selector: &str, slot: &str) -> bool {
    match selector.strip_suffix('*') {
        Some(prefix) => slot.starts_with(prefix),
        None => slot == selector,
    }
}

fn selectors_overlap(a: &str, b: &str) -> bool {
    slot_matches(a, b.strip_suffix('*').unwrap_or(b)) || slot_matches(b, a.strip_suffix('*').unwrap_or(a))
}

const HOURS: [&str; 7] = ["lauds", "prime", "terce", "sext", "none", "vespers", "compline"];

impl AppointmentScopes {
    /// The scopes in file order.
    pub fn list(&self) -> &[AppointmentScope] {
        &self.list
    }

    /// The single scope for a lookup, or `None` for an unrestricted one.
    /// `slot` is the base slot; a trailing `*` selector matches a family.
    pub fn seasonal(&self, season: Season, hour: &str, slot: &str) -> Option<&AppointmentScope> {
        self.index
            .get(&(season, hour.to_string()))?
            .iter()
            .map(|&i| &self.list[i])
            .find(|s| s.slots.iter().any(|sel| slot_matches(sel, slot)))
    }

    /// Parses and validates `appointment-scopes.json`. Errors are prefixed with `path`.
    pub fn load(path: &str, raw: &str, corpus: &Corpus) -> Result<AppointmentScopes, String> {
        let mut stream = serde_json::Deserializer::from_str(raw).into_iter::<Value>();

        let value = match stream.next() {
            Some(Ok(v)) => v,
            Some(Err(e)) => return Err(format!("{path}: {e}")),
            None => return Err(format!("{path}: EOF")),
        };
        if stream.next().is_some() {
            return Err(format!("{path}: expected one JSON array"));
        }
        let items = match value {
            Value::Null => return Err(format!("{path}: expected an array, not null")),
            Value::Array(items) => items,
            Value::Bool(_) | Value::Number(_) | Value::String(_) | Value::Object(_) => {
                return Err(format!("{path}: expected an array of appointment scopes"));
            }
        };
        let mut scopes = AppointmentScopes::default();
        let mut ids = std::collections::HashSet::new();
        for item in items {
            let scope = match item {
                Value::Null => return Err(format!("{path}: null appointment scope")),
                Value::Object(map) => decode(map).map_err(|e| format!("{path}: {e}"))?,
                Value::Bool(_) | Value::Number(_) | Value::String(_) | Value::Array(_) => {
                    return Err(format!("{path}: expected an appointment scope object"));
                }
            };
            let scope = validate(scope, corpus).map_err(|(id, e)| format!("{path}: scope {}: {e}", data_format::quote(&id)))?;
            if !ids.insert(scope.id.clone()) {
                return Err(format!("{path}: duplicate scope ID {}", data_format::quote(&scope.id)));
            }
            let idx = scopes.list.len();
            for hour in &scope.hours {
                let key = (scope.season, hour.clone());
                for &prior in scopes.index.get(&key).map_or(&[][..], Vec::as_slice) {
                    let prior = &scopes.list[prior];
                    for a in &prior.slots {
                        for b in &scope.slots {
                            if selectors_overlap(a, b) {
                                return Err(format!(
                                    "{path}: scopes {} and {} overlap at {}/{hour} ({a}, {b})",
                                    data_format::quote(&prior.id),
                                    data_format::quote(&scope.id),
                                    scope.season
                                ));
                            }
                        }
                    }
                }
                scopes.index.entry(key).or_default().push(idx);
            }
            scopes.list.push(scope);
        }
        Ok(scopes)
    }
}

/// A scope as decoded, before validation: the season is still text.
struct Raw {
    id: String,
    source: String,
    season: String,
    hours: Vec<String>,
    slots: Vec<String>,
    require_ferial: bool,
    exclude_weekdays: Vec<String>,
    from_easter: Option<i64>,
    until_easter: Option<i64>,
}

/// Decodes one object: field names match case-insensitively, unknown fields are rejected, and null
/// leaves a field at its default value.
fn decode(map: serde_json::Map<String, Value>) -> Result<Raw, String> {
    let mut raw = Raw {
        id: String::new(),
        source: String::new(),
        season: String::new(),
        hours: Vec::new(),
        slots: Vec::new(),
        require_ferial: false,
        exclude_weekdays: Vec::new(),
        from_easter: None,
        until_easter: None,
    };
    let string = |v: &Value, field: &str| match v {
        Value::Null => Ok(String::new()),
        Value::String(s) => Ok(s.clone()),
        Value::Bool(_) | Value::Number(_) | Value::Array(_) | Value::Object(_) => Err(type_error(field)),
    };
    let strings = |v: &Value, field: &str| match v {
        Value::Null => Ok(Vec::new()),
        Value::Array(items) => items.iter().map(|i| string(i, field)).collect(),
        Value::Bool(_) | Value::Number(_) | Value::String(_) | Value::Object(_) => Err(type_error(field)),
    };
    let int = |v: &Value, field: &str| match v {
        Value::Null => Ok(None),
        Value::Number(n) => n.as_i64().map(Some).ok_or_else(|| type_error(field)),
        Value::Bool(_) | Value::String(_) | Value::Array(_) | Value::Object(_) => Err(type_error(field)),
    };
    for (key, v) in &map {
        match key.to_ascii_lowercase().as_str() {
            "id" => raw.id = string(v, "id")?,
            "source" => raw.source = string(v, "source")?,
            "season" => raw.season = string(v, "season")?,
            "hours" => raw.hours = strings(v, "hours")?,
            "slots" => raw.slots = strings(v, "slots")?,
            "require_ferial" => {
                raw.require_ferial = match v {
                    Value::Null => false,
                    Value::Bool(b) => *b,
                    Value::Number(_) | Value::String(_) | Value::Array(_) | Value::Object(_) => return Err(type_error("require_ferial")),
                }
            }
            "exclude_weekdays" => raw.exclude_weekdays = strings(v, "exclude_weekdays")?,
            "from_easter" => raw.from_easter = int(v, "from_easter")?,
            "until_easter" => raw.until_easter = int(v, "until_easter")?,
            _ => return Err(format!("json: unknown field {}", data_format::quote(key))),
        }
    }
    Ok(raw)
}

fn type_error(field: &str) -> String {
    format!("invalid type for appointment scope field {field}")
}

fn validate(raw: Raw, corpus: &Corpus) -> Result<AppointmentScope, (String, String)> {
    let id = raw.id.clone();
    let fail = |e: String| (id.clone(), e);
    if raw.id.trim().is_empty() || raw.source.trim().is_empty() || raw.hours.is_empty() || raw.slots.is_empty() {
        return Err(fail("id, source, hours and slots are required".to_string()));
    }
    let season = Season::parse(&raw.season).map_err(|_| fail(format!("unknown season {}", data_format::quote(&raw.season))))?;
    if let (Some(f), Some(u)) = (raw.from_easter, raw.until_easter)
        && f >= u
    {
        return Err(fail("from_easter must precede until_easter".to_string()));
    }
    if [raw.from_easter, raw.until_easter].into_iter().flatten().any(|b| !(-366..=366).contains(&b)) {
        return Err(fail("easter offset must be between -366 and 366".to_string()));
    }
    if !raw.require_ferial && raw.exclude_weekdays.is_empty() && raw.from_easter.is_none() && raw.until_easter.is_none() {
        return Err(fail("scope requires an applicability restriction".to_string()));
    }
    let mut excluded = [false; 7];
    for weekday in &raw.exclude_weekdays {
        let Some(day) = Weekday::ALL.into_iter().find(|d| d.name().to_lowercase() == *weekday) else {
            return Err(fail(format!("unknown weekday {}", data_format::quote(weekday))));
        };
        let slot = &mut excluded[day.number() as usize];
        if *slot {
            return Err(fail(format!("duplicate weekday {}", data_format::quote(weekday))));
        }
        *slot = true;
    }
    let mut seen_hours = Vec::new();
    for hour in &raw.hours {
        if !HOURS.contains(&hour.as_str()) {
            return Err(fail(format!("unknown hour {}", data_format::quote(hour))));
        }
        if seen_hours.contains(&hour) {
            return Err(fail(format!("duplicate hour {}", data_format::quote(hour))));
        }
        seen_hours.push(hour);
        for (i, slot) in raw.slots.iter().enumerate() {
            if !matches!(slot.as_str(), "chapter" | "versicle" | "psalm-antiphon*") {
                return Err(fail(format!("invalid slot selector {}", data_format::quote(slot))));
            }
            if let Some(prior) = raw.slots[..i].iter().find(|p| selectors_overlap(p, slot)) {
                return Err(fail(format!("overlapping slot selectors {} and {}", data_format::quote(prior), data_format::quote(slot))));
            }
            if !has_seasonal_scope_slot(corpus, season, hour, slot) {
                return Err(fail(format!("slot {} has no seasonal corpus candidate for {season}/{hour}", data_format::quote(slot))));
            }
        }
    }
    Ok(AppointmentScope {
        id: raw.id,
        source: raw.source,
        season,
        hours: raw.hours,
        slots: raw.slots,
        require_ferial: raw.require_ferial,
        exclude_weekdays: raw.exclude_weekdays,
        from_easter: raw.from_easter,
        until_easter: raw.until_easter,
        excluded,
    })
}

fn has_seasonal_scope_slot(corpus: &Corpus, season: Season, hour: &str, slot: &str) -> bool {
    let prefix = format!("seasonal/{season}/");
    if let Some(family) = slot.strip_suffix('*') {
        let family = format!("{prefix}{family}");
        return corpus.concrete().map(|(k, _)| k).chain(corpus.alias_keys()).any(|k| k.starts_with(&family));
    }
    corpus.has(&format!("{prefix}{slot}-{hour}")) || corpus.has(&format!("{prefix}{slot}"))
}

#[cfg(test)]
mod tests {
    use super::*;

    fn corpus() -> Corpus {
        let files = [corpus::TextFile {
            rel_path: "seasonal/lent.txt".to_string(),
            content:
                "[chapter-terce]\nFerial chapter\n\n[versicle-terce]\n@omit\n\n[psalm-antiphon-terce]\n@use seasonal/lent/chapter-terce\n"
                    .to_string(),
        }];
        Corpus::load(&files, None, None).unwrap()
    }

    const SAMPLE: &str = r#"[{"id":"sample","source":"test requirement","season":"lent","hours":["terce"],"slots":["chapter"],"require_ferial":true,"exclude_weekdays":["sunday"],"from_easter":-10,"until_easter":-2}]"#;

    #[test]
    fn bounds_weekdays_and_slot_families() {
        let scopes = AppointmentScopes::load("s.json", SAMPLE, &corpus()).unwrap();
        let scope = scopes.seasonal(Season::Lent, "terce", "chapter").unwrap();
        assert!(scopes.seasonal(Season::Lent, "sext", "chapter").is_none());
        let easter = Date::new(2032, 5, 2);
        for (offset, weekday, ferial, want) in [
            (-11, Weekday::Monday, true, false),
            (-10, Weekday::Monday, true, true),
            (-3, Weekday::Monday, true, true),
            (-2, Weekday::Monday, true, false),
            (-5, Weekday::Sunday, true, false),
            (-5, Weekday::Monday, false, false),
        ] {
            assert_eq!(scope.allows(easter.add_days(offset), easter, weekday, ferial), want, "{offset} {weekday}");
        }
        // A trailing * selects a slot family.
        let raw = r#"[{"id":"f","source":"s","season":"lent","hours":["terce"],"slots":["psalm-antiphon*","versicle"],"until_easter":0}]"#;
        let scopes = AppointmentScopes::load("s.json", raw, &corpus()).unwrap();
        assert!(scopes.seasonal(Season::Lent, "terce", "psalm-antiphon").is_some());
        assert!(scopes.seasonal(Season::Lent, "terce", "versicle").is_some());
        assert_eq!(scopes.list().len(), 1);
    }

    #[test]
    fn rejects_invalid_data() {
        let c = corpus();
        let err = |raw: &str| AppointmentScopes::load("s.json", raw, &c).unwrap_err();
        let one = |fields: &str| format!(r#"[{{"id":"x","source":"s","season":"lent","hours":["terce"],"slots":["chapter"],{fields}}}]"#);
        assert_eq!(err("null"), "s.json: expected an array, not null");
        assert_eq!(err("{}"), "s.json: expected an array of appointment scopes");
        assert_eq!(err("[false]"), "s.json: expected an appointment scope object");
        assert_eq!(err(&one(r#""require_ferial":1"#)), "s.json: invalid type for appointment scope field require_ferial");
        assert_eq!(err("[] []"), "s.json: expected one JSON array");
        assert_eq!(err("[null]"), "s.json: null appointment scope");
        assert_eq!(err(&one(r#""require_ferial":true,"bogus":1"#)), "s.json: json: unknown field \"bogus\"");
        assert_eq!(err(&one(r#""from_easter":1"#).replace("lent", "winter")), "s.json: scope \"x\": unknown season \"winter\"");
        assert_eq!(err(&one(r#""from_easter":3,"until_easter":3"#)), "s.json: scope \"x\": from_easter must precede until_easter");
        assert_eq!(err(&one(r#""from_easter":400"#)), "s.json: scope \"x\": easter offset must be between -366 and 366");
        assert_eq!(err(&one(r#""require_ferial":false"#)), "s.json: scope \"x\": scope requires an applicability restriction");
        assert_eq!(err(&one(r#""exclude_weekdays":["funday"]"#)), "s.json: scope \"x\": unknown weekday \"funday\"");
        assert_eq!(err(&one(r#""exclude_weekdays":["monday","monday"]"#)), "s.json: scope \"x\": duplicate weekday \"monday\"");
        assert_eq!(
            err(&one(r#""require_ferial":true"#).replace(r#"["chapter"]"#, r#"["hymn"]"#)),
            "s.json: scope \"x\": invalid slot selector \"hymn\""
        );
        assert_eq!(
            err(&one(r#""require_ferial":true"#).replace(r#"["terce"]"#, r#"["sext"]"#)),
            "s.json: scope \"x\": slot \"chapter\" has no seasonal corpus candidate for lent/sext"
        );
        let entry = |id: &str| {
            format!(r#"{{"id":"{id}","source":"s","season":"lent","hours":["terce"],"slots":["chapter"],"require_ferial":true}}"#)
        };
        let two = format!("[{},{}]", entry("x"), entry("y"));
        assert_eq!(err(&two), "s.json: scopes \"x\" and \"y\" overlap at lent/terce (chapter, chapter)");
    }
}
