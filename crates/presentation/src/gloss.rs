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

/// Returns a quiet gloss for an hour's section heading when the term is specialized liturgical vocabulary.
pub fn heading_gloss(label: &str) -> Option<&'static Gloss> {
    match label.trim() {
        "Chapter" => Some(&CHAPTER_GLOSS),
        "Preces" => Some(&PRECES_GLOSS),
        "Suffrage" | "Suffrage of All Saints" => Some(&SUFFRAGE_GLOSS),
        _ => None,
    }
}

/// Tabula Temporaria & Ordo detail glosses, shared across Web, Android, and iOS.
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

/// Returns the gloss for a Tabula Temporaria figure label, if any.
pub fn tabula_figure_gloss(label: &str) -> Option<&'static Gloss> {
    match label {
        "Golden Number" => Some(&GOLDEN_NUMBER_GLOSS),
        "Dominical Letter" => Some(&DOMINICAL_LETTER_GLOSS),
        _ => None,
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
            ("lauds.txt", "Suffrage of All Saints"),
            ("vespers.txt", "Suffrage of All Saints"),
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
    fn only_whole_words_are_glossed() {
        assert!(terms("The secretlyness of it.").is_empty());
        assert!(terms("désecretly").is_empty());
        assert!(terms("secretly’s").is_empty());
        assert_eq!(terms("«secretly»"), ["secretly"]);
        assert!(terms("Then the Officiant makes Confession:").is_empty());
        assert_eq!(gloss_runs(""), [GlossRun::Text("")]);
    }
}
