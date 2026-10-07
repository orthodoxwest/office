//! One hour page for every prayer form: sections whose elements agree are shared, and each explicit
//! leader slot carries its alternatives, tagged with the forms they apply to.

use liturgy::{OfficeElement, OfficeHour, PrayerForm};

use crate::escape::html_escape_string;
use crate::html::render_section_elements;

/// A section rendered for every prayer form at once.
#[derive(Clone, Debug, PartialEq, Eq, serde::Serialize)]
pub struct LeaderSection {
    pub label: String,
    pub collapsible: bool,
    /// Trusted markup.
    pub html: String,
    /// The section as read with the Martyrology at Prime, when that differs; empty otherwise.
    /// Trusted markup.
    pub martyrology_html: String,
}

/// Aligns the private, deacon, and priest compositions of one hour. A
/// structural mismatch fails rather than attaching an alternative to the
/// wrong prayer.
pub fn leader_sections(forms: &[(PrayerForm, &OfficeHour)]) -> Result<Vec<LeaderSection>, String> {
    if forms.len() != PrayerForm::ALL.len() {
        return Err("expected three leader forms".into());
    }
    // The priest form keeps every greeting, so it is the alignment template.
    const BASE: usize = 2;
    let base = forms[BASE].1;
    for (i, (form, hour)) in forms.iter().enumerate() {
        if *form != PrayerForm::ALL[i] || hour.sections.len() != base.sections.len() {
            return Err("inconsistent leader forms".into());
        }
    }
    let mut sections = Vec::new();
    for (si, section) in base.sections.iter().enumerate() {
        let mut positions = vec![0usize; forms.len()];
        let mut html = String::new();
        while positions[BASE] < section.elements.len() {
            let base_start = positions[BASE];
            let first = &section.elements[base_start];
            let mut base_end = base_start + 1;
            while base_end < section.elements.len() && section.elements[base_end].leader_slot == first.leader_slot {
                base_end += 1;
            }
            let mut groups: Vec<&[OfficeElement]> = Vec::new();
            for (fi, (_, hour)) in forms.iter().enumerate() {
                let s = &hour.sections[si];
                if s.label != section.label || s.collapsible != section.collapsible {
                    return Err("leader section mismatch".into());
                }
                let start = positions[fi];
                let slot_here = start < s.elements.len() && s.elements[start].leader_slot == first.leader_slot;
                if fi == 0 && first.leader_slot == "greeting" && !slot_here {
                    groups.push(&[]);
                    continue;
                }
                if !slot_here {
                    return Err("leader slot mismatch".into());
                }
                let mut end = start + 1;
                if first.leader_slot.is_empty() {
                    // Common runs can join across a missing private greeting:
                    // take only the template's run and compare it below.
                    end = start + base_end - base_start;
                    if end > s.elements.len() {
                        return Err("incomplete common leader sequence".into());
                    }
                } else {
                    while end < s.elements.len() && s.elements[end].leader_slot == first.leader_slot {
                        end += 1;
                    }
                }
                groups.push(&s.elements[start..end]);
                positions[fi] = end;
            }
            if first.leader_slot.is_empty() {
                if groups[1..].iter().any(|g| *g != groups[0]) {
                    return Err("unmarked leader variation".into());
                }
                html.push_str(&render_section_elements(groups[0]));
                continue;
            }
            let available: Vec<&str> = forms.iter().zip(&groups).filter(|(_, g)| !g.is_empty()).map(|((f, _), _)| f.as_str()).collect();
            html.push_str(&format!("<div class=\"leader-slot\" data-leader-slot=\"{}\"", html_escape_string(&first.leader_slot)));
            if available.len() < forms.len() {
                // Hide the whole grid item so an omitted greeting leaves no gap.
                html.push_str(&format!(" data-leaders=\"{}\"", available.join(" ")));
            }
            html.push('>');
            // Identical alternatives render once, tagged with every form.
            let mut used = vec![false; forms.len()];
            for (fi, group) in groups.iter().enumerate() {
                if used[fi] || group.is_empty() {
                    continue;
                }
                let mut leaders = vec![forms[fi].0.as_str()];
                for next in fi + 1..forms.len() {
                    if groups[next] == *group {
                        leaders.push(forms[next].0.as_str());
                        used[next] = true;
                    }
                }
                html.push_str(&format!("<div data-leaders=\"{}\">{}</div>", leaders.join(" "), render_section_elements(group)));
            }
            html.push_str("</div>");
        }
        for (fi, (_, hour)) in forms.iter().enumerate() {
            if positions[fi] != hour.sections[si].elements.len() {
                return Err("unmatched leader elements".into());
            }
        }
        sections.push(LeaderSection {
            label: section.label.clone(),
            collapsible: section.collapsible,
            html,
            martyrology_html: String::new(),
        });
    }
    Ok(sections)
}

#[cfg(test)]
mod tests {
    use liturgy::{ElementType, OfficeElement, OfficeHour, OfficeSection, PrayerForm};

    use super::leader_sections;

    fn el(kind: ElementType, text: &str, slot: &str) -> OfficeElement {
        OfficeElement { leader_slot: slot.into(), ..OfficeElement::new(kind, text) }
    }

    fn hour(form: PrayerForm, elements: Vec<OfficeElement>) -> OfficeHour {
        OfficeHour {
            form,
            date: calendar::Date::new(2026, 1, 1),
            hour: String::new(),
            title: String::new(),
            season: None,
            feast: String::new(),
            rank: None,
            color: None,
            sections: vec![OfficeSection { label: String::new(), collapsible: false, elements }],
            decisions: Vec::new(),
        }
    }

    fn sections(hours: &[OfficeHour]) -> Result<Vec<super::LeaderSection>, String> {
        let forms: Vec<(PrayerForm, &OfficeHour)> = hours.iter().map(|h| (h.form, h)).collect();
        leader_sections(&forms)
    }

    #[test]
    fn share_common_text_and_expand_bounded_slots() {
        let mut hours: Vec<OfficeHour> = PrayerForm::ALL
            .iter()
            .map(|&form| {
                let mut elements = vec![el(ElementType::Prayer, "Common before", "")];
                if form == PrayerForm::Private {
                    elements.push(el(ElementType::Prayer, "Private confession", "confession"));
                } else {
                    elements.push(el(ElementType::Rubric, "Choir rubric", "confession"));
                    elements.push(el(ElementType::Prayer, "Choir confession", "confession"));
                }
                elements.push(el(ElementType::Prayer, "Common after", ""));
                hour(form, elements)
            })
            .collect();
        let html = &sections(&hours).unwrap()[0].html;
        for text in ["Common before", "Common after", "Private confession", "Choir confession", "Choir rubric"] {
            assert_eq!(html.matches(text).count(), 1, "{text:?} not emitted exactly once: {html}");
        }
        assert!(html.contains(r#"data-leaders="deacon priest""#), "identical choir forms not shared: {html}");
        hours[2].sections[0].elements[0].text = "unexpected variant".into();
        assert!(sections(&hours).is_err(), "unmarked variation accepted");
    }

    #[test]
    fn align_omitted_private_greeting() {
        let mut hours: Vec<OfficeHour> = PrayerForm::ALL
            .iter()
            .map(|&form| {
                let mut elements = vec![el(ElementType::Prayer, "Fixed preces response", "")];
                if form != PrayerForm::Private {
                    elements.push(el(ElementType::Versicle, "Clergy greeting", "greeting"));
                }
                elements.push(el(ElementType::Collect, "The collect", ""));
                elements.push(el(ElementType::Versicle, "Closing greeting", "greeting"));
                hour(form, elements)
            })
            .collect();
        let html = &sections(&hours).unwrap()[0].html;
        for text in ["Fixed preces response", "The collect", "Clergy greeting", "Closing greeting"] {
            assert_eq!(html.matches(text).count(), 1, "lost or duplicated {text:?}: {html}");
        }
        assert!(
            html.contains(
                r#"<div class="leader-slot" data-leader-slot="greeting" data-leaders="deacon priest"><div data-leaders="deacon priest">"#
            ),
            "missing private alternative not aligned: {html}"
        );
        // Omitting an unmarked prayer must still fail rather than pass as a
        // legitimate form variation.
        hours[0].sections[0].elements.remove(0);
        assert!(sections(&hours).is_err(), "accepted missing common prayer");
    }
}
