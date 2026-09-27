//! The prayer-form pass: marked ordinary slots take the private, deacon, or
//! priest form after composition. Ported from Go's `leader.go`.

use std::collections::HashSet;

use calendar::Decision;
use liturgy::{ElementType, OfficeElement, OfficeHour, PrayerForm, VoiceRole, VoiceSpan};

use crate::texts::OfficeTexts;

fn normalize_space(s: &str) -> String {
    s.split_whitespace().collect::<Vec<_>>().join(" ")
}

/// Resolves the explicitly marked leader slots; never touches other prayers.
pub fn apply_leader(hour: &mut OfficeHour, leader: PrayerForm, t: &OfficeTexts) -> Result<(), String> {
    hour.form = leader;
    let mut recorded = HashSet::new();
    let mut previous_prayer = String::new();
    for si in 0..hour.sections.len() {
        let elements = std::mem::take(&mut hour.sections[si].elements);
        let mut resolved = Vec::new();
        for elem in elements {
            if !elem.leader_slot.is_empty() && recorded.insert(elem.leader_slot.clone()) {
                let choir = leader == PrayerForm::Priest || (elem.leader_slot == "greeting" && leader == PrayerForm::Deacon);
                hour.decisions.push(Decision::new(
                    format!("prayer-form:{}", elem.leader_slot),
                    if choir { "choir" } else { "private" },
                    "",
                ));
            }
            let replacement: Option<Vec<OfficeElement>> = match elem.leader_slot.as_str() {
                "greeting" => {
                    let key = if leader == PrayerForm::Private { "greeting-lay" } else { "greeting-clergy" };
                    let greeting = leader_element(ElementType::Versicle, key, t)?;
                    // Omitted when the same pair immediately precedes it
                    // (tutorial p. 8, note 21).
                    if leader == PrayerForm::Private && previous_prayer.ends_with(&normalize_space(&greeting.text)) {
                        Some(Vec::new())
                    } else {
                        Some(vec![greeting])
                    }
                }
                "opening" if leader == PrayerForm::Priest => {
                    Some(vec![leader_element(ElementType::Versicle, "compline-opening-choir", t)?])
                }
                "confession" if leader == PrayerForm::Priest => Some(priest_confession(t)?),
                "confession" if leader == PrayerForm::Deacon => {
                    let mut common = elem.clone();
                    common.voice = vec![VoiceSpan::new(common.text.clone(), true, Some(VoiceRole::All))];
                    Some(vec![common])
                }
                _ => None,
            };
            let mut replacement = replacement.unwrap_or_else(|| vec![elem.clone()]);
            for r in &mut replacement {
                r.leader_slot = elem.leader_slot.clone();
                if r.kind != ElementType::Rubric && r.kind != ElementType::Heading && !r.text.is_empty() {
                    previous_prayer = normalize_space(&r.text);
                }
            }
            resolved.extend(replacement);
        }
        hour.sections[si].elements = resolved;
    }
    hour.decisions.push(Decision::new("context:prayer-form", leader.as_str(), ""));
    Ok(())
}

fn leader_element(kind: ElementType, slot: &str, t: &OfficeTexts) -> Result<OfficeElement, String> {
    let key = format!("shared/leader/{slot}");
    let text = t.get(&key);
    if text.is_empty() {
        return Err(format!("missing leader formula {key}"));
    }
    let mut e = OfficeElement::new(kind, text);
    e.source_ref = key.clone();
    e.source_refs = vec![key];
    Ok(e)
}

fn is_word_byte(b: u8) -> bool {
    b.is_ascii_alphanumeric() || b == b'_'
}

/// The byte ranges matching RE2 `\byou,\s+brethren\b`.
fn brethren_matches(text: &str) -> Vec<(usize, usize)> {
    let b = text.as_bytes();
    let mut out = Vec::new();
    let mut from = 0;
    while let Some(off) = text[from..].find("you,") {
        let start = from + off;
        from = start + 1;
        if start > 0 && is_word_byte(b[start - 1]) {
            continue;
        }
        let mut i = start + 4;
        let ws = i;
        while i < b.len() && matches!(b[i], b' ' | b'\t' | b'\n' | b'\r' | b'\x0c') {
            i += 1;
        }
        if i == ws || !text[i..].starts_with("brethren") {
            continue;
        }
        let end = i + "brethren".len();
        if end < b.len() && is_word_byte(b[end]) {
            continue;
        }
        out.push((start, end));
        from = end;
    }
    out
}

/// The full priestly exchange of the Confiteor (Diurnal pp. 7–8, 147–148).
fn priest_confession(t: &OfficeTexts) -> Result<Vec<OfficeElement>, String> {
    let parts = [
        (ElementType::Rubric, "confiteor-officiant-rubric"),
        (ElementType::Prayer, "confiteor-officiant"),
        (ElementType::Prayer, "confiteor-choir-response"),
        (ElementType::Rubric, "confiteor-choir-rubric"),
        (ElementType::Prayer, "confiteor-officiant"),
        (ElementType::Rubric, "confiteor-response-rubric"),
        (ElementType::Prayer, "confiteor-officiant-response"),
        (ElementType::Prayer, "confiteor-absolution-priest"),
    ];
    let mut elements = parts.iter().map(|(kind, slot)| leader_element(*kind, slot, t)).collect::<Result<Vec<_>, _>>()?;
    let matches = brethren_matches(&elements[4].text);
    if matches.len() != 2 {
        return Err("choir confession requires the two printed brethren addresses".to_string());
    }
    let choir = &mut elements[4];
    let mut text = String::new();
    let mut last = 0;
    for (s, e) in matches {
        text.push_str(&choir.text[last..s]);
        text.push_str("thee, father");
        last = e;
    }
    text.push_str(&choir.text[last..]);
    choir.text = text;
    choir.source_refs.push("shared/leader/confiteor-choir-rubric".to_string());
    let priest = Some(VoiceRole::Priest);
    let response = Some(VoiceRole::Response);
    elements[1].voice = vec![VoiceSpan::new(elements[1].text.clone(), true, priest)];
    elements[4].voice = vec![VoiceSpan::new(elements[4].text.clone(), true, response)];
    for (index, prayer, amen) in [(2, response, priest), (6, priest, response), (7, priest, response)] {
        let elem = &mut elements[index];
        const AMEN: &str = "\nR. Amen.";
        let seam = match elem.text.rfind(AMEN) {
            Some(s) if s + AMEN.len() == elem.text.len() => s,
            _ => return Err(format!("confession response requires a final Amen: {}", elem.source_ref)),
        };
        elem.voice = vec![VoiceSpan::new(&elem.text[..seam + 1], true, prayer), VoiceSpan::new(&elem.text[seam + 1..], true, amen)];
    }
    Ok(elements)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn brethren_regex() {
        assert_eq!(brethren_matches("pray for me, you, brethren, and you,\n brethren."), vec![(13, 26), (32, 46)]);
        assert!(brethren_matches("to you,brethren").is_empty());
        assert!(brethren_matches("yyou, brethren").is_empty());
        assert!(brethren_matches("you, brethrens").is_empty());
    }
}
