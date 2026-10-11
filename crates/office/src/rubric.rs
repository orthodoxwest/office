//! Prayer words quoted inside instructional rubrics.

use liturgy::RubricSpan;

/// The reviewed prayer phrases quoted in each rubric, by corpus ref.
fn phrases(reference: &str) -> &'static [&'static str] {
    match reference {
        "ordinary/session/our-father-hail-mary-rubric" => &["Our Father", "Hail Mary"],
        "ordinary/session/prime-opening-secret-prayers-rubric" => &["Our Father", "Hail Mary", "Apostles' Creed"],
        "ordinary/session/little-hours-opening-rubric" => &["Our Father", "Hail Mary"],
        "ordinary/session/closing-rubric" => &["Our Father", "Hail Mary"],
        "shared/formulas/closing-our-father" => &["Our Father"],
        "ordinary/compline/confiteor-rubric" => &["Our Father"],
        "shared/formulas/triduum-collect-rubric" => &["Let us pray", "Who with thee … liveth"],
        // Diurnal p. 313 sets "Our Father" in black and "Ps. 51" in red: a psalm
        // named in a rubric is a label, not words said.
        "shared/formulas/triduum-secret-our-father-rubric" => &["Our Father"],
        _ => &[],
    }
}

/// Partitions a known rubric into instruction and quoted prayer words;
/// empty (fail closed) when a phrase is missing.
pub fn build_rubric_spans(reference: &str, text: &str) -> Vec<RubricSpan> {
    let phrases = phrases(reference);
    if phrases.is_empty() {
        return Vec::new();
    }
    let mut spans = Vec::new();
    let mut pos = 0;
    for phrase in phrases {
        let Some(i) = phrase_index(text, phrase, pos) else { return Vec::new() };
        if i > pos {
            spans.push(RubricSpan { text: text[pos..i].to_string(), prayed: false });
        }
        spans.push(RubricSpan { text: phrase.to_string(), prayed: true });
        pos = i + phrase.len();
    }
    if pos < text.len() {
        spans.push(RubricSpan { text: text[pos..].to_string(), prayed: false });
    }
    spans
}

/// Only a whole phrase counts: "Our Father's" is instruction prose.
fn phrase_index(text: &str, phrase: &str, start: usize) -> Option<usize> {
    let b = text.as_bytes();
    let mut from = start;
    while from + phrase.len() <= text.len() {
        let i = from + text.get(from..)?.find(phrase)?;
        let before_ok = i == 0 || !word_byte(b[i - 1]);
        let after = i + phrase.len();
        let after_ok = after == b.len() || !word_byte(b[after]);
        if before_ok && after_ok {
            return Some(i);
        }
        from = i + phrase.len();
    }
    None
}

fn word_byte(b: u8) -> bool {
    b.is_ascii_alphabetic() || b == b'\''
}

#[cfg(test)]
mod tests {
    use super::*;

    fn prayed(reference: &str, text: &str) -> Vec<String> {
        build_rubric_spans(reference, text).into_iter().filter(|s| s.prayed).map(|s| s.text).collect()
    }

    #[test]
    fn triduum_collect_rubric_marks_the_incipits_it_quotes() {
        let collect = "The Psalm being ended, there is said, without Let us pray, in a like voice, the Collect of the day, its conclusion Who with thee … liveth being said in silence.";
        assert_eq!(prayed("shared/formulas/triduum-collect-rubric", collect), ["Let us pray", "Who with thee … liveth"]);
    }

    /// Maintainer's direction, 2026-10-11: words actually said inside a rubric
    /// are black. "Our Father" is said in secret; "Psalm 51" and the other
    /// Triduum rubrics name nothing said, so they stay wholly red.
    #[test]
    fn triduum_ending_rubrics_mark_only_the_words_said() {
        let secret = "Our Father is said entirely in secret.";
        assert_eq!(prayed("shared/formulas/triduum-secret-our-father-rubric", secret), ["Our Father"]);
        let psalm = "Then in a somewhat higher voice is said Psalm 51.";
        assert!(build_rubric_spans("shared/formulas/triduum-psalm-51-rubric", psalm).is_empty());
        assert!(build_rubric_spans("shared/formulas/triduum-kneel-rubric", "All kneel.").is_empty());
        let noise = "But at Lauds, after the Collect is ended, a little noise and clatter is made; the lighted candle is at once brought forth from beneath the Altar; and all rise and depart in silence.";
        assert!(build_rubric_spans("shared/formulas/triduum-lauds-noise-rubric", noise).is_empty());
    }
}
