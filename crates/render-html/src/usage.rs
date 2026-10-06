//! The usage report's view model.

use std::collections::BTreeMap;

use calendar::Date;
use data_format::json::{Json, Obj};
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
    /// Display date and short weekday, filled by the view model.
    pub label: String,
    pub weekday: String,
    /// Counts by qualified dimension scope ("appearance:apse").
    #[serde(skip)]
    pub dimensions: BTreeMap<String, i64>,
}

#[derive(Clone, Debug, Default, Serialize)]
pub struct UsageOffice {
    pub name: String,
    pub count: i64,
    pub width: String,
}

#[derive(Clone, Debug, Default, Serialize)]
pub struct UsageBar {
    pub day: String,
    pub users: i64,
    pub x: String,
    pub y: String,
    pub width: String,
    pub height: String,
    pub today: bool,
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
    /// Days in the window since records began; fewer than `days` when the
    /// window reaches back before collection.
    pub recorded_days: i64,
    /// The first recorded day, when it falls inside the window.
    pub since_date: String,
    pub max: i64,
    pub today: i64,
    pub yesterday: i64,
    pub browser_days: i64,
    pub complete_days: i64,
    /// `%.1f` of the average over the last seven completed days (fewer, while
    /// fewer are recorded).
    pub week_average: String,
    /// Completed days behind `week_average`, at most seven.
    pub week_days: i64,
    /// The note under `week_average`, comparing it with the seven days before
    /// once fourteen completed days are recorded.
    pub week_note: String,
    /// The trailing seven-day average across the chart, as an SVG path in its
    /// coordinates; empty when fewer than two days have one.
    pub rolling_path: String,
    pub office_totals: Vec<UsageOffice>,
    pub ordo_total: i64,
    pub reminders_total: i64,
    pub hours: Vec<String>,
    pub rows: Vec<UsageDay>,
    pub chart: Vec<UsageBar>,
    pub first_date: String,
    pub last_date: String,
    pub peak_date: String,
}

const MONTHS: [&str; 12] = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];

const WEEKDAYS: [&str; 7] = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];

/// Days before a window that its weekly figures read: the seven-day average at the window's
/// oldest day, and the week before the last.
pub const LOOKBACK: usize = 8;

/// Builds the chronological chart and newest-first table. The peak is a daily count, not a sum of
/// overlapping browsers. Days before `since`, the first recorded day, predate collection: they
/// are dropped rather than counted as quiet days, and today always remains. `rows` may reach up
/// to [`LOOKBACK`] days past the window; those older days feed only the weekly figures.
pub fn usage_data(mut rows: Vec<UsageDay>, days: i64, since: Option<&str>, dimensions: &[Dimension]) -> UsageData {
    let recorded = rows.iter().take_while(|r| since.is_some_and(|s| r.day.as_str() >= s)).count();
    rows.truncate(recorded.max(1));
    // Completed days, newest first, including any before the window.
    let history: Vec<i64> = rows.iter().skip(1).map(|r| r.users).collect();
    rows.truncate(days.max(1) as usize);
    let mut d = UsageData {
        chrome: Chrome { page: "usage".into(), ..Chrome::default() },
        days,
        recorded_days: rows.len() as i64,
        hours: HOURS.iter().map(|h| h.to_string()).collect(),
        ..UsageData::default()
    };
    d.trend_groups = trend_groups(&rows, dimensions);
    d.trend_json = trend_json(&d.trend_groups);
    let counts: Vec<i64> = (0..HOURS.len()).map(|h| rows.iter().map(|r| r.hours[h]).sum()).collect();
    let peak_office = counts.iter().copied().max().unwrap_or(0).max(0);
    d.office_totals = HOURS
        .iter()
        .zip(&counts)
        .map(|(name, &count)| UsageOffice {
            name: name.to_string(),
            count,
            width: format_float(if peak_office > 0 { 100.0 * count as f64 / peak_office as f64 } else { 0.0 }),
        })
        .collect();
    for (i, row) in rows.iter().enumerate() {
        d.browser_days += row.users;
        d.ordo_total += row.ordo;
        d.reminders_total += row.reminders;
        // Today is in progress; recorded zero days still count.
        if i > 0 {
            d.complete_days += 1;
        }
    }
    let mean = |days: &[i64]| days.iter().sum::<i64>() as f64 / days.len() as f64;
    let week = &history[..history.len().min(7)];
    d.week_days = week.len() as i64;
    d.week_average = if week.is_empty() { String::new() } else { format!("{:.1}", mean(week)) };
    d.week_note = match week.len() {
        0 => "No completed days yet".into(),
        1 => "Over 1 completed day".into(),
        n @ 2..7 => format!("A day, over {n} completed days"),
        _ if history.len() < 14 => "A day, over the last week".into(),
        _ => {
            let before = mean(&history[7..14]);
            let change = if before > 0.0 { (100.0 * (mean(week) - before) / before).round() as i64 } else { 0 };
            match change {
                _ if before == 0.0 => "A day; none the week before".into(),
                0 => "A day, level with the week before".into(),
                c if c > 0 => format!("A day, up {c}% on the week before"),
                c => format!("A day, down {}% on the week before", -c),
            }
        }
    };
    if rows.is_empty() {
        d.rows = rows;
        return d;
    }
    // A year-long window can reach into last year.
    let year = rows[0].day[..4].to_string();
    let label = |day: &str| match Date::parse(day) {
        None => day.to_string(),
        Some(t) if day[..4] != year => format!("{} {}, {}", MONTHS[t.month() as usize - 1], t.day(), t.year()),
        Some(t) => format!("{} {}", MONTHS[t.month() as usize - 1], t.day()),
    };
    if (rows.len() as i64) < days {
        d.since_date = label(&rows[rows.len() - 1].day);
    }
    for row in &mut rows {
        row.label = label(&row.day);
        if let Some(t) = Date::parse(&row.day) {
            row.weekday = WEEKDAYS[t.weekday().number() as usize].to_string();
        }
    }
    d.today = rows[0].users;
    if rows.len() > 1 {
        d.yesterday = rows[1].users;
    }
    d.last_date = label(&rows[0].day);
    d.first_date = label(&rows[rows.len() - 1].day);
    for row in &rows {
        if row.users > d.max {
            d.max = row.users;
            d.peak_date = label(&row.day);
        }
    }
    let scale = if d.max == 0 { 1 } else { d.max } as f64;
    let n = rows.len();
    let step = 720.0 / n as f64;
    let gap = step * 0.18;
    // A completed day's trailing average needs its six days before recorded too.
    let rolling: Vec<(usize, f64)> =
        (1..n).filter(|&i| i + 6 <= history.len()).map(|i| ((n - 1 - i), mean(&history[i - 1..i + 6]))).collect();
    if rolling.len() >= 2 {
        d.rolling_path = rolling
            .iter()
            .rev()
            .enumerate()
            .map(|(k, &(x, avg))| {
                let command = if k == 0 { "M" } else { "L" };
                format!("{command}{} {}", format_float((x as f64 + 0.5) * step), format_float(160.0 - 160.0 * avg / scale))
            })
            .collect::<Vec<_>>()
            .join(" ");
    }
    for i in (0..n).rev() {
        let height = 160.0 * rows[i].users as f64 / scale;
        d.chart.push(UsageBar {
            day: rows[i].day.clone(),
            users: rows[i].users,
            x: format_float((n - 1 - i) as f64 * step + gap / 2.0),
            y: format_float(160.0 - height),
            width: format_float(step - gap),
            height: format_float(height),
            today: i == 0,
        });
    }
    d.rows = rows;
    d
}

/// Each family's breakdown: its heading and its values' labels, in the store's value order.
pub const TREND_LABELS: [(&str, &str, &[&str]); 5] = [
    ("appearance", "Appearance", &["Nave", "Apse"]),
    ("screen", "Screen", &["Desktop", "Mobile"]),
    ("prayer-form", "Prayer form", &["Private", "Deacon", "Priest"]),
    ("client", "Client", &["Browser", "Web app", "Android", "iOS"]),
    ("martyrology", "Martyrology at Prime", &["Shown", "Hidden"]),
];

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
        groups.push(TrendGroup {
            key: key.into(),
            label: label.into(),
            series: series.iter().map(|l| l.to_string()).collect(),
            points: points(&scopes),
        });
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

    fn row(day: &str, users: i64) -> UsageDay {
        UsageDay { day: day.into(), users, ..UsageDay::default() }
    }

    fn num(s: &str) -> f64 {
        s.parse().unwrap()
    }

    #[test]
    fn summary_and_chronological_chart() {
        let d = usage_data(vec![row("2026-09-05", 3), row("2026-09-04", 12), row("2026-09-03", 0)], 3, Some("2026-09-01"), &DIMENSIONS);
        assert_eq!((d.today, d.yesterday, d.max, d.peak_date.as_str()), (3, 12, 12, "Sep 4"));
        assert!(d.chart[0].day == "2026-09-03" && d.chart[2].today && d.rows[0].day == "2026-09-05", "chart and table order");
        for bar in &d.chart {
            let (x, width, height) = (num(&bar.x), num(&bar.width), num(&bar.height));
            assert!((0.0..=160.0).contains(&height) && x >= 0.0 && x + width <= 720.0, "bar outside plot: {bar:?}");
        }
        let empty = usage_data(vec![row("2026-09-05", 0)], 7, None, &DIMENSIONS);
        assert!(empty.max == 0 && num(&empty.chart[0].height) == 0.0 && empty.peak_date.is_empty(), "empty report implies activity");
        // A window reaching into last year names that year.
        let crossing = usage_data(vec![row("2026-01-02", 1), row("2025-12-31", 1)], 2, Some("2025-12-31"), &DIMENSIONS);
        assert_eq!((crossing.first_date.as_str(), crossing.last_date.as_str()), ("Dec 31, 2025", "Jan 2"));
    }

    #[test]
    fn period_totals_and_completed_day_average() {
        let rows = vec![
            UsageDay { hours: [4, 1, 0, 0, 0, 0, 0], ordo: 2, reminders: 1, ..row("2026-09-05", 100) },
            UsageDay { hours: [3, 0, 0, 0, 0, 5, 0], ordo: 3, ..row("2026-09-04", 9) },
            row("2026-09-03", 0),
        ];
        let d = usage_data(rows, 3, Some("2026-09-03"), &DIMENSIONS);
        assert_eq!(
            (d.week_average.as_str(), d.complete_days, d.browser_days),
            ("4.5", 2, 109),
            "average excludes today, includes zero days"
        );
        assert_eq!(d.office_totals[0].count, 7);
        assert_eq!(d.office_totals[0].width, "100");
        assert_eq!(d.office_totals[5].count, 5);
        assert_eq!(d.office_totals[2].width, "0");
        assert_eq!((d.ordo_total, d.reminders_total), (5, 1));
        for rows in [vec![], vec![row("2026-09-05", 0)]] {
            let d = usage_data(rows, 7, None, &DIMENSIONS);
            assert_eq!(
                (d.complete_days, d.week_days, d.week_note.as_str(), d.browser_days),
                (0, 0, "No completed days yet", 0),
                "empty average"
            );
            assert!(d.office_totals.iter().all(|o| o.width == "0"), "empty office comparison");
        }
    }

    #[test]
    fn days_before_collection_are_absent_not_quiet() {
        let rows = vec![row("2026-09-05", 2), row("2026-09-04", 10), row("2026-09-03", 6), row("2026-09-02", 0), row("2026-09-01", 0)];
        let d = usage_data(rows, 5, Some("2026-09-03"), &DIMENSIONS);
        assert_eq!((d.recorded_days, d.complete_days, d.week_average.as_str()), (3, 2, "8.0"), "average spans recorded days");
        assert_eq!((d.rows.len(), d.chart.len(), d.first_date.as_str(), d.since_date.as_str()), (3, 3, "Sep 3", "Sep 3"));
        assert_eq!((d.rows[0].label.as_str(), d.rows[0].weekday.as_str()), ("Sep 5", "Sat"));
        assert!(d.rolling_path.is_empty(), "a seven-day line drawn from three days");
        let whole = usage_data(vec![row("2026-09-05", 2), row("2026-09-04", 10)], 2, Some("2026-08-01"), &DIMENSIONS);
        assert!(whole.since_date.is_empty() && whole.recorded_days == 2, "a fully recorded window names no start");
    }

    // Newest first: today, then `completed` days counting back.
    fn days(today: i64, completed: &[i64]) -> Vec<UsageDay> {
        let start = Date::new(2026, 10, 6);
        std::iter::once(today)
            .chain(completed.iter().copied())
            .enumerate()
            .map(|(i, n)| row(&presentation::date_slug(start.add_days(-(i as i32))), n))
            .collect()
    }

    #[test]
    fn last_week_against_the_week_before() {
        let week = |rows: Vec<UsageDay>| {
            let d = usage_data(rows, 7, Some("2026-01-01"), &DIMENSIONS);
            (d.week_average, d.week_note)
        };
        // The window is seven days, but the comparison reads the lookback past it.
        let mut launch = vec![20; 7];
        launch.extend([10; 7]);
        assert_eq!(week(days(500, &launch)), ("20.0".into(), "A day, up 100% on the week before".into()), "today is never averaged");
        let mut quieter = vec![9; 7];
        quieter.extend([12; 7]);
        assert_eq!(week(days(0, &quieter)).1, "A day, down 25% on the week before");
        assert_eq!(week(days(0, &[5; 14])).1, "A day, level with the week before");
        let mut first = vec![4; 7];
        first.extend([0; 7]);
        assert_eq!(week(days(0, &first)).1, "A day; none the week before");
        assert_eq!(week(days(0, &[6; 9])), ("6.0".into(), "A day, over the last week".into()));
        assert_eq!(week(days(0, &[3, 5])), ("4.0".into(), "A day, over 2 completed days".into()));
    }

    #[test]
    fn seven_day_line_follows_the_bars() {
        // Thirty days of 7, then a step to 14: the line rises over the week after it.
        let mut completed = vec![14; 7];
        completed.extend([7; 30]);
        let d = usage_data(days(1, &completed), 30, Some("2026-01-01"), &DIMENSIONS);
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
        let y = |avg: f64| 160.0 - 160.0 * avg / 14.0;
        assert_eq!(points[0].1, y(7.0));
        assert_eq!(points[28].1, y(14.0));
        let bar = &d.chart[28];
        assert_eq!(points[28].0, num(&bar.x) + num(&bar.width) / 2.0, "a point sits over its day's bar");
        assert!(d.rows.len() == 30 && d.chart.len() == 30, "lookback days are not shown");
    }

    #[test]
    fn trends_keep_scope_counts_and_chronological_dates() {
        let dimensions = [
            ("appearance:nave", 4),
            ("appearance:apse", 5),
            ("screen:mobile", 8),
            ("prayer-form:priest", 2),
            ("client:ios", 3),
            ("martyrology:hidden", 6),
        ]
        .into_iter()
        .map(|(k, v)| (k.to_string(), v))
        .collect();
        let rows = vec![UsageDay { hours: [3, 0, 0, 0, 0, 2, 0], dimensions, ..row("2026-09-14", 0) }, row("2026-09-13", 0)];
        let groups = trend_groups(&rows, &DIMENSIONS);
        assert_eq!(groups.len(), 5);
        for group in &groups {
            assert!(
                group.points[0].day == "2026-09-13"
                    && group.points[1].day == "2026-09-14"
                    && group.points[0].counts.len() == group.series.len(),
                "misaligned series: {group:?}"
            );
            assert!(group.points[0].counts.iter().all(|&c| c == 0), "missing observations invented: {group:?}");
        }
        assert_eq!(
            (
                groups[0].points[1].counts[1],
                groups[1].points[1].counts[1],
                groups[2].points[1].counts[2],
                groups[3].points[1].counts[3],
                groups[4].points[1].counts[1]
            ),
            (5, 8, 2, 3, 6)
        );
    }

    #[test]
    fn trend_json_matches_browser_contract() {
        let some = [Dimension { key: "appearance", values: &["nave", "apse"] }, Dimension { key: "prayer-form", values: PRAYER_FORMS }];
        let groups = trend_groups(&[row("2026-09-14", 0)], &some);
        assert_eq!(
            trend_json(&groups),
            r#"[{"Key":"appearance","Label":"Appearance","Series":["Nave","Apse"],"Points":[{"Day":"2026-09-14","Counts":[0,0]}]},{"Key":"prayer-form","Label":"Prayer form","Series":["Private","Deacon","Priest"],"Points":[{"Day":"2026-09-14","Counts":[0,0,0]}]}]"#
        );
        assert_eq!(
            trend_json(&trend_groups(&[], &DIMENSIONS[3..4])),
            r#"[{"Key":"client","Label":"Client","Series":["Browser","Web app","Android","iOS"],"Points":null}]"#
        );
    }
}
