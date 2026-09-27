//! `office-rs`: the command-line front end of the Rust engine. It mirrors the
//! Go `office` commands as they are ported (RUST-PORT.md).

mod args;
mod checks;
mod commands;
mod dump;
mod review;

use std::io::Write;
use std::process::ExitCode;

const USAGE: &str = "usage: office-rs <command> [args]

Commands: ordo, rubrics, validate, audit, lint, review, dump, lauds, prime, terce, sext, none, vespers, compline, tex";

fn main() -> ExitCode {
    let args: Vec<String> = std::env::args().skip(1).collect();
    let Some((name, rest)) = args.split_first() else {
        eprintln!("{USAGE}");
        return ExitCode::FAILURE;
    };
    let command: fn(&tools::fs::FsData, &[String], &mut dyn Write) -> Result<(), String> = match name.as_str() {
        "dump" => dump::cmd_dump,
        "validate" => checks::cmd_validate,
        "audit" => checks::cmd_audit,
        "lint" => checks::cmd_lint,
        "review" => review::cmd_review,
        "ordo" => commands::cmd_ordo,
        "rubrics" => commands::cmd_rubrics,
        "lauds" => |d, a, o| commands::cmd_hour("lauds", d, a, o),
        "prime" => |d, a, o| commands::cmd_hour("prime", d, a, o),
        "terce" => |d, a, o| commands::cmd_hour("terce", d, a, o),
        "sext" => |d, a, o| commands::cmd_hour("sext", d, a, o),
        "none" => |d, a, o| commands::cmd_hour("none", d, a, o),
        "vespers" => |d, a, o| commands::cmd_hour("vespers", d, a, o),
        "compline" => |d, a, o| commands::cmd_hour("compline", d, a, o),
        "tex" => commands::cmd_tex,
        _ => {
            eprintln!("Unknown command: {name}");
            return ExitCode::FAILURE;
        }
    };
    let Some(data) = tools::fs::FsData::find() else {
        eprintln!("Cannot find data directory");
        return ExitCode::FAILURE;
    };
    let stdout = std::io::stdout();
    let mut out = std::io::BufWriter::with_capacity(1 << 20, stdout.lock());
    let result = command(&data, rest, &mut out).and_then(|()| out.flush().map_err(|e| e.to_string()));
    match result {
        Ok(()) => ExitCode::SUCCESS,
        Err(e) => {
            drop(out);
            if e != checks::REPORTED {
                eprintln!("{e}");
            }
            ExitCode::FAILURE
        }
    }
}
