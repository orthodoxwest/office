//! Streaming inspection of canonical dumps. Digest field assignments are a
//! closed schema: adding a field without assigning it must fail the gate.
use std::collections::{BTreeMap, BTreeSet};
use std::fs::File;
use std::io::{BufRead, BufReader, Write};
use std::sync::atomic::{AtomicUsize, Ordering};

use calendar::CalendarData;
use serde_json::{Map, Value, json};
use sha2::{Digest, Sha256};

use crate::args::Flags;
use crate::dump::{FORMAT, HOUR_NAMES, PRAYER_FORMS, Selection, generate_year};

fn input(path: &str) -> Result<Box<dyn BufRead>, String> {
    if path == "-" {
        Ok(Box::new(BufReader::new(std::io::stdin())))
    } else {
        File::open(path).map(|f| Box::new(BufReader::new(f)) as Box<dyn BufRead>).map_err(|e| format!("{path}: {e}"))
    }
}

fn validate(v: &Value) -> Result<(), String> {
    match v {
        Value::String(s) if s.is_empty() => Err("empty string (absent values are null)".into()),
        Value::Number(n) if !n.is_i64() => Err("dump numbers must be signed integers".into()),
        Value::Array(a) => a.iter().try_for_each(validate),
        Value::Object(o) => o.values().try_for_each(validate),
        _ => Ok(()),
    }
}

fn parse(line: &str, canonical: bool) -> Result<Value, String> {
    let value: Value = serde_json::from_str(line).map_err(|e| e.to_string())?;
    if !value.is_object() {
        return Err("record is not a JSON object".into());
    }
    validate(&value)?;
    if canonical && serde_json::to_string(&value).map_err(|e| e.to_string())? != line {
        return Err("record is not canonically encoded".into());
    }
    Ok(value)
}

#[derive(Clone, Copy, PartialEq, Eq)]
enum Group {
    Key,
    Nested,
    Content,
    Presentation,
    Sources,
    Decisions,
}
const GROUPS: [Group; 4] = [Group::Content, Group::Presentation, Group::Sources, Group::Decisions];
const NAMES: [&str; 4] = ["content", "presentation", "sources", "decisions"];

#[derive(Clone, Copy)]
enum Level {
    Hour,
    Section,
    Element,
    Voice,
}
fn assignment(level: Level, field: &str) -> Result<Group, String> {
    use Group::*;
    let group = match (level, field) {
        (Level::Hour, "kind" | "date" | "hour" | "form") => Key,
        (Level::Hour, "hour_label" | "title" | "season" | "feast" | "color") => Content,
        (Level::Hour, "decisions") => Decisions,
        (Level::Hour, "sections") | (Level::Section, "elements") | (Level::Element, "voice") => Nested,
        (Level::Section, "label" | "collapsible") => Content,
        (Level::Element, "type" | "label" | "incipit" | "rubric" | "text") => Content,
        (Level::Element, "display_text" | "announce" | "leader_slot" | "rubric_spans") => Presentation,
        (Level::Element, "slot_ref" | "source_ref" | "source_refs" | "commemoration_owner_id" | "is_commemoration") => Sources,
        (Level::Voice, "text" | "spoken") => Content,
        (Level::Voice, "role") => Presentation,
        _ => return Err(format!("field {field:?} has no parity digest assignment")),
    };
    Ok(group)
}

fn project(value: &Value, level: Level, group: Group) -> Result<Value, String> {
    let object = value.as_object().ok_or("nested record is not an object")?;
    let mut out = Map::new();
    for (key, value) in object {
        let assigned = assignment(level, key)?;
        if assigned == group {
            out.insert(key.clone(), value.clone());
        }
        if assigned == Group::Nested {
            let array = value.as_array().ok_or_else(|| format!("field {key:?} is not a list"))?;
            let next = match level {
                Level::Hour => Level::Section,
                Level::Section => Level::Element,
                Level::Element => Level::Voice,
                Level::Voice => unreachable!(),
            };
            // Validate the entire nested schema even when this projection
            // doesn't include it (e.g. sources has no voice fields).
            let nested = array.iter().map(|v| project(v, next, group)).collect::<Result<Vec<_>, _>>()?;
            if group != Group::Decisions && (!matches!(level, Level::Element) || matches!(group, Group::Content | Group::Presentation)) {
                out.insert(key.clone(), Value::Array(nested));
            }
        }
    }
    if matches!(level, Level::Hour) {
        out.insert("date".into(), value["date"].clone());
        if group != Group::Decisions {
            out.entry("sections").or_insert(json!([]));
        }
    } else if matches!(level, Level::Section) {
        out.entry("elements").or_insert(json!([]));
    } else if matches!(level, Level::Element) && matches!(group, Group::Content | Group::Presentation) {
        out.entry("voice").or_insert(json!([]));
    }
    Ok(Value::Object(out))
}

#[derive(Default)]
struct Hash {
    state: Sha256,
    lines: usize,
}
impl Hash {
    fn add(&mut self, v: &Value) {
        self.state.update(serde_json::to_vec(v).expect("JSON value"));
        self.state.update(b"\n");
        self.lines += 1;
    }
    fn finish(self) -> Value {
        if self.lines == 0 { Value::Null } else { json!(self.state.finalize().iter().map(|b| format!("{b:02x}")).collect::<String>()) }
    }
}
#[derive(Default)]
struct Year {
    calendar: Hash,
    office: Hash,
    hours: BTreeMap<(String, String), [Hash; 4]>,
    merges: Vec<Value>,
    date_hours: usize,
}
fn merges(out: &mut Vec<Value>, date: &Value, surface: &str, winner: &Value, decisions: &Value) {
    if let Some(list) = decisions.as_array() {
        for d in list {
            if matches!(d["rule"].as_str(), Some("commemoration:matches-winner" | "commemoration:duplicate-name")) {
                out.push(json!({"date": date, "surface": surface, "winner": winner["id"], "rule": d["rule"], "detail": d["detail"]}));
            }
        }
    }
}

/// Folds dump records into a parity snapshot. Hashes run in record order, so a year is one
/// uninterrupted stretch of the stream; `absorb` joins digests of disjoint years.
#[derive(Default)]
struct Digester {
    years: BTreeMap<i64, Year>,
    corpus: Hash,
    start: Value,
    count: Value,
}

impl Digester {
    fn add(&mut self, r: &Value) -> Result<(), String> {
        let kind = r["kind"].as_str().ok_or("missing record kind")?;
        match kind {
            "meta" => {
                if r["format"] != FORMAT {
                    return Err("unsupported dump format".into());
                }
                if self.start.is_null() && self.count.is_null() {
                    self.start = r["selection"]["start_year"].clone();
                    self.count = r["selection"]["years"].clone();
                }
                return Ok(());
            }
            "corpus_entry" | "appointment_scope" => {
                self.corpus.add(r);
                return Ok(());
            }
            "calendar_year" => {
                let year = r["year"].as_i64().ok_or("calendar_year without year")?;
                self.years.entry(year).or_default().calendar.add(r);
                return Ok(());
            }
            "calendar_day" | "office_day" | "hour" => {}
            _ => return Err(format!("unknown record kind {kind:?}")),
        }
        let date = r["date"].as_str().ok_or("missing date")?;
        if !date.is_ascii() {
            return Err("invalid record date".into());
        }
        let year = calendar::Date::parse(date).ok_or("invalid record date")?.year();
        let y = self.years.entry(i64::from(year)).or_default();
        match kind {
            "calendar_day" => {
                y.calendar.add(r);
                merges(&mut y.merges, &r["date"], "occurrence", &r["celebration"], &r["occurrence_decisions"]);
            }
            "office_day" => {
                y.office.add(r);
                merges(&mut y.merges, &r["date"], "vespers", &r["vespers"]["feast"], &r["vespers"]["decisions"]);
            }
            "hour" => {
                let hour = r["hour"].as_str().ok_or("missing hour")?;
                let form = r["form"].as_str().ok_or("missing form")?;
                if !HOUR_NAMES.contains(&hour) || !PRAYER_FORMS.contains(&form) {
                    return Err("unknown hour or form".into());
                }
                let hashes = y.hours.entry((hour.into(), form.into())).or_default();
                for (hash, group) in hashes.iter_mut().zip(GROUPS) {
                    hash.add(&project(r, Level::Hour, group)?);
                }
                y.date_hours += usize::from(form == "private");
            }
            _ => unreachable!(),
        }
        Ok(())
    }

    /// Appends the digest of a later part of the same stream. A hash cannot be resumed, so the
    /// part may hold only years this digest has not begun.
    fn absorb(&mut self, part: Digester) -> Result<(), String> {
        if part.corpus.lines > 0 {
            return Err("corpus records belong to the leading digest".into());
        }
        for (year, y) in part.years {
            if self.years.insert(year, y).is_some() {
                return Err(format!("records for {year} were digested in two parts"));
            }
        }
        Ok(())
    }

    fn finish(self) -> Value {
        let mut snapshots = Vec::new();
        let mut all_merges = Vec::new();
        let mut date_hours = 0;
        for (year, mut y) in self.years {
            let mut hours = Vec::new();
            for hour in HOUR_NAMES {
                for form in PRAYER_FORMS {
                    if let Some(hashes) = y.hours.remove(&(hour.into(), form.into())) {
                        let mut h = json!({"hour": hour, "form": form});
                        for (name, hash) in NAMES.into_iter().zip(hashes) {
                            h[name] = hash.finish();
                        }
                        hours.push(h);
                    }
                }
            }
            snapshots.push(json!({"year": year, "calendar": y.calendar.finish(), "office": y.office.finish(), "hours": hours}));
            date_hours += y.date_hours;
            all_merges.extend(y.merges);
        }
        json!({"format": "office-parity/2", "dump_format": FORMAT, "start_year": self.start, "year_count": self.count,
            "date_hours": date_hours, "corpus": self.corpus.finish(), "years": snapshots, "commemoration_merges": all_merges})
    }
}

pub fn digest(reader: impl BufRead) -> Result<Value, String> {
    let mut digester = Digester::default();
    for (i, line) in reader.lines().enumerate() {
        let line = line.map_err(|e| e.to_string())?;
        parse(&line, true).and_then(|r| digester.add(&r)).map_err(|e| format!("line {}: {e}", i + 1))?;
    }
    Ok(digester.finish())
}

/// The snapshot `digest` would produce from the dump of `sel`, without encoding the stream:
/// each entry of the selection's plan is generated and digested by its own worker.
pub fn digest_selection(sel: &Selection, data: &CalendarData, engine: Option<&office::Engine>) -> Result<Value, String> {
    let add = |digester: &mut Digester, r: Value| validate(&r).and_then(|()| digester.add(&r)).map_err(|e| format!("{}: {e}", key(&r)));
    let mut digester = Digester::default();
    add(&mut digester, sel.meta())?;
    if sel.has("corpus") {
        let engine = engine.ok_or("the corpus group needs the loaded texts")?;
        for record in crate::dump::corpus_records(&engine.texts) {
            add(&mut digester, record)?;
        }
    }
    let plan = sel.plan();
    let next = AtomicUsize::new(0);
    let workers = std::thread::available_parallelism().map_or(1, |n| n.get()).min(plan.len().max(1));
    let mut parts: Vec<(usize, Result<Digester, String>)> = std::thread::scope(|scope| {
        let handles: Vec<_> = (0..workers)
            .map(|_| {
                scope.spawn(|| {
                    let mut done = Vec::new();
                    loop {
                        let i = next.fetch_add(1, Ordering::Relaxed);
                        let Some((year, dates)) = plan.get(i) else { return done };
                        let mut part = Digester::default();
                        let result = generate_year(sel, data, engine, *year, dates.as_deref(), &mut |r| add(&mut part, r)).map(|()| part);
                        if result.is_err() {
                            // Later years are moot once one fails.
                            next.store(plan.len(), Ordering::Relaxed);
                        }
                        done.push((i, result));
                    }
                })
            })
            .collect();
        handles.into_iter().flat_map(|h| h.join().expect("digest worker panicked")).collect()
    });
    parts.sort_by_key(|(i, _)| *i);
    for (_, part) in parts {
        digester.absorb(part?)?;
    }
    Ok(digester.finish())
}

pub fn write_snapshot(snapshot: &Value, out: &mut dyn Write) -> Result<(), String> {
    writeln!(out, "{}", serde_json::to_string_pretty(snapshot).map_err(|e| e.to_string())?).map_err(|e| e.to_string())
}

pub fn cmd_digest(args: &[String], out: &mut dyn Write) -> Result<(), String> {
    if args.len() > 1 {
        return Err("usage: office dump digest [FILE|-]".into());
    }
    write_snapshot(&digest(input(args.first().map(String::as_str).unwrap_or("-"))?)?, out)
}

fn key(r: &Value) -> String {
    let kind = r["kind"].as_str().unwrap_or("unknown");
    let fields: &[&str] = match kind {
        "calendar_year" => &["year"],
        "calendar_day" | "office_day" => &["date"],
        "corpus_entry" => &["key"],
        "appointment_scope" => &["id"],
        "hour" => &["date", "hour", "form"],
        _ => &[],
    };
    let mut key = kind.to_string();
    for field in fields {
        key.push(' ');
        key.push_str(&r[*field].as_str().map(str::to_string).unwrap_or_else(|| r[*field].to_string()));
    }
    key
}
fn render(v: Option<&Value>) -> String {
    v.map_or_else(
        || "(absent)".into(),
        |v| {
            let s = v.to_string();
            if s.chars().count() > 160 { format!("{}…", s.chars().take(160).collect::<String>()) } else { s }
        },
    )
}
/// Show long strings around their first differing character, without
/// splitting UTF-8 or hiding a late change behind an identical prefix.
fn excerpts(a: &str, b: &str) -> (String, String) {
    let a: Vec<_> = a.chars().collect();
    let b: Vec<_> = b.chars().collect();
    let at = a.iter().zip(&b).take_while(|(a, b)| a == b).count();
    let cut = |s: &[char]| {
        let start = at.saturating_sub(40);
        let end = s.len().min(at + 80);
        let text: String = s[start..end].iter().collect();
        format!("{}{}{}", if start > 0 { "…" } else { "" }, json!(text), if end < s.len() { "…" } else { "" })
    };
    (cut(&a), cut(&b))
}

fn differences(path: &str, a: &Value, b: &Value, out: &mut Vec<(String, String, String)>) {
    match (a, b) {
        (Value::Object(a), Value::Object(b)) => {
            for key in a.keys().chain(b.keys()).collect::<BTreeSet<_>>() {
                let child = format!("{path}/{}", key.replace('~', "~0").replace('/', "~1"));
                match (a.get(key), b.get(key)) {
                    (Some(a), Some(b)) => differences(&child, a, b, out),
                    (a, b) => out.push((child, render(a), render(b))),
                }
            }
        }
        (Value::Array(a), Value::Array(b)) => {
            for (i, (a, b)) in a.iter().zip(b).enumerate() {
                differences(&format!("{path}/{i}"), a, b, out);
            }
            if a.len() != b.len() {
                out.push((path.into(), format!("{} items", a.len()), format!("{} items", b.len())));
            }
        }
        (Value::String(a), Value::String(b)) if a != b => {
            let (a, b) = excerpts(a, b);
            out.push((path.into(), a, b));
        }
        _ if a != b => out.push((path.into(), render(Some(a)), render(Some(b)))),
        _ => {}
    }
}
fn diff(left: impl BufRead, right: impl BufRead, out: &mut dyn Write, max_records: usize, max_paths: usize) -> Result<bool, String> {
    let (mut left, mut right) = (left.lines(), right.lines());
    let (mut compared, mut differing, mut diverged) = (0, 0, false);
    let mut kinds = BTreeMap::<String, usize>::new();
    loop {
        let l = left.next().transpose().map_err(|e| e.to_string())?;
        let r = right.next().transpose().map_err(|e| e.to_string())?;
        let line = compared + 1;
        let (l, r) = match (l, r) {
            (None, None) => break,
            (Some(l), Some(r)) => (l, r),
            (l, r) => {
                diverged = true;
                let side = if l.is_none() { "left" } else { "right" };
                let next = key(&parse(l.as_deref().or(r.as_deref()).unwrap(), false)?);
                writeln!(out, "line {line}: {side} dump ended; the other continues with {next}").map_err(|e| e.to_string())?;
                break;
            }
        };
        compared += 1;
        let (a, b) = (parse(&l, false)?, parse(&r, false)?);
        if l == r {
            continue;
        }
        let (ak, bk) = (key(&a), key(&b));
        if ak != bk {
            diverged = true;
            writeln!(out, "line {line}: record sequence diverges: left has {ak}, right has {bk}").map_err(|e| e.to_string())?;
            break;
        }
        differing += 1;
        *kinds.entry(a["kind"].as_str().unwrap_or("unknown").into()).or_default() += 1;
        if differing > max_records {
            continue;
        }
        if a == b {
            let canonical = a.to_string();
            let side = if l == canonical {
                "right is not canonical"
            } else if r == canonical {
                "left is not canonical"
            } else {
                "neither side is canonical"
            };
            writeln!(out, "line {line}: {ak}: same values, different encoding ({side})").map_err(|e| e.to_string())?;
            continue;
        }
        writeln!(out, "line {line}: {ak}").map_err(|e| e.to_string())?;
        let mut paths = Vec::new();
        differences("", &a, &b, &mut paths);
        for (path, l, r) in paths.iter().take(max_paths) {
            writeln!(out, "  {path}\n    left:  {l}\n    right: {r}").map_err(|e| e.to_string())?;
        }
        if paths.len() > max_paths {
            writeln!(out, "  … {} more", paths.len() - max_paths).map_err(|e| e.to_string())?;
        }
    }
    if differing > max_records {
        writeln!(out, "… {} more differing records not shown", differing - max_records).map_err(|e| e.to_string())?;
    }
    write!(out, "{compared} records compared, {differing} differ").map_err(|e| e.to_string())?;
    for (kind, count) in kinds {
        write!(out, ", {kind} {count}").map_err(|e| e.to_string())?;
    }
    if diverged {
        write!(out, "; comparison stopped where the streams diverged").map_err(|e| e.to_string())?;
    }
    writeln!(out).map_err(|e| e.to_string())?;
    Ok(differing == 0 && !diverged)
}
pub fn cmd_diff(args: &[String], out: &mut dyn Write) -> Result<(), String> {
    let flags = Flags::parse(args, &["max-records", "max-paths"])?;
    if flags.rest.len() != 2 {
        return Err("usage: office dump diff [-max-records N] [-max-paths N] LEFT RIGHT".into());
    }
    let max_records = flags.int("max-records", 20)?;
    let max_paths = flags.int("max-paths", 8)?;
    if diff(
        input(&flags.rest[0])?,
        input(&flags.rest[1])?,
        out,
        if max_records > 0 { max_records as usize } else { 20 },
        if max_paths > 0 { max_paths as usize } else { 8 },
    )? {
        Ok(())
    } else {
        Err(crate::checks::REPORTED.into())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn excerpts_reveal_late_unicode_changes() {
        let prefix = "℣ ".repeat(100);
        let (a, b) = excerpts(&format!("{prefix}alpha"), &format!("{prefix}omega"));
        assert!(a.starts_with('…'));
        assert!(a.ends_with("alpha\""));
        assert!(b.ends_with("omega\""));
        assert!(!a.contains('�'));
    }

    #[test]
    fn digest_rejects_encoding_and_unassigned_nested_fields() {
        for input in ["{} ", "{\"kind\":\"hour\",\"value\":1.5}", "{\"kind\":\"x\",\"value\":\"\"}"] {
            assert!(digest(input.as_bytes()).is_err());
        }
        let r = json!({"kind":"hour","date":"2026-01-01","hour":"lauds","form":"private","sections":[{"elements":[{"voice":[{"future":"new"}]}]}]});
        assert!(digest(r.to_string().as_bytes()).unwrap_err().contains("no parity digest assignment"));
    }
    #[test]
    fn absorbed_year_parts_equal_one_stream() {
        let meta = json!({"kind":"meta","format":FORMAT,"selection":{"start_year":2026,"years":2}});
        let corpus = json!({"kind":"corpus_entry","key":"k","body":"b"});
        let day = |date: &str| json!({"kind":"calendar_day","date":date,"celebration":null,"occurrence_decisions":[]});
        let records = [meta.clone(), corpus.clone(), day("2026-12-31"), day("2027-01-01")];
        let stream: String = records.iter().map(|r| format!("{r}\n")).collect();

        let digested = |records: &[Value]| {
            let mut d = Digester::default();
            records.iter().for_each(|r| d.add(r).unwrap());
            d
        };
        let mut whole = digested(&records[..3]);
        whole.absorb(digested(&records[3..])).unwrap();
        assert_eq!(whole.finish(), digest(stream.as_bytes()).unwrap());

        let mut lead = digested(&records[..3]);
        assert!(lead.absorb(digested(&[day("2026-12-30")])).unwrap_err().contains("2026"));
        assert!(digested(&[meta]).absorb(digested(&[corpus])).is_err());
    }

    #[test]
    fn digest_groups_are_independent_and_keep_positions() {
        let a = json!({"kind":"hour","date":"2026-01-01","hour":"lauds","form":"private","sections":[{"elements":[{"text":"prayer", "display_text":"display", "source_ref":"ordinary/shared", "voice":[{"text":"prayer","role":"all","spoken":true}]}]}]});
        let before = digest(a.to_string().as_bytes()).unwrap();
        let mut b = a.clone();
        b["sections"][0]["elements"][0]["display_text"] = json!("changed");
        let after = digest(b.to_string().as_bytes()).unwrap();
        let (x, y) = (&before["years"][0]["hours"][0], &after["years"][0]["hours"][0]);
        assert_ne!(x["presentation"], y["presentation"]);
        for group in ["content", "sources", "decisions"] {
            assert_eq!(x[group], y[group]);
        }
        assert_eq!(before["date_hours"], 1);
        b["sections"][0]["elements"].as_array_mut().unwrap().insert(0, json!({"text":"first"}));
        assert_ne!(before["years"][0]["hours"][0]["content"], digest(b.to_string().as_bytes()).unwrap()["years"][0]["hours"][0]["content"]);
    }
    #[test]
    fn diff_reports_pointers_missing_values_and_sequence_divergence() {
        let a = "{\"date\":\"2026-01-01\",\"kind\":\"calendar_day\",\"list\":[1,2],\"only_left\":true}\n";
        let b = "{\"date\":\"2026-01-01\",\"kind\":\"calendar_day\",\"list\":[1],\"only_right\":null}\n";
        let mut out = Vec::new();
        assert!(!diff(a.as_bytes(), b.as_bytes(), &mut out, 20, 8).unwrap());
        let text = String::from_utf8(out).unwrap();
        assert!(text.contains("/list\n    left:  2 items\n    right: 1 items"));
        assert!(text.contains("/only_right\n    left:  (absent)"));
        assert!(text.contains("1 records compared, 1 differ, calendar_day 1"));
        assert!(diff(a.as_bytes(), a.as_bytes(), &mut Vec::new(), 20, 8).unwrap());
        assert!(!diff(a.as_bytes(), b"".as_slice(), &mut Vec::new(), 20, 8).unwrap());
        assert!(!diff(a.as_bytes(), a.replace("2026-01-01", "2026-01-02").as_bytes(), &mut Vec::new(), 20, 8).unwrap());
    }
    #[test]
    fn diff_detects_noncanonical_encoding_and_escapes_pointer_tokens() {
        let a = "{\"a/b~\":1,\"kind\":\"meta\"}";
        let b = "{\"a/b~\":2,\"kind\":\"meta\"}";
        let mut out = Vec::new();
        assert!(!diff(a.as_bytes(), b.as_bytes(), &mut out, 20, 8).unwrap());
        assert!(String::from_utf8(out).unwrap().contains("/a~1b~0"));
        let mut out = Vec::new();
        assert!(!diff(a.as_bytes(), a.replace(":1", ": 1").as_bytes(), &mut out, 20, 8).unwrap());
        assert!(String::from_utf8(out).unwrap().contains("right is not canonical"));
    }
}
