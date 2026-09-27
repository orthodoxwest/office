//! Navigation and asset URLs. Ported from Go's `render/links.go`. Appearance
//! is client-side only, so no link carries a theme.

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

/// The year calendar, anchored at the day's row.
pub fn calendar_link(date: &str) -> String {
    match Date::parse(date) {
        Some(d) => format!("/calendar/{}#d-{date}", d.year()),
        None => "/calendar".to_string(),
    }
}

/// A calendar year without a day anchor.
pub fn calendar_year_link(year: i32) -> String {
    format!("/calendar/{year}")
}

/// A build-stamped static asset path.
pub fn static_url(name: &str, version: &str) -> String {
    let name = name.strip_prefix('/').unwrap_or(name);
    if version.is_empty() { format!("/static/{name}") } else { format!("/static/{name}?v={version}") }
}

/// Capitalizes the first letter ("advent" → "Advent").
pub fn title_case(s: &str) -> String {
    let mut chars = s.chars();
    match chars.next() {
        Some(c) if c.is_ascii() => format!("{}{}", c.to_ascii_uppercase(), chars.as_str()),
        _ => s.to_string(),
    }
}

/// The ornament body class for a season: Passiontide veils the gold,
/// Paschaltide warms it, and every other season keeps the ordinary gold.
pub fn season_class(season: Option<calendar::Season>) -> &'static str {
    match season {
        Some(calendar::Season::Passiontide) => "season-passiontide",
        Some(calendar::Season::Easter) => "season-eastertide",
        _ => "",
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    // Ported from Go's `internal/render/links_test.go`.
    #[test]
    fn nav_link_keeps_chrome_dated() {
        const DATE: &str = "2026-06-07";
        for (base, want) in [
            ("/", "/?date=2026-06-07"),
            ("/lauds", "/lauds/2026-06-07"),
            ("/calendar", "/calendar/2026#d-2026-06-07"),
            ("/reminders", "/reminders"),
        ] {
            assert_eq!(nav_link(base, DATE), want, "nav_link({base:?})");
        }
        assert_eq!(nav_link("/lauds", ""), "/lauds");
    }

    #[test]
    fn static_urls() {
        assert_eq!(static_url("style.css", "abc"), "/static/style.css?v=abc");
        assert_eq!(static_url("/fonts/x.woff2", "v1"), "/static/fonts/x.woff2?v=v1");
        assert_eq!(static_url("app.js", ""), "/static/app.js");
    }

    #[test]
    fn links() {
        assert_eq!(nav_link("/", "2026-03-11"), "/?date=2026-03-11");
        assert_eq!(nav_link("/lauds", "2026-03-11"), "/lauds/2026-03-11");
        assert_eq!(nav_link("/calendar", "2026-03-11"), "/calendar/2026#d-2026-03-11");
        assert_eq!(nav_link("/calendar", ""), "/calendar");
        assert_eq!(nav_link("/reminders", "2026-03-11"), "/reminders");
        assert_eq!(static_url("/style.css", "abc"), "/static/style.css?v=abc");
        assert_eq!(title_case("advent"), "Advent");
        assert_eq!(title_case(""), "");
    }
}
