//! `validate`, `audit`, and `lint`. Ported from Go's `cli/checks.go`.

use std::io::Write;

use tools::fs::FsData;
use tools::review::{provenance, zero_occurrence};

/// Go's `errReported`: the findings are already written, so the command
/// fails without a further message.
pub const REPORTED: &str = "";

/// `validate`: every data layer, the hour definitions, and the review ledgers.
pub fn cmd_validate(data: &FsData, args: &[String], out: &mut dyn Write) -> Result<(), String> {
    if !args.is_empty() {
        return Err("usage: office validate".into());
    }
    let mut errs = tools::validate::validate_calendar(data);
    errs.extend(office::validate::validate_hour_definitions(data));
    errs.extend(tools::validate::validate_texts(data));
    match provenance::scan_provenance(data) {
        Err(e) => errs.push(format!("review provenance: {e}")),
        Ok(inventory) => {
            if let Err(e) = zero_occurrence::load_zero_classifications(data, &inventory) {
                errs.push(format!("review zero occurrences: {e}"));
            }
        }
    }
    let io = |e: std::io::Error| e.to_string();
    if errs.is_empty() {
        writeln!(out, "All data files valid.").map_err(io)?;
        return Ok(());
    }
    writeln!(out, "Validation errors found:\n").map_err(io)?;
    for e in &errs {
        writeln!(out, "  {e}").map_err(io)?;
    }
    Err(REPORTED.into())
}
