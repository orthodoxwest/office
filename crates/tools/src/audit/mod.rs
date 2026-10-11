//! Data completeness audit: placeholder texts, missing propers, flat antiphon sets, and
//! translation-register review. The composition sweep and the text lints are in [`sweep`] and
//! [`lint`].

pub mod lint;
pub mod sweep;

use std::collections::{HashMap, HashSet};
use std::fmt::Write as _;
use std::sync::LazyLock;

use calendar::loader::load_feasts;
use calendar::{DataSource, FeastRef, Rank};
use corpus::Corpus;
use data_format::scan_lines;
use regex::Regex;

/// The refs looked up under `proper/<feast-id>/`: the first four for the
/// day's celebration, the last two for a commemoration.
const PROPER_REFS: [&str; 6] =
    ["psalm-antiphon", "benedictus-antiphon", "magnificat-antiphon", "collect", "commemoration-antiphon", "commemoration-versicle"];

/// The ordinary keys each proper ref ultimately falls back to.
fn ordinary_fallbacks(reference: &str) -> &'static [&'static str] {
    match reference {
        "psalm-antiphon" => &["ordinary/lauds/psalm-antiphon", "ordinary/vespers/psalm-antiphon"],
        "benedictus-antiphon" => &["ordinary/lauds/benedictus-antiphon"],
        "magnificat-antiphon" => &["ordinary/vespers/magnificat-antiphon"],
        "collect" => &[
            "ordinary/lauds/collect",
            "ordinary/vespers/collect",
            "ordinary/prime/collect",
            "ordinary/terce/collect",
            "ordinary/sext/collect",
            "ordinary/none/collect",
            "ordinary/compline/collect",
        ],
        "commemoration-antiphon" => &["ordinary/lauds/commemoration-antiphon"],
        "commemoration-versicle" => &["ordinary/lauds/commemoration-versicle"],
        _ => &[],
    }
}

/// The proper texts one feast lacks or takes from its commons.
pub struct FeastGap {
    pub feast: FeastRef,
    /// Absent from both the proper and the category commons.
    pub missing_refs: Vec<&'static str>,
    /// Absent from the proper but covered by the commons.
    pub commons_fallback_refs: Vec<&'static str>,
    /// Missing refs whose ordinary fallback is itself a placeholder.
    pub ph_fallback_refs: Vec<&'static str>,
}

/// The full audit result.
pub struct Report {
    pub placeholders: Vec<String>,
    pub gaps: Vec<FeastGap>,
    /// Feast IDs suppressed wholesale in `data/audit-ok.txt`.
    pub suppressed: Vec<String>,
    /// Feasts whose texts contain "N." but have no ProperName.
    pub missing_prop_name: Vec<String>,
    pub flat_proper_antiphons: Vec<FeastRef>,
    pub flat_common_antiphons: Vec<&'static str>,
    pub mixed_register: Vec<String>,
    pub modern_collects: Vec<String>,
}

// Word boundaries are ASCII.
static ARCHAIC_PRONOUN_RE: LazyLock<Regex> =
    LazyLock::new(|| Regex::new(r"(?i)(?-u:\b)(thou|thee|thy|thine)(?-u:\b)").expect("valid regex"));
static MODERN_PRONOUN_RE: LazyLock<Regex> =
    LazyLock::new(|| Regex::new(r"(?i)(?-u:\b)(you|your|yours|yourself|yourselves)(?-u:\b)").expect("valid regex"));

/// `feast-id → refs` from `data/audit-ok.txt`; `*` suppresses every ref.
/// The [`REGISTER_OK`] line maps to corpus keys instead.
pub type Suppressions = HashMap<String, HashSet<String>>;

/// The `data/audit-ok.txt` line naming corpus keys whose mixed register is intended.
const REGISTER_OK: &str = "register-ok";

/// Reads `data/audit-ok.txt`; a missing file suppresses nothing.
pub fn load_suppress_file(src: &dyn DataSource) -> Result<Suppressions, String> {
    let mut result: Suppressions = HashMap::new();
    let Some(content) = src.read("audit-ok.txt")? else { return Ok(result) };
    for raw in scan_lines(&content) {
        let line = raw.trim();
        if line.is_empty() || line.starts_with('#') {
            continue;
        }
        let parts: Vec<&str> = line.split_whitespace().collect();
        let refs = result.entry(parts[0].to_string()).or_default();
        if parts.len() == 1 || (parts.len() == 2 && parts[1] == "*") {
            refs.insert("*".to_string());
        } else {
            refs.extend(parts[1..].iter().map(|r| r.to_string()));
        }
    }
    Ok(result)
}

/// Whether `prefix` supplies `reference`, counting the slots the composer
/// derives a commemoration from: the hour's own gospel antiphon and versicle.
fn derived_ref_present(corpus: &Corpus, prefix: &str, reference: &str) -> bool {
    let candidates: &[&str] = match reference {
        "commemoration-antiphon" => &[
            "commemoration-antiphon",
            "commemoration-antiphon-lauds",
            "commemoration-antiphon-vespers",
            "benedictus-antiphon",
            "magnificat-antiphon",
        ],
        "commemoration-versicle" => &[
            "commemoration-versicle",
            "commemoration-versicle-lauds",
            "commemoration-versicle-vespers",
            "versicle-lauds",
            "versicle-vespers",
            "versicle",
        ],
        _ => &[],
    };
    corpus.has(&format!("{prefix}{reference}")) || candidates.iter().any(|c| corpus.has(&format!("{prefix}{c}")))
}

pub fn run(src: &dyn DataSource) -> Result<Report, String> {
    let texts = office::texts::load_texts(src).map_err(|e| format!("loading texts: {e}"))?;
    let corpus: &Corpus = &texts;
    let feasts = load_feasts(src).map_err(|e| format!("loading feasts: {e}"))?;
    let suppress = load_suppress_file(src).map_err(|e| format!("loading audit-ok.txt: {e}"))?;

    let placeholders: Vec<String> = corpus.find_placeholders().into_iter().map(str::to_string).collect();
    let ph_set: HashSet<&str> = placeholders.iter().map(String::as_str).collect();
    let none = HashSet::new();

    let mut gaps = Vec::new();
    let mut suppressed = Vec::new();
    for feast in &feasts {
        if feast.rank == Rank::Commemoration {
            continue;
        }
        let supp = suppress.get(&feast.id).unwrap_or(&none);
        if supp.contains("*") {
            suppressed.push(feast.id.clone());
            continue;
        }
        let category = feast.category.map_or("", |c| c.as_str());
        let (mut missing, mut commons, mut ph) = (Vec::new(), Vec::new(), Vec::new());
        for reference in PROPER_REFS {
            if supp.contains(reference) || derived_ref_present(corpus, &format!("proper/{}/", feast.id), reference) {
                continue;
            }
            // A de Tempore commemoration takes the season's or Psalter's versicle (X, p. xxix).
            if reference == "commemoration-versicle"
                && (feast.is_category(calendar::Category::Sunday) || feast.is_category(calendar::Category::Feria))
            {
                continue;
            }
            if derived_ref_present(corpus, &format!("commons/{category}/"), reference) {
                commons.push(reference);
            } else {
                missing.push(reference);
                if ordinary_fallbacks(reference).iter().any(|fb| ph_set.contains(fb)) {
                    ph.push(reference);
                }
            }
        }
        if !missing.is_empty() || !commons.is_empty() {
            gaps.push(FeastGap { feast: feast.clone(), missing_refs: missing, commons_fallback_refs: commons, ph_fallback_refs: ph });
        }
    }

    let mut missing_prop_name = Vec::new();
    for feast in &feasts {
        if feast.rank == Rank::Commemoration || feast.proper_name.as_deref().is_some_and(|n| !n.is_empty()) {
            continue;
        }
        for reference in PROPER_REFS {
            let mut text = corpus.get(&format!("proper/{}/{reference}", feast.id));
            if text.is_empty()
                && let Some(category) = feast.category
            {
                text = corpus.get(&format!("commons/{}/{reference}", category.as_str()));
            }
            if text.contains("N.") {
                missing_prop_name.push(feast.id.clone());
                break;
            }
        }
    }

    let (modern_collects, mixed_register) = find_translation_review_entries(corpus, suppress.get(REGISTER_OK).unwrap_or(&none));
    Ok(Report {
        placeholders,
        gaps,
        suppressed,
        missing_prop_name,
        flat_proper_antiphons: find_flat_proper_antiphons(corpus, &feasts),
        flat_common_antiphons: find_flat_common_antiphons(corpus),
        mixed_register,
        modern_collects,
    })
}

fn source(feast: &FeastRef) -> &str {
    feast.source.as_deref().unwrap_or("")
}

fn find_flat_proper_antiphons(corpus: &Corpus, feasts: &[FeastRef]) -> Vec<FeastRef> {
    let mut flat: Vec<FeastRef> =
        feasts.iter().filter(|f| has_flat_indexed_psalm_antiphons(corpus, &format!("proper/{}", f.id))).cloned().collect();
    flat.sort_by(|a, b| source(a).cmp(source(b)).then(b.rank.weight().cmp(&a.rank.weight())).then(a.name.cmp(&b.name)));
    flat
}

fn find_flat_common_antiphons(corpus: &Corpus) -> Vec<&'static str> {
    const COMMON_IDS: [&str; 13] = [
        "apostle",
        "bishop-martyr",
        "blessed-virgin",
        "confessor",
        "confessor-bishop",
        "confessor-doctor",
        "dedication",
        "evangelist",
        "holy-woman",
        "martyr",
        "martyrs",
        "virgin",
        "virgin-martyr",
    ];
    COMMON_IDS.into_iter().filter(|id| has_flat_indexed_psalm_antiphons(corpus, &format!("commons/{id}"))).collect()
}

/// The paschal antiphon; five copies of it deliberately encode "psalms under
/// one antiphon".
const ALLELUIA_ANTIPHON: &str = "Alleluia, * alleluia, alleluia.";

fn has_flat_indexed_psalm_antiphons(corpus: &Corpus, prefix: &str) -> bool {
    let mut antiphons = Vec::with_capacity(5);
    for i in 1..=5 {
        let text = corpus.get(&format!("{prefix}/psalm-antiphon-{i}"));
        if text.is_empty() {
            return false;
        }
        antiphons.push(text);
    }
    antiphons.iter().all(|t| *t == antiphons[0]) && antiphons[0] != ALLELUIA_ANTIPHON
}

fn find_translation_review_entries(corpus: &Corpus, register_ok: &HashSet<String>) -> (Vec<String>, Vec<String>) {
    let (mut modern_collects, mut mixed) = (Vec::new(), Vec::new());
    for (key, text) in corpus.entries() {
        if !["proper/", "commons/", "ordinary/", "seasonal/"].iter().any(|p| key.starts_with(p)) {
            continue;
        }
        let has_modern = MODERN_PRONOUN_RE.is_match(text);
        let has_archaic = ARCHAIC_PRONOUN_RE.is_match(text);
        if has_modern && key.ends_with("/collect") {
            modern_collects.push(key.to_string());
        }
        if has_modern && has_archaic && has_mixed_register_section(key) && !register_ok.contains(key) {
            mixed.push(key.to_string());
        }
    }
    modern_collects.sort();
    mixed.sort();
    (modern_collects, mixed)
}

fn has_mixed_register_section(key: &str) -> bool {
    let section = key.rsplit('/').next().unwrap_or(key);
    matches!(
        section,
        "collect" | "psalm-antiphon" | "benedictus-antiphon" | "magnificat-antiphon" | "commemoration-antiphon" | "commemoration-versicle"
    ) || section.starts_with("psalm-antiphon-")
}

/// Feasts grouped by source, the two printed sources first, each group by
/// rank then name.
fn print_gaps(w: &mut String, gaps: &[&FeastGap], format: impl Fn(&FeastGap) -> String) {
    let mut sources = vec!["base", "awrv"];
    for g in gaps {
        if !sources.contains(&source(&g.feast)) {
            sources.push(source(&g.feast));
        }
    }
    sources[2..].sort_unstable();
    for src in sources {
        let mut gs: Vec<&&FeastGap> = gaps.iter().filter(|g| source(&g.feast) == src).collect();
        if gs.is_empty() {
            continue;
        }
        gs.sort_by(|a, b| b.feast.rank.weight().cmp(&a.feast.rank.weight()).then(a.feast.name.cmp(&b.feast.name)));
        let _ = write!(w, "\n  [{src}]\n");
        for g in gs {
            w.push_str(&format(g));
        }
    }
}

/// The human-readable report.
pub fn format_report(r: &Report) -> String {
    let mut w = String::new();
    let _ = writeln!(w, "=== Placeholders: {} corpus entries ===", r.placeholders.len());
    if !r.placeholders.is_empty() {
        w.push_str("These are shared texts used as fallbacks — fill these in before adding propers.\n");
        for k in &r.placeholders {
            let _ = writeln!(w, "  {k}");
        }
    }
    w.push('\n');

    let (gaps_missing, gaps_commons): (Vec<&FeastGap>, Vec<&FeastGap>) = r.gaps.iter().partition(|g| !g.missing_refs.is_empty());
    let ph_count = gaps_missing.iter().filter(|g| !g.ph_fallback_refs.is_empty()).count();
    let _ = writeln!(w, "=== Missing propers: {} feast(s) ===", gaps_missing.len());
    if !gaps_missing.is_empty() {
        if ph_count == gaps_missing.len() {
            w.push_str("All feasts fall back to placeholder ordinary texts (marked with !).\n");
        } else if ph_count > 0 {
            let _ = writeln!(w, "{ph_count} feast(s) fall back to placeholder ordinary texts (marked with !).");
        }
        print_gaps(&mut w, &gaps_missing, |g| {
            let note = if g.ph_fallback_refs.is_empty() { "" } else { " !" };
            format!("  [{}] {} ({}){note}\n    missing: {}\n", g.feast.rank.abbrev(), g.feast.name, g.feast.id, g.missing_refs.join(", "))
        });
    }
    w.push('\n');

    let _ = writeln!(w, "=== Commons fallback: {} feast(s) ===", gaps_commons.len());
    if !gaps_commons.is_empty() {
        w.push_str("These refs are covered by the category commons, not feast-specific propers.\n");
        w.push_str("Add to data/audit-ok.txt to acknowledge and suppress.\n");
        print_gaps(&mut w, &gaps_commons, |g| {
            format!(
                "  [{}] {} ({})\n    commons: {}\n",
                g.feast.rank.abbrev(),
                g.feast.name,
                g.feast.id,
                g.commons_fallback_refs.join(", ")
            )
        });
    }
    w.push('\n');

    let _ = writeln!(
        w,
        "=== Flat indexed psalm antiphons: {} proper, {} common ===",
        r.flat_proper_antiphons.len(),
        r.flat_common_antiphons.len()
    );
    if !r.flat_proper_antiphons.is_empty() || !r.flat_common_antiphons.is_empty() {
        w.push_str("These files have psalm-antiphon-1..5 all set to the same text.\n");
        w.push_str("They are likely candidates for Divinum Officium seeding cleanup under the newer antiphon model.\n");
        if !r.flat_proper_antiphons.is_empty() {
            w.push_str("\n  [proper]\n");
            for src in ["base", "awrv"] {
                let items: Vec<&FeastRef> = r.flat_proper_antiphons.iter().filter(|f| source(f) == src).collect();
                if items.is_empty() {
                    continue;
                }
                let _ = writeln!(w, "  [{src}]");
                for f in items {
                    let _ = writeln!(w, "    [{}] {} ({})", f.rank.abbrev(), f.name, f.id);
                }
            }
        }
        if !r.flat_common_antiphons.is_empty() {
            w.push_str("\n  [commons]\n");
            for id in &r.flat_common_antiphons {
                let _ = writeln!(w, "    {id}");
            }
        }
    }
    w.push('\n');

    let _ = writeln!(
        w,
        "=== Translation review: {} mixed-register, {} modern-pronoun collect(s) ===",
        r.mixed_register.len(),
        r.modern_collects.len()
    );
    if !r.mixed_register.is_empty() || !r.modern_collects.is_empty() {
        w.push_str("These entries are high-signal candidates for translation normalization.\n");
        w.push_str("Mixed-register entries contain both thou/thee/thy and you/your language in the same text.\n");
        if !r.mixed_register.is_empty() {
            w.push_str("\n  [mixed-register]\n");
            for key in &r.mixed_register {
                let _ = writeln!(w, "    {key}");
            }
        }
        if !r.modern_collects.is_empty() {
            w.push_str("\n  [modern-pronoun collects]\n");
            for key in &r.modern_collects {
                let _ = writeln!(w, "    {key}");
            }
        }
    }
    w.push('\n');

    if !r.suppressed.is_empty() {
        let _ = writeln!(w, "=== Suppressed: {} feast(s) via data/audit-ok.txt ===", r.suppressed.len());
        for id in &r.suppressed {
            let _ = writeln!(w, "  {id}");
        }
        w.push('\n');
    }

    if !r.missing_prop_name.is_empty() {
        let _ = writeln!(w, "=== Missing ProperName: {} feast(s) ===", r.missing_prop_name.len());
        w.push_str("These feasts have texts containing \"N.\" but no ProperName set.\n");
        w.push_str("Add ProperName = <name> to the feast definition to replace \"N.\" with the saint's name.\n");
        for id in &r.missing_prop_name {
            let _ = writeln!(w, "  {id}");
        }
        w.push('\n');
    }

    w.push_str("To mark intentional fallbacks, add lines to data/audit-ok.txt:\n");
    w.push_str("  feast-id *                   (suppress all warnings for this feast)\n");
    w.push_str("  feast-id ref1 ref2 ...        (suppress specific refs only)\n");
    w
}

/// Strips a trailing `-N` ("psalm-antiphon-3" → "psalm-antiphon").
pub fn trim_index_suffix(reference: &str) -> &str {
    let trimmed = reference.trim_end_matches(|c: char| c.is_ascii_digit());
    match trimmed.strip_suffix('-') {
        Some(base) if trimmed.len() < reference.len() => base,
        _ => reference,
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn gaps_from_other_sources_are_printed() {
        let gap = |id: &str, src: &str| {
            let mut f = calendar::Feast::synthetic(id, id, Rank::GreaterDouble, calendar::Color::White, calendar::Category::BlessedVirgin);
            f.source = Some(src.to_string());
            FeastGap {
                feast: std::sync::Arc::new(f),
                missing_refs: vec![],
                commons_fallback_refs: vec!["collect"],
                ph_fallback_refs: vec![],
            }
        };
        let (a, b) = (gap("from-ordo", "2026 archdiocesan ordo"), gap("from-base", "base"));
        let mut w = String::new();
        print_gaps(&mut w, &[&a, &b], |g| format!("{}\n", g.feast.id));
        assert_eq!(w, "\n  [base]\nfrom-base\n\n  [2026 archdiocesan ordo]\nfrom-ordo\n");
    }

    #[test]
    fn pronoun_patterns_use_ascii_boundaries() {
        assert!(MODERN_PRONOUN_RE.is_match("bless You."));
        assert!(!MODERN_PRONOUN_RE.is_match("youth"));
        assert!(ARCHAIC_PRONOUN_RE.is_match("éthou"));
    }
}
