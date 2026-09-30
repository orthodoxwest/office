//! Page handlers: resolve the request to a liturgical day, drive the engine, and fill the view
//! models.

use axum::body::Body;
use axum::http::{HeaderMap, HeaderValue, Method, Response, StatusCode, header};
use calendar::{MoveableDates, Tabula};
use jiff::tz::TimeZone;
use liturgy::{OfficeHour, PrayerForm};
use office::day::Day;
use office::summary::{CommSummary, ordo_day};
use office::{ComposeOptions, Engine};
use render_html::links::{calendar_all_link, calendar_link, calendar_month_link, calendar_year_link, home_link, hour_link};
use render_html::view::{
    CalendarData, Chrome, CommemorationRow, DayRow, ErrorData, HomeData, HomeHourLink, HourData, HourHeader, LeaderForm, MonthData,
    MonthLink, MonthStep, NotFoundData, ReminderDay, ReminderHour, RemindersData, TabulaData, TabulaRow,
};

use crate::Server;
use crate::http::{Query, cookie, redirect, response, set};
use crate::web_time::{load_location, local, now_in, parse_date};
use presentation::{MONTHS, date_slug, day_heading, day_name, invitation, long_date, month_name, report_url, season_class, season_str};

/// What a page handler reads from the request.
pub struct Req<'a> {
    pub method: &'a Method,
    pub headers: &'a HeaderMap,
    /// The decoded path (`r.URL.Path`).
    pub path: &'a str,
    pub query: &'a Query,
}

const VALID_HOURS: [&str; 7] = ["lauds", "prime", "terce", "sext", "none", "vespers", "compline"];

const ORDERED_HOURS: [(&str, &str); 7] = [
    ("Lauds", "lauds"),
    ("Prime", "prime"),
    ("Terce", "terce"),
    ("Sext", "sext"),
    ("None", "none"),
    ("Vespers", "vespers"),
    ("Compline", "compline"),
];

fn build_home_hours(date_slug: &str, current: &str) -> Vec<HomeHourLink> {
    ORDERED_HOURS
        .iter()
        .map(|(name, slug)| HomeHourLink {
            name: name.to_string(),
            slug: slug.to_string(),
            url: hour_link(slug, date_slug),
            is_current: *slug == current,
        })
        .collect()
}

fn adjacent_hours(hour: &str, date: &str) -> (String, String, String, String) {
    let mut out = (String::new(), String::new(), String::new(), String::new());
    if let Some(i) = ORDERED_HOURS.iter().position(|(_, slug)| *slug == hour) {
        if i > 0 {
            out.0 = ORDERED_HOURS[i - 1].0.to_string();
            out.1 = hour_link(ORDERED_HOURS[i - 1].1, date);
        }
        if i + 1 < ORDERED_HOURS.len() {
            out.2 = ORDERED_HOURS[i + 1].0.to_string();
            out.3 = hour_link(ORDERED_HOURS[i + 1].1, date);
        }
    }
    out
}

/// An HTML page that revalidates on every request.
fn html(status: StatusCode, body: String) -> Response<Body> {
    let mut resp = response(status, body);
    set(&mut resp, header::CONTENT_TYPE, "text/html; charset=utf-8");
    set(&mut resp, header::CACHE_CONTROL, "no-cache");
    resp
}

fn render_failed(e: &str) -> Response<Body> {
    crate::http::http_error(e, StatusCode::INTERNAL_SERVER_ERROR)
}

fn comm_rows(comms: &[CommSummary]) -> Vec<CommemorationRow> {
    comms.iter().map(|c| CommemorationRow { name: c.name.clone(), incipit: c.incipit.clone() }).collect()
}

/// One month of the ordo: each day's row with the composed Lauds, Hours,
/// and Vespers digest. `days` are the month's days in order.
pub fn build_month(days: &[Day], engine: &Engine, moveable: &MoveableDates) -> MonthData {
    let name = days.first().map(|d| month_name(d.date)).unwrap_or_default();
    let mut month = MonthData { name: name.to_string(), slug: name.to_lowercase(), days: Vec::new() };
    for d in days {
        let o = ordo_day(d, engine, moveable);
        let mut row = DayRow {
            day_num: d.date.day(),
            weekday: d.date.weekday().name()[..3].to_string(),
            date_slug: date_slug(d.date),
            rank: o.rank,
            rank_full: o.rank_full,
            color: o.color.as_str().to_string(),
            color_class: format!("day-color-{}", o.color.as_str()),
            feast_name: day_name(d),
            fast: o.fast,
            abstinence: o.abstinence,
            commemorations: o.commemorations,
            hours_preces: o.hours_preces,
            vespers_note: o.vespers_note,
            ..DayRow::default()
        };
        if let Some(lauds) = o.lauds {
            row.benedictus_antiphon = lauds.gospel_ant;
            row.lauds_preces = lauds.preces;
            row.lauds_suffrage = lauds.suffrage;
            row.lauds_comms = comm_rows(&lauds.comms);
        }
        if let Some(vespers) = o.vespers {
            row.magnificat_antiphon = vespers.gospel_ant;
            row.vespers_preces = vespers.preces;
            row.vespers_suffrage = vespers.suffrage;
            row.vespers_comms = comm_rows(&vespers.comms);
        }
        month.days.push(row);
    }
    month
}

fn invalid_date(s: &str) -> String {
    format!("Invalid date {} — please use YYYY-MM-DD format.", data_format::quote(s))
}

/// Which ordo page a URL names.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
enum OrdoView {
    Year,
    Month(u32),
    All,
}

impl OrdoView {
    fn name(self) -> &'static str {
        match self {
            OrdoView::Year => "year",
            OrdoView::Month(_) => "month",
            OrdoView::All => "all",
        }
    }
}

/// A month path segment: exactly two digits, "01" to "12".
fn month_segment(s: &str) -> Option<u32> {
    if s.len() != 2 || !s.bytes().all(|b| b.is_ascii_digit()) {
        return None;
    }
    s.parse().ok().filter(|m| (1..=12).contains(m))
}

/// A month named the way a reader might type it, "9" or "September", for a
/// redirect to its canonical two-digit page.
fn loose_month(s: &str) -> Option<u32> {
    if let Ok(m) = s.parse::<u32>()
        && s.len() == 1
        && m >= 1
    {
        return Some(m);
    }
    MONTHS.iter().zip(1..).find(|(name, _)| name.eq_ignore_ascii_case(s) || name[..3].eq_ignore_ascii_case(s)).map(|(_, m)| m)
}

/// The Tabula Temporaria as the printed ordo opens: the year's figures, its
/// moveable feasts, and its Ember days, each date leading to its row.
fn tabula(year: i32) -> TabulaData {
    let t = Tabula::compute(year);
    let moveable = MoveableDates::compute(year);
    let figure = |label: &str, value: String| TabulaRow { label: label.into(), value, href: String::new() };
    let date = |label: &str, d: calendar::Date| TabulaRow {
        label: label.into(),
        value: format!("{} {}", month_name(d), d.day()),
        href: calendar_link(&date_slug(d)),
    };
    let ember = |label: &str, e: &calendar::computus::EmberSet| TabulaRow {
        label: label.into(),
        value: format!("{} {}, {}, {}", month_name(e.wed), e.wed.day(), e.fri.day(), e.sat.day()),
        href: calendar_link(&date_slug(e.wed)),
    };
    TabulaData {
        figures: vec![
            figure("Golden Number", calendar::computus::roman(t.golden_number)),
            figure("Dominical Letter", t.dominical_letter.to_string()),
            figure("Sundays after Epiphany", t.sundays_after_epiphany.to_string()),
            figure("Sundays after Pentecost", t.sundays_after_pentecost.to_string()),
        ],
        moveable: vec![
            date("Septuagesima Sunday", moveable.septuagesima),
            date("Ash Wednesday", moveable.ash_wednesday),
            date("Easter Day", moveable.easter),
            date("Ascension Day", moveable.ascension),
            date("Pentecost", moveable.pentecost),
            date("Corpus Christi", moveable.corpus_christi),
            date("Advent Sunday", moveable.advent1),
        ],
        ember: vec![
            ember("Spring (Lent)", &t.spring),
            ember("Summer (Whitsun)", &t.summer),
            ember("Autumn (Holy Cross)", &t.autumn),
            ember("Winter (Advent)", &t.winter),
        ],
    }
}

impl Server {
    /// The zone in the browser's `tz` cookie, else the host zone.
    fn user_location(&self, req: &Req) -> TimeZone {
        match cookie(req.headers, "tz") {
            Some(name) => load_location(&name).unwrap_or_else(local),
            None => local(),
        }
    }

    /// Local today, for chrome links on pages without a liturgical day.
    fn nav_date_now(&self, req: &Req) -> String {
        date_slug(now_in(&self.user_location(req)).0)
    }

    /// The styled page for a 4xx or 5xx condition.
    pub fn error_page(&self, req: &Req, status: StatusCode, message: &str) -> Response<Body> {
        let data = ErrorData {
            chrome: Chrome { nav_date: self.nav_date_now(req), ..Chrome::default() },
            title: status.canonical_reason().unwrap_or("").to_string(),
            message: message.to_string(),
        };
        match self.pages.error_page(&data) {
            Ok(body) => html(status, body),
            // Preserve the response status if the error page itself cannot render.
            Err(_) => html(status, String::new()),
        }
    }

    pub fn not_found_page(&self, req: &Req) -> Response<Body> {
        let data = NotFoundData { chrome: Chrome { nav_date: self.nav_date_now(req), ..Chrome::default() } };
        match self.pages.not_found(&data) {
            Ok(body) => html(StatusCode::NOT_FOUND, body),
            Err(e) => render_failed(&e),
        }
    }

    /// "/" is the day page, "/{hour}" and "/{hour}/{date}" are hours.
    pub fn root(&self, req: &Req) -> Response<Body> {
        let path = req.path.trim_matches('/');
        if path.is_empty() {
            return self.home(req);
        }
        let parts: Vec<&str> = path.split('/').collect();
        match parts.as_slice() {
            [hour] => self.hour(req, hour, ""),
            [hour, date] => self.hour(req, hour, date),
            _ => self.not_found_page(req),
        }
    }

    fn home(&self, req: &Req) -> Response<Body> {
        let loc = self.user_location(req);
        let ds = req.query.get("date");
        let date = if ds.is_empty() {
            now_in(&loc).0
        } else {
            match parse_date(ds) {
                Some(d) => d,
                None => return self.error_page(req, StatusCode::BAD_REQUEST, &invalid_date(ds)),
            }
        };
        let slug = date_slug(date);
        let entry = match self.cache.get(date.year()) {
            Ok(e) => e,
            Err(e) => return self.error_page(req, StatusCode::INTERNAL_SERVER_ERROR, &format!("error building calendar: {e}")),
        };
        let Some(day) = entry.days.get(date.ordinal() as usize - 1) else {
            return self.error_page(req, StatusCode::INTERNAL_SERVER_ERROR, "date out of range");
        };

        let heading = day_heading(day);
        let (now, now_hour) = now_in(&loc);
        let now_slug = date_slug(now);
        let invite = invitation(date, now, now_hour);
        let data = HomeData {
            chrome: Chrome {
                page: "home".into(),
                nav_date: slug.clone(),
                usage_when: slug.clone(),
                // The ornament follows the day itself, not the display season.
                season_class: season_class(Some(day.season)).into(),
                show_today: slug != now_slug,
            },
            date_str: long_date(date),
            date_slug: slug.clone(),
            prev_link: home_link(&date_slug(date.add_days(-1))),
            next_link: home_link(&date_slug(date.add_days(1))),
            today_link: home_link(&now_slug),
            feast_name: heading.feast,
            commemorations: day.commemorations.iter().map(|c| c.name.clone()).collect(),
            color: day.color.as_str().into(),
            season: heading.season,
            octave_note: heading.octave_note,
            penitential: day.penitential.labels().into_iter().map(String::from).collect(),
            calendar_link: calendar_link(&slug),
            pray_now_label: invite.label,
            pray_now_link: hour_link(invite.hour, &date_slug(invite.date)),
            hours: build_home_hours(&slug, invite.current),
        };
        match self.pages.home(&data) {
            Ok(body) => html(StatusCode::OK, body),
            Err(e) => render_failed(&e),
        }
    }

    fn hour(&self, req: &Req, hour_name: &str, date_str: &str) -> Response<Body> {
        if !VALID_HOURS.contains(&hour_name) {
            return self.not_found_page(req);
        }
        let loc = self.user_location(req);
        let (date, date_str) = if date_str.is_empty() {
            let ds = req.query.get("date");
            if ds.is_empty() {
                let d = now_in(&loc).0;
                (d, date_slug(d))
            } else {
                match parse_date(ds) {
                    Some(d) => (d, ds.to_string()),
                    None => return self.error_page(req, StatusCode::BAD_REQUEST, &invalid_date(ds)),
                }
            }
        } else {
            match parse_date(date_str) {
                Some(d) => (d, date_str.to_string()),
                None => return self.error_page(req, StatusCode::BAD_REQUEST, &invalid_date(date_str)),
            }
        };
        let entry = match self.cache.get(date.year()) {
            Ok(e) => e,
            Err(e) => return self.error_page(req, StatusCode::INTERNAL_SERVER_ERROR, &format!("error building calendar: {e}")),
        };
        let Some(day) = entry.days.get(date.ordinal() as usize - 1) else {
            return self.error_page(req, StatusCode::BAD_REQUEST, "That date is outside the supported range.");
        };
        let preview = req.query.get("preview") == "martyrology" && hour_name == "prime";
        let compose = |form| {
            self.engine.compose_hour_with_options(hour_name, day, &entry.moveable, &ComposeOptions { form, martyrology_preview: preview })
        };
        let hour = match compose(PrayerForm::Private) {
            Ok(h) => h,
            Err(e) => return self.error_page(req, StatusCode::INTERNAL_SERVER_ERROR, &format!("error composing hour: {e}")),
        };
        let mut composed = vec![(PrayerForm::Private, hour.clone())];
        for form in [PrayerForm::Deacon, PrayerForm::Priest] {
            match compose(form) {
                Ok(h) => composed.push((form, h)),
                Err(_) => return self.error_page(req, StatusCode::INTERNAL_SERVER_ERROR, "Unable to compose the selected office form."),
            }
        }

        let (previous_hour_name, previous_hour_link, next_hour_name, next_hour_link) = adjacent_hours(hour_name, &date_str);
        let today_slug = date_slug(now_in(&loc).0);
        let mut data = HourData {
            chrome: Chrome {
                page: hour_name.into(),
                nav_date: date_str.clone(),
                usage_when: date_str.clone(),
                // The hour's own season: I Vespers of Easter on Holy Saturday
                // is unveiled while that day's Lauds is still veiled.
                season_class: season_class(hour.season).into(),
                show_today: date_str != today_slug,
            },
            leader_forms: composed
                .iter()
                .map(|(form, h)| LeaderForm {
                    form: form.as_str().into(),
                    label: form.label().into(),
                    report_url: report_url(h, hour_name, &date_str),
                })
                .collect(),
            hour_name: hour_name.into(),
            date_str: long_date(date),
            date_slug: date_str.clone(),
            prev_link: hour_link(hour_name, &date_slug(date.add_days(-1))),
            next_link: hour_link(hour_name, &date_slug(date.add_days(1))),
            today_link: hour_link(hour_name, &today_slug),
            day_link: home_link(&date_str),
            previous_hour_name,
            previous_hour_link,
            next_hour_name,
            next_hour_link,
            hour: HourHeader {
                title: hour.title.clone(),
                color: hour.color.map(|c| c.as_str()).unwrap_or("").into(),
                feast: hour.feast.clone(),
                season: season_str(&hour).into(),
            },
            report_url: report_url(&hour, hour_name, &date_str),
            ..HourData::default()
        };
        let forms: Vec<(PrayerForm, &OfficeHour)> = composed.iter().map(|(f, h)| (*f, h)).collect();
        let mut resp = match self.pages.hour(&mut data, &forms) {
            Ok(body) => html(StatusCode::OK, body),
            Err(e) => return render_failed(&e),
        };
        if req.query.has("preview") {
            set(&mut resp, header::CACHE_CONTROL, "private, no-store");
            resp.headers_mut().insert("x-robots-tag", HeaderValue::from_static("noindex, nofollow"));
        }
        resp
    }

    /// The ordo: "/calendar" leads to today's month; "/calendar/{year}" is
    /// the year's frontispiece; "/calendar/{year}/{MM}" one month; and
    /// "/calendar/{year}/all" the whole year on one page.
    pub fn calendar(&self, req: &Req) -> Response<Body> {
        let path = req.path.trim_matches('/');
        let parts: Vec<&str> = path.split('/').collect();
        if parts.len() > 3 {
            return self.not_found_page(req);
        }
        let now = now_in(&self.user_location(req)).0;
        if parts.len() == 1 {
            // No year: today's row in the current month.
            let form = req.query.get("form");
            let form_query = match PrayerForm::parse(form) {
                Ok(f) if !form.is_empty() => format!("?form={}", f.as_str()),
                _ => String::new(),
            };
            let target = format!("{}{form_query}#d-{}", calendar_month_link(now.year(), now.month()), date_slug(now));
            return redirect(&target, StatusCode::FOUND);
        }
        let year = match data_format::atoi(parts[1]) {
            Ok(y) if (1..=9999).contains(&y) => y as i32,
            // Beneath a year that is not one, there is no page at all.
            _ if parts.len() == 3 => return self.not_found_page(req),
            _ => return self.error_page(req, StatusCode::BAD_REQUEST, &format!("Invalid year {}.", data_format::quote(parts[1]))),
        };
        let view = match parts.get(2) {
            None => OrdoView::Year,
            Some(&"all") => OrdoView::All,
            Some(m) => match month_segment(m) {
                Some(m) => OrdoView::Month(m),
                // "9" or "september" is the reader's way of naming a month.
                None => match loose_month(m) {
                    Some(m) => return redirect(&calendar_month_link(year, m), StatusCode::MOVED_PERMANENTLY),
                    None => return self.not_found_page(req),
                },
            },
        };
        let (months, current) = match view {
            OrdoView::Year => (Ok(Vec::new()), None),
            OrdoView::Month(m) => (self.cache.month(year, m, &self.engine).map(|m| vec![m]), Some(m)),
            OrdoView::All => (self.cache.months(year, &self.engine), None),
        };
        let months = match months {
            Ok(m) => m,
            Err(e) => return self.error_page(req, StatusCode::INTERNAL_SERVER_ERROR, &format!("error building calendar: {e}")),
        };
        let same_view = |y: i32| match view {
            OrdoView::Year => calendar_year_link(y),
            OrdoView::Month(m) => calendar_month_link(y, m),
            OrdoView::All => calendar_all_link(y),
        };
        let step = |y: i32, m: u32| MonthStep {
            name: if y == year { MONTHS[m as usize - 1].to_string() } else { format!("{} {y}", MONTHS[m as usize - 1]) },
            href: calendar_month_link(y, m),
        };
        let data = CalendarData {
            chrome: Chrome { page: "calendar".into(), nav_date: date_slug(now), usage_when: year.to_string(), ..Chrome::default() },
            year,
            view: view.name().into(),
            year_roman: if year <= 3999 { calendar::computus::roman(year) } else { String::new() },
            prev_year: year - 1,
            next_year: year + 1,
            prev_year_link: same_view(year - 1),
            next_year_link: same_view(year + 1),
            year_link: calendar_year_link(year),
            all_link: calendar_all_link(year),
            month_links: MONTHS
                .iter()
                .zip(1..)
                .map(|(name, m)| {
                    let today = (year, m) == (now.year(), now.month());
                    MonthLink {
                        name: if today { format!("{name}, this month") } else { name.to_string() },
                        abbr: name[..3].to_string(),
                        // The whole year jumps within itself; other views lead to month pages.
                        href: if view == OrdoView::All { format!("#{}", name.to_lowercase()) } else { calendar_month_link(year, m) },
                        month: format!("{year:04}-{m:02}"),
                        current: current == Some(m),
                        today,
                    }
                })
                .collect(),
            months,
            prev_month: current.and_then(|m| match m {
                1 if year > 1 => Some(step(year - 1, 12)),
                1 => None,
                m => Some(step(year, m - 1)),
            }),
            next_month: current.and_then(|m| match m {
                12 if year < 9999 => Some(step(year + 1, 1)),
                12 => None,
                m => Some(step(year, m + 1)),
            }),
            tabula: (view == OrdoView::Year).then(|| tabula(year)),
        };
        match self.pages.calendar(&data) {
            Ok(body) => html(StatusCode::OK, body),
            Err(e) => render_failed(&e),
        }
    }

    pub fn reminders(&self, req: &Req) -> Response<Body> {
        let hour = |name: &str, slug: &str, default: &str, checked: bool| ReminderHour {
            name: name.into(),
            slug: slug.into(),
            default: default.into(),
            checked,
        };
        let data = RemindersData {
            chrome: Chrome { page: "reminders".into(), nav_date: self.nav_date_now(req), ..Chrome::default() },
            hours: vec![
                hour("Lauds", "lauds", "06:45", true),
                hour("Prime", "prime", "07:30", false),
                hour("Terce", "terce", "09:00", false),
                hour("Sext", "sext", "12:00", false),
                hour("None", "none", "15:00", false),
                hour("Vespers", "vespers", "18:00", true),
                hour("Compline", "compline", "21:00", true),
            ],
            days: ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"]
                .iter()
                .map(|d| ReminderDay { name: d.to_string(), slug: d.to_lowercase() })
                .collect(),
        };
        match self.pages.reminders(&data) {
            Ok(body) => html(StatusCode::OK, body),
            Err(e) => render_failed(&e),
        }
    }

    /// The civil date of now in the host zone, for the startup pre-warm.
    pub fn local_year() -> i32 {
        now_in(&local()).0.year()
    }
}

/// A day's display name for the reminder feed.
pub fn celebration_name(day: &Day) -> String {
    day_name(day)
}

#[cfg(test)]
mod tests {
    use super::*;
    use presentation::CURRENT_HOUR_SCHEDULE;

    // app.js mirrors the schedule so a cached home page can update itself.
    #[test]
    fn client_office_schedule_matches_server() {
        let src = std::str::from_utf8(crate::pwa::file("static/app.js").unwrap()).unwrap();
        let client: Vec<(i8, String, String, i32)> = src
            .split("{ start: ")
            .skip(1)
            .map(|rest| {
                let entry = &rest[..rest.find(" }").unwrap()];
                let field = |name: &str| {
                    let at = entry.find(&format!("{name}: ")).unwrap() + name.len() + 2;
                    entry[at..].split(',').next().unwrap().trim_matches('"').to_string()
                };
                let start = entry.split(',').next().unwrap().parse().unwrap();
                (start, field("slug"), field("label"), field("offset").parse().unwrap())
            })
            .collect();
        assert_eq!(client.len(), CURRENT_HOUR_SCHEDULE.len(), "client and server boundaries");
        for (got, want) in client.iter().zip(CURRENT_HOUR_SCHEDULE) {
            assert_eq!((got.0, got.1.as_str(), got.2.as_str(), got.3), want);
        }
    }

    use axum::http::Uri;

    use crate::test_server;

    /// A GET through the whole server, as the mux dispatches it.
    fn get(path: &str) -> (StatusCode, HeaderMap, String) {
        use tower::ServiceExt;
        let runtime = tokio::runtime::Builder::new_current_thread().build().unwrap();
        let request = axum::extract::Request::builder().uri(path.parse::<Uri>().unwrap()).body(Body::empty()).unwrap();
        let resp = runtime.block_on(test_server().router().oneshot(request)).unwrap();
        let (parts, body) = resp.into_parts();
        let bytes = runtime.block_on(axum::body::to_bytes(body, usize::MAX)).unwrap();
        (parts.status, parts.headers, String::from_utf8(bytes.to_vec()).unwrap())
    }

    /// The page ends with the continuation links and each form's report
    /// link; no review metadata, source keys, or local paths.
    #[test]
    fn hour_page_ends_with_reporting_not_review_metadata() {
        let (status, _, body) = get("/lauds/2026-06-07");
        assert_eq!(status, StatusCode::OK);
        for want in [r#"<details class="site-menu">"#, r#"class="today-link""#, r#"class="hour-continuation""#, "Report a problem"] {
            assert!(body.contains(want), "hour page missing {want:?}");
        }
        assert_eq!(body.matches(r#"class="report-issue""#).count(), 3, "one report link per prayer form");
        for unwanted in ["assurance", "Text dependencies", "Composition decisions", "SOURCE:", ".txt", "/home/", "../resources"] {
            assert!(!body.contains(unwanted) && !body.contains(&unwanted.replace('/', "&#x2f;")), "hour page contains {unwanted:?}");
        }
    }

    #[test]
    fn adjacent_hours_keep_date() {
        let s = |v: (String, String, String, String)| v;
        assert_eq!(
            s(adjacent_hours("sext", "2026-06-07")),
            ("Terce".into(), "/terce/2026-06-07".into(), "None".into(), "/none/2026-06-07".into())
        );
        let (prev, prev_link, _, _) = adjacent_hours("lauds", "2026-06-07");
        assert!(prev.is_empty() && prev_link.is_empty());
        let (_, _, next, next_link) = adjacent_hours("compline", "2026-06-07");
        assert!(next.is_empty() && next_link.is_empty());
    }

    #[test]
    fn calendar_rejects_extra_path_segments() {
        for path in [
            "/calendar/2026/extra",
            "/calendar/not-a-year/extra",
            "/calendar/not-a-year/09",
            "/calendar/2026/0",
            "/calendar/2026/00",
            "/calendar/2026/13",
            "/calendar/2026/+9",
            "/calendar/2026/09/extra",
        ] {
            assert_eq!(get(path).0, StatusCode::NOT_FOUND, "{path}");
        }
        assert_eq!(get("/calendar/not-a-year").0, StatusCode::BAD_REQUEST);
    }

    /// A month typed as a reader would name it finds its page.
    #[test]
    fn loose_months_redirect_to_their_page() {
        for path in ["/calendar/2026/9", "/calendar/2026/september", "/calendar/2026/SEPTEMBER", "/calendar/2026/Sep"] {
            let (status, headers, _) = get(path);
            assert_eq!(status, StatusCode::MOVED_PERMANENTLY, "{path}");
            assert_eq!(headers[header::LOCATION], "/calendar/2026/09", "{path}");
        }
        assert_eq!(get("/calendar/2026/may").1[header::LOCATION], "/calendar/2026/05");
    }

    /// An ordo page with its attribute-escaped slashes restored.
    fn ordo(path: &str) -> String {
        let (status, headers, body) = get(path);
        assert_eq!(status, StatusCode::OK, "{path}");
        assert_eq!(headers[header::CACHE_CONTROL], "no-cache");
        body.replace("&#x2f;", "/")
    }

    #[test]
    fn ordo_month_page_holds_one_month_between_its_neighbours() {
        let body = ordo("/calendar/2026/09");
        assert!(body.contains("<title>September 2026 — Ordo</title>"));
        assert_eq!(body.matches(r#"<tr class="day "#).count(), 30);
        assert!(body.contains(r#"id="d-2026-09-01""#) && body.contains(r#"id="d-2026-09-30""#));
        assert!(!body.contains(r#"id="d-2026-08-31""#) && !body.contains(r#"id="d-2026-10-01""#));
        // The strip leads to each month, naming this one as the current page.
        let strip = body.split(r#"<nav class="month-jump""#).nth(1).and_then(|s| s.split("</nav>").next()).unwrap();
        assert_eq!(strip.matches(r#"<a href="/calendar/2026/"#).count(), 12);
        assert!(strip.contains(r#"<a href="/calendar/2026/09" aria-label="September"#) && strip.contains(r#"aria-current="page">Sep</a>"#));
        assert_eq!(strip.matches(r#"aria-current="page""#).count(), 1);
        assert!(strip.contains(r#"data-month="2026-09""#));
        assert_eq!(body.matches(r#"aria-current="page""#).count(), 2, "the month and the Ordo menu item");
        assert!(body.contains(r#"href="/calendar/2025/09" aria-label="Previous year, 2025""#));
        assert!(body.contains(r#"href="/calendar/2027/09" aria-label="Next year, 2027""#));
        for want in [
            r#"href="/calendar/2026/08"><span class="continuation-label">Previous month</span><span>&larr; August</span>"#,
            r#"href="/calendar/2026/10"><span class="continuation-label">Next month</span><span>October &rarr;</span>"#,
            r#"<a class="day-hours-link" href="/calendar/2026">2026 Ordo</a>"#,
        ] {
            assert!(body.contains(want), "month continuation missing {want:?}");
        }
        // Neighbours across the year name their year.
        assert!(
            ordo("/calendar/2026/12")
                .contains(r#"href="/calendar/2027/01"><span class="continuation-label">Next month</span><span>January 2027 &rarr;</span>"#)
        );
        assert!(ordo("/calendar/2026/01").contains(
            r#"href="/calendar/2025/12"><span class="continuation-label">Previous month</span><span>&larr; December 2025</span>"#
        ));
        assert!(!ordo("/calendar/0001/01").contains("Previous month"));
        assert!(!ordo("/calendar/9999/12").contains("Next month"));
    }

    #[test]
    fn ordo_frontispiece_sets_out_the_tabula() {
        let body = ordo("/calendar/2026");
        assert!(body.contains("<title>Ordo 2026</title>") && body.contains("Tabula Temporaria"));
        let t = Tabula::compute(2026);
        for want in [
            format!("<dt>Golden Number</dt><dd>{}</dd>", calendar::computus::roman(t.golden_number)),
            format!("<dt>Dominical Letter</dt><dd>{}</dd>", t.dominical_letter),
            "<dt>Easter Day</dt><dd><a href=\"/calendar/2026/04#d-2026-04-12\">April 12</a></dd>".to_string(),
            "<dt>Autumn (Holy Cross)</dt><dd><a href=\"/calendar/2026/09#d-2026-09-16\">September 16, 18, 19</a></dd>".to_string(),
        ] {
            assert!(body.contains(&want), "frontispiece missing {want:?}");
        }
        assert!(body.contains("<p class=\"calendar-subtitle\">Anno Domini MMXXVI</p>"));
        assert!(body.contains(r#"<a class="calendar-whole-year" href="/calendar/2026/all">The whole year on one page</a>"#));
        // Months are a page away; none is current here, and no rows render.
        assert!(body.contains(r#"<a href="/calendar/2026/01" aria-label="January""#));
        assert!(!body.contains(r#"aria-current="page">Jan"#));
        assert!(!body.contains(r#"<tr class="day "#) && !body.contains("Previous month"));
        assert!(body.contains(r#"href="/calendar/2025" aria-label="Previous year, 2025""#));
    }

    /// Today's month carries the strip's lozenge and says so, in its own
    /// year only; the client moves both at midnight.
    #[test]
    fn ordo_strip_marks_this_month() {
        let now = now_in(&local()).0;
        let this_month = format!("data-month=\"{}-{:02}\"", now.year(), now.month());
        let body = ordo(&format!("/calendar/{}", now.year()));
        assert_eq!(body.matches("is-today-month").count(), 1);
        let link = body.split("<a ").find(|a| a.contains(&this_month)).unwrap();
        assert!(link.contains(r#"class="is-today-month""#) && link.contains(", this month\""), "{link}");
        assert!(!ordo(&format!("/calendar/{}", now.year() - 1)).contains("is-today-month"));
        assert!(!ordo(&format!("/calendar/{}", now.year() + 1)).contains("this month"));
    }

    #[test]
    fn ordo_whole_year_keeps_every_day_and_jumps_within_itself() {
        let body = ordo("/calendar/2026/all");
        assert!(body.contains("<title>Ordo 2026 — Whole year</title>"));
        assert_eq!(body.matches(r#"<tr class="day "#).count(), 365);
        assert!(body.contains(r##"<a href="#january" aria-label="January""##));
        assert!(body.contains(r#"<section class="month" id="december">"#));
        assert!(body.contains(r#"href="/calendar/2027/all" aria-label="Next year, 2027""#));
        assert!(!body.contains("Previous month") && !body.contains("Tabula Temporaria"));
    }

    /// A feria still names the day before its commemorations.
    #[test]
    fn home_names_feria_before_commemorations() {
        let (status, headers, body) = get("/?date=2026-09-22");
        assert_eq!(status, StatusCode::OK);
        assert_eq!(headers[header::CACHE_CONTROL], "no-cache");
        let feast = body.find(r#"<p class="feast">Pentecost feria</p>"#).expect("names the feria");
        let also = body.find(r#"class="commemorations-label""#).expect("commemorations");
        assert!(feast < also);
        assert!(!body.contains(r#"class="home-season""#), "the season is not repeated");
    }

    fn body_classes(path: &str) -> Vec<String> {
        let (status, _, body) = get(path);
        assert_eq!(status, StatusCode::OK, "{path}");
        let after = body.split_once(r#"<body class=""#).expect("body class").1;
        after[..after.find('"').unwrap()].split_whitespace().map(String::from).collect()
    }

    /// Dates come from the Julian paschalion, not hand-written Easter dates.
    #[test]
    fn seasonal_ornament_class_follows_the_paschalion() {
        let m = MoveableDates::compute(2026);
        for (what, date, want) in [
            // Veiled: Passion Sunday through Holy Saturday.
            ("Passion Sunday", m.passion_sunday, "season-passiontide"),
            ("mid-Passiontide", m.passion_sunday.add_days(3), "season-passiontide"),
            ("Palm Sunday", m.passion_sunday.add_days(7), "season-passiontide"),
            ("Good Friday", m.good_friday, "season-passiontide"),
            ("Holy Saturday", m.holy_saturday, "season-passiontide"),
            // Lent proper keeps the ordinary gold.
            ("Saturday before Passion Sunday", m.passion_sunday.add_days(-1), ""),
            ("Ash Wednesday", m.ash_wednesday, ""),
            // Bright: Easter Sunday through the eve of Pentecost.
            ("Easter Sunday", m.easter, "season-eastertide"),
            ("Easter Monday", m.easter.add_days(1), "season-eastertide"),
            ("Low Sunday", m.easter.add_days(7), "season-eastertide"),
            ("Ascension", m.ascension, "season-eastertide"),
            ("eve of Pentecost", m.pentecost.add_days(-1), "season-eastertide"),
            ("Pentecost", m.pentecost, ""),
            ("Trinity Sunday", m.pentecost.add_days(7), ""),
        ] {
            let slug = date_slug(date);
            for path in [format!("/lauds/{slug}"), format!("/?date={slug}")] {
                let classes = body_classes(&path);
                for class in ["season-passiontide", "season-eastertide"] {
                    assert_eq!(classes.iter().any(|c| c == class), class == want, "{what} ({path}): {class} in {classes:?}");
                }
            }
        }
    }

    /// Holy Saturday's Vespers is I Vespers of Easter: the ornament follows
    /// the hour on hour pages and the day on the home page.
    #[test]
    fn holy_saturday_vespers_unveils_ahead_of_the_day() {
        let slug = date_slug(MoveableDates::compute(2026).holy_saturday);
        assert!(body_classes(&format!("/lauds/{slug}")).contains(&"season-passiontide".into()));
        assert!(body_classes(&format!("/vespers/{slug}")).contains(&"season-eastertide".into()));
        assert!(body_classes(&format!("/?date={slug}")).contains(&"season-passiontide".into()));
    }
}
