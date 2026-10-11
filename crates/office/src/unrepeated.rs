//! A psalm whose antiphon is taken from its opening words does not repeat
//! them: "when the text of an Antiphon is taken from the beginning of a Psalm,
//! and begins exactly as the Psalm does, these beginning words of the Psalm
//! are not repeated after the Antiphon is begun. And that which follows in the
//! Psalm is continued from the place where the Antiphon text ends (whether it
//! is said whole or only begun), but only if the words are the same"
//! (General Rubrics XXIV.8).

use corpus::lines::{PsalmItem, parse_psalm};
use liturgy::{ElementType, OfficeHour, Unrepeated, antiphon_announcement, split_words};

/// Marks the psalms and canticles whose opening words their antiphon has just
/// said. Runs after antiphons are marked announced.
pub fn mark_unrepeated_openings(hour: &mut OfficeHour) {
    for section in &mut hour.sections {
        for i in 1..section.elements.len() {
            let (before, after) = section.elements.split_at_mut(i);
            let (antiphon, psalm) = (&before[i - 1], &mut after[0]);
            if antiphon.kind == ElementType::Antiphon && antiphon.label.is_empty() && psalm.kind.is_psalmody() {
                psalm.unrepeated = unrepeated(&antiphon.text, antiphon.announce, &psalm.text);
            }
        }
    }
}

/// What a psalm leaves unsaid after its antiphon. The antiphon must be taken
/// from the psalm's opening: all of it, or at least two words past its
/// intonation, not only a first few words that happen to coincide ("Blessed
/// be the holy Creator" before "Blessed be the Lord God of Israel"). When the whole
/// antiphon is said but departs from the psalm, only the intonation is not
/// repeated, as the Diurnal marks it ("The words Deliver me are not repeated
/// in the Psalm", p. 133).
fn unrepeated(antiphon: &str, announced: bool, psalm: &str) -> Option<Unrepeated> {
    let verse = keys(&first_verse(psalm)?);
    let whole = keys(antiphon);
    let begun = keys(&antiphon_announcement(antiphon));
    let common = whole.iter().zip(&verse).take_while(|(a, b)| a == b).count();
    if common < whole.len() && common < begun.len() + 2 {
        return None;
    }
    let said = if announced { begun.len() } else { whole.len() };
    let words = if common >= said { said } else { begun.len() };
    if words == 0 {
        return None;
    }
    let named = if words == said {
        String::new()
    } else {
        let bare = antiphon.replace(['*', '†', '‡'], " ");
        let head = split_words(&bare, words).0;
        head.split_whitespace().collect::<Vec<_>>().join(" ").trim_end_matches(|c: char| !c.is_alphanumeric()).to_string()
    };
    Some(Unrepeated { words, named })
}

/// The first verse of a psalm, its halves rejoined.
fn first_verse(psalm: &str) -> Option<String> {
    parse_psalm(psalm).items.into_iter().find_map(|item| match item {
        PsalmItem::Verse { first, second, .. } => Some(format!("{first} {second}")),
        PsalmItem::Section { .. } | PsalmItem::Gloria { .. } => None,
    })
}

/// The words of `s` for comparison: letters and digits only, lowercased, so
/// a psalm's capitals and an antiphon's punctuation do not matter.
fn keys(s: &str) -> Vec<String> {
    s.split_whitespace()
        .map(|w| w.chars().filter(|c| c.is_alphanumeric()).flat_map(char::to_lowercase).collect::<String>())
        .filter(|k| !k.is_empty())
        .collect()
}

#[cfg(test)]
mod tests {
    use super::*;

    const PS139: &str =
        "Psalm 139:1-11\n\nO LORD, thou hast searched me out and known me * thou knowest my down-sitting.\n2. Thou art about my path.";
    const PS140: &str = "Psalm 140\n\nDELIVER me, O Lord, from the evil man * and preserve me from the wicked man.";
    const PS133: &str = "Psalm 133\n\nBEHOLD, how good and joyful a thing it is * brethren, to dwell together in unity!";

    fn u(words: usize, named: &str) -> Option<Unrepeated> {
        Some(Unrepeated { words, named: named.to_string() })
    }

    #[test]
    fn unrepeated_opening_words() {
        let ps141 = "Psalm 141\n\nLORD, I call upon thee, haste thee unto me * and consider my voice.";
        let ps148 = "Psalm 148\n\nO PRAISE the Lord of heaven * praise him in the height.";
        let ps65 = "Psalm 65\n\nTHOU, O God, art praised in Sion * and unto thee shall the vow be performed.";
        let benedictus = "Benedictus\n\nBLESSED be the Lord God of Israel * for he hath visited, and redeemed his people;";
        for (ant, announced, psalm, want) in [
            // An antiphon from the opening is not repeated, whole or begun.
            ("O Lord, * thou hast searched me out and known me.", true, PS139, u(2, "")),
            ("O Lord, * thou hast searched me out and known me.", false, PS139, u(10, "")),
            // The unrepeated words may run past the mediant.
            ("Behold, * how good and joyful a thing it is, brethren, to dwell together in unity.", false, PS133, u(15, "")),
            // Diurnal p. 133: a whole antiphon that departs from the psalm leaves only its intonation unrepeated.
            ("Deliver me, * O Lord, from the wicked man.", false, PS140, u(2, "Deliver me")),
            ("Deliver me, * O Lord, from the wicked man.", true, PS140, u(2, "")),
            // An intonation that only coincides is repeated.
            ("O praise * God in his holiness.", true, ps148, None),
            ("Blessed be * the holy Creator and Governor of all things.", false, benedictus, None),
            ("Thou, O Lord, * that hearest the prayer.", true, ps65, None),
            // An antiphon without a mark counts whole.
            ("Lord, I call upon thee, haste thee unto me.", true, ps141, u(9, "")),
        ] {
            assert_eq!(unrepeated(ant, announced, psalm), want, "{ant} announced={announced}");
        }
    }
}
