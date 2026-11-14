//! The year's course: under the ordo's twelve-month strip runs a thin band of each day's
//! liturgical colour, the rows' own rail laid on its side, so the year is seen whole: Lent long,
//! Pascha moving, the fixed feasts standing. Every front draws it from these runs, the web as a
//! row of SVG rects in the strip, the apps on their canvases.
//!
//! Each month is drawn on a span of [`COURSE_SPAN`] days whatever its length, so February is
//! honestly short and the strip's hairline shows where a month ends early. The colours are the
//! calendar's own (`CalendarDay::color`, which the ordo's rows show): nothing here groups days by
//! season, so from July to November, when nearly every other day keeps a feast, the band is a run
//! of flecks, as the ordo is.

use calendar::{CalendarDay, Color};

/// The days each month's band is drawn across: the longest month's.
pub const COURSE_SPAN: u32 = 31;

/// Days of one colour in a row within a month: `start` days from the month's first (0-based).
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct CourseRun {
    pub start: u32,
    pub days: u32,
    pub color: Color,
}

/// The year's twelve bands, January first, each the runs of its days' colours in order. `days`
/// are the year's calendar days in date order (a month's days must be contiguous).
pub fn year_course<'a>(days: impl IntoIterator<Item = &'a CalendarDay>) -> Vec<Vec<CourseRun>> {
    let mut months: Vec<Vec<CourseRun>> = vec![Vec::new(); 12];
    for d in days {
        let runs = &mut months[d.date.month() as usize - 1];
        let at = d.date.day() - 1;
        match runs.last_mut() {
            Some(r) if r.color == d.color && r.start + r.days == at => r.days += 1,
            _ => runs.push(CourseRun { start: at, days: 1, color: d.color }),
        }
    }
    months
}

/// Today's place along its month's band, as a fraction of the band's width: the middle of the
/// day's own stretch, where the today lozenge hangs. `day` is the day of the month (1-based).
pub fn course_place(day: u32) -> f64 {
    (f64::from(day.clamp(1, COURSE_SPAN)) - 0.5) / f64::from(COURSE_SPAN)
}

/// [`course_place`] as a CSS percentage ("41.935%"), as the web's strip sets it.
pub fn course_place_percent(day: u32) -> String {
    format!("{:.3}%", course_place(day) * 100.0)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn a_day_hangs_at_the_middle_of_its_own_stretch() {
        assert_eq!(course_place_percent(1), "1.613%");
        assert_eq!(course_place_percent(13), "40.323%");
        assert_eq!(course_place_percent(31), "98.387%");
        assert_eq!(course_place(0), course_place(1));
    }
}
