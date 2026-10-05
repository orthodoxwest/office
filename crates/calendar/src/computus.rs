//! The Julian paschalion, the moveable dates derived from it, the season of
//! a date, and the Tabula Temporaria.

use crate::date::{Date, Weekday, is_leap_year};
use crate::model::Season;

/// Pascha by the Julian paschalion (Meeus), expressed as a proleptic
/// Gregorian date.
pub fn julian_easter(year: i32) -> Date {
    let a = year % 4;
    let b = year % 7;
    let c = year % 19;
    let d = (19 * c + 15) % 30;
    let e = (2 * a + 4 * b - d + 34) % 7;
    let month = (d + e + 114) / 31; // 3 = March, 4 = April (Julian)
    let day = ((d + e + 114) % 31) + 1;
    Date::new(year, month, day).add_days(julian_to_gregorian_offset(year))
}

/// Days to add to a Julian date to reach the Gregorian (13 for 1901–2099).
fn julian_to_gregorian_offset(year: i32) -> i32 {
    let century = year / 100;
    century - century / 4 - 2
}

/// Every moveable date of a civil year.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct MoveableDates {
    pub septuagesima: Date,
    pub sexagesima: Date,
    pub quinquagesima: Date,
    pub ash_wednesday: Date,
    pub lent1: Date,
    pub lent2: Date,
    pub lent3: Date,
    pub lent4: Date,
    pub passion_sunday: Date,
    pub palm_sunday: Date,
    pub holy_monday: Date,
    pub holy_tuesday: Date,
    pub holy_wednesday: Date,
    pub holy_thursday: Date,
    pub good_friday: Date,
    pub holy_saturday: Date,
    pub easter: Date,
    pub easter_monday: Date,
    pub easter_tuesday: Date,
    pub low_sunday: Date,
    pub ascension: Date,
    pub pentecost: Date,
    pub trinity_sunday: Date,
    pub corpus_christi: Date,
    pub advent1: Date,
    pub advent2: Date,
    pub advent3: Date,
    pub advent4: Date,
}

impl MoveableDates {
    pub fn compute(year: i32) -> MoveableDates {
        let easter = julian_easter(year);
        let offset = |days| easter.add_days(days);

        // The IV Sunday of Advent is the last Sunday before Christmas.
        let christmas = Date::new(year, 12, 25);
        let advent4 =
            if christmas.weekday() == Weekday::Sunday { christmas.add_days(-7) } else { christmas.add_days(-christmas.weekday().number()) };

        MoveableDates {
            septuagesima: offset(-63),
            sexagesima: offset(-56),
            quinquagesima: offset(-49),
            ash_wednesday: offset(-46),
            lent1: offset(-42),
            lent2: offset(-35),
            lent3: offset(-28),
            lent4: offset(-21),
            passion_sunday: offset(-14),
            palm_sunday: offset(-7),
            holy_monday: offset(-6),
            holy_tuesday: offset(-5),
            holy_wednesday: offset(-4),
            holy_thursday: offset(-3),
            good_friday: offset(-2),
            holy_saturday: offset(-1),
            easter,
            easter_monday: offset(1),
            easter_tuesday: offset(2),
            low_sunday: offset(7),
            ascension: offset(39),
            pentecost: offset(49),
            trinity_sunday: offset(56),
            corpus_christi: offset(60),
            advent1: advent4.add_days(-21),
            advent2: advent4.add_days(-14),
            advent3: advent4.add_days(-7),
            advent4,
        }
    }
}

/// The liturgical season of a date.
pub fn determine_season(date: Date, m: &MoveableDates) -> Season {
    let year = date.year();
    if date.month() == 12 && date.day() >= 25 {
        return Season::Christmas;
    }
    if date.month() == 1 && date.day() <= 5 {
        return Season::Christmas;
    }
    if date >= m.advent1 && date <= Date::new(year, 12, 24) {
        return Season::Advent;
    }
    if date >= m.passion_sunday && date <= m.holy_saturday {
        return Season::Passiontide;
    }
    if date >= m.ash_wednesday && date < m.passion_sunday {
        return Season::Lent;
    }
    if date >= m.septuagesima && date < m.ash_wednesday {
        return Season::Septuagesima;
    }
    if date >= m.easter && date < m.pentecost {
        return Season::Easter;
    }
    if date >= m.pentecost && date < m.advent1 {
        return Season::Pentecost;
    }
    if date >= Date::new(year, 1, 6) && date < m.septuagesima {
        return Season::Epiphany;
    }
    // Unreachable for valid dates; retain Christmas as the fallback.
    Season::Christmas
}

/// One of the four seasonal sets of Ember Days.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct EmberSet {
    pub wed: Date,
    pub fri: Date,
    pub sat: Date,
}

impl EmberSet {
    fn from_wednesday(wed: Date) -> EmberSet {
        EmberSet { wed, fri: wed.add_days(2), sat: wed.add_days(3) }
    }
}

/// The Tabula Temporaria figures the archdiocesan ordo prints for a year.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Tabula {
    pub year: i32,
    pub golden_number: i32,
    pub dominical_letter: char,
    pub sundays_after_epiphany: i32,
    pub sundays_after_pentecost: i32,
    pub spring: EmberSet,
    pub summer: EmberSet,
    pub autumn: EmberSet,
    pub winter: EmberSet,
}

impl Tabula {
    /// The paschalion is Julian; the golden number and dominical letter follow
    /// the civil Gregorian year, as the printed ordo lists them.
    pub fn compute(year: i32) -> Tabula {
        let m = MoveableDates::compute(year);
        Tabula {
            year,
            golden_number: year % 19 + 1,
            dominical_letter: dominical_letter(year),
            sundays_after_epiphany: count_sundays(Date::new(year, 1, 6).add_days(1), m.septuagesima),
            sundays_after_pentecost: count_sundays(m.trinity_sunday, m.advent1),
            spring: EmberSet::from_wednesday(first_wednesday_after(m.lent1)),
            summer: EmberSet::from_wednesday(first_wednesday_after(m.pentecost)),
            autumn: EmberSet::from_wednesday(first_wednesday_after(Date::new(year, 9, 14))),
            winter: EmberSet::from_wednesday(first_wednesday_after(m.advent3)),
        }
    }
}

/// The first Wednesday strictly after `d`.
fn first_wednesday_after(d: Date) -> Date {
    let offset = (Weekday::Wednesday.number() - d.weekday().number() + 7) % 7;
    d.add_days(if offset == 0 { 7 } else { offset })
}

/// Sundays in the half-open interval [start, end).
fn count_sundays(start: Date, end: Date) -> i32 {
    let mut s = start.add_days((7 - start.weekday().number()) % 7);
    let mut n = 0;
    while s < end {
        n += 1;
        s = s.add_days(7);
    }
    n
}

/// The letter falling on Sundays from March onward (in leap years Feb 29
/// carries no letter, and the ordo prints the later letter).
fn dominical_letter(year: i32) -> char {
    let mar1 = Date::new(year, 3, 1);
    let sunday = mar1.add_days((7 - mar1.weekday().number()) % 7);
    let doy = sunday.ordinal() as i32;
    let idx = if is_leap_year(year) { doy - 2 } else { doy - 1 };
    char::from(b'A' + idx.rem_euclid(7) as u8)
}

/// `n` as a Roman numeral; empty for n <= 0.
pub fn roman(n: i32) -> String {
    const VALUES: [(i32, &str); 13] = [
        (1000, "M"),
        (900, "CM"),
        (500, "D"),
        (400, "CD"),
        (100, "C"),
        (90, "XC"),
        (50, "L"),
        (40, "XL"),
        (10, "X"),
        (9, "IX"),
        (5, "V"),
        (4, "IV"),
        (1, "I"),
    ];
    let mut n = n;
    let mut out = String::new();
    for (v, s) in VALUES {
        while n >= v {
            out.push_str(s);
            n -= v;
        }
    }
    out
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn julian_easter_dates_and_weekdays() {
        for (year, m, d) in [(2024, 5, 5), (2025, 4, 20), (2026, 4, 12), (2027, 5, 2), (2028, 4, 16), (2029, 4, 8), (2030, 4, 28)] {
            assert_eq!(julian_easter(year), Date::new(year, m, d), "{year}");
        }
        for year in 1900..2200 {
            assert_eq!(julian_easter(year).weekday(), Weekday::Sunday, "{year}");
        }
    }

    #[test]
    fn tabula_matches_ordos() {
        for (year, golden, letter, after_epiphany, spring, summer, autumn, winter) in
            [(2026, 13, 'D', 4, (3, 4), (6, 3), (9, 16), (12, 16)), (2024, 11, 'F', 8, (3, 27), (6, 26), (9, 18), (12, 18))]
        {
            let t = Tabula::compute(year);
            assert_eq!(t.golden_number, golden);
            assert_eq!(t.dominical_letter, letter);
            assert_eq!(t.sundays_after_epiphany, after_epiphany);
            assert_eq!(t.spring.wed, Date::new(year, spring.0, spring.1));
            assert_eq!(t.summer.wed, Date::new(year, summer.0, summer.1));
            assert_eq!(t.autumn.wed, Date::new(year, autumn.0, autumn.1));
            assert_eq!(t.winter.wed, Date::new(year, winter.0, winter.1));
        }
    }
}
