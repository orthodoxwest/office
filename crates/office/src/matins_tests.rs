//! Matins of the Dead against the Monastic Diurnal, pp. 81*–99*.
use std::sync::OnceLock;

use calendar::{CalendarData, Date, MoveableDates, build_calendar};
use liturgy::{ElementType, INVITATORY_ANTIPHON, OfficeHour, PrayerForm};

use crate::{Day, Engine, HOUR_NAMES, resolve_office_days, testutil::TestData};

fn engine() -> &'static Engine {
    static ENGINE: OnceLock<Engine> = OnceLock::new();
    ENGINE.get_or_init(|| Engine::load(&TestData("../../data".into())).unwrap())
}

fn compose(date: Date, form: PrayerForm) -> Result<OfficeHour, String> {
    let days = year_days(date.year());
    engine().compose_hour("matins", &days[date.ordinal() as usize - 1], &MoveableDates::compute(date.year()), form)
}

/// The days of `year`, built once per test that asks.
fn year_days(year: i32) -> Vec<Day> {
    let data = CalendarData::load(&TestData("../../data".into())).unwrap();
    let cal = build_calendar(year, &data).unwrap();
    let offices = resolve_office_days(&cal);
    cal.days.into_iter().zip(offices).map(|(c, o)| Day::new(c, o)).collect()
}

fn all_souls() -> OfficeHour {
    compose(Date::new(2026, 11, 2), PrayerForm::Private).unwrap()
}

fn kinds(hour: &OfficeHour, kind: ElementType) -> Vec<&liturgy::OfficeElement> {
    hour.sections.iter().flat_map(|s| &s.elements).filter(|e| e.kind == kind).collect()
}

#[test]
fn matins_is_composed_but_not_published() {
    // The fronts publish HOUR_NAMES; Matins joins them when the fronts pray it.
    assert!(!HOUR_NAMES.contains(&"matins"));
    assert!(crate::COMPOSABLE_HOURS.starts_with(&["matins"]) && crate::COMPOSABLE_HOURS[1..] == HOUR_NAMES);
}

#[test]
fn matins_is_composed_only_for_the_office_of_the_dead() {
    let err = compose(Date::new(2026, 11, 3), PrayerForm::Private).unwrap_err();
    assert!(err.contains("Office of the Dead"), "{err}");
    assert!(compose(Date::new(2026, 12, 25), PrayerForm::Private).is_err());
    // All Souls falling on a Sunday is kept on the Monday (2025 ordo p. 112).
    assert!(compose(Date::new(2025, 11, 2), PrayerForm::Private).is_err());
    assert_eq!(compose(Date::new(2025, 11, 3), PrayerForm::Private).unwrap().hour, "Matins");
}

/// Matins is outside the 28-year parity digest, which covers the published
/// hours, so it is composed here on every All Souls of that range, in every
/// prayer form: a missing text is a composition error, and the hour is the
/// same each year whatever the weekday.
#[test]
fn matins_of_the_dead_composes_on_every_all_souls() {
    let mut first: Option<OfficeHour> = None;
    for year in 2026..2054 {
        let days = year_days(year);
        let moveable = MoveableDates::compute(year);
        let mut found = 0;
        for day in days.iter().filter(|d| crate::psalmody::is_office_of_the_dead(d)) {
            for form in PrayerForm::ALL {
                let hour = engine().compose_hour("matins", day, &moveable, form).unwrap_or_else(|e| panic!("{year} {form:?}: {e}"));
                if form == PrayerForm::Private {
                    match &first {
                        None => first = Some(hour),
                        Some(f) => assert_eq!(f.sections, hour.sections, "{}", day.date),
                    }
                }
            }
            found += 1;
        }
        assert_eq!(found, 1, "{year}: one day of the Office of the Dead");
    }
}

#[test]
fn three_nocturns_follow_the_invitatory_on_all_souls() {
    // Diurnal p. 82*: "except in the Commemoration of All the Faithful
    // Departed" one Nocturn may be said; on All Souls all three are.
    let hour = all_souls();
    let labels: Vec<&str> = hour.sections.iter().map(|s| s.label.as_str()).filter(|l| !l.is_empty()).collect();
    assert_eq!(labels, ["Invitatory", "Nocturn I", "Nocturn II", "Nocturn III"]);
    assert!(kinds(&hour, ElementType::Hymn).is_empty(), "the Office of the Dead has no hymn at Matins");
    assert!(hour.decisions.iter().any(|d| d.rule == "context:office-form" && d.outcome == "office-of-the-dead"));
    let psalms: Vec<&str> = kinds(&hour, ElementType::Psalm).iter().map(|e| e.source_ref.as_str()).collect();
    let want = ["005", "006", "007", "023", "025", "027", "040", "041", "042"].map(|n| format!("psalms/{n}"));
    assert_eq!(psalms, want);
    for nocturn in &hour.sections[2..5] {
        // Each psalm under its antiphon said whole before and after (doubled,
        // as on a Double), and ended with Rest eternal, never the Glory be.
        let elems = &nocturn.elements;
        for (i, e) in elems.iter().enumerate().filter(|(_, e)| e.kind == ElementType::Psalm) {
            assert_eq!(elems[i - 1].kind, ElementType::Antiphon);
            assert!(!elems[i - 1].announce, "{}", elems[i - 1].text);
            assert_eq!(elems[i + 1].kind, ElementType::PsalmDoxology);
            assert!(elems[i + 1].text.starts_with("Rest eternal"), "{}", e.source_ref);
            assert_eq!(elems[i + 2].text, elems[i - 1].text);
        }
        // Versicle, the Our Father in secret, then three lessons each followed
        // by its responsory, without blessing or Tu autem (pp. 86*–87*).
        let tail: Vec<ElementType> = elems.iter().skip_while(|e| e.kind != ElementType::Versicle).map(|e| e.kind).collect();
        use ElementType::{Lesson, Prayer, Responsory, Rubric, Versicle};
        assert_eq!(tail, [Versicle, Rubric, Prayer, Lesson, Responsory, Lesson, Responsory, Lesson, Responsory]);
        assert!(elems.iter().all(|e| e.kind != ElementType::Blessing));
    }
}

#[test]
fn the_invitatory_weaves_its_antiphon_through_psalm_95() {
    // Diurnal pp. 82*–83*: the antiphon twice, whole and half by turns after
    // the five strophes, then Rest eternal, the half and the whole.
    let hour = all_souls();
    let invitatory = kinds(&hour, ElementType::Invitatory);
    assert_eq!(invitatory.len(), 1);
    let whole = "The King unto whom all live, * O come, let us worship.";
    let half = "O come, let us worship.";
    let pattern: Vec<&str> = invitatory[0]
        .text
        .lines()
        .map(|l| match l.strip_prefix(INVITATORY_ANTIPHON) {
            Some(a) if a == whole => "W",
            Some(a) if a == half => "H",
            Some(a) => panic!("unexpected antiphon {a:?}"),
            None if l.starts_with("Rest eternal") => "R",
            None => "S",
        })
        .collect();
    assert_eq!(pattern.concat(), "WWSWSHSWSHSWRHW");
    assert_eq!(invitatory[0].label, "Psalm 94\u{a0}(95)");
    assert!(invitatory[0].text.contains("O come, let us worship and fall down"));
}

#[test]
fn nine_lessons_from_job_with_their_responsories() {
    let hour = all_souls();
    let lessons = kinds(&hour, ElementType::Lesson);
    let headings: Vec<String> = lessons.iter().map(|e| format!("{} {}", e.label, e.incipit)).collect();
    assert_eq!(
        headings,
        [
            "Lesson I Job 7:16-21",
            "Lesson II Job 10:1-7",
            "Lesson III Job 10:8-12",
            "Lesson IV Job 13:22-28",
            "Lesson V Job 14:1-6",
            "Lesson VI Job 14:13-16",
            "Lesson VII Job 17:1-3, 11-15",
            "Lesson VIII Job 19:20-27",
            "Lesson IX Job 10:18-22",
        ]
    );
    assert!(lessons.iter().all(|e| !e.text.starts_with('!') && !e.text.contains("But thou")));
    let responsories = kinds(&hour, ElementType::Responsory);
    assert_eq!(responsories.len(), 9);
    for r in &responsories {
        // Every responsory is said through: its repetend after the ℣.
        let lines: Vec<&str> = r.text.lines().collect();
        let repetend = lines[0].rsplit(" * ").next().unwrap();
        assert!(lines[0].starts_with("R. ") && lines[1].starts_with("V. "), "{}", r.text);
        assert!(lines[2].ends_with(repetend) || r.source_ref.ends_with("responsory-ix"), "{}", r.text);
    }
    // The third of each Nocturn but the first adds Rest eternal (pp. 88*, 93*, 99*).
    let rest: Vec<bool> = responsories.iter().map(|r| r.text.contains("V. Rest eternal")).collect();
    assert_eq!(rest, [false, false, true, false, false, true, false, false, true]);
    // With three Nocturns the ninth is Deliver me, O Lord, from death eternal (p. 99*).
    assert!(responsories[8].text.starts_with("R. Deliver me, O Lord, from death eternal"));
}

#[test]
fn separated_from_lauds_matins_ends_with_the_collect_of_vespers() {
    // Diurnal p. 99*: The Lord be with you, the Collect as at Vespers (p. 77*),
    // Rest eternal and May they rest in peace.
    let hour = all_souls();
    let collect = kinds(&hour, ElementType::Collect);
    assert_eq!(collect.len(), 1);
    assert!(collect[0].text.starts_with("O God, the Creator and Redeemer of all the faithful"));
    let last = hour.sections.last().unwrap();
    assert_eq!(last.elements.last().unwrap().text, "V. May they rest in peace.\nR. Amen.");
    let priest = compose(Date::new(2026, 11, 2), PrayerForm::Priest).unwrap();
    assert!(priest.sections.iter().flat_map(|s| &s.elements).any(|e| e.text.starts_with("V. The Lord be with you.")));
}
