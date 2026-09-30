//! The Office engine for the native apps. The data directory is compiled in, so the apps work
//! offline and every build prays exactly the corpus it was built from. Hours arrive as
//! `render-blocks` layouts, so Kotlin and Swift set styled text and never parse the corpus grammar.

mod data;

use std::fmt;
use std::sync::{Arc, Mutex};

use calendar::{CalendarData, Date, MoveableDates, build_calendar};
use corpus::typography::typeset;
use liturgy::{OfficeHour, PrayerForm};
use office::summary::{CommSummary, HourSummary, day_name, ordo_day};
use office::{Day, Engine, HOUR_NAMES, resolve_office_days};
use render_html::links::{report_url, season_class, season_label, title_case};

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
        let feast = day_name(day);
        let lower = feast.to_lowercase();
        // A short note for home; the ordo carries the full wording.
        let octave_note = match &day.within_octave_of {
            Some(id) if !id.is_empty() && !lower.contains("octave") => format!("Octave of {}", calendar::builder::octave_display_name(id)),
            _ => String::new(),
        };
        // Not repeated when the celebration already names the season.
        let mut season = title_case(day.season.as_str());
        if lower.contains(&season.to_lowercase()) {
            season.clear();
        }
        let (mut pray_now_hour, mut pray_now_label, mut pray_now_date, mut current_hour) = ("lauds", "Open Lauds".to_string(), shown, "");
        if shown == now {
            let current = current_office(clock_hour);
            pray_now_hour = HOUR_NAMES.iter().find(|h| **h == current.hour).copied().unwrap_or("lauds");
            pray_now_label = format!("Pray {}", title_case(pray_now_hour));
            if current.day_offset != 0 {
                pray_now_date = now.add_days(current.day_offset);
            } else {
                current_hour = pray_now_hour;
            }
        }
        Ok(HomeView {
            date_label: long_date(shown),
            feast: typeset(&feast),
            octave_note,
            season,
            color: day.color.as_str().to_string(),
            ornament: ornament(Some(day.season)),
            penitential: day.penitential.labels().into_iter().map(String::from).collect(),
            commemorations: day.commemorations.iter().map(|c| typeset(&c.name)).collect(),
            is_today: shown == now,
            pray_now_label,
            pray_now_hour: pray_now_hour.to_string(),
            pray_now_date: CivilDate::from(pray_now_date),
            current_hour: current_hour.to_string(),
        })
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

/// The web's schedule (`CURRENT_HOUR_SCHEDULE` in office-web and app.js):
/// from each clock hour, the office being prayed and its day offset.
const CURRENT_HOUR_SCHEDULE: [(i32, &str, i32); 8] = [
    (0, "compline", -1),
    (2, "lauds", 0),
    (7, "prime", 0),
    (9, "terce", 0),
    (11, "sext", 0),
    (13, "none", 0),
    (17, "vespers", 0),
    (20, "compline", 0),
];

#[uniffi::export]
pub fn current_office(clock_hour: i32) -> CurrentOffice {
    let (_, hour, day_offset) = CURRENT_HOUR_SCHEDULE.iter().rev().find(|b| clock_hour >= b.0).copied().unwrap_or(CURRENT_HOUR_SCHEDULE[0]);
    CurrentOffice { hour: hour.to_string(), day_offset }
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

/// "Wednesday, September 30, 2026".
fn long_date(d: Date) -> String {
    format!("{}, {} {}, {}", d.weekday().name(), MONTHS[d.month() as usize - 1], d.day(), d.year())
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
            feast: typeset(&o.name),
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

const MONTHS: [&str; 12] =
    ["January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"];

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
            report_url: report_url(hour, hour_name, &format!("{:04}-{:02}-{:02}", d.year(), d.month(), d.day())),
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
