//! Days bound together: the ordo's colour rail beside each day, run unbroken down the days of an
//! octave as printed kalendars bracket one. The web's month rows and the apps' ordo draw what
//! this says; it adds no words and changes no office.

use calendar::builder::OCTAVE_DAYS_KEPT_BY_OWN_ENTRY;
use calendar::model::OctaveClass;
use calendar::traits::octave_parent_id;
use calendar::{CalendarDay, Color, FeastRef, YearCalendar};

/// One day's rail. Outside a bound octave it is the day's own colour standing alone.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct Rail {
    /// The rail's colour: the day's own, or on a day another feast impedes, the octave's.
    pub color: Color,
    /// The impeding feast's own colour, set as a short bar beside the rail that runs on.
    pub inset: Option<Color>,
    /// The rail runs up into the day before, which is of the same octave.
    pub above: bool,
    /// The rail runs down into the day after.
    pub below: bool,
}

impl Rail {
    /// Outside any bound octave: today's lone bar.
    pub fn alone(color: Color) -> Rail {
        Rail { color, inset: None, above: false, below: false }
    }
}

/// Whether an octave of this group is drawn bound. Needs ruling #838 (question 1): until clergy
/// rule, the privileged octaves of the Lord and the common octaves of I Class feasts are bound
/// (option b). A simple octave is never bound, since only its octave day is kept (Rubrics VII.1).
pub fn octave_is_bound(class: OctaveClass) -> bool {
    match class {
        OctaveClass::PrivilegedFirst | OctaveClass::PrivilegedSecond | OctaveClass::PrivilegedThird | OctaveClass::Common => true,
        OctaveClass::Simple => false,
    }
}

/// The bound octave a day is within, as the engine assigns it (the governing octave where two
/// overlap, so the other is not drawn).
fn bound_octave<'a>(day: &CalendarDay, feasts: &'a [FeastRef]) -> Option<&'a FeastRef> {
    let id = day.within_octave_of.as_deref()?;
    feasts.iter().find(|f| f.id == id && f.has_octave).filter(|f| octave_is_bound(f.octave_class))
}

/// The day says the octave's own office: the feast, one of its generated days, an entry kept as
/// one of its days (Easter Monday, the Circumcision), or an `OctaveOf` day (St Paul). Any other
/// office within it impedes the octave, which runs on beneath it (#838, question 2, A: VII.1
/// counts the octave "for eight continuous days" whether its office or its commemoration is said).
fn says_octave(day: &CalendarDay, octave: &FeastRef) -> bool {
    let Some(c) = day.celebration.as_deref() else { return false };
    c.id == octave.id
        || octave_parent_id(c) == Some(octave.id.as_str())
        || OCTAVE_DAYS_KEPT_BY_OWN_ENTRY.iter().any(|&(o, _, entry)| o == octave.id && entry == c.id)
}

/// Each day's rail for a year's calendar, in the order of `cal.days`. The previous December and
/// the next January decide whether the year's first and last rails run off the page.
pub fn octave_rails(cal: &YearCalendar, feasts: &[FeastRef]) -> Vec<Rail> {
    let octave = |d: &CalendarDay| bound_octave(d, feasts).map(|f| f.id.as_str());
    let days: Vec<&CalendarDay> =
        std::iter::once(&cal.preceding_dec31).chain(&cal.days).chain(std::iter::once(&cal.following_jan1)).collect();
    days.windows(3)
        .map(|w| {
            let (before, day, after) = (w[0], w[1], w[2]);
            let Some(o) = bound_octave(day, feasts) else { return Rail::alone(day.color) };
            let (color, inset) = if says_octave(day, o) { (day.color, None) } else { (o.color, Some(day.color)) };
            Rail { color, inset, above: octave(before) == Some(o.id.as_str()), below: octave(after) == Some(o.id.as_str()) }
        })
        .collect()
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::feast::tests::Data;
    use calendar::{CalendarData, Date, build_calendar};

    /// `(month, day, rail, inset, above, below)` for the days of `year` from `from` to `to`.
    fn rails(year: i32, from: (u32, u32), to: (u32, u32)) -> Vec<(u32, u32, &'static str, &'static str, bool, bool)> {
        let data = CalendarData::load(&Data).unwrap();
        let cal = build_calendar(year, &data).unwrap();
        let r = octave_rails(&cal, &data.feasts);
        let (from, to) = (Date::new(year, from.0 as i32, from.1 as i32), Date::new(year, to.0 as i32, to.1 as i32));
        cal.days
            .iter()
            .zip(r)
            .filter(|(d, _)| d.date >= from && d.date <= to)
            .map(|(d, r)| (d.date.month(), d.date.day(), r.color.as_str(), r.inset.map_or("", |c| c.as_str()), r.above, r.below))
            .collect()
    }

    /// The Nativity binds 25 December to 1 January across the year's turn (#833): St Stephen,
    /// St John, the Holy Innocents, the Sunday within the octave and St Sylvester impede it and stand beside its
    /// white (#838, question 2, A); 30 December is its own office, and the Circumcision its eighth
    /// day. The vigil before and St Stephen's octave day after stand alone.
    #[test]
    fn the_nativity_is_bound_across_the_new_year() {
        assert_eq!(
            rails(2026, (12, 24), (12, 31)),
            [
                (12, 24, "violet", "", false, false),
                (12, 25, "white", "", false, true),
                (12, 26, "white", "red", true, true),
                (12, 27, "white", "white", true, true),
                (12, 28, "white", "red", true, true),
                (12, 29, "white", "white", true, true),
                (12, 30, "white", "", true, true),
                (12, 31, "white", "white", true, true),
            ]
        );
        assert_eq!(rails(2027, (1, 1), (1, 2)), [(1, 1, "white", "", true, false), (1, 2, "red", "", false, false)]);
    }

    /// Easter is bound through Low Sunday, which the engine keeps within the octave; Pentecost ends
    /// on its Saturday (Rubrics VII.2), so Trinity is not bound to it. In 2027 the Baptist's octave
    /// counts from 24 June though his feast is transferred to the 28th, so Trinity begins it,
    /// impeding it. Ss Peter & Paul's octave meets the Baptist's and does not join it, and gives
    /// way on 1 July to Corpus Christi's, the worthier.
    #[test]
    fn easter_and_pentecost_end_where_the_engine_ends_them() {
        let easter = rails(2027, (5, 1), (5, 10));
        assert_eq!(easter.first(), Some(&(5, 1, "white", "", false, false)));
        assert_eq!(easter[1], (5, 2, "white", "", false, true));
        assert_eq!(easter[8], (5, 9, "white", "", true, false));
        assert_eq!(easter[9], (5, 10, "red", "", false, false));
        let pentecost = rails(2027, (6, 19), (6, 30));
        assert_eq!(pentecost[0], (6, 19, "white", "", false, false));
        assert_eq!(pentecost[1], (6, 20, "red", "", false, true));
        assert_eq!(pentecost[7], (6, 26, "red", "", true, false));
        assert_eq!(pentecost[8], (6, 27, "white", "white", false, true));
        assert_eq!(pentecost[9], (6, 28, "white", "", true, false));
        assert_eq!(pentecost[10], (6, 29, "red", "", false, true));
        // Corpus Christi, 1 July, begins its own octave, which governs where the two overlap.
        assert_eq!(pentecost[11], (6, 30, "white", "", true, false));
    }

    /// Needs ruling #838 (question 1): the common octaves are bound with the privileged, and a
    /// simple octave never is. Flip `octave_is_bound` and this test with the ruling.
    #[test]
    fn privileged_and_common_octaves_are_bound_and_simple_ones_are_not() {
        for class in [OctaveClass::PrivilegedFirst, OctaveClass::PrivilegedSecond, OctaveClass::PrivilegedThird, OctaveClass::Common] {
            assert!(octave_is_bound(class), "{class:?}");
        }
        assert!(!octave_is_bound(OctaveClass::Simple));
        // All Saints' common octave, 1 to 8 November 2026.
        let all_saints = rails(2026, (10, 31), (11, 9));
        assert!(!all_saints[0].4 && !all_saints[0].5 && !all_saints[9].4, "{all_saints:?}");
        assert!(all_saints[1..9].iter().all(|r| r.4 || r.5), "{all_saints:?}");
    }
}
