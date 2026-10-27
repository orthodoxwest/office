//! A feast's own texts for its page: the Gospel-canticle antiphons, the versicles and the collect,
//! each from the feast's proper or else its Common, with the saint's name put in. These are the
//! texts it brings to the day's office when kept, and the ones it is commemorated with.

use calendar::{Category, Feast, Season};

use crate::commemoration::lookup_commemoration;
use crate::conclusion::apply_conclusion;
use crate::texts::OfficeTexts;

/// One of the feast's texts, under the label the feast page gives it.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct FeastText {
    pub label: String,
    pub text: String,
    /// Taken from the Common rather than the feast's own proper.
    pub common: bool,
    /// A prayer, set as flowing prose; an antiphon or versicle keeps its lines.
    pub prayer: bool,
}

/// The feast's texts and where they came from.
#[derive(Clone, Debug, Default, PartialEq, Eq)]
pub struct FeastTexts {
    pub texts: Vec<FeastText>,
    /// Some text is the feast's own.
    pub proper: bool,
    /// Some text is its Common's.
    pub common: bool,
}

/// The feast's texts as said in `season`. A Sunday, a feria or an observance without a category
/// has no texts of its own to show: its office is the season's.
pub fn feast_texts(feast: &Feast, season: Season, t: &OfficeTexts) -> FeastTexts {
    let mut out = FeastTexts::default();
    if feast.category.is_none() || feast.is_category(Category::Sunday) || feast.is_category(Category::Feria) {
        return out;
    }
    let lookup = |hour: &str, reference: &str| -> Option<(String, String)> {
        let (text, src) = lookup_commemoration(feast, season, hour, reference, t);
        let found = !text.trim().is_empty() && !text.starts_with('[') && !corpus::is_omitted(&text);
        (found && (src.starts_with("proper/") || src.starts_with("commons/"))).then_some((text, src))
    };
    let first = lookup("vespers", "magnificat-antiphon-first");
    let second = lookup("vespers", "magnificat-antiphon");
    let lauds_versicle = lookup("lauds", "versicle-lauds");
    let vespers_versicle = lookup("vespers", "versicle-vespers");
    let first_versicle = lookup("vespers", "versicle-first-vespers");
    let first = first.filter(|f| second.as_ref().is_none_or(|s| s.0 != f.0));
    let vespers_label = if first.is_some() { "At II Vespers · Magnificat" } else { "At Vespers · Magnificat" };
    let same = |a: &Option<(String, String)>, b: &Option<(String, String)>| matches!((a, b), (Some(a), Some(b)) if a.0 == b.0);
    let shared_versicle = same(&lauds_versicle, &vespers_versicle);
    let lauds_versicle_label = if shared_versicle { "Versicle at Lauds and Vespers" } else { "Versicle at Lauds" };
    let vespers_versicle = vespers_versicle.filter(|_| !shared_versicle);
    let first_versicle = first_versicle.filter(|f| {
        let f = Some(f.clone());
        !same(&f, &vespers_versicle) && !(vespers_versicle.is_none() && same(&f, &lauds_versicle))
    });
    let vespers_versicle_label = if first_versicle.is_some() { "Versicle at II Vespers" } else { "Versicle at Vespers" };
    let collect = lookup("lauds", "collect").map(|(text, src)| {
        let (text, _) = apply_conclusion(&text, &src, t);
        (text, src)
    });
    for (label, found) in [
        ("At I Vespers · Magnificat", first),
        ("Versicle at I Vespers", first_versicle),
        ("At Lauds · Benedictus", lookup("lauds", "benedictus-antiphon")),
        (lauds_versicle_label, lauds_versicle),
        (vespers_label, second),
        (vespers_versicle_label, vespers_versicle),
        ("Collect", collect),
    ] {
        let Some((text, src)) = found else { continue };
        let common = src.starts_with("commons/");
        out.proper |= !common;
        out.common |= common;
        out.texts.push(FeastText { label: label.to_string(), text, common, prayer: label == "Collect" });
    }
    out
}

#[cfg(test)]
mod tests {
    use calendar::CalendarData;

    use super::*;
    use crate::Engine;
    use crate::testutil::TestData;

    fn load() -> (Engine, CalendarData) {
        let data = TestData("../../data".into());
        (Engine::load(&data).unwrap(), CalendarData::load(&data).unwrap())
    }

    fn feast<'a>(cal: &'a CalendarData, id: &str) -> &'a Feast {
        cal.feasts.iter().find(|f| f.id == id).unwrap()
    }

    #[test]
    fn a_saint_without_propers_takes_his_common_with_his_name() {
        let (engine, cal) = load();
        let texts = feast_texts(feast(&cal, "st-raphael-of-brooklyn"), Season::Septuagesima, &engine.texts);
        assert!(texts.common && !texts.proper);
        let labels: Vec<&str> = texts.texts.iter().map(|t| t.label.as_str()).collect();
        assert_eq!(
            labels,
            [
                "At I Vespers · Magnificat",
                "Versicle at I Vespers",
                "At Lauds · Benedictus",
                "Versicle at Lauds and Vespers",
                "At II Vespers · Magnificat",
                "Collect"
            ]
        );
        let collect = &texts.texts.last().unwrap().text;
        assert!(collect.contains("blessed Raphael, thy Confessor and Bishop"), "{collect}");
        assert!(collect.contains("world without end"), "the collect ends with its conclusion: {collect}");
        assert!(texts.texts[2].text.starts_with("Well done"));
        assert!(texts.texts[1].text.contains("The Lord loved him and adorned him"));
    }

    #[test]
    fn a_feast_of_the_season_shows_no_texts_of_its_own() {
        let (engine, cal) = load();
        assert!(feast_texts(feast(&cal, "ash-wednesday"), Season::Lent, &engine.texts).texts.is_empty());
    }

    #[test]
    fn a_proper_collect_is_the_feasts_own() {
        let (engine, cal) = load();
        let texts = feast_texts(feast(&cal, "st-joseph"), Season::Lent, &engine.texts);
        let collect = texts.texts.iter().find(|t| t.label == "Collect").unwrap();
        assert!(!collect.common, "St Joseph has his own collect");
        assert!(texts.proper);
    }
}
