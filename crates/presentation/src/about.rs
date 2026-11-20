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
    /// The opening epigraph verse set as red work in the painted head, with its reference.
    Verse { text: &'static str, cite: &'static str },
    /// The opening paragraph, set with the app's painted two-line dropped initial.
    Intro(&'static str),
    /// A section's ruled titulus.
    Heading(&'static str),
    /// A paragraph of prose, which may carry links.
    Paragraph(&'static str),
    /// The painted Horologium (solar arc) and the seven hours in home's three periods ([`ABOUT_HOURS`]).
    Hours,
    /// A quiet italic note set under the block before it.
    Note(&'static str),
    /// A live miniature slice of an office page ([`ABOUT_SPECIMEN`]) demonstrating rubrics,
    /// versicles, the sign of the cross, silent prayer, and Prayer form.
    Specimen,
    /// A row of the ruled typographic key under the specimen: `id` links the row to the
    /// matching line in the specimen (`rubric`, `vr`, `cross`, `secret`, `form`); `tail`, when
    /// non-empty, is set in the unsaid ink after `mark` (e.g. `Our` + `Father…`).
    Key { id: &'static str, mark: &'static str, tail: &'static str, ink: KeyInk, text: &'static str },
    /// One chronological beat in "An ancient prayer", set beside a fine marginal thread.
    Era { period: &'static str, text: &'static str },
    /// The current year's liturgical color ribbon (`presentation::course`) and interactive color key
    /// ([`ABOUT_ORDO_COLORS`]).
    OrdoRibbon,
    /// Where else the Office is found, which each front words for itself: the web names the
    /// native apps and their stores ([`crate::stores::ABOUT_APPS`]), and the apps name the
    /// website ([`ABOUT_ON_THE_WEB`]), for those who found one and not the other.
    Elsewhere,
    /// Home's invitation to pray the hour it is now ([`crate::invitation`]): the same tablet,
    /// words and link as home's, still, as the page's last word before its tailpiece.
    Invitation,
}

/// The ink and treatment a key's mark takes, so it shows what it names.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum KeyInk {
    /// Red italic rubric text.
    Rubric,
    /// Red upright liturgical sigils (`℣. ℟.`).
    Symbol,
    /// Red Maltese cross (`✠`).
    Cross,
    /// Voiced opening word in body ink followed by silent grey tail.
    Secret,
    /// Small-caps titulus matching the hour's Prayer form control.
    Form,
}

impl KeyInk {
    /// The ink's name, as the web's class and the native apps read it.
    pub fn name(self) -> &'static str {
        match self {
            KeyInk::Rubric => "rubric",
            KeyInk::Symbol => "symbol",
            KeyInk::Cross => "cross",
            KeyInk::Secret => "secret",
            KeyInk::Form => "form",
        }
    }
}

/// The live miniature office specimen shown in "Reading the page": lines an hour prints, in
/// the order it prints them (Lauds' opening, then its close). Its switch offers the hour's three
/// prayer forms by their own labels ([`liturgy::PrayerForm::label`]); a deacon and a priest
/// greet alike, as the composed hour does, so `priest_*` serves both.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct AboutSpecimen {
    pub label: &'static str,
    pub alone_label: &'static str,
    pub deacon_label: &'static str,
    pub priest_label: &'static str,
    pub rubric: &'static str,
    pub opening_versicle: &'static str,
    pub opening_response: &'static str,
    pub alone_versicle: &'static str,
    pub alone_response: &'static str,
    pub priest_versicle: &'static str,
    pub priest_response: &'static str,
    pub secret_voiced: &'static str,
    pub secret_silent: &'static str,
}

/// One liturgical color explained beneath the Ordo's year ribbon.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct AboutOrdoColor {
    pub slug: &'static str,
    pub name: &'static str,
    pub meaning: &'static str,
}

/// What a screen reader hears for the year's ribbon of colors, which is drawn and not read.
pub const ABOUT_RIBBON_LABEL: &str = "This year’s liturgical colors, day by day. The Ordo gives each day’s.";

/// The small-caps kicker over the title in the painted head.
pub const ABOUT_KICKER: &str = "Benedictine Divine Office";

/// The page's title.
pub const ABOUT_TITLE: &str = "About the Office";

/// The link home offers a reader in their first week, and the menu's name for the page.
pub const ABOUT_INTRODUCTION: &str = "Introduction";
pub const ABOUT_MENU: &str = "About";

/// How long after the first day home keeps offering the introduction, in days.
pub const ABOUT_NEWCOMER_DAYS: i64 = 7;

/// The seven hours in home's three periods, with when each is said. The table marks the hour it
/// is now as home's directory does ([`about_current_hour`]). Every front sets the names in a
/// column as wide as the widest of the seven, measured, so the glosses line up and Prime's
/// ("Prayer as work begins") keeps one line beside it on a 390-point phone.
pub const ABOUT_HOURS: [(&str, &[(&str, &str)]); 3] = [
    ("Morning", &[("lauds", "at daybreak"), ("prime", "about 7 am")]),
    ("Day", &[("terce", "about 9 am"), ("sext", "at noon"), ("none", "about 3 pm")]),
    ("Evening", &[("vespers", "toward sunset"), ("compline", "late in the evening")]),
];

/// Each hour's station angle along the Horologium's semicircular arc, in degrees from the western
/// horizon (180° = dawn at left, 90° = solar noon at zenith, 0° = night at right).
pub fn about_horologium_angle(slug: &str) -> f64 {
    match slug {
        "lauds" => 168.0,
        "prime" => 144.0,
        "terce" => 118.0,
        "sext" => 90.0,
        "none" => 62.0,
        "vespers" => 34.0,
        "compline" => 10.0,
        _ => 90.0,
    }
}

/// Whether the Horologium marks an hour with the crescent moon rather than the sun.
pub fn about_horologium_is_night(slug: &str) -> bool {
    matches!(slug, "vespers" | "compline")
}

/// The miniature office leaf shown in "Reading the page".
pub const ABOUT_SPECIMEN: AboutSpecimen = AboutSpecimen {
    label: "From an hour",
    alone_label: liturgy::PrayerForm::Private.label(),
    deacon_label: liturgy::PrayerForm::Deacon.label(),
    priest_label: liturgy::PrayerForm::Priest.label(),
    // The rubric and the opening are the corpus's own (Lauds), the ✠ where the hour prints it.
    rubric: "The Our Father and Hail Mary are said secretly.",
    opening_versicle: "O God, ✠ make speed to save me.",
    opening_response: "O Lord, make haste to help me.",
    alone_versicle: "O Lord, hear my prayer.",
    alone_response: "And let my cry come unto thee.",
    priest_versicle: "The Lord be with you.",
    priest_response: "And with thy spirit.",
    secret_voiced: "Our Father,",
    secret_silent: "who art in heaven, Hallowed be thy Name…",
};

/// The six liturgical colors shown beneath the Ordo's year ribbon.
pub const ABOUT_ORDO_COLORS: &[AboutOrdoColor] = &[
    AboutOrdoColor { slug: "white", name: "White", meaning: "Feasts of the Lord, Eastertide, and most saints" },
    AboutOrdoColor { slug: "red", name: "Red", meaning: "Pentecost, apostles, and martyrs" },
    AboutOrdoColor { slug: "violet", name: "Violet", meaning: "Advent, Lent, and days of penance" },
    AboutOrdoColor { slug: "green", name: "Green", meaning: "Sundays and weekdays after Epiphany and Pentecost" },
    AboutOrdoColor { slug: "rose", name: "Rose", meaning: "Gaudete and Laetare Sundays" },
    AboutOrdoColor { slug: "black", name: "Black", meaning: "Good Friday and All Souls’ Day" },
];

/// The page, in reading order.
pub const ABOUT: &[AboutBlock] = &[
    AboutBlock::Verse { text: "“Seven times a day do I praise thee.”", cite: "Psalm 118 (119):164" },
    AboutBlock::Intro(
        "The Divine Office is the Church’s daily rhythm of psalms, hymns, and prayers, prayed at set \
         hours from morning to night. This app presents the Benedictine Office of the Antiochian \
         Western Rite Vicariate—assembled automatically for each day’s feast and season, so you can \
         simply open an hour and pray.",
    ),
    AboutBlock::Heading("The seven hours"),
    AboutBlock::Hours,
    AboutBlock::Note("Matins, the night office, is not yet included."),
    AboutBlock::Heading("Where to begin"),
    AboutBlock::Paragraph(
        "No one is expected to pray all seven hours at once. Most people begin with one—Prime in the \
         morning or Compline before sleep—and add others gradually. Your parish priest can help you \
         establish a prayer rule that fits your life. Whenever you open the home page, the main button \
         takes you straight to the hour for the current time of day.",
    ),
    AboutBlock::Heading("Reading the page"),
    AboutBlock::Specimen,
    AboutBlock::Key {
        id: "rubric",
        mark: "Rubrics",
        tail: "",
        ink: KeyInk::Rubric,
        text: "Directions for posture, gestures, and order—not read aloud.",
    },
    AboutBlock::Key {
        id: "vr",
        mark: "℣. ℟.",
        tail: "",
        ink: KeyInk::Symbol,
        text: "A versicle and its response. When praying alone, say both.",
    },
    AboutBlock::Key { id: "cross", mark: "✠", tail: "", ink: KeyInk::Cross, text: "Make the sign of the cross." },
    AboutBlock::Key {
        id: "secret",
        mark: "Our",
        tail: "Father…",
        ink: KeyInk::Secret,
        text: "Grey text is prayed silently; lines in full ink at the start or close of a silent prayer are spoken aloud.",
    },
    AboutBlock::Key {
        id: "form",
        mark: "Prayer form",
        tail: "",
        ink: KeyInk::Form,
        text: "At the top of each hour, switches between praying alone or in a group led by a deacon or priest.",
    },
    AboutBlock::Heading("An ancient prayer"),
    AboutBlock::Era {
        period: "From the Apostles",
        text: "Christians have prayed at set hours since the Apostles, who continued the daily times \
               of prayer they had known in Israel—the third hour at Pentecost, the sixth with St Peter, \
               and the ninth in the Temple. By the third century, Tertullian and St Cyprian describe \
               regular prayer at morning, evening, and the third, sixth, and ninth hours.",
    },
    AboutBlock::Era {
        period: "The Desert & St Benedict",
        text: "In the deserts of Egypt and Palestine, monastics made the psalms their unceasing prayer, \
               and St John Cassian carried their practice west. In the sixth century, St Benedict shaped \
               it in his Rule: seven daytime hours and a night watch, praying through the entire Psalter \
               each week. His life was written by St Gregory the Great, whom the East honors as St \
               Gregory the Dialogist.",
    },
    AboutBlock::Era {
        period: "Across the West",
        text: "The Benedictine Office spread through the monasteries of the West in the centuries \
               before the Great Schism.",
    },
    AboutBlock::Era {
        period: "The Western Rite",
        text: "Since 1958 the Antiochian Archdiocese has kept a Western Rite, and within it the \
               Benedictine hours are prayed again in the Orthodox Church.",
    },
    AboutBlock::Heading("The Ordo"),
    AboutBlock::Paragraph(
        "Every day follows the archdiocese’s printed ordo, observing either a feast or the office of \
         the season. Browse the [Ordo](/calendar) month by month to see each day’s feast, rank, \
         liturgical color, fasts, and commemorations—or search for any feast or saint to read its \
         collect, antiphons, versicles, and hymns, and see when it falls in coming years.",
    ),
    AboutBlock::OrdoRibbon,
    AboutBlock::Heading("This app"),
    AboutBlock::Paragraph(
        "The psalms follow the Coverdale translation, and the texts are checked against the printed \
         Western Rite books. You can set up [Reminders](/reminders) for the hours you pray, or open \
         Settings (under Menu on a phone) to switch between the Nave and Apse themes, adjust text \
         size, include the Martyrology at Prime, or display chant notation where available. The mobile \
         apps work without a connection.",
    ),
    AboutBlock::Paragraph(
        "Your preferences stay on your device—there are no accounts, no third-party trackers, and \
         no ads (see [Privacy](/privacy)). If you spot a typo or a rubrical error, please report it on the \
         [issue tracker](https://github.com/orthodoxwest/office/issues/new).",
    ),
    AboutBlock::Elsewhere,
    AboutBlock::Invitation,
];

/// The native apps' word on the website, in their [`AboutBlock::Elsewhere`]: a paragraph, which
/// may carry links as the page's paragraphs do.
pub const ABOUT_ON_THE_WEB: &str = "You can also pray from a computer or browser at \
     [orthodoxwestbreviary.com](https://orthodoxwestbreviary.com).";

/// The hour the table marks as now at a clock hour (0–23), as home's directory marks it: none
/// from midnight to 2 am, when the office being prayed is yesterday's Compline.
pub fn about_current_hour(clock_hour: i8) -> &'static str {
    let (hour, _, offset) = crate::current_hour_entry(clock_hour);
    if offset == 0 { hour } else { "" }
}

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
    fn the_website_is_named_by_its_address() {
        assert_eq!(
            about_runs(ABOUT_ON_THE_WEB).into_iter().find_map(|run| match run {
                AboutRun::Link { text, target } => Some((text, target)),
                AboutRun::Text(_) => None,
            }),
            Some(("orthodoxwestbreviary.com".into(), "https://orthodoxwestbreviary.com".into()))
        );
        assert!(crate::usage::ENDPOINT.starts_with("https://orthodoxwestbreviary.com/"));
    }

    /// Every front sets the hours, their heading and their note as one column (a rail when
    /// wide), finding the heading as the block before them: so the hours have one, and a note.
    #[test]
    fn the_hours_stand_under_their_heading() {
        let at = ABOUT.iter().position(|b| *b == AboutBlock::Hours).unwrap();
        assert!(matches!(ABOUT[at - 1], AboutBlock::Heading(_)));
        assert!(matches!(ABOUT.get(at + 1), Some(AboutBlock::Note(_))));
        assert_eq!(ABOUT.iter().filter(|b| **b == AboutBlock::Hours).count(), 1);
    }

    #[test]
    fn the_hours_are_the_seven_in_order() {
        let slugs: Vec<_> = ABOUT_HOURS.iter().flat_map(|(_, hours)| hours.iter().map(|(slug, _)| *slug)).collect();
        assert_eq!(slugs, ["lauds", "prime", "terce", "sext", "none", "vespers", "compline"]);
        // Solar angles progress strictly from east (dawn) to west (night), with Sext at zenith.
        let angles: Vec<_> = slugs.iter().map(|s| about_horologium_angle(s)).collect();
        assert!(angles.windows(2).all(|w| w[0] > w[1]));
        assert_eq!(about_horologium_angle("sext"), 90.0);
        assert!(about_horologium_is_night("vespers"));
        assert!(about_horologium_is_night("compline"));
        assert!(!about_horologium_is_night("none"));
    }

    #[test]
    fn the_table_lights_the_hour_home_lights() {
        let today = crate::Date::new(2026, 11, 13);
        for h in 0..24 {
            assert_eq!(about_current_hour(h), crate::invitation(today, today, h).current, "{h}");
        }
        assert_eq!(about_current_hour(1), "");
        assert_eq!(about_current_hour(2), "lauds");
        assert_eq!(about_current_hour(15), "none");
        assert_eq!(about_current_hour(23), "compline");
    }

    #[test]
    fn the_page_ends_with_home_s_invitation() {
        assert_eq!(ABOUT.last(), Some(&AboutBlock::Invitation));
        assert_eq!(ABOUT.iter().filter(|b| **b == AboutBlock::Invitation).count(), 1);
    }

    #[test]
    fn a_newcomer_is_offered_the_introduction_for_a_week() {
        assert!(about_newcomer(Some(0)));
        assert!(about_newcomer(Some(6)));
        assert!(!about_newcomer(Some(7)));
        assert!(!about_newcomer(None));
    }
}
