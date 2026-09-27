//! Plain-text rendering of a composed hour, for the CLI and the text goldens.

use liturgy::{ElementType, OfficeElement, OfficeHour, VoiceRole};

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
            let text = if elem.kind == ElementType::CorporateLordPrayer { corporate_lord_prayer_text(elem) } else { elem.display_text() };
            if !text.is_empty() {
                b.push_str(&text);
                b.push_str("\n\n");
            }
        }
    }
    b
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
