//! Loading the text corpus the Office reads: `data/texts/`, the sidecars, and
//! the appointment scopes (Go's `texts.LoadTexts`).

use calendar::DataSource;
use corpus::{Corpus, Sidecar, TextFile};

use crate::scopes::AppointmentScopes;

/// The corpus and the appointment scopes. `scopes` is `None` when
/// `appointment-scopes.json` does not exist, which is distinct from `[]`.
#[derive(Clone, Debug)]
pub struct OfficeTexts {
    pub corpus: Corpus,
    pub scopes: Option<AppointmentScopes>,
}

/// Loads `data/texts/` and its sidecars. Errors match Go's `LoadTexts`.
pub fn load_texts(src: &dyn DataSource) -> Result<OfficeTexts, String> {
    let mut files = Vec::new();
    for (rel, bytes) in src.walk("texts").map_err(|e| format!("loading texts: {e}"))? {
        if !rel.rsplit('/').next().is_some_and(|n| n.ends_with(".txt")) {
            continue;
        }
        // PORT(inherited): Go reads any bytes; the corpus is UTF-8 throughout.
        let content =
            String::from_utf8(bytes).map_err(|_| format!("loading texts: {}: invalid UTF-8", src.display_path(&format!("texts/{rel}"))))?;
        files.push(TextFile { rel_path: rel, content });
    }
    let conclusions_path = src.display_path("collect-conclusions.txt");
    let conclusions = src.read("collect-conclusions.txt")?;
    let incipits_path = src.display_path("latin-incipits.txt");
    let incipits = src.read("latin-incipits.txt")?;
    let corpus = Corpus::load(
        &files,
        conclusions.as_deref().map(|content| Sidecar { path: &conclusions_path, content }),
        incipits.as_deref().map(|content| Sidecar { path: &incipits_path, content }),
    )?;
    let scopes_path = src.display_path("appointment-scopes.json");
    let scopes = match src.read("appointment-scopes.json")? {
        None => None,
        Some(raw) => Some(AppointmentScopes::load(&scopes_path, &raw, &corpus)?),
    };
    Ok(OfficeTexts { corpus, scopes })
}

impl std::ops::Deref for OfficeTexts {
    type Target = Corpus;
    fn deref(&self) -> &Corpus {
        &self.corpus
    }
}

impl OfficeTexts {
    /// A corpus without appointment scopes, for tests.
    pub fn from_corpus(corpus: Corpus) -> OfficeTexts {
        OfficeTexts { corpus, scopes: None }
    }

    /// The appointment scope governing a seasonal fallback lookup, if any.
    pub fn seasonal_appointment_scope(&self, season: calendar::Season, hour: &str, slot: &str) -> Option<&crate::scopes::AppointmentScope> {
        self.scopes.as_ref()?.seasonal(season, hour, slot)
    }
}
