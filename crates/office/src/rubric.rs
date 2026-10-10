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
        "shared/formulas/triduum-compline-opening-rubric" => &["Sir, ask a blessing", "Our help"],
        "shared/formulas/triduum-collect-rubric" => &["Let us pray", "Who with thee ... liveth"],
        "shared/formulas/triduum-vespers-opening-rubric" => &["Our Father", "Hail Mary"],
        "shared/formulas/holy-saturday-compline-alleluia-rubric" => &["Praise be to thee, O Lord, King of eternal glory", "Alleluia"],
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
    fn triduum_rubrics_mark_the_incipits_they_quote() {
        let opening = "Sir, ask a blessing is not said, nor the Short Lesson, nor Our help, nor the Lord's Prayer; but the Officiant begins the Confession immediately.";
        assert_eq!(prayed("shared/formulas/triduum-compline-opening-rubric", opening), ["Sir, ask a blessing", "Our help"]);
        let collect = "The Psalm being ended, there is said, without Let us pray, in a low voice, the Collect of the day, its conclusion Who with thee ... liveth being said in silence.";
        assert_eq!(prayed("shared/formulas/triduum-collect-rubric", collect), ["Let us pray", "Who with thee ... liveth"]);
        let alleluia = "Henceforth Praise be to thee, O Lord, King of eternal glory is not said, but in its place is said Alleluia.";
        assert_eq!(
            prayed("shared/formulas/holy-saturday-compline-alleluia-rubric", alleluia),
            ["Praise be to thee, O Lord, King of eternal glory", "Alleluia"]
        );
    }
}
