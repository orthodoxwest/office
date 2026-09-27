//! The ordo goldens Go writes (tests/fixtures/golden/ordo-YEAR.txt).

use calendar::{CalendarData, MoveableDates, build_calendar};
use office::{Day, Engine, resolve_office_days};
use tools::fs::FsData;

#[test]
fn ordo_goldens_match_go() {
    let src = FsData::new("../../data");
    let engine = Engine::load(&src).unwrap();
    let data = CalendarData::load(&src).unwrap();
    for year in [2026, 2027] {
        let cal = build_calendar(year, &data).unwrap();
        let office = resolve_office_days(&cal);
        let days: Vec<Day> = cal.days.into_iter().zip(office).map(|(c, o)| Day::new(c, o)).collect();
        let got = ordo::format_calendar(&days, Some(&engine), &MoveableDates::compute(year));
        let want = std::fs::read_to_string(format!("../../tests/fixtures/golden/ordo-{year}.txt")).unwrap();
        if got != want {
            let line = got.lines().zip(want.lines()).position(|(a, b)| a != b).unwrap_or(0);
            panic!("ordo-{year} differs at line {}:\n  got:  {:?}\n  want: {:?}", line + 1, got.lines().nth(line), want.lines().nth(line));
        }
    }
}
