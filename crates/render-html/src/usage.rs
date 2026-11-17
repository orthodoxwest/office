//! The usage report's view model.

use std::collections::BTreeMap;

use calendar::Date;
use data_format::json::{Json, Obj};
use presentation::usage::week_start;
use serde::Serialize;

use crate::escape::format_float;
use crate::view::Chrome;

pub use presentation::usage::{Dimension, HOURS};

/// One reporting day's counts, newest first in a window.
#[derive(Clone, Debug, Default, PartialEq, Eq, Serialize)]
pub struct UsageDay {
    pub day: String,
    pub users: i64,
    pub hours: [i64; 7],
    pub ordo: i64,
    pub reminders: i64,
    /// Readers counted for the first time that day, filled by the view model: none on a day
    /// before new and returning readers were told apart.
    pub new_readers: Option<i64>,
    /// A day test devices swamped ([`Corrections::test_days`]), and why: shown, but left out of
    /// every average, total and scale.
    pub excluded: bool,
    pub note: String,
    /// Display date and short weekday, filled by the view model.
    pub label: String,
    pub weekday: String,
    /// A Monday: newest first, the week before begins under this row.
    pub week_start: bool,
    /// Counts by qualified dimension scope ("appearance:apse").
    #[serde(skip)]
    pub dimensions: BTreeMap<String, i64>,
}

/// Counts the report knows were not readers.
#[derive(Clone, Copy, Debug, Default)]
pub struct Corrections<'a> {
    /// Days store test devices swamped, and why, as (day, reason).
    pub test_days: &'a [(&'a str, &'a str)],
    /// A client's first day in readers' hands, as (client, day, what counted before it): every
    /// earlier count of that client was a test device, and is taken out of the day's readers.
    pub released: &'a [(&'a str, &'a str, &'a str)],
}

/// A row of a horizontal bar list.
#[derive(Clone, Debug, Default, Serialize)]
pub struct UsageShare {
    pub name: String,
    /// The figure beside the bar ("33%", "23").
    pub value: String,
    /// A second, quieter figure (a count, or what the share is of).
    pub detail: String,
    /// The bar, 0–100.
    pub width: String,
}

/// A client's row in "Where they read".
#[derive(Clone, Debug, Default, Serialize)]
pub struct UsageClient {
    pub name: String,
    /// Readers a day over the last seven completed days, and the seven before.
    pub now: String,
    pub before: String,
    /// "up", "down" or "" against the week before.
    pub direction: String,
    pub width: String,
    /// Where the week before falls on the bar, 0–100; empty without one.
    pub mark: String,
    /// New readers in the week.
    pub new: i64,
}

/// An hour's row in "How they keep the hours": of the readers whose client says so, the share who
/// opened it at its own time, and who prayed it to the end.
#[derive(Clone, Debug, Default, Serialize)]
pub struct UsageKeep {
    pub name: String,
    /// Readers whose client reports, who opened the hour.
    pub reported: i64,
    pub timely: String,
    pub prayed: String,
}

/// A summary figure.
#[derive(Clone, Debug, Default, Serialize)]
pub struct UsageTile {
    pub label: String,
    pub value: String,
    pub note: String,
    /// Small bars under the figure, oldest first: heights 0–100.
    pub bars: Vec<String>,
}

#[derive(Clone, Debug, Default, Serialize)]
pub struct UsageBar {
    pub day: String,
    /// The bar's spoken value ("Nov 14: 76 readers, 36 new").
    pub title: String,
    pub x: String,
    pub width: String,
    /// The whole bar, and the part of it that is new readers (zero height when not told apart).
    pub y: String,
    pub height: String,
    pub new_height: String,
    /// Whether new and returning readers were told apart that day.
    pub split: bool,
    pub today: bool,
    pub excluded: bool,
}

/// A gridline and its label, or a date under the axis: `at` is 0–100 along its axis.
#[derive(Clone, Debug, Default, Serialize)]
pub struct UsageTick {
    pub at: String,
    pub label: String,
}

/// A reporting window's tab.
#[derive(Clone, Debug, Default, Serialize)]
pub struct UsageTab {
    pub days: i64,
    pub label: String,
    pub current: bool,
}

#[derive(Clone, Debug, Default, Serialize)]
pub struct TrendPoint {
    pub day: String,
    pub counts: Vec<i64>,
}

#[derive(Clone, Debug, Default, Serialize)]
pub struct TrendGroup {
    pub key: String,
    pub label: String,
    pub series: Vec<String>,
    pub points: Vec<TrendPoint>,
}

#[derive(Clone, Debug, Default, Serialize)]
pub struct UsageData {
    #[serde(flatten)]
    pub chrome: Chrome,
    pub trend_groups: Vec<TrendGroup>,
    /// The trend groups encoded into the page.
    pub trend_json: String,
    pub days: i64,
    pub tabs: Vec<UsageTab>,
    /// Days in the window since records began; fewer than `days` when the
    /// window reaches back before collection.
    pub recorded_days: i64,
    pub first_date: String,
    pub last_date: String,
    /// The report in a sentence: its figure, the rest of the sentence, and a second sentence on
    /// returning and new readers.
    pub headline_figure: String,
    pub headline_rest: String,
    pub headline_detail: String,
    pub tiles: Vec<UsageTile>,
    /// Readers in a calendar week and month, once any client reports them.
    pub period_tiles: Vec<UsageTile>,
    pub period_note: String,
    /// The highest daily count the chart is scaled to.
    pub max: i64,
    /// No reader on any day of the window, a day set aside included.
    pub empty: bool,
    pub chart: Vec<UsageBar>,
    pub ticks: Vec<UsageTick>,
    /// The ticks' gridlines, as an SVG path in the chart's coordinates.
    pub grid_path: String,
    pub dates: Vec<UsageTick>,
    /// The trailing seven-day average across the chart, as an SVG path in its
    /// coordinates; empty when fewer than two days have one.
    pub rolling_path: String,
    /// Whether any bar tells new readers from returning, or stands outside the scale.
    pub chart_split: bool,
    pub chart_plain: bool,
    pub chart_excluded: bool,
    pub clients: Vec<UsageClient>,
    /// The hours, then the ordo and reminders, as shares of the window's readers.
    pub opens: Vec<UsageShare>,
    pub opens_extra: Vec<UsageShare>,
    /// Readers in the window, the shares' denominator.
    pub readers: i64,
    /// How many readers opened an hour, and how many hours; empty until the server counts it.
    pub offices_note: String,
    pub settings: Vec<UsageShare>,
    /// Each hour opened at its time and prayed, by readers whose client says so.
    pub keeping: Vec<UsageKeep>,
    /// The readers' habits, as shares: how long they have read, last week's days, a new reader's
    /// first day, an hour that could not report, reminders in the apps.
    pub habits: Vec<UsageShare>,
    /// The share of readers whose client reports these.
    pub habits_note: String,
    pub hours: Vec<String>,
    /// The newest days, then the rest of the window.
    pub rows: Vec<UsageDay>,
    pub more_rows: Vec<UsageDay>,
    /// What the report left out, in sentences.
    pub corrections: Vec<String>,
}

const MONTHS: [&str; 12] = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];

const WEEKDAYS: [&str; 7] = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];

const WEEKDAY_NAMES: [&str; 7] = ["Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"];

/// Days before a window that its summary reads: four weeks for the weekly averages, and back to
/// the start of last month for the month's readers.
pub const LOOKBACK: usize = 62;

/// The windows offered, in days.
pub const WINDOWS: [i64; 4] = [7, 30, 90, 365];

/// The room a date under the chart takes, as a share of the plot's width on a phone.
const DATE_GAP: f64 = 14.0;
const TODAY_GAP: f64 = 11.0;

/// Rows of the daily table shown before the rest are folded away.
const TABLE_ROWS: usize = 14;

/// A week-on-week change smaller than this is not told from a level week: at a few dozen readers
/// a day, two weeks' averages differ by a tenth or more by chance.
const LEVEL: f64 = 0.2;

fn whole(n: f64) -> String {
    if n > 0.0 && n < 0.5 { "<1".into() } else { format!("{}", n.round() as i64) }
}

fn percent(part: i64, of: i64) -> String {
    if of > 0 { format!("{}%", (100.0 * part as f64 / of as f64).round() as i64) } else { "—".into() }
}

fn dim(row: &UsageDay, scope: &str) -> i64 {
    row.dimensions.get(scope).copied().unwrap_or(0)
}

fn family(row: &UsageDay, key: &str) -> i64 {
    row.dimensions.iter().filter(|(k, _)| k.strip_prefix(key).is_some_and(|r| r.starts_with(':'))).map(|(_, v)| v).sum()
}

fn date_label(day: &str, year: &str) -> String {
    match Date::parse(day) {
        None => day.to_string(),
        Some(t) if &day[..4] != year => format!("{} {}, {}", MONTHS[t.month() as usize - 1], t.day(), t.year()),
        Some(t) => format!("{} {}", MONTHS[t.month() as usize - 1], t.day()),
    }
}

/// The mean of `f` over the counted days of `rows`; none when no day counts.
fn mean(rows: &[UsageDay], f: impl Fn(&UsageDay) -> Option<i64>) -> Option<f64> {
    let values: Vec<i64> = rows.iter().filter(|r| !r.excluded).filter_map(f).collect();
    (!values.is_empty()).then(|| values.iter().sum::<i64>() as f64 / values.len() as f64)
}

fn total(rows: &[UsageDay], f: impl Fn(&UsageDay) -> i64) -> i64 {
    rows.iter().filter(|r| !r.excluded).map(f).sum()
}

/// Takes out the counts [`Corrections`] names and marks the days it sets aside.
fn correct(rows: &mut [UsageDay], corrections: &Corrections) {
    for row in rows {
        for (client, since, _) in corrections.released {
            if row.day.as_str() >= *since {
                continue;
            }
            let all = row.dimensions.remove(&format!("client:{client}")).unwrap_or(0);
            let new = row.dimensions.remove(&format!("new-client:{client}")).unwrap_or(0).min(all);
            row.users = (row.users - all).max(0);
            for (scope, n) in [("visit:first", new), ("visit:returning", all - new)] {
                if let Some(v) = row.dimensions.get_mut(scope) {
                    *v = (*v - n).max(0);
                }
            }
        }
        if let Some((_, why)) = corrections.test_days.iter().find(|(day, _)| *day == row.day) {
            row.excluded = true;
            row.note = why.to_string();
        }
    }
}

/// A scale's top and step for counts up to `max`: at most four round steps.
fn scale(max: i64) -> (i64, i64) {
    let mut unit = 1;
    loop {
        for step in [unit, 2 * unit, 5 * unit] {
            if step * 4 >= max {
                return ((max.max(1) + step - 1) / step * step, step);
            }
        }
        unit *= 10;
    }
}

/// Builds the report from `rows`, newest first: the window's `days`, then up to [`LOOKBACK`] older
/// days that feed only the summary. Days before `since`, the first recorded day, predate
/// collection: they are dropped rather than counted as quiet days, and today always remains.
pub fn usage_data(
    mut rows: Vec<UsageDay>,
    days: i64,
    since: Option<&str>,
    dimensions: &[Dimension],
    corrections: &Corrections,
) -> UsageData {
    let recorded = rows.iter().take_while(|r| since.is_some_and(|s| r.day.as_str() >= s)).count();
    rows.truncate(recorded.max(1).min(rows.len()));
    let mut d = UsageData {
        chrome: Chrome { page: "usage".into(), ..Chrome::default() },
        days,
        hours: HOURS.iter().map(|h| h.to_string()).collect(),
        ..UsageData::default()
    };
    if rows.is_empty() {
        d.headline_rest = "No visits recorded yet.".into();
        d.trend_groups = trend_groups(&rows, dimensions);
        d.trend_json = trend_json(&d.trend_groups);
        return d;
    }
    correct(&mut rows, corrections);
    // A year-long window can reach into last year.
    let year = rows[0].day[..4].to_string();
    let label = |day: &str| date_label(day, &year);
    // New and returning readers are told apart from the first day either was counted.
    let told = rows.iter().rposition(|r| family(r, "visit") > 0);
    for (i, row) in rows.iter_mut().enumerate() {
        row.new_readers = told.filter(|&t| i <= t).map(|_| dim(row, "visit:first"));
        row.label = label(&row.day);
        if let Some(t) = Date::parse(&row.day) {
            row.weekday = WEEKDAYS[t.weekday().number() as usize].to_string();
            row.week_start = t.weekday().number() == 1;
        }
    }
    let all = rows;
    let window = &all[..all.len().min(days.max(1) as usize)];
    d.recorded_days = window.len() as i64;
    d.last_date = label(&window[0].day);
    d.first_date = label(&window[window.len() - 1].day);
    d.tabs = tabs(days, since, &all[0].day, &year);
    d.trend_groups = trend_groups(window, dimensions);
    d.trend_json = trend_json(&d.trend_groups);
    summary(&mut d, &all);
    periods(&mut d, &all, &year);
    chart(&mut d, &all, window);
    d.clients = clients(&all);
    breakdowns(&mut d, &all, window, &year);
    habits(&mut d, &all, window);
    for (day, why) in corrections.test_days {
        if window.iter().any(|r| r.day == *day) {
            d.corrections.push(format!("{}: {why}. The day is shown, but left out of every average, share and scale.", label(day)));
        }
    }
    for (client, day, why) in corrections.released {
        if window.iter().any(|r| r.day.as_str() < *day) {
            let name = CLIENT_NAMES.iter().find(|(key, ..)| key == client).map_or(*client, |c| c.1);
            d.corrections.push(format!(
                "{name} before {}: {why}. Those counts are left out of readers, new readers and the clients. The hours they opened and the settings they reported cannot be told from readers' and remain.",
                label(day)
            ));
        }
    }
    let mut table = window.to_vec();
    d.more_rows = table.split_off(table.len().min(TABLE_ROWS));
    d.rows = table;
    d
}

/// The window tabs: each window the record fills, then one for the whole record.
fn tabs(days: i64, since: Option<&str>, today: &str, year: &str) -> Vec<UsageTab> {
    let recorded = match (since.and_then(Date::parse), Date::parse(today)) {
        (Some(first), Some(today)) => i64::from(today.days_since(first)) + 1,
        _ => 0,
    };
    let whole = WINDOWS.iter().copied().find(|&n| n >= recorded);
    WINDOWS
        .iter()
        .filter(|&&n| n < recorded || Some(n) == whole)
        .map(|&n| UsageTab {
            days: n,
            label: match since {
                Some(first) if Some(n) == whole => format!("Since {}", date_label(first, year)),
                _ => format!("{n} days"),
            },
            current: n == days || (Some(n) == whole && days > n),
        })
        .collect()
}

/// The sentence and the four figures: the last seven completed days against the weeks before.
fn summary(d: &mut UsageData, all: &[UsageDay]) {
    let completed = &all[1..];
    let week = &completed[..completed.len().min(7)];
    let before = completed.get(7..14);
    let users = |r: &UsageDay| Some(r.users);
    let returning = |r: &UsageDay| r.new_readers.map(|_| dim(r, "visit:returning"));
    let today = UsageTile {
        label: "Today so far".into(),
        value: all[0].users.to_string(),
        note: {
            let mut parts = Vec::new();
            if let Some(y) = all.get(1).filter(|r| !r.excluded) {
                parts.push(format!("Yesterday {}", y.users));
            }
            if let (Some(w), Some(t)) = (all.get(7).filter(|r| !r.excluded), Date::parse(&all[0].day)) {
                parts.push(format!("last {} {}", WEEKDAY_NAMES[t.weekday().number() as usize], w.users));
            }
            parts.join(" · ")
        },
        bars: Vec::new(),
    };
    let Some(now) = mean(week, users) else {
        d.headline_rest = "No completed days yet.".into();
        d.tiles = vec![today];
        return;
    };
    d.headline_figure = format!("{} readers a day", whole(now));
    d.headline_rest = match before.and_then(|b| mean(b, users)) {
        _ if week.len() < 7 => format!(" over {} completed day{}.", week.len(), if week.len() == 1 { "" } else { "s" }),
        None => " last week.".into(),
        Some(b) if whole(now) == whole(b) || (b > 0.0 && ((now - b) / b).abs() < LEVEL) => {
            format!(" last week, about level with the week before ({}).", whole(b))
        }
        Some(b) if now > b => format!(" last week, up from {} the week before.", whole(b)),
        Some(b) => format!(" last week, down from {} the week before.", whole(b)),
    };
    // Up to four weeks, oldest first, for the first figure's small bars.
    let weeks: Vec<f64> = (0..4).rev().filter_map(|k| completed.get(7 * k..7 * k + 7).and_then(|w| mean(w, users))).collect();
    let peak = weeks.iter().copied().fold(1.0, f64::max);
    let mut first = UsageTile { label: "A day, last 7 days".into(), value: whole(now), ..UsageTile::default() };
    if week.len() < 7 {
        first.label = "A day".into();
        first.note = format!("Over {} completed day{}", week.len(), if week.len() == 1 { "" } else { "s" });
    } else if weeks.len() > 1 {
        first.note = format!("{} weeks: {}", weeks.len(), weeks.iter().map(|&w| whole(w)).collect::<Vec<_>>().join(" · "));
        first.bars = weeks.iter().map(|w| format_float((100.0 * w / peak).round())).collect();
    }
    let mut tiles = vec![first];
    let told = week.iter().all(|r| r.new_readers.is_some()) && week.len() == 7;
    let before_told = before.filter(|b| b.iter().all(|r| r.new_readers.is_some()));
    if told {
        let new = total(week, |r| r.new_readers.unwrap_or(0));
        let back = mean(week, returning).unwrap_or(0.0);
        let back_before = before_told.and_then(|b| mean(b, returning));
        let new_before = before_told.map(|b| total(b, |r| r.new_readers.unwrap_or(0)));
        tiles.push(UsageTile {
            label: "Returning, a day".into(),
            value: whole(back),
            note: back_before.map_or_else(|| "Counted before that day".into(), |b| format!("{} the week before", whole(b))),
            bars: Vec::new(),
        });
        tiles.push(UsageTile {
            label: "New, in 7 days".into(),
            value: new.to_string(),
            note: new_before.map_or_else(|| "First counted in the week".into(), |b| format!("{b} the week before")),
            bars: Vec::new(),
        });
        let by_client: Vec<String> = CLIENT_NAMES
            .iter()
            .map(|(key, _, phrase)| (total(week, |r| dim(r, &format!("new-client:{key}"))), phrase))
            .filter(|(n, _)| *n > 0)
            .map(|(n, phrase)| format!("{n} {phrase}"))
            .collect();
        d.headline_detail = format!(
            "Of those, {} a day were returning{}. {new} were new in the week{}.{}",
            whole(back),
            back_before.map_or(String::new(), |b| format!(" ({} the week before)", whole(b))),
            new_before.map_or(String::new(), |b| format!(" ({b} before)")),
            // A reader new to one client may have been counted on another, so these can pass the total.
            if by_client.is_empty() { String::new() } else { format!(" New to each client: {}.", by_client.join(", ")) },
        );
    }
    tiles.push(today);
    d.tiles = tiles;
}

/// Readers in a calendar week and month: the sum of each day's `week:first`, and `month:first`.
/// The last completed period leads once every day of it was reported; until then, the period
/// in progress.
fn periods(d: &mut UsageData, all: &[UsageDay], year: &str) {
    let Some(today) = Date::parse(&all[0].day) else { return };
    let reported = |r: &&UsageDay, key: &str| family(r, key) > 0;
    let mut partial = false;
    for (key, noun, this_start, last_start) in [
        ("week", "week", week_start(today), week_start(today).add_days(-7)),
        ("month", "month", Date::new(today.year(), today.month() as i32, 1), {
            let end = Date::new(today.year(), today.month() as i32, 1).add_days(-1);
            Date::new(end.year(), end.month() as i32, 1)
        }),
    ] {
        let first = format!("{key}:first");
        let span = |from: Date, to: Date| -> Vec<&UsageDay> {
            all.iter().filter(|r| Date::parse(&r.day).is_some_and(|t| t.days_since(from) >= 0 && to.days_since(t) > 0)).collect()
        };
        // Newest first: the period in progress runs from its first reported day.
        let mut this = span(this_start, today.add_days(1));
        this.truncate(this.iter().rposition(|r| reported(r, key)).map_or(0, |i| i + 1));
        // A day with no reader had nothing to report: the period is whole without it.
        let began_reading = all.iter().filter(|r| r.users > 0).filter_map(|r| Date::parse(&r.day)).rfind(|t| t.days_since(this_start) >= 0);
        let last = span(last_start, this_start);
        let last_whole = last.len() as i32 == this_start.days_since(last_start) && last.iter().all(|r| reported(r, key) || r.users == 0);
        let sum = |rows: &[&UsageDay]| rows.iter().map(|r| dim(r, &first)).sum::<i64>();
        let range = |rows: &[&UsageDay]| match (rows.last(), rows.first()) {
            (Some(a), Some(b)) if a.day != b.day => format!("{} – {}", date_label(&a.day, year), date_label(&b.day, year)),
            (Some(a), _) => date_label(&a.day, year),
            _ => String::new(),
        };
        // The share of the counted days' readers whose client said either word.
        let coverage = |rows: &[&UsageDay]| {
            let (said, of) = rows.iter().fold((0, 0), |(said, of), r| (said + family(r, key).min(r.users), of + r.users));
            (of > 0 && said * 20 < of * 19).then(|| format!("{} of its readers reporting", percent(said, of)))
        };
        let so_far = (!this.is_empty()).then(|| sum(&this));
        let (mut tile, counted): (UsageTile, Option<String>) = if last_whole && last.iter().any(|r| reported(r, key)) {
            let tile = UsageTile {
                label: format!("Readers, last {noun}"),
                value: sum(&last).to_string(),
                note: [Some(range(&last)), coverage(&last), so_far.map(|n| format!("this {noun} so far {n}"))]
                    .into_iter()
                    .flatten()
                    .collect::<Vec<_>>()
                    .join(" · "),
                bars: Vec::new(),
            };
            partial |= coverage(&last).is_some();
            (tile, None)
        } else if let (Some(n), Some(began)) = (so_far, this.last()) {
            // A period whose first days went unreported is counted from the day reports began.
            let whole = Date::parse(&began.day) == Some(this_start) || Date::parse(&began.day) == began_reading;
            let tile = UsageTile {
                label: if whole {
                    format!("Readers, this {noun} so far")
                } else {
                    format!("Readers since {}", date_label(&began.day, year))
                },
                value: n.to_string(),
                note: if whole { range(&this) } else { format!("Not the whole {noun}: {}", range(&this)) },
                bars: Vec::new(),
            };
            (tile, coverage(&this))
        } else {
            continue;
        };
        if let Some(c) = counted {
            tile.note.push_str(&format!(" · {c}"));
            partial = true;
        }
        d.period_tiles.push(tile);
    }
    if partial {
        d.period_note =
            "A browser or app reports these once it has the version that counts weeks, so they run low until all have it.".into();
    }
}

/// The daily bars, their scale and the trailing average.
fn chart(d: &mut UsageData, all: &[UsageDay], window: &[UsageDay]) {
    let peak = window.iter().filter(|r| !r.excluded).map(|r| r.users).max().unwrap_or(0);
    d.max = peak;
    d.empty = window.iter().all(|r| r.users == 0);
    let (top, step) = scale(peak);
    let n = window.len();
    let width = 720.0 / n as f64;
    let gap = width * 0.18;
    let y = |value: f64| 160.0 - 160.0 * value.min(top as f64) / top as f64;
    d.ticks = (0..=top / step)
        .map(|k| UsageTick { at: format_float(100.0 * (k * step) as f64 / top as f64), label: (k * step).to_string() })
        .collect();
    d.grid_path = (0..=top / step)
        .map(|k| format!("M0 {}H720", format_float((100.0 * y((k * step) as f64)).round() / 100.0)))
        .collect::<Vec<_>>()
        .join(" ");
    // A completed day's trailing average reads the seven days ending on it, when all are recorded.
    let rolling: Vec<(usize, f64)> =
        (1..n).filter_map(|i| all.get(i..i + 7).and_then(|week| mean(week, |r| Some(r.users))).map(|avg| (n - 1 - i, avg))).collect();
    if rolling.len() >= 2 {
        d.rolling_path = rolling
            .iter()
            .rev()
            .enumerate()
            .map(|(k, &(x, avg))| {
                format!("{}{} {}", if k == 0 { "M" } else { "L" }, format_float((x as f64 + 0.5) * width), format_float(y(avg)))
            })
            .collect::<Vec<_>>()
            .join(" ");
    }
    for i in (0..n).rev() {
        let row = &window[i];
        let height = 160.0 - y(row.users as f64);
        let new = row.new_readers.unwrap_or(0).clamp(0, row.users);
        let mut title = format!("{}: {} reader{}", row.label, row.users, if row.users == 1 { "" } else { "s" });
        if row.new_readers.is_some() {
            title.push_str(&format!(", {new} new"));
        }
        if row.excluded {
            title.push_str(&format!(" ({})", row.note));
        }
        d.chart_split |= row.new_readers.is_some() && !row.excluded;
        d.chart_plain |= row.new_readers.is_none() && !row.excluded;
        d.chart_excluded |= row.excluded;
        d.chart.push(UsageBar {
            day: row.day.clone(),
            title,
            x: format_float((n - 1 - i) as f64 * width + gap / 2.0),
            width: format_float(width - gap),
            y: format_float(160.0 - height),
            height: format_float(height),
            new_height: format_float(if row.users > 0 { height * new as f64 / row.users as f64 } else { 0.0 }),
            split: row.new_readers.is_some(),
            today: i == 0,
            excluded: row.excluded,
        });
    }
    // The first day, then each month's first, clear of both ends.
    let at = |i: usize| 100.0 * ((n - 1 - i) as f64 + 0.5) / n as f64;
    d.dates.push(UsageTick { at: format_float(at(n - 1)), label: window[n - 1].label.clone() });
    // A label is about a seventh of a phone's plot wide: the first is set from the left edge and
    // "Today" from the right, and each mark between stands a label clear of the one before.
    // A label that names its year is wider.
    let room = |label: &str| if label.contains(',') { 1.7 * DATE_GAP } else { DATE_GAP };
    let mut last = room(&window[n - 1].label);
    for (i, row) in window.iter().enumerate().rev() {
        let half = room(&row.label) / 2.0;
        if row.day.ends_with("-01") && at(i) >= last + half && at(i) <= 100.0 - TODAY_GAP - half {
            d.dates.push(UsageTick { at: format_float(at(i)), label: row.label.clone() });
            last = at(i) + half;
        }
    }
    if n > 1 {
        d.dates.push(UsageTick { at: format_float(at(0)), label: "Today".into() });
    }
}

/// The clients as the report names them: stored value, name, and how a new reader arrived.
const CLIENT_NAMES: [(&str, &str, &str); 4] = [
    ("browser", "Browser tab", "in a browser"),
    ("pwa", "Home-screen web app", "as a home-screen web app"),
    ("android", "Android app", "on Android"),
    ("ios", "iOS app", "on iOS"),
];

/// Each client's readers a day over the last seven completed days, against the seven before.
fn clients(all: &[UsageDay]) -> Vec<UsageClient> {
    let completed = &all[1..];
    let week = &completed[..completed.len().min(7)];
    let before = completed.get(7..14);
    let reported = |rows: &[UsageDay]| rows.iter().any(|r| family(r, "client") > 0);
    if week.len() < 7 || !reported(week) {
        return Vec::new();
    }
    let before = before.filter(|b| reported(b));
    let rows: Vec<(&str, f64, Option<f64>, i64)> = CLIENT_NAMES
        .iter()
        .map(|(key, name, _)| {
            let count = |r: &UsageDay| Some(dim(r, &format!("client:{key}")));
            (
                *name,
                mean(week, count).unwrap_or(0.0),
                before.and_then(|b| mean(b, count)),
                total(week, |r| dim(r, &format!("new-client:{key}"))),
            )
        })
        .collect();
    let peak = rows.iter().map(|r| r.1.max(r.2.unwrap_or(0.0))).fold(0.0, f64::max).max(1.0);
    rows.into_iter()
        .map(|(name, now, before, new)| UsageClient {
            name: name.into(),
            now: whole(now),
            before: before.map_or(String::new(), whole),
            direction: match before {
                Some(b) if whole(now) != whole(b) => if now > b { "up" } else { "down" }.into(),
                _ => String::new(),
            },
            width: format_float((100.0 * now / peak).round()),
            mark: before.map_or(String::new(), |b| format_float((100.0 * b / peak).round())),
            new,
        })
        .collect()
}

/// What the window's readers opened, and the settings they read with.
fn breakdowns(d: &mut UsageData, all: &[UsageDay], window: &[UsageDay], year: &str) {
    let readers = total(window, |r| r.users);
    d.readers = readers;
    let share = |name: &str, count: i64, peak: i64| UsageShare {
        name: name.into(),
        value: percent(count, readers),
        detail: count.to_string(),
        width: format_float(if peak > 0 { (100.0 * count as f64 / peak as f64).round() } else { 0.0 }),
    };
    let counts: Vec<i64> = (0..HOURS.len()).map(|h| total(window, |r| r.hours[h])).collect();
    let (ordo, reminders) = (total(window, |r| r.ordo), total(window, |r| r.reminders));
    let peak = counts.iter().copied().chain([ordo, reminders]).max().unwrap_or(0);
    d.opens = HOURS.iter().zip(&counts).map(|(name, &count)| share(name, count, peak)).collect();
    d.opens_extra = vec![share("ordo", ordo, peak), share("reminders", reminders, peak)];
    // Hours per reader, over completed days (today's readers have the rest of the day to open
    // more), from the day after the server began keeping them: on that first day the hours
    // opened before it began were not kept.
    let began = all.iter().rposition(|r| family(r, "offices") > 0).filter(|&i| i + 1 < all.len()).map(|i| all[i].day.as_str());
    let kept: Vec<&UsageDay> =
        window.iter().skip(1).filter(|r| !r.excluded && family(r, "offices") > 0 && Some(r.day.as_str()) != began).collect();
    if let Some(first) = kept.last() {
        let by = |n: usize| kept.iter().map(|r| dim(r, &format!("offices:{n}"))).sum::<i64>();
        let opened: i64 = (1..=7).map(by).sum();
        let hours: i64 = (1..=7).map(|n| n as i64 * by(n)).sum();
        let of = kept.iter().map(|r| r.users).sum::<i64>();
        d.offices_note = format!(
            "Since {}, {} of readers opened an hour, {:.1} each on average: one hour {}, two {}, three {}, four or more {}.",
            date_label(&first.day, year),
            percent(opened, of),
            hours as f64 / opened.max(1) as f64,
            percent(by(1), opened),
            percent(by(2), opened),
            percent(by(3), opened),
            percent((4..=7).map(by).sum(), opened),
        );
    }
    let sum = |scope: &str| total(window, |r| dim(r, scope));
    let mut setting = |name: &str, part: i64, of: i64, detail: String| {
        if of > 0 {
            d.settings.push(UsageShare {
                name: name.into(),
                value: percent(part, of),
                detail,
                width: format_float((100.0 * part as f64 / of as f64).round()),
            });
        }
    };
    let (nave, apse) = (sum("appearance:nave"), sum("appearance:apse"));
    setting("Appearance", apse, nave + apse, "Apse (dark)".into());
    let (desktop, mobile) = (sum("screen:desktop"), sum("screen:mobile"));
    setting("Screen", mobile, desktop + mobile, "phone or tablet".into());
    let (on, off) = (sum("chant:on"), sum("chant:off"));
    setting("Chant notation", on, on + off, "turned on, where an hour has a score".into());
    let (shown, hidden) = (sum("martyrology:shown"), sum("martyrology:hidden"));
    setting("Martyrology at Prime", shown, shown + hidden, "shown".into());
    let (private, deacon, priest) = (sum("prayer-form:private"), sum("prayer-form:deacon"), sum("prayer-form:priest"));
    let forms = private + deacon + priest;
    setting(
        "Prayer form",
        deacon + priest,
        forms,
        format!("clerical: priest {}, deacon {}", percent(priest, forms), percent(deacon, forms)),
    );
}

/// The families the newer clients report, read over the window's completed days.
fn habits(d: &mut UsageData, all: &[UsageDay], window: &[UsageDay]) {
    let days: Vec<&UsageDay> = window.iter().skip(1).filter(|r| !r.excluded).collect();
    let sum = |scope: &str| days.iter().map(|r| dim(r, scope)).sum::<i64>();
    let reporting = sum(&format!("version:{}", presentation::usage::VERSION));
    if reporting == 0 {
        return;
    }
    let readers: i64 = days.iter().map(|r| r.users).sum();
    d.habits_note = format!(
        "Reported by {} of the period’s readers{}.",
        percent(reporting, readers),
        if reporting * 20 < readers * 19 { ": a browser or app says these once it has the version that counts them" } else { "" }
    );
    d.keeping = HOURS
        .iter()
        .map(|h| {
            let reported = sum(&format!("reported:{h}"));
            UsageKeep {
                name: h.to_string(),
                reported,
                timely: percent(sum(&format!("timely:{h}")), reported),
                prayed: percent(sum(&format!("prayed:{h}")), reported),
            }
        })
        .collect();
    let mut row = |name: &str, part: i64, of: i64, detail: String| {
        if of > 0 {
            d.habits.push(UsageShare {
                name: name.into(),
                value: percent(part, of),
                detail,
                width: format_float((100.0 * part as f64 / of as f64).round()),
            });
        }
    };
    let tenure: Vec<i64> = presentation::usage::TENURES.iter().map(|t| sum(&format!("tenure:{t}"))).collect();
    let told: i64 = tenure.iter().sum();
    row(
        "In their first week",
        tenure[0],
        told,
        format!("of {told} who said; {} in their first month, {} longer", percent(tenure[1], told), percent(tenure[2], told)),
    );
    let weeks: Vec<i64> = presentation::usage::LAST_WEEK.iter().map(|n| sum(&format!("last-week:{n}"))).collect();
    let back: i64 = weeks.iter().sum();
    row(
        "Four or more days last week",
        weeks[2] + weeks[3],
        back,
        format!(
            "of {back} back for a new week; one day {}, two or three {}, all seven {}",
            percent(weeks[0], back),
            percent(weeks[1], back),
            percent(weeks[3], back)
        ),
    );
    // A new reader's first day, from the day after the server began keeping it: the first day a
    // client said its version, which came with it, unless that is where the record begins.
    let began = all.iter().rposition(|r| family(r, "version") > 0).filter(|&i| i + 1 < all.len()).map(|i| all[i].day.as_str());
    let kept: Vec<&&UsageDay> = days.iter().filter(|r| family(r, "version") > 0 && began.is_none_or(|b| r.day.as_str() > b)).collect();
    let new: i64 = kept.iter().map(|r| dim(r, "visit:first")).sum();
    let by = |n: usize| kept.iter().map(|r| dim(r, &format!("new-offices:{n}"))).sum::<i64>();
    let opened: i64 = (1..=7).map(by).sum();
    let hours: i64 = (1..=7).map(|n| n as i64 * by(n)).sum();
    row(
        "New readers who opened an hour",
        opened,
        new,
        format!("of {new} on their first day, {:.1} hours each", hours as f64 / opened.max(1) as f64),
    );
    row("Opened an hour that could not report", sum("offline:yes"), reporting, "since their last report: no network, or no answer".into());
    let (on, off) = (sum("reminder:on"), sum("reminder:off"));
    row("Reminders on, in the apps", on, on + off, format!("of {} app readers", on + off));
}

/// Each family's breakdown: its heading and its values' labels, in the store's value order.
/// The hours opened in a day are drawn as four series, [`OFFICES_FOLD`] and more as one.
/// The hours' own families (`timely`, `prayed`, `reported`) are read in "How they keep the hours".
pub const TREND_LABELS: [(&str, &str, &[&str]); 17] = [
    ("client", "Where they read", &["Browser tab", "Home-screen web app", "Android app", "iOS app"]),
    ("visit", "New or returning", &["New", "Returning"]),
    ("new-client", "New readers, by where they read", &["Browser tab", "Home-screen web app", "Android app", "iOS app"]),
    ("week", "First day of their week", &["First this week", "Again this week"]),
    ("month", "First day of their month", &["First this month", "Again this month"]),
    ("offices", "Hours opened in a day", &["One", "Two", "Three", "Four or more"]),
    ("prayer-form", "Prayer form", &["Private", "Deacon", "Priest"]),
    ("chant", "Chant notation", &["On", "Off"]),
    ("martyrology", "Martyrology at Prime", &["Shown", "Hidden"]),
    ("appearance", "Appearance", &["Nave", "Apse"]),
    ("screen", "Screen", &["Desktop", "Mobile"]),
    ("version", "Clients that report how they keep the hours", &["Reporting"]),
    ("tenure", "How long they have read", &["First week", "First month", "Longer"]),
    ("last-week", "Days read the week before", &["One", "Two or three", "Four to six", "All seven"]),
    ("new-offices", "Hours a new reader opened", &["One", "Two", "Three", "Four or more"]),
    ("offline", "Opened an hour that could not report", &["Yes"]),
    ("reminder", "Reminders, in the apps", &["On", "Off"]),
];

/// The families read in "How they keep the hours" rather than the explorer.
pub const HOUR_FAMILIES: [&str; 3] = ["timely", "prayed", "reported"];

/// The `offices` value from which the report draws one series.
pub const OFFICES_FOLD: usize = 4;

fn trend_groups(rows: &[UsageDay], dimensions: &[Dimension]) -> Vec<TrendGroup> {
    let points = |scopes: &[String]| -> Vec<TrendPoint> {
        rows.iter()
            .rev()
            .map(|r| TrendPoint { day: r.day.clone(), counts: scopes.iter().map(|s| r.dimensions.get(s).copied().unwrap_or(0)).collect() })
            .collect()
    };
    let mut groups = Vec::new();
    for (key, label, series) in TREND_LABELS {
        let Some(dimension) = dimensions.iter().find(|d| d.key == key) else { continue };
        let scopes: Vec<String> = dimension.values.iter().map(|v| format!("{key}:{v}")).collect();
        let mut points = points(&scopes);
        if key == "offices" || key == "new-offices" {
            for point in &mut points {
                let rest: i64 = point.counts.drain(OFFICES_FOLD - 1..).sum();
                point.counts.push(rest);
            }
        }
        groups.push(TrendGroup { key: key.into(), label: label.into(), series: series.iter().map(|l| l.to_string()).collect(), points });
    }
    groups
}

/// The trend groups as a JSON script element, with the field names expected by the browser.
fn trend_json(groups: &[TrendGroup]) -> String {
    let nil_or = |items: Vec<Json>| if items.is_empty() { Json::Null } else { Json::Arr(items) };
    let value = Json::Arr(
        groups
            .iter()
            .map(|g| {
                Obj::new()
                    .str("Key", &g.key)
                    .str("Label", &g.label)
                    .field("Series", Json::strings(&g.series))
                    .field(
                        "Points",
                        nil_or(
                            g.points
                                .iter()
                                .map(|p| {
                                    Obj::new()
                                        .str("Day", &p.day)
                                        .field("Counts", nil_or(p.counts.iter().map(|&c| Json::Int(c)).collect()))
                                        .build()
                                })
                                .collect(),
                        ),
                    )
                    .build()
            })
            .collect(),
    );
    data_format::json::encode_compact(&value)
}

#[cfg(test)]
mod tests {
    use super::*;

    use presentation::usage::{DIMENSIONS, PRAYER_FORMS};

    const NONE: Corrections = Corrections { test_days: &[], released: &[] };

    fn row(day: &str, users: i64) -> UsageDay {
        UsageDay { day: day.into(), users, ..UsageDay::default() }
    }

    fn with(mut row: UsageDay, dimensions: &[(&str, i64)]) -> UsageDay {
        row.dimensions.extend(dimensions.iter().map(|(k, v)| (k.to_string(), *v)));
        row
    }

    fn num(s: &str) -> f64 {
        s.parse().unwrap()
    }

    fn data(rows: Vec<UsageDay>, days: i64, since: &str) -> UsageData {
        usage_data(rows, days, Some(since), &DIMENSIONS, &NONE)
    }

    // Newest first: today (Tuesday 6 October 2026), then `completed` days counting back.
    fn days(today: i64, completed: &[i64]) -> Vec<UsageDay> {
        let start = Date::new(2026, 10, 6);
        std::iter::once(today)
            .chain(completed.iter().copied())
            .enumerate()
            .map(|(i, n)| row(&presentation::date_slug(start.add_days(-(i as i32))), n))
            .collect()
    }

    #[test]
    fn chart_runs_oldest_first_on_a_round_scale() {
        let d = data(vec![row("2026-09-05", 3), row("2026-09-04", 12), row("2026-09-03", 0)], 7, "2026-09-01");
        assert_eq!(d.max, 12);
        assert!(d.chart[0].day == "2026-09-03" && d.chart[2].today && d.rows[0].day == "2026-09-05", "chart and table order");
        assert_eq!(d.ticks.iter().map(|t| t.label.as_str()).collect::<Vec<_>>(), ["0", "5", "10", "15"]);
        assert_eq!(d.grid_path, "M0 160H720 M0 106.67H720 M0 53.33H720 M0 0H720");
        for bar in &d.chart {
            let (x, width, height) = (num(&bar.x), num(&bar.width), num(&bar.height));
            assert!((0.0..=160.0).contains(&height) && x >= 0.0 && x + width <= 720.0, "bar outside plot: {bar:?}");
            assert!(!bar.split && bar.new_height == "0", "a split drawn before new readers were told apart: {bar:?}");
        }
        assert_eq!(d.chart[1].title, "Sep 4: 12 readers");
        assert_eq!(d.dates.iter().map(|t| t.label.as_str()).collect::<Vec<_>>(), ["Sep 3", "Today"]);
        for (max, want) in [(0, (1, 1)), (4, (4, 1)), (7, (8, 2)), (76, (80, 20)), (105, (150, 50)), (400, (400, 100)), (401, (600, 200))] {
            assert_eq!(scale(max), want, "{max}");
        }
        let empty = usage_data(vec![row("2026-09-05", 0)], 7, None, &DIMENSIONS, &NONE);
        assert!(empty.max == 0 && num(&empty.chart[0].height) == 0.0, "empty report implies activity");
        assert_eq!(usage_data(vec![], 7, None, &DIMENSIONS, &NONE).headline_rest, "No visits recorded yet.");
        // A window reaching into last year names that year.
        let crossing = data(vec![row("2026-01-02", 1), row("2026-01-01", 1), row("2025-12-31", 1)], 7, "2025-12-31");
        assert_eq!((crossing.first_date.as_str(), crossing.last_date.as_str()), ("Dec 31, 2025", "Jan 2"));
    }

    #[test]
    fn days_before_collection_are_absent_not_quiet() {
        let rows = vec![row("2026-09-05", 2), row("2026-09-04", 10), row("2026-09-03", 6), row("2026-09-02", 0), row("2026-09-01", 0)];
        let d = data(rows, 7, "2026-09-03");
        assert_eq!((d.recorded_days, d.rows.len(), d.chart.len(), d.first_date.as_str()), (3, 3, 3, "Sep 3"));
        assert_eq!((d.headline_figure.as_str(), d.headline_rest.as_str()), ("8 readers a day", " over 2 completed days."));
        assert_eq!((d.rows[0].label.as_str(), d.rows[0].weekday.as_str()), ("Sep 5", "Sat"));
        assert!(d.rolling_path.is_empty(), "a seven-day line drawn from three days");
        // The record is shorter than every window but the first: one tab stands for all of it.
        let tabs: Vec<(i64, &str, bool)> = d.tabs.iter().map(|t| (t.days, t.label.as_str(), t.current)).collect();
        assert_eq!(tabs, [(7, "Since Sep 3", true)]);
        let d = data(days(0, &[1; 40]), 90, "2026-08-27");
        let tabs: Vec<(i64, &str, bool)> = d.tabs.iter().map(|t| (t.days, t.label.as_str(), t.current)).collect();
        assert_eq!(tabs, [(7, "7 days", false), (30, "30 days", false), (90, "Since Aug 27", true)]);
        // A record just as long as a window is that window, once.
        let d = data(days(0, &[1; 29]), 30, "2026-09-07");
        let tabs: Vec<(i64, &str, bool)> = d.tabs.iter().map(|t| (t.days, t.label.as_str(), t.current)).collect();
        assert_eq!(tabs, [(7, "7 days", false), (30, "Since Sep 7", true)]);
        // Dates under the bars stand clear of each other: a month's first beside the window's
        // first day, or beside today, is not named.
        let d = data(days(0, &[1; 40]), 38, "2026-08-27");
        assert_eq!(d.dates.iter().map(|t| t.label.as_str()).collect::<Vec<_>>(), ["Aug 30", "Today"]);
        let d = data(days(0, &[1; 40]), 365, "2026-08-27");
        assert!(d.tabs[2].current, "a longer window than the record is the whole record");
    }

    #[test]
    fn the_headline_sets_last_week_against_the_week_before() {
        let headline = |rows: Vec<UsageDay>| {
            let d = data(rows, 7, "2026-01-01");
            format!("{}{}", d.headline_figure, d.headline_rest)
        };
        // The window is seven days, but the comparison reads the days before it; today never counts.
        let weeks = |now: i64, before: i64| [vec![now; 7], vec![before; 7]].concat();
        assert_eq!(headline(days(500, &weeks(20, 10))), "20 readers a day last week, up from 10 the week before.");
        assert_eq!(headline(days(0, &weeks(9, 12))), "9 readers a day last week, down from 12 the week before.");
        // Under a fifth either way is not told from chance.
        assert_eq!(headline(days(0, &weeks(36, 33))), "36 readers a day last week, about level with the week before (33).");
        assert_eq!(headline(days(0, &weeks(4, 0))), "4 readers a day last week, up from 0 the week before.");
        assert_eq!(headline(days(0, &weeks(0, 0))), "0 readers a day last week, about level with the week before (0).");
        assert_eq!(headline(days(0, &[6; 9])), "6 readers a day last week.");
        assert_eq!(headline(days(0, &[3, 5])), "4 readers a day over 2 completed days.");
        assert_eq!(headline(days(3, &[])), "No completed days yet.");
        // Four weeks, oldest first, under the first figure.
        let d = data(days(7, &[vec![40; 7], vec![30; 7], vec![20; 7], vec![10; 7], vec![99; 7]].concat()), 7, "2026-01-01");
        assert_eq!((d.tiles[0].value.as_str(), d.tiles[0].note.as_str()), ("40", "4 weeks: 10 · 20 · 30 · 40"));
        assert_eq!(d.tiles[0].bars, ["25", "50", "75", "100"]);
        let today = d.tiles.last().unwrap();
        assert_eq!(
            (today.label.as_str(), today.value.as_str(), today.note.as_str()),
            ("Today so far", "7", "Yesterday 40 · last Tuesday 40")
        );
    }

    #[test]
    fn returning_and_new_readers_over_the_last_week() {
        let mut rows = days(9, &[10; 16]);
        for (i, row) in rows.iter_mut().enumerate().take(15) {
            let new = if i <= 7 { 3 } else { 1 };
            row.dimensions.extend([("visit:first".to_string(), new), ("visit:returning".to_string(), 10 - new)]);
            row.dimensions.extend([("new-client:browser".to_string(), new - 1), ("new-client:android".to_string(), 1)]);
        }
        let d = data(rows, 7, "2026-01-01");
        let tiles: Vec<(&str, &str, &str)> = d.tiles.iter().map(|t| (t.label.as_str(), t.value.as_str(), t.note.as_str())).collect();
        assert_eq!(tiles[1], ("Returning, a day", "7", "9 the week before"));
        assert_eq!(tiles[2], ("New, in 7 days", "21", "7 the week before"));
        assert_eq!(
            d.headline_detail,
            "Of those, 7 a day were returning (9 the week before). 21 were new in the week (7 before). New to each client: 14 in a browser, 7 on Android."
        );
        // New readers were first told apart fourteen days ago: the days before show none, not zero.
        assert_eq!(d.rows.iter().map(|r| r.new_readers).collect::<Vec<_>>(), [Some(3); 7]);
        let d = data(days(0, &[5; 20]), 30, "2026-01-01");
        assert!(d.rows.iter().chain(&d.more_rows).all(|r| r.new_readers.is_none()) && d.tiles.len() == 2 && d.headline_detail.is_empty());
        assert!(d.chart_plain && !d.chart_split);
    }

    #[test]
    fn bars_split_into_returning_and_new() {
        let rows = vec![
            with(row("2026-10-06", 8), &[("visit:first", 2), ("visit:returning", 6)]),
            with(row("2026-10-05", 10), &[("visit:first", 5), ("visit:returning", 5)]),
            row("2026-10-04", 10),
        ];
        let d = data(rows, 7, "2026-10-04");
        let bars: Vec<(bool, &str, &str, &str)> =
            d.chart.iter().map(|b| (b.split, b.height.as_str(), b.new_height.as_str(), b.title.as_str())).collect();
        assert_eq!(bars[0], (false, "160", "0", "Oct 4: 10 readers"));
        assert_eq!(bars[1], (true, "160", "80", "Oct 5: 10 readers, 5 new"));
        assert_eq!(bars[2], (true, "128", "32", "Oct 6: 8 readers, 2 new"));
        assert!(d.chart_split && d.chart_plain && !d.chart_excluded);
    }

    #[test]
    fn test_devices_are_shown_but_never_averaged() {
        let corrections = Corrections { test_days: &[("2026-10-03", "a crawl")], released: &[("ios", "2026-10-05", "review devices")] };
        let mut rows = days(4, &[20; 14]);
        // Saturday 3 October: a hundred test phones.
        rows[3].users = 120;
        rows[3].hours = [90; 7];
        // Every day reports six iOS installs, four of them new.
        for row in &mut rows {
            row.dimensions.extend(
                [("client:ios", 6), ("new-client:ios", 4), ("client:browser", 14), ("visit:first", 5), ("visit:returning", 15)]
                    .map(|(k, v)| (k.to_string(), v)),
            );
        }
        let d = usage_data(rows, 7, Some("2026-01-01"), &DIMENSIONS, &corrections);
        // Before its release iOS is taken out of readers, new readers and the clients.
        let table: Vec<(i64, Option<i64>, bool)> = d.rows.iter().map(|r| (r.users, r.new_readers, r.excluded)).collect();
        assert_eq!(
            table[..5],
            [(4, Some(5), false), (20, Some(5), false), (14, Some(1), false), (114, Some(1), true), (14, Some(1), false)]
        );
        // Yesterday counts whole; the five days before lose iOS, and the crawl counts nowhere.
        assert_eq!(d.headline_figure, "15 readers a day");
        assert_eq!(d.max, 20, "the crawl set the scale");
        let crawl = &d.chart[3];
        assert!(crawl.excluded && crawl.height == "160" && crawl.title == "Oct 3: 114 readers, 1 new (a crawl)", "{crawl:?}");
        assert!(d.chart_excluded);
        assert_eq!(d.readers, 4 + 20 + 14 * 4, "the period's readers");
        assert_eq!(d.opens[0].detail, "0", "the crawl's hours were counted");
        let ios = &d.clients[3];
        assert_eq!((ios.name.as_str(), ios.now.as_str(), ios.new), ("iOS app", "1", 4));
        assert_eq!(d.tiles[2].value, "10", "new readers in the week: 5, then five days of 1");
        assert_eq!(d.corrections.len(), 2);
        assert!(
            d.corrections[0].starts_with("Oct 3: a crawl.") && d.corrections[1].starts_with("iOS app before Oct 5: review devices."),
            "{:?}",
            d.corrections
        );
        // A window clear of them says nothing.
        let d = usage_data(days(4, &[20; 3]), 7, Some("2026-10-05"), &DIMENSIONS, &corrections);
        assert!(d.corrections.is_empty());
    }

    #[test]
    fn seven_day_line_follows_the_bars() {
        // Thirty days of 7, then a step to 14: the line rises over the week after it.
        let mut completed = vec![14; 7];
        completed.extend([7; 30]);
        let d = data(days(1, &completed), 30, "2026-01-01");
        let points: Vec<(f64, f64)> = d
            .rolling_path
            .split(['M', 'L'])
            .filter(|p| !p.trim().is_empty())
            .map(|p| {
                let mut xy = p.split_whitespace().map(num);
                (xy.next().unwrap(), xy.next().unwrap())
            })
            .collect();
        // Every completed day in the window has a full week behind it; today has no point.
        assert_eq!(points.len(), 29);
        assert!(points.windows(2).all(|w| w[0].0 < w[1].0), "line runs oldest to newest");
        let y = |avg: f64| 160.0 - 160.0 * avg / 15.0;
        assert_eq!(points[0].1, num(&format_float(y(7.0))));
        assert_eq!(points[28].1, num(&format_float(y(14.0))));
        let bar = &d.chart[28];
        assert_eq!(points[28].0, num(&bar.x) + num(&bar.width) / 2.0, "a point sits over its day's bar");
        assert_eq!((d.rows.len(), d.more_rows.len(), d.chart.len()), (14, 16, 30), "lookback days are not shown");
        // Mondays close a week in the table.
        assert_eq!(d.rows.iter().filter(|r| r.week_start).map(|r| r.label.as_str()).collect::<Vec<_>>(), ["Oct 5", "Sep 28"]);
        // The month's first day is marked under the axis.
        assert_eq!(d.dates.iter().map(|t| t.label.as_str()).collect::<Vec<_>>(), ["Sep 7", "Oct 1", "Today"]);
    }

    #[test]
    fn what_readers_open_is_a_share_of_readers() {
        let rows = vec![
            UsageDay { hours: [4, 1, 0, 0, 0, 0, 0], ordo: 2, reminders: 1, ..row("2026-09-05", 10) },
            with(
                UsageDay { hours: [4, 0, 0, 0, 0, 5, 0], ordo: 3, ..row("2026-09-04", 10) },
                &[
                    ("offices:1", 3),
                    ("offices:2", 2),
                    ("offices:5", 1),
                    ("appearance:apse", 6),
                    ("appearance:nave", 2),
                    ("prayer-form:private", 7),
                    ("prayer-form:priest", 2),
                    ("prayer-form:deacon", 1),
                ],
            ),
        ];
        let d = data(rows.clone(), 7, "2026-09-04");
        let opens: Vec<(&str, &str, &str, &str)> = d
            .opens
            .iter()
            .chain(&d.opens_extra)
            .map(|o| (o.name.as_str(), o.value.as_str(), o.detail.as_str(), o.width.as_str()))
            .collect();
        assert_eq!(opens[0], ("lauds", "40%", "8", "100"));
        assert_eq!(opens[5], ("vespers", "25%", "5", "63"));
        assert_eq!(opens[2], ("terce", "0%", "0", "0"));
        assert_eq!(opens[7..], [("ordo", "25%", "5", "63"), ("reminders", "5%", "1", "13")]);
        assert_eq!(d.readers, 20);
        // Hours per reader are read only from completed days the server kept them.
        assert_eq!(
            d.offices_note,
            "Since Sep 4, 60% of readers opened an hour, 2.0 each on average: one hour 50%, two 33%, three 0%, four or more 17%."
        );
        let settings: Vec<(&str, &str, &str)> = d.settings.iter().map(|o| (o.name.as_str(), o.value.as_str(), o.detail.as_str())).collect();
        assert_eq!(settings, [("Appearance", "75%", "Apse (dark)"), ("Prayer form", "30%", "clerical: priest 20%, deacon 10%")]);
        // Today is still filling, and on the server's first day of keeping them the hours opened
        // before it began are missing: neither is read.
        let mut later =
            vec![with(row("2026-09-07", 4), &[("offices:1", 4)]), with(row("2026-09-06", 10), &[("offices:1", 1), ("offices:3", 4)])];
        later.extend(rows);
        later.push(row("2026-09-03", 0));
        let d = data(later, 7, "2026-09-03");
        assert_eq!(
            d.offices_note,
            "Since Sep 6, 50% of readers opened an hour, 2.6 each on average: one hour 20%, two 0%, three 80%, four or more 0%."
        );
        let d = data(vec![row("2026-09-05", 0)], 7, "2026-09-05");
        assert!(d.empty && d.opens.iter().all(|o| o.width == "0" && o.value == "—") && d.offices_note.is_empty() && d.settings.is_empty());
    }

    #[test]
    fn where_they_read_sets_a_week_against_the_one_before() {
        let mut rows = days(0, &[30; 14]);
        for (i, row) in rows.iter_mut().enumerate() {
            let android = if (1..=7).contains(&i) { 20 } else { 5 };
            row.dimensions
                .extend([("client:browser", 10), ("client:android", android), ("new-client:android", 2)].map(|(k, v)| (k.to_string(), v)));
        }
        let d = data(rows.clone(), 7, "2026-01-01");
        let clients: Vec<(&str, &str, &str, &str, &str, &str, i64)> = d
            .clients
            .iter()
            .map(|c| (c.name.as_str(), c.now.as_str(), c.before.as_str(), c.direction.as_str(), c.width.as_str(), c.mark.as_str(), c.new))
            .collect();
        assert_eq!(clients[0], ("Browser tab", "10", "10", "", "50", "50", 0));
        assert_eq!(clients[2], ("Android app", "20", "5", "up", "100", "25", 14));
        assert_eq!(clients[3], ("iOS app", "0", "0", "", "0", "0", 0));
        // Under a week of it, nothing is drawn.
        rows.truncate(5);
        assert!(data(rows, 7, "2026-01-01").clients.is_empty());
    }

    #[test]
    fn a_weeks_readers_are_its_first_days_added_up() {
        // Today is Tuesday 6 October: the week began on Monday the 5th, the month on Thursday.
        let mut rows = days(10, &[10; 20]);
        let report = |row: &mut UsageDay, first: i64, month: i64| {
            row.dimensions.extend(
                [("week:first", first), ("week:again", 10 - first), ("month:first", month), ("month:again", 10 - month)]
                    .map(|(k, v)| (k.to_string(), v)),
            );
        };
        // Reported since Wednesday 30 September: this week whole so far, last week in part.
        for (i, (first, month)) in [(4, 1), (9, 2), (1, 3), (1, 4), (2, 6), (3, 10), (5, 0)].into_iter().enumerate() {
            report(&mut rows[i], first, month);
        }
        let d = data(rows.clone(), 7, "2026-01-01");
        let tiles: Vec<(&str, &str, &str)> = d.period_tiles.iter().map(|t| (t.label.as_str(), t.value.as_str(), t.note.as_str())).collect();
        assert_eq!(tiles, [("Readers, this week so far", "13", "Oct 5 – Oct 6"), ("Readers, this month so far", "26", "Oct 1 – Oct 6")]);
        assert!(d.period_note.is_empty(), "every reader reported: {}", d.period_note);
        // Once the week before was reported on every day, it leads.
        for row in &mut rows[7..9] {
            report(row, 6, 0);
        }
        let d = data(rows.clone(), 7, "2026-01-01");
        let week = &d.period_tiles[0];
        assert_eq!(
            (week.label.as_str(), week.value.as_str(), week.note.as_str()),
            ("Readers, last week", "24", "Sep 28 – Oct 4 · this week so far 13")
        );
        // Clients that have not updated say nothing, and the report says how many did.
        for row in &mut rows[..2] {
            row.users = 20;
        }
        let d = data(rows.clone(), 7, "2026-01-01");
        assert!(!d.period_note.is_empty());
        assert_eq!(d.period_tiles[0].note, "Sep 28 – Oct 4 · this week so far 13");
        assert_eq!(d.period_tiles[1].note, "Oct 1 – Oct 6 · 75% of its readers reporting");
        // A month whose first days went unreported is counted from the day reports began, and
        // its share is of those days' readers.
        for key in ["month:first", "month:again"] {
            rows[5].dimensions.remove(key);
        }
        let d = data(rows.clone(), 7, "2026-01-01");
        let month = &d.period_tiles[1];
        assert_eq!(
            (month.label.as_str(), month.value.as_str(), month.note.as_str()),
            ("Readers since Oct 2", "16", "Not the whole month: Oct 2 – Oct 6 · 71% of its readers reporting")
        );
        // A first day nobody read on had nothing to report, and the month is whole without it.
        rows[5].users = 0;
        assert_eq!(data(rows.clone(), 7, "2026-01-01").period_tiles[1].label, "Readers, this month so far");
        // Last week's share stands beside last week, before this week's figure.
        for row in &mut rows[2..9] {
            row.users = 20;
        }
        assert_eq!(
            data(rows.clone(), 7, "2026-01-01").period_tiles[0].note,
            "Sep 28 – Oct 4 · 50% of its readers reporting · this week so far 13"
        );
        assert!(data(days(10, &[10; 20]), 7, "2026-01-01").period_tiles.is_empty());
    }

    #[test]
    fn how_they_keep_the_hours() {
        // Today, then two completed days, the older of them the first the server kept a new
        // reader's hours: it is left out of those.
        let rows = vec![
            with(row("2026-11-20", 50), &[("version:2", 50), ("reported:lauds", 40), ("prayed:lauds", 40)]),
            with(
                row("2026-11-19", 20),
                &[
                    ("version:2", 10),
                    ("reported:lauds", 8),
                    ("timely:lauds", 6),
                    ("prayed:lauds", 2),
                    ("tenure:week", 2),
                    ("tenure:month", 3),
                    ("tenure:longer", 5),
                    ("last-week:1", 1),
                    ("last-week:7", 3),
                    ("visit:first", 4),
                    ("new-offices:1", 2),
                    ("new-offices:3", 1),
                    ("offline:yes", 1),
                    ("reminder:on", 1),
                    ("reminder:off", 3),
                ],
            ),
            with(row("2026-11-18", 10), &[("version:2", 2), ("reported:lauds", 2), ("visit:first", 2), ("new-offices:1", 2)]),
            row("2026-11-17", 0),
        ];
        let d = data(rows, 7, "2026-11-17");
        assert_eq!(
            d.habits_note,
            "Reported by 40% of the period’s readers: a browser or app says these once it has the version that counts them."
        );
        let lauds = &d.keeping[0];
        assert_eq!((lauds.reported, lauds.timely.as_str(), lauds.prayed.as_str()), (10, "60%", "20%"));
        assert_eq!(d.keeping[1].timely, "—");
        let habits: Vec<(&str, &str, &str)> = d.habits.iter().map(|o| (o.name.as_str(), o.value.as_str(), o.detail.as_str())).collect();
        assert_eq!(
            habits,
            [
                ("In their first week", "20%", "of 10 who said; 30% in their first month, 50% longer"),
                ("Four or more days last week", "75%", "of 4 back for a new week; one day 25%, two or three 0%, all seven 75%"),
                ("New readers who opened an hour", "75%", "of 4 on their first day, 1.7 hours each"),
                ("Opened an hour that could not report", "8%", "since their last report: no network, or no answer"),
                ("Reminders on, in the apps", "25%", "of 4 app readers"),
            ]
        );
        // Before any client reports, there is no section.
        let d = data(days(0, &[5; 9]), 7, "2026-01-01");
        assert!(d.habits_note.is_empty() && d.keeping.is_empty() && d.habits.is_empty());
    }

    #[test]
    fn trends_keep_scope_counts_and_chronological_dates() {
        let dimensions = [
            ("appearance:apse", 5),
            ("screen:mobile", 8),
            ("prayer-form:priest", 2),
            ("client:ios", 3),
            ("martyrology:hidden", 6),
            ("chant:on", 7),
            ("week:first", 9),
            ("offices:1", 4),
            ("offices:4", 2),
            ("offices:7", 1),
        ];
        let rows = vec![with(row("2026-09-14", 0), &dimensions), row("2026-09-13", 0)];
        let groups = trend_groups(&rows, &DIMENSIONS);
        assert_eq!(groups.len(), DIMENSIONS.len() - HOUR_FAMILIES.len());
        for group in &groups {
            assert!(
                group.points[0].day == "2026-09-13"
                    && group.points[1].day == "2026-09-14"
                    && group.points.iter().all(|p| p.counts.len() == group.series.len()),
                "misaligned series: {group:?}"
            );
            assert!(group.points[0].counts.iter().all(|&c| c == 0), "missing observations invented: {group:?}");
        }
        let counts = |key: &str| groups.iter().find(|g| g.key == key).unwrap().points[1].counts.clone();
        assert_eq!(groups[0].key, "client", "the explorer opens on where they read");
        assert_eq!(counts("client"), [0, 0, 0, 3]);
        assert_eq!(counts("appearance"), [0, 5]);
        assert_eq!(counts("week"), [9, 0]);
        // Four hours and more are one series.
        assert_eq!(counts("offices"), [4, 0, 0, 3]);
    }

    #[test]
    fn trend_json_matches_browser_contract() {
        let some = [Dimension { key: "appearance", values: &["nave", "apse"] }, Dimension { key: "prayer-form", values: PRAYER_FORMS }];
        let groups = trend_groups(&[row("2026-09-14", 0)], &some);
        assert_eq!(
            trend_json(&groups),
            r#"[{"Key":"prayer-form","Label":"Prayer form","Series":["Private","Deacon","Priest"],"Points":[{"Day":"2026-09-14","Counts":[0,0,0]}]},{"Key":"appearance","Label":"Appearance","Series":["Nave","Apse"],"Points":[{"Day":"2026-09-14","Counts":[0,0]}]}]"#
        );
        assert_eq!(
            trend_json(&trend_groups(&[], &DIMENSIONS[3..4])),
            r#"[{"Key":"client","Label":"Where they read","Series":["Browser tab","Home-screen web app","Android app","iOS app"],"Points":null}]"#
        );
    }
}
