//! Rubric terms a newcomer stumbles on, each with a short note a front can open beside the word.
//! The note describes what every front already shows: a secret prayer's voiced incipit at full
//! contrast and its silent body in the unsaid grey.

/// A rubric term and the note that explains it.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct Gloss {
    /// The words as the rubric prints them.
    pub term: &'static str,
    /// The note's heading.
    pub title: &'static str,
    /// The note itself.
    pub note: &'static str,
}

/// Longest first, so "entirely in secret" and "out of Choir" are never read as shorter terms inside them.
const GLOSSES: &[Gloss] = &[
    Gloss { term: "entirely in secret", title: "Entirely in secret", note: "The whole prayer, shown in grey, is said silently." },
    Gloss {
        term: "under the rite of a double",
        title: "Rite of a Double",
        note: "A fuller liturgical form where each antiphon is said in full before and after its psalm.",
    },
    Gloss { term: "out of choir", title: "Out of Choir", note: "Prayed privately or outside a church choir." },
    Gloss {
        term: "martyrology",
        title: "Martyrology",
        note: "The daily reading of the saints and martyrs commemorated on the coming day. You can turn it on in Settings.",
    },
    Gloss { term: "in choir", title: "In Choir", note: "Prayed together in church or community, rather than privately." },
    Gloss {
        term: "secretly",
        title: "Secretly",
        note: "Only the first words are said aloud. The rest of the prayer, shown in grey, is said silently.",
    },
];

/// Quiet structural glosses inside an hour: tappable without an underline at rest, so repeating
/// readers see uncluttered typography while a newcomer can tap an unfamiliar mark or heading.
pub const ANTIPHON_GLOSS: Gloss = Gloss {
    term: "Ant.",
    title: "Antiphon",
    note: "A short verse sung or read before and after a psalm, giving it the theme of the day or season. You do not read “Ant.” aloud.",
};

pub const PSALM_NUMBER_GLOSS: Gloss = Gloss {
    term: "Psalm numbering",
    title: "Psalm numbering",
    note: "The first number follows the traditional Septuagint and Vulgate psalter; the number in parentheses follows the Hebrew and King James numbering.",
};

pub const CHAPTER_GLOSS: Gloss = Gloss {
    term: "Chapter",
    title: "Chapter",
    note: "A brief Scripture reading (capitulum) appointed for this hour, usually a single verse.",
};

pub const PRECES_GLOSS: Gloss =
    Gloss { term: "Preces", title: "Preces", note: "Ferial prayers: extra versicles and responses said kneeling on penitential weekdays." };

pub const SUFFRAGE_GLOSS: Gloss =
    Gloss { term: "Suffrage", title: "Suffrage", note: "A short prayer asking the intercession of the Blessed Virgin and all the Saints." };

pub const COLLECT_GLOSS: Gloss = Gloss {
    term: "Collect",
    title: "Collect",
    note: "Pronounced KAH-lekt: the brief concluding prayer appointed for the day or hour, gathering the intentions of the office into one petition.",
};

pub const SHORT_RESPONSORY_GLOSS: Gloss = Gloss {
    term: "The Short Responsory",
    title: "Short Responsory",
    note: "A brief call-and-response verse following the Chapter, repeated in parts with half of the Glory be.",
};

pub const GOSPEL_CANTICLE_GLOSS: Gloss = Gloss {
    term: "The Gospel Canticle",
    title: "Gospel Canticle",
    note: "A song from the Gospel of St Luke: the Benedictus at Lauds, and the Magnificat at Vespers.",
};

pub const MARIAN_ANTIPHON_GLOSS: Gloss = Gloss {
    term: "Final Antiphon of the Blessed Virgin Mary",
    title: "Final Marian Antiphon",
    note: "One of the four seasonal antiphons of the Mother of God—Alma Redemptoris Mater, Ave Regina Caelorum, Regina Caeli, or Salve Regina—closing the office.",
};

pub const CANTICLE_ANTIPHON_GLOSS: Gloss = Gloss {
    term: "Gospel canticle antiphon",
    title: "Gospel canticle antiphon",
    note: "The antiphon framing the Benedictus at Lauds and the Magnificat at Vespers, said at each hour named.",
};

pub const VERSICLE_GLOSS: Gloss =
    Gloss {
        term: "Versicle", title: "Versicle", note: "A short verse (℣.) paired with a response (℟.). When praying alone, say both."
    };

pub const COMMEMORATION_OF_CROSS_GLOSS: Gloss = Gloss {
    term: "Commemoration of the Cross",
    title: "Commemoration of the Cross",
    note: "A short prayer honoring the Holy Cross, said from the Monday after Low Sunday to the Vigil of the Ascension, on days below a Double and outside octaves, in place of the Suffrage of All Saints.",
};

pub const POSTURE_GLOSS: Gloss = Gloss {
    term: "Posture cue",
    title: "Posture cue",
    note: "Choir directions (Sit., Stand., Bow.) for praying in community; optional when praying alone.",
};

/// Returns a quiet gloss for an hour's section heading when the term is specialized liturgical vocabulary.
pub fn heading_gloss(label: &str) -> Option<&'static Gloss> {
    match label.trim() {
        "Chapter" => Some(&CHAPTER_GLOSS),
        "Preces" => Some(&PRECES_GLOSS),
        "Suffrage" | "Suffrage of All Saints" => Some(&SUFFRAGE_GLOSS),
        "Collect" => Some(&COLLECT_GLOSS),
        "The Short Responsory" => Some(&SHORT_RESPONSORY_GLOSS),
        "The Gospel Canticle" => Some(&GOSPEL_CANTICLE_GLOSS),
        "Final Antiphon of the Blessed Virgin Mary" => Some(&MARIAN_ANTIPHON_GLOSS),
        "Opening Versicles" | "The Versicle" => Some(&VERSICLE_GLOSS),
        "Commemoration of the Cross" => Some(&COMMEMORATION_OF_CROSS_GLOSS),
        _ => None,
    }
}

/// Tabula Temporaria & Ordo detail glosses, shared across Web, Android, and iOS.
pub const ORDO_GLOSS: Gloss = Gloss {
    term: "Ordo",
    title: "Ordo",
    note: "The Church’s daily directory of feasts, ranks, liturgical colors, fasts, and office variations for the year.",
};

pub const TABULA_TEMPORARIA_GLOSS: Gloss = Gloss {
    term: "Tabula Temporaria",
    title: "Tabula Temporaria",
    note: "Latin for “table of the times”: the year’s computus figures, moveable feasts, and Ember days.",
};

pub const GOLDEN_NUMBER_GLOSS: Gloss = Gloss {
    term: "Golden Number",
    title: "Golden Number",
    note: "The year’s place (1–19) in the lunar cycle, used to reckon the date of Pascha (Easter).",
};

pub const DOMINICAL_LETTER_GLOSS: Gloss = Gloss {
    term: "Dominical Letter",
    title: "Dominical Letter",
    note: "Marks which day of January is the first Sunday of the year (A–G; two letters in a leap year).",
};

pub const EMBER_DAYS_GLOSS: Gloss = Gloss {
    term: "Ember days",
    title: "Ember days",
    note: "Four quarterly weeks of Wednesday, Friday, and Saturday fasting and prayer at the turning of the seasons.",
};

pub const ORDO_BENEDICTUS_GLOSS: Gloss = Gloss {
    term: "Ben.",
    title: "Benedictus antiphon",
    note: "The short verse framing the Song of Zechariah (Benedictus) near the end of Lauds.",
};

pub const ORDO_MAGNIFICAT_GLOSS: Gloss = Gloss {
    term: "Mag.",
    title: "Magnificat antiphon",
    note: "The short verse framing the Song of Mary (Magnificat) near the end of Vespers.",
};

pub const ORDO_COMMEMORATION_GLOSS: Gloss = Gloss {
    term: "Com.",
    title: "Commemoration",
    note: "An additional antiphon, versicle, and collect for a lesser feast or season kept on the same day.",
};

pub const FIRST_VESPERS_GLOSS: Gloss = Gloss {
    term: "I Vespers",
    title: "First Vespers",
    note: "Major feasts begin at Vespers the evening before. This evening’s Vespers already belongs to tomorrow’s feast.",
};

pub const SECOND_VESPERS_GLOSS: Gloss = Gloss {
    term: "II Vespers",
    title: "Second Vespers",
    note: "Vespers of the day itself, as distinct from First Vespers said the evening before.",
};

pub const MONASTIC_OBSERVANCE_GLOSS: Gloss = Gloss {
    term: "Monastics & Oblates",
    title: "Monastics & Oblates only",
    note: "An observance kept only by Benedictine monastics and oblates. The parish says the ordinary office of the day.",
};

/// Feast page quiet glosses.
pub const PROPER_GLOSS: Gloss = Gloss {
    term: "Proper of the feast",
    title: "Proper of the feast",
    note: "The texts said on this feast—its collect, antiphons, versicles, and hymns. Those it borrows from the texts shared by its class of saints are marked “Common”.",
};

pub const COMMON_GLOSS: Gloss = Gloss {
    term: "Common",
    title: "From the Common",
    note: "Texts shared across a class of saints (such as Apostles, Martyrs, or Confessors) when a feast does not have its own proper text.",
};

pub const RANK_1CL_GLOSS: Gloss = Gloss {
    term: "Double of the 1st Class",
    title: "Double of the 1st Class",
    note: "The highest rank of feast (such as Christmas, Easter, and Pentecost), giving way to almost no other observance.",
};

pub const RANK_2CL_GLOSS: Gloss = Gloss {
    term: "Double of the 2nd Class",
    title: "Double of the 2nd Class",
    note: "A principal feast, such as those of the Apostles and the greater feasts of the Lord and the Blessed Virgin, ranking below only the 1st Class.",
};

pub const RANK_GD_GLOSS: Gloss = Gloss {
    term: "Greater Double",
    title: "Greater Double",
    note: "A solemn Double, ranking above ordinary Doubles and below Doubles of the 2nd Class.",
};

pub const RANK_D_GLOSS: Gloss = Gloss {
    term: "Double",
    title: "Double",
    note: "A feast whose antiphons at Lauds and Vespers are said in full both before and after each psalm.",
};

pub const RANK_SD_GLOSS: Gloss = Gloss {
    term: "Semi-double",
    title: "Semi-double",
    note: "A lesser office whose antiphons are only begun before each psalm and said in full after it.",
};

pub const RANK_F2_GLOSS: Gloss = Gloss {
    term: "Privileged Feria",
    title: "Privileged Feria",
    note: "A weekday of the season with an office of its own, such as an Ember day or Rogation Monday, ranking with a Semi-double.",
};

pub const RANK_S_GLOSS: Gloss =
    Gloss { term: "Simple", title: "Simple", note: "A minor feast kept with a brief office and no Second Vespers." };

/// Home and hour-header quiet glosses.
pub const OCTAVE_GLOSS: Gloss = Gloss {
    term: "Octave",
    title: "Octave",
    note: "An eight-day celebration extending a major feast—from the feast day through the eighth day (the octave day).",
};

pub const SEPTUAGESIMA_GLOSS: Gloss = Gloss {
    term: "Septuagesima",
    title: "Septuagesima",
    note: "The pre-Lenten season of three Sundays preparing for Lent, beginning about seventy days before Easter.",
};

pub const PASSIONTIDE_GLOSS: Gloss = Gloss {
    term: "Passiontide",
    title: "Passiontide",
    note: "The final two weeks of Lent, from Passion Sunday through Holy Week, focused on Christ’s suffering and Cross.",
};

pub const FASTING_GLOSS: Gloss = Gloss {
    term: "Fasting",
    title: "Day of Fasting",
    note: "A traditional fast day of the Church. Consult your parish priest for your personal fasting rule.",
};

pub const ABSTINENCE_GLOSS: Gloss = Gloss {
    term: "Abstinence",
    title: "Day of Abstinence",
    note: "A day of abstaining from meat (and, in stricter seasons, other foods) according to the Church’s discipline.",
};

/// Returns the gloss for a Tabula Temporaria figure label, if any.
pub fn tabula_figure_gloss(label: &str) -> Option<&'static Gloss> {
    match label {
        "Golden Number" => Some(&GOLDEN_NUMBER_GLOSS),
        "Dominical Letter" => Some(&DOMINICAL_LETTER_GLOSS),
        _ => None,
    }
}

/// Returns the gloss for an Ordo Vespers note ("I Vespers of …" / "II Vespers of …").
pub fn vespers_note_gloss(note: &str) -> Option<&'static Gloss> {
    if note.starts_with("II Vespers") {
        Some(&SECOND_VESPERS_GLOSS)
    } else if note.starts_with("I Vespers") {
        Some(&FIRST_VESPERS_GLOSS)
    } else {
        None
    }
}

/// Returns a quiet gloss for a less-familiar season label, as `season_label` names it.
pub fn season_gloss(label: &str) -> Option<&'static Gloss> {
    match label.trim() {
        "Septuagesima" => Some(&SEPTUAGESIMA_GLOSS),
        "Passiontide" => Some(&PASSIONTIDE_GLOSS),
        _ => None,
    }
}

/// Returns a quiet gloss for Home's penitential badges ("Fasting", "Abstinence").
pub fn penitential_gloss(label: &str) -> Option<&'static Gloss> {
    match label.trim() {
        "Fasting" => Some(&FASTING_GLOSS),
        "Abstinence" => Some(&ABSTINENCE_GLOSS),
        _ => None,
    }
}

/// Returns a quiet gloss for a feast rank display name on the Feast page.
pub fn rank_gloss(label: &str) -> Option<&'static Gloss> {
    match label.trim() {
        "Double of the 1st Class" => Some(&RANK_1CL_GLOSS),
        "Double of the 2nd Class" => Some(&RANK_2CL_GLOSS),
        "Greater Double" => Some(&RANK_GD_GLOSS),
        "Double" => Some(&RANK_D_GLOSS),
        "Semi-double" => Some(&RANK_SD_GLOSS),
        "Privileged Feria" => Some(&RANK_F2_GLOSS),
        "Simple" => Some(&RANK_S_GLOSS),
        "Commemoration" => Some(&ORDO_COMMEMORATION_GLOSS),
        _ => None,
    }
}

/// Returns a quiet gloss for a non-collapsible text heading on the Feast page.
pub fn feast_text_gloss(label: &str) -> Option<&'static Gloss> {
    let label = label.trim();
    if label == "Collect" {
        Some(&COLLECT_GLOSS)
    } else if label.starts_with("Versicle") {
        Some(&VERSICLE_GLOSS)
    } else if label.ends_with("· Benedictus") {
        Some(&ORDO_BENEDICTUS_GLOSS)
    } else if label.ends_with("· Magnificat") {
        Some(&ORDO_MAGNIFICAT_GLOSS)
    } else if label.ends_with("· Antiphon") {
        // The antiphon of the feast's commemoration in another day's office (`office::feast`).
        Some(&ORDO_COMMEMORATION_GLOSS)
    } else if label.starts_with("At ") {
        // A canticle antiphon said at more than one hour ("At Lauds and II Vespers").
        Some(&CANTICLE_ANTIPHON_GLOSS)
    } else {
        None
    }
}

/// The ink style of a mark in the in-hour "How to read" key.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum ReadingKeyStyle {
    Rubric,
    Titulus,
    Unsaid,
    Psalm,
}

impl ReadingKeyStyle {
    pub fn name(self) -> &'static str {
        match self {
            ReadingKeyStyle::Rubric => "rubric",
            ReadingKeyStyle::Titulus => "titulus",
            ReadingKeyStyle::Unsaid => "unsaid",
            ReadingKeyStyle::Psalm => "psalm",
        }
    }
}

/// One row of the in-hour "How to read" disclosure, shared across every front.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct ReadingKeyItem {
    pub mark: &'static str,
    /// What a screen reader says for `mark`: its name where the mark is glyphs (℣. ℟., ✠), on
    /// every front.
    pub spoken: &'static str,
    pub style: ReadingKeyStyle,
    pub text: &'static str,
}

pub const READING_KEY_LABEL: &str = "How to read";
pub const READING_KEY_TITLE: &str = "Marks on this page";
pub const READING_KEY_FOOT_BEFORE: &str = "Posture cues (Sit., Stand., Bow.) are for choir and optional alone. More in ";
pub const READING_KEY_FOOT_LINK: &str = "About the Office";
pub const READING_KEY_FOOT_AFTER: &str = ".";

pub const READING_KEY_ITEMS: &[ReadingKeyItem] = &[
    ReadingKeyItem {
        mark: "Red words",
        spoken: "Red words",
        style: ReadingKeyStyle::Rubric,
        text: "Directions, not read aloud. Underlined words open a note when tapped.",
    },
    ReadingKeyItem {
        mark: "℣.  ℟.",
        spoken: "Versicle and response",
        style: ReadingKeyStyle::Rubric,
        text: "Versicle and response. Praying alone, say both.",
    },
    ReadingKeyItem {
        mark: "*  †",
        spoken: "Asterisk and dagger",
        style: ReadingKeyStyle::Rubric,
        text: "Pause briefly at * in each psalm verse, and take a breath at †.",
    },
    ReadingKeyItem { mark: "✠", spoken: "Cross", style: ReadingKeyStyle::Rubric, text: "Make the sign of the cross." },
    ReadingKeyItem {
        mark: "Ant.",
        spoken: "Antiphon",
        style: ReadingKeyStyle::Titulus,
        text: "Antiphon: a short verse framing a psalm or canticle. Tap it, a psalm’s number in parentheses or a heading like Chapter for a note.",
    },
    ReadingKeyItem {
        mark: "Grey type",
        spoken: "Grey type",
        style: ReadingKeyStyle::Unsaid,
        text: "Said silently, or opening words already spoken in the antiphon.",
    },
    ReadingKeyItem {
        mark: "62 (63)",
        spoken: "62 (63)",
        style: ReadingKeyStyle::Psalm,
        text: "Traditional Septuagint psalm number first; Hebrew number in parentheses.",
    },
];

/// A run of rubric text: plain instruction, or a glossed term.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum GlossRun<'a> {
    Text(&'a str),
    Term(&'a str, &'static Gloss),
}

/// Splits rubric instruction text at its glossed terms, matched as whole words. Text with no
/// term comes back as one run.
pub fn gloss_runs(text: &str) -> Vec<GlossRun<'_>> {
    let mut runs = Vec::new();
    let mut pos = 0;
    while let Some((i, gloss)) = next_term(text, pos) {
        if i > pos {
            runs.push(GlossRun::Text(&text[pos..i]));
        }
        let end = i + gloss.term.len();
        runs.push(GlossRun::Term(&text[i..end], gloss));
        pos = end;
    }
    if pos < text.len() || runs.is_empty() {
        runs.push(GlossRun::Text(&text[pos..]));
    }
    runs
}

/// The earliest whole-word term at or after `from`; at one position the longest term wins.
fn next_term(text: &str, from: usize) -> Option<(usize, &'static Gloss)> {
    // ASCII lowercasing keeps every byte where it was, so indices carry back to `text`.
    let lower = text.to_ascii_lowercase();
    let mut best: Option<(usize, &'static Gloss)> = None;
    for gloss in GLOSSES {
        let mut at = from;
        while let Some(found) = lower.get(at..).and_then(|rest| rest.find(gloss.term)) {
            let i = at + found;
            let end = i + gloss.term.len();
            let whole = !lower[..i].chars().next_back().is_some_and(word_char) && !lower[end..].chars().next().is_some_and(word_char);
            if whole {
                if best.is_none_or(|(j, _)| i < j) {
                    best = Some((i, gloss));
                }
                break;
            }
            at = i + 1;
        }
    }
    best
}

/// Letters of any script and both apostrophes belong to a word.
fn word_char(c: char) -> bool {
    c.is_alphanumeric() || c == '\'' || c == '’'
}

#[cfg(test)]
mod tests {
    use super::*;

    /// A screen reader names every mark in words; no front reads a glyph aloud.
    #[test]
    fn every_reading_key_mark_is_named_in_words() {
        for item in READING_KEY_ITEMS {
            assert!(!item.spoken.is_empty(), "{}", item.mark);
            assert!(!item.spoken.contains(['℣', '℟', '✠', '*', '†']), "{} is said {}", item.mark, item.spoken);
        }
    }

    fn terms(text: &str) -> Vec<&str> {
        gloss_runs(text)
            .into_iter()
            .filter_map(|r| match r {
                GlossRun::Term(t, _) => Some(t),
                GlossRun::Text(_) => None,
            })
            .collect()
    }

    fn joined(text: &str) -> String {
        gloss_runs(text)
            .into_iter()
            .map(|r| match r {
                GlossRun::Text(t) | GlossRun::Term(t, _) => t,
            })
            .collect()
    }

    #[test]
    fn secret_prayer_rubrics_gloss_their_term() {
        for rubric in [
            "The Our Father and Hail Mary are said secretly.",
            "Before the Office are said secretly Our Father and Hail Mary.",
            "Then secretly, Our Father, Hail Mary, and I believe.",
            "Secretly, the Our Father.",
        ] {
            assert_eq!(terms(rubric).len(), 1, "{rubric}");
            assert_eq!(joined(rubric), rubric);
        }
        assert_eq!(terms("Then secretly, Our Father")[0], "secretly");
        assert_eq!(terms("Secretly, the Our Father.")[0], "Secretly");
    }

    /// Nothing of a prayer said entirely in secret is voiced, so it takes its own note rather
    /// than the incipit's.
    #[test]
    fn entirely_in_secret_is_one_term_with_its_own_note() {
        let runs = gloss_runs("Our Father entirely in secret. Then confession is made:");
        let [GlossRun::Text("Our Father "), GlossRun::Term("entirely in secret", gloss), GlossRun::Text(". Then confession is made:")] =
            runs[..]
        else {
            panic!("{runs:?}");
        };
        assert_eq!(gloss.title, "Entirely in secret");
        assert!(!gloss.note.contains("aloud"));
    }

    #[test]
    fn prime_and_vespers_rubrics_gloss_liturgical_terms() {
        let prime = "Then the Martyrology is read in Choir, and this may laudably be done also in the private recitation of the Office out of Choir.";
        assert_eq!(terms(prime), ["Martyrology", "in Choir", "out of Choir"]);
        assert_eq!(joined(prime), prime);

        let dead = "After Let us bless the Lord, Vespers of the Dead are begun immediately under the rite of a Double, with the Antiphon I will walk.";
        assert_eq!(terms(dead), ["under the rite of a Double"]);
        assert_eq!(joined(dead), dead);
    }

    #[test]
    fn the_hours_section_labels_take_their_heading_gloss() {
        // The labels as data/office writes them, so a renamed section is caught here.
        for (file, label) in [
            ("lauds.txt", "Chapter"),
            ("prime.txt", "Preces"),
            ("compline.txt", "Preces"),
            ("prime.txt", "Collect"),
            ("compline.txt", "Collect"),
            ("lauds.txt", "Suffrage of All Saints"),
            ("vespers.txt", "Suffrage of All Saints"),
            ("lauds.txt", "Commemoration of the Cross"),
            ("vespers.txt", "Commemoration of the Cross"),
            ("lauds.txt", "The Short Responsory"),
            ("lauds.txt", "The Gospel Canticle"),
            ("lauds.txt", "The Versicle"),
            ("lauds.txt", "Opening Versicles"),
            ("lauds.txt", "Final Antiphon of the Blessed Virgin Mary"),
            ("vespers.txt", "Final Antiphon of the Blessed Virgin Mary"),
            ("compline.txt", "Final Antiphon of the Blessed Virgin Mary"),
        ] {
            let path = std::path::Path::new(env!("CARGO_MANIFEST_DIR")).join("../../data/office").join(file);
            let text = std::fs::read_to_string(&path).unwrap();
            if label != "Chapter" {
                assert!(text.lines().any(|l| l.trim() == format!("Label = {label}")), "{file} has no {label}");
            }
            assert!(heading_gloss(label).is_some(), "{label}");
        }
    }

    #[test]
    fn screen_gloss_lookups_cover_expected_terms() {
        assert_eq!(vespers_note_gloss("I Vespers of All Saints").map(|g| g.title), Some("First Vespers"));
        assert_eq!(vespers_note_gloss("II Vespers of preceding").map(|g| g.title), Some("Second Vespers"));
        assert!(vespers_note_gloss("").is_none());

        assert_eq!(season_gloss("Septuagesima").map(|g| g.title), Some("Septuagesima"));
        assert_eq!(season_gloss("Passiontide").map(|g| g.title), Some("Passiontide"));
        assert!(season_gloss("Advent").is_none());
        // The glosses key on the labels the headers actually show.
        for season in ["septuagesima", "passiontide"] {
            assert!(season_gloss(&crate::season_label(season)).is_some(), "{season}");
        }

        assert_eq!(penitential_gloss("Fasting").map(|g| g.title), Some("Day of Fasting"));
        assert_eq!(penitential_gloss("Abstinence").map(|g| g.title), Some("Day of Abstinence"));
        assert!(penitential_gloss("Other").is_none());
        // Keyed on the badges the calendar actually writes.
        for label in (calendar::model::Penitential { fast: true, abstinence: true }).labels() {
            assert!(penitential_gloss(label).is_some(), "{label}");
        }

        for rank in calendar::Rank::ALL {
            assert!(rank_gloss(rank.display_name()).is_some(), "missing rank gloss for {rank:?}");
        }

        assert_eq!(feast_text_gloss("Collect").map(|g| g.title), Some("Collect"));
        assert_eq!(feast_text_gloss("At Lauds · Benedictus").map(|g| g.title), Some("Benedictus antiphon"));
        assert_eq!(feast_text_gloss("At II Vespers · Magnificat").map(|g| g.title), Some("Magnificat antiphon"));
        assert_eq!(feast_text_gloss("At I Vespers · Antiphon").map(|g| g.title), Some(ORDO_COMMEMORATION_GLOSS.title));
        assert_eq!(feast_text_gloss("Versicle at Lauds").map(|g| g.title), Some("Versicle"));
        // A canticle antiphon said at several hours is an antiphon, not a note on Vespers.
        assert_eq!(feast_text_gloss("At Lauds and II Vespers").map(|g| g.title), Some("Gospel canticle antiphon"));
        assert_eq!(feast_text_gloss("At I Vespers, Lauds and II Vespers").map(|g| g.title), Some("Gospel canticle antiphon"));
    }

    #[test]
    fn only_whole_words_are_glossed() {
        assert!(terms("The secretlyness of it.").is_empty());
        assert!(terms("désecretly").is_empty());
        assert!(terms("secretly’s").is_empty());
        assert_eq!(terms("«secretly»"), ["secretly"]);
        assert!(terms("Then the Officiant makes Confession:").is_empty());
        assert_eq!(gloss_runs(""), [GlossRun::Text("")]);
    }
}
