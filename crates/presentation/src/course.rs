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

use calendar::{CalendarDay, Color, MoveableDates};

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

/// Rare-day threshold for the liturgical color ribbon: colors with at most this many days in the
/// year (Rose: Gaudete & Laetare; Black: Good Friday & All Souls) get subtle diamond pins when
/// selected and name their dates in the legend note.
pub const RARE_COLOR_MAX_DAYS: u32 = 6;

/// Short parenthetical note for a liturgical color across `months` in `year`: dates for rare colors
/// ("Mar 15, Dec 13"), or day count for frequent ones ("92 days in 2026"). Empty if no days match.
pub fn color_days_note(months: &[Vec<CourseRun>], color: Color, year: i32) -> String {
    let mut total: u32 = 0;
    let mut dates: Vec<String> = Vec::new();
    for (mi, runs) in months.iter().enumerate() {
        let abbr = crate::MONTHS.get(mi).map(|m| &m[..3]).unwrap_or("");
        for r in runs {
            if r.color != color {
                continue;
            }
            total += r.days;
            if total <= RARE_COLOR_MAX_DAYS {
                for offset in 0..r.days {
                    dates.push(format!("{abbr} {}", r.start + offset + 1));
                }
            }
        }
    }
    if total == 0 {
        return String::new();
    }
    if total <= RARE_COLOR_MAX_DAYS { dates.join(", ") } else { format!("{total} days in {year}") }
}

/// A day as the date picker marks it (#953): a hairline of its colour under the numeral, and a
/// gilt lozenge on it when home keeps the day as a first-class feast ([`crate::home_first_class`]),
/// so opening "Change date" shows the month's liturgical shape with no words added.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct DayMark {
    pub color: Color,
    pub first_class: bool,
}

/// How a screen reader names a first-class feast in the picker, after the day's date: app.js's
/// FIRST_CLASS_WORDS, and the apps' through mobile-ffi's `first_class_spoken`.
pub const FIRST_CLASS_SPOKEN: &str = "first-class feast";

/// The picker's mark for each of `days` (one year's calendar days, in date order).
pub fn day_marks<'a>(days: impl IntoIterator<Item = &'a CalendarDay>, moveable: &MoveableDates) -> Vec<DayMark> {
    days.into_iter().map(|d| DayMark { color: d.color, first_class: crate::home_first_class(d, moveable) }).collect()
}

/// The year's marks as the web's picker fetches them: a letter a day from the first of January
/// (the colour's [`Color::abbrev`]), a capital on a first-class feast.
pub fn day_marks_code(marks: &[DayMark]) -> String {
    marks
        .iter()
        .map(|m| {
            let letter = m.color.abbrev();
            if m.first_class { letter.to_ascii_uppercase() } else { letter.to_string() }
        })
        .collect()
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

    #[test]
    fn rare_colors_name_their_dates_and_frequent_colors_count_days() {
        let mut months: Vec<Vec<CourseRun>> = vec![Vec::new(); 12];
        months[2].push(CourseRun { start: 14, days: 1, color: Color::Rose });
        months[11].push(CourseRun { start: 12, days: 1, color: Color::Rose });
        months[0].push(CourseRun { start: 0, days: 20, color: Color::White });
        assert_eq!(color_days_note(&months, Color::Rose, 2026), "Mar 15, Dec 13");
        assert_eq!(color_days_note(&months, Color::White, 2026), "20 days in 2026");
        assert_eq!(color_days_note(&months, Color::Black, 2026), "");
    }

    /// The picker's marks spell the year's shape: Christmas gilt on white, Good Friday black and
    /// unmarked (kept, not kept festival), the Sundays of joy in rose; a letter a day.
    #[test]
    fn day_marks_spell_colours_and_gild_first_class_feasts() {
        use calendar::{CalendarData, Date, MoveableDates, build_calendar};
        let data = CalendarData::load(&crate::feast::tests::Data).unwrap();
        let cal = build_calendar(2026, &data).unwrap();
        let m = MoveableDates::compute(2026);
        let marks = day_marks(&cal.days, &m);
        let code = day_marks_code(&marks);
        assert_eq!(code.chars().count(), 365);
        let at = |date: Date| code.chars().nth(cal.days.iter().position(|d| d.date == date).unwrap()).unwrap();
        assert_eq!(at(Date::new(2026, 12, 25)), 'W');
        assert_eq!(at(m.easter), 'W');
        assert_eq!(at(m.easter.add_days(-2)), 'b');
        assert_eq!(code.matches('p').count(), 2, "Gaudete and Laetare: {code}");
        assert!(code.chars().all(|c| "wrgvbpWRGVBP".contains(c)), "{code}");
    }

    /// The web says a first-class day as the apps do.
    #[test]
    fn app_js_speaks_first_class_days_in_the_shared_words() {
        let js =
            std::fs::read_to_string(std::path::Path::new(env!("CARGO_MANIFEST_DIR")).join("../../apps/office-web/static/app.js")).unwrap();
        assert!(js.contains(&format!("var FIRST_CLASS_WORDS = \"{FIRST_CLASS_SPOKEN}\";")), "app.js's FIRST_CLASS_WORDS");
    }
}
