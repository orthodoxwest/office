//! Collect conclusions (General Rubrics XII / XXXIII.4-5).

use crate::texts::OfficeTexts;

const CONCLUSION_FORM_REF: &str = "shared/formulas/collect-conclusion-";
const DEFAULT_CONCLUSION: &str = "through";

/// The conclusion formula appointed for the collect resolved from `source_ref`.
fn conclusion_ref_for(source_ref: &str, t: &OfficeTexts) -> Option<String> {
    let form = t.collect_conclusion_form(source_ref).unwrap_or(DEFAULT_CONCLUSION);
    let r = format!("{CONCLUSION_FORM_REF}{form}");
    (!t.get(&r).is_empty()).then_some(r)
}

/// Appends the appointed conclusion; returns the text and the refs used. A
/// missing slot ("[…]") or an omission passes through unconcluded.
pub fn apply_conclusion(text: &str, source_ref: &str, t: &OfficeTexts) -> (String, Vec<String>) {
    if text.is_empty() || text.starts_with('[') || corpus::is_omitted(text) {
        return (text.to_string(), vec![source_ref.to_string()]);
    }
    match conclusion_ref_for(source_ref, t) {
        None => (text.to_string(), vec![source_ref.to_string()]),
        Some(r) => (format!("{}\n{}", text.trim_end_matches('\n'), t.get(&r)), vec![source_ref.to_string(), r]),
    }
}
