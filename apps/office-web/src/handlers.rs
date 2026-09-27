//! Page handlers: resolve the request to a liturgical day, drive the engine,
//! and fill the view models. Ported from Go's `web/handlers.go`.

use std::collections::HashSet;

use axum::body::Body;
use axum::http::{HeaderMap, HeaderValue, Method, Response, StatusCode, header};
use calendar::MoveableDates;
use jiff::tz::TimeZone;
use liturgy::{OfficeHour, PrayerForm};
use office::day::Day;
use office::summary::{CommSummary, summarize_hour};
use office::{ComposeOptions, Engine, VespersOwner};
use render_html::links::{calendar_link, home_link, hour_link, season_class, title_case};
use render_html::view::{
    AssuranceDecision, AssuranceDependency, AssuranceFlag, AssuranceResolution, CalendarData, Chrome, CommemorationRow, DayRow, ErrorData,
    HomeData, HomeHourLink, HourAssurance, HourData, HourHeader, LeaderForm, MonthData, NotFoundData, ReminderDay, ReminderHour,
    RemindersData,
};
use tools::review::assurance::{dedupe_decisions, hour_dependencies};
use tools::review::provenance::ProvenanceStatus;

use crate::Server;
use crate::gonet::{Query, cookie, redirect, response, set};
use crate::gotime::{date_slug, load_location, local, long_date, now_in, parse_date};

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

/// The GitHub new-issue endpoint behind "Report a problem".
const REPO_ISSUES_URL: &str = "https://github.com/orthodoxwest/office/issues/new";

/// Go's `url.QueryEscape`.
fn query_escape(s: &str) -> String {
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

fn season_str(hour: &OfficeHour) -> &'static str {
    hour.season.map(|s| s.as_str()).unwrap_or("")
}

fn celebration(hour: &OfficeHour) -> String {
    if hour.feast.is_empty() { format!("{} feria", title_case(season_str(hour))) } else { hour.feast.clone() }
}

/// A prefilled issue identifying the exact page under review.
fn report_url(hour: &OfficeHour, hour_name: &str, date_slug: &str) -> String {
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

fn dependency_report_url(hour: &OfficeHour, hour_name: &str, date_slug: &str, key: &str, status: ProvenanceStatus) -> String {
    let title = format!("[review] Source verification — {key}");
    let body = format!(
        "**Page:** /{hour_name}/{date_slug}?form={form}
**Prayer form:** {label}
**Celebration:** {celebration}
**Corpus entry:** {key}
**Current provenance status:** {status}

**Source and page/section locator:**


**Finding:**

",
        form = hour.form.as_str(),
        label = hour.form.label(),
        celebration = celebration(hour),
        status = status.as_str(),
    );
    issue_url(&title, &body)
}

/// The Go `currentHourSchedule`, mirrored in app.js: from each hour of the
/// clock, the office being prayed and its day offset.
const CURRENT_HOUR_SCHEDULE: [(i8, &str, &str, i32); 8] = [
    (0, "compline", "Compline", -1),
    (2, "lauds", "Lauds", 0),
    (7, "prime", "Prime", 0),
    (9, "terce", "Terce", 0),
    (11, "sext", "Sext", 0),
    (13, "none", "None", 0),
    (17, "vespers", "Vespers", 0),
    (20, "compline", "Compline", 0),
];

fn current_hour_entry(hour: i8) -> (&'static str, &'static str, i32) {
    CURRENT_HOUR_SCHEDULE.iter().rev().find(|b| hour >= b.0).map(|b| (b.1, b.2, b.3)).unwrap_or(("compline", "Compline", -1))
}

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

/// An HTML page: Go sniffs the type, and every page revalidates.
fn html(status: StatusCode, body: String) -> Response<Body> {
    let mut resp = response(status, body);
    set(&mut resp, header::CONTENT_TYPE, "text/html; charset=utf-8");
    set(&mut resp, header::CACHE_CONTROL, "no-cache");
    resp
}

fn render_failed(e: &str) -> Response<Body> {
    crate::gonet::http_error(e, StatusCode::INTERNAL_SERVER_ERROR)
}

/// The day's display name, as the ordo row names it.
fn day_name(day: &Day) -> String {
    if let Some(c) = &day.celebration {
        return c.name.clone();
    }
    if let Some(t) = &day.tempora {
        return t.clone();
    }
    format!("{} feria", title_case(day.season.as_str()))
}

fn comm_rows(comms: &[CommSummary]) -> Vec<CommemorationRow> {
    comms.iter().map(|c| CommemorationRow { name: c.name.clone(), incipit: c.incipit.clone() }).collect()
}

/// The calendar page's per-day rows with the composed Lauds, Hours, and
/// Vespers digest.
pub fn build_month_data(days: &[Day], engine: &Engine, moveable: &MoveableDates) -> Vec<MonthData> {
    let mut months: Vec<MonthData> = Vec::new();
    let summarize = |hour: &str, day: &Day| engine.compose_hour(hour, day, moveable, PrayerForm::Private).ok().map(|h| summarize_hour(&h));
    for d in days {
        let name = crate::gotime::month_name(d.date);
        if months.last().is_none_or(|m| m.name != name) {
            months.push(MonthData { name: name.to_string(), slug: name.to_lowercase(), days: Vec::new() });
        }
        let (rank, rank_full) = match &d.celebration {
            Some(c) => (c.rank.abbrev().to_string(), c.rank.display_name().to_string()),
            None => (String::new(), String::new()),
        };
        let mut row = DayRow {
            day_num: d.date.day(),
            weekday: d.date.weekday().name()[..3].to_string(),
            date_slug: date_slug(d.date),
            rank,
            rank_full,
            color: d.color.as_str().to_string(),
            color_class: format!("day-color-{}", d.color.as_str()),
            feast_name: day_name(d),
            fast: d.penitential.fast,
            abstinence: d.penitential.abstinence,
            commemorations: d.commemorations.iter().map(|c| c.name.clone()).collect(),
            ..DayRow::default()
        };
        if let Some(lauds) = summarize("lauds", d) {
            row.benedictus_antiphon = lauds.gospel_ant;
            row.lauds_preces = lauds.preces;
            row.lauds_suffrage = lauds.suffrage;
            row.lauds_comms = comm_rows(&lauds.comms);
        }
        // The minor hours share one preces disposition; Prime stands for them.
        if let Some(hours) = summarize("prime", d) {
            row.hours_preces = hours.preces;
        }
        if let Some(vespers) = summarize("vespers", d) {
            row.magnificat_antiphon = vespers.gospel_ant;
            row.vespers_preces = vespers.preces;
            row.vespers_suffrage = vespers.suffrage;
            row.vespers_comms = comm_rows(&vespers.comms);
        }
        row.vespers_note = match d.vespers.owner {
            VespersOwner::IIOfPreceding => "II Vespers of preceding".to_string(),
            VespersOwner::IOfFollowing => d.vespers.feast.as_ref().map(|f| format!("I Vespers of {}", f.name)).unwrap_or_default(),
            VespersOwner::NotApplicable => String::new(),
        };
        if let Some(m) = months.last_mut() {
            m.days.push(row);
        }
    }
    months
}

fn invalid_date(s: &str) -> String {
    format!("Invalid date {} — please use YYYY-MM-DD format.", compat::quote(s))
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
            // Go has already sent the status; the body is whatever rendered.
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

        let feast_name = day_name(day);
        let lower_feast = feast_name.to_lowercase();
        // A short note for home; the ordo carries the full wording.
        let octave_note = match &day.within_octave_of {
            Some(id) if !id.is_empty() && !lower_feast.contains("octave") => {
                format!("Octave of {}", calendar::builder::octave_display_name(id))
            }
            _ => String::new(),
        };
        // Not repeated when the celebration already names the season.
        let mut season = title_case(day.season.as_str());
        if !season.is_empty() && lower_feast.contains(&season.to_lowercase()) {
            season.clear();
        }

        let (now, now_hour) = now_in(&loc);
        let now_slug = date_slug(now);
        let (mut current_slug, mut label, mut pray_now_date, mut grid_current) = ("", "Open Lauds".to_string(), slug.clone(), "");
        if slug == now_slug {
            let (s, l, offset) = current_hour_entry(now_hour);
            current_slug = s;
            label = format!("Pray {l}");
            if offset != 0 {
                pray_now_date = date_slug(now.add_days(offset));
            } else {
                grid_current = s;
            }
        }
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
            feast_name,
            commemorations: day.commemorations.iter().map(|c| c.name.clone()).collect(),
            color: day.color.as_str().into(),
            season,
            octave_note,
            penitential: day.penitential.labels().into_iter().map(String::from).collect(),
            calendar_link: calendar_link(&slug),
            pray_now_label: label,
            pray_now_link: hour_link(if current_slug.is_empty() { "lauds" } else { current_slug }, &pray_now_date),
            hours: build_home_hours(&slug, grid_current),
        };
        match self.pages.home(&data) {
            Ok(body) => html(StatusCode::OK, body),
            Err(e) => render_failed(&e),
        }
    }

    fn show_vetting_banner(&self, hour: &OfficeHour) -> bool {
        let deps = hour_dependencies(hour);
        deps.is_empty() || deps.iter().any(|k| self.provenance.get(k) != Some(&ProvenanceStatus::Verified))
    }

    fn hour_assurance(&self, hour: &OfficeHour, hour_name: &str, slug: &str) -> HourAssurance {
        let mut data = HourAssurance {
            decisions: dedupe_decisions(&hour.decisions)
                .into_iter()
                .map(|d| AssuranceDecision {
                    rule: d.rule.clone(),
                    outcome: d.outcome.clone(),
                    detail: d.detail.clone().unwrap_or_default(),
                })
                .collect(),
            ..HourAssurance::default()
        };
        for key in hour_dependencies(hour) {
            let status = self.provenance.get(&key).copied().unwrap_or(ProvenanceStatus::SourceUnknown);
            match status {
                ProvenanceStatus::Verified => data.verified += 1,
                ProvenanceStatus::NeedsReview => data.needs_review += 1,
                ProvenanceStatus::SourceUnknown => data.source_unknown += 1,
            }
            let flags: Vec<AssuranceFlag> = self
                .suspicions
                .get(&key)
                .map(|s| {
                    s.iter()
                        .map(|f| AssuranceFlag {
                            label: f.label.clone(),
                            state: if f.addressed { "addressed" } else { "open" }.into(),
                            reason: f.reason.clone(),
                        })
                        .collect()
                })
                .unwrap_or_default();
            if !flags.is_empty() {
                data.flagged += 1;
            }
            let report_url = dependency_report_url(hour, hour_name, slug, &key, status);
            data.dependencies.push(AssuranceDependency { key, status: status.as_str().into(), flags, report_url });
        }
        let mut seen = HashSet::new();
        for element in hour.sections.iter().flat_map(|s| &s.elements) {
            if element.slot_ref.is_empty() || element.source_ref.is_empty() {
                continue;
            }
            let tier = element.source_ref.split('/').next().unwrap_or("");
            if seen.insert((element.slot_ref.clone(), tier.to_string(), element.source_ref.clone())) {
                data.resolutions.push(AssuranceResolution {
                    slot: element.slot_ref.clone(),
                    tier: tier.into(),
                    source: element.source_ref.clone(),
                });
            }
        }
        data
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
                    assurance: self.hour_assurance(h, hour_name, &date_str),
                    report_url: report_url(h, hour_name, &date_str),
                    show_banner: self.show_vetting_banner(h),
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
            show_banner: self.show_vetting_banner(&hour),
            assurance: self.hour_assurance(&hour, hour_name, &date_str),
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

    pub fn calendar(&self, req: &Req) -> Response<Body> {
        let path = req.path.trim_matches('/');
        let parts: Vec<&str> = path.split('/').collect();
        if parts.len() > 2 {
            return self.not_found_page(req);
        }
        let now = now_in(&self.user_location(req)).0;
        if parts.len() == 1 {
            // No year: today's row of the current year.
            let form = req.query.get("form");
            let form_query = match PrayerForm::parse(form) {
                Ok(f) if !form.is_empty() => format!("?form={}", f.as_str()),
                _ => String::new(),
            };
            let target = format!("/calendar/{}{form_query}#d-{}", now.year(), date_slug(now));
            return redirect(req.method, &target, StatusCode::FOUND);
        }
        let year = match compat::atoi(parts[1]) {
            Ok(y) if (1..=9999).contains(&y) => y as i32,
            _ => return self.error_page(req, StatusCode::BAD_REQUEST, &format!("Invalid year {}.", compat::quote(parts[1]))),
        };
        let months = match self.cache.months(year, &self.engine) {
            Ok(m) => m,
            Err(e) => return self.error_page(req, StatusCode::INTERNAL_SERVER_ERROR, &format!("error building calendar: {e}")),
        };
        let data = CalendarData {
            chrome: Chrome { page: "calendar".into(), nav_date: date_slug(now), usage_when: year.to_string(), ..Chrome::default() },
            year,
            prev_year: year - 1,
            next_year: year + 1,
            months,
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

    #[test]
    fn schedule_matches_go() {
        assert_eq!(current_hour_entry(0), ("compline", "Compline", -1));
        assert_eq!(current_hour_entry(1), ("compline", "Compline", -1));
        assert_eq!(current_hour_entry(2), ("lauds", "Lauds", 0));
        assert_eq!(current_hour_entry(16), ("none", "None", 0));
        assert_eq!(current_hour_entry(23), ("compline", "Compline", 0));
    }

    #[test]
    fn query_escape_matches_go() {
        assert_eq!(query_escape("a b/c?d=é—*"), "a+b%2Fc%3Fd%3D%C3%A9%E2%80%94%2A");
    }
}
