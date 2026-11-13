//! The reminder page says back what the reader has asked for, as one sentence under the choices:
//! "Lauds at 6:45 AM, Vespers at 6:00 PM, and Compline at 9:00 PM, every day, ten minutes
//! before." On the apps a second line names the reminder that will really come next, from the
//! schedule itself: "Next: Vespers, today at 5:50 PM."
//!
//! The words are here, for every front; each front formats its own clock ("6:45 AM", "18:45") and
//! passes it in. The web cannot ask the server on every change, so `app.js` builds the sentence
//! from [`RULE_WORDS`] (sent with the page) by the same grammar; the browser tests hold it to the
//! sentences these tests give.
//!
//! The grammar:
//! - the hours in the day's order, each "Name at time", joined as a list (an Oxford comma before
//!   "and" from three on). By the sun, an hour but Compline is "at sunrise", "at the first hour"…
//! - the days: "every day"; "on weekdays" for Monday to Friday; up to three days by name ("on
//!   Saturdays and Sundays"); from four, the days left out ("except Fridays");
//! - how long before: "ten minutes before", "at the hour", or, on the web's feed, "with no alert";
//! - with no hour or no day chosen, a plain line says so instead.

use calendar::Date;

/// The days of the week as the rule names them, Monday first as the page's Days row.
pub const DAY_PLURALS: [&str; 7] = ["Mondays", "Tuesdays", "Wednesdays", "Thursdays", "Fridays", "Saturdays", "Sundays"];

/// The rule's fixed words. The web is sent them as JSON with the page.
pub struct RuleWords {
    pub every_day: &'static str,
    pub weekdays: &'static str,
    /// Before up to three days named: "on Mondays".
    pub on: &'static str,
    /// Before the days left out, from four days chosen: "except Fridays".
    pub except: &'static str,
    /// Between an hour's name and its time: "Lauds at 6:45 AM".
    pub at: &'static str,
    /// The last joint of a list: "Lauds and Vespers", "Lauds, Vespers, and Compline".
    pub and: &'static str,
    /// Minutes before the hour, as the lead choices offer them; `None` is the web's alert-less feed.
    pub leads: [(Option<u32>, &'static str); 6],
    /// An hour's time while the hours follow the sun (the apps); Compline keeps its clock time.
    pub sun: [(&'static str, &'static str); 6],
    pub no_hour: &'static str,
    pub no_day: &'static str,
    pub no_hour_or_day: &'static str,
    /// The web's: a chosen hour whose time field is empty or invalid.
    pub no_time: &'static str,
}

pub const RULE_WORDS: RuleWords = RuleWords {
    every_day: "every day",
    weekdays: "on weekdays",
    on: "on",
    except: "except",
    at: "at",
    and: "and",
    leads: [
        (Some(0), "at the hour"),
        (Some(5), "five minutes before"),
        (Some(10), "ten minutes before"),
        (Some(15), "fifteen minutes before"),
        (Some(30), "thirty minutes before"),
        (None, "with no alert"),
    ],
    sun: [
        ("lauds", "sunrise"),
        ("prime", "the first hour"),
        ("terce", "the third hour"),
        ("sext", "solar noon"),
        ("none", "the ninth hour"),
        ("vespers", "sunset"),
    ],
    no_hour: "No hour is chosen yet.",
    no_day: "No day is chosen yet.",
    no_hour_or_day: "No hour or day is chosen yet.",
    no_time: "Choose a time for each chosen hour.",
};

/// When a chosen hour's reminder falls, as the rule says it.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum RuleTime<'a> {
    /// The reader's own time, in the front's clock style ("6:45 AM").
    Clock(&'a str),
    /// By the sun, where the apps follow it: "at sunrise" (Compline is never passed so).
    Sun,
}

/// A chosen hour, by its slug ("lauds"), in the day's order.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct RuleHour<'a> {
    pub hour: &'a str,
    pub time: RuleTime<'a>,
}

/// One phrase of the rule. `key` names what it says ("lauds", "lauds-time", "lauds-joint",
/// "days", "lead", "empty"), so a front that redraws the rule can leave the phrases that did not
/// change alone and fade only those that did.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct RulePhrase {
    pub key: String,
    pub text: String,
}

/// An hour's name by its slug, as the reminder page names it.
fn hour_name(slug: &str) -> String {
    crate::REMINDER_DEFAULTS.iter().find(|d| d.0 == slug).map_or_else(|| crate::title_case(slug), |d| d.1.to_string())
}

/// Joins a list as the rule does: "A", "A and B", "A, B, and C".
pub fn list(items: &[&str]) -> String {
    match items {
        [] => String::new(),
        [one] => one.to_string(),
        [a, b] => format!("{a} {} {b}", RULE_WORDS.and),
        [rest @ .., last] => format!("{}, {} {last}", rest.join(", "), RULE_WORDS.and),
    }
}

/// The joint after the `i`th of `n` items of a list: ", ", " and ", ", and ", or nothing after the last.
fn joint(i: usize, n: usize) -> String {
    match (n, n - 1 - i) {
        (_, 0) => String::new(),
        (2, _) => format!(" {} ", RULE_WORDS.and),
        (_, 1) => format!(", {} ", RULE_WORDS.and),
        _ => ", ".to_string(),
    }
}

/// The days, Monday first: "every day", "on weekdays", "on Saturdays and Sundays", "except
/// Fridays". None when no day is chosen.
pub fn days_phrase(days: [bool; 7]) -> Option<String> {
    let chosen: Vec<&str> = (0..7).filter(|&i| days[i]).map(|i| DAY_PLURALS[i]).collect();
    let left: Vec<&str> = (0..7).filter(|&i| !days[i]).map(|i| DAY_PLURALS[i]).collect();
    Some(match chosen.len() {
        0 => return None,
        7 => RULE_WORDS.every_day.to_string(),
        _ if days == [true, true, true, true, true, false, false] => RULE_WORDS.weekdays.to_string(),
        1..=3 => format!("{} {}", RULE_WORDS.on, list(&chosen)),
        _ => format!("{} {}", RULE_WORDS.except, list(&left)),
    })
}

/// How long before the hour: "ten minutes before", "at the hour"; `None` is "with no alert".
pub fn lead_phrase(lead: Option<u32>) -> String {
    RULE_WORDS.leads.iter().find(|l| l.0 == lead).map_or_else(|| format!("{} minutes before", lead.unwrap_or(0)), |l| l.1.to_string())
}

/// The rule as phrases: the chosen hours in order, the days, how long before (`days` Monday
/// first). With no hour or no day chosen, the one line that says so.
pub fn rule(hours: &[RuleHour], days: [bool; 7], lead: Option<u32>) -> Vec<RulePhrase> {
    let phrase = |key: String, text: String| RulePhrase { key, text };
    let Some(days) = days_phrase(days).filter(|_| !hours.is_empty()) else {
        let text = match (hours.is_empty(), days.contains(&true)) {
            (true, false) => RULE_WORDS.no_hour_or_day,
            (true, true) => RULE_WORDS.no_hour,
            _ => RULE_WORDS.no_day,
        };
        return vec![phrase("empty".into(), text.into())];
    };
    let mut out = Vec::new();
    for (i, h) in hours.iter().enumerate() {
        let time = match h.time {
            RuleTime::Clock(t) => t.to_string(),
            RuleTime::Sun => RULE_WORDS.sun.iter().find(|s| s.0 == h.hour).map_or("", |s| s.1).to_string(),
        };
        out.push(phrase(h.hour.into(), hour_name(h.hour)));
        out.push(phrase(format!("{}-time", h.hour), format!(" {} {time}", RULE_WORDS.at)));
        let j = joint(i, hours.len());
        if !j.is_empty() {
            out.push(phrase(format!("{}-joint", h.hour), j));
        }
    }
    out.push(phrase("days".into(), format!(", {days}")));
    out.push(phrase("lead".into(), format!(", {}.", lead_phrase(lead))));
    out
}

/// The rule as one sentence.
pub fn rule_text(hours: &[RuleHour], days: [bool; 7], lead: Option<u32>) -> String {
    rule(hours, days, lead).into_iter().map(|p| p.text).collect()
}

/// The reminder that will come next, under the rule on the apps: "Next: Vespers, today at 5:50
/// PM." `on` is its day and `time` the moment it rings, in the front's clock style; `today` is the
/// reader's. Tomorrow is named so, the rest of the week by its day, and the same day a week on
/// as "next Monday".
pub fn next_reminder(hour: &str, today: Date, on: Date, time: &str) -> String {
    let day = match on.days_since(today) {
        i32::MIN..=0 => "today".to_string(),
        1 => "tomorrow".to_string(),
        2..=6 => on.weekday().name().to_string(),
        _ => format!("next {}", on.weekday().name()),
    };
    format!("Next: {}, {day} at {time}.", hour_name(hour))
}

#[cfg(test)]
mod tests {
    use super::*;

    const EVERY: [bool; 7] = [true; 7];

    fn at(hour: &'static str, time: &'static str) -> RuleHour<'static> {
        RuleHour { hour, time: RuleTime::Clock(time) }
    }

    fn sun(hour: &'static str) -> RuleHour<'static> {
        RuleHour { hour, time: RuleTime::Sun }
    }

    /// Monday first, from the days' initials' positions: "MTWTF.." is Monday to Friday.
    fn week(pattern: &str) -> [bool; 7] {
        let mut out = [false; 7];
        for (i, c) in pattern.chars().enumerate() {
            out[i] = c != '.';
        }
        out
    }

    #[test]
    fn the_hours_are_listed_with_an_oxford_comma_from_three() {
        let one = [at("lauds", "6:45 AM")];
        let two = [at("lauds", "6:45 AM"), at("vespers", "6:00 PM")];
        let three = [at("lauds", "6:45 AM"), at("vespers", "6:00 PM"), at("compline", "9:00 PM")];
        assert_eq!(rule_text(&one, EVERY, Some(10)), "Lauds at 6:45 AM, every day, ten minutes before.");
        assert_eq!(rule_text(&two, EVERY, Some(10)), "Lauds at 6:45 AM and Vespers at 6:00 PM, every day, ten minutes before.");
        assert_eq!(
            rule_text(&three, EVERY, Some(10)),
            "Lauds at 6:45 AM, Vespers at 6:00 PM, and Compline at 9:00 PM, every day, ten minutes before."
        );
        let seven: Vec<RuleHour> = crate::REMINDER_DEFAULTS.iter().map(|d| at(d.0, "7:00")).collect();
        assert_eq!(
            rule_text(&seven, EVERY, Some(0)),
            "Lauds at 7:00, Prime at 7:00, Terce at 7:00, Sext at 7:00, None at 7:00, Vespers at 7:00, and Compline at 7:00, every day, at the hour."
        );
    }

    #[test]
    fn the_days_are_every_day_weekdays_named_or_excepted() {
        let lauds = [at("lauds", "6:45 AM")];
        let days = |pattern: &str| rule_text(&lauds, week(pattern), Some(10));
        for (pattern, want) in [
            ("MTWTFSS", "every day"),
            ("MTWTF..", "on weekdays"),
            ("......S", "on Sundays"),
            ("....F..", "on Fridays"),
            (".....SS", "on Saturdays and Sundays"),
            ("M.W.F..", "on Mondays, Wednesdays, and Fridays"),
            ("MTWT.SS", "except Fridays"),
            ("MTWTFS.", "except Sundays"),
            ("MT.T.SS", "except Wednesdays and Fridays"),
            ("MTWT...", "except Fridays, Saturdays, and Sundays"),
            (".TWTFS.", "except Mondays and Sundays"),
        ] {
            assert_eq!(days(pattern), format!("Lauds at 6:45 AM, {want}, ten minutes before."), "{pattern}");
        }
    }

    #[test]
    fn the_lead_is_said_in_words() {
        let lauds = [at("lauds", "06:45")];
        let lead = |l| rule_text(&lauds, EVERY, l);
        assert_eq!(lead(Some(0)), "Lauds at 06:45, every day, at the hour.");
        assert_eq!(lead(Some(5)), "Lauds at 06:45, every day, five minutes before.");
        assert_eq!(lead(Some(15)), "Lauds at 06:45, every day, fifteen minutes before.");
        assert_eq!(lead(Some(30)), "Lauds at 06:45, every day, thirty minutes before.");
        assert_eq!(lead(None), "Lauds at 06:45, every day, with no alert.");
        assert_eq!(lead(Some(20)), "Lauds at 06:45, every day, 20 minutes before.");
    }

    #[test]
    fn by_the_sun_the_hours_are_timed_by_it_and_compline_keeps_its_clock() {
        let hours = [sun("lauds"), sun("prime"), sun("terce"), sun("sext"), sun("none"), sun("vespers"), at("compline", "9:00 PM")];
        assert_eq!(
            rule_text(&hours, EVERY, Some(10)),
            "Lauds at sunrise, Prime at the first hour, Terce at the third hour, Sext at solar noon, None at the ninth hour, \
             Vespers at sunset, and Compline at 9:00 PM, every day, ten minutes before."
        );
        assert_eq!(rule_text(&[sun("vespers")], week("......S"), Some(5)), "Vespers at sunset, on Sundays, five minutes before.");
    }

    #[test]
    fn nothing_chosen_is_said_plainly() {
        assert_eq!(rule_text(&[], EVERY, Some(10)), "No hour is chosen yet.");
        assert_eq!(rule_text(&[at("lauds", "6:45 AM")], [false; 7], Some(10)), "No day is chosen yet.");
        assert_eq!(rule_text(&[], [false; 7], Some(10)), "No hour or day is chosen yet.");
    }

    #[test]
    fn each_phrase_is_keyed_by_what_it_says() {
        let keys: Vec<String> = rule(&[at("lauds", "6:45 AM"), at("vespers", "6:00 PM")], EVERY, Some(10)).into_iter().map(|p| p.key).collect();
        assert_eq!(keys, ["lauds", "lauds-time", "lauds-joint", "vespers", "vespers-time", "days", "lead"]);
        assert_eq!(rule(&[], EVERY, Some(10))[0].key, "empty");
    }

    #[test]
    fn the_next_reminder_names_its_day() {
        // Friday 13 November 2026.
        let today = Date::new(2026, 11, 13);
        let next = |days: i32, time| next_reminder("vespers", today, today.add_days(days), time);
        assert_eq!(next(0, "5:50 PM"), "Next: Vespers, today at 5:50 PM.");
        assert_eq!(next(1, "5:50 PM"), "Next: Vespers, tomorrow at 5:50 PM.");
        assert_eq!(next(2, "17:50"), "Next: Vespers, Sunday at 17:50.");
        assert_eq!(next(6, "5:50 PM"), "Next: Vespers, Thursday at 5:50 PM.");
        assert_eq!(next(7, "5:50 PM"), "Next: Vespers, next Friday at 5:50 PM.");
        assert_eq!(next_reminder("lauds", today, today.add_days(1), "6:35 AM"), "Next: Lauds, tomorrow at 6:35 AM.");
    }
}
