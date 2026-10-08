//! The view models the templates read: plain data filled in by the web server. Floats are formatted
//! here so the templates only substitute strings. Review metadata arrives as plain strings rather
//! than as `tools` types.

use serde::Serialize;

use crate::leader::LeaderSection;

/// The fields the shared layout reads on every page.
#[derive(Clone, Debug, Default, Serialize)]
pub struct Chrome {
    pub page: String,
    pub nav_date: String,
    /// Dates the page for the usage beacon: an ISO day, a bare year for the
    /// ordo, or empty when the page is always current.
    pub usage_when: String,
    pub season_class: String,
    /// `rank-first-class` on a first-class day's hour, which is framed; empty otherwise.
    pub rank_class: String,
    pub show_today: bool,
}

#[derive(Clone, Debug, Default, Serialize)]
pub struct HomeHourLink {
    pub name: String,
    pub slug: String,
    pub url: String,
    pub is_current: bool,
    /// What the hour is ("Evening prayer"), shown under its name while it is the current hour.
    pub note: String,
}

/// The day-landing page.
#[derive(Clone, Debug, Default, Serialize)]
pub struct HomeData {
    #[serde(flatten)]
    pub chrome: Chrome,
    pub date_str: String,
    pub date_slug: String,
    pub prev_link: String,
    pub next_link: String,
    pub today_link: String,
    /// The day's name without its alias, and the alias in its parentheses
    /// ("(Corpus Christi)") or empty (`presentation::split_alias`).
    pub feast_name: String,
    pub feast_alias: String,
    pub commemorations: Vec<String>,
    pub color: String,
    pub season: String,
    pub octave_note: String,
    pub penitential: Vec<String>,
    /// The day's versicle from Lauds and its response, set in the head on a plain day; both
    /// empty otherwise (`presentation::home_shows_versicle`).
    pub versicle: String,
    pub response: String,
    pub calendar_link: String,
    pub pray_now_label: String,
    pub pray_now_link: String,
    pub hours: Vec<HomeHourLink>,
}

/// One prayer form's composition of the hour.
#[derive(Clone, Debug, Serialize)]
pub struct LeaderForm {
    /// "private", "deacon", or "priest".
    pub form: String,
    pub label: String,
    pub report_url: String,
}

/// The composed hour's header fields.
#[derive(Clone, Debug, Default, Serialize)]
pub struct HourHeader {
    pub title: String,
    pub color: String,
    pub feast: String,
    pub season: String,
}

/// A section of a single-form page: trusted markup.
#[derive(Clone, Debug, Default, Serialize)]
pub struct SectionView {
    pub label: String,
    pub collapsible: bool,
    pub html: String,
}

/// A composed office hour and its chrome.
#[derive(Clone, Debug, Default, Serialize)]
pub struct HourData {
    #[serde(flatten)]
    pub chrome: Chrome,
    pub leader_forms: Vec<LeaderForm>,
    pub leader_sections: Vec<LeaderSection>,
    /// The single form's sections, when there are no leader forms.
    pub sections: Vec<SectionView>,
    pub hour_name: String,
    pub date_str: String,
    pub date_slug: String,
    pub prev_link: String,
    pub next_link: String,
    pub today_link: String,
    pub day_link: String,
    pub previous_hour_name: String,
    pub previous_hour_link: String,
    pub next_hour_name: String,
    pub next_hour_link: String,
    pub hour: HourHeader,
    pub report_url: String,
}

#[derive(Clone, Debug, Default, PartialEq, Eq, Serialize)]
pub struct CommemorationRow {
    pub name: String,
    pub incipit: String,
}

/// An observance the ordo brackets for monastics and oblates, printed under the day's office.
#[derive(Clone, Debug, Default, PartialEq, Eq, Serialize)]
pub struct MonasticRow {
    /// "Solemnity of St Benedict (Monastics & Oblates Only)", without the brackets.
    pub heading: String,
    pub rank: String,
    pub rank_full: String,
    /// Where its office is found, or empty.
    pub office: String,
}

/// One day of the year calendar, with its office digest.
#[derive(Clone, Debug, Default, PartialEq, Eq, Serialize)]
pub struct DayRow {
    pub day_num: u32,
    pub weekday: String,
    pub date_slug: String,
    pub rank: String,
    pub rank_full: String,
    pub color: String,
    pub color_class: String,
    pub feast_name: String,
    pub fast: bool,
    pub abstinence: bool,
    pub commemorations: Vec<String>,
    pub monastic: Vec<MonasticRow>,
    pub benedictus_antiphon: String,
    pub magnificat_antiphon: String,
    pub lauds_preces: bool,
    pub lauds_suffrage: bool,
    pub lauds_comms: Vec<CommemorationRow>,
    pub hours_preces: bool,
    pub vespers_preces: bool,
    pub vespers_suffrage: bool,
    pub vespers_comms: Vec<CommemorationRow>,
    pub vespers_note: String,
}

#[derive(Clone, Debug, Default, PartialEq, Eq, Serialize)]
pub struct MonthData {
    pub name: String,
    pub slug: String,
    pub days: Vec<DayRow>,
}

/// One entry in the ordo's month strip.
#[derive(Clone, Debug, Default, Serialize)]
pub struct MonthLink {
    pub name: String,
    /// The three-letter label the strip shows.
    pub abbr: String,
    pub href: String,
    /// "YYYY-MM", which the client matches against local today.
    pub month: String,
    /// This page is the month's own page.
    pub current: bool,
    /// Today falls in this month.
    pub today: bool,
}

/// A labelled figure or date in the Tabula Temporaria; `href` leads to the
/// date's row when it has one.
#[derive(Clone, Debug, Default, Serialize)]
pub struct TabulaRow {
    pub label: String,
    pub value: String,
    pub href: String,
}

/// The printed ordo's opening table for a year.
#[derive(Clone, Debug, Default, Serialize)]
pub struct TabulaData {
    pub figures: Vec<TabulaRow>,
    pub moveable: Vec<TabulaRow>,
    pub ember: Vec<TabulaRow>,
}

/// A neighbouring month, named with its year when it lies in another.
#[derive(Clone, Debug, Default, Serialize)]
pub struct MonthStep {
    pub name: String,
    pub href: String,
}

/// An ordo page: one month, the year's frontispiece, or the whole year.
#[derive(Clone, Debug, Default, Serialize)]
pub struct CalendarData {
    #[serde(flatten)]
    pub chrome: Chrome,
    pub year: i32,
    /// "month", "year" (the frontispiece), or "all".
    pub view: String,
    /// The year in Roman numerals for the frontispiece, empty past 3999.
    pub year_roman: String,
    pub prev_year: i32,
    pub next_year: i32,
    /// The neighbouring years in the same view: the same month, frontispiece,
    /// or whole year.
    pub prev_year_link: String,
    pub next_year_link: String,
    pub year_link: String,
    pub all_link: String,
    pub month_links: Vec<MonthLink>,
    /// The month shown, or every month of the whole-year view.
    pub months: Vec<std::sync::Arc<MonthData>>,
    pub prev_month: Option<MonthStep>,
    pub next_month: Option<MonthStep>,
    pub tabula: Option<TabulaData>,
}

#[derive(Clone, Debug, Default, Serialize)]
pub struct ReminderHour {
    pub name: String,
    pub slug: String,
    pub default: String,
    pub checked: bool,
}

#[derive(Clone, Debug, Default, Serialize)]
pub struct ReminderDay {
    pub name: String,
    pub slug: String,
}

/// The reminder-subscription settings page.
#[derive(Clone, Debug, Default, Serialize)]
pub struct RemindersData {
    #[serde(flatten)]
    pub chrome: Chrome,
    pub hours: Vec<ReminderHour>,
    pub days: Vec<ReminderDay>,
}

/// The styled 404 page.
#[derive(Clone, Debug, Default, Serialize)]
pub struct NotFoundData {
    #[serde(flatten)]
    pub chrome: Chrome,
}

/// The privacy policy.
#[derive(Clone, Debug, Default, Serialize)]
pub struct PrivacyData {
    #[serde(flatten)]
    pub chrome: Chrome,
}

/// The styled page for other 4xx and 5xx conditions.
#[derive(Clone, Debug, Default, Serialize)]
pub struct ErrorData {
    #[serde(flatten)]
    pub chrome: Chrome,
    pub title: String,
    pub message: String,
}
