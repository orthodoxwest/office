//! Expectations read from cited sources, independent of generated snapshots.
use std::collections::{HashMap, HashSet};
use std::sync::OnceLock;

use calendar::{CalendarData, Category, Color, Date, MoveableDates, Season, build_calendar};
use liturgy::{ElementType, PostureAnchor, PrayerForm};

use crate::{Day, Engine, HOUR_NAMES, resolve_office_days, testutil::TestData};

fn engine() -> &'static Engine {
    static ENGINE: OnceLock<Engine> = OnceLock::new();
    ENGINE.get_or_init(|| Engine::load(&TestData("../../data".into())).unwrap())
}

fn year(year: i32) -> (Vec<Day>, MoveableDates) {
    let data = CalendarData::load(&TestData("../../data".into())).unwrap();
    let cal = build_calendar(year, &data).unwrap();
    let offices = resolve_office_days(&cal);
    (cal.days.into_iter().zip(offices).map(|(c, o)| Day::new(c, o)).collect(), MoveableDates::compute(year))
}

#[test]
fn cited_composition_requirements() {
    let rules: serde_json::Value = serde_json::from_str(include_str!("../../../data/review/composition-requirements.json")).unwrap();
    let rules = rules.as_array().unwrap();
    assert!(!rules.is_empty());
    let mut ids = HashSet::new();
    let mut years = HashMap::new();
    let string = |v: &serde_json::Value, key: &str| v[key].as_str().unwrap_or("").to_string();
    for rule in rules {
        let id = string(rule, "id");
        let source = string(rule, "source");
        let cases = rule["cases"].as_array().unwrap();
        assert!(!id.is_empty() && !source.is_empty() && !cases.is_empty() && ids.insert(id.clone()), "{rule}");
        for case in cases {
            let date = Date::parse(case["date"].as_str().unwrap()).unwrap();
            let hour = case["hour"].as_str().unwrap();
            let owner = string(case, "commemoration_owner");
            let slot = string(case, "slot");
            let reference = string(case, "ref");
            let contains = string(case, "contains");
            let before = string(case, "before_slot");
            let absent = case["absent"].as_bool().unwrap_or(false);
            let context = format!("{id}: {source}: {date} {hour} {slot}");
            assert!(!slot.is_empty() && (absent || !reference.is_empty() || !contains.is_empty()), "{context}");
            let (days, moveable) = years.entry(date.year()).or_insert_with(|| year(date.year()));
            let composed = engine().compose_hour(hour, &days[date.ordinal() as usize - 1], moveable, PrayerForm::Private).unwrap();
            let elements: Vec<_> = composed
                .sections
                .iter()
                .flat_map(|s| &s.elements)
                .enumerate()
                .filter(|(_, e)| e.commemoration_owner_id == owner && e.is_commemoration != owner.is_empty())
                .collect();
            let found: Vec<_> = elements.iter().filter(|(_, e)| e.slot_ref == slot).collect();
            if absent {
                assert!(found.is_empty(), "{context}: unexpected slot");
                continue;
            }
            assert!(!found.is_empty(), "{context}: missing slot for {owner}");
            for (_, e) in &found {
                if !reference.is_empty() {
                    assert_eq!(e.source_ref, reference, "{context}");
                }
                if !contains.is_empty() {
                    assert!(e.text.contains(&contains), "{context}: lacks {contains:?}");
                }
            }
            if !before.is_empty() {
                let next = elements.iter().find(|(_, e)| e.slot_ref == before).expect(&context);
                assert!(found[0].0 < next.0, "{context}: must precede {before}");
            }
        }
    }
}

#[test]
fn all_souls_vespers_is_a_continuous_office() {
    // Diurnal pp. 642–643 and 72*–76*; 2026 ordo pp. 21, 112.
    // The 2025 ordo p. 112 permits the appended office on the transferred
    // Sunday eve. Once included, it follows the same order as November 1.
    for (y, month, date) in [(2026, 11, 1), (2025, 11, 2)] {
        let (days, moveable) = year(y);
        let eve = Date::new(y, month, date);
        let day = &days[eve.ordinal() as usize - 1];
        for form in [PrayerForm::Private, PrayerForm::Deacon, PrayerForm::Priest] {
            let hour = engine().compose_hour("vespers", day, &moveable, form).unwrap();
            let boundary = hour.sections.iter().position(|s| s.label == crate::psalmody::VESPERS_OF_THE_DEAD_LABEL).unwrap();
            let preceding: Vec<_> = hour.sections[..boundary].iter().flat_map(|s| &s.elements).collect();
            let dead: Vec<_> = hour.sections[boundary..].iter().flat_map(|s| &s.elements).collect();
            assert_eq!(preceding.last().unwrap().source_ref, "shared/leader/benedicamus-domino", "{eve} {form:?}");
            assert_eq!(dead[0].source_ref, "proper/all-souls/psalm-antiphon-1-vespers", "{eve} {form:?}");
            assert_eq!(dead[1].source_ref, "psalms/116");
            assert_eq!(dead.last().unwrap().source_ref, "shared/formulas/may-they-rest-in-peace");
            assert!(hour.sections.iter().flat_map(|s| &s.elements).all(|e| !e.source_ref.contains("appended-vespers-of-the-dead-rubric")));
            let psalms: Vec<_> = dead.iter().filter(|e| e.kind == ElementType::Psalm).map(|e| e.source_ref.as_str()).collect();
            assert_eq!(psalms, ["psalms/116", "psalms/120", "psalms/121", "psalms/130", "psalms/138"]);
            assert_eq!(dead.iter().filter(|e| e.kind == ElementType::Antiphon).count(), 12);
            assert!(dead.iter().filter(|e| e.kind == ElementType::Antiphon).all(|e| !e.announce));
            assert!(dead.iter().filter(|e| e.kind == ElementType::PsalmDoxology).all(|e| e.source_ref == "shared/formulas/rest-eternal"));
            assert_eq!(dead.iter().filter(|e| e.source_ref == "ordinary/shared/our-father").count(), 1);
            assert!(dead.iter().all(|e| !e.is_commemoration && !e.source_ref.contains("hail-mary")));

            // Diurnal pp. 642–643, 651–652: the Lord's Prayer before the
            // collect is secret until its final versicle, in all three hours.
            for (name, date) in [("vespers", eve), ("compline", eve), ("lauds", eve.add_days(1))] {
                let office = engine().compose_hour(name, &days[date.ordinal() as usize - 1], &moveable, form).unwrap();
                let prayer =
                    office.sections.iter().flat_map(|s| &s.elements).rev().find(|e| e.source_ref == "ordinary/shared/our-father").unwrap();
                assert!(prayer.voice.iter().any(|s| !s.spoken && s.text.contains("Thy kingdom come")), "{date} {name}");
                assert!(prayer.voice.iter().any(|s| s.spoken && s.text.contains("And lead us not into temptation")), "{date} {name}");
                assert_eq!(office.sections.last().unwrap().elements.last().unwrap().source_ref, "shared/formulas/may-they-rest-in-peace");
            }

            // All Souls ends after None: its civil evening is the octave's
            // Vespers, with the ordinary closing prayers restored.
            let following = &days[eve.ordinal() as usize];
            let evening = engine().compose_hour("vespers", following, &moveable, form).unwrap();
            assert!(!evening.sections.iter().any(|s| s.label == crate::psalmody::VESPERS_OF_THE_DEAD_LABEL));
            let elements: Vec<_> = evening.sections.iter().flat_map(|s| &s.elements).collect();
            assert!(elements.iter().any(|e| e.source_ref == "proper/all-saints/magnificat-antiphon"));
            assert!(elements.iter().any(|e| e.source_ref == "shared/formulas/faithful-departed"));
        }
    }
}

#[test]
fn all_souls_minor_hours_follow_the_weekday_psalter_and_special_prayers() {
    // Diurnal pp. 652–654; 2026 ordo pp. 21, 112. All six possible
    // weekdays, including the Sunday-to-Monday transfer in the 2025 ordo.
    for (y, date, prime_psalms) in [
        (2025, 3, vec!["001", "002", "006"]),
        (2026, 2, vec!["001", "002", "006"]),
        (2027, 2, vec!["007", "008", "009a"]),
        (2033, 2, vec!["009b", "010", "011", "012"]),
        (2028, 2, vec!["013", "014", "015"]),
        (2029, 2, vec!["016", "017", "018a"]),
        (2030, 2, vec!["018b", "019", "020"]),
    ] {
        let (days, moveable) = year(y);
        let date = Date::new(y, 11, date);
        let day = &days[date.ordinal() as usize - 1];
        assert!(day.celebration_is("all-souls"));
        let monday = day.civil_weekday() == calendar::Weekday::Monday;
        for form in [PrayerForm::Private, PrayerForm::Deacon, PrayerForm::Priest] {
            for name in ["prime", "terce", "sext", "none"] {
                let hour = engine().compose_hour(name, day, &moveable, form).unwrap();
                let elements: Vec<_> = hour.sections.iter().flat_map(|s| &s.elements).collect();
                let context = format!("{date} {name} {form:?}");
                assert!(hour.sections.iter().all(|s| !s.collapsible), "{context}");
                let refs: Vec<_> = elements.iter().map(|e| e.source_ref.as_str()).collect();
                assert_eq!(&refs[..2], ["ordinary/shared/our-father", "ordinary/shared/hail-mary"], "{context}");
                let first_psalm = if name == "prime" {
                    assert_eq!(refs[2], "ordinary/shared/apostles-creed");
                    3
                } else {
                    2
                };
                assert_eq!(elements[first_psalm].kind, ElementType::Psalm, "{context}");
                assert!(
                    elements.iter().all(|e| !matches!(e.kind, ElementType::Antiphon | ElementType::Hymn | ElementType::Chapter)),
                    "{context}"
                );
                let expected = match name {
                    "prime" => prime_psalms.clone(),
                    "terce" if monday => vec!["119-xiv", "119-xv", "119-xvi"],
                    "sext" if monday => vec!["119-xvii", "119-xviii", "119-xix"],
                    "none" if monday => vec!["119-xx", "119-xxi", "119-xxii"],
                    "terce" => vec!["120", "121", "122"],
                    "sext" => vec!["123", "124", "125"],
                    _ => vec!["126", "127", "128"],
                };
                let psalms: Vec<_> = refs.iter().filter_map(|r| r.strip_prefix("psalms/")).collect();
                assert_eq!(psalms, expected, "{context}");
                let doxologies: Vec<_> = elements.iter().filter(|e| e.kind == ElementType::PsalmDoxology).collect();
                assert_eq!(doxologies.len(), 3, "{context}");
                assert!(doxologies.iter().all(|e| e.source_ref == "shared/formulas/rest-eternal"), "{context}");
                assert_eq!(refs.iter().filter(|r| **r == "ordinary/shared/our-father").count(), 2, "{context}");
                assert_eq!(refs.iter().filter(|r| **r == "ordinary/shared/hail-mary").count(), 1, "{context}");
                assert!(elements.iter().any(|e| e.voice.iter().any(|s| !s.spoken && s.text.contains("Thy kingdom come"))), "{context}");
                let collects: Vec<_> = elements.iter().filter(|e| e.kind == ElementType::Collect).collect();
                let expected_collects = if name == "prime" {
                    vec!["proper/all-souls/collect-prime", "proper/all-souls/prime-concluding-collect"]
                } else {
                    vec!["proper/all-souls/collect"]
                };
                assert_eq!(collects.iter().map(|e| e.source_ref.as_str()).collect::<Vec<_>>(), expected_collects, "{context}");
                assert!(collects.iter().all(|e| e.text.ends_with("R. Amen.")), "{context}");
                if name == "prime" {
                    let i = refs.iter().position(|r| *r == "proper/all-souls/collect-prime").unwrap();
                    assert_eq!(refs[i + 1], "proper/all-souls/prime-concluding-versicle");
                    assert_eq!(refs[i + 2], "shared/leader/let-us-pray");
                    assert_eq!(refs[i + 3], "proper/all-souls/prime-concluding-collect");
                }
                assert_eq!(&refs[refs.len() - 2..], ["shared/formulas/rest-eternal", "shared/formulas/may-they-rest-in-peace"]);
                assert!(refs.iter().all(|r| !r.contains("martyrology")
                    && !r.contains("benedicamus")
                    && !r.contains("opening-versicle")
                    && !r.contains("kyrie")));
            }
            // A standalone Lauds has the preliminary prayers in the main flow
            // (Diurnal p. 651), while appended Vespers has no second opening.
            let lauds = engine().compose_hour("lauds", day, &moveable, form).unwrap();
            assert!(!lauds.sections[0].collapsible);
            let refs: Vec<_> = lauds.sections.iter().flat_map(|s| &s.elements).map(|e| e.source_ref.as_str()).collect();
            assert_eq!(&refs[..3], ["ordinary/shared/our-father", "ordinary/shared/hail-mary", "proper/all-souls/psalm-antiphon-1"]);
        }
    }
    for date in [Date::new(2025, 11, 2), Date::new(2026, 11, 1), Date::new(2026, 11, 3)] {
        let (days, moveable) = year(date.year());
        for name in ["prime", "terce", "sext", "none"] {
            let hour = engine().compose_hour(name, &days[date.ordinal() as usize - 1], &moveable, PrayerForm::Private).unwrap();
            let elements: Vec<_> = hour.sections.iter().flat_map(|s| &s.elements).collect();
            assert!(elements.iter().any(|e| e.kind == ElementType::Antiphon));
            assert!(elements.iter().any(|e| e.kind == ElementType::Hymn));
            assert!(elements.iter().any(|e| e.source_ref == "ordinary/shared/gloria-patri"));
            assert!(elements.iter().all(|e| e.source_ref != "shared/formulas/rest-eternal"));
        }
    }
    // Diurnal p. 654: when All Souls is Saturday, the following Sunday
    // owns the evening instead of the All Saints octave.
    let (days, moveable) = year(2030);
    let day = &days[Date::new(2030, 11, 2).ordinal() as usize - 1];
    assert_eq!(day.vespers.owner, crate::VespersOwner::IOfFollowing);
    assert_eq!(day.vespers.feast.as_ref().unwrap().category, Some(Category::Sunday));
    let evening = engine().compose_hour("vespers", day, &moveable, PrayerForm::Private).unwrap();
    assert!(!evening.sections.iter().any(|s| s.label == crate::psalmody::VESPERS_OF_THE_DEAD_LABEL));
}

#[test]
fn sunday_commemoration_versicles_across_calendars() {
    for y in [2026, 2027, 2032] {
        let (days, moveable) = year(y);
        let mut checked = 0;
        for day in days.iter().filter(|d| d.season == Season::Pentecost) {
            for comm in day
                .commemorations
                .iter()
                .filter(|c| c.category == Some(Category::Sunday) && !["pentecost-sunday-1", "pentecost-sunday-2"].contains(&c.id.as_str()))
            {
                let hour = engine().compose_hour("lauds", day, &moveable, PrayerForm::Private).unwrap();
                let found: Vec<_> = hour
                    .sections
                    .iter()
                    .flat_map(|s| &s.elements)
                    .filter(|e| e.commemoration_owner_id == comm.id && e.slot_ref == "commemoration-versicle")
                    .collect();
                assert!(!found.is_empty(), "{} {}", day.date, comm.id);
                for e in found {
                    assert_eq!(e.source_ref, "ordinary/lauds/versicle-sunday", "Diurnal pp. xxix, 41: {} {}", day.date, comm.id);
                    checked += 1;
                }
            }
        }
        assert!(checked > 0, "no Sunday commemorations in {y}");
    }
}

#[test]
fn composition_structure_across_calendars() {
    for y in 2024..=2028 {
        let (days, moveable) = year(y);
        for day in &days {
            for name in HOUR_NAMES {
                let hour = engine().compose_hour(name, day, &moveable, PrayerForm::Private).unwrap();
                let context = format!("{} {name}", day.date);
                let mut labels = HashSet::new();
                for section in &hour.sections {
                    assert!(section.label.is_empty() || labels.insert(&section.label), "{context}: duplicate {}", section.label);
                    for e in &section.elements {
                        assert!(!e.text.lines().any(|l| l.trim().starts_with('#')), "{context}: leaked annotation {}", e.text);
                    }
                }
                if name == "vespers" && y == 2026 {
                    // Diurnal pp. 72*–75*: five psalms of the Dead; pp. 313, 315:
                    // five Triduum psalms plus Miserere; p. 360: Vigil Psalm 117 only.
                    let mut expected = match day.celebration_id() {
                        Some("holy-thursday" | "good-friday") => 6,
                        Some("holy-saturday") => 1,
                        _ if hour.feast == "Commemoration of All the Faithful Departed (All Souls' Day)" => 5,
                        _ => 4,
                    };
                    if hour.sections.iter().any(|s| s.label == "Vespers of the Dead") {
                        expected += 5;
                    }
                    // A psalm said straight on from the one before it, without
                    // Glory be, belongs to that psalm's unit (Monday pp. 118, 120).
                    let kinds: Vec<_> = hour.sections.iter().flat_map(|s| &s.elements).map(|e| e.kind).collect();
                    let psalms = kinds
                        .iter()
                        .enumerate()
                        .filter(|&(i, &k)| k == ElementType::Psalm && (i == 0 || kinds[i - 1] != ElementType::Psalm))
                        .count();
                    assert_eq!(psalms, expected, "{context}");
                }
            }
        }
    }
}

#[test]
fn parallel_composition_preserves_shared_engine_and_day() {
    let (days, moveable) = year(2026);
    let day = &days[76];
    let expected = engine().compose_hour("vespers", day, &moveable, PrayerForm::Private).unwrap();
    std::thread::scope(|scope| {
        let jobs: Vec<_> = (0..8)
            .map(|_| {
                scope.spawn(|| {
                    for form in PrayerForm::ALL {
                        engine().compose_hour("vespers", day, &moveable, form).unwrap();
                    }
                    engine().compose_hour("vespers", day, &moveable, PrayerForm::Private).unwrap()
                })
            })
            .collect();
        for job in jobs {
            assert_eq!(job.join().unwrap().sections, expected.sections);
        }
    });
}

fn principal<'a>(hour: &'a liturgy::OfficeHour, slot: &str) -> &'a liturgy::OfficeElement {
    let elements: Vec<_> = hour.sections.iter().flat_map(|s| &s.elements).filter(|e| !e.is_commemoration && e.slot_ref == slot).collect();
    assert_eq!(elements.len(), 1, "{} {}: {slot}", hour.date, hour.hour);
    elements[0]
}

#[test]
fn seasonal_little_hours_and_weekday_vespers_across_calendars() {
    // Diurnal pp. 163–164, 250–251, 272–275, 372–374; weekday Vespers
    // pp. 121, 125, 131, 135, 139. Future years test interactions, not ordo agreement.
    for y in [2026, 2027, 2032] {
        let (days, m) = year(y);
        let mut checked = [0; 3];
        for d in &days {
            let feria = d
                .celebration
                .as_ref()
                .is_none_or(|f| f.category == Some(Category::Feria) && (f.id.contains("feria") || f.id.contains("ember")));
            for (name, antiphon) in [("terce", "psalm-antiphon-2"), ("sext", "psalm-antiphon-3"), ("none", "psalm-antiphon-5")] {
                let seasonal = (d.season == Season::Lent && d.date > m.lent1)
                    || (d.season == Season::Easter && d.date > m.low_sunday && d.date < m.ascension)
                    || d.season == Season::Advent;
                let passion = d.date >= m.passion_sunday && d.date < m.holy_thursday;
                let holy_week = matches!(d.celebration_id(), Some("holy-monday" | "holy-tuesday" | "holy-wednesday"));
                let sunday = matches!(d.celebration_id(), Some("passion-sunday" | "palm-sunday"));
                if feria && seasonal || passion && (feria || holy_week || sunday) {
                    let h = engine().compose_hour(name, d, &m, PrayerForm::Private).unwrap();
                    if feria && seasonal {
                        for slot in ["chapter", "versicle"] {
                            assert_eq!(
                                principal(&h, slot).source_ref,
                                format!("seasonal/{}/{slot}-{name}", d.season.as_str()),
                                "{}",
                                d.date
                            );
                            checked[0] += 1;
                        }
                    }
                    if passion && (feria || holy_week || sunday) {
                        assert_eq!(principal(&h, "versicle").source_ref, format!("seasonal/passiontide/versicle-{name}"));
                        checked[1] += 1;
                        if !sunday {
                            assert_eq!(principal(&h, "chapter").source_ref, format!("seasonal/passiontide/chapter-{name}"));
                            if d.date < m.palm_sunday {
                                let ants: Vec<_> = h
                                    .sections
                                    .iter()
                                    .flat_map(|s| &s.elements)
                                    .filter(|e| !e.is_commemoration && e.slot_ref == antiphon)
                                    .collect();
                                assert!(!ants.is_empty());
                                assert!(ants.iter().all(|e| e.source_ref == format!("seasonal/passiontide/psalm-antiphon-{name}")));
                            }
                        }
                    }
                }
            }
            if d.vespers.feast.is_none()
                && d.celebration.as_ref().is_none_or(|f| f.category == Some(Category::Feria))
                && (1..=5).contains(&d.date.weekday().number())
                && matches!(d.season, Season::Pentecost | Season::Epiphany | Season::Septuagesima)
            {
                let h = engine().compose_hour("vespers", d, &m, PrayerForm::Private).unwrap();
                let e = principal(&h, "short-responsory");
                assert_eq!(e.source_ref, format!("ordinary/vespers/short-responsory-{}", d.date.weekday().name().to_lowercase()));
                assert!(e.text.contains("I will bless the Lord"));
                checked[2] += 1;
            }
        }
        assert!(checked.iter().all(|n| *n > 0), "{y}: {checked:?}");
    }
}

#[test]
fn major_collects_have_invitations_and_only_first_and_last_conclusions() {
    // Diurnal General Rubrics XII, p. xxxi; ordinary pp. 42–43, 144–147;
    // Breviary XXXIII.3,5 p. 50. Fixtures do not adjudicate occurrence.
    let (days, m) = year(2026);
    for (date, name, comms, final_ref) in [
        ("2026-01-01", "lauds", 0, ""),
        ("2026-01-01", "vespers", 0, ""),
        ("2026-01-04", "vespers", 2, ""),
        ("2026-01-04", "lauds", 2, ""),
        ("2026-01-05", "lauds", 1, ""),
        ("2026-01-17", "vespers", 4, ""),
        ("2026-01-19", "lauds", 2, "ordinary/shared/suffrage-collect"),
        ("2026-02-03", "vespers", 2, "ordinary/shared/suffrage-collect"),
        ("2026-01-30", "vespers", 0, "ordinary/shared/suffrage-collect-bvm"),
        ("2026-01-31", "lauds", 0, "ordinary/shared/suffrage-collect-bvm"),
        ("2026-06-20", "lauds", 1, "ordinary/shared/suffrage-collect-bvm"),
        ("2026-04-21", "vespers", 1, "ordinary/shared/cross-collect"),
        ("2026-04-22", "lauds", 1, "ordinary/shared/cross-collect"),
    ] {
        let d = &days[Date::parse(date).unwrap().ordinal() as usize - 1];
        for form in PrayerForm::ALL {
            let h = engine().compose_hour(name, d, &m, form).unwrap();
            let elements: Vec<_> = h.sections.iter().flat_map(|s| &s.elements).collect();
            let start = elements.iter().position(|e| e.kind == ElementType::Collect).unwrap() - 1;
            let end = start + elements[start..].iter().position(|e| e.leader_slot == "greeting").unwrap();
            let run = &elements[start..end];
            let positions: Vec<_> = run.iter().enumerate().filter(|(_, e)| e.kind == ElementType::Collect).map(|(i, _)| i).collect();
            assert_eq!(positions.len(), 1 + comms + usize::from(!final_ref.is_empty()), "{date} {name}");
            assert_eq!(run.iter().filter(|e| e.kind == ElementType::Collect && e.is_commemoration).count(), comms);
            assert_eq!(run.iter().filter(|e| e.source_ref == "shared/leader/let-us-pray").count(), positions.len());
            if !final_ref.is_empty() {
                assert_eq!(run[*positions.last().unwrap()].source_ref, final_ref);
            }
            for (j, &i) in positions.iter().enumerate() {
                assert!(i > 0);
                let invitation = run[i - 1];
                assert_eq!(invitation.kind, ElementType::Prayer);
                assert_eq!(invitation.source_ref, "shared/leader/let-us-pray");
                assert_eq!(invitation.text, "Let us pray.");
                if run[i].is_commemoration {
                    assert!(i >= 3);
                    assert_eq!(run[i - 2].kind, ElementType::Versicle);
                    assert_eq!(run[i - 3].kind, ElementType::Antiphon);
                    assert!(invitation.is_commemoration);
                    assert_eq!(invitation.commemoration_owner_id, run[i].commemoration_owner_id);
                }
                let end = positions.get(j + 1).copied().unwrap_or(run.len());
                let text = run[i..end].iter().map(|e| e.text.as_str()).collect::<Vec<_>>().join(" ");
                let text = text.split_whitespace().collect::<Vec<_>>().join(" ");
                assert_eq!(
                    text.matches("world without end.").count(),
                    usize::from(j == 0 || j + 1 == positions.len()),
                    "{date} {name}: collect {j}"
                );
            }
        }
    }
}

#[test]
fn psalmody_posture_cues_follow_the_parish_booklets() {
    // Parish Lauds booklets (Common of Apostles out of Paschaltide, pp. 1, 4–5):
    // Sit. after the first mediant of each unit, Stand. at the mediant of the
    // verse before its doxology, Bow. at Glory be, Stand upright. at As it was.
    let (days, moveable) = year(2026);
    let cues = |hour: &str, date: Date| -> Vec<(String, ElementType, Vec<String>)> {
        let composed = engine().compose_hour(hour, &days[date.ordinal() as usize - 1], &moveable, PrayerForm::Private).unwrap();
        composed
            .sections
            .iter()
            .flat_map(|s| &s.elements)
            .filter(|e| e.kind.is_psalmody() || e.kind == ElementType::PsalmDoxology)
            .map(|e| {
                let at = |c: &liturgy::PostureCue| match c.at {
                    PostureAnchor::AfterMediant(n) => format!("{} *{n}", c.posture.as_str()),
                    PostureAnchor::BeforeVerse(n) => format!("{} ^{n}", c.posture.as_str()),
                };
                (e.source_ref.clone(), e.kind, e.postures.iter().map(at).collect())
            })
            .collect()
    };
    let of = |cues: &[(String, ElementType, Vec<String>)], source: &str| -> Vec<String> {
        cues.iter().find(|(s, _, _)| s == source).unwrap_or_else(|| panic!("{source} missing")).2.clone()
    };
    let bow = ["bow ^0", "stand-upright ^1"].map(String::from).to_vec();

    // Sunday Lauds: the Benedicite and Psalms 148–150 under one Gloria.
    let lauds = cues("lauds", Date::new(2026, 10, 4));
    assert_eq!(of(&lauds, "canticles/benedicite"), ["sit *0", "stand *17", "bow ^18", "stand-upright ^19"]);
    assert_eq!(of(&lauds, "psalms/148"), ["sit *0"]);
    assert!(of(&lauds, "psalms/149").is_empty());
    assert_eq!(of(&lauds, "psalms/150"), ["stand *5"]);
    assert_eq!(of(&lauds, "canticles/benedictus"), Vec::<String>::new());
    for (i, (_, kind, got)) in lauds.iter().enumerate() {
        if *kind == ElementType::PsalmDoxology {
            assert_eq!(*got, bow, "doxology after {}", lauds[i - 1].0);
        }
    }

    // Monday Vespers joins 114–115 and 116b–117 under one Gloria each.
    let vespers = cues("vespers", Date::new(2026, 1, 19));
    assert_eq!(of(&vespers, "psalms/114"), ["sit *0"]);
    assert_eq!(of(&vespers, "psalms/115").len(), 1);
    assert_eq!(of(&vespers, "psalms/117"), ["stand *1"]);
    assert!(of(&vespers, "canticles/magnificat").is_empty());

    // Nothing is cued without the Gloria: the Office of the Dead and the Triduum.
    for (hour, date) in [("lauds", Date::new(2026, 11, 2)), ("lauds", moveable.good_friday), ("vespers", moveable.good_friday)] {
        assert!(cues(hour, date).iter().all(|(_, _, got)| got.is_empty()), "{hour} {date}");
    }
    // Only Lauds and Vespers are cued.
    for hour in ["prime", "terce", "compline"] {
        assert!(cues(hour, Date::new(2026, 10, 4)).iter().all(|(_, _, got)| got.is_empty()), "{hour}");
    }
}

/// #475/#477: the Martyrology announces Holy Name and each vigil on the day
/// the calendar keeps it, including Sunday anticipation and St Matthias's
/// leap-year move, whether or not the vigil has an office that day.
#[test]
fn martyrology_announcements_follow_the_calendar() {
    let vigils = [
        ("vigil-01-05", "vigil-epiphany"),
        ("vigil-02-23", "vigil-of-st-matthias"),
        ("vigil-06-23", "vigil-of-nativity-john-baptist"),
        ("vigil-06-28", "vigil-of-ss-peter-paul"),
        ("vigil-07-24", "vigil-of-st-james-greater"),
        ("vigil-08-09", "vigil-of-st-lawrence"),
        ("vigil-08-14", "vigil-of-assumption-bvm"),
        ("vigil-08-23", "vigil-of-st-bartholomew"),
        ("vigil-09-20", "vigil-of-st-matthew"),
        ("vigil-10-27", "vigil-of-ss-simon-jude"),
        ("vigil-10-31", "vigil-of-all-saints"),
        ("vigil-11-29", "vigil-of-st-andrew"),
    ];
    let options = crate::ComposeOptions { martyrology: true, ..Default::default() };
    let mut announced = HashSet::new();
    for y in 2026..=2053 {
        let (days, moveable) = year(y);
        for pair in days.windows(2) {
            let (today, tomorrow) = (&pair[0], &pair[1]);
            let prime = engine().compose_hour_with_options("prime", today, &moveable, &options).unwrap();
            if !crate::prime::reads_martyrology(&prime) {
                continue;
            }
            let refs: HashSet<&str> = prime.sections.iter().flat_map(|s| &s.elements).map(|e| e.source_ref.as_str()).collect();
            let details = tomorrow.occurrence_decisions.iter().filter_map(|d| d.detail.as_deref());
            let named: HashSet<&str> = tomorrow
                .celebration
                .iter()
                .chain(&tomorrow.commemorations)
                .map(|f| f.id.as_str())
                .chain(details.flat_map(|d| d.split(|c: char| !(c.is_ascii_alphanumeric() || c == '-'))))
                .collect();
            for (key, id) in vigils {
                let read = refs.contains(format!("ordinary/martyrology/{key}").as_str());
                assert_eq!(read, named.contains(id), "{}: {key} vs calendar {id}", tomorrow.date);
                if read {
                    announced.insert(key);
                }
            }
            let holy_name = refs.contains("ordinary/martyrology/holy-name");
            assert_eq!(holy_name, tomorrow.celebration_is("holy-name-jesus"), "{}: Holy Name", tomorrow.date);
        }
    }
    assert_eq!(announced.len(), vigils.len(), "every vigil is announced in some year");
}

/// The All Saints fast falls on the vigil only, anticipated with it to
/// Saturday when 31 October is a Sunday (2017–2025 ordos; 2021, 30 October).
/// The 2026 ordo's extra fast on Friday 30 October is triaged as a reference
/// error in data/review/ordo-triage.csv.
#[test]
fn all_saints_fast_follows_the_vigil() {
    let data = CalendarData::load(&TestData("../../data".into())).unwrap();
    for (y, fast_day) in [(2021, 30), (2025, 31), (2026, 31), (2027, 30)] {
        let cal = build_calendar(y, &data).unwrap();
        for day in cal.days.iter().filter(|d| d.date.month() == 10 && d.date.day() >= 29) {
            let vigil = day.celebration.as_deref().is_some_and(|c| c.id == "vigil-of-all-saints");
            assert_eq!(vigil, day.date.day() == fast_day, "{}: vigil", day.date);
            assert_eq!(day.penitential.fast, vigil, "{}: fast", day.date);
        }
    }
}

#[test]
fn easter_and_pentecost_octaves_say_vespers_as_on_sunday() {
    // #608. Diurnal pp. 363–367: Easter Day Vespers takes the "Ants. of
    // Lauds, omitting the fourth. Psalms of Sunday", the Lauds chapter and
    // "The Lord is risen indeed"; "The Chapters and VV. are said as above at
    // the Hours during the Octave". Diurnal p. 398 and the 2026 ordo pp.
    // 54, 68–69 keep the Psalms of Sunday through both Octaves.
    for y in 2026..=2030 {
        let (days, moveable) = year(y);
        let compose = |name: &str, date: Date| {
            let hour = engine().compose_hour(name, &days[date.ordinal() as usize - 1], &moveable, PrayerForm::Private).unwrap();
            hour.sections.into_iter().flat_map(|s| s.elements).filter(|e| !e.is_commemoration).collect::<Vec<_>>()
        };
        for offset in 0..6 {
            let date = moveable.easter.add_days(offset);
            let vespers = compose("vespers", date);
            let psalms: Vec<_> = vespers.iter().filter(|e| e.kind == ElementType::Psalm).map(|e| e.source_ref.as_str()).collect();
            assert_eq!(psalms, ["psalms/110", "psalms/111", "psalms/112", "psalms/113"], "{date}");
            let antiphon = |n| vespers.iter().find(|e| e.slot_ref == format!("psalm-antiphon-{n}")).unwrap().text.clone();
            assert!(antiphon(4).starts_with("And the Angel answered"), "{date}");
            let chapter = vespers.iter().find(|e| e.kind == ElementType::Chapter).unwrap();
            assert_eq!(chapter.label, "I Cor. 5:7", "{date}");
            let responsory = vespers.iter().find(|e| e.kind == ElementType::ShortResponsory).unwrap();
            assert!(responsory.text.contains("hath appeared to Simon"), "{date}");
            let lauds = compose("lauds", date);
            assert_eq!(lauds.iter().find(|e| e.kind == ElementType::Chapter).unwrap().label, "I Cor. 5:7", "{date} lauds");
            // Diurnal pp. 364, 367: "This is the day" after the hymn at both.
            for (name, hour) in [("lauds", &lauds), ("vespers", &vespers)] {
                let versicle = hour.iter().find(|e| e.kind == ElementType::Versicle && e.slot_ref == "versicle").unwrap();
                assert!(versicle.text.starts_with("V. This is the day"), "{date} {name}");
            }
            // Diurnal p. 365: the double-Alleluia dismissal at Lauds and Vespers.
            for (name, hour) in [("lauds", &lauds), ("vespers", &vespers)] {
                assert!(hour.iter().any(|e| e.source_ref == "shared/formulas/benedicamus-domino-alleluia"), "{date} {name}");
            }
        }
        // "Only through Lauds of Saturday before Low Sunday": its Vespers and
        // the Little Hours keep the plain dismissal.
        let saturday = moveable.easter.add_days(6);
        assert!(compose("lauds", saturday).iter().any(|e| e.source_ref == "shared/formulas/benedicamus-domino-alleluia"), "{y}");
        for name in ["vespers", "prime"] {
            let hour = compose(name, if name == "prime" { moveable.easter } else { saturday });
            assert!(hour.iter().all(|e| e.source_ref != "shared/formulas/benedicamus-domino-alleluia"), "{y} {name}");
        }
        for offset in 0..6 {
            let date = moveable.pentecost.add_days(offset);
            let vespers = compose("vespers", date);
            let psalms: Vec<_> = vespers.iter().filter(|e| e.kind == ElementType::Psalm).map(|e| e.source_ref.as_str()).collect();
            assert_eq!(psalms, ["psalms/110", "psalms/111", "psalms/112", "psalms/113"], "{date}");
            // Diurnal p. 398: II Vespers "All as at I Vespers, except" this versicle.
            let versicle = vespers.iter().find(|e| e.kind == ElementType::Versicle && e.slot_ref == "versicle").unwrap();
            assert!(versicle.text.starts_with("V. The Apostles did speak"), "{date}");
        }
        let eve = compose("vespers", moveable.pentecost.add_days(-1));
        let versicle = eve.iter().find(|e| e.kind == ElementType::Versicle && e.slot_ref == "versicle").unwrap();
        assert!(versicle.text.starts_with("V. They were all filled"), "{y} I Vespers");
    }
}

#[test]
fn paschal_and_pentecost_hymn_doxologies_at_the_hours() {
    // #609. Diurnal p. 3 and p. 364: "To thee who, dead, again dost live"
    // ends the hymns of the metre through None of the Vigil of the Ascension;
    // the Pentecost ending runs through the Octave, and Veni Creator replaces
    // Nunc Sancte at Terce (p. 398). 2026 ordo pp. 54–69: "Easter dox.",
    // "Ascension dox.", "Pentecost dox. / Veni Creator at Terce".
    const EASTER: &str = "All glory, as is ever meet,\nTo Father and to Paraclete.";
    const PENTECOST: &str = "Whom with the Father we adore\nAnd Holy Ghost for evermore.";
    for y in 2026..=2030 {
        let (days, moveable) = year(y);
        let hymn = |name: &str, date: Date| {
            let hour = engine().compose_hour(name, &days[date.ordinal() as usize - 1], &moveable, PrayerForm::Private).unwrap();
            hour.sections.into_iter().flat_map(|s| s.elements).find(|e| e.kind == ElementType::Hymn).unwrap()
        };
        let paschal = |date: Date, name: &str| {
            let h = hymn(name, date);
            assert!(h.text.contains("To thee who, dead, again dost live") && h.text.contains(EASTER), "{date} {name}: {}", h.text);
        };
        for date in [moveable.easter, moveable.easter.add_days(1), moveable.easter.add_days(8), moveable.ascension.add_days(-1)] {
            for name in ["prime", "terce", "sext", "none", "compline"] {
                // Compline of the Vigil follows I Vespers of the Ascension.
                if date == moveable.ascension.add_days(-1) && name == "compline" {
                    assert!(hymn(name, date).text.contains("Ascending o'er the stars"), "{date} {name}");
                } else {
                    paschal(date, name);
                }
            }
        }
        assert!(hymn("prime", moveable.ascension).text.contains("Ascending o'er the stars"), "{y}");
        for offset in 0..6 {
            let date = moveable.pentecost.add_days(offset);
            for name in ["prime", "terce", "sext", "none", "compline"] {
                assert!(hymn(name, date).text.contains(PENTECOST), "{date} {name}");
            }
            let terce = hymn("terce", date);
            assert_eq!(terce.source_ref, "proper/pentecost/hymn-terce", "{date}");
            assert_eq!(terce.label, "Veni, Creator Spiritus", "{date}");
        }
        // Compline of the Pentecost Vigil follows I Vespers of Pentecost;
        // that of Ember Saturday follows I Vespers of Trinity Sunday.
        assert!(hymn("compline", moveable.pentecost.add_days(-1)).text.contains(PENTECOST), "{y}");
        let trinity_eve = moveable.pentecost.add_days(6);
        assert!(hymn("compline", trinity_eve).text.contains("Shall live and reign eternally"), "{y}");
        assert!(hymn("prime", trinity_eve.add_days(2)).text.contains("To God the Holy Paraclete"), "{y}");
        // The Ascension hymn's "Ending is never changed" (p. 389).
        for date in [moveable.ascension, moveable.ascension.add_days(3)] {
            for name in ["lauds", "vespers"] {
                let h = hymn(name, date);
                assert!(h.text.contains("Be thou our joy") && !h.text.contains("Ascending o'er"), "{date} {name}");
            }
        }
    }
    // Hymns of the metre at Lauds and Vespers take the seasonal ending too:
    // Jesu, corona celsior (St Bede) and the Saturday Office of Our Lady in
    // Eastertide (2026 ordo p. 64, "Of BVM ... Easter dox."). The sapphic
    // Iste Confessor keeps its own, before and after the Ascension.
    let check = |y: i32, m: i32, d: i32, name: &str, has: &str, lacks: &str| {
        let (days, moveable) = year(y);
        let date = Date::new(y, m, d);
        let hour = engine().compose_hour(name, &days[date.ordinal() as usize - 1], &moveable, PrayerForm::Private).unwrap();
        let h = hour.sections.into_iter().flat_map(|s| s.elements).find(|e| e.kind == ElementType::Hymn).unwrap();
        assert!(h.text.contains(has) && !h.text.contains(lacks), "{date} {name}: {}", h.text);
    };
    check(2027, 5, 27, "lauds", "To thee who, dead, again dost live", "Glory to thee, O Father, Lord");
    check(2026, 5, 16, "lauds", "To thee who, dead, again dost live", "Virgin-born");
    check(2026, 5, 26, "vespers", "Only and Trinal", "Ascending o'er");
    check(2026, 1, 14, "vespers", "Only and Trinal", "For thine Epiphany");
}

/// Diurnal p. 235: Vespers of the Saturday before Septuagesima end "Let us
/// bless the Lord, alleluia, alleluia", even when they are of the Saturday's
/// own feast (St Matthias, 2035); the next Saturday's do not (#615).
#[test]
fn septuagesima_eve_dismissal_keeps_the_alleluias() {
    const ALLELUIA: &str = "shared/formulas/benedicamus-domino-alleluia";
    const PLAIN: &str = "shared/leader/benedicamus-domino";
    for (date, want) in [("2026-02-07", ALLELUIA), ("2027-02-27", ALLELUIA), ("2035-02-24", ALLELUIA), ("2026-02-14", PLAIN)] {
        let date = Date::parse(date).unwrap();
        let (days, moveable) = year(date.year());
        let hour = engine().compose_hour("vespers", &days[date.ordinal() as usize - 1], &moveable, PrayerForm::Private).unwrap();
        let refs: Vec<_> = hour.sections.iter().flat_map(|s| &s.elements).map(|e| e.source_ref.as_str()).collect();
        let dismissals: Vec<_> = refs.iter().filter(|r| [ALLELUIA, PLAIN].contains(r)).collect();
        assert_eq!(dismissals, [&want], "{date}");
    }
}

/// Diurnal p. 223: I Vespers of the Epiphany take the "Ants. of Lauds,
/// omitting the fourth"; II Vespers are "All as at I Vespers" (p. 226), and
/// every Vespers of the Octave is as on the Feast (pp. 227, 229, 231; 2026
/// ordo, 5–13 January: "Ant. (224)") (#625).
#[test]
fn epiphany_vespers_omit_the_fourth_lauds_antiphon() {
    for y in 2026..=2033 {
        let (days, moveable) = year(y);
        for d in 5..=13 {
            let date = Date::new(y, 1, d);
            let hour = engine().compose_hour("vespers", &days[date.ordinal() as usize - 1], &moveable, PrayerForm::Private).unwrap();
            let elements: Vec<_> = hour.sections.iter().flat_map(|s| &s.elements).filter(|e| !e.is_commemoration).collect();
            let antiphon = |n: u32| elements.iter().find(|e| e.slot_ref == format!("psalm-antiphon-{n}")).unwrap().text.clone();
            assert!(antiphon(1).starts_with("Before the morning star"), "{date}");
            assert!(antiphon(4).starts_with("Like a flame of fire"), "{date}: {}", antiphon(4));
        }
    }
}

/// Commemorations whose feast definition chooses their texts: the Common is
/// the right one for who they are (#617), "N." is a name (#607), and the
/// Saturninus antiphon keeps the one-Martyr Common until clergy rule (#610).
#[test]
fn commemoration_definitions_choose_fitting_texts() {
    let commemorated = |date: Date, hour: &str, owner: &str| -> Vec<(String, String)> {
        let (days, moveable) = year(date.year());
        let composed = engine().compose_hour(hour, &days[date.ordinal() as usize - 1], &moveable, PrayerForm::Private).unwrap();
        let found: Vec<_> = composed
            .sections
            .iter()
            .flat_map(|s| &s.elements)
            .filter(|e| e.is_commemoration && e.commemoration_owner_id == owner)
            .map(|e| (e.source_ref.clone(), e.text.clone()))
            .collect();
        assert!(!found.is_empty(), "{date} {hour}: {owner} not commemorated");
        found
    };
    let has = |found: &[(String, String)], text: &str| found.iter().any(|(_, t)| t.contains(text));

    // #617: Diurnal p. 465, St Peter at Lauds of 25 January "as above at I Vespers".
    let peter = commemorated(Date::new(2027, 1, 25), "lauds", "comm-01-25-commemoration-of-st-peter");
    assert!(has(&peter, "Thou art the shepherd of the sheep") && has(&peter, "Thou art Peter."), "{peter:?}");
    // #617: the impeded Octave Day of Ss Peter & Paul takes its own texts.
    let octave = commemorated(Date::new(2024, 7, 6), "lauds", "comm-extra-07-06-the-octave-of-ss-peter-and-paul");
    assert!(has(&octave, "Glorious princes") && has(&octave, "whose right hand upheld blessed Peter"), "{octave:?}");
    for found in [&peter, &octave] {
        assert!(!has(found, "good and faithful servant") && !has(found, "Confessor"), "{found:?}");
    }

    // #607: Diurnal p. 484 gives the Forty their own collect; the New Martyrs
    // complete the Common's "thy holy Martyrs N.".
    let forty = commemorated(Date::new(2026, 3, 10), "lauds", "comm-03-10-the-forty-holy-martyrs");
    assert!(has(&forty, "the fortitude of thy glorious Martyrs in their confession"), "{forty:?}");
    let russia = commemorated(Date::new(2026, 2, 4), "lauds", "comm-02-04-the-new-martyrs-of-russia");
    assert!(has(&russia, "thy holy Martyrs of Russia:"), "{russia:?}");

    // #610 (needs ruling): the 2026 ordo prints "The very hairs" for Saturninus;
    // the one-Martyr Common stays until clergy rule (data/review/ordo-triage.csv).
    let saturninus = "comm-11-29-st-saturninus-bishop-and-martyr";
    let vespers = commemorated(Date::new(2026, 11, 28), "vespers", saturninus);
    let lauds = commemorated(Date::new(2026, 11, 29), "lauds", saturninus);
    assert!(has(&vespers, "This is a Martyr") && has(&lauds, "He that hateth his life"), "{vespers:?} {lauds:?}");
    assert!(!has(&vespers, "The very hairs") && !has(&lauds, "The very hairs"));
}

/// #602: the Expectation is a feast of Our Lady in white (2017, 2018, 2023 and
/// 2025 ordos); when an Ember day takes the office the day is violet.
#[test]
fn expectation_of_the_bvm_is_white() {
    for (y, color) in [(2025, Color::White), (2023, Color::White), (2026, Color::Violet)] {
        let (days, moveable) = year(y);
        let date = Date::new(y, 12, 18);
        let lauds = engine().compose_hour("lauds", &days[date.ordinal() as usize - 1], &moveable, PrayerForm::Private).unwrap();
        assert_eq!(lauds.color, Some(color), "{date}: {}", lauds.feast);
    }
}

/// General Rubrics VI.2 (#616): a Vigil on a Solemnity has not even a
/// Commemoration. The 2021, 2022 and 2024 ordos print none; 2033 and 2035 are
/// the issue's later Corpus Christi cases.
#[test]
fn no_vigil_commemoration_on_a_solemnity() {
    let data = CalendarData::load(&TestData("../../data".into())).unwrap();
    for (y, m, d, celebration, vigil) in [
        (2021, 6, 28, "nativity-john-baptist", "vigil-of-ss-peter-paul"),
        (2022, 6, 23, "corpus-christi", "vigil-of-nativity-john-baptist"),
        (2024, 6, 22, "vigil-pentecost", "vigil-of-nativity-john-baptist"),
        (2033, 6, 23, "corpus-christi", "vigil-of-nativity-john-baptist"),
        (2035, 6, 28, "corpus-christi", "vigil-of-ss-peter-paul"),
    ] {
        let cal = build_calendar(y, &data).unwrap();
        let day = cal.days.iter().find(|day| day.date == Date::new(y, m, d)).unwrap();
        assert_eq!(day.celebration.as_deref().map(|c| c.id.as_str()), Some(celebration), "{}", day.date);
        assert!(day.commemorations.iter().all(|c| c.id != vigil), "{}: {vigil} commemorated", day.date);
        assert!(day.occurrence_decisions.iter().any(|x| x.rule == "commemoration:vigil-on-first-class-double"), "{}", day.date);
    }
}
