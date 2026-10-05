//! The calendar pipeline: feasts and computed Sundays → dates → occurrence →
//! the resolved day, with fasting applied.

use std::collections::HashMap;
use std::sync::Arc;

use crate::computus::{MoveableDates, determine_season, roman};
use crate::date::{Date, Weekday, is_leap_year};
use crate::loader::CalendarData;
use crate::model::{CalendarDay, Category, Color, FERIA_COMMEMORATION_ID, Feast, FeastRef, MonthDay, Rank, Season, non_empty};
use crate::occurrence::resolve_day;
use crate::penitential::{PenitentialRule, apply_penitential_rules};
use crate::traits::is_penitential_feria_season;
use data_format::atoi;

/// Short names for octave and vigil references.
const SHORT_NAMES: [(&str, &str); 14] = [
    ("easter-sunday", "Easter"),
    ("pentecost", "Pentecost"),
    ("christmas", "Christmas"),
    ("epiphany", "the Epiphany"),
    ("ascension", "the Ascension"),
    ("corpus-christi", "Corpus Christi"),
    ("ss-peter-paul", "Ss Peter & Paul"),
    ("assumption-bvm", "the Assumption"),
    ("nativity-bvm", "the Nativity of the B.V.M."),
    ("conception-bvm", "the Conception of the B.V.M."),
    ("all-saints", "All Saints"),
    ("nativity-john-baptist", "St John the Baptist"),
    ("st-matthias", "St Matthias"),
    ("st-bartholomew", "St Bartholomew"),
];

fn short_name_for(id: &str) -> Option<&'static str> {
    SHORT_NAMES.iter().find(|(k, _)| *k == id).map(|(_, v)| *v)
}

/// The feast's name without its titles, for octave and vigil references:
/// "St James, Apostle" → "Vigil of St James".
fn short_name(feast: &Feast) -> String {
    short_name_for(&feast.id).map_or_else(|| feast.name.split(", ").next().unwrap_or_default().to_string(), str::to_string)
}

/// The display name of an octave's parent feast ("christmas" → "Christmas").
pub fn octave_display_name(feast_id: &str) -> String {
    short_name_for(feast_id).map_or_else(|| title_case(&feast_id.replace('-', " ")), str::to_string)
}

/// "advent ember wednesday" → "Advent Ember Wednesday".
pub fn title_case(s: &str) -> String {
    s.split_whitespace()
        .map(|w| {
            let mut chars = w.chars();
            match chars.next() {
                // Uppercase the initial byte of these ASCII weekday names.
                Some(c) => c.to_uppercase().chain(chars).collect::<String>(),
                None => String::new(),
            }
        })
        .collect::<Vec<_>>()
        .join(" ")
}

fn fixed(date: Date) -> Option<MonthDay> {
    Some(MonthDay { month: date.month(), day: date.day() })
}

fn first_sunday_after_epiphany(year: i32) -> Date {
    let epiphany = Date::new(year, 1, 6);
    epiphany.add_days(7 - epiphany.weekday().number())
}

fn sunday(id: String, name: String, rank: Rank, color: Color) -> Feast {
    Feast::synthetic(id, name, rank, color, Category::Sunday)
}

/// The Sundays after Epiphany that fall before Septuagesima.
fn epiphany_sunday_feasts(year: i32, septuagesima: Date) -> Vec<Feast> {
    let mut feasts = Vec::new();
    let mut current = first_sunday_after_epiphany(year);
    let mut n = 1;
    while current < septuagesima {
        let (name, color) = if n == 1 {
            ("Sunday within the Octave of the Epiphany".to_string(), Color::White)
        } else {
            (format!("{} Sunday after Epiphany", roman(n)), Color::Green)
        };
        let mut f = sunday(format!("epiphany-sunday-{n}"), name, Rank::SemiDouble, color);
        f.date_rule = Some(format!("epiphany-sunday-{n}"));
        if n == 1 {
            f.proper_id = Some("epiphany-sunday-within-octave".to_string());
            // When the Octave Day falls on Sunday, the Office of the Sunday
            // within the Octave is said on the preceding Saturday (Diurnal
            // p. 228; 2019 ordo, 12-13 January).
            if current == Date::new(year, 1, 13) {
                f.date_rule = None;
                f.fixed = fixed(current.add_days(-1));
            }
        }
        feasts.push(f);
        current = current.add_days(7);
        n += 1;
    }
    // In years with seven or eight Sundays after Epiphany, VII and VIII use
    // the XXII and XXIII Pentecost offices.
    match feasts.len() {
        7 => feasts[6].proper_id = Some("pentecost-sunday-23".to_string()),
        8 => {
            feasts[6].proper_id = Some("pentecost-sunday-22".to_string());
            feasts[7].proper_id = Some("pentecost-sunday-23".to_string());
        }
        _ => {}
    }
    feasts
}

/// The office of the first Sunday after Epiphany displaced by Septuagesima,
/// anticipated on the preceding Saturday, unless it will be resumed in the
/// autumn.
fn anticipated_epiphany_sunday_feast(m: &MoveableDates) -> Option<Feast> {
    let first = first_sunday_after_epiphany(m.septuagesima.year());
    let fitted = m.septuagesima.days_since(first) / 7;
    let n = fitted + 1;
    let trinity = m.easter.add_days(56);
    let surplus = (m.advent1.days_since(trinity) / 7 - 24).max(0);
    if n > 6 || n > 6 - surplus {
        return None;
    }
    let mut f = sunday(
        format!("epiphany-sunday-{n}-anticipated"),
        format!("Office of the {} Sunday after Epiphany", roman(n)),
        Rank::SemiDouble,
        Color::Green,
    );
    f.proper_id = Some(format!("epiphany-sunday-{n}"));
    f.date_rule = Some("easter-64".to_string()); // Saturday before Septuagesima
    Some(f)
}

/// A Sunday after Pentecost left over before the last Sunday, anticipated on
/// the preceding Saturday (General Rubrics, Sundays 4): the XXIII when there
/// are 23 Sundays after Pentecost (2022 ordo, 19 November), or the XXII when
/// there are 22 and the VII Sunday after Epiphany already took the XXIII
/// (2021 ordo, 20 November). None when Epiphany VII/VIII used them all.
fn anticipated_pentecost_sunday_feast(m: &MoveableDates, epiphany_sundays: usize) -> Option<Feast> {
    let trinity = m.easter.add_days(56);
    let total = m.advent1.days_since(trinity) / 7;
    let used_after_epiphany: &[i32] = match epiphany_sundays {
        7 => &[23],
        8 => &[22, 23],
        _ => &[],
    };
    let n = (total..=23).rev().find(|n| !used_after_epiphany.contains(n))?;
    let saturday = m.advent1.add_days(-8);
    let mut f = sunday(
        format!("pentecost-sunday-{n}-anticipated"),
        format!("Office of the {} Sunday after Pentecost", roman(n)),
        Rank::SemiDouble,
        Color::Green,
    );
    f.proper_id = Some(format!("pentecost-sunday-{n}"));
    f.date_rule = Some(format!("easter+{}", saturday.days_since(m.easter)));
    Some(f)
}

/// The Sundays after Easter through Trinity.
fn eastertide_sunday_feasts() -> Vec<Feast> {
    [
        ("easter-sunday-2", "II Sunday after Easter", 14, Color::White),
        ("easter-sunday-3", "III Sunday after Easter", 21, Color::White),
        ("easter-sunday-4", "IV Sunday after Easter", 28, Color::White),
        ("easter-sunday-5", "V Sunday after Easter", 35, Color::White),
        ("ascension-sunday-within-octave", "Sunday within the Octave of the Ascension", 42, Color::White),
        ("pentecost-sunday-1", "I Sunday after Pentecost", 56, Color::Green),
    ]
    .into_iter()
    .map(|(id, name, offset, color)| {
        let mut f = sunday(id.to_string(), name.to_string(), Rank::SemiDouble, color);
        f.date_rule = Some(format!("easter+{offset}"));
        f
    })
    .collect()
}

fn advent_sunday_feasts() -> Vec<Feast> {
    let names = ["I Sunday of Advent", "II Sunday of Advent", "III Sunday of Advent (Gaudete)", "IV Sunday of Advent"];
    let colors = [Color::Violet, Color::Violet, Color::Rose, Color::Violet];
    (0..4)
        .map(|i| {
            let mut f = sunday(format!("advent-sunday-{}", i + 1), names[i].to_string(), Rank::Double2ndClass, colors[i]);
            f.date_rule = Some(format!("advent-sunday-{}", i + 1));
            f
        })
        .collect()
}

/// The Sundays II–XXIV after Pentecost. Surplus Sundays between the XXIII
/// and the last are the resumed Sundays after Epiphany; the last Sunday
/// always takes the XXIV propers.
fn pentecost_sunday_feasts(easter: Date, advent1: Date) -> Vec<Feast> {
    let first = easter.add_days(56);
    let total = advent1.days_since(first) / 7;
    let surplus = total - 24;
    let straight = if surplus > 0 { 23 } else { 24 };
    let mut feasts = Vec::new();
    for n in 2..=straight {
        let offset = 49 + n * 7;
        if easter.add_days(offset) >= advent1 {
            break;
        }
        let (name, color) = if n == 2 {
            // Easter+63 always falls within the octave of Corpus Christi.
            ("Sunday within the Octave of Corpus Christi".to_string(), Color::White)
        } else {
            (format!("{} Sunday after Pentecost", roman(n)), Color::Green)
        };
        let mut f = sunday(format!("pentecost-sunday-{n}"), name, Rank::SemiDouble, color);
        f.date_rule = Some(format!("easter+{offset}"));
        feasts.push(f);
    }
    if surplus > 0 {
        for i in 0..surplus {
            let n = 6 - surplus + 1 + i;
            let offset = 49 + (24 + i) * 7;
            let mut f = sunday(
                format!("epiphany-sunday-{n}-resumed"),
                format!("{} Sunday after Epiphany (Resumed)", roman(n)),
                Rank::SemiDouble,
                Color::Green,
            );
            f.proper_id = Some(format!("epiphany-sunday-{n}"));
            f.date_rule = Some(format!("easter+{offset}"));
            feasts.push(f);
        }
        let mut f =
            sunday("pentecost-sunday-24".to_string(), "XXIV & Last Sunday after Pentecost".to_string(), Rank::SemiDouble, Color::Green);
        f.date_rule = Some(format!("easter+{}", 49 + (24 + surplus) * 7));
        feasts.push(f);
    }
    if let Some(last) = feasts.last_mut()
        && last.id != "pentecost-sunday-24"
    {
        let mut f = sunday(last.id.clone(), "XXIV & Last Sunday after Pentecost".to_string(), last.rank, last.color);
        f.category = last.category;
        f.proper_id = Some("pentecost-sunday-24".to_string());
        f.date_rule = last.date_rule.clone();
        f.notes = Some("Uses 24th Sunday propers".to_string());
        *last = f;
    }
    feasts
}

/// The Sunday within the Nativity Octave, observed Dec 29 when the only
/// Sunday falls on Dec 26–28 or Christmas is itself a Sunday.
fn nativity_octave_sunday_feast(year: i32) -> Feast {
    let mut observed = Date::new(year, 12, 29);
    for day in 26..=31 {
        let candidate = Date::new(year, 12, day);
        if candidate.weekday() != Weekday::Sunday {
            continue;
        }
        if day >= 29 {
            observed = candidate;
        }
        break;
    }
    let mut f = sunday(
        "nativity-sunday-within-octave".to_string(),
        "Sunday within the Octave of the Nativity".to_string(),
        Rank::SemiDouble,
        Color::White,
    );
    f.fixed = fixed(observed);
    f
}

/// Vigils of feasts with `HasVigil`, anticipated to Saturday from Sunday.
fn vigil_feasts(feasts: &[FeastRef], year: i32, m: &MoveableDates) -> Vec<Feast> {
    let mut vigils = Vec::new();
    for feast in feasts.iter().filter(|f| f.has_vigil) {
        let Some(parent) = resolve_feast_date(feast, year, m) else { continue };
        let mut date = parent.add_days(-1);
        if date.weekday() == Weekday::Sunday {
            date = date.add_days(-1);
        }
        let mut v = Feast::synthetic(
            format!("vigil-of-{}", feast.id),
            format!("Vigil of {}", short_name(feast)),
            Rank::Simple,
            Color::Violet,
            Category::Feria,
        );
        v.fixed = fixed(date);
        v.is_vigil = true;
        v.vigil_of = Some(feast.id.clone());
        v.proper_name = feast.proper_name.clone();
        vigils.push(v);
    }
    vigils
}

/// "From Ash Wednesday, all Octaves cease until Low Sunday inclusive"
/// (every ordo 2017–2026; Diurnal VII.1, VII.3). Easter's own octave is the
/// exception. A day omitted here keeps its number: St George's octave resumes
/// after Low Sunday counting from April 23 (2025 ordo, 30 April).
fn octave_ceases(feast: &Feast, date: Date, m: &MoveableDates) -> bool {
    feast.id != "easter-sunday" && date >= m.ash_wednesday && date <= m.low_sunday
}

/// Days 2–8 of every octave.
fn octave_feasts(feasts: &[FeastRef], year: i32, m: &MoveableDates) -> Vec<Feast> {
    use crate::model::OctaveClass;
    let mut generated = Vec::new();
    for feast in feasts.iter().filter(|f| f.has_octave) {
        let Some(parent) = resolve_feast_date(feast, year, m) else { continue };
        let is_privileged = feast.octave_class == OctaveClass::PrivilegedFirst;
        let is_simple = feast.octave_class == OctaveClass::Simple;
        let is_christmas = feast.id == "christmas";
        for day_num in 2..=8 {
            let date = parent.add_days(day_num - 1);
            if is_christmas && day_num == 8 {
                continue; // Jan 1 has its own feast
            }
            if octave_ceases(feast, date, m) {
                continue;
            }
            if is_privileged {
                if feast.id == "easter-sunday" && matches!(day_num, 2 | 3 | 8) {
                    continue; // explicit entries for Monday, Tuesday, Low Sunday
                }
                if feast.id == "pentecost" && day_num == 8 {
                    continue; // no separate octave day on Trinity Sunday
                }
            }
            let is_octave_day = day_num == 8;
            let rank = if is_christmas {
                Rank::SemiDouble
            } else if is_privileged {
                Rank::Double1stClass
            } else if is_simple {
                Rank::Simple
            } else if is_octave_day {
                Rank::GreaterDouble
            } else {
                Rank::SemiDouble
            };
            let short = short_name(feast);
            let n = roman(day_num);
            let name = match (feast.octave_day_names.get(&day_num), &feast.octave_days) {
                (Some(name), _) => name.clone(),
                (None, Some(pattern)) if !is_octave_day => pattern.replace("{n}", &n).replace("{weekday}", date.weekday().name()),
                _ if is_octave_day => format!("Octave Day of {short}"),
                _ => format!("Day {n} within the Octave of {short}"),
            };
            let id = if is_octave_day { format!("{}-octave-day", feast.id) } else { format!("{}-octave-day-{day_num}", feast.id) };
            // Ferial days within the octave take per-day antiphon sets in
            // course, skipping Sundays.
            let mut proper_id = feast.id.clone();
            if !is_octave_day {
                let set_idx = (2..=day_num).filter(|n| parent.add_days(n - 1).weekday() != Weekday::Sunday).count();
                if date.weekday() != Weekday::Sunday {
                    proper_id = format!("{}-octave-set-{set_idx}", feast.id);
                }
            }
            let mut f = Feast::synthetic(id, name, rank, feast.color, Category::Feria);
            f.category = feast.category;
            f.proper_id = Some(proper_id);
            f.octave_class = feast.octave_class;
            f.is_privileged_octave_day = feast.octave_class.privileged() && !is_octave_day;
            if feast.is_fixed() {
                f.fixed = fixed(date);
            } else {
                f.date_rule = Some(format!("easter+{}", date.days_since(m.easter)));
            }
            generated.push(f);
        }
    }
    generated
}

/// The Advent and September Ember Days.
fn remaining_ember_feasts(m: &MoveableDates, year: i32) -> Vec<Feast> {
    let advent_wed = m.advent3.add_days(3);
    let sep15 = Date::new(year, 9, 15);
    let sep_wed = sep15.add_days((Weekday::Wednesday.number() - sep15.weekday().number() + 7) % 7);
    let mut feasts = Vec::new();
    for (season, wed) in [("advent", advent_wed), ("september", sep_wed)] {
        for offset in [0, 2, 3] {
            let date = wed.add_days(offset);
            let weekday = date.weekday().name();
            let mut f = Feast::synthetic(
                format!("{season}-ember-{}", weekday.to_lowercase()),
                format!("Ember {weekday} in {}", title_case(season)),
                Rank::PrivilegedFeria,
                Color::Violet,
                Category::Feria,
            );
            f.fixed = fixed(date);
            feasts.push(f);
        }
    }
    feasts
}

/// Roman bissextile: in leap years, fixed feasts of Feb 24–28 move one day.
fn adjust_fixed_date_for_leap_year(year: i32, month: u32, day: u32, feast_id: &str) -> (u32, u32) {
    if feast_id.starts_with("vigil-") {
        return (month, day);
    }
    if month == 2 && (24..=28).contains(&day) && is_leap_year(year) {
        return (month, day + 1);
    }
    (month, day)
}

/// Parses an integer, returning 0 on failure.
fn atoi_or_zero(s: &str) -> i32 {
    atoi(s).ok().and_then(|n| i32::try_from(n).ok()).unwrap_or(0)
}

/// The feast's date in `year`, or `None` when its rule does not apply.
pub fn resolve_feast_date(feast: &Feast, year: i32, m: &MoveableDates) -> Option<Date> {
    if let Some(md) = feast.fixed {
        let (month, day) = if feast.skip_roman_leap_shift {
            (md.month, md.day)
        } else {
            adjust_fixed_date_for_leap_year(year, md.month, md.day, &feast.id)
        };
        return Some(Date::new(year, month as i32, day as i32));
    }
    let rule = feast.date_rule.as_deref()?;
    if rule == "holy-name" {
        let jan2 = Date::new(year, 1, 2);
        return Some((0..=3).map(|i| jan2.add_days(i)).find(|d| d.weekday() == Weekday::Sunday).unwrap_or(jan2));
    }
    if rule == "last-sunday-october" {
        let oct31 = Date::new(year, 10, 31);
        return Some(oct31.add_days(-oct31.weekday().number()));
    }
    // ^easter([+-]\d+)$
    if let Some(rest) = rule.strip_prefix("easter")
        && let Some(digits) = rest.strip_prefix(['+', '-'])
        && !digits.is_empty()
        && digits.bytes().all(|b| b.is_ascii_digit())
    {
        return Some(m.easter.add_days(atoi_or_zero(rest)));
    }
    if rule.starts_with("epiphany-sunday-") {
        let idx = atoi_or_zero(rule.rsplit('-').next().unwrap_or(""));
        return Some(first_sunday_after_epiphany(year).add_days((idx - 1) * 7));
    }
    if rule.starts_with("advent-sunday-") {
        let idx = atoi_or_zero(rule.rsplit('-').next().unwrap_or(""));
        match idx {
            1 => return Some(m.advent1),
            2 => return Some(m.advent2),
            3 => return Some(m.advent3),
            4 => return Some(m.advent4),
            _ => {}
        }
    }
    if let Some(suffix) = rule.strip_prefix("pentecost-sunday-") {
        return Some(m.pentecost.add_days(7 * atoi_or_zero(suffix)));
    }
    None
}

/// The temporal Sunday office governing the week that begins on this Sunday.
/// Resumed Sundays report their ProperID.
fn temporal_week_id(candidates: &[FeastRef]) -> Option<String> {
    let mut fallback = None;
    for f in candidates {
        if f.is_category(Category::Sunday) {
            return Some(f.proper_id.clone().unwrap_or_else(|| f.id.clone()));
        }
        if matches!(f.id.as_str(), "easter-sunday" | "low-sunday" | "pentecost") {
            fallback = Some(f.id.clone());
        }
    }
    fallback
}

/// The ordo's name for an unnamed weekday of a penitential season:
/// "Wednesday after Lent III", "Thursday after Septuagesima", "Tuesday after
/// Advent I". In Septuagesimatide and Advent a Saturday is named for the
/// Sunday it precedes ("Saturday before Sexagesima", "Saturday before Advent
/// II"); Lent keeps "Saturday after Lent IV" (2026 ordo).
fn seasonal_feria_name(date: Date, m: &MoveableDates, season: Season) -> Option<String> {
    let weekday = date.weekday();
    if weekday == Weekday::Sunday {
        return None;
    }
    let to_easter = m.easter.days_since(date);
    let (after, before) = match season {
        Season::Lent => match to_easter {
            43..=45 => ("Ash Wednesday".to_string(), None),
            36..=41 => ("Lent I".to_string(), None),
            29..=34 => ("Lent II".to_string(), None),
            22..=27 => ("Lent III".to_string(), None),
            15..=20 => ("Lent IV".to_string(), None),
            _ => return None,
        },
        Season::Passiontide => match to_easter {
            8..=13 => ("Passion Sunday".to_string(), None),
            1..=6 => ("Palm Sunday".to_string(), None),
            _ => return None,
        },
        Season::Septuagesima => match to_easter {
            57..=62 => ("Septuagesima".to_string(), Some("Sexagesima".to_string())),
            50..=55 => ("Sexagesima".to_string(), Some("Quinquagesima".to_string())),
            47..=48 => ("Quinquagesima".to_string(), None),
            _ => return None,
        },
        Season::Advent => {
            let week = date.days_since(m.advent1) / 7 + 1;
            (format!("Advent {}", roman(week)), (week < 4).then(|| format!("Advent {}", roman(week + 1))))
        }
        Season::Christmas | Season::Epiphany | Season::Easter | Season::Pentecost => return None,
    };
    Some(match before {
        Some(next) if weekday == Weekday::Saturday => format!("Saturday before {next}"),
        _ => format!("{} after {after}", weekday.name()),
    })
}

/// The F2 feria that takes the office on weekdays in Lent and Passiontide.
fn privileged_lenten_feria(date: Date, m: &MoveableDates, season: Season, week_id: Option<&str>) -> Option<Feast> {
    if date.weekday() == Weekday::Sunday || !matches!(season, Season::Lent | Season::Passiontide) {
        return None;
    }
    let mut f = Feast::synthetic(
        "privileged-lenten-feria",
        feria_commemoration_name(date, m, season),
        Rank::PrivilegedFeria,
        season.color(),
        Category::Feria,
    );
    f.proper_id = week_id.and_then(non_empty);
    Some(f)
}

/// "Friday after Lent III" or "Thursday after Septuagesima", else "the Feria".
fn feria_commemoration_name(date: Date, m: &MoveableDates, season: Season) -> String {
    seasonal_feria_name(date, m, season).unwrap_or_else(|| "the Feria".to_string())
}

/// The commemoration of the occurring seasonal feria displaced by a feast in
/// Advent or on a penitential weekday (XIV.2,9).
fn feria_commemoration(day: &CalendarDay, m: &MoveableDates) -> Option<Feast> {
    if !is_penitential_feria_season(day.season) || day.date.weekday() == Weekday::Sunday {
        return None;
    }
    let celebration = day.celebration.as_ref()?; // office is already of the feria
    if celebration.is_moveable() || celebration.is_category(Category::Feria) || celebration.is_category(Category::Sunday) {
        return None;
    }
    if day.commemorations.iter().any(|c| c.is_category(Category::Feria) && !c.is_vigil) {
        return None; // e.g. a demoted Ember day
    }
    Some(seasonal_feria_commemoration(day, m))
}

/// The displaced seasonal feria as a commemoration. The office crate also
/// uses it for a free feria at the next feast's I Vespers.
pub fn seasonal_feria_commemoration(day: &CalendarDay, m: &MoveableDates) -> Feast {
    let mut f = Feast::synthetic(
        FERIA_COMMEMORATION_ID,
        feria_commemoration_name(day.date, m, day.season),
        Rank::Commemoration,
        day.season.color(),
        Category::Feria,
    );
    f.proper_id = day.temporal_week_id.clone();
    f
}

fn saturday_office_bvm_allowed(season: Season) -> bool {
    match season {
        Season::Christmas | Season::Epiphany | Season::Easter | Season::Pentecost => true,
        Season::Advent | Season::Septuagesima | Season::Lent | Season::Passiontide => false,
    }
}

fn saturday_office_bvm_feast(date: Date, season: Season) -> Feast {
    let mut f =
        Feast::synthetic("saturday-office-bvm", "Saturday Office of the B.V.M.", Rank::Simple, Color::White, Category::BlessedVirgin);
    f.fixed = fixed(date);
    // Until the Purification the office keeps the Christmastide antiphons.
    if season == Season::Christmas || (season == Season::Epiphany && (date.month() == 1 || (date.month() == 2 && date.day() < 2))) {
        f.proper_id = Some("saturday-office-bvm-christmastide".to_string());
    }
    f
}

/// A civil year of resolved days plus the following Jan 1, which Vespers
/// concurrence needs to resolve Dec 31's evening.
#[derive(Clone, Debug)]
pub struct YearCalendar {
    pub year: i32,
    pub days: Vec<CalendarDay>,
    pub following_jan1: CalendarDay,
}

/// Builds the calendar for a year. Adjacent years are computed as padding so
/// transfers can cross Jan 1 and Dec 31 can see the following day.
pub fn build_calendar(year: i32, data: &CalendarData) -> Result<YearCalendar, String> {
    let (_, incoming) = build_calendar_year(year - 1, &data.feasts, &data.penitential_rules, &[])
        .map_err(|e| format!("building previous-year padding: {e}"))?;
    let (days, outgoing) = build_calendar_year(year, &data.feasts, &data.penitential_rules, &incoming)?;
    let (next, _) = build_calendar_year(year + 1, &data.feasts, &data.penitential_rules, &outgoing)
        .map_err(|e| format!("building following-year padding: {e}"))?;
    let following_jan1 = next.into_iter().next().ok_or_else(|| format!("following-year padding for {} is empty", year + 1))?;
    Ok(YearCalendar { year, days, following_jan1 })
}

fn push_candidate(candidates: &mut HashMap<Date, Vec<FeastRef>>, feast: &FeastRef, year: i32, m: &MoveableDates) {
    if let Some(d) = resolve_feast_date(feast, year, m) {
        candidates.entry(d).or_default().push(feast.clone());
    }
}

/// A feast awaiting transfer, with the date it was impeded on its own day.
type Transferred = (Date, FeastRef);

/// Resolves occurrence for one civil year, taking and returning the transfer
/// queue at its boundaries.
fn build_calendar_year(
    year: i32,
    feasts: &[FeastRef],
    rules: &[PenitentialRule],
    incoming: &[Transferred],
) -> Result<(Vec<CalendarDay>, Vec<Transferred>), String> {
    let m = MoveableDates::compute(year);

    let mut computed = epiphany_sunday_feasts(year, m.septuagesima);
    let epiphany_sundays = computed.len();
    computed.extend(anticipated_epiphany_sunday_feast(&m));
    computed.extend(anticipated_pentecost_sunday_feast(&m, epiphany_sundays));
    computed.extend(advent_sunday_feasts());
    computed.extend(eastertide_sunday_feasts());
    computed.extend(pentecost_sunday_feasts(m.easter, m.advent1));
    computed.push(nativity_octave_sunday_feast(year));

    let all_base: Vec<FeastRef> = feasts.iter().cloned().chain(computed.into_iter().map(Arc::new)).collect();
    let mut candidates: HashMap<Date, Vec<FeastRef>> = HashMap::new();
    for f in &all_base {
        push_candidate(&mut candidates, f, year, &m);
    }

    let synthetics: Vec<FeastRef> = vigil_feasts(&all_base, year, &m)
        .into_iter()
        .chain(octave_feasts(&all_base, year, &m))
        .chain(remaining_ember_feasts(&m, year))
        .map(Arc::new)
        .collect();
    for f in &synthetics {
        push_candidate(&mut candidates, f, year, &m);
    }

    let mut anchor_dates: HashMap<String, Date> = HashMap::new();
    for f in all_base.iter().chain(&synthetics) {
        if let Some(d) = resolve_feast_date(f, year, &m) {
            anchor_dates.insert(f.id.clone(), d);
        }
    }

    // Days 1–8 of each octave map to the parent; a later octave overwrites.
    let mut octave_ranges: HashMap<Date, String> = HashMap::new();
    for f in all_base.iter().filter(|f| f.has_octave) {
        if let Some(parent) = resolve_feast_date(f, year, &m) {
            for n in 0..8 {
                let date = parent.add_days(n);
                if octave_ceases(f, date, &m) {
                    continue;
                }
                octave_ranges.insert(date, f.id.clone());
            }
        }
    }

    let end = Date::new(year, 12, 31);
    let mut days = Vec::with_capacity(366);
    let mut pending: Vec<Transferred> = incoming.to_vec();
    let mut week_id: Option<String> = None;
    // A Sunday office anticipated on Saturday still governs the week that follows.
    let mut anticipated_week: Option<String> = None;
    let mut current = Date::new(year, 1, 1);
    while current <= end {
        let season = determine_season(current, &m);
        let mut day_candidates = candidates.get(&current).cloned().unwrap_or_default();
        if current.weekday() == Weekday::Sunday {
            week_id = temporal_week_id(&day_candidates).or(anticipated_week.take());
        }
        if current.weekday() == Weekday::Saturday {
            anticipated_week = day_candidates
                .iter()
                .find(|f| f.is_category(Category::Sunday))
                .map(|f| f.proper_id.clone().unwrap_or_else(|| f.id.clone()));
        }
        if let Some(feria) = privileged_lenten_feria(current, &m, season, week_id.as_deref())
            && !day_candidates.iter().any(|f| f.is_category(Category::Feria))
        {
            day_candidates.push(Arc::new(feria));
        }
        // XI.8: equal transferred feasts are kept in the order of their own days.
        let queue = std::mem::take(&mut pending);
        let transferred_in: Vec<FeastRef> = queue.iter().map(|(_, f)| f.clone()).collect();
        let (mut day, out) = resolve_day(current, &day_candidates, season, season.color(), &transferred_in);
        for f in out {
            let own_day = queue.iter().find(|(_, q)| Arc::ptr_eq(q, &f)).map_or(current, |(d, _)| *d);
            pending.push((own_day, f));
        }
        pending.sort_by_key(|(d, _)| *d);
        day.temporal_week_id = week_id.clone();
        if day.celebration.is_none() {
            day.tempora = seasonal_feria_name(current, &m, season);
        }
        if current.weekday() == Weekday::Saturday && day.celebration.is_none() && saturday_office_bvm_allowed(season) {
            let bvm = saturday_office_bvm_feast(current, season);
            day.color = bvm.color;
            day.celebration = Some(Arc::new(bvm));
        }
        day.within_octave_of = octave_ranges.get(&current).cloned();
        day.feria_commemoration = feria_commemoration(&day, &m).map(Arc::new);
        days.push(day);
        current = current.add_days(1);
    }

    apply_penitential_rules(&mut days, rules, &anchor_dates).map_err(|e| format!("applying penitential rules: {e}"))?;
    Ok((days, pending))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn pentecost_sundays_resume_epiphany() {
        // 2024: Easter May 5 → few Sundays after Pentecost; 2026 has more.
        for year in [2024, 2026, 2029] {
            let m = MoveableDates::compute(year);
            let feasts = pentecost_sunday_feasts(m.easter, m.advent1);
            let last = feasts.last().unwrap();
            assert_eq!(resolve_feast_date(last, year, &m), Some(m.advent1.add_days(-7)), "{year}");
            assert!(last.id == "pentecost-sunday-24" || last.proper_id.as_deref() == Some("pentecost-sunday-24"));
        }
    }
}
