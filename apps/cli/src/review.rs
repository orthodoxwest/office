//! `review` and its subcommands.

use std::io::Write;

use calendar::Date;
use tools::fs::FsData;
use tools::review::{explain, provenance};

use crate::args::Flags;
use crate::commands::{take_prayer_form, today};

const REVIEW_USAGE: &str = "Usage: office review <subcommand> [args]

Subcommands:
  provenance [-csv] [-start YEAR] [-years N]
                                         Report structured corpus provenance, plus (unless -csv)
                                         verified % of that sweep's rendered text weighted by
                                         how often each entry is actually prayed
  attest [flags] KEY REVIEWER            Record a source attestation for one text
  explain HOUR YYYY-MM-DD [--form private|deacon|priest]
                                         Print a composition explanation as JSON";

type Out<'a> = &'a mut dyn Write;

fn put(out: Out, s: &str) -> Result<(), String> {
    out.write_all(s.as_bytes()).map_err(|e| e.to_string())
}

pub fn cmd_review(data: &FsData, args: &[String], out: Out) -> Result<(), String> {
    let Some((name, rest)) = args.split_first() else { return Err(REVIEW_USAGE.into()) };
    match name.as_str() {
        "provenance" => review_provenance(data, rest, out),
        "attest" => review_attest(data, rest, out),
        "explain" => review_explain(data, rest, out),
        _ => Err(REVIEW_USAGE.into()),
    }
}

fn review_provenance(data: &FsData, args: &[String], out: Out) -> Result<(), String> {
    let flags = Flags::parse_with_bools(args, &["start", "years"], &["csv"])?;
    let start = flags.int("start", today().year())?;
    let years = flags.int("years", 1)?;
    let inventory = provenance::scan_provenance(data).map_err(|e| format!("scanning provenance: {e}"))?;
    if flags.bool("csv") {
        return put(out, &provenance::provenance_csv(&inventory));
    }
    put(out, &provenance::provenance_summary(&inventory))?;
    let weighted = provenance::build_usage_weighted(data, start, years).map_err(|e| format!("building usage-weighted provenance: {e}"))?;
    put(out, "\n")?;
    put(out, &provenance::usage_weighted_summary(&weighted))
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

fn review_explain(data: &FsData, args: &[String], out: Out) -> Result<(), String> {
    let (args, form) = take_prayer_form(args)?;
    let [hour, date] = args.as_slice() else {
        return Err("usage: office review explain HOUR YYYY-MM-DD [--form private|deacon|priest]".into());
    };
    let date = Date::parse(date).ok_or_else(|| format!("invalid date (use YYYY-MM-DD): {date}"))?;
    let json = explain::explain_composition(data, hour, date, form).map_err(|e| format!("explaining composition: {e}"))?;
    put(out, &json)
}
