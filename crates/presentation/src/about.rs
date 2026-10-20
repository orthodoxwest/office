//! About the Office: the page for newcomers, in the words every front sets. The web renders it
//! at `/about`; the native apps read the same blocks through mobile-ffi, so a change to the
//! wording is made once, here.
//!
//! A paragraph may carry links as `[words](target)`: a site path (`/calendar`, `/reminders`,
//! `/privacy`), which the native apps map to their own screens or to the website, or a full
//! `https://` address.

/// One block of the About page, in reading order.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum AboutBlock {
    /// The opening paragraph, under the title.
    Intro(&'static str),
    /// A verse set as red work, with its reference.
    Verse { text: &'static str, cite: &'static str },
    /// A section's titulus.
    Heading(&'static str),
    /// A paragraph of prose, which may carry links.
    Paragraph(&'static str),
    /// The seven hours in home's three periods ([`ABOUT_HOURS`]).
    Hours,
    /// A quiet line set under the block before it.
    Note(&'static str),
    /// A key to the page's marks: the mark, set in its `ink`, then what it means.
    Key { mark: &'static str, ink: KeyInk, text: &'static str },
}

/// The ink a key's mark is set in, so it shows what it names.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum KeyInk {
    /// The body text's.
    Text,
    /// The rubrics' red.
    Rubric,
    /// The faded ink of words said silently.
    Unsaid,
}

impl KeyInk {
    /// The ink's name, as the web's class and the native apps read it.
    pub fn name(self) -> &'static str {
        match self {
            KeyInk::Text => "text",
            KeyInk::Rubric => "rubric",
            KeyInk::Unsaid => "unsaid",
        }
    }
}

/// The page's title.
pub const ABOUT_TITLE: &str = "About the Office";

/// The link home offers a reader in their first week, and the menu's name for the page.
pub const ABOUT_INTRODUCTION: &str = "Introduction";
pub const ABOUT_MENU: &str = "About";

/// How long after the first day home keeps offering the introduction, in days.
pub const ABOUT_NEWCOMER_DAYS: i64 = 7;

/// The seven hours in home's three periods, with when each is said.
pub const ABOUT_HOURS: [(&str, &[(&str, &str)]); 3] = [
    ("Morning", &[("lauds", "at daybreak"), ("prime", "about 7 am")]),
    ("Day", &[("terce", "about 9 am"), ("sext", "at noon"), ("none", "about 3 pm")]),
    ("Evening", &[("vespers", "toward sunset"), ("compline", "late in the evening")]),
];

/// The page, in reading order.
pub const ABOUT: &[AboutBlock] = &[
    AboutBlock::Intro(
        "The Divine Office is the Church’s prayer through the day: psalms, hymns, Scripture and prayers said at set \
         hours, morning to night. This is the Benedictine Office as the Antiochian Western Rite Vicariate prays it. \
         Each hour here is already put together for its day, with the feast, the season and the psalms in place, so \
         you need only begin.",
    ),
    AboutBlock::Verse { text: "“Seven times a day do I praise thee.”", cite: "Psalm 118 (119):164" },
    AboutBlock::Heading("An ancient prayer"),
    AboutBlock::Paragraph(
        "The Church has prayed at set hours since the Apostles, who kept the hours of prayer they had known in \
         Israel. The Holy Spirit came at Pentecost at the third hour; St Peter went up to pray at the sixth; St Peter \
         and St John went into the Temple at the ninth, “the hour of prayer.” By the third century Tertullian and St \
         Cyprian of Carthage wrote of prayer at the third, sixth and ninth hours, and at morning and evening. In the \
         deserts of Egypt and Palestine the monks made the psalms their unceasing prayer, and St John Cassian carried \
         their way of prayer to the West.",
    ),
    AboutBlock::Paragraph(
        "In the sixth century St Benedict of Nursia set the hours in order in his Rule for monks, taking the \
         Psalmist’s “seven times a day” as his measure and giving out the whole Psalter in each week. His life was \
         written by St Gregory the Great, whom the East honors as St Gregory the Dialogist, and St Benedict is kept \
         as a saint in East and West alike. His Rule spread through the monasteries of the West in the centuries \
         before the Great Schism. Since 1958 the Antiochian Archdiocese has kept a Western Rite, and within it the \
         Benedictine hours are prayed again in the Orthodox Church.",
    ),
    AboutBlock::Heading("The seven hours"),
    AboutBlock::Hours,
    AboutBlock::Note("The night office, Matins, is not yet included."),
    AboutBlock::Heading("Where to begin"),
    AboutBlock::Paragraph(
        "No one is expected to keep every hour at once. Many begin with one, Lauds in the morning or Compline at \
         night, and add others as they can. Your parish priest can help you determine an appropriate prayer rule. The \
         button on the home page always opens the hour for the time of day.",
    ),
    AboutBlock::Heading("Reading the page"),
    AboutBlock::Key {
        mark: "Red words",
        ink: KeyInk::Rubric,
        text: "are rubrics, directions for the one praying, and are not read aloud.",
    },
    AboutBlock::Key { mark: "℣. ℟.", ink: KeyInk::Rubric, text: "mark a versicle and its response. Praying alone, you say both." },
    AboutBlock::Key { mark: "✠", ink: KeyInk::Rubric, text: "marks where to make the sign of the cross." },
    AboutBlock::Key {
        mark: "Grey type",
        ink: KeyInk::Unsaid,
        text: "is said silently. A secret prayer’s opening words stand out and are said aloud, the rest in silence; \
               some Our Fathers and Creeds also end aloud, with a closing line in full type.",
    },
    AboutBlock::Key {
        mark: "Prayer form,",
        ink: KeyInk::Text,
        text: "at the head of each hour, sets the page for praying alone or with others led by a deacon or a priest.",
    },
    AboutBlock::Heading("The Ordo"),
    AboutBlock::Paragraph(
        "Each day keeps a feast or the office of its season, as the archdiocese’s printed ordo appoints, and the home \
         page wears the day’s liturgical color. The app’s [Ordo](/calendar) sets out the year a month at a time: each \
         day’s feast, rank and color, its fasts, and the antiphons and commemorations of Lauds and Vespers. A key \
         explains its marks, and the year’s title page gives the moveable feasts and Ember days.",
    ),
    AboutBlock::Heading("This app"),
    AboutBlock::Paragraph(
        "The psalm translation is from the Coverdale Psalter, and each text is checked against the printed book it \
         comes from. The phone apps work without a connection, and the website keeps the coming two weeks for \
         reading offline. [Reminders](/reminders) can call you to the hours you keep. In Settings (in the Menu on a \
         phone) you can choose the Nave, the church by day, or the Apse, its vault by night, change the text size, \
         and have Prime read the Martyrology, the saints of the coming day. Your settings are kept on your device. \
         The app counts pages read without knowing who reads them; there are no accounts and no advertising. See \
         [Privacy](/privacy). If you find a mistake, please tell us through the \
         [issue tracker](https://github.com/orthodoxwest/office/issues/new).",
    ),
];

/// A piece of a paragraph: plain words, or a link's words and its target.
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum AboutRun {
    Text(String),
    Link { text: String, target: String },
}

/// Splits a paragraph at its `[words](target)` links.
pub fn about_runs(paragraph: &str) -> Vec<AboutRun> {
    let mut runs = Vec::new();
    let mut rest = paragraph;
    while let Some(open) = rest.find('[') {
        let Some(close) = rest[open..].find("](").map(|i| open + i) else { break };
        let Some(end) = rest[close..].find(')').map(|i| close + i) else { break };
        if open > 0 {
            runs.push(AboutRun::Text(rest[..open].to_string()));
        }
        runs.push(AboutRun::Link { text: rest[open + 1..close].to_string(), target: rest[close + 2..end].to_string() });
        rest = &rest[end + 1..];
    }
    if !rest.is_empty() {
        runs.push(AboutRun::Text(rest.to_string()));
    }
    runs
}

/// Whether home still offers the introduction: the first day is within [`ABOUT_NEWCOMER_DAYS`]
/// of today. `days_since_first` is `None` for a reader from before first days were kept.
pub fn about_newcomer(days_since_first: Option<i64>) -> bool {
    days_since_first.is_some_and(|d| (0..ABOUT_NEWCOMER_DAYS).contains(&d))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn links_split_from_their_paragraph() {
        assert_eq!(
            about_runs("The [Ordo](/calendar) sets out every day."),
            vec![
                AboutRun::Text("The ".into()),
                AboutRun::Link { text: "Ordo".into(), target: "/calendar".into() },
                AboutRun::Text(" sets out every day.".into()),
            ]
        );
        assert_eq!(about_runs("No links."), vec![AboutRun::Text("No links.".into())]);
    }

    #[test]
    fn every_link_names_a_page_the_apps_can_open() {
        for block in ABOUT {
            if let AboutBlock::Paragraph(p) = block {
                for run in about_runs(p) {
                    if let AboutRun::Link { target, .. } = run {
                        assert!(
                            ["/calendar", "/reminders", "/privacy"].contains(&target.as_str()) || target.starts_with("https://"),
                            "{target}"
                        );
                    }
                }
            }
        }
    }

    #[test]
    fn the_hours_are_the_seven_in_order() {
        let slugs: Vec<_> = ABOUT_HOURS.iter().flat_map(|(_, hours)| hours.iter().map(|(slug, _)| *slug)).collect();
        assert_eq!(slugs, ["lauds", "prime", "terce", "sext", "none", "vespers", "compline"]);
    }

    #[test]
    fn a_newcomer_is_offered_the_introduction_for_a_week() {
        assert!(about_newcomer(Some(0)));
        assert!(about_newcomer(Some(6)));
        assert!(!about_newcomer(Some(7)));
        assert!(!about_newcomer(None));
    }
}
