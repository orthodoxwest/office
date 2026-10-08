//! The INI-like feast and penitential data files. The crate reads nothing
//! itself: callers hand in file contents through [`DataSource`].

use std::collections::BTreeMap;
use std::sync::Arc;

use crate::date::Date;
use crate::model::{Category, Color, CommemorationClass, Feast, FeastRef, MonasticObservance, MonthDay, OctaveClass, Rank};
use crate::penitential::{PenitentialRule, section_to_penitential_rule};
use data_format::atoi;
use data_format::quote;

/// The feast definition files, in load order.
pub const FEAST_FILES: [&str; 4] = ["temporal.txt", "sanctoral.txt", "awrv.txt", "commemorations.txt"];

/// Observances the ordo brackets for monastics and oblates. Not a feast file: nothing in it
/// enters occurrence.
pub const MONASTIC_FILE: &str = "feasts/monastic.txt";

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

/// Per-day octave name keys, `OctaveDay2` through `OctaveDay8`.
const OCTAVE_DAY_KEYS: [&str; 7] = ["OctaveDay2", "OctaveDay3", "OctaveDay4", "OctaveDay5", "OctaveDay6", "OctaveDay7", "OctaveDay8"];

const KNOWN_FEAST_KEYS: [&str; 24] = [
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
    "OctaveOf",
    "OctaveDays",
    "CommemorationClass",
    "OctaveClass",
    "PrimaryOfOurLord",
    "Secondary",
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
    if let Some(v) = flag("Secondary")? {
        f.secondary = v;
        if v && f.primary_of_our_lord {
            return Err(fail("Secondary contradicts PrimaryOfOurLord".to_string()));
        }
    }
    if let Some(v) = flag("IsVigil")? {
        f.is_vigil = v;
    }
    let text = |key: &str| m.get(key).and_then(crate::model::non_empty);
    if m.get("VigilOf").is_some() {
        f.vigil_of = text("VigilOf");
    }
    if m.get("OctaveOf").is_some() {
        f.octave_of = text("OctaveOf");
    }
    if let Some(pattern) = text("OctaveDays") {
        if pattern.replace("{n}", "").replace("{weekday}", "").contains(['{', '}']) {
            return Err(fail(format!("OctaveDays: unknown placeholder in {}", quote(&pattern))));
        }
        f.octave_days = Some(pattern);
    }
    for (day, key) in (2..).zip(OCTAVE_DAY_KEYS) {
        if let Some(name) = text(key) {
            f.octave_day_names.insert(day, name);
        }
    }
    if (f.octave_days.is_some() || !f.octave_day_names.is_empty()) && !f.has_octave {
        return Err(fail("OctaveDays and OctaveDay2–OctaveDay8 require HasOctave = true".to_string()));
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
    if let Some(key) = m.values.keys().find(|k| !KNOWN_FEAST_KEYS.contains(&k.as_str()) && !OCTAVE_DAY_KEYS.contains(&k.as_str())) {
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

const KNOWN_MONASTIC_KEYS: [&str; 7] = ["Name", "Rank", "Month", "Day", "Office", "Source", "Notes"];

/// Converts a parsed section into a monastic observance.
pub fn section_to_monastic(m: &Section, source_file: &str) -> Result<MonasticObservance, String> {
    let qid = quote(&m.id);
    let fail = |e: String| format!("{source_file}: observance {qid}: {e}");
    if m.id.is_empty() {
        return Err(format!("{source_file}: section missing ID"));
    }
    if let Some(key) = m.values.keys().find(|k| !KNOWN_MONASTIC_KEYS.contains(&k.as_str())) {
        return Err(fail(format!("unrecognized key {}", quote(key))));
    }
    let name = m.value("Name").to_string();
    if name.is_empty() {
        return Err(fail("missing Name".to_string()));
    }
    let rank = Rank::parse(m.value("Rank")).map_err(fail)?;
    let month = atoi(m.value("Month")).map_err(|e| fail(format!("invalid Month: {e}")))?;
    let day = atoi(m.value("Day")).map_err(|e| fail(format!("invalid Day: {e}")))?;
    if !valid_fixed_date(month, day) {
        return Err(fail(format!("invalid fixed date {month}/{day}")));
    }
    Ok(MonasticObservance {
        id: m.id.clone(),
        name,
        rank,
        fixed: MonthDay { month: month as u32, day: day as u32 },
        office: m.get("Office").and_then(crate::model::non_empty),
    })
}

/// Loads the monastic observances, none when the file is absent.
pub fn load_monastic(src: &dyn DataSource) -> Result<Vec<MonasticObservance>, String> {
    let Some(content) = src.read(MONASTIC_FILE)? else { return Ok(Vec::new()) };
    let sections = parse_ini_sections(&src.display_path(MONASTIC_FILE), &content).map_err(|e| format!("parsing {MONASTIC_FILE}: {e}"))?;
    sections.iter().map(|s| section_to_monastic(s, MONASTIC_FILE)).collect()
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
    pub monastic: Vec<MonasticObservance>,
}

impl CalendarData {
    pub fn load(src: &dyn DataSource) -> Result<CalendarData, String> {
        let feasts = load_feasts(src).map_err(|e| format!("loading feasts: {e}"))?;
        let penitential_rules = load_penitential_rules(src).map_err(|e| format!("loading penitential rules: {e}"))?;
        let monastic = load_monastic(src).map_err(|e| format!("loading monastic observances: {e}"))?;
        Ok(CalendarData { feasts, penitential_rules, monastic })
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parses_sections_and_comments() {
        let sections = parse_ini_sections("f.txt", "# c\n\n[a]\nName = A = B\n  Rank=double \n[b]\n").unwrap();
        assert_eq!(sections.len(), 2);
        assert_eq!(sections[0].get("Name"), Some("A = B"));
        assert_eq!(sections[0].get("Rank"), Some("double"));
        assert_eq!(sections[1].id, "b");
        assert_eq!(parse_ini_sections("f.txt", "Name = x\n").unwrap_err(), "f.txt:1: key-value pair outside of section");
        assert_eq!(parse_ini_sections("f.txt", "[a]\n\nnonsense\n").unwrap_err(), "f.txt:3: expected Key = value, got \"nonsense\"");
    }

    #[test]
    fn monastic_observances_parse_and_reject_feast_keys() {
        let parse = |raw: &str| section_to_monastic(&parse_ini_sections("m.txt", raw).unwrap()[0], "m.txt");
        let o =
            parse("[x]\nName = Solemnity of St Benedict\nRank = greater-double\nMonth = 7\nDay = 11\nOffice = Proper Office\n").unwrap();
        assert_eq!(o.heading(), "Solemnity of St Benedict (Monastics & Oblates Only)");
        assert_eq!((o.rank, o.fixed, o.office.as_deref()), (Rank::GreaterDouble, MonthDay { month: 7, day: 11 }, Some("Proper Office")));
        assert_eq!(
            parse("[x]\nName = X\nRank = double\nMonth = 7\nDay = 32\n").unwrap_err(),
            "m.txt: observance \"x\": invalid fixed date 7/32"
        );
        assert_eq!(
            parse("[x]\nName = X\nRank = double\nMonth = 7\nDay = 1\nHasOctave = true\n").unwrap_err(),
            "m.txt: observance \"x\": unrecognized key \"HasOctave\""
        );
    }
}
