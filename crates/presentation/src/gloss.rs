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

/// Longest first, so "entirely in secret" is never read as a shorter term inside it.
const GLOSSES: &[Gloss] = &[
    Gloss { term: "entirely in secret", title: "Entirely in secret", note: "The whole prayer, shown in grey, is said silently." },
    Gloss {
        term: "secretly",
        title: "Secretly",
        note: "Only the first words are said aloud. The rest of the prayer, shown in grey, is said silently.",
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
    fn only_whole_words_are_glossed() {
        assert!(terms("The secretlyness of it.").is_empty());
        assert!(terms("désecretly").is_empty());
        assert!(terms("secretly’s").is_empty());
        assert_eq!(terms("«secretly»"), ["secretly"]);
        assert!(terms("Then the Officiant makes Confession:").is_empty());
        assert_eq!(gloss_runs(""), [GlossRun::Text("")]);
    }
}
