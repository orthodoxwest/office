//! The day as the composers see it: the shared calendar day plus the Office's own resolution
//! (Vespers, the Marian antiphon) and the two fields that exist only on the synthetic office-day
//! used while composing I Vespers.

use std::ops::{Deref, DerefMut};

use calendar::{CalendarDay, Date, MoveableDates, Season, Weekday};

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
        self.civil_date().weekday()
    }

    /// The civil date on which the hour is said: the eve at I Vespers.
    pub fn civil_date(&self) -> Date {
        if self.first_vespers { self.date.add_days(-1) } else { self.date }
    }

    /// The Saturday before Septuagesima, whose Vespers say Alleluia for the
    /// last time even when they are of the Saturday's own feast (Diurnal
    /// p. 235).
    pub fn is_septuagesima_eve(&self) -> bool {
        let civil = self.civil_date();
        MoveableDates::compute(civil.year()).septuagesima.add_days(-1) == civil
    }

    /// Paschaltide at this hour: Eastertide, and the Octave of Pentecost
    /// "until None of Saturday" before I Vespers of Trinity Sunday (General
    /// Rubrics XXIV.3, XXXI.5; 2026 ordo, 6 June). The calendar's season
    /// turns at Pentecost.
    pub fn is_paschaltide(&self, hour_name: &str) -> bool {
        if self.season == Season::Easter {
            return true;
        }
        let civil = self.civil_date();
        let since = civil.days_since(MoveableDates::compute(civil.year()).pentecost);
        (0..6).contains(&since) || (since == 6 && !matches!(hour_name, "vespers" | "compline"))
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
