//! The text ordo (Tabula Temporaria plus a stanza per day) and the rubrics
//! TSV the ordo cross-checks read. Ported from Go's `output/ordo.go` and
//! `cli/ordo.go`.

use calendar::{Date, Feast, MoveableDates, Rank, Tabula, Weekday};
use liturgy::PrayerForm;
use office::summary::{CommSummary, HourSummary, summarize_hour};
use office::{Day, Engine, VespersOwner};

const MONTHS: [&str; 12] =
    ["January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"];

/// "January 2".
pub fn month_day(d: Date) -> String {
    format!("{} {}", MONTHS[d.month() as usize - 1], d.day())
}

fn day_abbrev(wd: Weekday) -> &'static str {
    match wd {
        Weekday::Monday => "Mon",
        Weekday::Tuesday => "Tue",
        Weekday::Wednesday => "Wed",
        Weekday::Thursday => "Thu",
        Weekday::Friday => "Fri",
        Weekday::Saturday => "Sat",
        Weekday::Sunday => "Sun",
    }
}

/// Go's `strings.ToUpper`: rune by rune, keeping runes with no single-rune
/// uppercase.
fn go_upper(s: &str) -> String {
    s.chars()
        .map(|c| {
            let mut up = c.to_uppercase();
            match (up.next(), up.next()) {
                (Some(u), None) => u,
                _ => c,
            }
        })
        .collect()
}

/// Go's `%-Ns`: left-justified, padded to `width` runes.
fn pad(s: &str, width: usize) -> String {
    let n = s.chars().count();
    if n >= width { s.to_string() } else { format!("{s}{}", " ".repeat(width - n)) }
}

/// Presentation rank: the printed ordos rank days 4–7 of the Easter and
/// Pentecost octaves Sd despite their privileged precedence.
fn ordo_display_rank(feast: &Feast) -> Rank {
    for prefix in ["easter-sunday-octave-day-", "pentecost-octave-day-"] {
        if let Some(after) = feast.id.strip_prefix(prefix)
            && matches!(after, "4" | "5" | "6" | "7")
        {
            return Rank::SemiDouble;
        }
    }
    feast.rank
}

fn ordo_subtitle(feast: Option<&Feast>) -> Option<&'static str> {
    feast.filter(|f| f.id == "pentecost-sunday-2").map(|_| "II Sunday after Pentecost")
}

fn preces_label(b: bool) -> &'static str {
    if b { "Preces" } else { "No Preces" }
}

fn suffrage_label(b: bool) -> &'static str {
    if b { "Suff." } else { "No Suff." }
}

fn join_bar(parts: &[&str]) -> String {
    parts.iter().filter(|p| !p.is_empty()).copied().collect::<Vec<_>>().join(" · ")
}

fn color_abbrev(c: Option<calendar::Color>) -> &'static str {
    c.map_or("?", |c| c.abbrev())
}

const INDENT: &str = "           ";

/// One day's ordo stanza. Without an engine only the header lines are
/// written.
pub fn format_day(day: &Day, engine: Option<&Engine>, moveable: &MoveableDates) -> String {
    let dow = day_abbrev(day.date.weekday());
    let day_num = format!("{:>3}", day.date.day());
    let marker = pad(&day.penitential.marker(), 2);
    let color = day.color.abbrev();
    let line = match day.celebration.as_deref() {
        None => {
            let name = day.tempora.as_deref().unwrap_or("Feria");
            format!("{day_num}  {dow}  {marker} {}       {color}", pad(name, 42))
        }
        Some(feast) => {
            let rank = ordo_display_rank(feast);
            let name = if rank == Rank::Double1stClass { go_upper(&feast.name) } else { feast.name.clone() };
            format!("{day_num}  {dow}  {marker} {} {} {color}", pad(&name, 42), pad(&format!("[{}]", rank.abbrev()), 5))
        }
    };
    let mut lines = vec![line];
    if let Some(subtitle) = ordo_subtitle(day.celebration.as_deref()) {
        lines.push(format!("{INDENT}{subtitle}"));
    }
    for comm in &day.commemorations {
        lines.push(format!("{INDENT}{} ({})", comm.commemoration_name(), comm.rank.abbrev()));
    }

    let compose = |hour_name: &str| -> Option<HourSummary> {
        let engine = engine?;
        engine.compose_hour(hour_name, day, moveable, PrayerForm::Private).ok().map(|h| summarize_hour(&h))
    };
    let comm_lines = |comms: &[CommSummary]| -> Vec<String> {
        comms
            .iter()
            .map(|c| {
                if c.incipit.is_empty() {
                    format!("{INDENT}    Com. {}", c.name)
                } else {
                    format!("{INDENT}    Com. {} ({})", c.name, compat::quote(&c.incipit))
                }
            })
            .collect()
    };

    if let Some(lauds) = compose("lauds") {
        let ben = if lauds.gospel_ant.is_empty() { String::new() } else { format!("Ben. {}", lauds.gospel_ant) };
        lines.push(format!(
            "{INDENT}Lauds   {}",
            join_bar(&[color_abbrev(lauds.color), &ben, preces_label(lauds.preces), suffrage_label(lauds.suffrage)])
        ));
        lines.extend(comm_lines(&lauds.comms));
    }
    if let Some(hours) = compose("prime") {
        lines.push(format!("{INDENT}Hours   {}", preces_label(hours.preces)));
    }
    let v = &day.vespers;
    let feast_name = || v.feast.as_deref().map_or("", |f| f.name.as_str()).to_string();
    if let Some(vespers) = compose("vespers") {
        let owner = match v.owner {
            VespersOwner::IIOfPreceding => "II prec.".to_string(),
            VespersOwner::IOfFollowing => format!("I fol. ({})", feast_name()),
            VespersOwner::NotApplicable => String::new(),
        };
        let mag = if vespers.gospel_ant.is_empty() { String::new() } else { format!("Mag. {}", vespers.gospel_ant) };
        lines.push(format!("{INDENT}Vespers {}", join_bar(&[color_abbrev(vespers.color), &owner, &mag, suffrage_label(vespers.suffrage)])));
        lines.extend(comm_lines(&vespers.comms));
        if v.appended_office_of_the_dead {
            if day.celebration_is("all-saints") {
                lines.push(format!("{INDENT}Vespers of the Dead after Let us bless the Lord"));
            } else {
                lines.push(format!("{INDENT}Vespers of the Dead (optional) after Let us bless the Lord"));
            }
            lines.push(format!("{INDENT}Compline of the Dead"));
        }
    } else {
        match v.owner {
            VespersOwner::IIOfPreceding => lines.push(format!("{INDENT}Vespers: II prec. {}", color_abbrev(v.color))),
            VespersOwner::IOfFollowing => lines.push(format!("{INDENT}Vespers: I fol. {} ({})", color_abbrev(v.color), feast_name())),
            VespersOwner::NotApplicable => {}
        }
    }
    lines.join("\n")
}

/// A year's text ordo: the Tabula, then each month's days.
pub fn format_calendar(days: &[Day], engine: Option<&Engine>, moveable: &MoveableDates) -> String {
    let mut lines = Vec::new();
    if let Some(first) = days.first() {
        lines.push(format_tabula(first.date.year(), moveable));
    }
    let mut current_month = 0;
    for day in days {
        if day.date.month() != current_month {
            current_month = day.date.month();
            if !lines.is_empty() {
                lines.push(String::new());
            }
            lines.push(MONTHS[current_month as usize - 1].to_uppercase());
            lines.push("-".repeat(60));
        }
        lines.push(format_day(day, engine, moveable));
    }
    lines.join("\n") + "\n"
}

/// The Tabula Temporaria block.
pub fn format_tabula(year: i32, moveable: &MoveableDates) -> String {
    let t = Tabula::compute(year);
    let ember = |e: &calendar::computus::EmberSet| format!("{}, {}, {}", month_day(e.wed), e.fri.day(), e.sat.day());
    let mut b = String::new();
    b.push_str(&format!("TABULA TEMPORARIA — ANNO DOMINI {year}\n"));
    b.push_str(&"=".repeat(60));
    b.push('\n');
    let row = |label: &str, value: String| format!("  {} {value}\n", pad(label, 28));
    b.push_str(&row("Golden Number", calendar::computus::roman(t.golden_number)));
    b.push_str(&row("Dominical Letter", t.dominical_letter.to_string()));
    b.push_str(&row("Sundays after Epiphany", t.sundays_after_epiphany.to_string()));
    b.push_str(&row("Sundays after Pentecost", t.sundays_after_pentecost.to_string()));
    b.push('\n');
    b.push_str("  MOVEABLE FEASTS\n");
    let row4 = |label: &str, value: String| format!("    {} {value}\n", pad(label, 24));
    for (label, d) in [
        ("Septuagesima Sunday", moveable.septuagesima),
        ("Ash Wednesday", moveable.ash_wednesday),
        ("Easter (Pascha) Day", moveable.easter),
        ("Ascension Day", moveable.ascension),
        ("Pentecost", moveable.pentecost),
        ("Corpus Christi", moveable.corpus_christi),
        ("Advent Sunday", moveable.advent1),
    ] {
        b.push_str(&row4(label, month_day(d)));
    }
    b.push('\n');
    b.push_str("  EMBER DAYS\n");
    for (label, e) in
        [("Spring (Lent)", &t.spring), ("Summer (Whitsun)", &t.summer), ("Autumn (Holy Cross)", &t.autumn), ("Winter (Advent)", &t.winter)]
    {
        b.push_str(&row4(label, ember(e)));
    }
    b
}

/// The rubrics TSV: per-day composed flags for the ordo cross-check. Column
/// order and spelling are a contract with `scripts/ordo-compare.py`.
pub fn rubrics_tsv(days: &[Day], engine: &Engine, moveable: &MoveableDates) -> Result<String, String> {
    let mut out = String::from(
        "date\tcelebration\tlauds_preces\tlauds_suffrage\tlauds_comms\thours_preces\tvespers_preces\tvespers_suffrage\tvespers_comms\tbenedictus_ant\tmagnificat_ant\n",
    );
    let names = |comms: &[CommSummary]| comms.iter().map(|c| c.name.as_str()).collect::<Vec<_>>().join("; ");
    for day in days {
        let celebration = day.celebration.as_deref().map_or("Feria", |c| c.name.as_str());
        let summarize = |hour_name: &str| -> Result<HourSummary, String> {
            let h = engine
                .compose_hour(hour_name, day, moveable, PrayerForm::Private)
                .map_err(|e| format!("composing {hour_name} for {}: {e}", day.date))?;
            Ok(summarize_hour(&h))
        };
        let lauds = summarize("lauds")?;
        let prime = summarize("prime")?;
        let vespers = summarize("vespers")?;
        let row = [
            day.date.to_string(),
            celebration.to_string(),
            lauds.preces.to_string(),
            lauds.suffrage.to_string(),
            names(&lauds.comms),
            prime.preces.to_string(),
            vespers.preces.to_string(),
            vespers.suffrage.to_string(),
            names(&vespers.comms),
            lauds.gospel_ant_full,
            vespers.gospel_ant_full,
        ];
        out.push_str(&row.join("\t"));
        out.push('\n');
    }
    Ok(out)
}
