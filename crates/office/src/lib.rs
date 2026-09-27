//! The Office: what only the Office resolves from the shared calendar
//! (Vespers concurrence, the Compline Marian antiphon, the historia weeks),
//! and, as the port proceeds, the hour composers (RUST-PORT.md).
//!
//! Like the calendar crate, this crate does no file, network, or clock access.

pub mod commemoration;
pub mod compline;
pub mod conclusion;
pub mod concurrence;
pub mod day;
pub mod engine;
pub mod hourdef;
pub mod hymn;
pub mod lauds_psalmody;
pub mod leader;
pub mod major;
pub mod minor;
pub mod preces;
pub mod prime;
pub mod proper;
pub mod psalmody;
pub mod rubric;
pub mod scopes;
pub mod seasonal;
pub mod summary;
pub mod texts;
pub mod trace;
pub mod validate;
pub mod vespers;
pub mod voice;

#[cfg(test)]
mod conclusion_tests;
#[cfg(test)]
mod preces_tests;
#[cfg(test)]
mod proper_tests;
#[cfg(test)]
mod testutil;

use calendar::{CalendarDay, MoveableDates, YearCalendar};

pub use concurrence::{VespersDesignation, VespersOwner};
pub use day::Day;
pub use engine::{ComposeOptions, Engine, HOUR_NAMES};

/// The Office-only resolution of one day.
#[derive(Clone, Debug)]
pub struct OfficeDay {
    /// Corpus subkey under `ordinary/marian/`.
    pub marian_antiphon: &'static str,
    pub vespers: VespersDesignation,
}

/// Resolves the Office days of a built calendar year, in order.
pub fn resolve_office_days(cal: &YearCalendar) -> Vec<OfficeDay> {
    let moveable = MoveableDates::compute(cal.year);
    concurrence::resolve_vespers(&cal.days, &cal.following_jan1)
        .into_iter()
        .zip(&cal.days)
        .map(|(vespers, day): (VespersDesignation, &CalendarDay)| OfficeDay {
            marian_antiphon: seasonal::marian_antiphon(day.date, &moveable),
            vespers,
        })
        .collect()
}
