//! The Office: what only the Office resolves from the shared calendar
//! (Vespers concurrence, the Compline Marian antiphon, the historia weeks),
//! and, as the port proceeds, the hour composers (RUST-PORT.md).
//!
//! Like the calendar crate, this crate does no file, network, or clock access.

pub mod concurrence;
pub mod scopes;
pub mod seasonal;
pub mod texts;

use calendar::{CalendarDay, MoveableDates, YearCalendar};

pub use concurrence::{VespersDesignation, VespersOwner};

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
