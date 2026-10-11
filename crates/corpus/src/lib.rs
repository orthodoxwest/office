//! The liturgical text corpus: the `data/texts/` file format, the loader, the `@use` and `@omit`
//! directives, the collect-conclusion and Latin-incipit sidecars, and the line grammar. The crate
//! does no file access. Callers pass the files under `data/texts/` in depth-first order, with each
//! directory’s entries in byte order, which matters because a later file may redefine an earlier
//! key.

pub mod lines;
pub mod typography;

use std::collections::{BTreeMap, HashMap, HashSet};

use data_format::{quote, scan_lines};

/// One file under `data/texts/`.
#[derive(Clone, Debug)]
pub struct TextFile {
    /// The path relative to `data/texts/`, with `/` separators.
    pub rel_path: String,
    pub content: String,
}

/// A sidecar file: its path as it appears in error messages, and its contents.
#[derive(Clone, Copy, Debug)]
pub struct Sidecar<'a> {
    pub path: &'a str,
    pub content: &'a str,
}

/// The corpus-key prefix of the conclusion formulas.
pub const COLLECT_CONCLUSION_PREFIX: &str = "shared/formulas/collect-conclusion-";

/// Corpus-key prefixes whose entries carry a Latin incipit.
pub const INCIPIT_PREFIXES: [&str; 2] = ["psalms/", "canticles/"];

/// The body that suppresses an element outright.
pub const OMIT_MARKER: &str = "@omit";

/// Whether a resolved body suppresses its element.
pub fn is_omitted(text: &str) -> bool {
    text.trim() == OMIT_MARKER
}

/// All loaded liturgical texts, keyed by reference path.
#[derive(Clone, Debug, Default)]
pub struct Corpus {
    texts: HashMap<String, String>,
    aliases: HashMap<String, String>,
    collect_conclusions: HashMap<String, String>,
    incipits: HashMap<String, String>,
}

/// Whether a trimmed line is a `[section]` header: non-empty, with no space,
/// colon, or tab inside the brackets.
fn section_name(trimmed: &str) -> Option<&str> {
    let inner = trimmed.strip_prefix('[')?.strip_suffix(']')?;
    (!inner.is_empty() && !inner.contains([' ', ':', '\t'])).then_some(inner)
}

/// Whether the text has any `[section]` header, making it an INI-style file.
pub fn has_ini_sections(text: &str) -> bool {
    scan_lines(text).any(|l| section_name(l.trim()).is_some())
}

/// Drops whole-line `#` comments from a plain-text file.
fn strip_comment_lines(text: &str) -> String {
    text.split('\n').filter(|l| !l.trim().starts_with('#')).collect::<Vec<_>>().join("\n")
}

impl Corpus {
    /// A corpus of the given concrete entries, for tests.
    pub fn from_entries(texts: impl IntoIterator<Item = (String, String)>) -> Corpus {
        Corpus { texts: texts.into_iter().collect(), ..Corpus::default() }
    }

    /// Loads the corpus from the files under `data/texts/` (in walk order) and the optional
    /// sidecars `collect-conclusions.txt` and `latin-incipits.txt`. Appointment scopes are loaded
    /// by the office crate.
    pub fn load(files: &[TextFile], conclusions: Option<Sidecar<'_>>, incipits: Option<Sidecar<'_>>) -> Result<Corpus, String> {
        let mut c = Corpus::default();
        for f in files.iter().filter(|f| f.rel_path.rsplit('/').next().is_some_and(|n| n.ends_with(".txt"))) {
            if has_ini_sections(&f.content) {
                c.load_ini_file(&f.rel_path, &f.content).map_err(|e| format!("loading texts: {e}"))?;
                continue;
            }
            // Comment-only files are not corpus entries.
            let body = strip_comment_lines(&f.content);
            let body = body.trim();
            if !body.is_empty() {
                c.texts.insert(f.rel_path.strip_suffix(".txt").unwrap_or(&f.rel_path).to_string(), body.to_string());
            }
        }
        c.extract_and_validate_aliases()?;
        if let Some(s) = conclusions {
            c.load_collect_conclusions(s)?;
        }
        if let Some(s) = incipits {
            c.load_incipits(s)?;
        }
        Ok(c)
    }

    /// Each `[section]` becomes `dir/stem/section`.
    fn load_ini_file(&mut self, rel_path: &str, text: &str) -> Result<(), String> {
        let (dir, base) = rel_path.rsplit_once('/').unwrap_or(("", rel_path));
        let stem = base.strip_suffix(".txt").unwrap_or(base);
        let mut current: Option<String> = None;
        let mut lines: Vec<&str> = Vec::new();
        let mut sections: HashMap<&str, usize> = HashMap::new();
        let flush = |texts: &mut HashMap<String, String>, key: &Option<String>, lines: &[&str]| {
            if let Some(k) = key {
                texts.insert(k.clone(), lines.join("\n").trim().to_string());
            }
        };
        for (i, line) in scan_lines(text).enumerate() {
            let line_number = i + 1;
            let trimmed = line.trim();
            if trimmed.starts_with('#') {
                continue;
            }
            if trimmed.is_empty() {
                if current.is_some() {
                    lines.push(line);
                }
                continue;
            }
            if let Some(inner) = section_name(trimmed) {
                if let Some(first) = sections.get(inner) {
                    return Err(format!("{rel_path}:{line_number}: duplicate INI section [{inner}] (first declared at line {first})"));
                }
                sections.insert(inner, line_number);
                flush(&mut self.texts, &current, &lines);
                current = Some(if dir.is_empty() { format!("{stem}/{inner}") } else { format!("{dir}/{stem}/{inner}") });
                lines.clear();
                continue;
            }
            if current.is_some() {
                lines.push(line);
            }
        }
        flush(&mut self.texts, &current, &lines);
        Ok(())
    }

    /// Moves exact `@use` directives out of the concrete texts, then checks
    /// every alias resolves. Also rejects an `@omit` that is not the whole
    /// body. Keys are visited in byte order.
    fn extract_and_validate_aliases(&mut self) -> Result<(), String> {
        let mut keys: Vec<String> = self.texts.keys().cloned().collect();
        keys.sort();
        for key in &keys {
            let trimmed = self.texts[key].trim();
            if trimmed.starts_with(OMIT_MARKER) && trimmed != OMIT_MARKER {
                return Err(format!("invalid corpus omission {}: {OMIT_MARKER} must be the whole body", quote(key)));
            }
            if !trimmed.starts_with("@use") {
                continue;
            }
            let fields: Vec<&str> = trimmed.split_whitespace().collect();
            if fields.len() != 2 || fields[0] != "@use" {
                return Err(format!("invalid corpus alias {}: expected @use <corpus-key>", quote(key)));
            }
            let target = fields[1].to_string();
            self.aliases.insert(key.clone(), target);
            self.texts.remove(key);
        }
        let mut aliases: Vec<&String> = self.aliases.keys().collect();
        aliases.sort();
        for alias in aliases {
            if self.canonical_ref(alias).is_none() {
                return Err(format!("corpus alias {} does not resolve (target {})", quote(alias), quote(&self.aliases[alias])));
            }
        }
        Ok(())
    }

    fn load_collect_conclusions(&mut self, s: Sidecar<'_>) -> Result<(), String> {
        let path = s.path;
        for (i, line) in s.content.split('\n').enumerate() {
            let n = i + 1;
            let trimmed = line.trim();
            if trimmed.is_empty() || trimmed.starts_with('#') {
                continue;
            }
            let fields: Vec<&str> = trimmed.split_whitespace().collect();
            if fields.len() != 2 {
                return Err(format!("{path}:{n}: expected \"<corpus-key> <form>\", got {}", quote(trimmed)));
            }
            let (key, form) = (fields[0], fields[1]);
            if self.get(&format!("{COLLECT_CONCLUSION_PREFIX}{form}")).is_empty() {
                return Err(format!(
                    "{path}:{n}: {key} names conclusion form {}, but {COLLECT_CONCLUSION_PREFIX}{form} is not in the corpus",
                    quote(form)
                ));
            }
            if self.get(key).is_empty() {
                return Err(format!("{path}:{n}: {} is not in the corpus", quote(key)));
            }
            if let Some(prior) = self.collect_conclusions.get(key) {
                return Err(format!("{path}:{n}: {} listed twice ({} then {})", quote(key), quote(prior), quote(form)));
            }
            self.collect_conclusions.insert(key.to_string(), form.to_string());
        }
        Ok(())
    }

    fn load_incipits(&mut self, s: Sidecar<'_>) -> Result<(), String> {
        let path = s.path;
        for (i, line) in s.content.split('\n').enumerate() {
            let n = i + 1;
            let trimmed = line.trim();
            if trimmed.is_empty() || trimmed.starts_with('#') {
                continue;
            }
            let Some((raw_key, raw_incipit)) = trimmed.split_once('=') else {
                return Err(format!("{path}:{n}: expected \"<corpus-key> = <incipit>\", got {}", quote(trimmed)));
            };
            let (key, incipit) = (raw_key.trim(), raw_incipit.trim());
            if incipit.is_empty() {
                return Err(format!("{path}:{n}: {} has an empty incipit", quote(key)));
            }
            if !INCIPIT_PREFIXES.iter().any(|p| key.starts_with(p)) {
                return Err(format!("{path}:{n}: {} is not a psalm or canticle key", quote(key)));
            }
            if self.get(key).is_empty() {
                return Err(format!("{path}:{n}: {} is not in the corpus", quote(key)));
            }
            if let Some(prior) = self.incipits.get(key) {
                return Err(format!("{path}:{n}: {} listed twice ({} then {})", quote(key), quote(prior), quote(incipit)));
            }
            self.incipits.insert(key.to_string(), incipit.to_string());
        }
        Ok(())
    }

    /// The concrete key behind `reference`, following aliases; `None` when it
    /// does not resolve (or an alias cycle, which loading rejects).
    pub fn canonical_ref<'a>(&'a self, reference: &'a str) -> Option<&'a str> {
        let mut seen = HashSet::new();
        let mut r = reference;
        loop {
            if !seen.insert(r) {
                return None;
            }
            if let Some(target) = self.aliases.get(r) {
                r = target;
                continue;
            }
            return self.texts.get_key_value(r).map(|(k, _)| k.as_str());
        }
    }

    /// The resolved body, or `""` when the reference does not exist. (A
    /// declared but empty section also reads `""`.)
    pub fn get(&self, reference: &str) -> &str {
        self.canonical_ref(reference).and_then(|k| self.texts.get(k)).map_or("", String::as_str)
    }

    pub fn has(&self, reference: &str) -> bool {
        self.canonical_ref(reference).is_some()
    }

    /// The direct `@use` target of an alias key.
    pub fn alias_target(&self, key: &str) -> Option<&str> {
        self.aliases.get(key).map(String::as_str)
    }

    /// Whether any key (concrete or alias) ends with `/suffix`.
    pub fn has_key_suffix(&self, suffix: &str) -> bool {
        let target = format!("/{suffix}");
        self.texts.keys().chain(self.aliases.keys()).any(|k| k.ends_with(&target))
    }

    /// Every resolvable key, aliases included, in byte order.
    pub fn references(&self) -> Vec<&str> {
        let mut refs: Vec<&str> = self.texts.keys().chain(self.aliases.keys()).map(String::as_str).collect();
        refs.sort_unstable();
        refs
    }

    /// Whether `key` is a concrete (non-alias) entry.
    pub fn is_concrete(&self, key: &str) -> bool {
        self.texts.contains_key(key)
    }

    /// The concrete renderable entries: no aliases, no `psalmody/`
    /// declarations, no omissions.
    pub fn entries(&self) -> BTreeMap<&str, &str> {
        self.texts.iter().filter(|(k, v)| !k.starts_with("psalmody/") && !is_omitted(v)).map(|(k, v)| (k.as_str(), v.as_str())).collect()
    }

    /// Concrete keys whose text begins with "placeholder", in byte order.
    pub fn find_placeholders(&self) -> Vec<&str> {
        let mut keys: Vec<&str> =
            self.texts.iter().filter(|(_, v)| v.to_lowercase().starts_with("placeholder")).map(|(k, _)| k.as_str()).collect();
        keys.sort_unstable();
        keys
    }

    /// The Latin incipit of a psalm or canticle key, following aliases.
    pub fn incipit(&self, reference: &str) -> Option<&str> {
        if let Some(i) = self.incipits.get(reference) {
            return Some(i);
        }
        self.canonical_ref(reference).and_then(|k| self.incipits.get(k)).map(String::as_str)
    }

    /// Psalm and canticle keys with no recorded incipit, in byte order.
    pub fn missing_incipits(&self) -> Vec<&str> {
        let mut keys: Vec<&str> = self
            .texts
            .keys()
            .filter(|k| INCIPIT_PREFIXES.iter().any(|p| k.starts_with(p)) && self.incipits.get(*k).is_none_or(String::is_empty))
            .map(String::as_str)
            .collect();
        keys.sort_unstable();
        keys
    }

    /// The conclusion form recorded for a collect, following aliases; an
    /// explicit alias entry takes precedence.
    pub fn collect_conclusion_form(&self, key: &str) -> Option<&str> {
        if let Some(f) = self.collect_conclusions.get(key) {
            return Some(f);
        }
        self.canonical_ref(key).and_then(|k| self.collect_conclusions.get(k)).map(String::as_str)
    }

    /// Records an incipit, for tests.
    pub fn set_incipit(&mut self, key: &str, incipit: &str) {
        self.incipits.insert(key.to_string(), incipit.to_string());
    }

    /// Iterates the concrete entries (aliases excluded), unordered.
    pub fn concrete(&self) -> impl Iterator<Item = (&str, &str)> {
        self.texts.iter().map(|(k, v)| (k.as_str(), v.as_str()))
    }

    /// Iterates the alias keys, unordered.
    pub fn alias_keys(&self) -> impl Iterator<Item = &str> {
        self.aliases.keys().map(String::as_str)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn file(rel_path: &str, content: &str) -> TextFile {
        TextFile { rel_path: rel_path.to_string(), content: content.to_string() }
    }

    #[test]
    fn ini_and_plain_files() {
        let c = Corpus::load(
            &[
                file(
                    "proper/st-x/lauds.txt",
                    "# header\n[collect]\nO God,\n# SOURCE: note\nwho.\n\n[empty]\n\n[chapter]\n@use shared/x/chapter\n",
                ),
                file("shared/x.txt", "[chapter]\nBrethren.\n"),
                file("psalms/1.txt", "# comment\nBlessed is the man.\n# more\n"),
                file("psalms/2.txt", "# only a comment\n"),
                file("notes.md", "[ignored]\nnot a text file\n"),
                file("top.txt", "[entry]\nTop.\n"),
            ],
            None,
            None,
        )
        .unwrap();
        assert_eq!(c.get("proper/st-x/lauds/collect"), "O God,\nwho.");
        assert!(c.has("proper/st-x/lauds/empty"));
        assert_eq!(c.get("proper/st-x/lauds/empty"), "");
        assert_eq!(c.get("proper/st-x/lauds/chapter"), "Brethren.");
        assert_eq!(c.canonical_ref("proper/st-x/lauds/chapter"), Some("shared/x/chapter"));
        assert_eq!(c.alias_target("proper/st-x/lauds/chapter"), Some("shared/x/chapter"));
        assert_eq!(c.get("psalms/1"), "Blessed is the man.");
        assert!(!c.has("psalms/2"));
        assert!(!c.has("notes/ignored"));
        assert_eq!(c.get("top/entry"), "Top.");
        assert!(c.has_key_suffix("chapter"));
        assert_eq!(c.missing_incipits(), vec!["psalms/1"]);
        // Comment lines strip; blank lines and inline hashes stay.
        let c = Corpus::load(
            &[
                file(
                    "ordinary/lauds.txt",
                    "# File-level comment before any section.\n\n[collect]\n# SOURCE: divinum-officium Sancti/10-DU\nO God, who hast prepared.\n\n[hymn]\n# annotation\nFirst stanza.\n\nSecond stanza.\n",
                ),
                file("psalms/116b.txt", "Psalm 116:10-16\n\n# SOURCE: printed psalter\nI believed * but I was troubled.\nThe # character remains.\n"),
                file(".gitkeep", ""),
                file("proper/empty.txt", "\n\n"),
            ],
            None,
            None,
        )
        .unwrap();
        assert_eq!(c.get("ordinary/lauds/collect"), "O God, who hast prepared.");
        assert_eq!(c.get("ordinary/lauds/hymn"), "First stanza.\n\nSecond stanza.");
        assert_eq!(c.get("psalms/116b"), "Psalm 116:10-16\n\nI believed * but I was troubled.\nThe # character remains.");
        assert!(!c.has("proper/empty"));
        assert_eq!(c.references().len(), 3);
    }

    #[test]
    fn sidecars() {
        let files = [
            file("psalms/23.txt", "The Lord is my shepherd."),
            file("shared/formulas.txt", "[collect-conclusion-per-dominum]\nThrough.\n"),
        ];
        let c = Corpus::load(
            &files,
            Some(Sidecar { path: "c.txt", content: "psalms/23 per-dominum\n" }),
            Some(Sidecar { path: "i.txt", content: "# c\npsalms/23 = Dominus regit me\n" }),
        )
        .unwrap();
        assert_eq!(c.incipit("psalms/23"), Some("Dominus regit me"));
        assert_eq!(c.collect_conclusion_form("psalms/23"), Some("per-dominum"));
        assert!(c.missing_incipits().is_empty());
        let incipit_err = |s| Corpus::load(&files, None, Some(Sidecar { path: "i.txt", content: s })).unwrap_err();
        assert_eq!(incipit_err("psalms/23"), "i.txt:1: expected \"<corpus-key> = <incipit>\", got \"psalms/23\"");
        assert_eq!(incipit_err("psalms/23 ="), "i.txt:1: \"psalms/23\" has an empty incipit");
        assert_eq!(incipit_err("hymns/x = Y"), "i.txt:1: \"hymns/x\" is not a psalm or canticle key");
        assert_eq!(incipit_err("psalms/23 = A\npsalms/23 = B"), "i.txt:2: \"psalms/23\" listed twice (\"A\" then \"B\")");
        let conclusion_err = |s| Corpus::load(&files, Some(Sidecar { path: "c.txt", content: s }), None).unwrap_err();
        assert_eq!(conclusion_err("psalms/23"), "c.txt:1: expected \"<corpus-key> <form>\", got \"psalms/23\"");
        assert_eq!(conclusion_err("psalms/99 per-dominum"), "c.txt:1: \"psalms/99\" is not in the corpus");
    }
}
