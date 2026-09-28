//! Composition explanations and the sample planner.

use std::collections::{BTreeSet, HashMap, HashSet};
use std::fmt::Write as _;

use calendar::{CalendarData, DataSource, Date, Decision};
use data_format::csv;
use data_format::json::{Json, Obj};
use liturgy::{OfficeElement, OfficeHour, PrayerForm};
use office::day::Day;
use office::engine::Engine;
use office::psalmody::{VESPERS_OF_THE_DEAD_LABEL, dead_office_day};
use office::trace::ProperResolutionTrace;

use super::provenance::{ProvenanceStatus, SourceCitation, scan_provenance};
use super::sweep::{Forms, sweep};
use super::{base_url, celebration_name, celebration_rank, context_note, hash_hour, hour_order, hour_tier, priority, unit_key};

/// The sorted, unique corpus keys behind a rendered hour.
pub fn hour_dependencies(hour: &OfficeHour) -> Vec<String> {
    let mut refs = BTreeSet::new();
    for elem in hour.sections.iter().flat_map(|s| &s.elements) {
        if elem.source_refs.is_empty() {
            if !elem.source_ref.is_empty() {
                refs.insert(elem.source_ref.clone());
            }
        } else {
            refs.extend(elem.source_refs.iter().filter(|r| !r.is_empty()).cloned());
        }
    }
    refs.into_iter().collect()
}

/// Trace events without repeats, in first-seen order.
pub fn dedupe_decisions(decisions: &[Decision]) -> Vec<&Decision> {
    let mut seen = HashSet::new();
    decisions.iter().filter(|d| seen.insert((d.rule.as_str(), d.outcome.as_str(), d.detail.as_deref().unwrap_or("")))).collect()
}

/// Traces a rendered slot, as a commemoration when it is one.
pub fn trace_element(engine: &Engine, day: &Day, hour_name: &str, elem: &OfficeElement, appended_dead: bool) -> ProperResolutionTrace {
    let dead;
    let day = if appended_dead {
        dead = dead_office_day(day);
        &dead
    } else {
        day
    };
    if elem.is_commemoration {
        return engine.trace_commemoration_resolution(day, hour_name, &elem.slot_ref, &elem.source_ref, &elem.commemoration_owner_id);
    }
    engine.trace_proper_resolution(day, hour_name, &elem.slot_ref, &elem.source_ref)
}

fn is_resolution_source(r: &str) -> bool {
    ["proper/", "commons/", "seasonal/", "ordinary/"].iter().any(|p| r.starts_with(p))
}

/// One hour joined to its provenance, as the `review explain` JSON.
pub fn explain_composition(src: &dyn DataSource, hour_name: &str, date: Date, form: PrayerForm) -> Result<String, String> {
    let days = crate::year::load_office_days(src, date.year())?;
    let day = days.get(date.ordinal() as usize - 1).ok_or_else(|| format!("date out of range: {date}"))?;
    let engine = Engine::load(src)?;
    let hour = engine.compose_hour(hour_name, day, &calendar::MoveableDates::compute(date.year()), form)?;
    let inventory = scan_provenance(src)?;
    let by_key = inventory.by_key();

    let decisions = dedupe_decisions(&hour.decisions)
        .into_iter()
        .map(|d| {
            Obj::new().str("rule", &d.rule).str("outcome", &d.outcome).str_omitempty("detail", d.detail.as_deref().unwrap_or("")).build()
        })
        .collect();
    let dependencies: Vec<Json> = hour_dependencies(&hour)
        .iter()
        .map(|r| match by_key.get(r.as_str()) {
            None => Obj::new().str("key", r).str("status", ProvenanceStatus::SourceUnknown.as_str()).build(),
            Some(e) => Obj::new()
                .str("key", r)
                .str("status", e.status.as_str())
                .str_omitempty("file", &e.file)
                .str_omitempty("section", &e.section)
                .field_omitempty_arr("sources", e.sources.iter().map(citation_json).collect())
                .build(),
        })
        .collect();
    let mut resolutions = Vec::new();
    let mut appended_dead = false;
    for section in &hour.sections {
        appended_dead |= section.label == VESPERS_OF_THE_DEAD_LABEL;
        for elem in &section.elements {
            if elem.slot_ref.is_empty() {
                continue;
            }
            let t = trace_element(&engine, day, hour_name, elem, appended_dead);
            if !is_resolution_source(&elem.source_ref) && t.selected_tier != "not-found" {
                continue;
            }
            resolutions.push(
                Obj::new()
                    .str("requested_slot", &t.requested_slot)
                    .str("resolver_hour", &t.resolver_hour)
                    .str("resolver_slot", &t.resolver_slot)
                    .str_omitempty("canonical_owner", &t.canonical_owner)
                    .strings_omitempty("proper_ids", &t.proper_ids)
                    .strings_omitempty("direct_candidates", &t.direct_candidates)
                    .strings_omitempty("direct_existing", &t.direct_existing)
                    .str("selected_ref", &t.selected_ref)
                    .str("selected_tier", &t.selected_tier)
                    .str("reason", &t.reason)
                    .bool_omitempty("first_vespers", t.first_vespers)
                    .build(),
            );
        }
    }
    let v = Obj::new()
        .str("form", hour.form.as_str())
        .str("date", &date.to_string())
        .str("hour", hour_name)
        .str("unit_key", &unit_key(day, hour_name))
        .str("celebration", &celebration_name(day))
        .str("season", hour.season.map_or("", |s| s.as_str()))
        .str("color", hour.color.map_or("", |c| c.as_str()))
        .field("decisions", Json::Arr(decisions))
        .field("dependencies", if dependencies.is_empty() { Json::Null } else { Json::Arr(dependencies) })
        .field_omitempty_arr("resolutions", resolutions)
        .build();
    Ok(data_format::json::encode_indent(&v))
}

fn citation_json(s: &SourceCitation) -> Json {
    let mut o = Obj::new()
        .str("kind", &s.kind)
        .str("source", &s.source)
        .str_omitempty("locator", &s.locator)
        .str_omitempty("page", &s.page)
        .str_omitempty("note", &s.note);
    if s.line != 0 {
        o = o.int("line", s.line as i64);
    }
    o.build()
}

trait ObjExt {
    fn field_omitempty_arr(self, name: &str, items: Vec<Json>) -> Obj;
}

impl ObjExt for Obj {
    fn field_omitempty_arr(self, name: &str, items: Vec<Json>) -> Obj {
        if items.is_empty() { self } else { self.field(name, Json::Arr(items)) }
    }
}

/// One composition considered by the planner and the provenance queue.
#[derive(Clone, Debug)]
pub struct ReviewCandidate {
    pub form: PrayerForm,
    pub hash: String,
    pub priority: &'static str,
    pub hour: &'static str,
    pub date: Date,
    pub unit_key: String,
    pub celebration: String,
    pub context: String,
    pub dependencies: Vec<String>,
    pub decisions: Vec<String>,
    /// Observed engine decisions and source tiers, not rubric requirements.
    pub features: Vec<String>,
}

/// Whether a feature takes part in the default sample: context tags, pure
/// weekday psalmody gates, and the redundant occurrence alias are left out.
fn is_sample_feature(feature: &str) -> bool {
    if feature.starts_with("source:")
        || feature.starts_with("decision:context:")
        || feature.starts_with("decision:office-context:")
        || feature.starts_with("decision:occurrence=")
        || is_weekday_psalmody_noise(feature)
    {
        return false;
    }
    feature.starts_with("decision:") || feature.starts_with("resolution:")
}

fn is_weekday_psalmody_noise(feature: &str) -> bool {
    let Some(body) = feature.strip_prefix("decision:condition:") else { return false };
    let cond = body.split_once('=').map_or(body, |(c, _)| c);
    cond.contains("not-festal-vespers-psalmody,weekday-")
        || (cond.contains("not-is-feast,weekday-") && cond.contains("not-festal-lauds-psalmody"))
        || (cond.starts_with("weekday-") && !cond.contains(','))
}

fn cover_feature(feature: &str, include_sources: bool) -> bool {
    if feature.starts_with("source:") {
        return include_sources;
    }
    is_sample_feature(feature)
}

pub fn candidate_for(day: &Day, hour_name: &'static str, hour: &OfficeHour, include_sources: bool) -> ReviewCandidate {
    let dependencies = hour_dependencies(hour);
    let mut features: BTreeSet<String> = BTreeSet::new();
    if include_sources {
        features.extend(dependencies.iter().map(|r| format!("source:{r}")));
    }
    let mut decisions: BTreeSet<String> = BTreeSet::new();
    for d in dedupe_decisions(&hour.decisions) {
        let decision = format!("{}={}", d.rule, d.outcome);
        let feat = format!("decision:{decision}");
        if cover_feature(&feat, include_sources) {
            features.insert(feat);
            decisions.insert(decision);
        }
    }
    for elem in hour.sections.iter().flat_map(|s| &s.elements) {
        if elem.slot_ref.is_empty() || elem.source_ref.is_empty() {
            continue;
        }
        let tier = elem.source_ref.split('/').next().unwrap_or("");
        let feat = format!("resolution:{}={tier}", elem.slot_ref);
        if cover_feature(&feat, include_sources) {
            features.insert(feat);
        }
    }
    ReviewCandidate {
        form: hour.form,
        hash: hash_hour(hour),
        priority: priority(celebration_rank(day, hour_name), day.date),
        hour: hour_name,
        date: day.date,
        unit_key: unit_key(day, hour_name),
        celebration: celebration_name(day),
        context: context_note(day, hour_name),
        dependencies,
        decisions: decisions.into_iter().collect(),
        features: features.into_iter().collect(),
    }
}

pub fn candidate_cmp(a: &ReviewCandidate, b: &ReviewCandidate) -> std::cmp::Ordering {
    a.priority
        .cmp(b.priority)
        .then_with(|| hour_tier(a.hour).cmp(&hour_tier(b.hour)))
        .then_with(|| a.date.cmp(&b.date))
        .then_with(|| hour_order(a.hour).cmp(&hour_order(b.hour)))
        // Order by the serialized prayer-form names.
        .then_with(|| a.form.as_str().cmp(b.form.as_str()))
        .then_with(|| a.hash.cmp(&b.hash))
}

fn candidate_less(a: &ReviewCandidate, b: &ReviewCandidate) -> bool {
    candidate_cmp(a, b) == std::cmp::Ordering::Less
}

fn better_representative(a: &ReviewCandidate, b: &ReviewCandidate, primary_year: i32) -> bool {
    let (ap, bp) = (a.date.year() == primary_year, b.date.year() == primary_year);
    if ap != bp {
        return ap;
    }
    candidate_less(a, b)
}

/// Why one sample was chosen.
pub struct PlannedReview {
    pub candidate: ReviewCandidate,
    pub new_features: Vec<String>,
    /// Sum of the occurrences of `new_features`.
    pub exposure: usize,
    pub primary_year: bool,
}

/// A sample of observed engine behavior; not a completeness measure.
pub struct ReviewPlan {
    pub start_year: i32,
    pub years: i32,
    pub candidate_count: usize,
    pub features: Vec<String>,
    pub selected: Vec<PlannedReview>,
    pub include_sources: bool,
    pub primary_year_pages: usize,
    pub future_year_pages: usize,
}

/// A greedy frequency-weighted cover of every observed feature, primary year first.
pub fn build_review_plan(src: &dyn DataSource, start: i32, years: i32, include_sources: bool) -> Result<ReviewPlan, String> {
    if years < 1 {
        return Err("years must be at least 1".into());
    }
    let engine = Engine::load(src)?;
    let cal = CalendarData::load(src)?;
    let mut all_features: BTreeSet<String> = BTreeSet::new();
    let mut fan_out: HashMap<String, usize> = HashMap::new();
    // One representative per observed feature combination.
    let mut representatives: HashMap<String, ReviewCandidate> = HashMap::new();
    let mut candidate_count = 0;
    let mut fold = |c: ReviewCandidate| {
        candidate_count += 1;
        for f in &c.features {
            all_features.insert(f.clone());
            *fan_out.entry(f.clone()).or_default() += 1;
        }
        let sig = c.features.join("\x1f");
        match representatives.get(&sig) {
            Some(old) if !better_representative(&c, old, start) => {}
            _ => {
                representatives.insert(sig, c);
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
        &|day, hour_name, hour| {
            let mut c = candidate_for(day, hour_name, hour, include_sources);
            c.dependencies = Vec::new(); // source keys, when requested, are already features
            c
        },
        &mut fold,
    )?;
    let mut candidates: Vec<ReviewCandidate> = representatives.into_values().collect();
    candidates.sort_by(|a, b| {
        if better_representative(a, b, start) {
            std::cmp::Ordering::Less
        } else if better_representative(b, a, start) {
            std::cmp::Ordering::Greater
        } else {
            std::cmp::Ordering::Equal
        }
    });

    let mut plan = ReviewPlan {
        start_year: start,
        years,
        candidate_count,
        features: all_features.iter().cloned().collect(),
        selected: Vec::new(),
        include_sources,
        primary_year_pages: 0,
        future_year_pages: 0,
    };
    // Features as IDs in sorted order, so ID order is string order.
    let ids: HashMap<&str, usize> = plan.features.iter().enumerate().map(|(i, f)| (f.as_str(), i)).collect();
    let weights: Vec<usize> = plan.features.iter().map(|f| fan_out[f]).collect();
    let candidate_ids: Vec<Vec<usize>> = candidates.iter().map(|c| c.features.iter().map(|f| ids[f.as_str()]).collect()).collect();
    let mut unsampled = vec![true; plan.features.len()];
    let mut remaining = plan.features.len();
    let mut used = vec![false; candidates.len()];
    for primary_only in [true, false] {
        while remaining > 0 {
            // (index, new feature count, impact) of the best pick so far.
            let mut best: Option<(usize, usize, usize)> = None;
            for (i, c) in candidates.iter().enumerate() {
                if used[i] || (primary_only && c.date.year() != start) {
                    continue;
                }
                let (mut count, mut impact) = (0, 0);
                for &f in &candidate_ids[i] {
                    if unsampled[f] {
                        count += 1;
                        impact += weights[f];
                    }
                }
                if count == 0 {
                    continue;
                }
                let better = match best {
                    None => true,
                    Some((b, b_count, b_impact)) => better_cover_pick(c, impact, count, &candidates[b], b_impact, b_count, start),
                };
                if better {
                    best = Some((i, count, impact));
                }
            }
            let Some((i, _, impact)) = best else { break };
            used[i] = true;
            let mut newly = Vec::new();
            for &f in &candidate_ids[i] {
                if unsampled[f] {
                    unsampled[f] = false;
                    remaining -= 1;
                    newly.push(plan.features[f].clone());
                }
            }
            let primary = candidates[i].date.year() == start;
            if primary {
                plan.primary_year_pages += 1;
            } else {
                plan.future_year_pages += 1;
            }
            plan.selected.push(PlannedReview {
                candidate: candidates[i].clone(),
                new_features: newly,
                exposure: impact,
                primary_year: primary,
            });
        }
    }
    if remaining > 0 {
        return Err(format!("sample planner could not represent {remaining} observed features"));
    }
    Ok(plan)
}

fn better_cover_pick(
    c: &ReviewCandidate,
    impact: usize,
    new_count: usize,
    best: &ReviewCandidate,
    best_impact: usize,
    best_new: usize,
    primary: i32,
) -> bool {
    if impact != best_impact {
        return impact > best_impact;
    }
    let (cp, bp) = (c.date.year() == primary, best.date.year() == primary);
    if cp != bp {
        return cp;
    }
    if new_count != best_new {
        return new_count > best_new;
    }
    candidate_less(c, best)
}

pub fn review_plan_summary(p: &ReviewPlan) -> String {
    let mut w = String::new();
    let _ = writeln!(w, "=== Composition samples: {}-{} ===", p.start_year, p.start_year + p.years - 1);
    w.push_str("Examples of observed engine behavior; not a rubric checklist or correctness measure.\n");
    let _ = writeln!(w, "  composed date-hour forms: {}", p.candidate_count);
    let _ = writeln!(w, "  observed features:       {}", p.features.len());
    let _ = writeln!(w, "  sample pages:            {}", p.selected.len());
    let _ = writeln!(w, "  primary-year samples:    {}", p.primary_year_pages);
    let _ = writeln!(w, "  later-year samples:      {}", p.future_year_pages);
    let _ = writeln!(w, "  source keys included:    {}", p.include_sources);
    w
}

pub fn review_plan_csv(p: &ReviewPlan, base: &str) -> String {
    let base = base_url(base);
    let mut out = String::new();
    csv::write_record(
        &mut out,
        &[
            "order",
            "priority",
            "hour",
            "date",
            "unit_key",
            "celebration",
            "context",
            "primary_year",
            "feature_exposure",
            "sampled_features",
            "url",
        ],
    );
    for (i, s) in p.selected.iter().enumerate() {
        let c = &s.candidate;
        let (order, date, exposure) = ((i + 1).to_string(), c.date.to_string(), s.exposure.to_string());
        let features = s.new_features.join("; ");
        let url = format!("{base}/{}/{date}?form={}", c.hour, c.form.as_str());
        csv::write_record(
            &mut out,
            &[
                &order,
                c.priority,
                c.hour,
                &date,
                &c.unit_key,
                &c.celebration,
                &c.context,
                if s.primary_year { "yes" } else { "no" },
                &exposure,
                &features,
                &url,
            ],
        );
    }
    out
}
