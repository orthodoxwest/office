//! Render every retained hour/date golden without regenerating expectations.

use std::collections::HashMap;

use calendar::{CalendarData, Date, MoveableDates, build_calendar};
use liturgy::PrayerForm;
use office::{Day, Engine, HOUR_NAMES, resolve_office_days};
use render_text::format_office_hour;
use tools::fs::FsData;

const GOLDEN: &str = "../../tests/fixtures/golden";

#[test]
fn hour_goldens_match() {
    let src = FsData::new("../../data");
    let engine = Engine::load(&src).unwrap();
    let data = CalendarData::load(&src).unwrap();
    let mut years: HashMap<i32, (Vec<Day>, MoveableDates)> = HashMap::new();
    let mut names: Vec<String> = std::fs::read_dir(GOLDEN).unwrap().map(|e| e.unwrap().file_name().into_string().unwrap()).collect();
    names.sort();
    let mut checked = 0;
    let mut failures = Vec::new();
    for name in names {
        let Some(stem) = name.strip_suffix(".txt") else { continue };
        let Some((hour, date)) = stem.split_once('-') else { continue };
        let Some(date) = Date::parse(date) else { continue };
        if !HOUR_NAMES.contains(&hour) {
            continue;
        }
        let (days, moveable) = years.entry(date.year()).or_insert_with(|| {
            let cal = build_calendar(date.year(), &data).unwrap();
            let office = resolve_office_days(&cal);
            (cal.days.into_iter().zip(office).map(|(c, o)| Day::new(c, o)).collect(), MoveableDates::compute(date.year()))
        });
        let day = &days[date.ordinal() as usize - 1];
        let composed = engine.compose_hour(hour, day, moveable, PrayerForm::Private).unwrap();
        let want = std::fs::read_to_string(format!("{GOLDEN}/{name}")).unwrap();
        if format_office_hour(&composed) != want {
            failures.push(name.clone());
        }
        checked += 1;
    }
    assert!(checked >= 100, "only {checked} hour goldens found");
    assert!(failures.is_empty(), "{} of {checked} hour goldens differ: {failures:?}", failures.len());
}
