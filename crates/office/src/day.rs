//! The day as the composers see it: the shared calendar day plus the Office's own resolution
//! (Vespers) and, on the synthetic office-day of an evening, which Vespers it stands for.

use std::ops::{Deref, DerefMut};

use calendar::{CalendarDay, Date, MoveableDates, Season, Weekday};

use crate::concurrence::VespersDesignation;

#[derive(Clone, Debug)]
pub struct Day {
    pub cal: CalendarDay,
    pub vespers: VespersDesignation,
    /// Which Vespers a synthetic office-day stands for, as `vespers_office_day` sets it. `None` on
    /// the civil day, and on the synthetic days that need neither mark (the psalmody day of a
    /// Vespers split at the Chapter, the Office of the Dead).
    pub evening: Option<Evening>,
}

/// The Vespers a synthetic office-day stands for.
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum Evening {
    /// A following feast's I Vespers: `date` is the feast's, the hour is said the evening before,
    /// and resolution prefers "-first" ref variants.
    First,
    /// II Vespers of the day's office.
    Second {
        /// The following celebration commemorated here, which takes its I-Vespers antiphon and
        /// versicle (XIV.14).
        following_commemorated: Option<String>,
    },
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
        Day { cal, vespers: office.vespers, evening: None }
    }

    /// The synthetic office-day of a following feast's I Vespers.
    pub fn first_vespers(&self) -> bool {
        self.evening == Some(Evening::First)
    }

    /// Marks the day as a following feast's I Vespers, or no longer as one. Marking it replaces an
    /// `Evening::Second`, and with it the following office's commemoration id, which I Vespers
    /// never carries.
    pub fn set_first_vespers(&mut self, first: bool) {
        if first {
            self.evening = Some(Evening::First);
        } else if self.first_vespers() {
            self.evening = None;
        }
    }

    /// The following celebration commemorated at this II Vespers, from its I Vespers ("" when none).
    pub fn following_office_commemoration_id(&self) -> &str {
        match &self.evening {
            Some(Evening::Second { following_commemorated: Some(id) }) => id,
            _ => "",
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
        if self.first_vespers() { self.date.add_days(-1) } else { self.date }
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

    /// The season a saint's proper and Common are read in: Easter throughout
    /// Paschaltide, so the Octave of Pentecost keeps the Commons' paschal
    /// forms (#631, #640).
    pub fn saints_season(&self, hour_name: &str) -> Season {
        if self.is_paschaltide(hour_name) { Season::Easter } else { self.season }
    }

    /// Lowercase civil weekday name ("monday").
    pub fn civil_weekday_name(&self) -> String {
        self.civil_weekday().name().to_lowercase()
    }

    /// I Vespers of a Sunday, by the liturgical office date: Low Sunday is a
    /// feast of the Lord, but its I Vespers is still said on Saturday.
    pub fn is_sunday_first_vespers(&self) -> bool {
        self.first_vespers() && self.date.weekday() == Weekday::Sunday
    }

    pub fn celebration_id(&self) -> Option<&str> {
        self.celebration.as_deref().map(|c| c.id.as_str())
    }

    pub fn celebration_is(&self, id: &str) -> bool {
        self.celebration_id() == Some(id)
    }
}
