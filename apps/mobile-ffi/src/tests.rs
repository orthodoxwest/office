use std::path::Path;

use calendar::DataSource;
use tools::fs::FsData;

use super::*;

fn repo_data() -> FsData {
    FsData::new(Path::new(env!("CARGO_MANIFEST_DIR")).join("../../data"))
}

#[test]
fn the_embedded_corpus_is_the_data_directory() {
    let fs = repo_data();
    for dir in ["texts", "feasts", "office"] {
        let embedded =
            |rel: &str| !(rel.starts_with("chant/") || rel.ends_with(".md") || rel.rsplit('/').next().is_some_and(|n| n.starts_with('.')));
        let want: Vec<_> = fs.walk(dir).unwrap().into_iter().filter(|(rel, _)| embedded(rel)).collect();
        let got = EmbeddedData.walk(dir).unwrap();
        let paths = |files: &[(String, Vec<u8>)]| files.iter().map(|(rel, _)| rel.clone()).collect::<Vec<_>>();
        assert_eq!(paths(&got), paths(&want), "{dir}");
        assert!(got == want, "{dir}: contents differ");
    }
    for file in ["appointment-scopes.json", "collect-conclusions.txt", "latin-incipits.txt", "penitential.txt"] {
        assert_eq!(EmbeddedData.read(file).unwrap(), fs.read(file).unwrap(), "{file}");
    }
    assert_eq!(EmbeddedData.read("review/provenance.csv").unwrap(), None);
}

#[test]
fn composes_what_the_web_composes() {
    let core = OfficeCore::new().expect("embedded engine");
    let fs = repo_data();
    let engine = Engine::load(&fs).unwrap();
    let days = tools::year::load_office_days(&fs, 2026).unwrap();
    let moveable = MoveableDates::compute(2026);
    for (month, day) in [(1, 6), (4, 3), (4, 5), (9, 30), (12, 25)] {
        for hour in hour_names() {
            let view = core.compose(hour.clone(), 2026, month, day, "private".into(), false).unwrap();
            let date = Date::new(2026, month, day);
            let want = engine.compose_hour(&hour, &days[date.ordinal() as usize - 1], &moveable, PrayerForm::Private).unwrap();
            let want_sections: Vec<SectionView> = render_blocks::hour_sections(&want).into_iter().map(SectionView::from).collect();
            assert_eq!(format!("{:?}", view.sections), format!("{want_sections:?}"), "{hour} 2026-{month}-{day}");
            assert_eq!(view.title, want.title);
        }
    }
}

#[test]
fn the_martyrology_setting_reads_tomorrows_entry_at_prime() {
    let core = OfficeCore::new().unwrap();
    let text = |v: &HourView| format!("{:?}", v.sections);
    let off = core.compose("prime".into(), 2026, 9, 7, "private".into(), false).unwrap();
    let on = core.compose("prime".into(), 2026, 9, 7, "private".into(), true).unwrap();
    assert_eq!((off.martyrology, on.martyrology), (Some(false), Some(true)));
    assert!(!text(&off).contains("Martyrology — September 8") && text(&on).contains("Martyrology — September 8"));
    // Another hour, and a Prime without the Martyrology (Good Friday), have nothing to show.
    assert_eq!(core.compose("lauds".into(), 2026, 9, 7, "private".into(), true).unwrap().martyrology, None);
    for setting in [false, true] {
        assert_eq!(core.compose("prime".into(), 2026, 4, 10, "private".into(), setting).unwrap().martyrology, None);
    }
}

#[test]
fn rejects_bad_requests() {
    let core = OfficeCore::new().unwrap();
    assert!(core.compose("matins".into(), 2026, 1, 1, "private".into(), false).is_err());
    assert!(core.compose("lauds".into(), 2026, 2, 30, "private".into(), false).is_err());
    assert!(core.compose("lauds".into(), 2026, 1, 1, "bishop".into(), false).is_err());
}

fn civil(year: i32, month: i32, day: i32) -> CivilDate {
    CivilDate { year, month, day }
}

#[test]
fn home_invites_to_the_current_office_only_today() {
    let core = OfficeCore::new().unwrap();
    let today = civil(2026, 3, 15);
    let home = core.home(today, today, 18).unwrap();
    assert_eq!(home.date_label, "Sunday, March 15, 2026");
    assert_eq!(home.feast, "III Sunday in Lent");
    // The celebration already names the season.
    assert_eq!(home.season, "");
    assert_eq!(home.color, "violet");
    assert_eq!((home.pray_now_label.as_str(), home.current_hour.as_str()), ("Pray Vespers", "vespers"));
    // Compline after midnight belongs to yesterday, and marks no hour today.
    let late = core.home(today, today, 1).unwrap();
    assert_eq!((late.pray_now_hour.as_str(), late.pray_now_date, late.current_hour.as_str()), ("compline", civil(2026, 3, 14), ""));
    let other = core.home(civil(2026, 3, 16), today, 18).unwrap();
    assert_eq!((other.pray_now_label.as_str(), other.is_today), ("Open Lauds", false));
    assert_eq!(core.home(civil(2026, 3, 30), today, 9).unwrap().ornament, "passiontide");
}

#[test]
fn the_ordo_month_matches_the_web_rows() {
    let core = OfficeCore::new().unwrap();
    let march = core.ordo_month(2026, 3).unwrap();
    assert_eq!((march.name.as_str(), march.days.len()), ("March", 31));
    let first = &march.days[0];
    assert_eq!((first.feast.as_str(), first.rank.as_str(), first.weekday.as_str()), ("I Sunday in Lent", "1cl", "Sun"));
    assert_eq!(first.commemorations, ["St David of Wales, Bishop & Confessor"]);
    assert_eq!(first.benedictus_antiphon, "Then was Jesus");
    assert!(first.lauds_suffrage);
    assert_eq!(first.vespers_note, "II Vespers of preceding");
    assert!(core.ordo_month(2026, 13).is_err());
}

#[test]
fn reminders_name_each_office_on_the_chosen_days() {
    let core = OfficeCore::new().unwrap();
    let choose = |hour: &str, h: i32, m: i32| ReminderChoice { hour: hour.into(), hour_of_day: h, minute: m };
    // Sundays only, starting Saturday 14 March 2026: the one Sunday in three days is the 15th.
    let sundays = vec![true, false, false, false, false, false, false];
    let got = core.reminders(civil(2026, 3, 14), 3, vec![choose("vespers", 18, 0), choose("lauds", 6, 45)], sundays).unwrap();
    assert_eq!(got.len(), 2);
    // In time order, whatever the order chosen.
    assert_eq!((got[0].hour.as_str(), got[0].hour_of_day, got[0].minute), ("lauds", 6, 45));
    assert_eq!(got[1].date, civil(2026, 3, 15));
    assert_eq!((got[1].title.as_str(), got[1].feast.as_str()), ("Vespers", "III Sunday in Lent"));
    assert_eq!(got[1].summary, "Vespers — III Sunday in Lent");
    assert!(got[1].description.contains("Lent"), "{}", got[1].description);
    // Across the year's end, every day.
    let week = core.reminders(civil(2026, 12, 30), 4, vec![choose("compline", 21, 0)], vec![true; 7]).unwrap();
    assert_eq!(
        week.iter().map(|r| r.date).collect::<Vec<_>>(),
        [civil(2026, 12, 30), civil(2026, 12, 31), civil(2027, 1, 1), civil(2027, 1, 2)]
    );
    assert!(core.reminders(civil(2026, 1, 1), 1, vec![choose("matins", 3, 0)], vec![true; 7]).is_err());
    assert!(core.reminders(civil(2026, 1, 1), 1, vec![choose("lauds", 24, 0)], vec![true; 7]).is_err());
    assert!(core.reminders(civil(2026, 1, 1), 1, vec![], vec![true; 6]).is_err());
}

#[test]
fn usage_beacons_count_current_pages_as_the_web_does() {
    let today = civil(2026, 3, 15);
    let beacon = |event: UsageEvent| usage_beacon(event, today, false, "deacon".into(), UsageClient::Android, None);
    assert_eq!(
        beacon(UsageEvent::Hour { date: civil(2026, 3, 14), hour: "vespers".into() }).as_deref(),
        Some("vespers appearance:nave screen:mobile prayer-form:deacon client:android")
    );
    assert_eq!(beacon(UsageEvent::Home { date: today }).as_deref(), Some("site appearance:nave screen:mobile client:android"));
    assert_eq!(beacon(UsageEvent::Ordo { year: 2027 }).as_deref(), Some("ordo appearance:nave screen:mobile client:android"));
    assert_eq!(beacon(UsageEvent::RemindersPage).as_deref(), Some("site appearance:nave screen:mobile client:android"));
    assert_eq!(beacon(UsageEvent::RemindersOn).as_deref(), Some("reminders appearance:nave screen:mobile client:android"));
    assert_eq!(
        usage_beacon(
            UsageEvent::Hour { date: today, hour: "prime".into() },
            today,
            false,
            "private".into(),
            UsageClient::Android,
            Some(true)
        )
        .as_deref(),
        Some("prime appearance:nave screen:mobile prayer-form:private martyrology:shown client:android")
    );
    assert_eq!(
        usage_beacon(UsageEvent::Home { date: today }, today, true, "private".into(), UsageClient::Ios, None).as_deref(),
        Some("site appearance:apse screen:mobile client:ios")
    );
    // The archive, and anything the server would refuse, are not counted.
    for event in [
        UsageEvent::Hour { date: civil(2026, 3, 17), hour: "lauds".into() },
        UsageEvent::Home { date: civil(2019, 3, 15) },
        UsageEvent::Ordo { year: 2030 },
        UsageEvent::Hour { date: today, hour: "matins".into() },
        UsageEvent::Home { date: civil(2026, 2, 30) },
    ] {
        assert_eq!(beacon(event.clone()), None, "{event:?}");
    }
    assert!(usage_endpoint().starts_with("https://") && usage_endpoint().ends_with("/api/usage"));
}
