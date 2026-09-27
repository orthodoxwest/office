//! The ordo-relevant digest of a composed hour: preces, suffrage,
//! commemorations, and the gospel-canticle antiphon. Shared by the ordo, the
//! calendar view, and `office rubrics`. Ported from Go's `summary.go`.

use calendar::Color;
use liturgy::{ElementType, OfficeHour};

/// One commemoration: its name and its antiphon's incipit.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct CommSummary {
    pub name: String,
    pub incipit: String,
}

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct HourSummary {
    pub color: Option<Color>,
    /// The Benedictus/Magnificat antiphon as an incipit.
    pub gospel_ant: String,
    /// The whole antiphon on one line.
    pub gospel_ant_full: String,
    pub preces: bool,
    pub suffrage: bool,
    pub comms: Vec<CommSummary>,
}

pub fn summarize_hour(hour: &OfficeHour) -> HourSummary {
    let mut s = HourSummary {
        color: hour.color,
        gospel_ant: String::new(),
        gospel_ant_full: String::new(),
        preces: false,
        suffrage: false,
        comms: Vec::new(),
    };
    for sec in &hour.sections {
        if sec.label.contains("Suffrage") {
            s.suffrage = true;
        }
        for (i, el) in sec.elements.iter().enumerate() {
            if el.kind == ElementType::Preces {
                s.preces = true;
            } else if el.kind == ElementType::Heading
                && let Some(name) = el.text.strip_prefix("Commemoration of ")
            {
                let mut c = CommSummary { name: name.to_string(), incipit: String::new() };
                for next in &sec.elements[i + 1..] {
                    if next.kind == ElementType::Antiphon {
                        c.incipit = incipit(&next.text);
                        break;
                    }
                    if next.kind == ElementType::Heading {
                        break;
                    }
                }
                s.comms.push(c);
            } else if s.gospel_ant.is_empty() && (el.slot_ref == "benedictus-antiphon" || el.slot_ref == "magnificat-antiphon") {
                s.gospel_ant = incipit(&el.text);
                s.gospel_ant_full = el.text.replace('\n', " ");
            } else if s.gospel_ant.is_empty()
                && el.kind == ElementType::Canticle
                && (el.source_ref == "canticles/benedictus" || el.source_ref == "canticles/magnificat")
                && i > 0
                && sec.elements[i - 1].kind == ElementType::Antiphon
            {
                let antiphon = &sec.elements[i - 1].text;
                s.gospel_ant = incipit(antiphon);
                s.gospel_ant_full = antiphon.replace('\n', " ");
            }
        }
    }
    s
}

/// An antiphon's opening: through the mediant, at most nine words, without
/// trailing punctuation.
pub fn incipit(text: &str) -> String {
    let s = text.replace('\n', " ");
    let mut s = s.trim();
    if s.is_empty() {
        return String::new();
    }
    if let Some(i) = s.find('*')
        && i > 0
    {
        s = s[..i].trim();
    }
    let mut words: Vec<&str> = s.split_whitespace().collect();
    let truncated = words.len() > 9;
    words.truncate(9);
    let mut out = words.join(" ").trim_end_matches([' ', ',', ';', ':', '.']).to_string();
    if truncated {
        out.push('…');
    }
    out
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn incipits() {
        assert_eq!(incipit("O Wisdom, * which camest out."), "O Wisdom");
        assert_eq!(incipit("one two three four five six seven eight nine ten."), "one two three four five six seven eight nine…");
        assert_eq!(incipit("Great."), "Great");
        assert_eq!(incipit("  "), "");
    }
}
