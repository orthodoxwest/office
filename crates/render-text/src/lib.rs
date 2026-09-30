//! Plain-text rendering of a composed hour, for the CLI and the text goldens.

use corpus::lines::{psalm_verse_lines, split_leading_verse_number};
use liturgy::{ElementType, OfficeElement, OfficeHour, PostureAnchor, VoiceRole, posture_cues_at};

const MONTHS: [&str; 12] =
    ["January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"];

/// Formats a composed hour. Announced antiphons print only through the
/// mediant.
pub fn format_office_hour(hour: &OfficeHour) -> String {
    let mut b = String::new();
    b.push_str(&hour.hour.to_uppercase());
    b.push('\n');
    let d = hour.date;
    b.push_str(&format!("{}, {} {}, {:04}\n", d.weekday().name(), MONTHS[d.month() as usize - 1], d.day(), d.year()));
    if !hour.feast.is_empty() {
        b.push_str(&hour.feast);
        b.push('\n');
    }
    b.push_str(&format!("Season: {} | Color: {}\n", hour.season.map_or("", |s| s.as_str()), hour.color.map_or("", |c| c.as_str())));
    b.push_str(&"=".repeat(60));
    b.push_str("\n\n");
    for section in &hour.sections {
        for elem in &section.elements {
            if !elem.label.is_empty() {
                // The Latin incipit rides on the label line.
                let mut label = elem.label.clone();
                if !elem.incipit.is_empty() {
                    label.push_str(" · ");
                    label.push_str(&elem.incipit);
                }
                b.push_str(&format!("--- {label} ---\n"));
            }
            let text = if elem.kind == ElementType::CorporateLordPrayer {
                corporate_lord_prayer_text(elem)
            } else if !elem.postures.is_empty() {
                posture_text(elem)
            } else {
                elem.display_text()
            };
            if !text.is_empty() {
                b.push_str(&text);
                b.push_str("\n\n");
            }
        }
    }
    b
}

/// A psalm, canticle or doxology with its posture cues bracketed: after the
/// " * " mediant, or before the verse's words, past its number.
fn posture_text(elem: &OfficeElement) -> String {
    let mut lines: Vec<String> = elem.text.split('\n').map(str::to_string).collect();
    let verses: Vec<usize> = if elem.kind.is_psalmody() {
        psalm_verse_lines(&elem.text)
    } else {
        (0..lines.len()).filter(|&i| !lines[i].trim().is_empty()).collect()
    };
    let bracketed = |cues: Vec<&str>| cues.iter().map(|c| format!("[{c}]")).collect::<Vec<_>>().join(" ");
    for (n, &i) in verses.iter().enumerate() {
        let after = bracketed(posture_cues_at(&elem.postures, PostureAnchor::AfterMediant(n)).collect());
        if !after.is_empty() {
            lines[i] = match lines[i].split_once(" * ") {
                Some((first, second)) => format!("{first} * {after} {second}"),
                None => format!("{} {after}", lines[i].trim_end()),
            };
        }
        let before = bracketed(posture_cues_at(&elem.postures, PostureAnchor::BeforeVerse(n)).collect());
        if !before.is_empty() {
            let line = &lines[i];
            let words = line.find(split_leading_verse_number(line.trim()).1).unwrap_or(0);
            lines[i] = format!("{}{before} {}", &line[..words], &line[words..]);
        }
    }
    lines.join("\n")
}

/// Adds only the response sigil the corporate form requires.
fn corporate_lord_prayer_text(elem: &OfficeElement) -> String {
    let (mut officiant, mut response) = (String::new(), String::new());
    for span in &elem.voice {
        match span.role {
            Some(VoiceRole::Officiant) => officiant.push_str(&span.text),
            Some(VoiceRole::Response) => response.push_str(&span.text),
            Some(VoiceRole::Priest | VoiceRole::All) | None => {}
        }
    }
    if officiant.is_empty() || response.is_empty() {
        return elem.text.clone();
    }
    format!("{}\nR. {}", officiant.trim_end_matches([' ', '\t', '\n']), response.trim())
}

#[cfg(test)]
mod tests {
    use liturgy::{Posture, PostureCue};

    use super::*;

    #[test]
    fn posture_cues_are_bracketed_in_place() {
        let mut psalm = OfficeElement::new(
            ElementType::Canticle,
            "Title\n\nO ALL ye Works * bless ye.\n2 Ananias * praise him.\n3 Let us bless the Father * praise him.\n4 No mediant",
        );
        psalm.postures = vec![
            PostureCue::new(Posture::Sit, PostureAnchor::AfterMediant(0)),
            PostureCue::new(Posture::Stand, PostureAnchor::AfterMediant(1)),
            PostureCue::new(Posture::Bow, PostureAnchor::BeforeVerse(2)),
            PostureCue::new(Posture::StandUpright, PostureAnchor::BeforeVerse(3)),
            PostureCue::new(Posture::Sit, PostureAnchor::AfterMediant(3)),
        ];
        assert_eq!(
            posture_text(&psalm),
            "Title\n\nO ALL ye Works * [Sit.] bless ye.\n2 Ananias * [Stand.] praise him.\n3 [Bow.] Let us bless the Father * praise him.\n4 [Stand upright.] No mediant [Sit.]"
        );
        let mut gloria = OfficeElement::new(ElementType::PsalmDoxology, "Glory be * Ghost;\nAs it was * Amen.");
        gloria.postures = vec![
            PostureCue::new(Posture::Bow, PostureAnchor::BeforeVerse(0)),
            PostureCue::new(Posture::StandUpright, PostureAnchor::BeforeVerse(1)),
        ];
        assert_eq!(posture_text(&gloria), "[Bow.] Glory be * Ghost;\n[Stand upright.] As it was * Amen.");
    }
}
