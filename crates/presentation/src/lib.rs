//! How the Office is named on every front: the web and the native apps set it in their own
//! type, but read the same words from here. A day's name and heading, a season's, a date's; the
//! office being prayed at a clock hour and home's invitation to it; the prefilled "Report a
//! problem" issue; and the usage beacon's vocabulary ([`usage`]). Pure functions of the calendar
//! and the composed hour: no templates, no platform, and nothing that composes an hour.

pub mod usage;

use calendar::{CalendarDay, Date, Season};
use liturgy::OfficeHour;

/// Capitalizes the first letter ("advent" → "Advent").
pub fn title_case(s: &str) -> String {
    let mut chars = s.chars();
    match chars.next() {
        Some(c) if c.is_ascii() => format!("{}{}", c.to_ascii_uppercase(), chars.as_str()),
        _ => s.to_string(),
    }
}

/// The ornament body class for a season: Passiontide veils the gold,
/// Paschaltide warms it, and every other season keeps the ordinary gold.
pub fn season_class(season: Option<Season>) -> &'static str {
    match season {
        Some(Season::Passiontide) => "season-passiontide",
        Some(Season::Easter) => "season-eastertide",
        Some(Season::Advent | Season::Christmas | Season::Epiphany | Season::Septuagesima | Season::Lent | Season::Pentecost) | None => "",
    }
}

/// The season as an hour header names it, or nothing. A bare "Easter",
/// "Christmas", "Epiphany" or "Pentecost" beside a date reads as that feast
/// day, so the tides take their season names. The season after Pentecost is
/// left unnamed: "Time after Pentecost" is clumsy in a header line, and its
/// Sundays and feasts already name themselves.
pub fn season_label(season: &str) -> String {
    use calendar::Season::*;
    match Season::parse(season) {
        Ok(Christmas) => "Christmastide".into(),
        Ok(Epiphany) => "Epiphanytide".into(),
        Ok(Easter) => "Eastertide".into(),
        Ok(Pentecost) => String::new(),
        Ok(Advent | Septuagesima | Lent | Passiontide) | Err(_) => title_case(season),
    }
}

/// The day's display name, as home and the ordo name it: its celebration,
/// else its temporal title, else the season's feria.
pub fn day_name(day: &CalendarDay) -> String {
    if let Some(c) = &day.celebration {
        return c.name.clone();
    }
    if let Some(t) = &day.tempora {
        return t.clone();
    }
    format!("{} feria", title_case(day.season.as_str()))
}

/// Home's heading for a day.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct DayHeading {
    /// The day's display name ([`day_name`]).
    pub feast: String,
    /// "Octave of …" within an octave the name does not already mention; the
    /// ordo carries the full wording.
    pub octave_note: String,
    /// The season's name, empty when the celebration already names it.
    pub season: String,
}

pub fn day_heading(day: &CalendarDay) -> DayHeading {
    let feast = day_name(day);
    let lower = feast.to_lowercase();
    let octave_note = match &day.within_octave_of {
        Some(id) if !id.is_empty() && !lower.contains("octave") => format!("Octave of {}", calendar::builder::octave_display_name(id)),
        _ => String::new(),
    };
    let mut season = title_case(day.season.as_str());
    if !season.is_empty() && lower.contains(&season.to_lowercase()) {
        season.clear();
    }
    DayHeading { feast, octave_note, season }
}

pub const MONTHS: [&str; 12] =
    ["January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"];

pub fn month_name(d: Date) -> &'static str {
    MONTHS[d.month() as usize - 1]
}

/// At least four year digits, with the sign outside them.
pub fn format_year(y: i32) -> String {
    if y < 0 { format!("-{:04}", -i64::from(y)) } else { format!("{y:04}") }
}

/// `Format("2006-01-02")`.
pub fn date_slug(d: Date) -> String {
    format!("{}-{:02}-{:02}", format_year(d.year()), d.month(), d.day())
}

/// `Format("Monday, January 2, 2006")`.
pub fn long_date(d: Date) -> String {
    format!("{}, {} {}, {}", d.weekday().name(), month_name(d), d.day(), format_year(d.year()))
}

/// From each clock hour, the office being prayed (slug and name) and its day offset: Compline
/// after midnight is yesterday's. The web's app.js mirrors it; office-web tests the two agree.
pub const CURRENT_HOUR_SCHEDULE: [(i8, &str, &str, i32); 8] = [
    (0, "compline", "Compline", -1),
    (2, "lauds", "Lauds", 0),
    (7, "prime", "Prime", 0),
    (9, "terce", "Terce", 0),
    (11, "sext", "Sext", 0),
    (13, "none", "None", 0),
    (17, "vespers", "Vespers", 0),
    (20, "compline", "Compline", 0),
];

/// The office being prayed at a clock hour (0–23): slug, name, and day offset.
pub fn current_hour_entry(hour: i8) -> (&'static str, &'static str, i32) {
    CURRENT_HOUR_SCHEDULE.iter().rev().find(|b| hour >= b.0).map(|b| (b.1, b.2, b.3)).unwrap_or(("compline", "Compline", -1))
}

/// Home's invitation to pray.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Invitation {
    /// The hour it opens.
    pub hour: &'static str,
    /// "Pray Vespers" today; "Open Lauds" on another day.
    pub label: String,
    /// The day of the hour it opens: yesterday for Compline after midnight.
    pub date: Date,
    /// The hour home's directory marks as now; empty when the invitation is
    /// not to this day's office.
    pub current: &'static str,
}

/// The invitation on home for `shown`, given the reader's own day and clock hour.
pub fn invitation(shown: Date, now: Date, now_hour: i8) -> Invitation {
    if shown != now {
        return Invitation { hour: "lauds", label: "Open Lauds".to_string(), date: shown, current: "" };
    }
    let (hour, name, offset) = current_hour_entry(now_hour);
    Invitation { hour, label: format!("Pray {name}"), date: now.add_days(offset), current: if offset == 0 { hour } else { "" } }
}

/// The hour home's leaf points at, given the reader's own day and clock hour: the hour being
/// prayed and `true` ("pray now") when the leaf is that office's day — yesterday's for Compline
/// after midnight — or Lauds and `false` ("begin here") on any other day. `None` on today's leaf
/// while yesterday's Compline is still the hour: no hour on the leaf is the one being prayed, and
/// [`invitation`] leads off it. app.js mirrors it.
pub fn leaf_pointer(shown: Date, now: Date, now_hour: i8) -> Option<(&'static str, bool)> {
    let (hour, _, offset) = current_hour_entry(now_hour);
    if shown == now.add_days(offset) {
        Some((hour, true))
    } else if shown == now {
        None
    } else {
        Some(("lauds", false))
    }
}

/// Each hour's time in the old reckoning, as home's horarium sets it beside the hour's name.
pub const HOUR_TIMES: [(&str, &str); 7] = [
    ("lauds", "at daybreak"),
    ("prime", "the first hour"),
    ("terce", "the third hour"),
    ("sext", "the sixth hour"),
    ("none", "the ninth hour"),
    ("vespers", "at evening"),
    ("compline", "at nightfall"),
];

/// An hour's time in the old reckoning ([`HOUR_TIMES`]), or nothing for an unknown slug.
pub fn hour_time(slug: &str) -> &'static str {
    HOUR_TIMES.iter().find(|(s, _)| *s == slug).map_or("", |(_, t)| t)
}

/// The light of the hour being prayed, which tints home's wall by day: dawn at Lauds and Prime,
/// day through the little hours, dusk at Vespers, night at Compline (after midnight too). Only
/// the ground moves; no ink changes with it. app.js mirrors it.
pub fn time_of_day(hour: &str) -> &'static str {
    match hour {
        "lauds" | "prime" => "dawn",
        "terce" | "sext" | "none" => "day",
        "vespers" => "dusk",
        _ => "night",
    }
}

/// How home's leaf names a day's rank, and the grade of its title's initial: 0 a feria (no
/// initial), 1 a simple or semidouble and 2 a double or greater double (a red initial in the
/// line), 3 the second class (a gilt one), 4 the first class (a gilt woodcut initial in a square
/// two lines tall, with a penwork border down the leaf).
pub fn leaf_rank(day: &CalendarDay) -> (&'static str, u8) {
    use calendar::Rank::*;
    let Some(c) = &day.celebration else { return ("Feria", 0) };
    let (label, grade) = match c.rank {
        Double1stClass => ("Double of the first class", 4),
        Double2ndClass => ("Double of the second class", 3),
        GreaterDouble => ("Greater double", 2),
        Double => ("Double", 2),
        SemiDouble => ("Semidouble", 1),
        Simple => ("Simple", 1),
        PrivilegedFeria => ("Greater feria", 0),
        Commemoration => ("Commemoration", 0),
    };
    match (day.is_ferial(), c.rank) {
        (true, PrivilegedFeria) => (label, 0),
        (true, _) => ("Feria", 0),
        (false, _) => (label, grade),
    }
}

/// The name opens with a Roman numeral ("III Sunday in Lent"), which a versal would split.
pub fn opens_with_numeral(name: &str) -> bool {
    name.split_whitespace().next().is_some_and(|w| w.len() < name.trim().len() && w.chars().all(|c| "IVXLC".contains(c)))
}

/// The day's collect as its Lauds says it, run together as one paragraph: the hour's first
/// collect that is not a commemoration's, without the people's Amen, which the page sets as red
/// work of its own. Empty when the hour has none.
pub fn day_collect(hour: &OfficeHour) -> String {
    let Some(collect) =
        hour.sections.iter().flat_map(|s| &s.elements).find(|e| e.kind == liturgy::ElementType::Collect && !e.is_commemoration)
    else {
        return String::new();
    };
    collect
        .text
        .lines()
        .map(str::trim)
        .filter(|l| !l.is_empty() && !l.starts_with("R.") && !l.starts_with('℟'))
        .collect::<Vec<_>>()
        .join(" ")
}

/// The reminder page's hours, as the web offers them and the apps start from: slug, name, the
/// suggested time (hour, minute), and whether it starts chosen.
pub const REMINDER_DEFAULTS: [(&str, &str, u32, u32, bool); 7] = [
    ("lauds", "Lauds", 6, 45, true),
    ("prime", "Prime", 7, 30, false),
    ("terce", "Terce", 9, 0, false),
    ("sext", "Sext", 12, 0, false),
    ("none", "None", 15, 0, false),
    ("vespers", "Vespers", 18, 0, true),
    ("compline", "Compline", 21, 0, true),
];

/// A reminder's headline: the office and the day it keeps ("Vespers — III Sunday in Lent"), as
/// the web's calendar feed names its events and the apps name their notifications.
pub fn reminder_summary(hour: &str, day: &CalendarDay) -> String {
    format!("{} — {}", title_case(hour), day_name(day))
}

/// A reminder's detail: the celebration's rank, the season, the colour, and any commemorations.
pub fn reminder_description(day: &CalendarDay) -> String {
    let mut parts = Vec::new();
    if let Some(c) = &day.celebration {
        parts.push(c.rank.display_name().to_string());
    }
    parts.push(title_case(day.season.as_str()));
    parts.push(day.color.as_str().to_string());
    parts.extend(day.commemorations.iter().map(|c| format!("Comm. {}", c.name)));
    parts.join(" · ")
}

/// The GitHub new-issue endpoint behind "Report a problem".
const REPO_ISSUES_URL: &str = "https://github.com/orthodoxwest/office/issues/new";

/// Escapes a query component; spaces become `+`.
pub fn query_escape(s: &str) -> String {
    let mut out = String::with_capacity(s.len());
    for &c in s.as_bytes() {
        match c {
            b' ' => out.push('+'),
            c if c.is_ascii_alphanumeric() || matches!(c, b'-' | b'_' | b'.' | b'~') => out.push(c as char),
            c => out.push_str(&format!("%{c:02X}")),
        }
    }
    out
}

/// `url.Values{title, body, labels=review}.Encode()`: keys sorted.
fn issue_url(title: &str, body: &str) -> String {
    format!("{REPO_ISSUES_URL}?body={}&labels=review&title={}", query_escape(body), query_escape(title))
}

pub fn season_str(hour: &OfficeHour) -> &'static str {
    hour.season.map(|s| s.as_str()).unwrap_or("")
}

fn celebration(hour: &OfficeHour) -> String {
    if hour.feast.is_empty() { format!("{} feria", title_case(season_str(hour))) } else { hour.feast.clone() }
}

/// A prefilled issue identifying the exact page under review.
pub fn report_url(hour: &OfficeHour, hour_name: &str, date_slug: &str) -> String {
    let celebration = celebration(hour);
    let title = format!("[review] {} — {date_slug} ({celebration})", hour.title);
    let body = format!(
        "**Page:** /{hour_name}/{date_slug}?form={form}
**Prayer form:** {label}
**Celebration:** {celebration}
**Season:** {season}

**Category** (check all that apply):
- [ ] Missing proper — the app shows a generic/ordinary text where the diurnal or archdiocese supplement has a specific one
- [ ] Incorrect translation — wording differs from our diocesan books
- [ ] Logic or rubric error — wrong structure, missing or extra element, wrong psalms/antiphons for the day

**What the books say** (cite diurnal/supplement page if possible):

**What the app shows:**

",
        form = hour.form.as_str(),
        label = hour.form.label(),
        season = title_case(season_str(hour)),
    );
    issue_url(&title, &body)
}

/// One row of the Tabula Temporaria: a figure, or a date and the day it leads to.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct TabulaRow {
    pub label: &'static str,
    pub value: String,
    pub date: Option<Date>,
}

/// The Tabula Temporaria as the printed ordo opens: the year's figures, its moveable feasts, and
/// its Ember days.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct TabulaRows {
    pub figures: Vec<TabulaRow>,
    pub moveable: Vec<TabulaRow>,
    pub ember: Vec<TabulaRow>,
}

pub fn tabula(year: i32) -> TabulaRows {
    let t = calendar::Tabula::compute(year);
    let moveable = calendar::MoveableDates::compute(year);
    let figure = |label, value| TabulaRow { label, value, date: None };
    let date = |label, d: Date| TabulaRow { label, value: format!("{} {}", month_name(d), d.day()), date: Some(d) };
    let ember = |label, e: &calendar::computus::EmberSet| TabulaRow {
        label,
        value: format!("{} {}, {}, {}", month_name(e.wed), e.wed.day(), e.fri.day(), e.sat.day()),
        date: Some(e.wed),
    };
    TabulaRows {
        figures: vec![
            figure("Golden Number", calendar::computus::roman(t.golden_number)),
            figure("Dominical Letter", t.dominical_letter.to_string()),
            figure("Sundays after Epiphany", t.sundays_after_epiphany.to_string()),
            figure("Sundays after Pentecost", t.sundays_after_pentecost.to_string()),
        ],
        moveable: vec![
            date("Septuagesima Sunday", moveable.septuagesima),
            date("Ash Wednesday", moveable.ash_wednesday),
            date("Easter Day", moveable.easter),
            date("Ascension Day", moveable.ascension),
            date("Pentecost", moveable.pentecost),
            date("Corpus Christi", moveable.corpus_christi),
            date("Advent Sunday", moveable.advent1),
        ],
        ember: vec![
            ember("Spring (Lent)", &t.spring),
            ember("Summer (Whitsun)", &t.summer),
            ember("Autumn (Holy Cross)", &t.autumn),
            ember("Winter (Advent)", &t.winter),
        ],
    }
}

/// "Anno Domini MMXXVI"'s numeral, or empty past the numerals' reach.
pub fn year_roman(year: i32) -> String {
    if (1..=3999).contains(&year) { calendar::computus::roman(year) } else { String::new() }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn query_escape_encodes_special_characters() {
        assert_eq!(query_escape("a b/c?d=é—*"), "a+b%2Fc%3Fd%3D%C3%A9%E2%80%94%2A");
    }

    #[test]
    fn season_label_names_tides_and_leaves_pentecost_unnamed() {
        assert_eq!(season_label("easter"), "Eastertide");
        assert_eq!(season_label("christmas"), "Christmastide");
        assert_eq!(season_label("epiphany"), "Epiphanytide");
        assert_eq!(season_label("pentecost"), "");
        assert_eq!(season_label("lent"), "Lent");
        assert_eq!(season_label("passiontide"), "Passiontide");
        assert_eq!(season_label("septuagesima"), "Septuagesima");
        assert_eq!(season_label("advent"), "Advent");
        assert_eq!(season_label(""), "");
    }

    #[test]
    fn season_class_veils_passiontide_and_brightens_paschaltide() {
        assert_eq!(season_class(Some(Season::Passiontide)), "season-passiontide");
        assert_eq!(season_class(Some(Season::Easter)), "season-eastertide");
        for s in [Season::Advent, Season::Christmas, Season::Epiphany, Season::Septuagesima, Season::Lent, Season::Pentecost] {
            assert_eq!(season_class(Some(s)), "", "{s:?}");
        }
        assert_eq!(season_class(None), "");
    }

    #[test]
    fn titles_and_dates() {
        assert_eq!(title_case("advent"), "Advent");
        assert_eq!(title_case(""), "");
        let d = Date::new(2026, 3, 15);
        assert_eq!(long_date(d), "Sunday, March 15, 2026");
        assert_eq!(date_slug(d), "2026-03-15");
        assert_eq!(date_slug(Date::new(812, 1, 2)), "0812-01-02");
        assert_eq!(date_slug(Date::new(-1, 1, 1)), "-0001-01-01");
        assert_eq!(date_slug(Date::new(10000, 1, 1)), "10000-01-01");
        assert_eq!(long_date(Date::new(2026, 3, 11)), "Wednesday, March 11, 2026");
        assert_eq!(month_name(d), "March");
    }

    #[test]
    fn current_hour_entry_at_every_boundary() {
        for (start, slug, label, offset) in CURRENT_HOUR_SCHEDULE {
            assert_eq!(current_hour_entry(start), (slug, label, offset), "{start:02}:00");
        }
        assert_eq!(current_hour_entry(1), ("compline", "Compline", -1));
        assert_eq!(current_hour_entry(16), ("none", "None", 0));
        assert_eq!(current_hour_entry(23), ("compline", "Compline", 0));
    }

    #[test]
    fn the_leaf_points_at_the_hour_being_prayed_on_its_own_day() {
        let today = Date::new(2026, 3, 15);
        assert_eq!(leaf_pointer(today, today, 10), Some(("terce", true)));
        assert_eq!(leaf_pointer(today, today, 21), Some(("compline", true)));
        // After midnight Compline is yesterday's: yesterday's leaf prays it, today's has none.
        assert_eq!(leaf_pointer(today.add_days(-1), today, 1), Some(("compline", true)));
        assert_eq!(leaf_pointer(today, today, 1), None);
        // Any other day begins at Lauds.
        assert_eq!(leaf_pointer(today.add_days(1), today, 10), Some(("lauds", false)));
        assert_eq!(leaf_pointer(today.add_days(-1), today, 10), Some(("lauds", false)));
    }

    #[test]
    fn a_numeral_keeps_its_title_whole() {
        assert!(opens_with_numeral("III Sunday in Lent"));
        assert!(opens_with_numeral("XVIII Sunday after Pentecost"));
        assert!(!opens_with_numeral("St Placidus & companions, Martyrs"));
        assert!(!opens_with_numeral("Lent feria"));
        assert!(!opens_with_numeral("Ivo"));
        assert!(!opens_with_numeral("V"));
    }

    #[test]
    fn every_hour_has_its_old_time_and_its_light() {
        for (slug, _, _, _, _) in REMINDER_DEFAULTS {
            assert!(!hour_time(slug).is_empty(), "{slug}");
        }
        assert_eq!(hour_time("vespers"), "at evening");
        assert_eq!(hour_time("matins"), "");
        let lights: Vec<_> = HOUR_TIMES.iter().map(|(slug, _)| time_of_day(slug)).collect();
        assert_eq!(lights, ["dawn", "dawn", "day", "day", "day", "dusk", "night"]);
    }

    #[test]
    fn the_invitation_is_to_the_hour_being_prayed_only_today() {
        let today = Date::new(2026, 3, 15);
        let evening = invitation(today, today, 18);
        assert_eq!((evening.hour, evening.label.as_str(), evening.date, evening.current), ("vespers", "Pray Vespers", today, "vespers"));
        let late = invitation(today, today, 1);
        assert_eq!((late.hour, late.date, late.current), ("compline", Date::new(2026, 3, 14), ""));
        let other = invitation(today.add_days(1), today, 18);
        assert_eq!((other.hour, other.label.as_str(), other.date, other.current), ("lauds", "Open Lauds", today.add_days(1), ""));
    }
}
