//! Builders for synthetic calendar days, feasts, and text corpora used in unit tests.

use std::sync::Arc;

use calendar::model::Penitential;
use calendar::{CalendarDay, Category, Color, Date, Feast, FeastRef, Rank, Season};
use corpus::Corpus;

use crate::concurrence::VespersDesignation;
use crate::day::Day;
use crate::texts::OfficeTexts;

/// Concrete corpus entries, with no aliases or scopes.
pub fn texts(entries: &[(&str, &str)]) -> OfficeTexts {
    OfficeTexts::from_corpus(Corpus::from_entries(entries.iter().map(|(k, v)| (k.to_string(), v.to_string()))))
}

pub fn date(y: i32, m: i32, d: i32) -> Date {
    Date::new(y, m, d)
}

/// A day with no celebration, using white as its color.
pub fn day(date: Date, season: Season) -> Day {
    Day {
        cal: CalendarDay {
            date,
            season,
            tempora: None,
            celebration: None,
            commemorations: Vec::new(),
            color: Color::White,
            notes: None,
            resolution_rule: String::new(),
            occurrence_decisions: Vec::new(),
            feria_commemoration: None,
            temporal_week_id: None,
            within_octave_of: None,
            penitential: Penitential::default(),
        },
        marian_antiphon: String::new(),
        vespers: VespersDesignation::unowned(),
        first_vespers: false,
        following_office_commemoration_id: String::new(),
    }
}

/// A semi-double feast with the given ID and category.
pub fn feast(id: &str, category: Option<Category>) -> Feast {
    let mut f = Feast::synthetic(id, "", Rank::SemiDouble, Color::White, Category::Feria);
    f.category = category;
    f
}

pub fn arc(f: Feast) -> FeastRef {
    Arc::new(f)
}

/// A day celebrating `f`.
pub fn celebrating(date: Date, season: Season, f: Feast) -> Day {
    let mut d = day(date, season);
    d.celebration = Some(arc(f));
    d
}

/// A fixed Monday: 0001-01-01.
pub fn zero_date() -> Date {
    Date::new(1, 1, 1)
}

/// A test-only reader of the repository's `data/` directory.
pub struct TestData(pub std::path::PathBuf);

impl calendar::DataSource for TestData {
    fn display_path(&self, rel: &str) -> String {
        self.0.join(rel).display().to_string()
    }
    fn read(&self, rel: &str) -> Result<Option<String>, String> {
        match std::fs::read_to_string(self.0.join(rel)) {
            Ok(s) => Ok(Some(s)),
            Err(e) if e.kind() == std::io::ErrorKind::NotFound => Ok(None),
            Err(e) => Err(e.to_string()),
        }
    }
    fn walk(&self, rel: &str) -> Result<Vec<(String, Vec<u8>)>, String> {
        fn visit(dir: &std::path::Path, rel: &str, out: &mut Vec<(String, Vec<u8>)>) -> std::io::Result<()> {
            let mut names: Vec<_> = std::fs::read_dir(dir)?.map(|e| e.map(|e| e.file_name())).collect::<Result<_, _>>()?;
            names.sort();
            for name in names {
                let path = dir.join(&name);
                let child = if rel.is_empty() { name.to_string_lossy().to_string() } else { format!("{rel}/{}", name.to_string_lossy()) };
                if path.is_dir() {
                    visit(&path, &child, out)?;
                } else {
                    out.push((child, std::fs::read(&path)?));
                }
            }
            Ok(())
        }
        let mut out = Vec::new();
        visit(&self.0.join(rel), "", &mut out).map_err(|e| e.to_string())?;
        Ok(out)
    }
}

/// The live corpus, loaded once per test binary.
pub fn live_texts() -> &'static OfficeTexts {
    static TEXTS: std::sync::OnceLock<OfficeTexts> = std::sync::OnceLock::new();
    TEXTS.get_or_init(|| crate::texts::load_texts(&TestData(std::path::PathBuf::from("../../data"))).expect("loading data/"))
}
