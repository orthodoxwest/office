//! The passes that finish every composed hour, as states it moves through. Each later pass reads
//! what an earlier one established, so each state is a type that only the pass before it makes,
//! and a pass that needs a fact takes the state that holds it:
//!
//! - [`Settled`]: the prayer form applied, empty sections dropped, source refs canonical and
//!   uniform antiphons collapsed. The sequence of elements is final from here.
//! - [`Classified`]: every doxology directly after a psalm or canticle is a
//!   [`ElementType::PsalmDoxology`], which the posture cues ([`crate::posture`]) look for.
//! - [`Announced`]: the antiphons said only begun before their psalm are marked `announce`, which
//!   decides how much of a psalm's opening is left unrepeated ([`crate::unrepeated`]).
//!
//! Scores and context decisions depend on none of these and are added to the finished hour.

use liturgy::{ElementType, OfficeHour, PrayerForm};

use crate::day::Day;
use crate::engine::{antiphons_doubled, canonicalize_source_refs, collapse_uniform_antiphons};
use crate::psalmody::VESPERS_OF_THE_DEAD_LABEL;
use crate::texts::OfficeTexts;

/// A composed hour whose sequence of elements is final.
pub struct Settled(OfficeHour);

/// A settled hour whose psalm doxologies are told from other doxologies.
pub struct Classified(OfficeHour);

/// A classified hour whose antiphons said only begun are marked.
pub struct Announced(OfficeHour);

impl Settled {
    /// Applies the prayer form, then fixes the sequence of elements.
    pub fn settle(mut hour: OfficeHour, form: PrayerForm, t: &OfficeTexts) -> Result<Settled, String> {
        crate::leader::apply_leader(&mut hour, form, t)?;
        // The form can leave a section with nothing in it.
        hour.sections.retain(|s| !s.elements.is_empty());
        canonicalize_source_refs(&mut hour, t);
        collapse_uniform_antiphons(&mut hour);
        Ok(Settled(hour))
    }

    /// A doxology directly after a psalm or canticle is a psalm doxology.
    pub fn classify(mut self) -> Classified {
        for section in &mut self.0.sections {
            for i in 1..section.elements.len() {
                if section.elements[i].kind == ElementType::Doxology && section.elements[i - 1].kind.is_psalmody() {
                    section.elements[i].kind = ElementType::PsalmDoxology;
                }
            }
        }
        Classified(self.0)
    }
}

impl Classified {
    pub fn hour_mut(&mut self) -> &mut OfficeHour {
        &mut self.0
    }

    /// Flags the opening antiphon of each psalm when antiphons are not doubled.
    /// Vespers of the Dead is said as a Double.
    pub fn announce(mut self, day: &Day, hour_name: &str) -> Announced {
        let doubled = antiphons_doubled(day, hour_name);
        let mut in_dead = false;
        for section in &mut self.0.sections {
            if section.label == VESPERS_OF_THE_DEAD_LABEL {
                in_dead = true;
            }
            if doubled || in_dead {
                continue;
            }
            let n = section.elements.len();
            for i in 0..n {
                if section.elements[i].kind == ElementType::Antiphon && i + 1 < n && section.elements[i + 1].kind.is_psalmody() {
                    section.elements[i].announce = true;
                }
            }
        }
        Announced(self.0)
    }
}

impl Announced {
    pub fn hour_mut(&mut self) -> &mut OfficeHour {
        &mut self.0
    }

    pub fn into_hour(self) -> OfficeHour {
        self.0
    }
}
