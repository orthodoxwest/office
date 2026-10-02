//! `office dump -digest`: the parity snapshot, per-year hashes of each record group. Composed
//! hours are hashed straight from the document model; the exhaustive patterns in `hash_hour`
//! make a new document field a compile error until it is assigned a group.
use std::collections::BTreeMap;
use std::io::Write;
use std::sync::atomic::{AtomicUsize, Ordering};

use calendar::{CalendarData, Decision};
use liturgy::{OfficeElement, OfficeHour, OfficeSection, PostureAnchor, PostureCue, RubricSpan, VoiceSpan};
use serde_json::{Value, json};
use sha2::{Digest, Sha256};

use crate::dump::{FORMAT, HOUR_NAMES, PRAYER_FORMS, Record, Selection, generate_year};
use crate::dump_tools::{key, validate};

const GROUPS: [&str; 4] = ["content", "presentation", "sources", "decisions"];

#[derive(Default)]
struct Hash {
    state: Sha256,
    records: usize,
    buf: Vec<u8>,
}

impl Hash {
    /// A dump record in its canonical JSON encoding.
    fn record(&mut self, v: &Value) {
        self.buf.clear();
        serde_json::to_writer(&mut self.buf, v).expect("JSON value");
        self.buf.push(b'\n');
        self.state.update(&self.buf);
        self.records += 1;
    }
    fn count(&mut self, n: usize) {
        self.state.update((n as u64).to_le_bytes());
    }
    fn str(&mut self, s: &str) {
        self.count(s.len());
        self.state.update(s.as_bytes());
    }
    fn opt(&mut self, s: Option<&str>) {
        match s {
            None => self.state.update([0]),
            Some(s) => {
                self.state.update([1]);
                self.str(s);
            }
        }
    }
    fn bool(&mut self, b: bool) {
        self.state.update([u8::from(b)]);
    }
    fn finish(self) -> Value {
        if self.records == 0 { Value::Null } else { json!(self.state.finalize().iter().map(|b| format!("{b:02x}")).collect::<String>()) }
    }
}

/// Feeds one composed hour into its group hashes. Every field is length- or count-prefixed, so
/// the encoding is unambiguous; the date is part of every group and the section and element
/// structure of every group but decisions.
fn hash_hour(h: &OfficeHour, hashes: &mut [Hash; 4]) {
    let OfficeHour { form: _, date, hour: label, title, season, feast, color, sections, decisions } = h;
    let [content, presentation, sources, rules] = hashes;
    let date = date.to_string();
    for hash in [&mut *content, &mut *presentation, &mut *sources, &mut *rules] {
        hash.str(&date);
        hash.records += 1;
    }
    content.str(label);
    content.str(title);
    content.opt(season.map(|s| s.as_str()));
    content.str(feast);
    content.opt(color.map(|c| c.as_str()));
    rules.count(decisions.len());
    for Decision { rule, outcome, detail } in decisions {
        rules.str(rule);
        rules.str(outcome);
        rules.opt(detail.as_deref());
    }
    for hash in [&mut *content, &mut *presentation, &mut *sources] {
        hash.count(sections.len());
    }
    for OfficeSection { label, collapsible, elements } in sections {
        content.str(label);
        content.bool(*collapsible);
        for hash in [&mut *content, &mut *presentation, &mut *sources] {
            hash.count(elements.len());
        }
        for e in elements {
            let OfficeElement {
                kind,
                text,
                label,
                incipit,
                rubric,
                voice,
                announce,
                leader_slot,
                rubric_spans,
                postures,
                slot_ref,
                source_ref,
                source_refs,
                commemoration_owner_id,
                is_commemoration,
                unrepeated,
            } = e;
            content.str(kind.as_str());
            content.str(text);
            content.str(label);
            content.str(incipit);
            content.str(rubric);
            presentation.str(&e.display_text());
            presentation.bool(*announce);
            presentation.opt(unrepeated.as_ref().map(|u| format!("{}:{}", u.words, u.named)).as_deref());
            presentation.str(leader_slot);
            presentation.count(rubric_spans.len());
            for RubricSpan { text, prayed } in rubric_spans {
                presentation.str(text);
                presentation.bool(*prayed);
            }
            presentation.count(postures.len());
            for PostureCue { posture, at } in postures {
                presentation.str(posture.as_str());
                match at {
                    PostureAnchor::AfterMediant(n) => presentation.str(&format!("after-mediant:{n}")),
                    PostureAnchor::BeforeVerse(n) => presentation.str(&format!("before-verse:{n}")),
                }
            }
            sources.str(slot_ref);
            sources.str(source_ref);
            sources.count(source_refs.len());
            for r in source_refs {
                sources.str(r);
            }
            sources.str(commemoration_owner_id);
            sources.bool(*is_commemoration);
            content.count(voice.len());
            presentation.count(voice.len());
            for VoiceSpan { text, spoken, role } in voice {
                content.str(text);
                content.bool(*spoken);
                presentation.opt(role.map(|r| r.as_str()));
            }
        }
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

/// Folds records into a parity snapshot. Hashes run in record order, so a year is one
/// uninterrupted stretch of records; `absorb` joins digests of disjoint years.
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
                self.corpus.record(r);
                return Ok(());
            }
            "calendar_year" => {
                let year = r["year"].as_i64().ok_or("calendar_year without year")?;
                self.years.entry(year).or_default().calendar.record(r);
                return Ok(());
            }
            "calendar_day" | "office_day" => {}
            _ => return Err(format!("unknown record kind {kind:?}")),
        }
        let date = r["date"].as_str().ok_or("missing date")?;
        if !date.is_ascii() {
            return Err("invalid record date".into());
        }
        let year = calendar::Date::parse(date).ok_or("invalid record date")?.year();
        let y = self.years.entry(i64::from(year)).or_default();
        if kind == "calendar_day" {
            y.calendar.record(r);
            merges(&mut y.merges, &r["date"], "occurrence", &r["celebration"], &r["occurrence_decisions"]);
        } else {
            y.office.record(r);
            merges(&mut y.merges, &r["date"], "vespers", &r["vespers"]["feast"], &r["vespers"]["decisions"]);
        }
        Ok(())
    }

    fn add_hour(&mut self, name: &str, h: &OfficeHour) -> Result<(), String> {
        if !HOUR_NAMES.contains(&name) {
            return Err(format!("unknown hour {name:?}"));
        }
        let y = self.years.entry(i64::from(h.date.year())).or_default();
        hash_hour(h, y.hours.entry((name.into(), h.form.as_str().into())).or_default());
        y.date_hours += usize::from(h.form == liturgy::PrayerForm::Private);
        Ok(())
    }

    /// Appends the digest of a later part of the same selection. A hash cannot be resumed, so
    /// the part may hold only years this digest has not begun.
    fn absorb(&mut self, part: Digester) -> Result<(), String> {
        if part.corpus.records > 0 {
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
                        for (name, hash) in GROUPS.into_iter().zip(hashes) {
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
        json!({"format": "office-parity/3", "dump_format": FORMAT, "start_year": self.start, "year_count": self.count,
            "date_hours": date_hours, "corpus": self.corpus.finish(), "years": snapshots, "commemoration_merges": all_merges})
    }

    fn add_record(&mut self, record: Record) -> Result<(), String> {
        match record {
            Record::Value(r) => validate(&r).and_then(|()| self.add(&r)).map_err(|e| format!("{}: {e}", key(&r))),
            Record::Hour { name, hour } => {
                self.add_hour(name, hour).map_err(|e| format!("hour {} {name} {}: {e}", hour.date, hour.form.as_str()))
            }
        }
    }
}

/// The parity snapshot of `sel`: each entry of the selection's plan is generated and digested
/// by its own worker.
pub fn digest_selection(sel: &Selection, data: &CalendarData, engine: Option<&office::Engine>) -> Result<Value, String> {
    let mut digester = Digester::default();
    digester.add_record(Record::Value(sel.meta()))?;
    if sel.has("corpus") {
        let engine = engine.ok_or("the corpus group needs the loaded texts")?;
        for record in crate::dump::corpus_records(&engine.texts) {
            digester.add_record(Record::Value(record))?;
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
                        let result = generate_year(sel, data, engine, *year, dates.as_deref(), &mut |r| part.add_record(r)).map(|()| part);
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

#[cfg(test)]
mod tests {
    use super::*;
    use calendar::Date;
    use liturgy::{ElementType, PrayerForm, VoiceRole};

    fn hour() -> OfficeHour {
        let mut e = OfficeElement::new(ElementType::Prayer, "prayer");
        e.source_ref = "ordinary/shared".into();
        e.voice = vec![VoiceSpan::new("prayer", true, Some(VoiceRole::All))];
        OfficeHour {
            form: PrayerForm::Private,
            date: Date::parse("2026-01-01").unwrap(),
            hour: "Lauds".into(),
            title: String::new(),
            season: None,
            feast: String::new(),
            color: None,
            sections: vec![OfficeSection { label: String::new(), collapsible: false, elements: vec![e] }],
            decisions: Vec::new(),
        }
    }

    fn digest(h: &OfficeHour) -> Value {
        let mut d = Digester::default();
        d.add_hour("lauds", h).unwrap();
        d.finish()["years"][0]["hours"][0].clone()
    }

    #[test]
    fn groups_are_independent_and_keep_positions() {
        let before = digest(&hour());
        let changed = |edit: fn(&mut OfficeHour)| {
            let mut h = hour();
            edit(&mut h);
            let after = digest(&h);
            GROUPS.into_iter().filter(|g| before[*g] != after[*g]).collect::<Vec<_>>()
        };
        assert_eq!(changed(|h| h.sections[0].elements[0].leader_slot = "officiant".into()), ["presentation"]);
        assert_eq!(changed(|h| h.sections[0].elements[0].source_refs = vec!["x".into()]), ["sources"]);
        assert_eq!(changed(|h| h.sections[0].elements[0].voice[0].text = "other".into()), ["content"]);
        assert_eq!(changed(|h| h.decisions.push(Decision::new("r", "o", ""))), ["decisions"]);
        // Fields are length-prefixed: moving text between adjacent fields is a change.
        assert_eq!(
            changed(|h| {
                h.hour = "Laud".into();
                h.title = "s".into();
            }),
            ["content"]
        );
        let inserted = changed(|h| h.sections[0].elements.insert(0, OfficeElement::new(ElementType::Prayer, "first")));
        assert_eq!(inserted, ["content", "presentation", "sources"]);
    }

    #[test]
    fn absorbed_year_parts_equal_one_digest() {
        let meta = json!({"kind":"meta","format":FORMAT,"selection":{"start_year":2026,"years":2}});
        let corpus = json!({"kind":"corpus_entry","key":"k","body":"b"});
        let day = |date: &str| json!({"kind":"calendar_day","date":date,"celebration":null,"occurrence_decisions":[]});
        let digested = |records: &[Value]| {
            let mut d = Digester::default();
            records.iter().for_each(|r| d.add(r).unwrap());
            d
        };
        let records = [meta.clone(), corpus.clone(), day("2026-12-31"), day("2027-01-01")];
        let mut whole = digested(&records[..3]);
        whole.absorb(digested(&records[3..])).unwrap();
        assert_eq!(whole.finish(), digested(&records).finish());

        let mut lead = digested(&records[..3]);
        assert!(lead.absorb(digested(&[day("2026-12-30")])).unwrap_err().contains("2026"));
        assert!(digested(&[meta]).absorb(digested(&[corpus])).is_err());
    }
}
