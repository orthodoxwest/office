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

fn short_name(feast: &Feast) -> String {
    short_name_for(&feast.id).map_or_else(|| feast.name.clone(), str::to_string)
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
            let (id, name) = if is_octave_day {
                let name = match feast.id.as_str() {
                    "conception-bvm" => "Octave of the Conception of the B.V.M.".to_string(),
                    "nativity-john-baptist" => "Octave of St John Baptist".to_string(),
                    "ss-peter-paul" => "The Octave of Ss Peter & Paul".to_string(),
                    _ => format!("Octave Day of {short}"),
                };
                (format!("{}-octave-day", feast.id), name)
            } else {
                let name = match feast.id.as_str() {
                    _ if is_christmas => format!("Day {n} within the Nativity Octave"),
                    "conception-bvm" => format!("Day {n} within Conception Octave"),
                    "nativity-john-baptist" => format!("Day {n} within the Octave of St John Baptist"),
                    "ss-peter-paul" => format!("Day {n} within the Octave of Ss Peter & Paul"),
                    _ => format!("Day {n} within the Octave of {short}"),
                };
                (format!("{}-octave-day-{day_num}", feast.id), name)
            };
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
            let label = format!("{season}-ember-{}", date.weekday().name().to_lowercase());
            let mut f = Feast::synthetic(
                label.clone(),
                title_case(&label.replace('-', " ")),
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

/// "Wednesday after Lent III" for unnamed Lenten and Passiontide weekdays.
fn lenten_feria_name(date: Date, easter: Date, season: Season) -> Option<String> {
    if date.weekday() == Weekday::Sunday {
        return None;
    }
    let weekday = date.weekday().name();
    let to_easter = easter.days_since(date);
    let after = match season {
        Season::Lent => match to_easter {
            43..=45 => "Ash Wednesday",
            36..=41 => "Lent I",
            29..=34 => "Lent II",
            22..=27 => "Lent III",
            15..=20 => "Lent IV",
            _ => return None,
        },
        Season::Passiontide => match to_easter {
            8..=13 => "Passion Sunday",
            _ => return None,
        },
        Season::Advent | Season::Christmas | Season::Epiphany | Season::Septuagesima | Season::Easter | Season::Pentecost => return None,
    };
    Some(format!("{weekday} after {after}"))
}

/// The F2 feria that takes the office on weekdays in Lent and Passiontide.
fn privileged_lenten_feria(date: Date, easter: Date, season: Season, week_id: Option<&str>) -> Option<Feast> {
    if date.weekday() == Weekday::Sunday || !matches!(season, Season::Lent | Season::Passiontide) {
        return None;
    }
    let mut f = Feast::synthetic(
        "privileged-lenten-feria",
        feria_commemoration_name(date, easter, season, week_id),
        Rank::PrivilegedFeria,
        season.color(),
        Category::Feria,
    );
    f.proper_id = week_id.and_then(non_empty);
    Some(f)
}

/// "Friday after Lent III" or "Thursday after Septuagesima", else "the Feria".
fn feria_commemoration_name(date: Date, easter: Date, season: Season, week_id: Option<&str>) -> String {
    if let Some(name) = lenten_feria_name(date, easter, season) {
        return name;
    }
    match week_id {
        Some(w) if !w.is_empty() => format!("{} after {}", date.weekday().name(), title_case(&w.replace('-', " "))),
        _ => "the Feria".to_string(),
    }
}

/// The commemoration of the occurring seasonal feria displaced by a feast in
/// Advent or on a penitential weekday (XIV.2,9).
fn feria_commemoration(day: &CalendarDay, easter: Date) -> Option<Feast> {
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
    Some(seasonal_feria_commemoration(day, easter))
}

/// The displaced seasonal feria as a commemoration. The office crate also
/// uses it for a free feria at the next feast's I Vespers.
pub fn seasonal_feria_commemoration(day: &CalendarDay, easter: Date) -> Feast {
    let mut f = Feast::synthetic(
        FERIA_COMMEMORATION_ID,
        feria_commemoration_name(day.date, easter, day.season, day.temporal_week_id.as_deref()),
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

/// Resolves occurrence for one civil year, taking and returning the transfer
/// queue at its boundaries.
fn build_calendar_year(
    year: i32,
    feasts: &[FeastRef],
    rules: &[PenitentialRule],
    incoming: &[FeastRef],
) -> Result<(Vec<CalendarDay>, Vec<FeastRef>), String> {
    let m = MoveableDates::compute(year);

    let mut computed = epiphany_sunday_feasts(year, m.septuagesima);
    computed.extend(anticipated_epiphany_sunday_feast(&m));
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
                octave_ranges.insert(parent.add_days(n), f.id.clone());
            }
        }
    }

    let end = Date::new(year, 12, 31);
    let mut days = Vec::with_capacity(366);
    let mut pending: Vec<FeastRef> = incoming.to_vec();
    let mut week_id: Option<String> = None;
    let mut current = Date::new(year, 1, 1);
    while current <= end {
        let season = determine_season(current, &m);
        let mut day_candidates = candidates.get(&current).cloned().unwrap_or_default();
        if current.weekday() == Weekday::Sunday {
            week_id = temporal_week_id(&day_candidates);
        }
        if let Some(feria) = privileged_lenten_feria(current, m.easter, season, week_id.as_deref())
            && !day_candidates.iter().any(|f| f.is_category(Category::Feria))
        {
            day_candidates.push(Arc::new(feria));
        }
        let transferred_in = std::mem::take(&mut pending);
        let (mut day, out) = resolve_day(current, &day_candidates, season, season.color(), &transferred_in);
        pending.extend(out);
        day.temporal_week_id = week_id.clone();
        if day.celebration.is_none() {
            day.tempora = lenten_feria_name(current, m.easter, season);
        }
        if current.weekday() == Weekday::Saturday && day.celebration.is_none() && saturday_office_bvm_allowed(season) {
            let bvm = saturday_office_bvm_feast(current, season);
            day.color = bvm.color;
            day.celebration = Some(Arc::new(bvm));
        }
        day.within_octave_of = octave_ranges.get(&current).cloned();
        day.feria_commemoration = feria_commemoration(&day, m.easter).map(Arc::new);
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
    fn title_case_words() {
        assert_eq!(title_case("advent ember wednesday"), "Advent Ember Wednesday");
        assert_eq!(octave_display_name("christmas"), "Christmas");
        assert_eq!(octave_display_name("st-lawrence"), "St Lawrence");
    }

    #[test]
    fn nativity_octave_sunday() {
        // 2022: Christmas on Sunday → Dec 29. 2027: Sunday Dec 26 → Dec 29.
        // 2025: Sunday Dec 28 → Dec 29. 2024: Sunday Dec 29.
        for (year, day) in [(2022, 29), (2027, 29), (2025, 29), (2024, 29), (2023, 31)] {
            assert_eq!(nativity_octave_sunday_feast(year).fixed, Some(MonthDay { month: 12, day }), "{year}");
        }
    }

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

    #[test]
    fn leap_shift() {
        assert_eq!(adjust_fixed_date_for_leap_year(2024, 2, 24, "st-matthias"), (2, 25));
        assert_eq!(adjust_fixed_date_for_leap_year(2025, 2, 24, "st-matthias"), (2, 24));
        assert_eq!(adjust_fixed_date_for_leap_year(2024, 2, 24, "vigil-of-x"), (2, 24));
    }

    #[test]
    fn date_rules() {
        let m = MoveableDates::compute(2026);
        let rule = |r: &str| {
            let mut f = Feast::synthetic("x", "x", Rank::Double, Color::White, Category::Lord);
            f.date_rule = Some(r.to_string());
            resolve_feast_date(&f, 2026, &m)
        };
        assert_eq!(rule("easter+1"), Some(m.easter_monday));
        assert_eq!(rule("easter-63"), Some(m.septuagesima));
        assert_eq!(rule("holy-name"), Some(Date::new(2026, 1, 4)));
        assert_eq!(rule("last-sunday-october"), Some(Date::new(2026, 10, 25)));
        assert_eq!(rule("advent-sunday-3"), Some(m.advent3));
        assert_eq!(rule("advent-sunday-5"), None);
        assert_eq!(rule("epiphany-sunday-2"), Some(Date::new(2026, 1, 18)));
        assert_eq!(rule("pentecost-sunday-1"), Some(m.trinity_sunday));
        assert_eq!(rule("nonsense"), None);
        assert_eq!(rule("easter"), None);
    }
}

#[cfg(test)]
mod octave_tests {
    use super::*;
    use crate::model::OctaveClass;

    #[test]
    fn privileged_octaves_exclude_explicit_feasts_and_keep_easter_offsets() {
        for (id, offset, days) in [("easter-sunday", 0, vec![4, 5, 6, 7]), ("pentecost", 49, vec![2, 3, 4, 5, 6, 7])] {
            let mut f = Feast::synthetic(id, id, Rank::Double1stClass, Color::White, Category::Lord);
            f.has_octave = true;
            f.octave_class = OctaveClass::PrivilegedFirst;
            f.date_rule = Some(format!("easter+{offset}"));
            let generated = octave_feasts(&[Arc::new(f)], 2026, &MoveableDates::compute(2026));
            assert_eq!(generated.len(), days.len());
            for (f, n) in generated.iter().zip(days) {
                assert_eq!(f.id, format!("{id}-octave-day-{n}"));
                assert_eq!(f.date_rule, Some(format!("easter+{}", offset + n - 1)));
                assert_eq!(f.rank, Rank::Double1stClass);
                assert!(f.is_privileged_octave_day);
            }
        }
    }

    #[test]
    fn octave_antiphon_sets_skip_sunday_and_terminal_day() {
        let mut f = Feast::synthetic("example", "Example", Rank::Double1stClass, Color::White, Category::Apostle);
        f.fixed = Some(MonthDay { month: 6, day: 11 });
        f.has_octave = true;
        f.octave_class = OctaveClass::PrivilegedThird;
        let generated = octave_feasts(&[Arc::new(f)], 2026, &MoveableDates::compute(2026));
        let expected = [
            "example-octave-set-1",
            "example-octave-set-2",
            "example",
            "example-octave-set-3",
            "example-octave-set-4",
            "example-octave-set-5",
            "example",
        ];
        for (f, want) in generated.iter().zip(expected) {
            assert_eq!(f.proper_id.as_deref(), Some(want));
        }
        assert_eq!(generated.last().unwrap().rank, Rank::GreaterDouble);
        assert!(!generated.last().unwrap().is_privileged_octave_day);
    }
}
