//! The dependency-weighted provenance queue, usage-weighted provenance, and the zero-occurrence
//! report built on it.

use std::collections::{BTreeMap, BTreeSet, HashMap};
use std::fmt::Write as _;

use calendar::{CalendarData, DataSource, Date};
use data_format::csv;
use liturgy::PrayerForm;
use office::engine::Engine;

use super::assurance::{ReviewCandidate, candidate_for};
use super::prescreen::{Suspicion, suspicion_by_key};
use super::provenance::{ProvenanceStatus, SourceCitation, scan_provenance};
use super::sweep::{Forms, sweep};
use super::zero_occurrence::{ZeroClassification, ZeroDisposition, ZeroHeuristic, load_zero_classifications};
use super::{base_url, hour_order, hour_tier};

/// One atomic corpus review task.
#[derive(Clone, Debug)]
pub struct QueueEntry {
    pub key: String,
    pub content_hash: String,
    pub status: ProvenanceStatus,
    pub score: usize,
    pub occurrences: usize,
    pub priority_a_occurrences: usize,
    pub principal_occurrences: usize,
    pub distinct_compositions: usize,
    pub hours: Vec<&'static str>,
    pub representative: Option<(&'static str, PrayerForm, Date)>,
    pub sources: Vec<SourceCitation>,
    pub flags: Vec<Suspicion>,
    pub zero_classification: Option<ZeroClassification>,
}

impl QueueEntry {
    pub fn suspect(&self) -> bool {
        !self.flags.is_empty()
    }

    fn classified_zero(&self) -> bool {
        self.zero_classification.as_ref().is_some_and(ZeroClassification::classified)
    }

    fn work_rank(&self) -> u8 {
        if self.occurrences > 0 {
            0
        } else if self.classified_zero() {
            2
        } else {
            1
        }
    }

    fn work_type(&self) -> &'static str {
        match self.work_rank() {
            0 => "attestation",
            2 => "classified-zero",
            _ => "zero-needs-classification",
        }
    }
}

pub struct ProvenanceQueue {
    pub start_year: i32,
    pub years: i32,
    pub include_verified: bool,
    pub entries: Vec<QueueEntry>,
}

/// The representative page of an entry: its best-placed composition.
#[derive(Clone, Copy)]
struct Representative {
    priority: &'static str,
    hour: &'static str,
    form: PrayerForm,
    date: Date,
}

/// Suspect entries first, then by 20·compositions + 5·priority-A uses + 3·principal-hour uses +
/// uses.
pub fn build_provenance_queue(src: &dyn DataSource, start: i32, years: i32, include_verified: bool) -> Result<ProvenanceQueue, String> {
    if years < 1 {
        return Err("years must be at least 1".into());
    }
    let inventory = scan_provenance(src)?;
    let suspicions = suspicion_by_key(src, &inventory)?;
    let zero = load_zero_classifications(src, &inventory)?;

    struct Acc {
        entry: QueueEntry,
        hours: BTreeSet<usize>,
        compositions: BTreeSet<String>,
        representative: Option<Representative>,
    }
    let mut acc: HashMap<String, Acc> = HashMap::new();
    for p in &inventory.entries {
        if p.status == ProvenanceStatus::Verified && !include_verified {
            continue;
        }
        acc.insert(
            p.key.clone(),
            Acc {
                entry: QueueEntry {
                    key: p.key.clone(),
                    content_hash: p.content_hash.clone(),
                    status: p.status,
                    score: 0,
                    occurrences: 0,
                    priority_a_occurrences: 0,
                    principal_occurrences: 0,
                    distinct_compositions: 0,
                    hours: Vec::new(),
                    representative: None,
                    sources: p.sources.clone(),
                    flags: suspicions.get(&p.key).cloned().unwrap_or_default(),
                    zero_classification: zero.get(&p.key).cloned(),
                },
                hours: BTreeSet::new(),
                compositions: BTreeSet::new(),
                representative: None,
            },
        );
    }

    let engine = Engine::load(src)?;
    let cal = CalendarData::load(src)?;
    let mut fold = |candidate: ReviewCandidate| {
        let rep = Representative { priority: candidate.priority, hour: candidate.hour, form: candidate.form, date: candidate.date };
        let priority_a = candidate.priority == "A";
        let principal = hour_tier(candidate.hour) == 0;
        for r in &candidate.dependencies {
            let Some(a) = acc.get_mut(r) else { continue };
            a.entry.occurrences += 1;
            if priority_a {
                a.entry.priority_a_occurrences += 1;
            }
            if principal {
                a.entry.principal_occurrences += 1;
            }
            a.hours.insert(hour_order(candidate.hour));
            a.compositions.insert(candidate.hash.clone());
            if a.representative.as_ref().is_none_or(|old| representative_less(&rep, old)) {
                a.representative = Some(rep);
            }
        }
    };
    sweep(
        &engine,
        &cal,
        start,
        years,
        Forms::All,
        &|_, e| e,
        &|day, hour_name, hour| candidate_for(day, hour_name, hour, false),
        &mut fold,
    )?;

    let mut entries: Vec<QueueEntry> = acc
        .into_values()
        .map(|a| {
            let mut e = a.entry;
            e.hours = a.hours.iter().map(|&i| office::engine::HOUR_NAMES[i]).collect();
            e.distinct_compositions = a.compositions.len();
            e.score = 20 * e.distinct_compositions + 5 * e.priority_a_occurrences + 3 * e.principal_occurrences + e.occurrences;
            e.representative = a.representative.map(|c| (c.hour, c.form, c.date));
            e
        })
        .collect();
    entries.sort_by(|a, b| {
        a.work_rank()
            .cmp(&b.work_rank())
            .then_with(|| b.suspect().cmp(&a.suspect()))
            .then_with(|| b.score.cmp(&a.score))
            .then_with(|| b.priority_a_occurrences.cmp(&a.priority_a_occurrences))
            .then_with(|| b.occurrences.cmp(&a.occurrences))
            .then_with(|| a.key.cmp(&b.key))
    });
    Ok(ProvenanceQueue { start_year: start, years, include_verified, entries })
}

fn representative_less(a: &Representative, b: &Representative) -> bool {
    if a.priority != b.priority {
        return a.priority < b.priority;
    }
    let (ap, bp) = (hour_tier(a.hour) == 0, hour_tier(b.hour) == 0);
    if ap != bp {
        return ap;
    }
    if a.date != b.date {
        return a.date < b.date;
    }
    hour_order(a.hour) < hour_order(b.hour)
}

impl ProvenanceQueue {
    /// Keeps only entries with a suspicion: the findings-sprint list.
    pub fn filter_suspect(&mut self) {
        self.entries.retain(QueueEntry::suspect);
    }
}

pub fn queue_summary(q: &ProvenanceQueue) -> String {
    let (mut used, mut suspect, mut classified, mut unclassified, mut stale) = (0, 0, 0, 0, 0);
    let mut statuses: BTreeMap<ProvenanceStatus, usize> = BTreeMap::new();
    for e in &q.entries {
        if e.occurrences > 0 {
            used += 1;
            *statuses.entry(e.status).or_default() += 1;
        } else if e.classified_zero() {
            classified += 1;
        } else {
            unclassified += 1;
            if e.zero_classification.as_ref().is_some_and(|z| z.stale) {
                stale += 1;
            }
        }
        if e.suspect() {
            suspect += 1;
        }
    }
    let mut w = String::new();
    let _ = writeln!(w, "=== Atomic provenance review queue: {}-{} ===", q.start_year, q.start_year + q.years - 1);
    let _ = writeln!(w, "  queued entries:      {}", q.entries.len());
    let _ = writeln!(w, "  rendered entries:    {used}");
    let _ = writeln!(w, "  classified zeroes:   {classified}");
    let _ = writeln!(w, "  unclassified zeroes: {unclassified}");
    let _ = writeln!(w, "  stale zero classes:  {stale}");
    let _ = writeln!(w, "  suspect entries:     {suspect}");
    let _ = writeln!(w, "  include verified:    {}", q.include_verified);
    for status in [ProvenanceStatus::Verified, ProvenanceStatus::NeedsReview, ProvenanceStatus::SourceUnknown] {
        let _ = writeln!(w, "  {:<19} {}", format!("rendered {}:", status.as_str()), statuses.get(&status).copied().unwrap_or(0));
    }
    w
}

fn push_unique(values: &mut Vec<String>, value: &str) {
    if !value.is_empty() && !values.iter().any(|v| v == value) {
        values.push(value.to_string());
    }
}

pub fn queue_csv(q: &ProvenanceQueue, base: &str) -> String {
    let base = base_url(base);
    let mut out = String::new();
    csv::write_record(
        &mut out,
        &[
            "rank",
            "score",
            "key",
            "status",
            "work_type",
            "zero_classification_state",
            "zero_disposition",
            "zero_issue",
            "zero_reason",
            "flags",
            "occurrences",
            "priority_a_occurrences",
            "principal_occurrences",
            "distinct_compositions",
            "hours",
            "source",
            "locator",
            "page",
            "representative_url",
        ],
    );
    for (i, e) in q.entries.iter().enumerate() {
        let (mut sources, mut locators, mut pages) = (Vec::new(), Vec::new(), Vec::new());
        for s in &e.sources {
            push_unique(&mut sources, &s.source);
            push_unique(&mut locators, &s.locator);
            push_unique(&mut pages, &s.page);
        }
        let url = e.representative.map_or(String::new(), |(hour, form, date)| format!("{base}/{hour}/{date}?form={}", form.as_str()));
        let flags: Vec<String> = e.flags.iter().map(ToString::to_string).collect();
        let (mut state, mut disposition, mut issue, mut reason) = ("", "", "", "");
        if e.occurrences == 0
            && let Some(z) = &e.zero_classification
        {
            state = if z.stale { "stale" } else { "current" };
            disposition = z.disposition.as_str();
            issue = &z.issue;
            reason = &z.reason;
        }
        let nums = [i + 1, e.score, e.occurrences, e.priority_a_occurrences, e.principal_occurrences, e.distinct_compositions]
            .map(|n| n.to_string());
        csv::write_record(
            &mut out,
            &[
                &nums[0],
                &nums[1],
                &e.key,
                e.status.as_str(),
                e.work_type(),
                state,
                disposition,
                issue,
                reason,
                &flags.join("; "),
                &nums[2],
                &nums[3],
                &nums[4],
                &nums[5],
                &e.hours.join("; "),
                &sources.join("; "),
                &locators.join("; "),
                &pages.join("; "),
                &url,
            ],
        );
    }
    out
}

/// Verified share of the text a sweep actually renders, each entry weighted
/// by how often it is prayed.
pub struct UsageWeighted {
    pub start_year: i32,
    pub years: i32,
    pub rendered_entries: usize,
    pub verified_entries: usize,
    pub total_occurrences: usize,
    pub verified_occurrences: usize,
    pub needs_review_occurrences: usize,
    pub source_unknown_occurrences: usize,
}

pub fn build_usage_weighted(src: &dyn DataSource, start: i32, years: i32) -> Result<UsageWeighted, String> {
    let queue = build_provenance_queue(src, start, years, true)?;
    let mut u = UsageWeighted {
        start_year: start,
        years,
        rendered_entries: 0,
        verified_entries: 0,
        total_occurrences: 0,
        verified_occurrences: 0,
        needs_review_occurrences: 0,
        source_unknown_occurrences: 0,
    };
    for e in queue.entries.iter().filter(|e| e.occurrences > 0) {
        u.rendered_entries += 1;
        u.total_occurrences += e.occurrences;
        match e.status {
            ProvenanceStatus::Verified => {
                u.verified_entries += 1;
                u.verified_occurrences += e.occurrences;
            }
            ProvenanceStatus::NeedsReview => u.needs_review_occurrences += e.occurrences,
            ProvenanceStatus::SourceUnknown => u.source_unknown_occurrences += e.occurrences,
        }
    }
    Ok(u)
}

fn percent(part: usize, whole: usize) -> f64 {
    if whole == 0 { 0.0 } else { 100.0 * part as f64 / whole as f64 }
}

pub fn usage_weighted_summary(u: &UsageWeighted) -> String {
    let mut w = String::new();
    let _ = writeln!(w, "=== Usage-weighted provenance: {}-{} ===", u.start_year, u.start_year + u.years - 1);
    let _ = writeln!(w, "  rendered entries:       {:5}", u.rendered_entries);
    let _ = writeln!(
        w,
        "  verified entries:       {:5} ({:.1}% of rendered entries)",
        u.verified_entries,
        percent(u.verified_entries, u.rendered_entries)
    );
    let _ = writeln!(w, "  total occurrences:      {:5}", u.total_occurrences);
    let _ = writeln!(
        w,
        "  verified occurrences:   {:5} ({:.1}% of prayed text, usage-weighted)",
        u.verified_occurrences,
        percent(u.verified_occurrences, u.total_occurrences)
    );
    let _ = writeln!(w, "  needs-review occ.:      {:5}", u.needs_review_occurrences);
    let _ = writeln!(w, "  source-unknown occ.:    {:5}", u.source_unknown_occurrences);
    w
}

/// The unverified entries a sweep never selects, by key shape.
pub struct ZeroReport {
    pub start_year: i32,
    pub years: i32,
    pub entries: Vec<(QueueEntry, ZeroHeuristic)>,
}

pub fn build_zero_report(src: &dyn DataSource, start: i32, years: i32) -> Result<ZeroReport, String> {
    let queue = build_provenance_queue(src, start, years, false)?;
    let mut entries: Vec<(QueueEntry, ZeroHeuristic)> =
        queue.entries.into_iter().filter(|e| e.occurrences == 0).map(|e| (ZeroHeuristic::detect(&e.key), e)).map(|(h, e)| (e, h)).collect();
    entries.sort_by(|(a, ha), (b, hb)| ha.cmp(hb).then_with(|| a.key.cmp(&b.key)));
    Ok(ZeroReport { start_year: queue.start_year, years: queue.years, entries })
}

pub fn zero_csv(r: &ZeroReport) -> String {
    let mut out = String::new();
    csv::write_record(&mut out, &["key", "content_hash", "status", "heuristic", "classification_state", "disposition", "issue", "reason"]);
    for (e, h) in &r.entries {
        let (state, disposition, issue, reason) = match &e.zero_classification {
            None => ("missing", "", "", ""),
            Some(z) => (if z.stale { "stale" } else { "current" }, z.disposition.as_str(), z.issue.as_str(), z.reason.as_str()),
        };
        csv::write_record(&mut out, &[&e.key, &e.content_hash, e.status.as_str(), h.as_str(), state, disposition, issue, reason]);
    }
    out
}

pub fn zero_summary(r: &ZeroReport) -> String {
    let mut heuristics: BTreeMap<ZeroHeuristic, usize> = BTreeMap::new();
    let mut dispositions: BTreeMap<ZeroDisposition, usize> = BTreeMap::new();
    let (mut classified, mut unclassified, mut stale) = (0, 0, 0);
    for (e, h) in &r.entries {
        *heuristics.entry(*h).or_default() += 1;
        match &e.zero_classification {
            None => unclassified += 1,
            Some(z) if z.stale => {
                stale += 1;
                unclassified += 1;
            }
            Some(z) if z.disposition == ZeroDisposition::Unclassified => {
                unclassified += 1;
                *dispositions.entry(z.disposition).or_default() += 1;
            }
            Some(z) => {
                classified += 1;
                *dispositions.entry(z.disposition).or_default() += 1;
            }
        }
    }
    let mut w = String::new();
    let _ = writeln!(w, "=== Zero-occurrence corpus entries: {}-{} ===", r.start_year, r.start_year + r.years - 1);
    let _ = writeln!(w, "  zero entries:         {}", r.entries.len());
    let _ = writeln!(w, "  classified:           {classified}");
    let _ = writeln!(w, "  need classification:  {unclassified}");
    let _ = writeln!(w, "  stale classifications:{stale:5}");
    for h in ZeroHeuristic::ALL {
        let _ = writeln!(w, "  heuristic {:<25} {}", format!("{}:", h.as_str()), heuristics.get(&h).copied().unwrap_or(0));
    }
    for d in ZeroDisposition::ALL {
        let _ = writeln!(w, "  disposition {:<23} {}", format!("{}:", d.as_str()), dispositions.get(&d).copied().unwrap_or(0));
    }
    w
}
