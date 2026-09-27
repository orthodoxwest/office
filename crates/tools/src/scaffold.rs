//! Proper-file scaffolds: create missing proper files and append missing
//! commented keys to sparse ones, never touching live sections. Ported from
//! Go's `internal/scaffold`.

use std::collections::HashSet;
use std::path::Path;

use calendar::loader::load_feasts;
use calendar::{DataSource, Feast, Rank};

/// One proper section the engine can resolve.
pub struct Key {
    pub key: &'static str,
    pub blurb: &'static str,
    pub optional: bool,
}

const fn core(key: &'static str, blurb: &'static str) -> Key {
    Key { key, blurb, optional: false }
}

const fn opt(key: &'static str, blurb: &'static str) -> Key {
    Key { key, blurb, optional: true }
}

/// The fill-in slots a normal sanctoral or temporal feast needs.
pub const CORE_KEYS: [Key; 13] = [
    core("collect", "Collect of the feast (all hours when celebrated). Use N. for the proper name if needed."),
    core("benedictus-antiphon", "Antiphon on the Benedictus at Lauds."),
    core("magnificat-antiphon", "Antiphon on the Magnificat at II Vespers."),
    core("magnificat-antiphon-first", "Antiphon on the Magnificat at I Vespers (omit if the same as II Vespers)."),
    core("psalm-antiphon", "Single antiphon for all five psalms (fallback when psalm-antiphon-N is absent)."),
    core("psalm-antiphon-1", "First psalm antiphon at Lauds (and Vespers unless a *-vespers override exists)."),
    core("psalm-antiphon-2", "Second psalm antiphon at Lauds."),
    core("psalm-antiphon-3", "Third psalm antiphon at Lauds."),
    core("psalm-antiphon-4", "Fourth psalm antiphon at Lauds (often the canticle antiphon)."),
    core("psalm-antiphon-5", "Fifth psalm antiphon at Lauds (Laudate psalms)."),
    core("commemoration-antiphon", "Antiphon when this feast is only commemorated (not the day's celebration)."),
    core("commemoration-versicle", "Versicle pair (V. … / R. …) for the commemoration of this feast."),
    core("commemoration-collect", "Collect for the commemoration (defaults to [collect] when omitted)."),
];

/// Common hour-qualified spellings.
pub const OPTIONAL_KEYS: [Key; 53] = [
    opt(
        "commemoration-antiphon-at-first-vespers",
        "Antiphon for nonterminal days of this octave at I Vespers of another feast. Sundays use their separate context appointments.",
    ),
    opt(
        "commemoration-versicle-at-first-vespers",
        "Verse for nonterminal days of this octave at I Vespers of another feast. Sundays use their separate context appointments.",
    ),
    opt(
        "commemoration-antiphon-sunday-first-vespers",
        "Antiphon for this octave at I Vespers of the Sunday within this octave. No implicit fallback between these contexts.",
    ),
    opt(
        "commemoration-antiphon-sunday-second-vespers",
        "Antiphon for this octave at Sunday II Vespers when tomorrow celebrates this octave. No implicit fallback between these contexts.",
    ),
    opt(
        "commemoration-antiphon-sunday-second-vespers-before-other-office",
        "Antiphon for this octave at Sunday II Vespers when tomorrow celebrates another office. No implicit fallback between these contexts.",
    ),
    opt(
        "commemoration-versicle-sunday-first-vespers",
        "Versicle pair for this octave at I Vespers of the Sunday within this octave. No implicit fallback between these contexts.",
    ),
    opt(
        "commemoration-versicle-sunday-second-vespers",
        "Versicle pair for this octave at Sunday II Vespers when tomorrow celebrates this octave. No implicit fallback between these contexts.",
    ),
    opt(
        "commemoration-versicle-sunday-second-vespers-before-other-office",
        "Versicle pair for this octave at Sunday II Vespers when tomorrow celebrates another office. No implicit fallback between these contexts.",
    ),
    opt("chapter-first-vespers", "Chapter at I Vespers when it differs from II Vespers."),
    opt("chapter-lauds", "Chapter (capitulum) at Lauds. Bare [chapter] also works for all hours."),
    opt("chapter-prime", "Proper chapter at Prime."),
    opt("chapter-ferial-prime", "Proper ferial chapter at Prime when the office explicitly supplies one."),
    opt("chapter-terce", "Proper chapter at Terce."),
    opt("chapter-sext", "Proper chapter at Sext."),
    opt("chapter-none", "Proper chapter at None."),
    opt("chapter-vespers", "Chapter at Vespers (often @use of chapter-lauds)."),
    opt("chapter-compline", "Proper chapter at Compline."),
    opt("hymn-first-vespers", "Hymn at I Vespers when it differs from II Vespers."),
    opt("hymn-lauds", "Hymn at Lauds. First line may be a Latin incipit title."),
    opt("hymn-prime", "Proper hymn at Prime."),
    opt("hymn-terce", "Proper hymn at Terce."),
    opt("hymn-sext", "Proper hymn at Sext."),
    opt("hymn-none", "Proper hymn at None."),
    opt("hymn-vespers", "Hymn at Vespers."),
    opt("hymn-compline", "Proper hymn at Compline."),
    opt("versicle-lauds", "Versicle after the hymn at Lauds (V. … / R. …)."),
    opt("versicle-prime", "Proper versicle at Prime."),
    opt("versicle-terce", "Proper versicle at Terce."),
    opt("versicle-sext", "Proper versicle at Sext."),
    opt("versicle-none", "Proper versicle at None."),
    opt("versicle-vespers", "Versicle after the hymn at Vespers."),
    opt("versicle-first-vespers", "Versicle at I Vespers when it differs from II Vespers."),
    opt("versicle-compline", "Proper versicle at Compline."),
    opt("short-responsory-lauds", "Short responsory at Lauds."),
    opt("short-responsory-prime", "Proper short responsory at Prime."),
    opt("short-responsory-terce", "Proper short responsory at Terce."),
    opt("short-responsory-sext", "Proper short responsory at Sext."),
    opt("short-responsory-none", "Proper short responsory at None."),
    opt("short-responsory-vespers", "Short responsory at Vespers."),
    opt("short-responsory-first-vespers", "Short responsory at I Vespers when it differs from II Vespers."),
    opt("short-responsory-compline", "Proper short responsory at Compline."),
    opt("vespers-psalmody", "Festal Vespers psalmody table (@use ordinary/vespers/festal-psalmody or a common)."),
    opt("vespers-psalmody-first", "I Vespers psalmody table when it differs from II Vespers."),
    opt("psalm-antiphon-1-vespers", "Vespers-only override for psalm antiphon 1."),
    opt("psalm-antiphon-2-vespers", "Vespers-only override for psalm antiphon 2."),
    opt("psalm-antiphon-3-vespers", "Vespers-only override for psalm antiphon 3."),
    opt("psalm-antiphon-4-vespers", "Vespers-only override for psalm antiphon 4 (often @use of psalm-antiphon-5)."),
    opt("psalm-antiphon-5-vespers", "Vespers-only override for psalm antiphon 5."),
    opt("psalm-antiphon-1-first-vespers", "I Vespers override for psalm antiphon 1."),
    opt("psalm-antiphon-2-first-vespers", "I Vespers override for psalm antiphon 2."),
    opt("psalm-antiphon-3-first-vespers", "I Vespers override for psalm antiphon 3."),
    opt("psalm-antiphon-4-first-vespers", "I Vespers override for psalm antiphon 4."),
    opt("psalm-antiphon-5-first-vespers", "I Vespers override for psalm antiphon 5."),
];

/// The thinner catalog for rank=commemoration feasts.
pub const COMMEMORATION_KEYS: [Key; 8] = [
    core("commemoration-antiphon", "Antiphon when this feast is commemorated."),
    core("commemoration-antiphon-lauds", "Benedictus antiphon when this feast is commemorated at Lauds."),
    core("commemoration-antiphon-vespers", "Magnificat antiphon when this feast is commemorated at Vespers."),
    core("commemoration-versicle", "Versicle pair (V. … / R. …) for the commemoration."),
    core("commemoration-versicle-lauds", "Versicle pair for the commemoration at Lauds."),
    core("commemoration-versicle-vespers", "Versicle pair for the commemoration at Vespers."),
    core("commemoration-collect", "Collect for the commemoration (defaults to a proper [collect] or the common)."),
    core("collect", "Collect of this feast (used when it is celebrated, or as commemoration fallback)."),
];

/// The catalog for a feast: core then optional, or the commemoration set.
pub fn keys_for(is_commemoration: bool) -> Vec<&'static Key> {
    if is_commemoration {
        return COMMEMORATION_KEYS.iter().collect();
    }
    CORE_KEYS.iter().chain(OPTIONAL_KEYS.iter()).collect()
}

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum Action {
    Create,
    Append,
    Ok,
    Skip,
}

/// What `ensure_propers` did, or would do, for one feast.
pub struct ScaffoldResult {
    pub feast_id: String,
    /// Relative to the data directory.
    pub path: String,
    pub action: Action,
    pub added_keys: Vec<&'static str>,
    pub skip_reason: &'static str,
}

pub struct Options<'a> {
    pub write: bool,
    pub include_commemorations: bool,
    pub feast_id: &'a str,
}

/// Go's `EnsurePropers`.
pub fn ensure_propers(src: &dyn DataSource, dir: &Path, opts: &Options) -> Result<Vec<ScaffoldResult>, String> {
    let feasts = load_feasts(src).map_err(|e| format!("loading feasts: {e}"))?;
    let proper_dir = dir.join("texts").join("proper");
    if opts.write {
        std::fs::create_dir_all(&proper_dir).map_err(|e| format!("creating proper dir: {e}"))?;
    }
    let mut results = Vec::new();
    for feast in feasts.iter().filter(|f| opts.feast_id.is_empty() || f.id == opts.feast_id) {
        results.push(ensure_one(&proper_dir, feast, opts)?);
    }
    if !opts.feast_id.is_empty() && results.is_empty() {
        return Err(format!("unknown feast id {}", compat::quote(opts.feast_id)));
    }
    results.sort_by(|a, b| a.feast_id.cmp(&b.feast_id));
    Ok(results)
}

fn ensure_one(proper_dir: &Path, feast: &Feast, opts: &Options) -> Result<ScaffoldResult, String> {
    let rel = format!("texts/proper/{}.txt", feast.id);
    let path = proper_dir.join(format!("{}.txt", feast.id));
    let mut r =
        ScaffoldResult { feast_id: feast.id.clone(), path: rel.clone(), action: Action::Ok, added_keys: Vec::new(), skip_reason: "" };
    let is_comm = feast.rank == Rank::Commemoration;
    if is_comm && !opts.include_commemorations {
        r.action = Action::Skip;
        r.skip_reason = "commemoration (pass -include-commemorations to scaffold)";
        return Ok(r);
    }
    let keys = keys_for(is_comm);
    let existing = match std::fs::read(&path) {
        Ok(b) => Some(String::from_utf8_lossy(&b).into_owned()),
        Err(e) if e.kind() == std::io::ErrorKind::NotFound => None,
        Err(e) => return Err(format!("open {}: {e}", path.display())),
    };
    let present = present_keys(existing.as_deref());
    let missing: Vec<&Key> = keys.iter().copied().filter(|k| !present.contains(k.key)).collect();
    let write = |body: &str| std::fs::write(&path, body).map_err(|e| format!("writing {rel}: {e}"));
    match existing {
        None if missing.is_empty() => Ok(r),
        None => {
            r.action = Action::Create;
            r.added_keys = keys.iter().map(|k| k.key).collect();
            if opts.write {
                write(&render_new_file(feast, &keys))?;
            }
            Ok(r)
        }
        Some(_) if missing.is_empty() => Ok(r),
        Some(content) => {
            r.action = Action::Append;
            r.added_keys = missing.iter().map(|k| k.key).collect();
            if opts.write {
                // Preserve the original exactly; append after a blank line.
                write(&format!("{}\n\n{}", content.trim_end_matches('\n'), render_key_blocks(&missing)))?;
            }
            Ok(r)
        }
    }
}

/// Section names present live or as a commented scaffold.
fn present_keys(content: Option<&str>) -> HashSet<String> {
    let mut out = HashSet::new();
    for line in content.unwrap_or("").split('\n') {
        let trimmed = line.trim();
        let name = match trimmed.strip_prefix('#') {
            Some(rest) => section_name(rest.trim_start()),
            None => section_name(trimmed),
        };
        if let Some(n) = name {
            out.insert(n.to_string());
        }
    }
    out
}

/// `[a-z0-9-]+` between brackets, trailing whitespace allowed.
fn section_name(s: &str) -> Option<&str> {
    let inner = s.strip_prefix('[')?.trim_end().strip_suffix(']')?;
    (!inner.is_empty() && inner.bytes().all(|c| c.is_ascii_lowercase() || c.is_ascii_digit() || c == b'-')).then_some(inner)
}

fn render_new_file(feast: &Feast, keys: &[&Key]) -> String {
    let category = feast.category.map_or("", |c| c.as_str());
    let mut b =
        format!("# Proper scaffold for [{}]\n# Name: {}\n# Rank: {}  Category: {category}\n", feast.id, feast.name, feast.rank.as_str());
    if let Some(p) = feast.proper_id.as_deref().filter(|p| !p.is_empty() && *p != feast.id) {
        b.push_str(&format!("# ProperID: {p} (looked up before this file)\n"));
    }
    b.push_str("#\n");
    b.push_str("# Uncomment a section header and put text (or @use …) beneath it to activate.\n");
    b.push_str("# Commented sections are not loaded. Do not leave an empty uncommented [key] —\n");
    b.push_str("# that silences make audit without providing text.\n");
    if !category.is_empty() {
        b.push_str(&format!("# Commons fallback: commons/{category}/{{ref}}\n"));
    }
    b.push_str("# Intentional commons-only: add this feast id to data/audit-ok.txt\n");
    b.push_str("#\n");
    let (optional, core): (Vec<&Key>, Vec<&Key>) = keys.iter().copied().partition(|k| k.optional);
    if !core.is_empty() {
        b.push_str("# --- core ---\n#\n");
        b.push_str(&render_key_blocks(&core));
    }
    if !optional.is_empty() {
        if !core.is_empty() {
            b.push('\n');
        }
        b.push_str("# --- optional ---\n#\n");
        b.push_str(&render_key_blocks(&optional));
    }
    b
}

fn render_key_blocks(keys: &[&Key]) -> String {
    let mut b = String::new();
    for (i, k) in keys.iter().enumerate() {
        if i > 0 {
            b.push_str("#\n");
        }
        b.push_str(&format!("# [{}]\n# {}\n", k.key, k.blurb));
    }
    b
}

/// Whether any create or append is pending.
pub fn needs_work(results: &[ScaffoldResult]) -> bool {
    results.iter().any(|r| matches!(r.action, Action::Create | Action::Append))
}

/// Counts: created, appended, ok, skipped.
pub fn summarize(results: &[ScaffoldResult]) -> (usize, usize, usize, usize) {
    let count = |a: Action| results.iter().filter(|r| r.action == a).count();
    (count(Action::Create), count(Action::Append), count(Action::Ok), count(Action::Skip))
}
