//! `office`: the command-line front end and web-server entry point.

// The CLI matches serde_json values, not liturgical enums.
#![allow(clippy::wildcard_enum_match_arm)]

mod args;
mod checks;
mod commands;
mod dump;
mod dump_tools;
mod edit;
mod parity;
mod review;

use std::io::Write;
use std::process::ExitCode;

const USAGE: &str = "usage: office <command> [args]

Commands: ordo, rubrics, validate, audit, lint, review, corpus, scaffold, dump, lauds, prime, terce, sext, none, vespers, compline, tex, serve";

/// Starts the web server and blocks: `serve [ADDR]`, ":8080" by default.
fn cmd_serve(data: &tools::fs::FsData, args: &[String], _out: &mut dyn Write) -> Result<(), String> {
    let addr = args.first().map(String::as_str).unwrap_or(":8080");
    let mut server = office_web::Server::new(&data.dir).map_err(|e| format!("creating server: {e}"))?;
    server.open_usage_from_env();
    server.canonical_host_from_env();
    eprintln!("Listening on http://localhost{addr}");
    office_web::run(server, addr)
}

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
        "corpus" => edit::cmd_corpus,
        "scaffold" => edit::cmd_scaffold,
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
        "serve" => cmd_serve,
        _ => {
            eprintln!("Unknown command: {name}");
            return ExitCode::FAILURE;
        }
    };
    let stdout = std::io::stdout();
    let mut out = std::io::BufWriter::with_capacity(1 << 20, stdout.lock());
    // File inspection does not need an installed corpus.
    let result = match (name.as_str(), rest.first().map(String::as_str)) {
        ("dump", Some("diff")) => dump_tools::cmd_diff(&rest[1..], &mut out),
        _ => {
            let Some(data) = tools::fs::FsData::find() else {
                eprintln!("Cannot find data directory");
                return ExitCode::FAILURE;
            };
            command(&data, rest, &mut out)
        }
    }
    .and_then(|()| out.flush().map_err(|e| e.to_string()));
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
