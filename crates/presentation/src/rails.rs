//! Days bound together: the ordo's colour rail beside each day, run unbroken down the days of an
//! octave as printed kalendars bracket one, and a vigil's thread into its feast. The web's month
//! rows and the apps' ordo draw what this says; it adds no words and changes no office.

use calendar::builder::OCTAVE_DAYS_KEPT_BY_OWN_ENTRY;
use calendar::model::OctaveClass;
use calendar::traits::octave_parent_id;
use calendar::{CalendarDay, Color, FeastRef, YearCalendar};

/// One day's rail. Outside a bound octave it is the day's own colour standing alone.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct Rail {
    /// The rail's colour: within a strand the octave's, else the day's own.
    pub color: Color,
    /// The day's own colour, set as a short bar beside the strand, where the day says another
    /// office or keeps another colour.
    pub inset: Option<Color>,
    /// The rail runs up into the day before, which is of the same octave.
    pub above: bool,
    /// The rail runs down into the day after.
    pub below: bool,
    /// A vigil's thread (`vigil_leads_into_feast`): on the vigil's row, the colour of the next
    /// day's rail, its feast's, into which a thin line runs from the foot of this rail across
    /// the rule.
    pub lead_to: Option<Color>,
    /// On the feast's row, the colour of the vigil's rail above, from which the thread comes
    /// down into the top of this rail.
    pub lead_from: Option<Color>,
}

impl Rail {
    /// Outside any bound octave: today's lone bar.
    pub fn alone(color: Color) -> Rail {
        Rail { color, inset: None, above: false, below: false, lead_to: None, lead_from: None }
    }
}

/// The day's office is a vigil, and the next day's office is its feast: the vigil leads into the
/// feast, whose First Vespers are said on the vigil's evening. Needs ruling #838 (question 3(d)):
/// until clergy rule, the ordo draws a thin thread from the vigil's rail into the feast's, and
/// only here. Not from a vigil that is only commemorated (Rubrics VI.1: a Double or an octave day
/// occurs, as for Ss Peter & Paul within the Baptist's octave), nor across a day between: a vigil
/// anticipated on Saturday because its day is a Sunday, or a feast transferred away from it.
pub fn vigil_leads_into_feast(vigil: &CalendarDay, next: &CalendarDay) -> bool {
    let (Some(v), Some(f)) = (vigil.celebration.as_deref(), next.celebration.as_deref()) else { return false };
    v.is_vigil && v.vigil_of.as_deref() == Some(f.id.as_str()) && next.date == vigil.date.add_days(1)
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
///
/// A strand is drawn in the octave's colour from end to end. A day whose own colour is not the
/// octave's, or that says another office (#838, question 2, A), shows its own colour beside the
/// strand: St Stephen's red beside the Nativity's white, St Paul's white beside the red of Ss
/// Peter & Paul. A day bound to neither neighbour is not a strand: it keeps its lone bar in its
/// own colour, as outside any octave.
///
/// A vigil said as the day's office leads into its feast the next day (`vigil_leads_into_feast`;
/// #838, question 3(d)) by a thread, never by the strand: the two rails stay apart. Across a
/// month's or a year's last day the thread runs off the foot and in at the top, as a strand does.
pub fn octave_rails(cal: &YearCalendar, feasts: &[FeastRef]) -> Vec<Rail> {
    let days: Vec<&CalendarDay> =
        std::iter::once(&cal.preceding_dec31).chain(&cal.days).chain(std::iter::once(&cal.following_jan1)).collect();
    // The two days beyond the year are railed against the year's side only: enough to say
    // whether they are bound to it, and their colour.
    let mut rails: Vec<Rail> =
        (0..days.len()).map(|i| rail(i.checked_sub(1).map(|j| days[j]), days[i], days.get(i + 1).copied(), feasts)).collect();
    for i in 0..days.len() - 1 {
        if vigil_leads_into_feast(days[i], days[i + 1]) && !rails[i].below && !rails[i + 1].above {
            rails[i].lead_to = Some(rails[i + 1].color);
            rails[i + 1].lead_from = Some(rails[i].color);
        }
    }
    rails.truncate(days.len() - 1);
    rails.remove(0);
    rails
}

/// One day's rail between its neighbours (`None` beyond the days known).
fn rail(before: Option<&CalendarDay>, day: &CalendarDay, after: Option<&CalendarDay>, feasts: &[FeastRef]) -> Rail {
    let octave = |d: Option<&CalendarDay>| d.and_then(|d| bound_octave(d, feasts)).map(|f| f.id.as_str());
    let Some(o) = bound_octave(day, feasts) else { return Rail::alone(day.color) };
    let (above, below) = (octave(before) == Some(o.id.as_str()), octave(after) == Some(o.id.as_str()));
    if !above && !below {
        return Rail::alone(day.color);
    }
    let inset = (day.color != o.color || !says_octave(day, o)).then_some(day.color);
    Rail { color: o.color, inset, above, below, lead_to: None, lead_from: None }
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
        // St Paul's day is the octave's own, in white (the ordos' "Lauds W"), beside its red.
        // Corpus Christi, 1 July, begins its own octave, which governs where the two overlap.
        assert_eq!(pentecost[11], (6, 30, "red", "white", true, false));
    }

    /// A row bound to neither neighbour is drawn as outside any octave: its own colour, no inset.
    /// Overlapping and transferred octaves leave such one-day remnants (Trinity 2027's like days
    /// in 2018, 2029, 2035, 2040, 2043, 2046 and 2051), and they must not read as a strand.
    #[test]
    fn a_row_bound_to_no_neighbour_keeps_its_lone_bar() {
        let data = CalendarData::load(&Data).unwrap();
        for year in 2017..=2053 {
            let cal = build_calendar(year, &data).unwrap();
            for (d, r) in cal.days.iter().zip(octave_rails(&cal, &data.feasts)) {
                if !r.above && !r.below {
                    assert_eq!(Rail { lead_to: None, lead_from: None, ..r }, Rail::alone(d.color), "{}", d.date);
                }
            }
        }
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

    /// `(month, day, lead_to, lead_from)` for the days of `year` from `from` to `to`.
    fn leads(year: i32, from: (u32, u32), to: (u32, u32)) -> Vec<(u32, u32, &'static str, &'static str)> {
        let data = CalendarData::load(&Data).unwrap();
        let cal = build_calendar(year, &data).unwrap();
        let r = octave_rails(&cal, &data.feasts);
        let (from, to) = (Date::new(year, from.0 as i32, from.1 as i32), Date::new(year, to.0 as i32, to.1 as i32));
        cal.days
            .iter()
            .zip(r)
            .filter(|(d, _)| d.date >= from && d.date <= to)
            .map(|(d, r)| (d.date.month(), d.date.day(), r.lead_to.map_or("", |c| c.as_str()), r.lead_from.map_or("", |c| c.as_str())))
            .collect()
    }

    /// Needs ruling #838 (question 3(d)): a vigil said as the day's office leads into its feast
    /// the next day by a thread, its colour passing from the vigil's into the feast's. Christmas
    /// Eve's violet runs into the white that begins the Nativity's strand, the Vigil of St
    /// Laurence's violet into his red, the Vigil of Pentecost's white into its red, and the
    /// Vigil of All Saints off October's page into November's. Flip `vigil_leads_into_feast`
    /// and this test with the ruling.
    #[test]
    fn a_vigil_leads_into_its_feast_the_next_day() {
        assert_eq!(leads(2026, (12, 23), (12, 26)), [(12, 23, "", ""), (12, 24, "white", ""), (12, 25, "", "violet"), (12, 26, "", "")]);
        assert_eq!(leads(2027, (8, 9), (8, 10)), [(8, 9, "red", ""), (8, 10, "", "violet")]);
        assert_eq!(leads(2027, (6, 19), (6, 20)), [(6, 19, "red", ""), (6, 20, "", "white")]);
        assert_eq!(leads(2027, (1, 5), (1, 6)), [(1, 5, "white", ""), (1, 6, "", "white")]);
        assert_eq!(leads(2026, (10, 31), (11, 1)), [(10, 31, "white", ""), (11, 1, "", "violet")]);
        // The vigil is not bound to the feast's strand: its own rail stays a lone bar.
        assert_eq!(rails(2026, (12, 24), (12, 25)), [(12, 24, "violet", "", false, false), (12, 25, "white", "", false, true)]);
    }

    /// No thread from an anticipated vigil (St Andrew's, kept on Saturday 28 November 2026 with
    /// Advent Sunday between), from a vigil only commemorated (Ss Peter & Paul's on 28 June 2028,
    /// within the Baptist's octave), or to a feast transferred away from its vigil (the Baptist
    /// over Corpus Christi, 2038).
    #[test]
    fn no_thread_across_a_day_or_from_a_commemorated_vigil() {
        assert!(leads(2026, (11, 27), (11, 30)).iter().all(|l| l.2.is_empty() && l.3.is_empty()));
        assert!(leads(2028, (6, 27), (6, 29)).iter().all(|l| l.2.is_empty() && l.3.is_empty()));
        assert!(leads(2038, (6, 22), (6, 26)).iter().all(|l| l.2.is_empty() && l.3.is_empty()));
    }

    /// Every thread, 2017 to 2053: it runs from a vigil said as the day's office to its feast
    /// kept the next day, both ends carry each other's rail colour, and neither end is bound
    /// across the rule between them (no vigil said as the office falls within a strand). About
    /// ten a year: 360 in these 37 years, and 273 in 2026 to 2053, as the
    /// M8 mock's harvest of the engine's calendar counted them.
    #[test]
    fn every_thread_joins_a_vigil_to_its_feast_the_next_day() {
        let data = CalendarData::load(&Data).unwrap();
        let (mut all, mut since_2026) = (0, 0);
        for year in 2017..=2053 {
            let cal = build_calendar(year, &data).unwrap();
            let r = octave_rails(&cal, &data.feasts);
            for i in 0..cal.days.len() {
                let (d, next) = (&cal.days[i], cal.days.get(i + 1).unwrap_or(&cal.following_jan1));
                let leads = vigil_leads_into_feast(d, next);
                assert_eq!(r[i].lead_to.is_some(), leads, "{}", d.date);
                let before = if i == 0 { &cal.preceding_dec31 } else { &cal.days[i - 1] };
                assert_eq!(r[i].lead_from.is_some(), vigil_leads_into_feast(before, d), "{}", d.date);
                if !leads {
                    continue;
                }
                assert!(!r[i].below, "{}", d.date);
                if let Some(n) = r.get(i + 1) {
                    assert!(!n.above, "{}", d.date);
                    assert_eq!((r[i].lead_to, n.lead_from), (Some(n.color), Some(r[i].color)), "{}", d.date);
                }
                all += 1;
                if year >= 2026 {
                    since_2026 += 1;
                }
            }
        }
        assert_eq!(since_2026, 273);
        assert_eq!(all, 360);
    }
}
