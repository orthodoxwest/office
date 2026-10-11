//! Navigation and asset URLs. Appearance is client-side only, so no link carries a theme. The
//! words (season and day names, dates, the report issue) live in the `presentation` crate.

use calendar::Date;

/// A chrome link for `base` ("/", "/lauds", "/calendar", "/reminders"),
/// carrying the page's liturgical day so it hits the service worker's keys.
pub fn nav_link(base: &str, date: &str) -> String {
    match base {
        "/" => home_link(date),
        "/calendar" => calendar_link(date),
        "/reminders" => "/reminders".to_string(),
        _ => hour_link(base.strip_prefix('/').unwrap_or(base), date),
    }
}

/// The day's landing page, dated with `?date=` when a date is given.
pub fn home_link(date: &str) -> String {
    if date.is_empty() { "/".to_string() } else { format!("/?date={date}") }
}

/// An hour page, dated with a path segment when a date is given.
pub fn hour_link(hour: &str, date: &str) -> String {
    if date.is_empty() { format!("/{hour}") } else { format!("/{hour}/{date}") }
}

/// The ordo's page for the day's month, anchored at the day's row.
pub fn calendar_link(date: &str) -> String {
    match Date::parse(date) {
        Some(d) => format!("{}#d-{date}", calendar_month_link(d.year(), d.month())),
        None => "/calendar".to_string(),
    }
}

/// A year's ordo frontispiece: the Tabula Temporaria and its months.
pub fn calendar_year_link(year: i32) -> String {
    format!("/calendar/{year}")
}

/// One month of the ordo (`month` is 1-based).
pub fn calendar_month_link(year: i32, month: u32) -> String {
    format!("/calendar/{year}/{month:02}")
}

/// The whole year's ordo on one page, for print and search.
pub fn calendar_all_link(year: i32) -> String {
    format!("/calendar/{year}/all")
}

/// A build-stamped static asset path.
pub fn static_url(name: &str, version: &str) -> String {
    let name = name.strip_prefix('/').unwrap_or(name);
    if version.is_empty() { format!("/static/{name}") } else { format!("/static/{name}?v={version}") }
}

#[cfg(test)]
mod tests {
    use super::*;

    /// Chrome links carry the page's liturgical day, so they hit the service worker's keys.
    #[test]
    fn links() {
        const DATE: &str = "2026-06-07";
        for (base, want) in [
            ("/", "/?date=2026-06-07"),
            ("/lauds", "/lauds/2026-06-07"),
            ("/calendar", "/calendar/2026/06#d-2026-06-07"),
            ("/reminders", "/reminders"),
        ] {
            assert_eq!(nav_link(base, DATE), want, "nav_link({base:?})");
        }
        assert_eq!(nav_link("/lauds", ""), "/lauds");
        assert_eq!(nav_link("/calendar", ""), "/calendar");
        assert_eq!(calendar_month_link(2027, 1), "/calendar/2027/01");
        assert_eq!(calendar_year_link(2027), "/calendar/2027");
        assert_eq!(calendar_all_link(2027), "/calendar/2027/all");
        assert_eq!(static_url("style.css", "abc"), "/static/style.css?v=abc");
        assert_eq!(static_url("/fonts/x.woff2", "v1"), "/static/fonts/x.woff2?v=v1");
        assert_eq!(static_url("app.js", ""), "/static/app.js");
    }
}
