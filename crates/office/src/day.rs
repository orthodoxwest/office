//! The day as the composers see it: the shared calendar day plus the Office's
//! own resolution (Vespers, the Marian antiphon) and the two fields that exist
//! only on the synthetic office-day used while composing I Vespers. This is
//! Go's whole `models.CalendarDay`.

use std::ops::{Deref, DerefMut};

use calendar::{CalendarDay, Weekday};

use crate::concurrence::VespersDesignation;

#[derive(Clone, Debug)]
pub struct Day {
    pub cal: CalendarDay,
    /// Corpus subkey under `ordinary/marian/`.
    pub marian_antiphon: String,
    pub vespers: VespersDesignation,
    /// Set only on the synthetic office-day of a following feast's I Vespers:
    /// resolution then prefers "-first" ref variants.
    pub first_vespers: bool,
    /// Copied from Vespers onto the synthetic office-day ("" when absent).
    pub following_office_commemoration_id: String,
}

impl Deref for Day {
    type Target = CalendarDay;
    fn deref(&self) -> &CalendarDay {
        &self.cal
    }
}

impl DerefMut for Day {
    fn deref_mut(&mut self) -> &mut CalendarDay {
        &mut self.cal
    }
}

impl Day {
    pub fn new(cal: CalendarDay, office: crate::OfficeDay) -> Day {
        Day {
            cal,
            marian_antiphon: office.marian_antiphon.to_string(),
            vespers: office.vespers,
            first_vespers: false,
            following_office_commemoration_id: String::new(),
        }
    }

    /// The weekday of the psalter and weekday ordinary. At I Vespers the
    /// office day has advanced to the following feast; the psalter keeps the
    /// civil evening on which Vespers is said.
    pub fn civil_weekday(&self) -> Weekday {
        let date = if self.first_vespers { self.date.add_days(-1) } else { self.date };
        date.weekday()
    }

    /// Lowercase civil weekday name ("monday").
    pub fn civil_weekday_name(&self) -> String {
        self.civil_weekday().name().to_lowercase()
    }

    /// I Vespers of a Sunday, by the liturgical office date: Low Sunday is a
    /// feast of the Lord, but its I Vespers is still said on Saturday.
    pub fn is_sunday_first_vespers(&self) -> bool {
        self.first_vespers && self.date.weekday() == Weekday::Sunday
    }

    pub fn celebration_id(&self) -> Option<&str> {
        self.celebration.as_deref().map(|c| c.id.as_str())
    }

    pub fn celebration_is(&self, id: &str) -> bool {
        self.celebration_id() == Some(id)
    }
}
