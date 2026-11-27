//! The usage beacon's words. The server parses them ([`parse_event`]), the web's app.js writes
//! them (it mirrors [`app_beacon`]'s grammar and [`current_day`]'s window), and the native apps
//! write them through [`app_beacon`], so every front is counted in one vocabulary.
//!
//! A beacon is one line: a page scope, then "family:value" tokens describing how the page was
//! read ("lauds appearance:apse screen:mobile prayer-form:private client:android").

use calendar::Date;

/// Where the native apps report until the server names another: the canonical host's usage
/// endpoint. Builds from before the move post to office.fly.dev, which keeps answering and tells
/// them the new address (see [`ENDPOINT_HEADER`]).
pub const ENDPOINT: &str = "https://orthodoxwestbreviary.com/api/usage";

/// The response header naming where the apps should report from now on: the canonical host's
/// endpoint, sent once the server has one (`OFFICE_CANONICAL_HOST`). An app stores it and
/// posts there next time, so builds already installed follow the site to its new address.
pub const ENDPOINT_HEADER: &str = "office-usage-endpoint";

/// The request header on which a native app's beacon carries its build number: the count of
/// master's commits it was built from (`git rev-list --count HEAD`), which Android's version code
/// and iOS's `CFBundleVersion` both are. So one number orders every build of either app, and a
/// correction merged at master's Nth commit is in every build numbered N or more. The server
/// counts a reader once a day under `build:<client>:<n>` ([`build_scope`]); the beacon's words
/// stay a closed vocabulary. Builds from before it was sent say nothing, and are older than any
/// build that does.
pub const BUILD_HEADER: &str = "x-office-build";

/// The scope a native app's beacon is counted under for its build: `build:android:2009`. None for
/// a browser, which is always the site's current build, or a header that is not a plain build
/// number.
pub fn build_scope(dimensions: &[String], header: &str) -> Option<String> {
    let client = dimensions.iter().find_map(|d| d.strip_prefix("client:")).filter(|c| matches!(*c, "android" | "ios"))?;
    let build = parse_build(header)?;
    Some(format!("build:{client}:{build}"))
}

/// A build number as the header says it: one to six digits, no leading zero.
pub fn parse_build(text: &str) -> Option<u32> {
    let ok = (1..=6).contains(&text.len()) && text.bytes().all(|b| b.is_ascii_digit()) && !text.starts_with('0');
    ok.then(|| text.parse().ok()).flatten()
}

/// A stored `build:<client>:<n>` scope's client and build.
pub fn build_of(scope: &str) -> Option<(&str, u32)> {
    let (client, build) = scope.strip_prefix("build:")?.split_once(':')?;
    Some((client, parse_build(build)?))
}

/// The longest beacon body the server reads. A longer one is refused whole, so the limit keeps
/// well clear of the longest beacon any front writes (`beacons_fit_the_limit`).
pub const MAX_BEACON: usize = 288;

/// The seven hours, as the usage store names its office scopes.
pub const HOURS: [&str; 7] = liturgy::HOUR_NAMES;

/// Single-purpose scopes counted beside the hours: the ordo (calendar) page
/// and turning reminders on (the web's generated feed link).
pub const EXTRA_SCOPES: [&str; 2] = ["ordo", "reminders"];

/// The prayer forms an office page reports.
pub const PRAYER_FORMS: &[&str] = &liturgy::PrayerForm::NAMES;

/// A dimension family and its values.
pub struct Dimension<'a> {
    pub key: &'a str,
    pub values: &'a [&'a str],
}

/// Families of mutually exclusive values describing how a page was
/// rendered. Stored as "<key>:<value>"; a written key or value is never
/// redefined.
pub const DIMENSIONS: [Dimension<'static>; 20] = [
    Dimension { key: "appearance", values: &["nave", "apse"] },
    Dimension { key: "screen", values: &["desktop", "mobile"] },
    Dimension { key: "prayer-form", values: PRAYER_FORMS },
    // A browser tab, the installed web app, or a native app.
    Dimension { key: "client", values: CLIENTS },
    // Prime only, on a day with a Martyrology reading: whether the reader's
    // setting showed it or left the rubric in its place.
    Dimension { key: "martyrology", values: MARTYROLOGY },
    // An hour with a scored hymn or Marian antiphon: whether the reader's Chant setting showed
    // the notation or the words alone.
    Dimension { key: "chant", values: CHANT },
    // Whether this browser or installation is counted for the first time today. The device
    // remembers the day it was first counted; nothing sent links one day to another.
    Dimension { key: "visit", values: VISITS },
    // The client of a first-day visit, so new readers can be told apart by client. The server
    // derives it from a beacon's `visit:first` and `client` ([`new_client`]); no client sends it.
    Dimension { key: "new-client", values: CLIENTS },
    // Whether this is the first reporting day of the calendar week (Monday to Sunday), and of the
    // calendar month, on which this browser or installation is counted. The device remembers the
    // last two days it was counted and sends only the word, so a week's `week:first` counts sum to
    // the readers of that week with nothing sent that links one day to another ([`periods`]).
    Dimension { key: "week", values: PERIODS },
    Dimension { key: "month", values: PERIODS },
    // How many of the seven hours a reader has opened today. The server derives it as each hour
    // is first opened ([`offices_scope`]): a reader sits under one value, the day's count so far.
    Dimension { key: "offices", values: OFFICES },
    // The beacon's vocabulary: a client that writes the families below says which version of it
    // it writes, so the report can tell how many readers' clients report them. A client from
    // before says nothing.
    Dimension { key: "version", values: VERSIONS },
    // How long ago this browser or installation was first counted: in its first week, its first
    // month, or longer ([`tenure`]). The device works it out from the day it keeps; only the
    // bucket is sent.
    Dimension { key: "tenure", values: TENURES },
    // On the first day of a week it is counted, on how many days the device was counted the week
    // before, if it was counted then ([`last_week`]): only the bucket is sent, and only once a week.
    Dimension { key: "last-week", values: LAST_WEEK },
    // An hour opened at its own time by the reader's clock, as home's invitation reckons it
    // ([`at_its_time`]). Sent only on that hour's own beacon.
    Dimension { key: "timely", values: HOURS_ },
    // An hour read to its end at a praying pace, as home's mark of it ([`crate::Reading`]). Sent
    // only on that hour's own beacon, once the mark is made.
    Dimension { key: "prayed", values: HOURS_ },
    // An hour opened by a reader whose client says its `version`: the denominator of `timely` and
    // `prayed`, which older clients never send. The server derives it.
    Dimension { key: "reported", values: HOURS_ },
    // Whether the device opened an hour that could not report (no network) since it last
    // reported. Only the yes is sent; the readers whose client says its `version` are the rest.
    Dimension { key: "offline", values: OFFLINE },
    // In the apps, whether the app's reminders are on. The web's reminders are a calendar link,
    // which the page cannot see.
    Dimension { key: "reminder", values: CHANT },
    // How many hours a reader opened on their first day: `offices` for the readers whose visit
    // that day was their first. The server derives it.
    Dimension { key: "new-offices", values: OFFICES },
];

/// The hours as a family's values (the `timely`, `prayed` and `reported` families).
const HOURS_: &[&str] = &HOURS;

/// The beacon vocabulary this build writes, as its `version` token says.
pub const VERSION: &str = "2";

/// The `version` family's values.
pub const VERSIONS: &[&str] = &[VERSION];

/// The `tenure` family's values: the first week, the first month, and after.
pub const TENURES: &[&str] = &["week", "month", "longer"];

/// The `last-week` family's values: days counted in the week before.
pub const LAST_WEEK: &[&str] = &["1", "2-3", "4-6", "7"];

/// The `offline` family's values.
pub const OFFLINE: &[&str] = &["yes"];

/// The families a beacon may send only from the hour they name.
const OWN_HOUR: &[&str] = &["timely", "prayed"];

/// The day from which every front kept the day it was first counted. A device that cannot say its
/// first day ("before") was first counted earlier, so it is known to be past its first month only
/// once that day is a month gone.
pub const FIRST_DAYS_KEPT_SINCE: (i32, u32, u32) = (2026, 11, 10);

/// The `tenure` of a device first counted on `first` (`None` for one from before first days were
/// kept), on `today`: none when it cannot be told.
pub fn tenure(first: Option<Date>, today: Date) -> Option<&'static str> {
    let days = match first {
        Some(first) => today.days_since(first),
        None => {
            let (y, m, d) = FIRST_DAYS_KEPT_SINCE;
            let kept = today.days_since(Date::new(y, m as i32, d as i32));
            return (kept >= 30).then_some("longer");
        }
    };
    Some(match days {
        // A first day ahead of today (a clock set back) says nothing, as the web's does.
        ..0 => return None,
        0..7 => "week",
        7..30 => "month",
        _ => "longer",
    })
}

/// A device's tally of the days it was counted in a week: the week's Monday and the days so far.
/// It moves on each time a beacon is counted on a new reporting day.
pub fn tally_after(kept: Option<(Date, u32)>, day: Date) -> (Date, u32) {
    let monday = week_start(day);
    match kept {
        Some((m, n)) if m == monday => (m, n + 1),
        _ => (monday, 1),
    }
}

/// The tally as a front keeps it: "2026-11-16 3".
pub fn tally_text((monday, days): (Date, u32)) -> String {
    format!("{} {days}", crate::date_slug(monday))
}

/// A kept tally read back; none when garbled.
pub fn tally_parse(text: &str) -> Option<(Date, u32)> {
    let (monday, days) = text.split_once(' ')?;
    Some((Date::parse(monday)?, days.parse().ok().filter(|&n| (1..=7).contains(&n))?))
}

/// The `last-week` bucket a device's beacon carries on `today`, the first day it is counted in
/// its week: its tally when that was of the week before, and none otherwise (it was not counted
/// then, or the tally is of an older week).
pub fn last_week(kept: Option<(Date, u32)>, today: Date) -> Option<&'static str> {
    let (monday, days) = kept?;
    (monday == week_start(today).add_days(-7)).then_some(match days {
        1 => "1",
        2..=3 => "2-3",
        4..=6 => "4-6",
        _ => "7",
    })
}

/// Whether `hour` of `page_day` is the hour home's invitation would open at the reader's
/// `clock_hour` on their `local_day`: Compline after midnight is the day before's.
pub fn at_its_time(hour: &str, page_day: Date, local_day: Date, clock_hour: i8) -> bool {
    let (slug, _, offset) = crate::current_hour_entry(clock_hour);
    slug == hour && page_day == local_day.add_days(offset)
}

/// What a device says beside the page's own facts: how long it has been counted, how often it was
/// last week, whether the hour was opened at its time or prayed, whether it opened one offline,
/// and, in the apps, whether reminders are on. Every front that writes these says [`VERSION`].
#[derive(Clone, Copy, Debug, Default, PartialEq, Eq)]
pub struct Habits {
    pub tenure: Option<&'static str>,
    pub last_week: Option<&'static str>,
    pub timely: bool,
    pub prayed: bool,
    pub offline: bool,
    pub reminders: Option<bool>,
}

impl Habits {
    /// The tokens, in the order every front writes them, after the week and month words.
    pub fn tokens(&self, scope: &str) -> String {
        let mut out = format!(" version:{VERSION}");
        if let Some(t) = self.tenure {
            out.push_str(&format!(" tenure:{t}"));
        }
        if let Some(n) = self.last_week {
            out.push_str(&format!(" last-week:{n}"));
        }
        if HOURS.contains(&scope) {
            for (key, on) in [("timely", self.timely), ("prayed", self.prayed)] {
                if on {
                    out.push_str(&format!(" {key}:{scope}"));
                }
            }
        }
        if self.offline {
            out.push_str(" offline:yes");
        }
        if let Some(on) = self.reminders {
            out.push_str(&format!(" reminder:{}", CHANT[usize::from(!on)]));
        }
        out
    }
}

/// The `week` and `month` families' values: the period's first counted day, then every later one.
pub const PERIODS: &[&str] = &["first", "again"];

/// The `offices` family's values: the number of hours opened in a day.
pub const OFFICES: &[&str] = &["1", "2", "3", "4", "5", "6", "7"];

/// The families the server derives; a beacon cannot claim them.
const DERIVED: &[&str] = &["new-client", "offices", "reported", "new-offices"];

/// The `offices` scope of a reader who has opened `hours` of the seven today.
pub fn offices_scope(hours: usize) -> Option<String> {
    (1..=OFFICES.len()).contains(&hours).then(|| format!("offices:{hours}"))
}

/// The Monday that begins `day`'s reporting week.
pub fn week_start(day: Date) -> Date {
    day.add_days(-((day.weekday().number() + 6) % 7))
}

/// Whether `today` is the first day this device is counted in its calendar week, and in its
/// calendar month. `last` is the latest reporting day one of its beacons counted and `previous`
/// the counted day before that; today's own beacons all say the same, since once today is `last`
/// the answer is read from `previous`. A device with no days kept is first in both.
pub fn periods(last: Option<Date>, previous: Option<Date>, today: Date) -> (bool, bool) {
    let before = if last == Some(today) { previous } else { last };
    match before {
        None => (true, true),
        Some(b) => (week_start(b) != week_start(today), (b.year(), b.month()) != (today.year(), today.month())),
    }
}

/// The `client` family's values.
pub const CLIENTS: &[&str] = &["browser", "pwa", "android", "ios"];

/// The `new-client` scope a beacon's dimensions add: its client, when the visit is a first one.
pub fn new_client(dimensions: &[String]) -> Option<String> {
    if !dimensions.iter().any(|d| d == "visit:first") {
        return None;
    }
    let client = dimensions.iter().find_map(|d| d.strip_prefix("client:"))?;
    Some(format!("new-client:{client}"))
}

/// The `visit` family's values: a first day, then every day after.
pub const VISITS: &[&str] = &["first", "returning"];

/// The `martyrology` family's values, shown first.
pub const MARTYROLOGY: &[&str] = &["shown", "hidden"];

/// The `chant` family's values, notation first.
pub const CHANT: &[&str] = &["on", "off"];

/// Prime's `martyrology` token: whether the reading was shown.
fn martyrology_token(shown: bool) -> String {
    format!("martyrology:{}", MARTYROLOGY[usize::from(!shown)])
}

/// The family a stored dimension scope belongs to.
pub fn dimension_key(scope: &str) -> Option<&'static str> {
    DIMENSIONS
        .iter()
        .find(|d| d.values.iter().any(|v| scope.strip_prefix(d.key).and_then(|r| r.strip_prefix(':')) == Some(v)))
        .map(|d| d.key)
}

pub fn valid_scope(scope: &str) -> bool {
    scope == "site" || HOURS.contains(&scope) || EXTRA_SCOPES.contains(&scope)
}

/// One beacon: the page scope and the dimensions reported about it.
#[derive(Debug, PartialEq, Eq)]
pub struct Event {
    pub scope: String,
    pub dimensions: Vec<String>,
}

/// Reads a beacon body ("lauds appearance:apse screen:mobile"). Only the
/// scope must be understood; unknown or repeated dimension tokens are
/// dropped, so clients from either side of a deploy still count.
pub fn parse_event(body: &str) -> Option<Event> {
    let mut fields = body.split(' ');
    let scope = fields.next().unwrap_or("");
    if !valid_scope(scope) {
        return None;
    }
    let mut event = Event { scope: scope.to_string(), dimensions: Vec::new() };
    let mut seen = Vec::new();
    for field in fields {
        let Some(key) = dimension_key(field) else { continue };
        if DERIVED.contains(&key)
            || seen.contains(&key)
            || (matches!(key, "prayer-form" | "chant") && !HOURS.contains(&scope))
            || (key == "martyrology" && scope != "prime")
            || (OWN_HOUR.contains(&key) && field.split_once(':').map(|(_, hour)| hour) != Some(scope))
        {
            continue;
        }
        seen.push(key);
        event.dimensions.push(field.to_string());
    }
    // The hours' habits are read against `reported`, which only a beacon with a version adds to.
    if !event.dimensions.iter().any(|d| d.starts_with("version:")) {
        event.dimensions.retain(|d| !OWN_HOUR.iter().any(|f| d.strip_prefix(f).is_some_and(|v| v.starts_with(':'))));
    }
    Some(event)
}

/// A native app, as the `client` family names it.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum App {
    Android,
    Ios,
}

impl App {
    pub fn as_str(self) -> &'static str {
        match self {
            App::Android => "android",
            App::Ios => "ios",
        }
    }
}

/// A native app's beacon for `scope`, or `None` for a scope the server would refuse. The apps
/// run on phones and tablets, which the web's screen family counts as mobile (a touch-primary
/// pointer); an office page adds the prayer form it was read in, and Prime whether the reader's
/// setting showed its Martyrology (`None` on a day without one), and an hour with a score whether
/// the Chant setting showed it (`None` on an hour without one). `first` is whether the
/// installation is counted for the first time today (`None` when it cannot tell); `week` and
/// `month` whether today is its first counted day of the week and of the month ([`periods`]);
/// `habits` the rest of what the installation says ([`Habits`]), from a build that says it.
#[allow(clippy::too_many_arguments)]
pub fn app_beacon(
    scope: &str,
    app: App,
    dark: bool,
    form: &str,
    martyrology: Option<bool>,
    chant: Option<bool>,
    first: Option<bool>,
    week: Option<bool>,
    month: Option<bool>,
    habits: Option<&Habits>,
) -> Option<String> {
    if !valid_scope(scope) {
        return None;
    }
    let appearance = if dark { "apse" } else { "nave" };
    let mut body = format!("{scope} appearance:{appearance} screen:mobile");
    if HOURS.contains(&scope) && PRAYER_FORMS.contains(&form) {
        body.push_str(&format!(" prayer-form:{form}"));
    }
    if let Some(shown) = martyrology.filter(|_| scope == "prime") {
        body.push(' ');
        body.push_str(&martyrology_token(shown));
    }
    if let Some(on) = chant.filter(|_| HOURS.contains(&scope)) {
        body.push_str(&format!(" chant:{}", CHANT[usize::from(!on)]));
    }
    if let Some(first) = first {
        body.push_str(&format!(" visit:{}", VISITS[usize::from(!first)]));
    }
    body.push_str(&format!(" client:{}", app.as_str()));
    for (key, first) in [("week", week), ("month", month)] {
        if let Some(first) = first {
            body.push_str(&format!(" {key}:{}", PERIODS[usize::from(!first)]));
        }
    }
    if let Some(habits) = habits {
        body.push_str(&habits.tokens(scope));
    }
    Some(body)
}

/// The endpoint an [`ENDPOINT_HEADER`] names, if it is one an app may post to: the usage path
/// on a plain HTTPS host name, so a garbled or hostile value can only be ignored.
pub fn advertised_endpoint(value: &str) -> Option<String> {
    let host = value.trim().strip_prefix("https://")?.strip_suffix("/api/usage")?;
    let labels: Vec<&str> = host.split('.').collect();
    let valid = host.len() <= 253
        && labels.len() >= 2
        && labels.iter().all(|l| {
            (1..=63).contains(&l.len())
                && l.bytes().all(|c| c.is_ascii_lowercase() || c.is_ascii_digit() || c == b'-')
                && !l.starts_with('-')
                && !l.ends_with('-')
        });
    valid.then(|| format!("https://{host}/api/usage"))
}

/// The header value naming `host`'s endpoint, for [`advertised_endpoint`] to read back.
pub fn endpoint_for(host: &str) -> String {
    format!("https://{host}/api/usage")
}

/// Whether a page dated `day` is current enough to count: today, or a day either side, which
/// covers a reader in another time zone than the reporting day's. The dated archive is
/// unbounded, so counting it would let a crawler mint a visitor per URL.
pub fn current_day(day: Date, today: Date) -> bool {
    day.days_since(today).abs() <= 1
}

/// Whether an ordo year is current enough to count: this year or either neighbour.
pub fn current_year(year: i32, today: Date) -> bool {
    (year - today.year()).abs() <= 1
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn beacon_dimensions_parse() {
        let dims = |v: &[&str]| v.iter().map(|s| s.to_string()).collect::<Vec<_>>();
        for (body, scope, want) in [
            ("lauds appearance:apse screen:mobile", "lauds", vec!["appearance:apse", "screen:mobile"]),
            ("site appearance:nave screen:desktop", "site", vec!["appearance:nave", "screen:desktop"]),
            // A client from before dimensions existed.
            ("vespers", "vespers", vec![]),
            // Unreadable tokens are dropped; a bare value names no family.
            ("ordo chant:gabc appearance:apse", "ordo", vec!["appearance:apse"]),
            ("ordo apse", "ordo", vec![]),
            // One value per family wins.
            ("prime appearance:nave appearance:apse screen:mobile", "prime", vec!["appearance:nave", "screen:mobile"]),
            ("site client:pwa client:browser", "site", vec!["client:pwa"]),
            // The Martyrology belongs to Prime alone.
            ("prime martyrology:shown client:pwa", "prime", vec!["martyrology:shown", "client:pwa"]),
            ("lauds martyrology:shown", "lauds", vec![]),
            ("site martyrology:hidden", "site", vec![]),
            // Chant notation is reported by the hours alone, and only in its own words.
            ("compline chant:on client:ios", "compline", vec!["chant:on", "client:ios"]),
            ("vespers chant:off chant:on", "vespers", vec!["chant:off"]),
            ("site chant:on", "site", vec![]),
            ("ordo chant:off", "ordo", vec![]),
            // The prayer form belongs to the hours; unknown families are dropped.
            (
                "lauds appearance:apse screen:mobile prayer-form:priest",
                "lauds",
                vec!["appearance:apse", "screen:mobile", "prayer-form:priest"],
            ),
            ("ordo prayer-form:priest appearance:nave appearance:apse future:x", "ordo", vec!["appearance:nave"]),
            // Any page can say whether the reader is new.
            ("site visit:first client:browser", "site", vec!["visit:first", "client:browser"]),
            ("vespers visit:returning visit:first", "vespers", vec!["visit:returning"]),
            // The server derives the new reader's client; a beacon cannot claim it.
            ("site new-client:android visit:first", "site", vec!["visit:first"]),
            ("lauds offices:3 client:ios", "lauds", vec!["client:ios"]),
            // Any page can say whether the week and the month are new.
            ("ordo client:pwa week:first month:again", "ordo", vec!["client:pwa", "week:first", "month:again"]),
            ("site week:again week:first month:sometimes", "site", vec!["week:again"]),
            // An hour says it was opened at its time, or prayed, only of itself.
            ("lauds version:2 timely:lauds prayed:lauds", "lauds", vec!["version:2", "timely:lauds", "prayed:lauds"]),
            ("lauds timely:vespers prayed:prime offline:yes", "lauds", vec!["offline:yes"]),
            ("site timely:site prayed:lauds tenure:month last-week:2-3", "site", vec!["tenure:month", "last-week:2-3"]),
            // The server derives which hours were opened by a reporting client, and a new
            // reader's hours; a beacon cannot claim them.
            ("lauds reported:lauds new-offices:2 reminder:on", "lauds", vec!["reminder:on"]),
        ] {
            let event = parse_event(body).unwrap_or_else(|| panic!("{body:?} rejected"));
            assert_eq!((event.scope.as_str(), event.dimensions), (scope, dims(&want)), "{body:?}");
        }
        for body in ["", "matins", "matins appearance:nave", "appearance:nave", " lauds"] {
            assert!(parse_event(body).is_none(), "{body:?} accepted");
        }
    }

    #[test]
    fn new_readers_are_counted_by_client() {
        let dims = |body: &str| parse_event(body).unwrap().dimensions;
        assert_eq!(new_client(&dims("site visit:first client:android")).as_deref(), Some("new-client:android"));
        assert_eq!(new_client(&dims("lauds client:browser screen:mobile visit:first")).as_deref(), Some("new-client:browser"));
        for body in ["site visit:returning client:android", "site client:ios", "site visit:first", "site"] {
            assert_eq!(new_client(&dims(body)), None, "{body:?}");
        }
    }

    #[test]
    fn an_apps_build_is_counted_apart_from_its_words() {
        let dims = |body: &str| parse_event(body).unwrap().dimensions;
        assert_eq!(build_scope(&dims("lauds client:android"), "2009").as_deref(), Some("build:android:2009"));
        assert_eq!(build_scope(&dims("site client:ios visit:first"), "31").as_deref(), Some("build:ios:31"));
        assert_eq!(build_of("build:ios:31"), Some(("ios", 31)));
        // A browser is always the current site; an app that says nothing, or something else, is unbuilt.
        for (body, header) in [
            ("lauds client:browser", "2009"),
            ("lauds client:pwa", "2009"),
            ("lauds", "2009"),
            ("lauds client:android", ""),
            ("lauds client:android", "0"),
            ("lauds client:android", "02009"),
            ("lauds client:android", "1234567"),
            ("lauds client:android", "20 09"),
            ("lauds client:android", "+2009"),
            ("lauds client:android", "2009:ios"),
        ] {
            assert_eq!(build_scope(&dims(body), header), None, "{body:?} {header:?}");
        }
        // No build scope is taken for a dimension, so the beacon's vocabulary stays closed.
        assert_eq!(dimension_key("build:android:2009"), None);
    }

    #[test]
    fn dimension_vocabulary_is_unambiguous() {
        // Counts live under "<key>:<value>" forever: keys and values stay
        // distinct and colon-free, and no page scope looks qualified.
        let mut keys = Vec::new();
        let mut scopes = Vec::new();
        for d in &DIMENSIONS {
            assert!(!d.key.is_empty() && !d.key.contains(':') && !keys.contains(&d.key), "key {:?}", d.key);
            keys.push(d.key);
            for (i, value) in d.values.iter().enumerate() {
                assert!(!value.is_empty() && !value.contains(':') && !d.values[..i].contains(value), "{} value {value:?}", d.key);
                let scope = format!("{}:{value}", d.key);
                assert!(!scopes.contains(&scope) && !valid_scope(&scope), "scope {scope:?} collides");
                assert_eq!(dimension_key(&scope), Some(d.key));
                scopes.push(scope);
            }
        }
        for scope in ["site", "ordo", "reminders"].iter().chain(HOURS.iter()) {
            assert!(!scope.contains(':'));
        }
    }

    // Everything an app can send is read back whole by the server, and fits its limit.
    #[test]
    fn app_beacons_round_trip() {
        for app in [App::Android, App::Ios] {
            for scope in ["site", "ordo", "reminders"].iter().chain(HOURS.iter()) {
                for dark in [false, true] {
                    for form in PRAYER_FORMS {
                        let options = [None, Some(true), Some(false)];
                        for (martyrology, chant, first) in
                            options.into_iter().flat_map(|m| options.into_iter().flat_map(move |c| options.map(|f| (m, c, f))))
                        {
                            let body = app_beacon(scope, app, dark, form, martyrology, chant, first, first, martyrology, None).unwrap();
                            assert!(body.len() <= MAX_BEACON, "{body:?} is too long");
                            let event = parse_event(&body).unwrap();
                            assert_eq!(event.scope, *scope);
                            assert_eq!(event.dimensions.len(), body.split(' ').count() - 1, "{body:?} lost a token");
                            assert_eq!(body.contains("prayer-form:"), HOURS.contains(scope), "{body:?}");
                            assert_eq!(body.contains("martyrology:"), *scope == "prime" && martyrology.is_some(), "{body:?}");
                            assert_eq!(body.contains("chant:"), HOURS.contains(scope) && chant.is_some(), "{body:?}");
                            assert_eq!(body.contains("visit:"), first.is_some(), "{body:?}");
                            assert_eq!(body.contains(" week:"), first.is_some(), "{body:?}");
                            assert_eq!(body.contains(" month:"), martyrology.is_some(), "{body:?}");
                        }
                    }
                }
            }
        }
        assert_eq!(
            app_beacon("compline", App::Android, false, "priest", None, None, None, None, None, None).as_deref(),
            Some("compline appearance:nave screen:mobile prayer-form:priest client:android")
        );
        assert_eq!(
            app_beacon("site", App::Ios, true, "priest", Some(true), Some(true), Some(true), None, None, None).as_deref(),
            Some("site appearance:apse screen:mobile visit:first client:ios")
        );
        assert_eq!(
            app_beacon("vespers", App::Ios, false, "cantor", None, Some(true), None, Some(true), Some(false), None).as_deref(),
            Some("vespers appearance:nave screen:mobile chant:on client:ios week:first month:again")
        );
        assert_eq!(
            app_beacon(
                "prime",
                App::Android,
                true,
                "private",
                Some(false),
                Some(false),
                Some(false),
                Some(false),
                Some(false),
                Some(&Habits { prayed: true, offline: true, ..Habits::default() })
            )
            .as_deref(),
            Some(
                "prime appearance:apse screen:mobile prayer-form:private martyrology:hidden chant:off visit:returning client:android week:again month:again version:2 prayed:prime offline:yes"
            )
        );
        assert!(app_beacon("matins", App::Android, false, "private", None, None, None, None, None, None).is_none());
    }

    // The longest beacon each front writes leaves room: one over the limit loses the whole visit.
    #[test]
    fn beacons_fit_the_limit() {
        // app.js's usageBeaconBody at its longest: Compline with a score, at its time and prayed,
        // after a week offline, from a browser past its first month.
        let web = "compline appearance:nave screen:desktop prayer-form:private chant:off visit:returning client:browser week:first month:again version:2 tenure:longer last-week:4-6 timely:compline prayed:compline offline:yes";
        let event = parse_event(web).unwrap();
        assert_eq!(event.dimensions.len(), 14, "the server dropped one of the web's tokens");
        let all =
            Habits { tenure: Some("longer"), last_week: Some("4-6"), timely: true, prayed: true, offline: true, reminders: Some(false) };
        let app =
            app_beacon("compline", App::Android, false, "private", None, Some(false), Some(false), Some(false), Some(false), Some(&all))
                .unwrap();
        assert_eq!(parse_event(&app).unwrap().dimensions.len(), app.split(' ').count() - 1, "{app:?}");
        for body in [web, app.as_str()] {
            assert!(body.len() + 32 <= MAX_BEACON, "{} bytes leaves no room in {MAX_BEACON}: {body:?}", body.len());
        }
    }

    #[test]
    fn habits_are_told_in_buckets() {
        let d = |m, day| Date::new(2026, m, day);
        // A device's first week, month and after; one from before first days were kept is past
        // its first month only once the keeping is a month old.
        assert_eq!(tenure(Some(d(11, 17)), d(11, 17)), Some("week"));
        assert_eq!(tenure(Some(d(11, 11)), d(11, 17)), Some("week"));
        assert_eq!(tenure(Some(d(11, 10)), d(11, 17)), Some("month"));
        assert_eq!(tenure(Some(d(10, 18)), d(11, 17)), Some("longer"));
        assert_eq!(tenure(None, d(12, 9)), None);
        assert_eq!(tenure(None, d(12, 10)), Some("longer"));
        // A first day ahead of today (a clock set back) says nothing.
        assert_eq!(tenure(Some(d(11, 18)), d(11, 17)), None);
        // An hour's habits are read against the hours a version reported, so without one they
        // are dropped.
        let words = |body: &str| parse_event(body).unwrap().dimensions;
        assert_eq!(words("lauds timely:lauds prayed:lauds offline:yes"), ["offline:yes"]);
        assert_eq!(words("lauds version:2 timely:lauds prayed:lauds"), ["version:2", "timely:lauds", "prayed:lauds"]);
        // The tally counts a week's days, and starts again with the next week.
        let monday = d(11, 16);
        let mut kept = None;
        for day in [d(11, 16), d(11, 18), d(11, 22)] {
            kept = Some(tally_after(kept, day));
        }
        assert_eq!(kept, Some((monday, 3)));
        assert_eq!(tally_text(kept.unwrap()), "2026-11-16 3");
        assert_eq!(tally_parse("2026-11-16 3"), kept);
        assert_eq!(tally_after(kept, d(11, 23)), (d(11, 23), 1));
        for garbled in ["", "2026-11-16", "2026-11-16 0", "2026-11-16 8", "x 3"] {
            assert_eq!(tally_parse(garbled), None, "{garbled:?}");
        }
        // It is told once the next week opens, and not after a week away.
        assert_eq!(last_week(kept, d(11, 25)), Some("2-3"));
        assert_eq!(last_week(Some((monday, 7)), d(11, 23)), Some("7"));
        assert_eq!(last_week(kept, d(11, 20)), None, "the same week");
        assert_eq!(last_week(kept, d(12, 1)), None, "a week missed");
        assert_eq!(last_week(None, d(11, 23)), None);
        // An hour at its time, by the reader's clock: Compline after midnight is yesterday's.
        assert!(at_its_time("lauds", d(11, 17), d(11, 17), 6));
        assert!(!at_its_time("lauds", d(11, 17), d(11, 17), 7));
        assert!(!at_its_time("lauds", d(11, 16), d(11, 17), 6), "yesterday's Lauds");
        assert!(at_its_time("compline", d(11, 16), d(11, 17), 1));
        assert!(!at_its_time("compline", d(11, 17), d(11, 17), 1));
        assert!(at_its_time("compline", d(11, 17), d(11, 17), 21));
        // The tokens come in one order, and an hour's own only on that hour.
        let habits = Habits { tenure: Some("week"), last_week: None, timely: true, prayed: true, offline: false, reminders: Some(true) };
        assert_eq!(habits.tokens("sext"), " version:2 tenure:week timely:sext prayed:sext reminder:on");
        assert_eq!(habits.tokens("site"), " version:2 tenure:week reminder:on");
        assert_eq!(Habits::default().tokens("ordo"), " version:2");
    }

    #[test]
    fn a_week_and_a_month_begin_on_the_first_counted_day() {
        let d = |m, day| Date::new(2026, m, day);
        // Monday 16 November 2026.
        assert_eq!(week_start(d(11, 16)), d(11, 16));
        assert_eq!(week_start(d(11, 22)), d(11, 16));
        assert_eq!(week_start(d(11, 15)), d(11, 9));
        for (last, previous, today, want) in [
            (None, None, d(11, 18), (true, true)),
            // Counted on Monday; Wednesday is the same week and month.
            (Some(d(11, 16)), None, d(11, 18), (false, false)),
            // Every beacon of the first day says first, the day's own mark notwithstanding.
            (Some(d(11, 18)), None, d(11, 18), (true, true)),
            (Some(d(11, 18)), Some(d(11, 15)), d(11, 18), (true, false)),
            (Some(d(11, 18)), Some(d(11, 16)), d(11, 18), (false, false)),
            // Sunday to Monday is a new week; the 30th to the 1st a new month in the same week.
            (Some(d(11, 22)), Some(d(11, 20)), d(11, 23), (true, false)),
            (Some(d(11, 30)), None, d(12, 1), (false, true)),
            // The same day of the month a year on is a new month.
            (Some(Date::new(2025, 11, 18)), None, d(11, 18), (true, true)),
        ] {
            assert_eq!(periods(last, previous, today), want, "{last:?} {previous:?} {today:?}");
        }
        assert_eq!(offices_scope(1).as_deref(), Some("offices:1"));
        assert_eq!(offices_scope(7).as_deref(), Some("offices:7"));
        assert!(offices_scope(0).is_none() && offices_scope(8).is_none());
    }

    #[test]
    fn apps_follow_only_a_plain_https_endpoint() {
        for host in ["office.fly.dev", "example.org", "office.example-parish.org"] {
            assert_eq!(advertised_endpoint(&endpoint_for(host)), Some(endpoint_for(host)), "{host}");
        }
        assert_eq!(advertised_endpoint(&endpoint_for("example.org")).as_deref(), Some("https://example.org/api/usage"));
        for value in [
            "",
            "example.org",
            "http://example.org/api/usage",
            "https://example.org/api/usage/",
            "https://example.org/other",
            "https://localhost/api/usage",
            "https://example.org:8443/api/usage",
            "https://user@example.org/api/usage",
            "https://Example.org/api/usage",
            "https://-bad.example.org/api/usage",
            "https://example..org/api/usage",
            "https://example.org/api/usage?x=1",
            "https://example.org/x/api/usage",
        ] {
            assert_eq!(advertised_endpoint(value), None, "{value:?}");
        }
        assert_eq!(advertised_endpoint(ENDPOINT).as_deref(), Some(ENDPOINT));
    }

    #[test]
    fn only_current_pages_count() {
        let today = Date::new(2026, 1, 1);
        for (day, current) in
            [((2025, 12, 31), true), ((2026, 1, 1), true), ((2026, 1, 2), true), ((2025, 12, 30), false), ((2026, 1, 3), false)]
        {
            assert_eq!(current_day(Date::new(day.0, day.1, day.2), today), current, "{day:?}");
        }
        assert!(current_year(2025, today) && current_year(2027, today) && !current_year(2028, today) && !current_year(2024, today));
    }
}
