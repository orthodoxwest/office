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
    /// Vespers on an evening that already keeps the morrow: "First Vespers of …", which the
    /// invitation sets under "Pray Vespers" in place of the note; empty otherwise.
    pub eve: String,
}

/// The day-landing page.
#[derive(Clone, Debug, Default, Serialize)]
pub struct HomeData {
    #[serde(flatten)]
    pub chrome: Chrome,
    /// The date field's span, `presentation::offered_years`.
    pub first_year: i32,
    pub last_year: i32,
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
    /// What the hour the invitation opens is ("Evening prayer"), set under its words on a phone
    /// so the hours' table keeps one shape whichever hour is now; empty for "Open Lauds".
    pub pray_now_gloss: String,
    /// Whether the gloss is the eve's "First Vespers of …", which the wide niche sets under the
    /// invitation too (#959).
    pub pray_now_eve: bool,
    pub pray_now_link: String,
    pub hours: Vec<HomeHourLink>,
    /// The Night's row before the seven: Matins, not yet prayed in the app.
    pub night: HomeNight,
}

/// Home's Night row: Matins under its period, with what it is and that it is coming soon
/// (`presentation::about::ABOUT_NIGHT`, `presentation::COMING_SOON`). Its words are the same
/// every day, so its default is the row.
#[derive(Clone, Debug, Serialize)]
pub struct HomeNight {
    pub label: String,
    pub name: String,
    pub gloss: String,
    pub coming_soon: String,
}

impl Default for HomeNight {
    fn default() -> Self {
        let (label, hours) = presentation::about::ABOUT_NIGHT;
        let slug = hours[0].0;
        HomeNight {
            label: label.into(),
            name: presentation::title_case(slug),
            gloss: presentation::hour_gloss(slug).into(),
            coming_soon: presentation::COMING_SOON.into(),
        }
    }
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
    /// The feast's overview page, or empty when the day has none.
    pub feast_href: String,
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
    /// The date field's span, `presentation::offered_years`.
    pub first_year: i32,
    pub last_year: i32,
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
    /// A vigil's thread (`presentation::rails::vigil_leads_into_feast`): on the vigil's row the
    /// feast's rail colour, on the feast's row the vigil's, or empty.
    pub lead_to: String,
    pub lead_from: String,
    pub feast_name: String,
    /// The principal feast's overview page, or empty on plain ferias without one.
    pub feast_href: String,
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
    /// The Commemoration of the Cross, said in Paschaltide in the Suffrage's place.
    pub lauds_cross: bool,
    pub lauds_comms: Vec<CommemorationRow>,
    pub hours_preces: bool,
    pub vespers_preces: bool,
    pub vespers_suffrage: bool,
    pub vespers_cross: bool,
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
    /// Today's place along the month's band ("40.323%"), on today's month only; the client
    /// moves it with the local date.
    pub today_x: String,
    /// The month's runs of day colours, the year's course (`presentation::course`).
    pub course: Vec<CourseRunView>,
    /// Rare-color pins (Rose, Black) in this month so selecting a rare color on the frontispiece
    /// legend visibly locates its single-day flecks.
    pub rare_pins: Vec<AboutRibbonPin>,
}

/// One run of a month's band: `days` days of `color` from `start` (0-based), on a span of 31.
#[derive(Clone, Debug, Default, Serialize)]
pub struct CourseRunView {
    pub start: u32,
    pub days: u32,
    pub color: String,
}

/// A labelled figure or date in the Tabula Temporaria; `href` leads to the
/// date's row when it has one, and `feast_href` leads to the feast overview page.
#[derive(Clone, Debug, Default, Serialize)]
pub struct TabulaRow {
    pub label: String,
    pub value: String,
    pub href: String,
    pub feast_href: String,
    pub gloss_title: String,
    pub gloss_note: String,
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
    /// Whether a crawler may follow it: only within the years search indexes, as the year
    /// links (`CalendarData::prev_year_follow`), so that it does not walk the ordo a month at
    /// a time from end to end either.
    pub follow: bool,
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
    /// Whether a crawler may follow the year links: only to the years search indexes, so
    /// that a crawler does not walk the ordo a year at a time from end to end.
    pub prev_year_follow: bool,
    pub next_year_follow: bool,
    /// Whether the neighbouring years are offered (`presentation::offered_years`).
    pub has_prev_year: bool,
    pub has_next_year: bool,
    pub year_link: String,
    pub all_link: String,
    pub month_links: Vec<MonthLink>,
    /// The six liturgical colors with this year's dates/counts, shown interactively on the frontispiece.
    pub ordo_colors: Vec<AboutOrdoColorView>,
    /// The month shown, or every month of the whole-year view.
    pub months: Vec<std::sync::Arc<MonthData>>,
    pub prev_month: Option<MonthStep>,
    pub next_month: Option<MonthStep>,
    pub tabula: Option<TabulaData>,
}

#[derive(Clone, Debug, Default, Serialize)]
pub struct ReminderHour {
    pub name: String,
    pub gloss: String,
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
    pub kicker: String,
    pub title: String,
    /// The epigraph, a verse to a line.
    pub verse_lines: Vec<String>,
    pub verse_cite: String,
    pub intro: String,
    pub hours_heading: String,
    /// Matins in the Night, then the seven hours in home's three bands.
    pub about_hours: Vec<AboutPeriod>,
    /// Flat list of the hours with Horologium coordinates for the arc of the day and the night.
    pub horologium: AboutHorologium,
    pub hours_note: String,
    /// What an hour not yet prayed in the app says (`presentation::COMING_SOON`).
    pub coming_soon: String,
    /// The remaining sections of the page (Where to begin, Reading the page, An ancient prayer,
    /// The Ordo, This app) in reading order.
    pub sections: Vec<AboutSection>,
    /// The live miniature office specimen shown in "Reading the page".
    pub specimen: AboutSpecimenView,
    /// This year's liturgical color ribbon for "The Ordo".
    pub ordo_ribbon: AboutOrdoRibbon,
    /// The six liturgical colors explained beneath the ribbon.
    pub ordo_colors: Vec<AboutOrdoColorView>,
    /// What a screen reader hears for the ribbon.
    pub ribbon_label: String,
    /// Home's invitation at the page's foot, by the server's clock in the reader's zone (app.js
    /// restates it by the device's): "Pray None", and the hour it opens.
    pub pray_now_label: String,
    pub pray_now_link: String,
}

/// One titled section of the About page's prose column.
#[derive(Clone, Debug, Default, Serialize)]
pub struct AboutSection {
    pub heading: String,
    pub blocks: Vec<AboutBlockView>,
}

/// One block inside an About section (`paragraph`, `specimen`, `key`, `era`, `ordo_ribbon`, `elsewhere`).
#[derive(Clone, Debug, Default, Serialize)]
pub struct AboutBlockView {
    pub kind: &'static str,
    pub id: String,
    pub text: String,
    pub html: String,
    pub period: String,
    pub mark: String,
    pub tail: String,
    /// What a screen reader says for a key's mark (`presentation::about::AboutBlock::Key`).
    pub spoken: String,
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
    /// Shown but not yet prayed in the app: unlinked, with `presentation::COMING_SOON`.
    pub coming_soon: bool,
}

/// A single station along the Horologium's arc (viewBox 320×160, center 160×128, radii 118×104
/// above the horizon and 118×22 under it: `presentation::about::about_horologium_point`).
#[derive(Clone, Debug, Default, Serialize)]
pub struct AboutHorologiumStop {
    pub slug: String,
    pub name: String,
    pub gloss: String,
    pub time: String,
    pub angle: f64,
    pub night: bool,
    /// `star`, `moon` or `sun` (`presentation::about::about_horologium_sign`).
    pub sign: &'static str,
    pub is_current: bool,
    pub x: String,
    pub y: String,
    pub tick_x1: String,
    pub tick_y1: String,
    pub left_pct: String,
    pub top_pct: String,
}

/// Initial SSR state for the painted Horologium (Arc of the Day).
#[derive(Clone, Debug, Default, Serialize)]
pub struct AboutHorologium {
    pub stops: Vec<AboutHorologiumStop>,
    pub active_slug: String,
    pub active_name: String,
    pub active_gloss: String,
    pub active_time: String,
    pub active_x: String,
    pub active_y: String,
    pub active_cx_pct: String,
    pub active_cy_pct: String,
    pub active_night: bool,
    pub active_is_now: bool,
    pub dawn_x: String,
    pub dawn_y: String,
}

/// The miniature office leaf shown in "Reading the page".
#[derive(Clone, Debug, Default, Serialize)]
pub struct AboutSpecimenView {
    pub label: String,
    pub alone_label: String,
    pub deacon_label: String,
    pub priest_label: String,
    pub rubric: String,
    pub opening_versicle: String,
    /// The opening versicle's words either side of its ✠, which the leaf sets in red.
    pub opening_before: String,
    pub opening_after: String,
    pub opening_response: String,
    pub alone_versicle: String,
    pub alone_response: String,
    pub priest_versicle: String,
    pub priest_response: String,
    pub deacon_note: String,
    pub priest_note: String,
    pub secret_voiced: String,
    pub secret_silent: String,
}

/// One liturgical color explained under the Ordo's year ribbon.
#[derive(Clone, Debug, Default, Serialize)]
pub struct AboutOrdoColorView {
    pub slug: String,
    pub name: String,
    pub meaning: String,
    /// Dates in this year for rare colors ("Mar 15, Dec 13"), or day count ("92 days in 2026").
    pub note: String,
}

/// One month's slice of the full-year liturgical color ribbon shown in "The Ordo".
#[derive(Clone, Debug, Default, Serialize)]
pub struct AboutCourseMonth {
    pub month: u32,
    pub name: &'static str,
    pub href: String,
    pub offset_x: u32,
    pub runs: Vec<CourseRunView>,
    pub short_start: u32,
    pub short_days: u32,
}

/// A subtle diamond pin locating a rare liturgical color (Rose, Black) along a color band.
#[derive(Clone, Debug, Default, Serialize)]
pub struct AboutRibbonPin {
    pub color: String,
    pub left_pct: String,
}

/// Full-year liturgical color ribbon for the About page's "The Ordo" section.
#[derive(Clone, Debug, Default, Serialize)]
pub struct AboutOrdoRibbon {
    pub year: i32,
    /// The days each month's band spans, and the days' widths between two bands.
    pub span: u32,
    pub gap: u32,
    pub width: u32,
    pub today_pct: String,
    pub months: Vec<AboutCourseMonth>,
    /// Rare-color pins (Rose, Black) positioned along the full ribbon.
    pub rare_pins: Vec<AboutRibbonPin>,
    /// Every other month's name, set at its own month's start (a percentage of the width).
    pub labels: Vec<AboutRibbonLabel>,
}

/// A month's name under the ribbon, at the start of its band.
#[derive(Clone, Debug, Default, Serialize)]
pub struct AboutRibbonLabel {
    pub name: &'static str,
    pub left_pct: String,
}

/// The gap between two months' bands on About's ribbon, in days' widths.
pub const ABOUT_RIBBON_GAP: u32 = 3;

/// Builds the six liturgical color legend entries, enriched with this year's dates/counts when `course` is given.
pub fn build_ordo_colors(year: i32, course: Option<&[Vec<presentation::course::CourseRun>]>) -> Vec<AboutOrdoColorView> {
    use calendar::Color;
    presentation::about::ABOUT_ORDO_COLORS
        .iter()
        .map(|c| {
            let note = course
                .and_then(|months| {
                    let color = Color::parse(c.slug).ok()?;
                    Some(presentation::course::color_days_note(months, color, year))
                })
                .unwrap_or_default();
            AboutOrdoColorView { slug: c.slug.into(), name: c.name.into(), meaning: c.meaning.into(), note }
        })
        .collect()
}

/// Computes which liturgical colors in `course` are rare (total days <= `RARE_COLOR_MAX_DAYS`).
pub fn rare_colors_in(course: &[Vec<presentation::course::CourseRun>]) -> Vec<calendar::Color> {
    use calendar::Color;
    let all = [Color::White, Color::Red, Color::Violet, Color::Green, Color::Rose, Color::Black];
    all.into_iter()
        .filter(|&color| {
            let total: u32 = course.iter().flat_map(|m| m.iter()).filter(|r| r.color == color).map(|r| r.days).sum();
            total > 0 && total <= presentation::course::RARE_COLOR_MAX_DAYS
        })
        .collect()
}

impl AboutData {
    /// The page as presentation words it, for the page chrome given, with the hour it is now
    /// marked and invited to as home does at `now`'s `clock_hour` (0–23).
    pub fn new(chrome: Chrome, now: calendar::Date, clock_hour: i8) -> AboutData {
        use crate::escape::html_escape_string as esc;
        use presentation::about::{
            ABOUT, ABOUT_HOROLOGIUM_HEIGHT, ABOUT_HOURS, ABOUT_KICKER, ABOUT_NIGHT, ABOUT_SPECIMEN, ABOUT_TITLE, AboutBlock, AboutRun,
            about_current_hour, about_horologium_angle, about_horologium_is_night, about_horologium_point, about_horologium_sign,
            about_runs,
        };
        let current = about_current_hour(clock_hour);
        let invite = presentation::invitation(now, now, clock_hour);
        let block = |kind| AboutBlockView { kind, ..AboutBlockView::default() };

        let mut verse_lines = Vec::new();
        let mut verse_cite = String::new();
        let mut intro = String::new();
        let mut hours_heading = String::new();
        let mut hours_note = String::new();
        let mut sections: Vec<AboutSection> = Vec::new();
        let mut in_hours = false;

        for (i, b) in ABOUT.iter().enumerate() {
            match *b {
                AboutBlock::Verse { text, cite } => {
                    verse_lines = text.lines().map(String::from).collect();
                    verse_cite = cite.into();
                }
                AboutBlock::Intro(text) => intro = text.into(),
                AboutBlock::Heading(text) => {
                    // The heading over the hours heads the rail, whatever its words.
                    if matches!(ABOUT.get(i + 1), Some(AboutBlock::Hours)) {
                        hours_heading = text.into();
                        in_hours = true;
                    } else {
                        in_hours = false;
                        sections.push(AboutSection { heading: text.into(), blocks: Vec::new() });
                    }
                }
                AboutBlock::Hours => {
                    in_hours = true;
                }
                AboutBlock::Note(text) if in_hours => {
                    hours_note = text.into();
                }
                AboutBlock::Note(text) => {
                    if let Some(sec) = sections.last_mut() {
                        sec.blocks.push(AboutBlockView { text: text.into(), ..block("note") });
                    }
                }
                AboutBlock::Paragraph(text) => {
                    let html = about_runs(text)
                        .into_iter()
                        .map(|run| match run {
                            AboutRun::Text(t) => esc(&t),
                            AboutRun::Link { text, target } => format!("<a href=\"{}\">{}</a>", esc(&target), esc(&text)),
                        })
                        .collect();
                    if let Some(sec) = sections.last_mut() {
                        sec.blocks.push(AboutBlockView { html, ..block("paragraph") });
                    }
                }
                AboutBlock::Specimen => {
                    if let Some(sec) = sections.last_mut() {
                        sec.blocks.push(block("specimen"));
                    }
                }
                AboutBlock::Key { id, mark, tail, spoken, ink, text } => {
                    if let Some(sec) = sections.last_mut() {
                        sec.blocks.push(AboutBlockView {
                            id: id.into(),
                            text: text.into(),
                            mark: mark.into(),
                            tail: tail.into(),
                            spoken: spoken.into(),
                            ink: ink.name(),
                            ..block("key")
                        });
                    }
                }
                AboutBlock::Era { period, text } => {
                    if let Some(sec) = sections.last_mut() {
                        sec.blocks.push(AboutBlockView { period: period.into(), text: text.into(), ..block("era") });
                    }
                }
                AboutBlock::OrdoRibbon => {
                    if let Some(sec) = sections.last_mut() {
                        sec.blocks.push(block("ordo_ribbon"));
                    }
                }
                AboutBlock::Elsewhere => {
                    if let Some(sec) = sections.last_mut() {
                        sec.blocks.push(block("elsewhere"));
                    }
                }
                AboutBlock::Invitation => {}
            }
        }

        // Matins in the Night, then the seven: the table's order and the Horologium's.
        let periods = || std::iter::once(&ABOUT_NIGHT).chain(ABOUT_HOURS.iter());
        let about_hours: Vec<AboutPeriod> = periods()
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
                        coming_soon: presentation::hour_coming_soon(slug),
                    })
                    .collect(),
            })
            .collect();

        // Horologium coordinates: viewBox 320×160, the day's arc over the horizon at y 128 and
        // the night's shallow course under it (presentation::about::about_horologium_point).
        let height = ABOUT_HOROLOGIUM_HEIGHT;
        let point = about_horologium_point;
        let stops: Vec<AboutHorologiumStop> = periods()
            .flat_map(|(_, hours)| hours.iter())
            .map(|(slug, time)| {
                let angle = about_horologium_angle(slug);
                let (x, y) = point(angle, 1.0);
                let (tx, ty) = point(angle, 0.91);
                AboutHorologiumStop {
                    slug: (*slug).into(),
                    name: presentation::title_case(slug),
                    gloss: presentation::hour_gloss(slug).into(),
                    time: (*time).into(),
                    angle,
                    night: about_horologium_is_night(slug),
                    sign: about_horologium_sign(slug),
                    is_current: *slug == current,
                    x: format!("{x:.1}"),
                    y: format!("{y:.1}"),
                    tick_x1: format!("{tx:.1}"),
                    tick_y1: format!("{ty:.1}"),
                    left_pct: format!("{:.2}%", x / 320.0 * 100.0),
                    top_pct: format!("{:.2}%", y / height * 100.0),
                }
            })
            .collect();
        // Between midnight and 2 am, `current` is empty (yesterday's Compline): the Horologium
        // rests calmly on Compline without claiming "Now" for today's row.
        let active_stop = stops.iter().find(|s| s.slug == current).or_else(|| stops.last()).cloned().unwrap_or_default();
        let (ax, ay) = point(active_stop.angle, 1.0);
        let (dx, dy) = point(about_horologium_angle("lauds"), 1.0);
        let horologium = AboutHorologium {
            stops,
            active_slug: active_stop.slug,
            active_name: active_stop.name,
            active_gloss: active_stop.gloss,
            active_time: active_stop.time,
            active_x: format!("{ax:.1}"),
            active_y: format!("{ay:.1}"),
            active_cx_pct: format!("{:.1}%", ax / 320.0 * 100.0),
            active_cy_pct: format!("{:.1}%", ay / height * 100.0),
            active_night: active_stop.night,
            active_is_now: !current.is_empty(),
            dawn_x: format!("{dx:.1}"),
            dawn_y: format!("{dy:.1}"),
        };

        let (opening_before, opening_after) =
            ABOUT_SPECIMEN.opening_versicle.split_once('✠').unwrap_or((ABOUT_SPECIMEN.opening_versicle, ""));
        let specimen = AboutSpecimenView {
            label: ABOUT_SPECIMEN.label.into(),
            alone_label: ABOUT_SPECIMEN.alone_label.into(),
            deacon_label: ABOUT_SPECIMEN.deacon_label.into(),
            priest_label: ABOUT_SPECIMEN.priest_label.into(),
            rubric: ABOUT_SPECIMEN.rubric.into(),
            opening_versicle: ABOUT_SPECIMEN.opening_versicle.into(),
            opening_before: opening_before.into(),
            opening_after: opening_after.into(),
            opening_response: ABOUT_SPECIMEN.opening_response.into(),
            alone_versicle: ABOUT_SPECIMEN.alone_versicle.into(),
            alone_response: ABOUT_SPECIMEN.alone_response.into(),
            priest_versicle: ABOUT_SPECIMEN.priest_versicle.into(),
            priest_response: ABOUT_SPECIMEN.priest_response.into(),
            deacon_note: ABOUT_SPECIMEN.deacon_note.into(),
            priest_note: ABOUT_SPECIMEN.priest_note.into(),
            secret_voiced: ABOUT_SPECIMEN.secret_voiced.into(),
            secret_silent: ABOUT_SPECIMEN.secret_silent.into(),
        };

        let ordo_colors = build_ordo_colors(now.year(), None);

        AboutData {
            chrome,
            kicker: ABOUT_KICKER.into(),
            title: ABOUT_TITLE.into(),
            verse_lines,
            verse_cite,
            intro,
            hours_heading,
            about_hours,
            horologium,
            hours_note,
            coming_soon: presentation::COMING_SOON.into(),
            sections,
            specimen,
            ordo_ribbon: AboutOrdoRibbon { year: now.year(), ..AboutOrdoRibbon::default() },
            ordo_colors,
            ribbon_label: presentation::about::ABOUT_RIBBON_LABEL.into(),
            pray_now_label: invite.label,
            pray_now_link: crate::links::hour_link(invite.hour, &presentation::date_slug(invite.date)),
        }
    }

    /// Attaches the current year's liturgical color course (`presentation::course::year_course`)
    /// so "The Ordo" renders the full 12-month ribbon with today's gilt lozenge.
    pub fn with_course(mut self, now: calendar::Date, course: &[Vec<presentation::course::CourseRun>]) -> Self {
        let span = presentation::course::COURSE_SPAN;
        let gap = ABOUT_RIBBON_GAP;
        let total = 12 * span + 11 * gap;
        let year = now.year();
        let rare = rare_colors_in(course);
        let mut rare_pins = Vec::new();
        let months = course
            .iter()
            .enumerate()
            .map(|(mi, runs)| {
                let offset_x = (mi as u32) * (span + gap);
                let end = runs.last().map_or(0, |r| r.start + r.days);
                let month = (mi as u32) + 1;
                for r in runs {
                    if rare.contains(&r.color) {
                        let mid = f64::from(offset_x + r.start) + f64::from(r.days) * 0.5;
                        rare_pins.push(AboutRibbonPin {
                            color: r.color.as_str().into(),
                            left_pct: format!("{:.3}%", mid / f64::from(total) * 100.0),
                        });
                    }
                }
                AboutCourseMonth {
                    month,
                    name: presentation::MONTHS.get(mi).copied().unwrap_or(""),
                    href: format!("/calendar/{year:04}/{month:02}"),
                    offset_x,
                    runs: runs.iter().map(|r| CourseRunView { start: r.start, days: r.days, color: r.color.as_str().into() }).collect(),
                    short_start: end,
                    short_days: span.saturating_sub(end),
                }
            })
            .collect();
        let today_x = f64::from((now.month() - 1) * (span + gap)) + f64::from(now.day().clamp(1, span)) - 0.5;
        let labels = ["Jan", "Mar", "May", "Jul", "Sep", "Nov"]
            .into_iter()
            .enumerate()
            .map(|(i, name)| AboutRibbonLabel {
                name,
                left_pct: format!("{:.3}%", f64::from(2 * i as u32 * (span + gap)) / f64::from(total) * 100.0),
            })
            .collect();
        self.ordo_colors = build_ordo_colors(year, Some(course));
        self.ordo_ribbon = AboutOrdoRibbon {
            year,
            span,
            gap,
            width: total,
            today_pct: format!("{:.3}%", today_x / f64::from(total) * 100.0),
            months,
            rare_pins,
            labels,
        };
        self
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
    /// A hymn, set in stanzas under its Latin title (`hymn_title`, which may be empty).
    pub hymn: bool,
    pub hymn_title: String,
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
