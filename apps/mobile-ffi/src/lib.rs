//! The Office engine for the native apps. The data directory is compiled in, so the apps work
//! offline and every build prays exactly the corpus it was built from. Hours arrive as
//! `render-blocks` layouts, so Kotlin and Swift set styled text and never parse the corpus grammar.

mod data;

use std::fmt;
use std::sync::{Arc, Mutex};

use calendar::{CalendarData, Date, MoveableDates, build_calendar};
use liturgy::{OfficeHour, PrayerForm};
use office::{Day, Engine, HOUR_NAMES, resolve_office_days};

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
    /// The liturgical color ("green"), empty when none.
    pub color: String,
    pub sections: Vec<SectionView>,
}

impl HourView {
    fn new(hour_name: &str, hour: &OfficeHour) -> HourView {
        let d = hour.date;
        HourView {
            hour: hour_name.to_string(),
            title: hour.title.clone(),
            date_label: format!("{}, {} {}, {}", d.weekday().name(), MONTHS[d.month() as usize - 1], d.day(), d.year()),
            feast: hour.feast.clone(),
            season: hour.season.map(|s| s.as_str().to_string()).unwrap_or_default(),
            color: hour.color.map(|c| c.as_str().to_string()).unwrap_or_default(),
            sections: render_blocks::hour_sections(hour).into_iter().map(SectionView::from).collect(),
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
    pub runs: Vec<RunView>,
}

impl From<render_blocks::Block> for BlockView {
    fn from(b: render_blocks::Block) -> BlockView {
        BlockView {
            kind: b.kind.into(),
            marker: b.marker,
            drop_cap: b.drop_cap,
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
            S::Break => RunStyle::Break,
        }
    }
}

#[cfg(test)]
mod tests;
