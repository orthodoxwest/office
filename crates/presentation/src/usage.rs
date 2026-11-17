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

/// The longest beacon body the server reads. A longer one is refused whole, so the limit keeps
/// well clear of the longest beacon any front writes (`beacons_fit_the_limit`).
pub const MAX_BEACON: usize = 192;

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
pub const DIMENSIONS: [Dimension<'static>; 11] = [
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
];

/// The `week` and `month` families' values: the period's first counted day, then every later one.
pub const PERIODS: &[&str] = &["first", "again"];

/// The `offices` family's values: the number of hours opened in a day.
pub const OFFICES: &[&str] = &["1", "2", "3", "4", "5", "6", "7"];

/// The families the server derives; a beacon cannot claim them.
const DERIVED: &[&str] = &["new-client", "offices"];

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
        {
            continue;
        }
        seen.push(key);
        event.dimensions.push(field.to_string());
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
/// `month` whether today is its first counted day of the week and of the month ([`periods`]).
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
                            let body = app_beacon(scope, app, dark, form, martyrology, chant, first, first, martyrology).unwrap();
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
            app_beacon("compline", App::Android, false, "priest", None, None, None, None, None).as_deref(),
            Some("compline appearance:nave screen:mobile prayer-form:priest client:android")
        );
        assert_eq!(
            app_beacon("site", App::Ios, true, "priest", Some(true), Some(true), Some(true), None, None).as_deref(),
            Some("site appearance:apse screen:mobile visit:first client:ios")
        );
        assert_eq!(
            app_beacon("vespers", App::Ios, false, "cantor", None, Some(true), None, Some(true), Some(false)).as_deref(),
            Some("vespers appearance:nave screen:mobile chant:on client:ios week:first month:again")
        );
        assert_eq!(
            app_beacon("prime", App::Android, true, "private", Some(false), Some(false), Some(false), Some(false), Some(false)).as_deref(),
            Some(
                "prime appearance:apse screen:mobile prayer-form:private martyrology:hidden chant:off visit:returning client:android week:again month:again"
            )
        );
        assert!(app_beacon("matins", App::Android, false, "private", None, None, None, None, None).is_none());
    }

    // The longest beacon each front writes leaves room: one over the limit loses the whole visit.
    #[test]
    fn beacons_fit_the_limit() {
        // app.js's usageBeaconBody at its longest: Prime with a Martyrology and a score.
        let web = "prime appearance:nave screen:desktop prayer-form:private martyrology:hidden chant:off visit:returning client:browser week:again month:again";
        let event = parse_event(web).unwrap();
        assert_eq!(event.dimensions.len(), 9, "the server dropped one of the web's tokens");
        let app =
            app_beacon("prime", App::Android, false, "private", Some(false), Some(false), Some(false), Some(false), Some(false)).unwrap();
        for body in [web, app.as_str()] {
            assert!(body.len() + 32 <= MAX_BEACON, "{} bytes leaves no room in {MAX_BEACON}: {body:?}", body.len());
        }
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
