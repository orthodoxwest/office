//! Streaming inspection of canonical dumps: `office dump diff`.
use std::collections::{BTreeMap, BTreeSet};
use std::fs::File;
use std::io::{BufRead, BufReader, Write};

use serde_json::{Value, json};

use crate::args::Flags;

fn input(path: &str) -> Result<Box<dyn BufRead>, String> {
    if path == "-" {
        Ok(Box::new(BufReader::new(std::io::stdin())))
    } else {
        File::open(path).map(|f| Box::new(BufReader::new(f)) as Box<dyn BufRead>).map_err(|e| format!("{path}: {e}"))
    }
}

pub(crate) fn validate(v: &Value) -> Result<(), String> {
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

pub(crate) fn key(r: &Value) -> String {
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
                let next = key(&parse(l.as_deref().or(r.as_deref()).expect("one dump continues"), false)?);
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
