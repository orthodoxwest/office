//! Date-driven Office selections: the Compline Marian antiphon and the
//! scripture-cycle ("historia") weeks of August–November.

use calendar::{Date, MoveableDates, Weekday};

/// The corpus subkey under `ordinary/marian/` for the antiphon at Compline.
///
/// - alma-redemptoris-advent: day before Advent 1 through Dec 24
/// - alma-redemptoris-christmas: Dec 25 through Feb 1
/// - ave-regina-caelorum: Feb 2 through Holy Wednesday
/// - regina-caeli: Holy Saturday through Friday of the Pentecost octave
/// - salve-regina: Saturday of the Pentecost octave until Advent
///
/// Regina Caeli is sung through None of the Saturday; Salve Regina begins
/// with I Vespers of Trinity and is therefore that evening's antiphon.
pub fn marian_antiphon(date: Date, m: &MoveableDates) -> &'static str {
    let year = date.year();
    let day_before_advent1 = m.advent1.add_days(-1);
    let dec24 = Date::new(year, 12, 24);
    let dec25 = Date::new(year, 12, 25);
    let feb2 = Date::new(year, 2, 2);
    let pentecost_friday = m.pentecost.add_days(5);
    let pentecost_saturday = m.pentecost.add_days(6);
    let two_days_before_advent1 = m.advent1.add_days(-2);

    if date >= day_before_advent1 && date <= dec24 {
        return "alma-redemptoris-advent";
    }
    if date >= dec25 || date < feb2 {
        return "alma-redemptoris-christmas";
    }
    if date <= m.holy_wednesday {
        return "ave-regina-caelorum";
    }
    if date >= m.holy_saturday && date <= pentecost_friday {
        return "regina-caeli";
    }
    if date >= pentecost_saturday && date <= two_days_before_advent1 {
        return "salve-regina";
    }
    // Holy Thursday and Good Friday fall through to Salve Regina.
    "salve-regina"
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
