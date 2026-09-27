//! `corpus` and `scaffold`: the data-editing commands. Ported from Go's
//! `cli/corpus.go` and `cli/scaffold.go`.

use std::io::Write;

use tools::fs::FsData;
use tools::scaffold::{self, Action};

use crate::args::Flags;
use crate::checks::REPORTED;

const CORPUS_USAGE: &str = "Usage: office corpus <subcommand> [args]

Subcommands:
  show KEY                                  Print a corpus section body
  put KEY --file BODY.txt --source SOURCE   Replace or activate a corpus section";

pub fn cmd_corpus(data: &FsData, args: &[String], out: &mut dyn Write) -> Result<(), String> {
    if args.len() < 2 {
        return Err(CORPUS_USAGE.into());
    }
    let io = |e: std::io::Error| e.to_string();
    match args[0].as_str() {
        "show" => {
            if args.len() != 2 {
                return Err("usage: office corpus show KEY".into());
            }
            let body = tools::corpus_edit::read_corpus_body(&data.dir, &args[1])?;
            writeln!(out, "{body}").map_err(io)
        }
        "put" => {
            let key = &args[1];
            let flags = Flags::parse(&args[2..], &["file", "source"])?;
            if !flags.rest.is_empty() || flags.str("file").is_empty() || flags.str("source").is_empty() {
                return Err("usage: office corpus put KEY --file BODY.txt --source SOURCE".into());
            }
            let path = std::path::Path::new(flags.str("file"));
            let body = std::fs::read(path).map_err(|e| format!("reading body file: {}", tools::fs::io_error("open", path, &e)))?;
            tools::corpus_edit::put_corpus_body(&data.dir, key, &String::from_utf8_lossy(&body), flags.str("source"))?;
            writeln!(out, "Updated {key}").map_err(io)
        }
        _ => Err(CORPUS_USAGE.into()),
    }
}

pub fn cmd_scaffold(data: &FsData, args: &[String], out: &mut dyn Write) -> Result<(), String> {
    let Some((sub, rest)) = args.split_first() else {
        return Err("usage: office scaffold <propers> [flags]\n\nSubcommands:\n  propers   ensure proper text files exist with commented key catalogs".into());
    };
    if sub != "propers" {
        return Err(format!("unknown scaffold subcommand {}\nusage: office scaffold propers [flags]", compat::quote(sub)));
    }
    let flags = Flags::parse_with_bools(rest, &["feast"], &["check", "dry-run", "include-commemorations"])?;
    if !flags.rest.is_empty() {
        return Err("usage: office scaffold propers [-check] [-dry-run] [-include-commemorations] [-feast ID]".into());
    }
    let (check, dry_run) = (flags.bool("check"), flags.bool("dry-run"));
    if check && dry_run {
        return Err("use only one of -check and -dry-run".into());
    }
    let feast = flags.str("feast");
    let results = scaffold::ensure_propers(
        data,
        &data.dir,
        &scaffold::Options { write: !check && !dry_run, include_commemorations: flags.bool("include-commemorations"), feast_id: feast },
    )?;
    let (created, appended, ok, skipped) = scaffold::summarize(&results);
    let mode = if check {
        "check"
    } else if dry_run {
        "dry-run"
    } else {
        "wrote"
    };
    let mut w = String::new();
    for r in &results {
        match r.action {
            Action::Create => w.push_str(&format!("create  {}  (+{} keys)\n", r.path, r.added_keys.len())),
            Action::Append => w.push_str(&format!("append  {}  (+{} keys: {})\n", r.path, r.added_keys.len(), r.added_keys.join(", "))),
            Action::Skip if !feast.is_empty() => w.push_str(&format!("skip    {}  ({})\n", r.feast_id, r.skip_reason)),
            Action::Ok if !feast.is_empty() => w.push_str(&format!("ok      {}\n", r.path)),
            Action::Skip | Action::Ok => {}
        }
    }
    w.push_str(&format!("\nscaffold propers ({mode}): create={created} append={appended} ok={ok} skip={skipped}\n"));
    out.write_all(w.as_bytes()).map_err(|e| e.to_string())?;
    if check && scaffold::needs_work(&results) { Err(REPORTED.into()) } else { Ok(()) }
}
