//! Spoken and silent spans of the secret and corporate prayers.

use liturgy::{VoiceRole, VoiceSpan};

fn incipit(reference: &str) -> Option<&'static str> {
    match reference {
        "ordinary/shared/our-father" => Some("Our Father"),
        "ordinary/shared/hail-mary" => Some("Hail, Mary"),
        "ordinary/shared/apostles-creed" => Some("I believe"),
        _ => None,
    }
}

const OUR_FATHER_ALOUD_SEAM: &str = "And lead us not into temptation";

fn aloud_seam(reference: &str) -> Option<&'static str> {
    match reference {
        "ordinary/shared/our-father" => Some(OUR_FATHER_ALOUD_SEAM),
        "ordinary/shared/apostles-creed" => Some("The Resurrection of the body"),
        _ => None,
    }
}

/// A secret prayer: aloud incipit, silent remainder. Partly secret: aloud
/// incipit, silent middle, aloud tail from the seam. Empty when the text does
/// not begin with the known incipit.
pub fn build_prayer_voice(reference: &str, text: &str, partly: bool) -> Vec<VoiceSpan> {
    let Some(incipit) = incipit(reference).filter(|i| text.starts_with(i)) else { return Vec::new() };
    let secret = || {
        let rest = &text[incipit.len()..];
        let mut spans = vec![VoiceSpan::new(incipit, true, None)];
        if !rest.is_empty() {
            spans.push(VoiceSpan::new(rest, false, None));
        }
        spans
    };
    if !partly {
        return secret();
    }
    let Some(seam) = aloud_seam(reference) else { return secret() };
    let seam_idx = match text.find(seam) {
        Some(i) if i >= incipit.len() => i,
        _ => return secret(),
    };
    let mut spans = vec![VoiceSpan::new(incipit, true, None)];
    let middle = &text[incipit.len()..seam_idx];
    if !middle.is_empty() {
        spans.push(VoiceSpan::new(middle, false, None));
    }
    let tail = &text[seam_idx..];
    if !tail.is_empty() {
        spans.push(VoiceSpan::new(tail, true, None));
    }
    spans
}

/// The corporate Lord's Prayer: the officiant through the seam, the people
/// from the final response.
pub fn build_corporate_lord_prayer_voice(reference: &str, text: &str) -> Vec<VoiceSpan> {
    if reference != "ordinary/shared/our-father" {
        return Vec::new();
    }
    let Some(seam) = text.find(OUR_FATHER_ALOUD_SEAM) else { return Vec::new() };
    let mut end = seam + OUR_FATHER_ALOUD_SEAM.len();
    while end < text.len() && matches!(text.as_bytes()[end], b',' | b'.' | b';' | b':') {
        end += 1;
    }
    let response = text[end..].trim_start_matches([' ', '\t', '\n']);
    if response.is_empty() {
        return Vec::new();
    }
    vec![
        VoiceSpan::new(&text[..text.len() - response.len()], true, Some(VoiceRole::Officiant)),
        VoiceSpan::new(response, true, Some(VoiceRole::Response)),
    ]
}
