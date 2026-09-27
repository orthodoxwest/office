//! The INI-like feast and penitential data files. The crate reads nothing
//! itself: callers hand in file contents through [`DataSource`].

use std::collections::BTreeMap;
use std::sync::Arc;

use crate::date::Date;
use crate::model::{Category, Color, CommemorationClass, Feast, FeastRef, MonthDay, OctaveClass, Rank};
use crate::penitential::{PenitentialRule, section_to_penitential_rule};
use data_format::atoi;
use data_format::quote;

/// The feast definition files, in load order.
pub const FEAST_FILES: [&str; 4] = ["temporal.txt", "sanctoral.txt", "awrv.txt", "commemorations.txt"];

/// The penitential rules file at the data root.
pub const PENITENTIAL_RULES_FILE: &str = "penitential.txt";

/// Supplies data files by path relative to the data directory.
pub trait DataSource {
    /// The path as it should appear in error messages, including the data directory.
    fn display_path(&self, rel: &str) -> String;
    /// The file's contents, `Ok(None)` when it does not exist.
    fn read(&self, rel: &str) -> Result<Option<String>, String>;
    /// Every regular file under the directory `rel`, with its path relative to `rel`
    /// (`/`-separated) and its bytes: depth first, each directory's entries in byte order. An error
    /// names the failing path.
    fn walk(&self, rel: &str) -> Result<Vec<(String, Vec<u8>)>, String>;
}

/// One `[section]` of a data file. Keys keep their last assignment.
#[derive(Clone, Debug, Default, PartialEq, Eq)]
pub struct Section {
    pub id: String,
    pub values: BTreeMap<String, String>,
}

impl Section {
    pub fn get(&self, key: &str) -> Option<&str> {
        self.values.get(key).map(String::as_str)
    }

    /// The value for a key, or the empty string.
    fn value(&self, key: &str) -> &str {
        self.get(key).unwrap_or("")
    }
}

/// Parses an INI-like file into sections. `#` lines and blank lines are
/// ignored; each `[id]` starts a section; other lines are `Key = value`.
pub fn parse_ini_sections(path: &str, content: &str) -> Result<Vec<Section>, String> {
    let mut sections: Vec<Section> = Vec::new();
    for (i, line) in data_format::scan_lines(content).enumerate() {
        let trimmed = line.trim();
        if trimmed.is_empty() || trimmed.starts_with('#') {
            continue;
        }
        if trimmed.len() >= 2 && trimmed.starts_with('[') && trimmed.ends_with(']') {
            sections.push(Section { id: trimmed[1..trimmed.len() - 1].to_string(), values: BTreeMap::new() });
            continue;
        }
        let line_num = i + 1;
        let Some((key, value)) = trimmed.split_once('=') else {
            return Err(format!("{path}:{line_num}: expected Key = value, got {}", quote(trimmed)));
        };
        let Some(current) = sections.last_mut() else {
            return Err(format!("{path}:{line_num}: key-value pair outside of section"));
        };
        current.values.insert(key.trim().to_string(), value.trim().to_string());
    }
    Ok(sections)
}

fn parse_data_bool(value: &str) -> Result<bool, String> {
    match value {
        "true" => Ok(true),
        "false" => Ok(false),
        _ => Err(format!("expected true or false, got {}", quote(value))),
    }
}

fn valid_fixed_date(month: i64, day: i64) -> bool {
    if !(1..=12).contains(&month) || !(1..=31).contains(&day) {
        return false;
    }
    // Leap year 2000 permits February 29 while rejecting impossible dates.
    let date = Date::new(2000, month as i32, day as i32);
    i64::from(date.month()) == month && i64::from(date.day()) == day
}

const KNOWN_FEAST_KEYS: [&str; 21] = [
    "Name",
    "Rank",
    "Color",
    "Category",
    "ProperName",
    "ProperID",
    "DateRule",
    "Month",
    "Day",
    "HasOctave",
    "HasVigil",
    "IsVigil",
    "VigilOf",
    "CommemorationClass",
    "OctaveClass",
    "PrimaryOfOurLord",
    "CompanionOf",
    "OnlyWith",
    "SkipRomanLeapShift",
    "Source",
    "Notes",
];

/// Converts a parsed section into a feast, applying the schema checks.
pub fn section_to_feast(m: &Section, source_file: &str) -> Result<Feast, String> {
    let id = m.id.clone();
    let qid = quote(&id);
    if id.is_empty() {
        return Err(format!("{source_file}: section missing ID"));
    }
    let name = m.value("Name").to_string();
    if name.is_empty() {
        return Err(format!("{source_file}: feast {qid} missing Name"));
    }
    let fail = |e: String| format!("{source_file}: feast {qid}: {e}");

    let rank = Rank::parse(m.value("Rank")).map_err(fail)?;
    let color = Color::parse(m.value("Color")).map_err(fail)?;
    let mut f = Feast::synthetic(id.clone(), name, rank, color, Category::Feria);
    f.category = None;
    f.source = Some("base".to_string());

    if let Some(cat) = m.get("Category") {
        f.category = Some(Category::parse(cat).map_err(fail)?);
    }
    if let Some(dr) = m.get("DateRule") {
        f.date_rule = crate::model::non_empty(dr);
    }
    let mut month = 0i64;
    let mut day = 0i64;
    if let Some(ms) = m.get("Month") {
        month = atoi(ms).map_err(|e| fail(format!("invalid Month: {e}")))?;
    }
    if let Some(ds) = m.get("Day") {
        day = atoi(ds).map_err(|e| fail(format!("invalid Day: {e}")))?;
    }
    let flag = |key: &str| -> Result<Option<bool>, String> {
        match m.get(key) {
            None => Ok(None),
            Some(v) => parse_data_bool(v).map(Some).map_err(|e| fail(format!("{key}: {e}"))),
        }
    };
    if let Some(v) = flag("HasOctave")? {
        f.has_octave = v;
    }
    if let Some(v) = flag("HasVigil")? {
        f.has_vigil = v;
    }
    if let Some(v) = m.get("OctaveClass") {
        f.octave_class = OctaveClass::parse(v).ok_or_else(|| fail(format!("invalid OctaveClass {}", quote(v))))?;
    }
    if let Some(v) = m.get("CommemorationClass") {
        f.commemoration_class = CommemorationClass::parse(v).ok_or_else(|| fail(format!("invalid CommemorationClass {}", quote(v))))?;
        if f.commemoration_class != CommemorationClass::Default && !f.is_category(Category::Feria) {
            return Err(fail("CommemorationClass requires feria category".to_string()));
        }
    }
    if let Some(v) = flag("PrimaryOfOurLord")? {
        f.primary_of_our_lord = v;
        if v && (f.rank != Rank::Double1stClass || !f.is_category(Category::Lord)) {
            return Err(fail("PrimaryOfOurLord requires a Double I Class feast of Our Lord".to_string()));
        }
    }
    if let Some(v) = flag("IsVigil")? {
        f.is_vigil = v;
    }
    let text = |key: &str| m.get(key).and_then(crate::model::non_empty);
    if m.get("VigilOf").is_some() {
        f.vigil_of = text("VigilOf");
    }
    if m.get("CompanionOf").is_some() {
        f.companion_of = text("CompanionOf");
    }
    if m.get("OnlyWith").is_some() {
        f.only_with = text("OnlyWith");
    }
    if let Some(v) = flag("SkipRomanLeapShift")? {
        f.skip_roman_leap_shift = v;
    }
    if m.get("Source").is_some() {
        f.source = text("Source");
    }
    if m.get("Notes").is_some() {
        f.notes = text("Notes");
    }
    if m.get("ProperName").is_some() {
        f.proper_name = text("ProperName");
    }
    if m.get("ProperID").is_some() {
        f.proper_id = text("ProperID");
    }

    // Report the first unknown key in byte order.
    if let Some(key) = m.values.keys().find(|k| !KNOWN_FEAST_KEYS.contains(&k.as_str())) {
        return Err(fail(format!("unrecognized key {}", quote(key))));
    }

    if (month == 0) != (day == 0) {
        return Err(format!("{source_file}: feast {qid} must specify Month and Day together"));
    }
    let has_fixed = month != 0 && day != 0;
    let has_rule = f.date_rule.is_some();
    if !has_fixed && !has_rule {
        return Err(format!("{source_file}: feast {qid} must have either Month/Day or DateRule"));
    }
    if has_fixed && has_rule {
        return Err(format!("{source_file}: feast {qid} must not have both Month/Day and DateRule"));
    }
    if has_fixed && !valid_fixed_date(month, day) {
        return Err(format!("{source_file}: feast {qid} has invalid fixed date {month}/{day}"));
    }
    if has_fixed {
        f.fixed = Some(MonthDay { month: month as u32, day: day as u32 });
    }
    if f.is_vigil && !f.is_category(Category::Feria) {
        let cat = f.category.map_or("", Category::as_str);
        return Err(format!("{source_file}: feast {qid} is a vigil but has category {} instead of feria", quote(cat)));
    }
    if f.is_vigil && f.vigil_of.is_none() {
        return Err(format!("{source_file}: feast {qid} is a vigil but does not specify VigilOf"));
    }
    if !f.is_vigil && f.vigil_of.is_some() {
        return Err(format!("{source_file}: feast {qid} specifies VigilOf but is not a vigil"));
    }
    if f.octave_class != OctaveClass::Common && !f.has_octave && !(f.octave_class == OctaveClass::Simple && f.rank == Rank::Simple) {
        return Err(format!("{source_file}: feast {qid} specifies OctaveClass without HasOctave (except a Simple octave day)"));
    }
    Ok(f)
}

/// Loads and merges every feast file under `feasts/`, skipping missing files.
pub fn load_feasts(src: &dyn DataSource) -> Result<Vec<FeastRef>, String> {
    let mut feasts = Vec::new();
    for fname in FEAST_FILES {
        let rel = format!("feasts/{fname}");
        let Some(content) = src.read(&rel)? else { continue };
        let sections = parse_ini_sections(&src.display_path(&rel), &content).map_err(|e| format!("parsing {fname}: {e}"))?;
        for section in &sections {
            feasts.push(Arc::new(section_to_feast(section, fname)?));
        }
    }
    Ok(feasts)
}

/// Loads the penitential rules file, which must exist.
pub fn load_penitential_rules(src: &dyn DataSource) -> Result<Vec<PenitentialRule>, String> {
    let path = src.display_path(PENITENTIAL_RULES_FILE);
    let content = src.read(PENITENTIAL_RULES_FILE)?.ok_or_else(|| format!("parsing {PENITENTIAL_RULES_FILE}: {path} does not exist"))?;
    let sections = parse_ini_sections(&path, &content).map_err(|e| format!("parsing {PENITENTIAL_RULES_FILE}: {e}"))?;
    sections.iter().map(|s| section_to_penitential_rule(s, PENITENTIAL_RULES_FILE)).collect()
}

/// Everything the calendar builder reads from the data directory.
#[derive(Clone, Debug)]
pub struct CalendarData {
    pub feasts: Vec<FeastRef>,
    pub penitential_rules: Vec<PenitentialRule>,
}

impl CalendarData {
    pub fn load(src: &dyn DataSource) -> Result<CalendarData, String> {
        let feasts = load_feasts(src).map_err(|e| format!("loading feasts: {e}"))?;
        let penitential_rules = load_penitential_rules(src).map_err(|e| format!("loading penitential rules: {e}"))?;
        Ok(CalendarData { feasts, penitential_rules })
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn section(text: &str) -> Section {
        parse_ini_sections("test.txt", text).unwrap().remove(0)
    }

    #[test]
    fn parses_sections_and_comments() {
        let sections = parse_ini_sections("f.txt", "# c\n\n[a]\nName = A = B\n  Rank=double \n[b]\n").unwrap();
        assert_eq!(sections.len(), 2);
        assert_eq!(sections[0].get("Name"), Some("A = B"));
        assert_eq!(sections[0].get("Rank"), Some("double"));
        assert_eq!(sections[1].id, "b");
    }

    #[test]
    fn syntax_errors() {
        assert_eq!(parse_ini_sections("f.txt", "Name = x\n").unwrap_err(), "f.txt:1: key-value pair outside of section");
        assert_eq!(parse_ini_sections("f.txt", "[a]\n\nnonsense\n").unwrap_err(), "f.txt:3: expected Key = value, got \"nonsense\"");
    }

    #[test]
    fn fixed_feast() {
        let f = section_to_feast(
            &section("[st-x]\nName = St X\nRank = double\nColor = red\nCategory = martyr\nMonth = 2\nDay = 29\n"),
            "s.txt",
        )
        .unwrap();
        assert_eq!(f.fixed, Some(MonthDay { month: 2, day: 29 }));
        assert_eq!(f.source.as_deref(), Some("base"));
        assert_eq!(f.category, Some(Category::Martyr));
    }

    #[test]
    fn schema_errors() {
        let cases = [
            ("[x]\nRank = double\nColor = red\nMonth = 1\nDay = 1\n", "s.txt: feast \"x\" missing Name"),
            ("[x]\nName = X\nRank = big\nColor = red\n", "s.txt: feast \"x\": invalid rank: \"big\""),
            ("[x]\nName = X\nRank = double\nColor = red\nMonth = 1\n", "s.txt: feast \"x\" must specify Month and Day together"),
            ("[x]\nName = X\nRank = double\nColor = red\n", "s.txt: feast \"x\" must have either Month/Day or DateRule"),
            ("[x]\nName = X\nRank = double\nColor = red\nMonth = 2\nDay = 30\n", "s.txt: feast \"x\" has invalid fixed date 2/30"),
            (
                "[x]\nName = X\nRank = double\nColor = red\nMonth = 1\nDay = 1\nBogus = 1\n",
                "s.txt: feast \"x\": unrecognized key \"Bogus\"",
            ),
            (
                "[x]\nName = X\nRank = double\nColor = red\nMonth = 1\nDay = 1\nHasOctave = yes\n",
                "s.txt: feast \"x\": HasOctave: expected true or false, got \"yes\"",
            ),
            (
                "[x]\nName = X\nRank = simple\nColor = red\nCategory = martyr\nMonth = 1\nDay = 1\nIsVigil = true\n",
                "s.txt: feast \"x\" is a vigil but has category \"martyr\" instead of feria",
            ),
            (
                "[x]\nName = X\nRank = double\nColor = red\nMonth = 1\nDay = 1\nOctaveClass = simple\n",
                "s.txt: feast \"x\" specifies OctaveClass without HasOctave (except a Simple octave day)",
            ),
            (
                "[x]\nName = X\nRank = double\nColor = red\nCategory = lord\nMonth = 1\nDay = 1\nPrimaryOfOurLord = true\n",
                "s.txt: feast \"x\": PrimaryOfOurLord requires a Double I Class feast of Our Lord",
            ),
        ];
        for (text, want) in cases {
            assert_eq!(section_to_feast(&section(text), "s.txt").unwrap_err(), want);
        }
    }
}
