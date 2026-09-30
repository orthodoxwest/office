//! Navigation and asset URLs. Appearance is client-side only, so no link carries a theme.

use calendar::Date;
use liturgy::OfficeHour;

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

/// The season as an hour header names it, or nothing. A bare "Easter",
/// "Christmas", "Epiphany" or "Pentecost" beside a date reads as that feast
/// day, so the tides take their season names. The season after Pentecost is
/// left unnamed: "Time after Pentecost" is clumsy in a header line, and its
/// Sundays and feasts already name themselves.
pub fn season_label(season: &str) -> String {
    use calendar::Season::*;
    match calendar::Season::parse(season) {
        Ok(Christmas) => "Christmastide".into(),
        Ok(Epiphany) => "Epiphanytide".into(),
        Ok(Easter) => "Eastertide".into(),
        Ok(Pentecost) => String::new(),
        Ok(Advent | Septuagesima | Lent | Passiontide) | Err(_) => title_case(season),
    }
}

/// The GitHub new-issue endpoint behind "Report a problem".
const REPO_ISSUES_URL: &str = "https://github.com/orthodoxwest/office/issues/new";

/// Escapes a query component; spaces become `+`.
pub fn query_escape(s: &str) -> String {
    let mut out = String::with_capacity(s.len());
    for &c in s.as_bytes() {
        match c {
            b' ' => out.push('+'),
            c if c.is_ascii_alphanumeric() || matches!(c, b'-' | b'_' | b'.' | b'~') => out.push(c as char),
            c => out.push_str(&format!("%{c:02X}")),
        }
    }
    out
}

/// `url.Values{title, body, labels=review}.Encode()`: keys sorted.
fn issue_url(title: &str, body: &str) -> String {
    format!("{REPO_ISSUES_URL}?body={}&labels=review&title={}", query_escape(body), query_escape(title))
}

pub fn season_str(hour: &OfficeHour) -> &'static str {
    hour.season.map(|s| s.as_str()).unwrap_or("")
}

fn celebration(hour: &OfficeHour) -> String {
    if hour.feast.is_empty() { format!("{} feria", title_case(season_str(hour))) } else { hour.feast.clone() }
}

/// A prefilled issue identifying the exact page under review.
pub fn report_url(hour: &OfficeHour, hour_name: &str, date_slug: &str) -> String {
    let celebration = celebration(hour);
    let title = format!("[review] {} — {date_slug} ({celebration})", hour.title);
    let body = format!(
        "**Page:** /{hour_name}/{date_slug}?form={form}
**Prayer form:** {label}
**Celebration:** {celebration}
**Season:** {season}

**Category** (check all that apply):
- [ ] Missing proper — the app shows a generic/ordinary text where the diurnal or archdiocese supplement has a specific one
- [ ] Incorrect translation — wording differs from our diocesan books
- [ ] Logic or rubric error — wrong structure, missing or extra element, wrong psalms/antiphons for the day

**What the books say** (cite diurnal/supplement page if possible):

**What the app shows:**

",
        form = hour.form.as_str(),
        label = hour.form.label(),
        season = title_case(season_str(hour)),
    );
    issue_url(&title, &body)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn query_escape_encodes_special_characters() {
        assert_eq!(query_escape("a b/c?d=é—*"), "a+b%2Fc%3Fd%3D%C3%A9%E2%80%94%2A");
    }

    #[test]
    fn season_label_names_tides_and_leaves_pentecost_unnamed() {
        assert_eq!(season_label("easter"), "Eastertide");
        assert_eq!(season_label("christmas"), "Christmastide");
        assert_eq!(season_label("epiphany"), "Epiphanytide");
        assert_eq!(season_label("pentecost"), "");
        assert_eq!(season_label("lent"), "Lent");
        assert_eq!(season_label("passiontide"), "Passiontide");
        assert_eq!(season_label("septuagesima"), "Septuagesima");
        assert_eq!(season_label("advent"), "Advent");
        assert_eq!(season_label(""), "");
    }

    #[test]
    fn nav_link_keeps_chrome_dated() {
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
        assert_eq!(nav_link("/calendar", "2026-03-11"), "/calendar/2026/03#d-2026-03-11");
        assert_eq!(nav_link("/calendar", "2026-12-31"), "/calendar/2026/12#d-2026-12-31");
        assert_eq!(calendar_month_link(2027, 1), "/calendar/2027/01");
        assert_eq!(calendar_year_link(2027), "/calendar/2027");
        assert_eq!(calendar_all_link(2027), "/calendar/2027/all");
        assert_eq!(nav_link("/calendar", ""), "/calendar");
        assert_eq!(nav_link("/reminders", "2026-03-11"), "/reminders");
        assert_eq!(static_url("/style.css", "abc"), "/static/style.css?v=abc");
        assert_eq!(title_case("advent"), "Advent");
        assert_eq!(title_case(""), "");
    }
}
