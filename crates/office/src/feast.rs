//! A feast's own texts for its page, read from the office as it is said on a day the feast is kept
//! or commemorated: the collect, then the Gospel-canticle antiphons and the versicles, each from the
//! feast's proper or its Common, with the saint's name put in. Reading them from the composed hours
//! keeps the page to what the office says, in that day's season.

use calendar::{Category, MoveableDates};
use liturgy::{ElementType, OfficeHour, PrayerForm};

use crate::conclusion::{apply_conclusion, is_concluded};
use crate::day::Day;
use crate::engine::Engine;
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
    /// The Common some text is taken from.
    pub common: Option<Category>,
}

/// A day and its year's moveable dates, as an hour is composed from them.
#[derive(Clone, Copy)]
pub struct DayOf<'a> {
    pub day: &'a Day,
    pub moveable: &'a MoveableDates,
}

/// The hours a feast's texts are read from.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
enum Place {
    FirstVespers,
    Lauds,
    SecondVespers,
}

impl Place {
    fn name(self) -> &'static str {
        match self {
            Place::FirstVespers => "I Vespers",
            Place::Lauds => "Lauds",
            Place::SecondVespers => "II Vespers",
        }
    }
}

/// A text and the corpus entry it came from.
type Sourced = (String, String);

/// What one hour says of the feast: its antiphon, versicle and collect.
#[derive(Default)]
struct Said {
    /// The hour is the feast's own, not one it is commemorated in.
    own: bool,
    antiphon: Option<Sourced>,
    versicle: Option<Sourced>,
    collect: Option<Sourced>,
}

/// The feast's texts as the office says them on `on`, a day it is kept or commemorated, with the
/// Vespers of `eve`, the day before, where they are its first. `is_it(id, name)` says which of a
/// day's celebration and commemorations is the feast. Only the feast's proper and its Common are
/// shown: a day whose office is the season's or the psalter's has no texts of its own here.
pub fn feast_texts(engine: &Engine, is_it: &dyn Fn(&str, &str) -> bool, eve: Option<DayOf>, on: DayOf) -> FeastTexts {
    let t = &engine.texts;
    let compose = |hour: &str, d: DayOf| engine.compose_hour(hour, d.day, d.moveable, PrayerForm::Private).ok();
    // An hour is the feast's own when it is the feast's office (as its title names it); else
    // the feast may be commemorated in it.
    let read_hour = |hour: &str, d: DayOf| compose(hour, d).map(|h| read(&h, is_it("", &h.feast), is_it, t));
    let first = eve.and_then(|eve| read_hour("vespers", eve));
    let lauds = read_hour("lauds", on);
    let second = read_hour("vespers", on);

    let mut own = Vec::new();
    let mut antiphons = Vec::new();
    let mut versicles = Vec::new();
    let mut collects = Vec::new();
    for (place, said) in [(Place::FirstVespers, first), (Place::Lauds, lauds), (Place::SecondVespers, second)] {
        let Some(said) = said else { continue };
        if said.own {
            own.push(place);
        }
        antiphons.extend(said.antiphon.map(|a| (place, a)));
        versicles.extend(said.versicle.map(|v| (place, v)));
        collects.extend(said.collect.map(|c| (place, c)));
    }
    let mut out = FeastTexts::default();
    let mut push = |label: String, (text, src): Sourced, prayer: bool| {
        let common = src.starts_with("commons/");
        if common {
            out.common = out.common.or_else(|| common_of(&src));
        } else {
            out.proper = true;
        }
        out.texts.push(FeastText { label, text, common, prayer });
    };
    // With no I Vespers read, the day's Vespers are simply its Vespers.
    let first_read = antiphons.iter().chain(&versicles).any(|(p, _)| *p == Place::FirstVespers);
    let name = |places: &[Place]| places_name(places, first_read);
    let antiphons = grouped(antiphons, &own);
    // A versicle's label is the same in a commemoration, so its hours are not kept apart.
    let versicles = grouped(versicles, &[]);
    // The collect first: it is the day's, said at every hour, and it names the feast. Then the
    // texts of single hours. The collect as Lauds says it, else as Vespers does.
    if let Some((_, collect)) = collects.iter().find(|(p, _)| *p == Place::Lauds).or_else(|| collects.first()) {
        push("Collect".to_string(), collect.clone(), true);
    }
    // Each hour's antiphon, then its versicle, in the order they are said.
    for place in [Place::FirstVespers, Place::Lauds, Place::SecondVespers] {
        for (places, text) in antiphons.iter().filter(|(p, _)| p[0] == place) {
            // In the feast's own hour the antiphon is the canticle's; in a commemoration, its own.
            let what = match places.as_slice() {
                p if !own.contains(&p[0]) => " · Antiphon",
                [Place::Lauds] => " · Benedictus",
                p if !p.contains(&Place::Lauds) => " · Magnificat",
                _ => "",
            };
            push(format!("At {}{what}", name(places)), text.clone(), false);
        }
        for (places, text) in versicles.iter().filter(|(p, _)| p[0] == place) {
            push(format!("Versicle at {}", name(places)), text.clone(), false);
        }
    }
    out
}

/// What `hour` says of the feast: its own antiphon, versicle and collect when the hour is the
/// feast's (`own`), else those of its commemoration. A collect comes with its conclusion.
fn read(hour: &OfficeHour, own: bool, is_it: &dyn Fn(&str, &str) -> bool, t: &OfficeTexts) -> Said {
    let mut said = Said { own, ..Said::default() };
    let mut in_commemoration = false;
    for e in hour.sections.iter().flat_map(|s| &s.elements) {
        if e.kind == ElementType::Heading && !e.commemoration_owner_id.is_empty() {
            let name = e.text.strip_prefix("Commemoration of ").unwrap_or(&e.text);
            in_commemoration = !own && is_it(&e.commemoration_owner_id, name);
            continue;
        }
        let ours = if own { e.commemoration_owner_id.is_empty() } else { in_commemoration && !e.commemoration_owner_id.is_empty() };
        if !ours || !is_feasts(&e.source_ref) || e.text.trim().is_empty() || e.text.starts_with('[') {
            continue;
        }
        let found = (e.text.clone(), e.source_ref.clone());
        match (own, e.slot_ref.as_str()) {
            (true, "benedictus-antiphon" | "magnificat-antiphon") | (false, "commemoration-antiphon") => {
                said.antiphon.get_or_insert(found);
            }
            (true, "versicle") | (false, "commemoration-versicle") => {
                said.versicle.get_or_insert(found);
            }
            (true, "collect") | (false, "commemoration-collect") if said.collect.is_none() => {
                let text = if is_concluded(&e.source_refs) { found.0 } else { apply_conclusion(&found.0, &found.1, t).0 };
                said.collect = Some((text, found.1));
            }
            _ => {}
        }
    }
    said
}

/// A text of the feast's own proper or of a Common, as against the season's or the psalter's.
fn is_feasts(source: &str) -> bool {
    source.starts_with("proper/") || source.starts_with("commons/")
}

/// The Common a text was taken from: "commons/holy-woman-paschal/…" is that of Holy Women.
fn common_of(source: &str) -> Option<Category> {
    let common = source.strip_prefix("commons/")?.split('/').next()?;
    Category::parse(common.strip_suffix("-paschal").unwrap_or(common)).ok()
}

/// Texts said alike at several hours, as one, at the first hour that says them. The feast's own
/// hours and those it is commemorated in are kept apart, so that one label holds for each.
fn grouped(said: Vec<(Place, Sourced)>, own: &[Place]) -> Vec<(Vec<Place>, Sourced)> {
    let mut out: Vec<(Vec<Place>, Sourced)> = Vec::new();
    for (place, text) in said {
        let alike = |(p, t): &&mut (Vec<Place>, Sourced)| t.0 == text.0 && own.contains(&p[0]) == own.contains(&place);
        match out.iter_mut().find(alike) {
            Some((places, _)) => places.push(place),
            None => out.push((vec![place], text)),
        }
    }
    out
}

/// "Lauds", "I Vespers", "Lauds and II Vespers", named in the order they are said. I and II Vespers
/// together are simply Vespers, as II Vespers are when no I Vespers were read (`first_read`).
fn places_name(places: &[Place], first_read: bool) -> String {
    if places.contains(&Place::FirstVespers) && places.contains(&Place::SecondVespers) {
        return if places.contains(&Place::Lauds) { "Vespers and Lauds" } else { "Vespers" }.to_string();
    }
    let name = |p: &Place| if *p == Place::SecondVespers && !first_read { "Vespers" } else { p.name() };
    places.iter().map(name).collect::<Vec<_>>().join(" and ")
}

#[cfg(test)]
mod tests {
    use calendar::{CalendarData, Date, build_calendar, commemoration::normalize_commemoration_name};

    use super::*;
    use crate::resolve_office_days;
    use crate::testutil::TestData;

    struct Years {
        engine: Engine,
        data: CalendarData,
    }

    impl Years {
        fn load() -> Years {
            let data = TestData("../../data".into());
            Years { engine: Engine::load(&data).unwrap(), data: CalendarData::load(&data).unwrap() }
        }

        fn days(&self, year: i32) -> (Vec<Day>, MoveableDates) {
            let cal = build_calendar(year, &self.data).unwrap();
            let offices = resolve_office_days(&cal);
            (cal.days.into_iter().zip(offices).map(|(c, o)| Day::new(c, o)).collect(), MoveableDates::compute(year))
        }

        /// The texts of the feast `id` (or of its name) as said on `date`.
        fn texts(&self, id: &str, date: &str) -> FeastTexts {
            let date = Date::parse(date).unwrap();
            let name = normalize_commemoration_name(&self.data.feasts.iter().find(|f| f.id == id).unwrap().name);
            let is_it = |i: &str, n: &str| i == id || normalize_commemoration_name(n) == name;
            let (days, moveable) = self.days(date.year());
            let eve = date.add_days(-1);
            let (eve_days, eve_moveable) = if eve.year() == date.year() { (days.clone(), moveable.clone()) } else { self.days(eve.year()) };
            let at = |days: &[Day], d: Date| days.iter().position(|x| x.date == d).unwrap();
            let (on, before) = (at(&days, date), at(&eve_days, eve));
            feast_texts(
                &self.engine,
                &is_it,
                Some(DayOf { day: &eve_days[before], moveable: &eve_moveable }),
                DayOf { day: &days[on], moveable: &moveable },
            )
        }
    }

    fn text<'a>(t: &'a FeastTexts, label: &str) -> &'a str {
        &t.texts.iter().find(|x| x.label == label).unwrap_or_else(|| panic!("no {label} in {:?}", t.texts)).text
    }

    fn labels(t: &FeastTexts) -> Vec<&str> {
        t.texts.iter().map(|x| x.label.as_str()).collect()
    }

    #[test]
    fn a_saint_without_propers_takes_his_common_with_his_name() {
        let texts = Years::load().texts("st-raphael-of-brooklyn", "2027-02-27");
        assert_eq!((texts.proper, texts.common), (false, Some(Category::ConfessorBishop)));
        assert_eq!(
            labels(&texts),
            [
                "Collect",
                "At I Vespers · Magnificat",
                "Versicle at I Vespers",
                "At Lauds · Benedictus",
                "Versicle at Lauds and II Vespers",
                "At II Vespers · Antiphon"
            ],
            "his II Vespers give way to Septuagesima's I Vespers, where he is commemorated"
        );
        let collect = text(&texts, "Collect");
        assert!(collect.contains("blessed Raphael, thy Confessor and Bishop"), "{collect}");
        assert!(collect.contains("world without end"), "the collect ends with its conclusion: {collect}");
        assert!(text(&texts, "At Lauds · Benedictus").starts_with("Well done"));
        assert!(text(&texts, "Versicle at I Vespers").contains("The Lord loved him and adorned him"));
    }

    #[test]
    fn a_commemoration_shows_its_own_collect_not_the_commons() {
        let texts = Years::load().texts("comm-01-04-st-titus-bishop-and-confessor", "2027-01-04");
        let collect = text(&texts, "Collect");
        assert!(collect.contains("adorn blessed Titus"), "{collect}");
        assert!(texts.proper);
        assert!(labels(&texts).iter().all(|l| !l.contains("Benedictus") && !l.contains("Magnificat")), "{:?}", labels(&texts));
    }

    #[test]
    fn a_holy_womans_commemoration_takes_her_texts() {
        let texts = Years::load().texts("comm-08-29-st-sabina-martyr", "2027-08-29");
        assert!(texts.texts.iter().all(|t| !t.text.contains("his Master")), "{:?}", texts.texts);
    }

    #[test]
    fn i_vespers_take_the_feasts_own_versicle_before_the_commons() {
        let texts = Years::load().texts("chains-st-peter", "2026-08-01");
        assert!(text(&texts, "Versicle at Vespers and Lauds").contains("Thou art Peter"), "{:?}", texts.texts);
        assert!(text(&texts, "At I Vespers · Magnificat").starts_with("Thou art the shepherd"));
    }

    #[test]
    fn a_saint_in_whitsun_week_keeps_his_paschal_alleluias() {
        let texts = Years::load().texts("st-alban", "2027-06-22");
        assert!(texts.texts.iter().filter(|t| !t.prayer).all(|t| t.text.contains("alleluia")), "{:?}", texts.texts);
    }

    #[test]
    fn a_commemoration_is_shown_only_at_the_hours_that_say_it() {
        let texts = Years::load().texts("st-herman-alaska", "2026-12-13");
        assert!(labels(&texts).iter().all(|l| !l.contains("II Vespers")), "{:?}", labels(&texts));
    }

    #[test]
    fn a_proper_collect_is_the_feasts_own() {
        let texts = Years::load().texts("st-joseph", "2027-03-19");
        let collect = texts.texts.iter().find(|t| t.label == "Collect").unwrap();
        assert!(!collect.common, "St Joseph has his own collect");
        assert!(texts.proper);
    }

    #[test]
    fn the_office_of_the_dead_shows_its_own_texts() {
        let texts = Years::load().texts("all-souls", "2027-11-02");
        assert!(text(&texts, "Collect").starts_with("O God, the Creator and Redeemer of all the faithful"));
        assert!(labels(&texts).iter().all(|l| !l.contains("Vespers")), "its evening is the octave's: {:?}", labels(&texts));
    }

    #[test]
    fn a_day_without_i_vespers_names_its_vespers_plainly() {
        let years = Years::load();
        let ash = MoveableDates::compute(2027).ash_wednesday.to_string();
        let texts = years.texts("ash-wednesday", &ash);
        assert!(labels(&texts).iter().all(|l| !l.contains("II Vespers")), "{:?}", labels(&texts));
        assert!(labels(&texts).iter().any(|l| l.contains("Vespers")), "{:?}", labels(&texts));
    }

    #[test]
    fn the_hours_are_named_in_the_order_they_are_said() {
        let texts = Years::load().texts("st-thomas-apostle", "2026-12-21");
        assert!(labels(&texts).iter().all(|l| !l.contains("Lauds and Vespers")), "{:?}", labels(&texts));
    }
}
