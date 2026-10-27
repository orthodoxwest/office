//! Chant scores for sung texts: GABC files under `data/texts/chant/`, in a directory named by the
//! corpus key of the text they set and a file named by the tune
//! (`chant/ordinary/compline/hymn/ferial.gabc` sets `ordinary/compline/hymn` to its ferial tune;
//! one text takes several tunes by season and rank). A hymn's score sets its first stanza or
//! every stanza; a Marian antiphon's sets the antiphon, up to its ℣. The composed hour carries a
//! score only on an element whose sung words are the score's own, so a corpus edit or a composed
//! change of wording (a doxology swapped by season) falls back to the text rather than
//! underlaying the wrong words.
//!
//! Which tune is sung when awaits a tune table (a data file the engine reads, with Jacob's "sung
//! when" tags): until then only an entry with a single tune is sung, and validation refuses an
//! entry with several, since guessing one would sound a ferial melody on a feast.
//!
//! The booklet's slug-named scores one level under `psalms/`, `canticles/` and `hymns/`
//! (`chant/psalms/067.gabc`) are the booklet's own and are not loaded here.

use std::collections::BTreeMap;
use std::sync::Arc;

use calendar::DataSource;
use corpus::lines::{hymn_rubric_stanza, hymn_sung_stanzas, marian_antiphon_part, parse_hymn};
use liturgy::{ChantScore, ElementType, OfficeElement, OfficeHour, ScoreSpan};

use crate::texts::OfficeTexts;

/// The directory under `data/texts/` that holds scores.
pub const CHANT_DIR: &str = "chant/";

/// The key prefix of the Marian antiphons.
const MARIAN_PREFIX: &str = "ordinary/marian/";

/// One score: its GABC, the words it sets, and what neuma could not read.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Score {
    pub gabc: Arc<str>,
    /// The sung words, normalized by [`words`].
    pub words: Vec<String>,
    /// neuma's error diagnostics; a score with any fails validation.
    pub errors: Vec<String>,
}

impl Score {
    pub fn new(gabc: &str) -> Score {
        let chant = neuma::Chant::new(gabc);
        let errors = chant
            .diagnostics()
            .iter()
            .filter(|d| d.severity == neuma::Severity::Error)
            .map(|d| format!("{}: {}", d.code, d.message))
            .collect();
        Score { gabc: Arc::from(gabc), words: words(&chant.summary().text), errors }
    }
}

/// What kind of sung text a corpus key names, if it is one the apps offer as chant.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum SungKind {
    Hymn,
    MarianAntiphon,
}

impl SungKind {
    /// A hymn is a `hymn…` section; a Marian antiphon is one of `ordinary/marian/`.
    pub fn of_key(key: &str) -> Option<SungKind> {
        if key.starts_with(MARIAN_PREFIX) {
            return Some(SungKind::MarianAntiphon);
        }
        key.rsplit('/').next().is_some_and(|last| last.starts_with("hymn")).then_some(SungKind::Hymn)
    }

    /// What of `text` a score with these words sets, if it sets exactly a span of it: a hymn's
    /// first stanza or every stanza (rubric stanzas aside), or a Marian antiphon's antiphon.
    pub fn span(self, text: &str, words_set: &[String]) -> Option<ScoreSpan> {
        let spans: Vec<(ScoreSpan, Vec<String>)> = match self {
            SungKind::Hymn => {
                let hymn = parse_hymn(text);
                let stanzas: Vec<&Vec<String>> = hymn.stanzas.iter().filter(|s| hymn_rubric_stanza(s).is_none()).collect();
                let first = stanzas.first().map_or_else(Vec::new, |s| words(&s.join(" ")));
                let whole = stanzas.iter().flat_map(|s| words(&s.join(" "))).collect();
                vec![(ScoreSpan::FirstStanza, first), (ScoreSpan::WholeHymn, whole)]
            }
            SungKind::MarianAntiphon => vec![(ScoreSpan::Antiphon, words(marian_antiphon_part(text)))],
        };
        spans.into_iter().find(|(_, w)| !w.is_empty() && w == words_set).map(|(span, _)| span)
    }

    /// The words a score of this kind is compared with first, for a validation message.
    fn first_span_words(self, text: &str) -> Vec<String> {
        match self {
            SungKind::Hymn => {
                let hymn = parse_hymn(text);
                hymn.stanzas.iter().find(|s| hymn_rubric_stanza(s).is_none()).map_or_else(Vec::new, |s| words(&s.join(" ")))
            }
            SungKind::MarianAntiphon => words(marian_antiphon_part(text)),
        }
    }
}

/// Words for comparing a score with its text: lower case, each word's letters and digits only, so
/// punctuation, the pointing asterisk, quotes and hyphens never count.
pub fn words(text: &str) -> Vec<String> {
    text.split_whitespace()
        .map(|w| w.chars().filter(|c| c.is_alphanumeric()).flat_map(char::to_lowercase).collect::<String>())
        .filter(|w| !w.is_empty())
        .collect()
}

/// The corpus key and tune a path under `data/texts/` names, if it is an app score (not the
/// booklet's).
pub fn score_key(rel: &str) -> Option<(&str, &str)> {
    let path = rel.strip_prefix(CHANT_DIR)?.strip_suffix(".gabc")?;
    let (key, tune) = path.rsplit_once('/')?;
    let booklet = ["psalms", "canticles", "hymns"].contains(&key);
    (!booklet && !key.is_empty() && !tune.is_empty()).then_some((key, tune))
}

/// Every app score under `data/texts/chant/`, by key and then tune.
pub fn load_scores(src: &dyn DataSource) -> Result<BTreeMap<String, BTreeMap<String, Score>>, String> {
    let mut scores: BTreeMap<String, BTreeMap<String, Score>> = BTreeMap::new();
    for (rel, bytes) in src.walk("texts").map_err(|e| format!("loading chant: {e}"))? {
        let Some((key, tune)) = score_key(&rel) else { continue };
        let gabc =
            String::from_utf8(bytes).map_err(|_| format!("loading chant: {}: invalid UTF-8", src.display_path(&format!("texts/{rel}"))))?;
        scores.entry(key.to_string()).or_default().insert(tune.to_string(), Score::new(&gabc));
    }
    Ok(scores)
}

/// The tune an entry is sung to: its only one, until a tune table says which of several.
fn sung_tune(tunes: &BTreeMap<String, Score>) -> Option<&Score> {
    if tunes.len() == 1 { tunes.values().next() } else { None }
}

/// Whether any hymn or Marian antiphon has a score to offer: until one does, the fronts keep the
/// Chant setting out of sight.
pub fn offers_chant(t: &OfficeTexts) -> bool {
    t.scores.iter().any(|(key, tunes)| SungKind::of_key(key).is_some() && sung_tune(tunes).is_some_and(|s| s.errors.is_empty()))
}

/// Gives each hymn and Marian antiphon its score, where one sets exactly its words.
pub fn attach_scores(hour: &mut OfficeHour, t: &OfficeTexts) {
    if t.scores.is_empty() {
        return;
    }
    for elem in hour.sections.iter_mut().flat_map(|s| s.elements.iter_mut()) {
        elem.score = score_for(elem, t);
    }
}

/// The score that sets an element's sung words, if any.
fn score_for(elem: &OfficeElement, t: &OfficeTexts) -> Option<ChantScore> {
    let wanted = if elem.kind == ElementType::Hymn {
        SungKind::Hymn
    } else if elem.kind == ElementType::Antiphon && elem.slot_ref == "marian-antiphon" {
        SungKind::MarianAntiphon
    } else {
        return None;
    };
    let key = t.canonical_ref(&elem.source_ref)?;
    let score = t.scores.get(key).and_then(sung_tune).filter(|s| s.errors.is_empty())?;
    if SungKind::of_key(key) != Some(wanted) {
        return None;
    }
    let sets = wanted.span(&elem.text, &score.words)?;
    Some(ChantScore { gabc: score.gabc.clone(), sets })
}

/// Problems with the scores: a key that names no text or an alias, a text that is not a hymn or
/// Marian antiphon, several tunes before a tune table can choose among them, GABC neuma cannot
/// read, words that differ from the text's, or a whole hymn whose double bars `(::)` do not part
/// it stanza by stanza (the fronts would then show one long score, unnumbered).
pub fn validate_scores(t: &OfficeTexts) -> Vec<String> {
    let mut errs = Vec::new();
    for (key, tunes) in &t.scores {
        if tunes.len() > 1 {
            errs.push(format!("texts/{CHANT_DIR}{key}/: several tunes, and no tune table yet says which is sung when"));
        }
        for (tune, score) in tunes {
            let path = format!("texts/{CHANT_DIR}{key}/{tune}.gabc");
            for e in &score.errors {
                errs.push(format!("{path}: {e}"));
            }
            let Some(kind) = SungKind::of_key(key) else {
                errs.push(format!("{path}: scores are offered only for hymns and Marian antiphons, not {key}"));
                continue;
            };
            match t.canonical_ref(key) {
                None => errs.push(format!("{path}: no corpus text {key}")),
                Some(canonical) if canonical != key => errs.push(format!("{path}: {key} is an alias; name the score {canonical}")),
                Some(_) => match kind.span(t.get(key), &score.words) {
                    None => {
                        let want = kind.first_span_words(t.get(key));
                        errs.push(format!("{path}: its words differ from the text's: {}", first_difference(&score.words, &want)));
                    }
                    Some(ScoreSpan::WholeHymn) => {
                        let parts = ChantScore { gabc: score.gabc.clone(), sets: ScoreSpan::WholeHymn }.stanza_parts().len();
                        let stanzas = hymn_sung_stanzas(t.get(key)).len();
                        if parts != stanzas {
                            errs.push(format!("{path}: its double bars part it into {parts} stanzas, but the text has {stanzas}"));
                        }
                    }
                    Some(_) => {}
                },
            }
        }
    }
    errs
}

/// Where two word lists part, for a validation message.
fn first_difference(got: &[String], want: &[String]) -> String {
    let at = got.iter().zip(want).take_while(|(g, w)| g == w).count();
    let show = |ws: &[String]| if at < ws.len() { format!("\"{}\"", ws[at]) } else { "the end".to_string() };
    format!("word {} is {} in the score but {} in the text", at + 1, show(got), show(want))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn words_ignore_case_punctuation_and_pointing() {
        assert_eq!(words("To thee, * before the close of day,"), ["to", "thee", "before", "the", "close", "of", "day"]);
        assert_eq!(words("Heaven’s gate-way; O!"), ["heavens", "gateway", "o"]);
    }

    #[test]
    fn keys_name_corpus_entries() {
        assert_eq!(score_key("chant/ordinary/compline/hymn/ferial.gabc"), Some(("ordinary/compline/hymn", "ferial")));
        assert_eq!(score_key("ordinary/compline.txt"), None);
        assert_eq!(score_key("chant/psalms/134.gabc"), None, "the booklet's own");
        assert_eq!(score_key("chant/hymns/te-lucis.gabc"), None, "the booklet's own");
        assert_eq!(SungKind::of_key("ordinary/compline/hymn"), Some(SungKind::Hymn));
        assert_eq!(SungKind::of_key("shared/blessed-virgin/hymn-vespers"), Some(SungKind::Hymn));
        assert_eq!(SungKind::of_key("ordinary/marian/salve-regina"), Some(SungKind::MarianAntiphon));
        assert_eq!(SungKind::of_key("psalms/134"), None);
    }

    #[test]
    fn a_score_sets_the_first_stanza_every_stanza_or_the_antiphon() {
        let hymn = "Te lucis ante terminum\n\n/:Kneel.:/\n\nTo thee, before\nthe close of day,\n\nFrom all ill dreams.\n\nAmen.";
        let span = |w: &str| SungKind::Hymn.span(hymn, &words(w));
        assert_eq!(span("To thee before the close of day"), Some(ScoreSpan::FirstStanza));
        assert_eq!(span("To thee before the close of day from all ill dreams amen"), Some(ScoreSpan::WholeHymn));
        assert_eq!(span("To thee before the close"), None);
        let marian = "Mary we hail thee * Mother\nand Queen.\n\nV. Pray for us.\nR. That we.";
        assert_eq!(SungKind::MarianAntiphon.span(marian, &words("Mary we hail thee Mother and Queen")), Some(ScoreSpan::Antiphon));
    }

    #[test]
    fn a_score_reads_its_words_and_errors() {
        let score = Score::new("name: t;\n%%\n(c4) To(g) thee,(h) be(j)fore(h) the(g) close(h) of(g) day.(g.) (::)");
        assert!(score.errors.is_empty(), "{:?}", score.errors);
        assert_eq!(score.words, words("To thee, before the close of day."));
    }
}

#[cfg(test)]
mod engine_tests {
    use calendar::{CalendarData, MoveableDates, build_calendar};
    use corpus::lines::{hymn_rubric_stanza, parse_hymn};
    use liturgy::{ElementType, PrayerForm, ScoreSpan};

    use super::Score;
    use crate::{Day, Engine, resolve_office_days, testutil::TestData};

    const KEY: &str = "ordinary/compline/hymn";

    /// A placeholder melody on one note for `words`, one syllable to a word.
    fn gabc(words: &str) -> String {
        let notes: Vec<String> = words.split_whitespace().map(|w| format!("{w}(g)")).collect();
        format!("name: test;\n%%\n(c4) {} (::)", notes.join(" "))
    }

    fn compline_hymn(engine: &Engine) -> liturgy::OfficeElement {
        let data = CalendarData::load(&TestData("../../data".into())).unwrap();
        let cal = build_calendar(2026, &data).unwrap();
        let offices = resolve_office_days(&cal);
        let day = Day::new(cal.days[279].clone(), offices[279].clone());
        let hour = engine.compose_hour("compline", &day, &MoveableDates::compute(2026), PrayerForm::Private).unwrap();
        let hymn = hour.sections.into_iter().flat_map(|s| s.elements).find(|e| e.kind == ElementType::Hymn).unwrap();
        assert_eq!(engine.texts.canonical_ref(&hymn.source_ref), Some(KEY), "{}", hymn.source_ref);
        hymn
    }

    #[test]
    fn a_score_rides_on_the_hymn_whose_words_it_sets() {
        let mut engine = Engine::load(&TestData("../../data".into())).unwrap();
        let text = engine.texts.get(KEY).to_string();
        let hymn = parse_hymn(&text);
        let first = hymn.stanzas.iter().find(|s| hymn_rubric_stanza(s).is_none()).unwrap().join(" ");
        let set = |engine: &mut Engine, words: &str| {
            engine.texts.scores.insert(KEY.into(), [("ferial".to_string(), Score::new(&gabc(words)))].into());
        };

        set(&mut engine, &first);
        assert!(super::offers_chant(&engine.texts));
        assert!(super::validate_scores(&engine.texts).is_empty(), "{:?}", super::validate_scores(&engine.texts));
        let score = compline_hymn(&engine).score.expect("scored");
        assert_eq!(score.sets, ScoreSpan::FirstStanza);

        // A word changed in the text, or a second tune with no table to choose it, and the hymn
        // goes back to its words.
        set(&mut engine, &first.replacen("close", "end", 1));
        assert!(compline_hymn(&engine).score.is_none());
        assert!(super::validate_scores(&engine.texts)[0].contains("word 5 is \"end\" in the score but \"close\" in the text"));
        set(&mut engine, &first);
        engine.texts.scores.get_mut(KEY).unwrap().insert("festal".into(), Score::new(&gabc(&first)));
        assert!(compline_hymn(&engine).score.is_none());
        assert!(super::validate_scores(&engine.texts)[0].contains("several tunes"));
    }

    #[test]
    fn a_whole_hymns_score_parts_at_each_stanza() {
        let mut engine = Engine::load(&TestData("../../data".into())).unwrap();
        let stanzas = corpus::lines::hymn_sung_stanzas(engine.texts.get(KEY));
        assert!(stanzas.len() > 1);
        let set = |engine: &mut Engine, gabc: String| {
            engine.texts.scores.insert(KEY.into(), [("ferial".to_string(), Score::new(&gabc))].into());
        };

        set(&mut engine, gabc(&stanzas.join(" ")));
        let errs = super::validate_scores(&engine.texts);
        assert!(errs.len() == 1 && errs[0].contains(&format!("into 1 stanzas, but the text has {}", stanzas.len())), "{errs:?}");

        let parted: Vec<String> = stanzas.iter().map(|s| gabc(s).split_once("(c4) ").unwrap().1.to_string()).collect();
        set(&mut engine, format!("name: test;\n%%\n(c4) {}", parted.join(" ")));
        assert!(super::validate_scores(&engine.texts).is_empty(), "{:?}", super::validate_scores(&engine.texts));
    }
}
