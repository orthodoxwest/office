//! The Office engine for the native apps. The data directory is compiled in, so the apps work
//! offline and every build prays exactly the corpus it was built from. Hours arrive as
//! `render-blocks` layouts, so Kotlin and Swift set styled text and never parse the corpus grammar.

mod data;

use std::fmt;
use std::sync::{Arc, Mutex};

use calendar::{CalendarData, Date, MoveableDates, build_calendar};
use corpus::typography::typeset;
use liturgy::{OfficeHour, PrayerForm};
use office::summary::{CommSummary, HourSummary, ordo_day};
use office::{Day, Engine, HOUR_NAMES, resolve_office_days};
use presentation::{
    MONTHS, REMINDER_DEFAULTS, current_hour_entry, date_slug, day_heading, day_name, invitation, long_date, reminder_description,
    reminder_summary, report_url, season_class, season_label, title_case,
};

pub use data::EmbeddedData;

uniffi::setup_scaffolding!();

#[derive(Debug, uniffi::Error)]
pub enum OfficeError {
    Failed { detail: String },
}

impl fmt::Display for OfficeError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            OfficeError::Failed { detail } => f.write_str(detail),
        }
    }
}

impl std::error::Error for OfficeError {}

fn failed(message: impl Into<String>) -> OfficeError {
    OfficeError::Failed { detail: message.into() }
}

/// One year's office days: the most recent year composed, kept for paging
/// between days and hours.
struct Year {
    year: i32,
    days: Vec<Day>,
    moveable: MoveableDates,
}

/// The loaded engine. Loading parses the whole corpus, so an app keeps one.
#[derive(uniffi::Object)]
pub struct OfficeCore {
    engine: Engine,
    calendar: CalendarData,
    year: Mutex<Option<Arc<Year>>>,
}

#[uniffi::export]
impl OfficeCore {
    #[uniffi::constructor]
    pub fn new() -> Result<Arc<OfficeCore>, OfficeError> {
        let src = EmbeddedData;
        let engine = Engine::load(&src).map_err(|e| failed(format!("creating office engine: {e}")))?;
        let calendar = CalendarData::load(&src).map_err(|e| failed(format!("loading calendar data: {e}")))?;
        Ok(Arc::new(OfficeCore { engine, calendar, year: Mutex::new(None) }))
    }

    /// Composes `hour` (a name from `hour_names`) for the civil date in the
    /// prayer form "private", "deacon", or "priest".
    pub fn compose(&self, hour: String, year: i32, month: i32, day: i32, form: String) -> Result<HourView, OfficeError> {
        let date =
            Date::parse(&format!("{year:04}-{month:02}-{day:02}")).ok_or_else(|| failed(format!("invalid date {year}-{month}-{day}")))?;
        let form = PrayerForm::parse(&form).map_err(failed)?;
        let year = self.year(date.year())?;
        let office_day = year.days.get(date.ordinal() as usize - 1).ok_or_else(|| failed(format!("no office day for {date}")))?;
        let composed = self.engine.compose_hour(&hour, office_day, &year.moveable, form).map_err(failed)?;
        Ok(HourView::new(&hour, &composed))
    }

    /// Home for a civil date, as the web's home handler builds it. `today`
    /// and `clock_hour` are the device's, which decide the invitation.
    pub fn home(&self, date: CivilDate, today: CivilDate, clock_hour: i32) -> Result<HomeView, OfficeError> {
        let shown = date.parse()?;
        let now = today.parse()?;
        let year = self.year(shown.year())?;
        let day = year.days.get(shown.ordinal() as usize - 1).ok_or_else(|| failed(format!("no office day for {shown}")))?;
        let heading = day_heading(day);
        let invite = invitation(shown, now, clock_hour_i8(clock_hour));
        Ok(HomeView {
            date_label: long_date(shown),
            feast: typeset(&heading.feast),
            octave_note: heading.octave_note,
            season: heading.season,
            color: day.color.as_str().to_string(),
            ornament: ornament(Some(day.season)),
            penitential: day.penitential.labels().into_iter().map(String::from).collect(),
            commemorations: day.commemorations.iter().map(|c| typeset(&c.name)).collect(),
            is_today: shown == now,
            pray_now_label: invite.label,
            pray_now_hour: invite.hour.to_string(),
            pray_now_date: CivilDate::from(invite.date),
            current_hour: invite.current.to_string(),
        })
    }

    /// The reminders due over `days` days from `from`: each chosen hour at its
    /// time, on the chosen weekdays (`weekdays[0]` is Sunday), named as the
    /// web's calendar feed names its events. In date and time order.
    pub fn reminders(
        &self,
        from: CivilDate,
        days: i32,
        choices: Vec<ReminderChoice>,
        weekdays: Vec<bool>,
    ) -> Result<Vec<ReminderView>, OfficeError> {
        if weekdays.len() != 7 {
            return Err(failed("weekdays must name all seven days, Sunday first"));
        }
        for c in &choices {
            if !HOUR_NAMES.contains(&c.hour.as_str()) {
                return Err(failed(format!("unknown hour: {}", c.hour)));
            }
            if !(0..24).contains(&c.hour_of_day) || !(0..60).contains(&c.minute) {
                return Err(failed(format!("invalid time {}:{} for {}", c.hour_of_day, c.minute, c.hour)));
            }
        }
        let mut chosen: Vec<&ReminderChoice> = choices.iter().collect();
        chosen.sort_by_key(|c| (c.hour_of_day, c.minute));
        let start = from.parse()?;
        let mut out = Vec::new();
        for i in 0..days.max(0) {
            let date = start.add_days(i);
            if !weekdays[date.weekday().number() as usize] {
                continue;
            }
            let year = self.year(date.year())?;
            let day = year.days.get(date.ordinal() as usize - 1).ok_or_else(|| failed(format!("no office day for {date}")))?;
            for c in &chosen {
                out.push(ReminderView {
                    hour: c.hour.clone(),
                    date: CivilDate::from(date),
                    hour_of_day: c.hour_of_day,
                    minute: c.minute,
                    title: title_case(&c.hour),
                    feast: typeset(&day_name(day)),
                    summary: typeset(&reminder_summary(&c.hour, day)),
                    description: typeset(&reminder_description(day)),
                });
            }
        }
        Ok(out)
    }

    /// One month of the ordo, each day with its office digest. Composes three
    /// hours a day, so call it off the main thread.
    pub fn ordo_month(&self, year: i32, month: i32) -> Result<OrdoMonthView, OfficeError> {
        if !(1..=12).contains(&month) {
            return Err(failed(format!("invalid month {month}")));
        }
        let y = self.year(year)?;
        let days: Vec<OrdoDayView> = y
            .days
            .iter()
            .filter(|d| d.date.month() as i32 == month)
            .map(|d| OrdoDayView::new(d, ordo_day(d, &self.engine, &y.moveable)))
            .collect();
        Ok(OrdoMonthView { name: MONTHS[month as usize - 1].to_string(), year, month, days })
    }
}

impl OfficeCore {
    fn year(&self, year: i32) -> Result<Arc<Year>, OfficeError> {
        let mut cached = self.year.lock().map_err(|e| failed(e.to_string()))?;
        if let Some(y) = cached.as_ref().filter(|y| y.year == year) {
            return Ok(Arc::clone(y));
        }
        let cal = build_calendar(year, &self.calendar).map_err(failed)?;
        let office = resolve_office_days(&cal);
        let days = cal.days.into_iter().zip(office).map(|(c, o)| Day::new(c, o)).collect();
        let y = Arc::new(Year { year, days, moveable: MoveableDates::compute(year) });
        *cached = Some(Arc::clone(&y));
        Ok(y)
    }
}

/// The seven hours, in order.
#[uniffi::export]
pub fn hour_names() -> Vec<String> {
    HOUR_NAMES.iter().map(|h| h.to_string()).collect()
}

/// The office being prayed at a clock hour, and the day it belongs to
/// relative to the civil date (Compline after midnight is yesterday's).
#[derive(Clone, Debug, PartialEq, Eq, uniffi::Record)]
pub struct CurrentOffice {
    pub hour: String,
    pub day_offset: i32,
}

/// The office being prayed at a clock hour (0–23), by the schedule the web shares.
#[uniffi::export]
pub fn current_office(clock_hour: i32) -> CurrentOffice {
    let (hour, _, day_offset) = current_hour_entry(clock_hour_i8(clock_hour));
    CurrentOffice { hour: hour.to_string(), day_offset }
}

/// A native app, as the usage report names it.
#[derive(Clone, Copy, Debug, PartialEq, Eq, uniffi::Enum)]
pub enum UsageClient {
    Android,
    Ios,
}

/// What the apps count, as the web counts its pages: home and the reminders page count toward
/// the day's readers, an hour and the ordo in their own columns too, and turning reminders on
/// as the web counts a generated feed link.
#[derive(Clone, Debug, PartialEq, Eq, uniffi::Enum)]
pub enum UsageEvent {
    Home { date: CivilDate },
    Hour { date: CivilDate, hour: String },
    Ordo { year: i32 },
    RemindersPage,
    RemindersOn,
}

/// The usage beacon for `event`, or none when it does not count: as on the web, only a page
/// for today or a day either side (the ordo: this year or either side) is counted, so reading
/// the archive leaves no trace. `dark` is the appearance on screen; `form` the prayer form.
#[uniffi::export]
pub fn usage_beacon(event: UsageEvent, today: CivilDate, dark: bool, form: String, client: UsageClient) -> Option<String> {
    use presentation::usage::{App, app_beacon, current_day, current_year};
    let today = today.parse().ok()?;
    let scope = match &event {
        UsageEvent::Home { date } => current_day(date.parse().ok()?, today).then_some("site")?,
        UsageEvent::Hour { date, hour } => current_day(date.parse().ok()?, today).then_some(hour.as_str())?,
        UsageEvent::Ordo { year } => current_year(*year, today).then_some("ordo")?,
        UsageEvent::RemindersPage => "site",
        UsageEvent::RemindersOn => "reminders",
    };
    let app = match client {
        UsageClient::Android => App::Android,
        UsageClient::Ios => App::Ios,
    };
    app_beacon(scope, app, dark, &form)
}

/// Where a release build of the apps posts its beacons.
#[uniffi::export]
pub fn usage_endpoint() -> String {
    presentation::usage::ENDPOINT.to_string()
}

/// An hour the reader asks to be reminded of, at a time of day.
#[derive(Clone, Debug, PartialEq, Eq, uniffi::Record)]
pub struct ReminderChoice {
    pub hour: String,
    pub hour_of_day: i32,
    pub minute: i32,
}

/// One reminder: an office on a civil date at a local time, with its words.
#[derive(Clone, Debug, PartialEq, Eq, uniffi::Record)]
pub struct ReminderView {
    pub hour: String,
    pub date: CivilDate,
    pub hour_of_day: i32,
    pub minute: i32,
    /// The office's name ("Vespers").
    pub title: String,
    /// The day it keeps ("III Sunday in Lent").
    pub feast: String,
    /// "Vespers — III Sunday in Lent", as the web's calendar feed names the event.
    pub summary: String,
    /// Rank, season, colour and commemorations, as the feed describes the event.
    pub description: String,
}

/// The reminder page's hours as the web offers them: the suggested time and
/// whether each starts chosen.
#[derive(Clone, Debug, PartialEq, Eq, uniffi::Record)]
pub struct ReminderDefault {
    pub hour: String,
    pub name: String,
    pub hour_of_day: i32,
    pub minute: i32,
    pub chosen: bool,
}

/// One row of the Tabula Temporaria; `date` is the day a date leads to, and a figure has none.
#[derive(Clone, Debug, PartialEq, Eq, uniffi::Record)]
pub struct TabulaRowView {
    pub label: String,
    pub value: String,
    pub date: Option<CivilDate>,
}

/// A year's ordo frontispiece: its title and the Tabula Temporaria.
#[derive(Clone, Debug, PartialEq, Eq, uniffi::Record)]
pub struct OrdoYearView {
    pub year: i32,
    /// "MMXXVI", for "Anno Domini"; empty past the numerals' reach.
    pub roman: String,
    pub figures: Vec<TabulaRowView>,
    pub moveable: Vec<TabulaRowView>,
    pub ember: Vec<TabulaRowView>,
}

/// The year's frontispiece, as the web's /calendar/{year}. Arithmetic only: no engine needed.
#[uniffi::export]
pub fn ordo_year(year: i32) -> OrdoYearView {
    let rows = |rows: Vec<presentation::TabulaRow>| {
        rows.into_iter().map(|r| TabulaRowView { label: r.label.into(), value: r.value, date: r.date.map(CivilDate::from) }).collect()
    };
    let t = presentation::tabula(year);
    OrdoYearView { year, roman: presentation::year_roman(year), figures: rows(t.figures), moveable: rows(t.moveable), ember: rows(t.ember) }
}

#[uniffi::export]
pub fn reminder_defaults() -> Vec<ReminderDefault> {
    REMINDER_DEFAULTS
        .iter()
        .map(|&(hour, name, hh, mm, chosen)| ReminderDefault {
            hour: hour.to_string(),
            name: name.to_string(),
            hour_of_day: hh as i32,
            minute: mm as i32,
            chosen,
        })
        .collect()
}

/// A device's clock hour, held to 0–23.
fn clock_hour_i8(hour: i32) -> i8 {
    hour.clamp(0, 23) as i8
}

/// A civil date across the bindings.
#[derive(Clone, Copy, Debug, PartialEq, Eq, uniffi::Record)]
pub struct CivilDate {
    pub year: i32,
    pub month: i32,
    pub day: i32,
}

impl CivilDate {
    fn parse(self) -> Result<Date, OfficeError> {
        let CivilDate { year, month, day } = self;
        Date::parse(&format!("{year:04}-{month:02}-{day:02}")).ok_or_else(|| failed(format!("invalid date {year}-{month}-{day}")))
    }
}

impl From<Date> for CivilDate {
    fn from(d: Date) -> CivilDate {
        CivilDate { year: d.year(), month: d.month() as i32, day: d.day() as i32 }
    }
}

/// The ornament season that retints the gilding: "passiontide", "eastertide", or empty.
fn ornament(season: Option<calendar::Season>) -> String {
    season_class(season).trim_start_matches("season-").to_string()
}

/// Home's frontispiece for one day.
#[derive(Clone, Debug, uniffi::Record)]
pub struct HomeView {
    pub date_label: String,
    /// The celebration, or the temporal title or feria when there is none.
    pub feast: String,
    pub octave_note: String,
    /// The season's name, empty when the celebration already names it.
    pub season: String,
    pub color: String,
    pub ornament: String,
    /// "Fasting", "Abstinence" and the like, set as red work.
    pub penitential: Vec<String>,
    pub commemorations: Vec<String>,
    pub is_today: bool,
    /// "Pray Vespers" today; "Open Lauds" on another day.
    pub pray_now_label: String,
    pub pray_now_hour: String,
    /// The day the invitation opens: yesterday for Compline after midnight.
    pub pray_now_date: CivilDate,
    /// The hour the directory marks as now, or empty.
    pub current_hour: String,
}

#[derive(Clone, Debug, uniffi::Record)]
pub struct OrdoMonthView {
    pub name: String,
    pub year: i32,
    pub month: i32,
    pub days: Vec<OrdoDayView>,
}

#[derive(Clone, Debug, uniffi::Record)]
pub struct CommemorationView {
    pub name: String,
    pub incipit: String,
}

/// One ordo row, as the web's month page sets it.
#[derive(Clone, Debug, uniffi::Record)]
pub struct OrdoDayView {
    pub date: CivilDate,
    /// "Sun".
    pub weekday: String,
    pub feast: String,
    pub rank: String,
    pub rank_full: String,
    pub color: String,
    pub fast: bool,
    pub abstinence: bool,
    pub commemorations: Vec<String>,
    pub benedictus_antiphon: String,
    pub lauds_preces: bool,
    pub lauds_suffrage: bool,
    pub lauds_comms: Vec<CommemorationView>,
    pub hours_preces: bool,
    pub magnificat_antiphon: String,
    pub vespers_preces: bool,
    pub vespers_suffrage: bool,
    pub vespers_comms: Vec<CommemorationView>,
    pub vespers_note: String,
}

impl OrdoDayView {
    fn new(d: &Day, o: office::summary::OrdoDay) -> OrdoDayView {
        let comms =
            |c: &[CommSummary]| c.iter().map(|c| CommemorationView { name: typeset(&c.name), incipit: typeset(&c.incipit) }).collect();
        let lauds = o.lauds.unwrap_or_else(empty_summary);
        let vespers = o.vespers.unwrap_or_else(empty_summary);
        OrdoDayView {
            date: CivilDate::from(d.date),
            weekday: d.date.weekday().name()[..3].to_string(),
            feast: typeset(&day_name(d)),
            rank: o.rank,
            rank_full: o.rank_full,
            color: o.color.as_str().to_string(),
            fast: o.fast,
            abstinence: o.abstinence,
            commemorations: o.commemorations.iter().map(|c| typeset(c)).collect(),
            benedictus_antiphon: lauds.gospel_ant.clone(),
            lauds_preces: lauds.preces,
            lauds_suffrage: lauds.suffrage,
            lauds_comms: comms(&lauds.comms),
            hours_preces: o.hours_preces,
            magnificat_antiphon: vespers.gospel_ant.clone(),
            vespers_preces: vespers.preces,
            vespers_suffrage: vespers.suffrage,
            vespers_comms: comms(&vespers.comms),
            vespers_note: typeset(&o.vespers_note),
        }
    }
}

fn empty_summary() -> HourSummary {
    HourSummary {
        color: None,
        gospel_ant: String::new(),
        gospel_ant_full: String::new(),
        preces: false,
        suffrage: false,
        comms: Vec::new(),
    }
}

/// A composed hour, laid out for native text.
#[derive(Clone, Debug, uniffi::Record)]
pub struct HourView {
    /// The hour's name, as passed to `compose`.
    pub hour: String,
    pub title: String,
    /// "Wednesday, September 30, 2026".
    pub date_label: String,
    /// The day's celebration; empty on a feria.
    pub feast: String,
    /// The season in lower case ("pentecost"), empty when none.
    pub season: String,
    /// The season as the hour header names it ("Lent", "Eastertide"); empty after Pentecost.
    pub season_label: String,
    /// The ornament season: "passiontide", "eastertide", or empty.
    pub ornament: String,
    /// The liturgical color ("green"), empty when none.
    pub color: String,
    pub sections: Vec<SectionView>,
    /// A prefilled GitHub issue naming this hour, date, and form, as the web's "Report a problem".
    pub report_url: String,
}

impl HourView {
    fn new(hour_name: &str, hour: &OfficeHour) -> HourView {
        let d = hour.date;
        HourView {
            hour: hour_name.to_string(),
            title: hour.title.clone(),
            date_label: long_date(d),
            feast: typeset(&hour.feast),
            season: hour.season.map(|s| s.as_str().to_string()).unwrap_or_default(),
            season_label: hour.season.map(|s| season_label(s.as_str())).unwrap_or_default(),
            ornament: ornament(hour.season),
            color: hour.color.map(|c| c.as_str().to_string()).unwrap_or_default(),
            sections: render_blocks::hour_sections(hour).into_iter().map(SectionView::from).collect(),
            report_url: report_url(hour, hour_name, &date_slug(d)),
        }
    }
}

#[derive(Clone, Debug, uniffi::Record)]
pub struct SectionView {
    pub label: String,
    pub collapsible: bool,
    pub blocks: Vec<BlockView>,
}

impl From<render_blocks::SectionBlocks> for SectionView {
    fn from(s: render_blocks::SectionBlocks) -> SectionView {
        SectionView { label: s.label, collapsible: s.collapsible, blocks: s.blocks.into_iter().map(BlockView::from).collect() }
    }
}

#[derive(Clone, Debug, uniffi::Record)]
pub struct BlockView {
    pub kind: BlockKind,
    pub marker: String,
    pub drop_cap: bool,
    pub starts_element: bool,
    pub runs: Vec<RunView>,
}

impl From<render_blocks::Block> for BlockView {
    fn from(b: render_blocks::Block) -> BlockView {
        BlockView {
            kind: b.kind.into(),
            marker: b.marker,
            drop_cap: b.drop_cap,
            starts_element: b.starts_element,
            runs: b.runs.into_iter().map(|r| RunView { text: r.text, style: r.style.into() }).collect(),
        }
    }
}

#[derive(Clone, Debug, uniffi::Record)]
pub struct RunView {
    pub text: String,
    pub style: RunStyle,
}

/// `render_blocks::BlockKind`, for the bindings.
#[derive(Clone, Copy, Debug, PartialEq, Eq, uniffi::Enum)]
pub enum BlockKind {
    Heading,
    CommemorationHeading,
    ItemLabel,
    LatinTitle,
    ChapterRef,
    ScriptureRef,
    CanticleSection,
    Rubric,
    AntiphonNote,
    Antiphon,
    Verse,
    Paragraph,
    ChantLine,
    Versicle,
    Response,
    All,
    Speaker,
    Stanza,
    Gap,
}

impl From<render_blocks::BlockKind> for BlockKind {
    fn from(k: render_blocks::BlockKind) -> BlockKind {
        use render_blocks::BlockKind as K;
        match k {
            K::Heading => BlockKind::Heading,
            K::CommemorationHeading => BlockKind::CommemorationHeading,
            K::ItemLabel => BlockKind::ItemLabel,
            K::LatinTitle => BlockKind::LatinTitle,
            K::ChapterRef => BlockKind::ChapterRef,
            K::ScriptureRef => BlockKind::ScriptureRef,
            K::CanticleSection => BlockKind::CanticleSection,
            K::Rubric => BlockKind::Rubric,
            K::AntiphonNote => BlockKind::AntiphonNote,
            K::Antiphon => BlockKind::Antiphon,
            K::Verse => BlockKind::Verse,
            K::Paragraph => BlockKind::Paragraph,
            K::ChantLine => BlockKind::ChantLine,
            K::Versicle => BlockKind::Versicle,
            K::Response => BlockKind::Response,
            K::All => BlockKind::All,
            K::Speaker => BlockKind::Speaker,
            K::Stanza => BlockKind::Stanza,
            K::Gap => BlockKind::Gap,
        }
    }
}

/// `render_blocks::RunStyle`, for the bindings.
#[derive(Clone, Copy, Debug, PartialEq, Eq, uniffi::Enum)]
pub enum RunStyle {
    Plain,
    Mediant,
    Cross,
    Prayed,
    Secret,
    Latin,
    Kicker,
    Posture,
    Break,
}

impl From<render_blocks::RunStyle> for RunStyle {
    fn from(s: render_blocks::RunStyle) -> RunStyle {
        use render_blocks::RunStyle as S;
        match s {
            S::Plain => RunStyle::Plain,
            S::Mediant => RunStyle::Mediant,
            S::Cross => RunStyle::Cross,
            S::Prayed => RunStyle::Prayed,
            S::Secret => RunStyle::Secret,
            S::Latin => RunStyle::Latin,
            S::Kicker => RunStyle::Kicker,
            S::Posture => RunStyle::Posture,
            S::Break => RunStyle::Break,
        }
    }
}

#[cfg(test)]
mod tests;
