//! Year and hour commands: `ordo`, `rubrics`, and one command per hour.
//! Mirrors Go's `cli/ordo.go` and `cli/hours.go`.

use std::io::Write;

use calendar::{CalendarData, Date, MoveableDates, build_calendar};
use liturgy::PrayerForm;
use office::{Day, Engine, resolve_office_days};
use tools::fs::FsData;

fn parse_year(args: &[String], command: &str) -> Result<i32, String> {
    let arg = args.first().ok_or_else(|| format!("usage: office {command} YEAR"))?;
    compat::atoi(arg).ok().and_then(|y| i32::try_from(y).ok()).ok_or_else(|| format!("invalid year: {arg}"))
}

/// The calendar days (with their Office resolution) and the engine.
fn build_year(data: &FsData, year: i32) -> Result<(Vec<Day>, MoveableDates, Engine), String> {
    let cal_data = CalendarData::load(data).map_err(|e| format!("building calendar: {e}"))?;
    let cal = build_calendar(year, &cal_data).map_err(|e| format!("building calendar: {e}"))?;
    let engine = Engine::load(data).map_err(|e| format!("creating office engine: {e}"))?;
    let office = resolve_office_days(&cal);
    let days = cal.days.into_iter().zip(office).map(|(c, o)| Day::new(c, o)).collect();
    Ok((days, MoveableDates::compute(year), engine))
}

pub fn cmd_ordo(data: &FsData, args: &[String], out: &mut dyn Write) -> Result<(), String> {
    let year = parse_year(args, "ordo")?;
    let (days, moveable, engine) = build_year(data, year)?;
    write!(out, "{}", ordo::format_calendar(&days, Some(&engine), &moveable)).map_err(|e| e.to_string())
}

pub fn cmd_rubrics(data: &FsData, args: &[String], out: &mut dyn Write) -> Result<(), String> {
    let year = parse_year(args, "rubrics")?;
    let (days, moveable, engine) = build_year(data, year)?;
    write!(out, "{}", ordo::rubrics_tsv(&days, &engine, &moveable)?).map_err(|e| e.to_string())
}

/// Accepts `--form X` or `--form=X` anywhere among the arguments.
pub fn take_prayer_form(args: &[String]) -> Result<(Vec<String>, PrayerForm), String> {
    let mut form = PrayerForm::Private;
    let mut rest = Vec::new();
    let mut seen = false;
    let mut i = 0;
    while i < args.len() {
        let arg = &args[i];
        if arg != "--form" && !arg.starts_with("--form=") {
            rest.push(arg.clone());
            i += 1;
            continue;
        }
        if seen {
            return Err("--form may be supplied only once".to_string());
        }
        seen = true;
        let value = if arg == "--form" {
            i += 1;
            args.get(i).cloned().ok_or("--form requires private, deacon, or priest")?
        } else {
            arg["--form=".len()..].to_string()
        };
        if value.is_empty() {
            return Err("--form requires private, deacon, or priest".to_string());
        }
        form = PrayerForm::parse(&value)?;
        i += 1;
    }
    Ok((rest, form))
}

/// Prints one composed hour as plain text.
pub fn cmd_hour(hour_name: &str, data: &FsData, args: &[String], out: &mut dyn Write) -> Result<(), String> {
    let (args, form) = take_prayer_form(args)?;
    let arg = args.first().ok_or_else(|| format!("usage: office {hour_name} YYYY-MM-DD [--form private|deacon|priest]"))?;
    let date = Date::parse(arg).ok_or_else(|| format!("invalid date (use YYYY-MM-DD): {arg}"))?;
    let (days, moveable, engine) = build_year(data, date.year()).map_err(|e| format!("composing {hour_name}: {e}"))?;
    let day = days.get(date.ordinal() as usize - 1).ok_or_else(|| format!("composing {hour_name}: date out of range"))?;
    let hour = engine.compose_hour(hour_name, day, &moveable, form).map_err(|e| format!("composing {hour_name}: {e}"))?;
    write!(out, "{}", render_text::format_office_hour(&hour)).map_err(|e| e.to_string())
}

const TEX_USAGE: &str = "usage: office tex [--chant] [--form private|deacon|priest] HOUR [YYYY-MM-DD]\n\
Example: office tex lauds 2026-03-11 > lauds.tex\n         office tex --chant compline > compline.tex";

/// `tex [--chant] [--form F] HOUR [DATE]`: the LuaLaTeX booklet of one hour.
pub fn cmd_tex(data: &FsData, args: &[String], out: &mut dyn Write) -> Result<(), String> {
    let (args, form) = take_prayer_form(args)?;
    let chant = args.iter().any(|a| a == "--chant");
    let positional: Vec<&String> = args.iter().filter(|a| *a != "--chant").collect();
    let hour_name = positional.first().ok_or_else(|| TEX_USAGE.to_string())?.as_str();
    let date = match positional.get(1) {
        Some(arg) => Date::parse(arg).ok_or_else(|| format!("invalid date (use YYYY-MM-DD): {arg}"))?,
        None => today(),
    };
    let (days, moveable, engine) = build_year(data, date.year()).map_err(|e| format!("composing {hour_name}: {e}"))?;
    let day = days.get(date.ordinal() as usize - 1).ok_or_else(|| format!("composing {hour_name}: date out of range"))?;
    let hour = engine.compose_hour(hour_name, day, &moveable, form).map_err(|e| format!("composing {hour_name}: {e}"))?;

    // \gregorioscore paths must be relative to the working directory:
    // kpathsea blocks writes to absolute paths outside the TeX run directory.
    let mut data_dir = data.dir.clone();
    if chant
        && data_dir.is_absolute()
        && let Ok(cwd) = std::env::current_dir()
    {
        data_dir = tools::fs::rel(&cwd, &data_dir);
    }
    let base = |category: &str, slug: &str| {
        if data_dir.as_os_str().is_empty() {
            return None;
        }
        let base = tools::fs::clean(&data_dir.join("texts").join("chant").join(category).join(slug));
        let mut gabc = base.clone().into_os_string();
        gabc.push(".gabc");
        std::path::Path::new(&gabc).exists().then(|| base.display().to_string())
    };
    let lookup = render_tex::Chant { base: &base };
    write!(out, "{}", render_tex::format_office_hour_tex(&hour, chant.then_some(&lookup))).map_err(|e| e.to_string())
}

/// Today's civil date. PORT(inherited): Go's `time.Now()` is local time; this
/// is UTC, which only differs when no date is given near midnight.
pub fn today() -> Date {
    let secs = std::time::SystemTime::now().duration_since(std::time::UNIX_EPOCH).map_or(0, |d| d.as_secs());
    Date::new(1970, 1, 1).add_days((secs / 86_400) as i32)
}
