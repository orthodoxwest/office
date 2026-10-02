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
    /// The light of the hour being prayed ("dawn", "day", "dusk", "night"), which home's wall
    /// takes as a `time-*` body class; empty elsewhere.
    pub time_of_day: String,
    pub show_today: bool,
}

#[derive(Clone, Debug, Default, Serialize)]
pub struct HomeHourLink {
    pub name: String,
    pub slug: String,
    pub url: String,
    /// The hour in the old reckoning ("the third hour").
    pub time: String,
    pub is_current: bool,
    /// The hour the leaf points at: the one being prayed, or where another day begins.
    pub is_now: bool,
}

/// Text opened by a versal: its first letter, set apart, and the rest, both typeset. The
/// initial is empty when the text takes none; `rest` is then the whole text.
#[derive(Clone, Debug, Default, PartialEq, Eq, Serialize)]
pub struct Versal {
    pub initial: String,
    pub rest: String,
}

impl Versal {
    /// Typesets `text` and, when `opened` and it begins with a capital A–Z (the versal faces hold
    /// no other letter), sets that capital apart.
    pub fn new(text: &str, opened: bool) -> Versal {
        let text = crate::html::typeset(text.trim());
        match text.chars().next() {
            Some(c) if opened && c.is_ascii_uppercase() => Versal { initial: c.to_string(), rest: text[1..].to_string() },
            _ => Versal { initial: String::new(), rest: text },
        }
    }
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
    pub feast_name: String,
    pub commemorations: Vec<String>,
    pub color: String,
    pub season: String,
    pub octave_note: String,
    pub penitential: Vec<String>,
    pub calendar_link: String,
    pub pray_now_label: String,
    pub pray_now_link: String,
    pub hours: Vec<HomeHourLink>,
    /// The weekday and the rest of the date, which the leaf's rubric line sets apart.
    pub weekday: String,
    pub date_rest: String,
    /// The day's title with its versal ([`Versal`]), and the rank's name and grade
    /// (`presentation::leaf_rank`).
    pub title: Versal,
    pub rank_label: String,
    pub grade: u8,
    /// The title opens with a Roman numeral ("III Sunday in Lent"), which keeps it whole.
    pub numeral: bool,
    /// The leaf's pointer says "pray now" (the hour being prayed) or "begin here".
    pub now_cue: String,
    /// The day's collect, from its Lauds; empty when there is none.
    pub collect: Versal,
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

/// The styled page for other 4xx and 5xx conditions.
#[derive(Clone, Debug, Default, Serialize)]
pub struct ErrorData {
    #[serde(flatten)]
    pub chrome: Chrome,
    pub title: String,
    pub message: String,
}
