//! Text-corpus lints: mechanical findings fail `make check`; advisory ones
//! are heuristics for a human eye. Ported from Go's `audit/lint.go`.

use std::collections::{BTreeMap, BTreeSet, HashMap, HashSet};
use std::fmt::Write as _;
use std::sync::LazyLock;

use calendar::DataSource;
use compat::quote;
use regex::Regex;

use super::trim_index_suffix;

/// One lint hit: a corpus key (or a file path for non-corpus findings).
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct LintFinding {
    pub key: String,
    pub class: &'static str,
    pub detail: String,
}

#[derive(Default)]
pub struct LintReport {
    pub mechanical: Vec<LintFinding>,
    pub advisory: Vec<LintFinding>,
}

/// Distinctly Latin tokens; an entry is flagged only for two or more.
const LATIN_WORDS: [&str; 21] = [
    "saecula",
    "saeculorum",
    "saeculi",
    "domine",
    "dominus",
    "dominum",
    "domini",
    "deus",
    "deum",
    "nobis",
    "omnia",
    "semper",
    "sancto",
    "sancti",
    "spiritui",
    "misericordia",
    "peccata",
    "caeli",
    "terrae",
    "eleison",
    "oremus",
];

static WORD_RE: LazyLock<Regex> = LazyLock::new(|| Regex::new(r"[A-Za-z]+").expect("valid regex"));
static INDEXED_ANTIPHON_RE: LazyLock<Regex> = LazyLock::new(|| Regex::new(r"/psalm-antiphon(-[0-9]+)?$").expect("valid regex"));

/// Scans the corpus (and the chant scores) for formatting and content
/// anomalies.
pub fn lint(src: &dyn DataSource) -> Result<LintReport, String> {
    let texts = office::texts::load_texts(src).map_err(|e| format!("loading texts: {e}"))?;
    let entries = texts.entries();
    let mut r = LintReport::default();
    for (key, text) in &entries {
        lint_mechanical(&mut r, key, text);
        lint_latin(&mut r, key, text);
        lint_truncation(&mut r, key, text);
        lint_unpointed_antiphon(&mut r, key, text);
    }
    lint_duplicate_candidates(&mut r, &entries);
    lint_near_duplicates(&mut r, &entries);
    // Every advisory finding, ordered totally (one key can carry several
    // findings of a class).
    r.advisory.sort_by(|a, b| a.class.cmp(b.class).then_with(|| a.key.cmp(&b.key)).then_with(|| a.detail.cmp(&b.detail)));
    lint_chant_orphans(&mut r, src);
    Ok(r)
}

fn lint_duplicate_candidates(r: &mut LintReport, entries: &BTreeMap<&str, &str>) {
    let mut by_text: HashMap<&str, Vec<&str>> = HashMap::new();
    for (key, text) in entries {
        if text.len() >= 20 {
            by_text.entry(text).or_default().push(key);
        }
    }
    for duplicates in by_text.values().filter(|d| d.len() >= 2) {
        r.advisory.push(LintFinding {
            key: duplicates[0].to_string(),
            class: "duplicate-candidate",
            detail: format!("identical concrete entries: {}", duplicates[1..].join(", ")),
        });
    }
}

fn lint_mechanical(r: &mut LintReport, key: &str, text: &str) {
    let mut add = |class: &'static str, detail: String| r.mechanical.push(LintFinding { key: key.to_string(), class, detail });
    for c in text.chars() {
        let detail = match c {
            '\u{FFFD}' => "contains U+FFFD replacement character".to_string(),
            '\u{A0}' => "contains non-breaking space".to_string(),
            c if c < '\u{20}' && c != '\n' => format!("contains control character U+{:04X}", c as u32),
            _ => continue,
        };
        add("control-char", detail);
        break; // one finding per entry is enough
    }
    if let Some(i) = text.split('\n').position(|l| l.contains("  ")) {
        add("double-space", format!("line {} has doubled spaces", i + 1));
    }
    if let Some(i) = text.split('\n').position(|l| l != l.trim_end_matches([' ', '\t'])) {
        add("trailing-space", format!("line {} has trailing whitespace", i + 1));
    }
    if text.contains("**") {
        add("asterisk", "contains doubled asterisk".into());
    }
    if text.contains("\\n") {
        add("escape-sequence", "contains literal \\n escape".into());
    }
    if text.contains('`') {
        add("backtick", "contains backtick (use apostrophe)".into());
    }
}

/// Indexed psalm antiphons normally carry a pointing asterisk.
fn lint_unpointed_antiphon(r: &mut LintReport, key: &str, text: &str) {
    if INDEXED_ANTIPHON_RE.is_match(key) && !text.contains('*') {
        r.advisory.push(LintFinding { key: key.to_string(), class: "unpointed-antiphon", detail: "no pointing asterisk".into() });
    }
}

/// Two or more distinct Latin words, skipping a hymn's title line and
/// canticle section markup.
fn lint_latin(r: &mut LintReport, key: &str, text: &str) {
    let mut seen = BTreeSet::new();
    for (i, line) in text.split('\n').enumerate() {
        if line.trim().starts_with("[section:") || (i == 0 && key.contains("hymn")) {
            continue;
        }
        for w in WORD_RE.find_iter(line) {
            let lw = w.as_str().to_lowercase();
            if LATIN_WORDS.contains(&lw.as_str()) {
                seen.insert(lw);
            }
        }
    }
    if seen.len() >= 2 {
        let words: Vec<String> = seen.into_iter().collect();
        r.advisory.push(LintFinding { key: key.to_string(), class: "latin", detail: format!("Latin words: {}", words.join(", ")) });
    }
}

/// An entry ending in a letter was probably cut off.
fn lint_truncation(r: &mut LintReport, key: &str, text: &str) {
    let chars: Vec<char> = text.trim().chars().collect();
    // PORT(inherited): Go's unicode.IsLetter (category L) against Rust's
    // Alphabetic property; they agree on the corpus's final characters.
    if !chars.last().is_some_and(|c| c.is_alphabetic()) {
        return;
    }
    let tail =
        if chars.len() > 40 { format!("…{}", chars[chars.len() - 40..].iter().collect::<String>()) } else { chars.iter().collect() };
    r.advisory.push(LintFinding { key: key.to_string(), class: "truncated", detail: format!("no terminal punctuation: {}", quote(&tail)) });
}

/// Pairs sharing a slot name whose word sets nearly coincide without being
/// identical: usually one text seeded twice with a transcription slip.
fn lint_near_duplicates(r: &mut LintReport, entries: &BTreeMap<&str, &str>) {
    let mut by_slot: HashMap<&str, Vec<(&str, HashSet<String>)>> = HashMap::new();
    for (key, text) in entries {
        if text.len() < 60 {
            continue;
        }
        let slot = trim_index_suffix(key.rsplit('/').next().unwrap_or(key));
        let words = WORD_RE.find_iter(&text.to_lowercase()).map(|m| m.as_str().to_string()).collect();
        by_slot.entry(slot).or_default().push((key, words));
    }
    for docs in by_slot.values() {
        for (i, (a_key, a)) in docs.iter().enumerate() {
            for (b_key, b) in &docs[i + 1..] {
                if entries[a_key] == entries[b_key] {
                    continue; // identical is intentional reuse
                }
                let inter = a.iter().filter(|w| b.contains(*w)).count();
                let union = a.len() + b.len() - inter;
                let s = if union == 0 { 0.0 } else { inter as f64 / union as f64 };
                if s >= 0.85 {
                    r.advisory.push(LintFinding {
                        key: a_key.to_string(),
                        class: "near-duplicate",
                        detail: format!("differs slightly from {b_key} (similarity {s:.2})"),
                    });
                }
            }
        }
    }
}

/// Psalm and canticle scores with no matching text. Hymn scores are keyed
/// by title slug and not checked.
fn lint_chant_orphans(r: &mut LintReport, src: &dyn DataSource) {
    for category in ["psalms", "canticles"] {
        let Ok(files) = src.walk(&format!("texts/chant/{category}")) else { continue };
        for (rel, _) in files {
            let Some(base) = rel.strip_suffix(".gabc").filter(|b| !b.contains('/')) else { continue };
            if !matches!(src.read(&format!("texts/{category}/{base}.txt")), Ok(Some(_))) {
                r.mechanical.push(LintFinding {
                    key: format!("chant/{category}/{base}.gabc"),
                    class: "gabc-orphan",
                    detail: format!("no matching {category}/{base}.txt"),
                });
            }
        }
    }
}

/// The report, and whether any mechanical finding was present.
pub fn format_lint(r: &LintReport) -> (String, bool) {
    let mut w = String::new();
    let _ = writeln!(w, "=== Mechanical lint: {} finding(s) ===", r.mechanical.len());
    for f in &r.mechanical {
        let _ = writeln!(w, "  [{}] {}: {}", f.class, f.key, f.detail);
    }
    w.push('\n');
    let _ = writeln!(w, "=== Advisory lint: {} finding(s) ===", r.advisory.len());
    if !r.advisory.is_empty() {
        let mut classes: BTreeMap<&str, usize> = BTreeMap::new();
        for f in &r.advisory {
            *classes.entry(f.class).or_default() += 1;
        }
        for (class, n) in classes {
            let _ = writeln!(w, "  {class}: {n}");
        }
        w.push('\n');
        for f in &r.advisory {
            let _ = writeln!(w, "  [{}] {}: {}", f.class, f.key, f.detail);
        }
    }
    w.push('\n');
    (w, !r.mechanical.is_empty())
}
