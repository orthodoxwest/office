//! Validation of feast files and the text corpus.

use std::collections::HashMap;

use calendar::loader::{FEAST_FILES, parse_ini_sections, section_to_feast};
use calendar::{DataSource, Feast, Rank};
use data_format::{atoi, quote, scan_lines};

/// Validates the feast files in three layers: syntax, schema, semantics.
pub fn validate_calendar(src: &dyn DataSource) -> Vec<String> {
    let mut errors = Vec::new();
    let mut all = Vec::new();
    for fname in FEAST_FILES {
        let rel = format!("feasts/{fname}");
        let path = src.display_path(&rel);
        let content = match src.read(&rel) {
            Ok(Some(c)) => c,
            Ok(None) => {
                errors.push(format!("Missing data file: {path}"));
                continue;
            }
            Err(e) => {
                errors.push(format!("Syntax error in {fname}: {e}"));
                continue;
            }
        };
        let sections = match parse_ini_sections(&path, &content) {
            Ok(s) => s,
            Err(e) => {
                errors.push(format!("Syntax error in {fname}: {e}"));
                continue;
            }
        };
        let mut schema_errors = Vec::new();
        let mut feasts = Vec::new();
        for section in &sections {
            match section_to_feast(section, fname) {
                Ok(f) => feasts.push(f),
                Err(e) => schema_errors.push(e),
            }
        }
        if schema_errors.is_empty() {
            all.extend(feasts);
        }
        errors.extend(schema_errors);
    }
    if !all.is_empty() {
        errors.extend(validate_semantics(&all));
    }
    for f in &all {
        if let Some(name) = title_as_proper_name(src, f) {
            errors.push(format!(
                "Feast '{}' would fill \"N.\" in its Common's collect with its title {}; give it a ProperName or a proper collect (#607)",
                f.id,
                quote(&name)
            ));
        }
    }
    errors
}

/// Cross-entry checks over parsed feasts.
fn validate_semantics(feasts: &[Feast]) -> Vec<String> {
    let mut errs = Vec::new();
    let mut seen: HashMap<&str, usize> = HashMap::new();
    let mut by_id: HashMap<&str, usize> = HashMap::new();
    for (i, f) in feasts.iter().enumerate() {
        let n = seen.entry(&f.id).or_default();
        *n += 1;
        by_id.entry(&f.id).or_insert(i);
        if *n > 1 {
            errs.push(format!("Duplicate feast ID: '{}'", f.id));
        }
    }
    let exists = |id: &str| seen.get(id).is_some_and(|n| *n > 0);

    for f in feasts {
        if let Some(target) = &f.only_with
            && !exists(target)
        {
            errs.push(format!("Feast '{}' has OnlyWith target '{target}' which does not exist", f.id));
        }
    }
    for (i, f) in feasts.iter().enumerate() {
        let Some(target) = &f.companion_of else { continue };
        let valid = by_id.get(target.as_str()).is_some_and(|&p| p != i && feasts[p].companion_of.is_none());
        if !valid {
            errs.push(format!("Feast '{}' has invalid CompanionOf target '{target}'", f.id));
        }
    }
    let mut explicit_vigil_by_owner: HashMap<&str, &str> = HashMap::new();
    for (i, f) in feasts.iter().enumerate() {
        if !f.is_vigil {
            continue;
        }
        let target = f.vigil_of.as_deref().unwrap_or("");
        if !exists(target) {
            errs.push(format!("Vigil '{}' has VigilOf target '{target}' which does not exist", f.id));
            continue;
        }
        let owner_idx = by_id[target];
        let owner = &feasts[owner_idx];
        if owner_idx == i || owner.is_vigil {
            errs.push(format!("Vigil '{}' has invalid vigil target '{target}'", f.id));
            continue;
        }
        if owner.has_vigil {
            errs.push(format!("Feast '{}' generates a vigil but explicit vigil '{}' defines the same observance", owner.id, f.id));
        }
        match explicit_vigil_by_owner.get(target) {
            Some(prior) => errs.push(format!("Explicit vigils '{prior}' and '{}' define the same observance for '{target}'", f.id)),
            None => {
                explicit_vigil_by_owner.insert(target, &f.id);
            }
        }
    }

    // Conflicting fixed dates at the same rank, in first-appearance order.
    type DateRank = (u32, u32, Rank);
    let mut fixed: Vec<(DateRank, Vec<&str>)> = Vec::new();
    for f in feasts {
        let Some(md) = f.fixed else { continue };
        let key = (md.month, md.day, f.rank);
        match fixed.iter_mut().find(|(k, _)| *k == key) {
            Some((_, ids)) => ids.push(&f.id),
            None => fixed.push((key, vec![&f.id])),
        }
    }
    for ((month, day, rank), ids) in &fixed {
        if *rank != Rank::Commemoration && ids.len() > 1 {
            errs.push(format!("Multiple feasts on {month}/{day} at rank {rank}: {}", ids.join(", ")));
        }
    }

    for f in feasts {
        if f.has_octave && !matches!(f.rank, Rank::Double1stClass | Rank::Double2ndClass) {
            errs.push(format!("Feast '{}' has an octave but is only ranked {}", f.id, f.rank));
        }
        if f.companion_of.is_some() && f.rank != Rank::Commemoration {
            errs.push(format!("Feast '{}' is an apostolic companion but is ranked {} instead of commemoration", f.id, f.rank));
        }
    }
    for f in feasts {
        if let Some(rule) = &f.date_rule
            && !is_valid_date_rule(rule)
        {
            errs.push(format!(
                "Feast '{}' has unrecognized DateRule: {} (expected easter±N, epiphany-sunday-N, advent-sunday-N, pentecost-sunday-N, holy-name, or last-sunday-october)",
                f.id,
                quote(rule)
            ));
        }
    }
    errs
}

/// The name a saint's Common would put for "N." when it is a title rather than
/// the saint's name: "thy holy Martyrs The Forty Holy Martyrs" (#607). Feasts
/// with their own collect never reach the Common's "N.".
fn title_as_proper_name(src: &dyn DataSource, f: &Feast) -> Option<String> {
    use calendar::Category::{Angel, BlessedVirgin, Dedication, Feria, Lord};
    if f.proper_name.is_some() || f.category.is_none_or(|c| matches!(c, Lord | BlessedVirgin | Angel | Dedication | Feria)) {
        return None;
    }
    let name = office::proper::derive_proper_name_from_title(&f.name);
    let title = name.starts_with("The ") || name.split_whitespace().any(|w| matches!(w, "Martyr" | "Martyrs" | "Octave"));
    (title && !has_proper_collect(src, f)).then_some(name)
}

fn has_proper_collect(src: &dyn DataSource, f: &Feast) -> bool {
    f.proper_id.iter().chain([&f.id]).any(|id| {
        let Ok(Some(text)) = src.read(&format!("texts/proper/{id}.txt")) else { return false };
        text.lines().any(|l| matches!(l.trim(), "[collect]" | "[commemoration-collect]"))
    })
}

/// The DateRule patterns `resolve_feast_date` understands.
fn is_valid_date_rule(rule: &str) -> bool {
    if let Some(rest) = rule.strip_prefix("easter")
        && let Some(digits) = rest.strip_prefix(['+', '-'])
        && !digits.is_empty()
        && digits.bytes().all(|b| b.is_ascii_digit())
    {
        return true;
    }
    let number = |suffix: &str| atoi(suffix).ok();
    if let Some(s) = rule.strip_prefix("epiphany-sunday-") {
        return number(s).is_some_and(|n| n >= 1);
    }
    if let Some(s) = rule.strip_prefix("advent-sunday-") {
        return number(s).is_some_and(|n| (1..=4).contains(&n));
    }
    if let Some(s) = rule.strip_prefix("pentecost-sunday-") {
        return number(s).is_some_and(|n| n >= 1);
    }
    rule == "holy-name" || rule == "last-sunday-october"
}

/// Validates `data/texts/`: leftover import directives, near-miss psalmody
/// section names, control characters, then a full load, placeholders, and
/// missing incipits. The report is sorted.
pub fn validate_texts(src: &dyn DataSource) -> Vec<String> {
    let files = match src.walk("texts") {
        Ok(files) => files,
        Err(e) => return vec![format!("validating texts: {e}")],
    };
    let mut errs = Vec::new();
    for (rel, bytes) in &files {
        if !rel.rsplit('/').next().is_some_and(|n| n.ends_with(".txt")) {
            continue;
        }
        errs.extend(validate_file(&format!("texts/{rel}"), bytes));
    }
    let texts = match office::texts::load_texts(src) {
        Ok(t) => t,
        Err(e) => return vec![format!("loading text corpus: {e}")],
    };
    for key in texts.corpus.find_placeholders() {
        errs.push(format!("placeholder text remains in corpus: {key}"));
    }
    for key in texts.corpus.missing_incipits() {
        errs.push(format!("no Latin incipit in data/latin-incipits.txt for: {key}"));
    }
    errs.sort();
    errs
}

fn validate_file(rel_path: &str, bytes: &[u8]) -> Vec<String> {
    let text = String::from_utf8_lossy(bytes);
    let has_sections = corpus::has_ini_sections(&text);
    let mut errs = Vec::new();
    let mut current_section = "";
    for (i, line) in scan_lines(&text).enumerate() {
        let line_no = i + 1;
        let trimmed = line.trim();
        if has_sections
            && let Some(inner) = trimmed.strip_prefix('[').and_then(|t| t.strip_suffix(']'))
            && !inner.is_empty()
            && !inner.contains([' ', ':', '\t'])
        {
            if let Some(suggestion) = near_psalmody_section_name(rel_path, inner) {
                errs.push(format!(
                    "{rel_path}:{line_no}: unrecognized section name {} (did you mean {}?)",
                    quote(inner),
                    quote(suggestion)
                ));
            }
            current_section = inner;
            continue;
        }
        if let Some(reason) = unexpected_directive_reason(trimmed) {
            let mut location = format!("{rel_path}:{line_no}");
            if !current_section.is_empty() {
                location.push_str(&format!(" [{current_section}]"));
            }
            errs.push(format!("{location}: {reason}: {}", quote(trimmed)));
        }
    }
    errs.extend(validate_control_characters(rel_path, bytes));
    errs
}

const RESERVED_PSALMODY: [&str; 3] = ["lauds-psalmody", "vespers-psalmody", "vespers-psalmody-first"];

fn near_psalmody_section_name(rel_path: &str, section: &str) -> Option<&'static str> {
    if !rel_path.starts_with("texts/proper/") && !rel_path.starts_with("texts/commons/") {
        return None;
    }
    if RESERVED_PSALMODY.contains(&section) {
        return None;
    }
    let mut best = None;
    let mut best_distance = 3;
    for name in RESERVED_PSALMODY {
        let d = edit_distance(section.as_bytes(), name.as_bytes());
        if d < best_distance {
            best = Some(name);
            best_distance = d;
        }
    }
    best.filter(|_| best_distance <= 2)
}

fn edit_distance(a: &[u8], b: &[u8]) -> usize {
    let mut previous: Vec<usize> = (0..=b.len()).collect();
    for i in 1..=a.len() {
        let mut current = vec![i; b.len() + 1];
        for j in 1..=b.len() {
            let cost = usize::from(a[i - 1] != b[j - 1]);
            current[j] = (current[j - 1] + 1).min(previous[j] + 1).min(previous[j - 1] + cost);
        }
        previous = current;
    }
    previous[b.len()]
}

fn unexpected_directive_reason(trimmed: &str) -> Option<&'static str> {
    if trimmed.is_empty() || trimmed.starts_with('#') {
        return None;
    }
    let fields: Vec<&str> = trimmed.split_whitespace().collect();
    if fields.len() == 2 && fields[0] == "@use" {
        return None;
    }
    if trimmed == corpus::OMIT_MARKER {
        return None;
    }
    if trimmed.starts_with('@') {
        return Some("unexpected Divinum Officium directive");
    }
    if trimmed.starts_with("ex ") {
        return Some("unexpected Divinum Officium extract directive");
    }
    None
}

fn validate_control_characters(rel_path: &str, content: &[u8]) -> Vec<String> {
    let mut errs = Vec::new();
    let (mut line, mut column) = (1, 1);
    for &b in content {
        if b < 32 && b != b'\n' && b != b'\r' && b != b'\t' {
            errs.push(format!("{rel_path}:{line}:{column}: unexpected control character U+{b:04X}"));
        }
        match b {
            b'\n' => {
                line += 1;
                column = 1;
            }
            b'\r' => column = 1,
            _ => column += 1,
        }
    }
    if content.starts_with(&[0xEF, 0xBB, 0xBF]) {
        errs.push(format!("{rel_path}:1:1: unexpected UTF-8 BOM"));
    }
    errs
}
