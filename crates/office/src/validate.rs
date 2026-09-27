//! Hour-definition validation: every `Ref` in `data/office/` resolves, and the psalmody
//! declarations and omission markers are well formed.

use std::collections::HashMap;

use calendar::DataSource;
use corpus::{Corpus, OMIT_MARKER, is_omitted};
use data_format::quote;

use crate::engine::{HOLY_SATURDAY_VESPERS_DEFINITION, HOUR_NAMES, hour_definition_names};
use crate::hourdef::{Condition, parse_hour_definition};
use crate::lauds_psalmody::{LAUDS_LAUDATE_PSALMODY_REF, LAUDS_PSALMODY_REF, valid_hour_psalmody_ref};
use crate::proper::ref_candidates;
use crate::psalmody::{
    DEFAULT_VESPERS_PSALMODY_KEY, DOXOLOGY_GLORIA_PATRI, DOXOLOGY_REF_PER_OFFICE, DOXOLOGY_REST_ETERNAL, VESPERS_PSALMODY_REF,
    parse_psalmody_declaration,
};
use crate::texts::load_texts;

/// The `Type` values the engine recognises; others silently become rubrics.
const VALID_ELEMENT_TYPES: [&str; 32] = [
    "officiant-greeting",
    "officiant-confession",
    "officiant-opening",
    "psalm",
    "canticle",
    "hymn",
    "antiphon",
    "versicle",
    "response",
    "prayer",
    "secret-prayer",
    "silent-prayer",
    "partly-secret-prayer",
    "corporate-lord-prayer",
    "dialogue",
    "preces",
    "rubric",
    "chapter",
    "collect",
    "blessing",
    "marian",
    "proper-antiphon",
    "proper-opening-acclamation",
    "proper-collect",
    "proper-hymn",
    "proper-responsory",
    "proper-short-responsory",
    "proper-versicle",
    "proper-chapter",
    "proper-psalmody",
    "commemorations",
    "gloria-patri",
];

/// Every key a `marian seasonal` element may resolve to.
const MARIAN_CORPUS_KEYS: [&str; 5] = [
    "ordinary/marian/alma-redemptoris-advent",
    "ordinary/marian/alma-redemptoris-christmas",
    "ordinary/marian/ave-regina-caelorum",
    "ordinary/marian/regina-caeli",
    "ordinary/marian/salve-regina",
];

const WEEKDAYS: [&str; 7] = ["sunday", "monday", "tuesday", "wednesday", "thursday", "friday", "saturday"];

fn validation_hours<'a>(hour: &'a str, kind: &str, reference: &str) -> &'a str {
    if kind == "proper-collect" && reference == "collect" && matches!(hour, "terce" | "sext" | "none") {
        return "lauds";
    }
    hour
}

/// Returns definition parse errors, then psalmody declaration and omission errors, then unresolved
/// refs (sorted).
pub fn validate_hour_definitions(src: &dyn DataSource) -> Vec<String> {
    let texts = match load_texts(src) {
        Ok(t) => t,
        Err(e) => return vec![format!("loading text corpus: {e}")],
    };
    let corpus: &Corpus = &texts;
    let mut declaration_errors = validate_vespers_psalmody_declarations(corpus);
    declaration_errors.extend(validate_lauds_psalmody_declarations(corpus));

    let mut parse_errors = Vec::new();
    // Corpus key → the first definition section that requires it.
    let mut required: HashMap<String, String> = HashMap::new();
    let mut require = |key: &str, src: &str| {
        required.entry(key.to_string()).or_insert_with(|| src.to_string());
    };

    for definition in hour_definition_names() {
        let hour = if definition == HOLY_SATURDAY_VESPERS_DEFINITION { "vespers" } else { definition };
        let rel = format!("office/{definition}.txt");
        let path = src.display_path(&rel);
        let sections = match src.read(&rel) {
            Err(e) => Err(format!("opening hour definition: {e}")),
            Ok(None) => Err(format!("opening hour definition: {path} does not exist")),
            Ok(Some(content)) => parse_hour_definition(&path, &content),
        };
        let sections = match sections {
            Ok(s) => s,
            Err(e) => {
                parse_errors.push(format!("office/{definition}.txt: {e}"));
                continue;
            }
        };

        for section in &sections {
            let src = format!("office/{definition}.txt [{}]", section.name);
            if !section.condition.is_empty() && Condition::parse(&section.condition).is_err() {
                parse_errors.push(format!("{src}: unknown Condition {}", quote(&section.condition)));
            }
            for elem in &section.elements {
                let (kind, reference) = (elem.kind.as_str(), elem.reference.as_str());
                if !VALID_ELEMENT_TYPES.contains(&kind) {
                    parse_errors.push(format!("{src}: unknown element Type {}", quote(kind)));
                }
                match kind {
                    "proper-psalmody" => {
                        if !valid_hour_psalmody_ref(hour, reference) {
                            parse_errors.push(format!("{src}: unsupported {hour} proper-psalmody Ref {}", quote(reference)));
                        }
                    }
                    "proper-antiphon"
                    | "proper-opening-acclamation"
                    | "proper-collect"
                    | "proper-hymn"
                    | "proper-responsory"
                    | "proper-short-responsory"
                    | "proper-versicle"
                    | "proper-chapter" => {
                        // The ordinary fallbacks the engine tries: hour-specific,
                        // per-weekday, then shared.
                        let candidates = ref_candidates(reference);
                        let mut found_ordinary = false;
                        let validation_hour = validation_hours(hour, kind, reference);
                        for cand in &candidates {
                            let hour_ref = format!("ordinary/{validation_hour}/{cand}");
                            if corpus.has(&hour_ref) {
                                found_ordinary = true;
                                require(&hour_ref, &src);
                            }
                        }
                        for cand in &candidates {
                            for wd in WEEKDAYS {
                                let wd_ref = format!("ordinary/{validation_hour}/{cand}-{wd}");
                                if corpus.has(&wd_ref) {
                                    found_ordinary = true;
                                    require(&wd_ref, &src);
                                }
                            }
                        }
                        for cand in &candidates {
                            let shared_ref = format!("ordinary/shared/{cand}");
                            if corpus.has(&shared_ref) {
                                found_ordinary = true;
                                require(&shared_ref, &src);
                            }
                        }
                        // Otherwise a feast or seasonal text resolves it at runtime.
                        if !found_ordinary && !candidates.iter().any(|c| corpus.has_key_suffix(c)) {
                            parse_errors.push(format!(
                                "{src}: proper ref {} not found in corpus (checked ordinary, shared, weekday, and feast/seasonal paths)",
                                quote(reference)
                            ));
                        }
                    }
                    "marian" => {
                        if reference != "seasonal" {
                            parse_errors
                                .push(format!("{src}: marian element has unsupported ref {} (expected \"seasonal\")", quote(reference)));
                            continue;
                        }
                        for key in MARIAN_CORPUS_KEYS {
                            require(key, &src);
                        }
                    }
                    "gloria-patri" => {
                        if reference == DOXOLOGY_REF_PER_OFFICE {
                            require(DOXOLOGY_GLORIA_PATRI, &src);
                            require(DOXOLOGY_REST_ETERNAL, &src);
                        } else {
                            require(reference, &src);
                        }
                    }
                    // Resolved at runtime; nothing to validate.
                    "commemorations" => {}
                    _ => require(reference, &src),
                }
            }
        }
    }

    let mut ref_errors: Vec<String> =
        required.iter().filter(|(key, _)| !corpus.has(key)).map(|(key, src)| format!("{src}: ref not found in corpus: {key}")).collect();
    ref_errors.sort();

    parse_errors.extend(declaration_errors);
    parse_errors.extend(validate_omissions(corpus));
    parse_errors.extend(ref_errors);
    parse_errors
}

fn validate_vespers_psalmody_declarations(corpus: &Corpus) -> Vec<String> {
    let mut errs = Vec::new();
    for key in corpus.references() {
        if key != DEFAULT_VESPERS_PSALMODY_KEY
            && !key.starts_with("psalmody/vespers/")
            && !key.ends_with(&format!("/{VESPERS_PSALMODY_REF}"))
            && !key.ends_with(&format!("/{VESPERS_PSALMODY_REF}-first"))
        {
            continue;
        }
        let (items, ferial) = match parse_psalmody_declaration(corpus.get(key)) {
            Ok(parsed) => parsed,
            Err(e) => {
                errs.push(format!("{key}: invalid Vespers psalmody declaration: {e}"));
                continue;
            }
        };
        if ferial {
            continue;
        }
        for item in &items {
            if !corpus.has(&item.psalm) {
                errs.push(format!("{key}: psalm ref not found in corpus: {}", item.psalm));
            }
            if !corpus.has_key_suffix(&item.antiphon) {
                errs.push(format!("{key}: antiphon ref not found in corpus: {}", item.antiphon));
            }
        }
    }
    if !corpus.has(DEFAULT_VESPERS_PSALMODY_KEY) {
        errs.push(format!("default Vespers psalmody declaration not found: {DEFAULT_VESPERS_PSALMODY_KEY}"));
    }
    errs.sort();
    errs
}

fn validate_lauds_psalmody_declarations(corpus: &Corpus) -> Vec<String> {
    let mut errs = Vec::new();
    let psalmody_suffix = format!("/{LAUDS_PSALMODY_REF}");
    let laudate_suffix = format!("/{LAUDS_LAUDATE_PSALMODY_REF}");
    for key in corpus.references() {
        if !key.ends_with(&psalmody_suffix) && !key.ends_with(&laudate_suffix) && !key.starts_with("psalmody/lauds/") {
            continue;
        }
        let body = corpus.get(key);
        if body == "festal" && !key.ends_with(&laudate_suffix) {
            continue;
        }
        let items = match parse_psalmody_declaration(body) {
            Ok((items, false)) => items,
            Ok((_, true)) | Err(_) => {
                errs.push(format!("{key}: invalid Lauds psalmody declaration (expected festal or antiphon/psalm rows)"));
                continue;
            }
        };
        for item in &items {
            if (!item.psalm.starts_with("psalms/") && !item.psalm.starts_with("canticles/")) || !corpus.has(&item.psalm) {
                errs.push(format!("{key}: psalm or canticle ref not found: {}", item.psalm));
            }
            if !corpus.has_key_suffix(&item.antiphon) {
                errs.push(format!("{key}: antiphon ref not found: {}", item.antiphon));
            }
        }
        if let Some(stem) = key.strip_suffix(LAUDS_PSALMODY_REF)
            && key.ends_with(&psalmody_suffix)
            && !corpus.has(&format!("{stem}{LAUDS_LAUDATE_PSALMODY_REF}"))
        {
            errs.push(format!("{key}: declared Lauds psalmody requires a Laudate declaration"));
        }
    }
    errs
}

/// Where the omission marker may appear: never on a psalm antiphon, and only
/// on a slot that names its hour, so an omission stays inside the rubric
/// that licenses it.
fn validate_omissions(corpus: &Corpus) -> Vec<String> {
    let mut errs = Vec::new();
    for key in corpus.references() {
        if !is_omitted(corpus.get(key)) {
            continue;
        }
        let slot = key.rsplit('/').next().unwrap_or(key);
        if slot.starts_with("psalm-antiphon") {
            errs.push(format!("{key}: {OMIT_MARKER} is not permitted on a psalm antiphon; every psalm is sung under one"));
        } else if !HOUR_NAMES.iter().any(|h| slot.ends_with(&format!("-{h}"))) {
            errs.push(format!(
                "{key}: {OMIT_MARKER} must name the hour it applies to (e.g. {slot}-lauds); a bare slot omits the element at every hour"
            ));
        }
    }
    errs.sort();
    errs
}
