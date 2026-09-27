//! `review` and its subcommands. Ported from Go's `cli/review.go`.

use std::io::Write;

use calendar::Date;
use tools::fs::FsData;
use tools::review::{DEFAULT_BASE_URL, assurance, gate, manifest, prescreen, provenance, queue, resolution};

use crate::args::Flags;
use crate::checks::REPORTED;
use crate::commands::{take_prayer_form, today};

const REVIEW_USAGE: &str = "Usage: office review <subcommand> [args]

Subcommands:
  manifest [-start YEAR] [-years N] [-base URL]
                                         Inventory distinct rendered compositions as CSV
  provenance [-csv] [-start YEAR] [-years N]
                                         Report structured corpus provenance, plus (unless -csv)
                                         verified % of that sweep's rendered text weighted by
                                         how often each entry is actually prayed
  provenance-queue [-start YEAR] [-years N] [-base URL] [-summary] [-include-verified] [-suspect-only]
                                         Rank atomic text review by dependency fan-out,
                                         suspect (pre-flagged) entries first
  zero-occurrences [-start YEAR] [-years N] [-summary]
                                         List unverified entries never selected in a sweep
  resolution-inventory [-start YEAR] [-years N] [-json] [-fallback-only] [-summary]
                                         List effective dynamic-proper resolutions and fallbacks
  attest [flags] KEY REVIEWER            Record a source attestation for one text
  flag [flags] KEY                       Record a prescreen suspicion for one text
  assurance [-markdown] [-update-baseline] Check text-provenance floor and print summary
  explain HOUR YYYY-MM-DD               Print a composition assurance manifest as JSON
  plan [-start YEAR] [-years N] [-base URL] [-summary] [-include-sources]
                                         Sample observed engine behavior (default 28y); no completion score";

type Out<'a> = &'a mut dyn Write;

fn put(out: Out, s: &str) -> Result<(), String> {
    out.write_all(s.as_bytes()).map_err(|e| e.to_string())
}

pub fn cmd_review(data: &FsData, args: &[String], out: Out) -> Result<(), String> {
    let Some((name, rest)) = args.split_first() else { return Err(REVIEW_USAGE.into()) };
    match name.as_str() {
        "manifest" => review_manifest(data, rest, out),
        "status" | "sign" => {
            Err(format!("office review {name} is retired; record source-backed composition checks in tests and issues (see REVIEWING.md)"))
        }
        "provenance" => review_provenance(data, rest, out),
        "provenance-queue" => review_provenance_queue(data, rest, out),
        "zero-occurrences" => review_zero_occurrences(data, rest, out),
        "resolution-inventory" => review_resolution_inventory(data, rest, out),
        "attest" => review_attest(data, rest, out),
        "flag" => review_flag(data, rest, out),
        "assurance" => review_assurance(data, rest, out),
        "explain" => review_explain(data, rest, out),
        "plan" => review_plan(data, rest, out),
        _ => Err(REVIEW_USAGE.into()),
    }
}

fn year_flag(flags: &Flags, name: &str, default: i32) -> Result<i32, String> {
    let v = flags.int(name, i64::from(default))?;
    i32::try_from(v).map_err(|_| format!("invalid value \"{v}\" for flag -{name}: value out of range"))
}

/// `-start`, `-years`, and `-base`, shared by the sweeping subcommands.
fn sweep_flags(flags: &Flags, default_years: i32) -> Result<(i32, i32), String> {
    Ok((year_flag(flags, "start", today().year())?, year_flag(flags, "years", default_years)?))
}

fn review_manifest(data: &FsData, args: &[String], out: Out) -> Result<(), String> {
    let flags = Flags::parse(args, &["start", "years", "base"])?;
    let (start, years) = sweep_flags(&flags, 1)?;
    let m = manifest::build_manifest(data, start, years).map_err(|e| format!("building review manifest: {e}"))?;
    put(out, &manifest::manifest_csv(&m, flags.str_or("base", DEFAULT_BASE_URL)))
}

fn review_provenance(data: &FsData, args: &[String], out: Out) -> Result<(), String> {
    let flags = Flags::parse_with_bools(args, &["start", "years"], &["csv"])?;
    let (start, years) = sweep_flags(&flags, 1)?;
    let inventory = provenance::scan_provenance(data).map_err(|e| format!("scanning provenance: {e}"))?;
    if flags.bool("csv") {
        return put(out, &provenance::provenance_csv(&inventory));
    }
    put(out, &provenance::provenance_summary(&inventory))?;
    let weighted = queue::build_usage_weighted(data, start, years).map_err(|e| format!("building usage-weighted provenance: {e}"))?;
    put(out, "\n")?;
    put(out, &queue::usage_weighted_summary(&weighted))
}

fn review_provenance_queue(data: &FsData, args: &[String], out: Out) -> Result<(), String> {
    let flags = Flags::parse_with_bools(args, &["start", "years", "base"], &["summary", "include-verified", "suspect-only"])?;
    let (start, years) = sweep_flags(&flags, 1)?;
    let mut q = queue::build_provenance_queue(data, start, years, flags.bool("include-verified"))
        .map_err(|e| format!("building provenance queue: {e}"))?;
    if flags.bool("suspect-only") {
        q.filter_suspect();
    }
    if flags.bool("summary") {
        return put(out, &queue::queue_summary(&q));
    }
    put(out, &queue::queue_csv(&q, flags.str_or("base", DEFAULT_BASE_URL)))
}

fn review_zero_occurrences(data: &FsData, args: &[String], out: Out) -> Result<(), String> {
    let flags = Flags::parse_with_bools(args, &["start", "years"], &["summary"])?;
    let (start, years) = sweep_flags(&flags, 1)?;
    let report = queue::build_zero_report(data, start, years).map_err(|e| format!("building zero-occurrence report: {e}"))?;
    if flags.bool("summary") {
        return put(out, &queue::zero_summary(&report));
    }
    put(out, &queue::zero_csv(&report))
}

fn review_attest(data: &FsData, args: &[String], out: Out) -> Result<(), String> {
    let flags = Flags::parse_with_bools(args, &["source", "locator", "page", "note", "date"], &["replace"])?;
    let [key, reviewer] = flags.rest.as_slice() else {
        return Err("usage: office review attest --source SOURCE [--page PAGE|--locator LOCATOR] [--note NOTE] [--date YYYY-MM-DD] [--replace] KEY REVIEWER".into());
    };
    let today = today().to_string();
    let entry = provenance::record_attestation(
        data,
        &data.dir,
        provenance::AttestOptions {
            key: key.clone(),
            reviewer: reviewer.clone(),
            source: flags.str("source").into(),
            locator: flags.str("locator").into(),
            page: flags.str("page").into(),
            reviewed_on: flags.str_or("date", &today).into(),
            notes: flags.str("note").into(),
            replace: flags.bool("replace"),
        },
    )
    .map_err(|e| format!("recording attestation: {e}"))?;
    put(out, &format!("Verified {} ({}, {})\n", entry.key, entry.reviewer, entry.reviewed_on))
}

fn review_flag(data: &FsData, args: &[String], out: Out) -> Result<(), String> {
    let flags = Flags::parse_with_bools(args, &["severity", "reason", "flagged", "issue"], &["replace"])?;
    let [key] = flags.rest.as_slice() else { return Err(FLAG_USAGE.into()) };
    if flags.str("reason").is_empty() {
        return Err(FLAG_USAGE.into());
    }
    let month = today().to_string()[..7].to_string();
    let recorded = prescreen::record_prescreen_flag(
        data,
        &data.dir,
        prescreen::PrescreenFlag {
            key: key.clone(),
            content_hash: String::new(),
            severity: flags.str_or("severity", "high").into(),
            reason: flags.str("reason").into(),
            flagged: flags.str_or("flagged", &month).into(),
            issue: flags.str("issue").into(),
        },
        flags.bool("replace"),
    )
    .map_err(|e| format!("recording prescreen flag: {e}"))?;
    put(out, &format!("Flagged {} ({}): {}\n", recorded.key, recorded.severity, recorded.reason))
}

const FLAG_USAGE: &str =
    "usage: office review flag --reason REASON [--severity high|medium] [--flagged YYYY-MM] [--issue N] [--replace] KEY";

fn review_assurance(data: &FsData, args: &[String], out: Out) -> Result<(), String> {
    let flags = Flags::parse_with_bools(args, &[], &["markdown", "update-baseline"])?;
    let mut baseline = gate::load_assurance_baseline(data).map_err(|e| format!("loading assurance baseline: {e}"))?;
    let report =
        gate::build_assurance_report(data, baseline.start_year, baseline.years).map_err(|e| format!("building assurance report: {e}"))?;
    if flags.bool("update-baseline") {
        gate::update_assurance_baseline(&data.dir, &report).map_err(|e| format!("updating assurance baseline: {e}"))?;
        baseline.verified_minimum = report.verified;
    }
    let failures = gate::evaluate_assurance(&report, &baseline);
    put(out, &gate::assurance_summary(&report, &failures, flags.bool("markdown")))?;
    if failures.is_empty() { Ok(()) } else { Err(REPORTED.into()) }
}

fn review_explain(data: &FsData, args: &[String], out: Out) -> Result<(), String> {
    let (args, form) = take_prayer_form(args)?;
    let [hour, date] = args.as_slice() else {
        return Err("usage: office review explain HOUR YYYY-MM-DD [--form private|deacon|priest]".into());
    };
    let date = Date::parse(date).ok_or_else(|| format!("invalid date (use YYYY-MM-DD): {date}"))?;
    let json = assurance::explain_composition(data, hour, date, form).map_err(|e| format!("explaining composition: {e}"))?;
    put(out, &json)
}

fn review_plan(data: &FsData, args: &[String], out: Out) -> Result<(), String> {
    let flags = Flags::parse_with_bools(args, &["start", "years", "base"], &["summary", "include-sources"])?;
    // Include examples of rare concurrence and octave edges absent from one year.
    let (start, years) = sweep_flags(&flags, 28)?;
    let plan = assurance::build_review_plan(data, start, years, flags.bool("include-sources"))
        .map_err(|e| format!("building review plan: {e}"))?;
    if flags.bool("summary") {
        return put(out, &assurance::review_plan_summary(&plan));
    }
    put(out, &assurance::review_plan_csv(&plan, flags.str_or("base", DEFAULT_BASE_URL)))?;
    // Keep the CSV machine-readable; describe its limited purpose on stderr.
    eprintln!(
        "composition samples: {} pages, {} observed features; not a rubric checklist or correctness measure",
        plan.selected.len(),
        plan.features.len()
    );
    Ok(())
}

fn review_resolution_inventory(data: &FsData, args: &[String], out: Out) -> Result<(), String> {
    let flags = Flags::parse_with_bools(args, &["start", "years"], &["json", "fallback-only", "summary"])?;
    let (start, years) = sweep_flags(&flags, 28)?;
    let mut inventory =
        resolution::build_resolution_inventory(data, start, years).map_err(|e| format!("building resolution inventory: {e}"))?;
    if flags.bool("fallback-only") {
        inventory.filter_fallbacks();
    }
    if flags.bool("json") {
        return put(out, &inventory.json());
    }
    if flags.bool("summary") {
        return put(out, &inventory.summary());
    }
    put(out, &inventory.tsv())
}
