//! Date-driven Office selections: the Final Antiphon of Our Lady and the
//! scripture-cycle ("historia") weeks of August–November.

use calendar::{Date, MoveableDates, Weekday};

/// The corpus subkey under `ordinary/marian/` for the Final Antiphon of Our
/// Lady said at `hour_name` on the civil `date`. The Diurnal bounds each
/// antiphon by hour, not by day (Final Antiphons B.V.M., pp. 153-155; 2026
/// ordo, Advent and Paschaltide notes):
///
/// - Alma Redemptoris: from Vespers of the Saturday before Advent I through
///   II Vespers of the Purification, Feb 2; its Advent versicle and collect
///   give way to the Christmas ones at I Vespers of the Nativity
/// - Ave Regina caelorum: from Compline of Feb 2 through Compline of Holy
///   Wednesday
/// - Regina caeli: from Compline of Holy Saturday through None of the
///   Saturday in the Octave of Pentecost
/// - Salve Regina: from I Vespers of Trinity through None of the Saturday
///   before Advent
///
/// Holy Thursday and Good Friday say none and fall through to Salve Regina.
pub fn marian_antiphon(date: Date, hour_name: &str, m: &MoveableDates) -> &'static str {
    let now = (date, hour_order(hour_name));
    let from = |day: Date, hour: &str| now >= (day, hour_order(hour));
    let year = date.year();
    if from(m.advent1.add_days(-1), "vespers") {
        return if from(Date::new(year, 12, 24), "vespers") { "alma-redemptoris-christmas" } else { "alma-redemptoris-advent" };
    }
    if !from(Date::new(year, 2, 2), "compline") {
        return "alma-redemptoris-christmas";
    }
    if date <= m.holy_wednesday {
        return "ave-regina-caelorum";
    }
    if from(m.holy_saturday, "compline") && !from(m.pentecost.add_days(6), "vespers") {
        return "regina-caeli";
    }
    "salve-regina"
}

/// The order of the hours within one civil day; the Little Hours sit
/// between Lauds and Vespers.
fn hour_order(hour_name: &str) -> u8 {
    match hour_name {
        "lauds" => 0,
        "vespers" => 2,
        "compline" => 3,
        _ => 1,
    }
}

const HISTORIA_MONTHS: [&str; 4] = ["august", "september", "october", "november"];

/// The historia month-week ("august-1" … "november-5") governing the week
/// containing `date`, or `None` before the first liturgical Sunday of August
/// and from Advent onward. A liturgical month begins on the Sunday nearest
/// the first (a first falling Thursday–Saturday defers to the next Sunday);
/// November's later weeks count back from Advent so the last reads week 5.
pub fn historia_week_id(date: Date) -> Option<String> {
    let sunday = date.add_days(-date.weekday().number());
    let year = sunday.year();
    let advent1 = advent_sunday(year);
    if sunday >= advent1 {
        return None;
    }
    let mut lit: Option<(usize, Date)> = None;
    for (i, m) in (8..=11).enumerate() {
        let first = Date::new(year, m, 1);
        let mut start = first.add_days(-first.weekday().number());
        if first.weekday().number() >= Weekday::Thursday.number() {
            start = start.add_days(7);
        }
        if sunday < start {
            break;
        }
        lit = Some((i, start));
    }
    let (month, start) = lit?;
    let mut week = sunday.days_since(start) / 7;
    if month == 3 && week > 0 {
        // Count back from Advent: the week ending at Advent is the fifth.
        week = 4 - (advent1.days_since(sunday) * 24 - 24) / (24 * 7);
    }
    Some(format!("{}-{}", HISTORIA_MONTHS[month], week + 1))
}

/// The First Sunday of Advent: the Sunday nearest St Andrew (Nov 30).
fn advent_sunday(year: i32) -> Date {
    let andrew = Date::new(year, 11, 30);
    let w = andrew.weekday().number();
    if w <= 3 { andrew.add_days(-w) } else { andrew.add_days(7 - w) }
}
