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
    /// `rank-first-class` on a first-class day's hour, which is framed, and on its home
    /// (`presentation::home_first_class`); empty otherwise.
    pub rank_class: String,
    /// A bare hour (`OfficeHour::bare`): the page takes off its leaf (`hour-bare`).
    pub bare: bool,
    pub show_today: bool,
    /// The site's origin (`https://host`), for the absolute addresses search engines and link
    /// previews read; empty on pages with no address of their own (errors, the usage report).
    pub site: String,
    /// The page's path on `site`, without the reader's own choices (`?form=`).
    pub canonical: String,
    /// Kept out of search results: a dated day or hour, an ordo year far from now, an error.
    pub noindex: bool,
}

#[derive(Clone, Debug, Default, Serialize)]
pub struct HomeHourLink {
    pub name: String,
    pub slug: String,
    pub url: String,
    pub is_current: bool,
    /// What the hour is ("Evening prayer"), shown under its name while it is the current hour on
    /// a phone, and under every hour's name in the wide home's niche.
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
    /// The day's feast's page, or empty when it has none (a feria, a Sunday of the season).
    pub feast_href: String,
    pub commemorations: Vec<String>,
    /// Each commemoration's page, in the same order, or empty.
    pub commemoration_hrefs: Vec<String>,
    /// The feast page the reader came from, offered as the way back.
    pub from_feast: Option<LinkRow>,
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
    /// The chooser's words for the form: "Praying privately".
    pub phrase: String,
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
    pub bare: bool,
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
    /// Whether the prayers before and after the Office start open
    /// ([`presentation::prayers_open_by_default`]).
    pub prayers_open: bool,
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
    /// The consecration cross under the last Amen takes no leaf (`presentation::EndLeaf`): a bare
    /// hour, or one whose last section is said bare (Vespers of the Dead after the day's).
    pub end_bare: bool,
}

#[derive(Clone, Debug, Default, PartialEq, Eq, Serialize)]
pub struct CommemorationRow {
    pub name: String,
    pub incipit: String,
    /// The commemorated feast's page, or empty.
    pub href: String,
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
    /// The rail's colour class: within an octave's strand the octave's, else the day's own
    /// (`presentation::rails`).
    pub color_class: String,
    /// Days bound together: the day's own colour beside the octave's strand, or empty.
    pub rail_inset: String,
    /// The rail runs up into the day before and down into the day after, of the same octave.
    pub bound_above: bool,
    pub bound_below: bool,
    pub feast_name: String,
    pub fast: bool,
    pub abstinence: bool,
    pub commemorations: Vec<String>,
    /// Each commemoration's page, in the same order, or empty.
    pub commemoration_hrefs: Vec<String>,
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

/// The reminder page's hours in one of home's three periods ("morning", "Morning").
#[derive(Clone, Debug, Default, Serialize)]
pub struct ReminderPeriod {
    pub key: String,
    pub label: String,
    pub hours: Vec<ReminderHour>,
}

/// The reminder-subscription settings page.
#[derive(Clone, Debug, Default, Serialize)]
pub struct RemindersData {
    #[serde(flatten)]
    pub chrome: Chrome,
    pub periods: Vec<ReminderPeriod>,
    pub days: Vec<ReminderDay>,
    /// The rule written out for the default choices, phrase by phrase, for the first paint.
    pub rule: Vec<ReminderRulePhrase>,
    /// The rule's words for app.js (`presentation::reminders::RULE_WORDS`), as JSON.
    pub rule_words: String,
}

/// One phrase of the reminder rule, keyed by what it says (`presentation::reminders::RulePhrase`).
#[derive(Clone, Debug, Default, Serialize)]
pub struct ReminderRulePhrase {
    pub key: String,
    pub text: String,
}

/// The reminder rule's words as the page's JSON: app.js writes the rule from them by
/// `presentation::reminders`' grammar, so the server need not be asked on every change.
pub fn reminder_rule_words() -> String {
    use data_format::json::{Json, Obj};
    use presentation::reminders::{DAY_PLURALS, RULE_WORDS as w};
    let names = presentation::REMINDER_DEFAULTS.iter().fold(Obj::new(), |o, d| o.str(d.0, d.1)).build();
    let sun = w.sun.iter().fold(Obj::new(), |o, (h, t)| o.str(h, t)).build();
    let leads = w.leads.iter().fold(Obj::new(), |o, (m, t)| o.str(&m.map_or("none".to_string(), |m| m.to_string()), t)).build();
    let value = Obj::new()
        .field("hours", names)
        .field("days", Json::strings(&DAY_PLURALS.map(String::from)))
        .str("everyDay", w.every_day)
        .str("weekdays", w.weekdays)
        .str("on", w.on)
        .str("except", w.except)
        .str("at", w.at)
        .str("and", w.and)
        .field("leads", leads)
        .field("sun", sun)
        .str("noHour", w.no_hour)
        .str("noDay", w.no_day)
        .str("noHourOrDay", w.no_hour_or_day)
        .str("noTime", w.no_time)
        .build();
    data_format::json::encode_compact(&value)
}

/// The styled 404 page.
#[derive(Clone, Debug, Default, Serialize)]
pub struct NotFoundData {
    #[serde(flatten)]
    pub chrome: Chrome,
}

/// The page for newcomers: what the Office is and how to pray it ([`presentation::about`]).
#[derive(Clone, Debug, Default, Serialize)]
pub struct AboutData {
    #[serde(flatten)]
    pub chrome: Chrome,
    pub title: String,
    pub blocks: Vec<AboutBlockView>,
    /// The seven hours in home's three bands.
    pub about_hours: Vec<AboutPeriod>,
    /// Home's invitation at the page's foot, by the server's clock in the reader's zone (app.js
    /// restates it by the device's): "Pray None", and the hour it opens.
    pub pray_now_label: String,
    pub pray_now_link: String,
}

/// One block of the About page. `kind` is intro, verse, heading, paragraph, hours, note, key or
/// invitation;
/// `html` is a paragraph's words, escaped, with its links made anchors.
#[derive(Clone, Debug, Default, Serialize)]
pub struct AboutBlockView {
    pub kind: &'static str,
    pub text: String,
    pub html: String,
    pub cite: String,
    pub mark: String,
    pub ink: &'static str,
}

/// One of home's bands of hours (Morning, Day, Evening) on the about page.
#[derive(Clone, Debug, Default, Serialize)]
pub struct AboutPeriod {
    pub label: String,
    pub hours: Vec<AboutHour>,
}

/// An hour on the about page: its name, what it is, and when it is said; `is_current` when it is
/// the hour home marks as now.
#[derive(Clone, Debug, Default, Serialize)]
pub struct AboutHour {
    pub slug: String,
    pub is_current: bool,
    pub name: String,
    pub url: String,
    pub gloss: String,
    pub time: String,
}

impl AboutData {
    /// The page as presentation words it, for the page chrome given, with the hour it is now
    /// marked and invited to as home does at `now`'s `clock_hour` (0–23).
    pub fn new(chrome: Chrome, now: calendar::Date, clock_hour: i8) -> AboutData {
        use crate::escape::html_escape_string as esc;
        use presentation::about::{ABOUT, ABOUT_HOURS, ABOUT_TITLE, AboutBlock, AboutRun, about_current_hour, about_runs};
        let current = about_current_hour(clock_hour);
        let invite = presentation::invitation(now, now, clock_hour);
        let block = |kind| AboutBlockView { kind, ..AboutBlockView::default() };
        let blocks = ABOUT
            .iter()
            .map(|b| match *b {
                AboutBlock::Intro(text) => AboutBlockView { text: text.into(), ..block("intro") },
                AboutBlock::Verse { text, cite } => AboutBlockView { text: text.into(), cite: cite.into(), ..block("verse") },
                AboutBlock::Heading(text) => AboutBlockView { text: text.into(), ..block("heading") },
                AboutBlock::Paragraph(text) => {
                    let html = about_runs(text)
                        .into_iter()
                        .map(|run| match run {
                            AboutRun::Text(t) => esc(&t),
                            AboutRun::Link { text, target } => format!("<a href=\"{}\">{}</a>", esc(&target), esc(&text)),
                        })
                        .collect();
                    AboutBlockView { html, ..block("paragraph") }
                }
                AboutBlock::Hours => block("hours"),
                AboutBlock::Note(text) => AboutBlockView { text: text.into(), ..block("note") },
                AboutBlock::Key { mark, ink, text } => {
                    AboutBlockView { text: text.into(), mark: mark.into(), ink: ink.name(), ..block("key") }
                }
                AboutBlock::Invitation => block("invitation"),
            })
            .collect();
        let about_hours = ABOUT_HOURS
            .iter()
            .map(|(label, hours)| AboutPeriod {
                label: (*label).into(),
                hours: hours
                    .iter()
                    .map(|(slug, time)| AboutHour {
                        slug: (*slug).into(),
                        is_current: *slug == current,
                        name: presentation::title_case(slug),
                        url: format!("/{slug}"),
                        gloss: presentation::hour_gloss(slug).into(),
                        time: (*time).into(),
                    })
                    .collect(),
            })
            .collect();
        AboutData {
            chrome,
            title: ABOUT_TITLE.into(),
            blocks,
            about_hours,
            pray_now_label: invite.label,
            pray_now_link: crate::links::hour_link(invite.hour, &presentation::date_slug(invite.date)),
        }
    }
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

/// A feast's keeping in one year, as a row leading to that day.
#[derive(Clone, Debug, Default, Serialize)]
pub struct FeastYearRow {
    pub year: i32,
    /// "Sat, Feb 27", or empty when the feast has no date that year.
    pub date: String,
    /// The day's page, or empty.
    pub href: String,
    /// "Kept", "Commemorated", …
    pub label: String,
    /// "kept", "commemorated", "monastic", "transferred", "not-kept".
    pub class: String,
    /// Kept as the feast usually is, so the feast page leaves the status unsaid.
    pub usual: bool,
    /// Why it is kept so, or empty.
    pub note: String,
    /// The colour of that day's office.
    pub color: String,
}

/// A feast in the search results.
#[derive(Clone, Debug, Default, Serialize)]
pub struct FeastResult {
    pub name: String,
    pub href: String,
    pub rank: String,
    pub rank_full: String,
    /// "is-greater" (a greater double or above), "is-double", or empty, as the ordo marks ranks.
    pub rank_class: String,
    /// "February 27", or "February 27 · Monastics & Oblates only".
    pub meta: String,
    pub years: Vec<FeastYearRow>,
}

/// The ordo's search: a feast by name, with how it is kept this year and next.
#[derive(Clone, Debug, Default, Serialize)]
pub struct FindData {
    #[serde(flatten)]
    pub chrome: Chrome,
    pub query: String,
    pub subtitle: String,
    /// What was found, or empty before a search.
    pub count: String,
    pub results: Vec<FeastResult>,
}

/// One of a feast's texts on its page.
#[derive(Clone, Debug, Default, Serialize)]
pub struct FeastTextRow {
    pub label: String,
    pub text: String,
    pub common: bool,
    /// A prayer, set flowing; an antiphon or versicle keeps its lines.
    pub prayer: bool,
}

/// A labelled link.
#[derive(Clone, Debug, Default, Serialize)]
pub struct LinkRow {
    pub label: String,
    pub href: String,
}

/// The next day a feast is kept, and the way to that day's hours.
#[derive(Clone, Debug, Default, Serialize)]
pub struct NextKeeping {
    pub label: String,
    pub date: String,
    pub href: String,
    pub color: String,
    /// The next keeping is today: its way on is "Today's hours".
    pub today: bool,
}

/// A feast's page: its rank and date, how it is kept in the years ahead, and its texts.
#[derive(Clone, Debug, Default, Serialize)]
pub struct FeastPageData {
    #[serde(flatten)]
    pub chrome: Chrome,
    pub name: String,
    pub subtitle: String,
    /// "February 27", or "Its date moves from year to year".
    pub date: String,
    /// The titulus over the name: "February 27", or "Its date moves".
    pub titulus: String,
    /// "first", "feast" or "plain": how much the head carries (`presentation::feast::feast_register`).
    pub register: String,
    /// The feast's own colour.
    pub color: String,
    /// The way back: to the day or the ordo's month the page was opened from, else the search.
    pub back: LinkRow,
    pub monastic_note: String,
    /// For a monastic observance, the feast of the same saint everyone keeps.
    pub kept_by_all: Option<LinkRow>,
    pub kept_by_all_note: String,
    pub next: Option<NextKeeping>,
    pub years_heading: String,
    pub years: Vec<FeastYearRow>,
    pub texts_source: String,
    pub texts: Vec<FeastTextRow>,
    /// The day whose hours say these texts in full, or none.
    pub whole_office: Option<LinkRow>,
}
