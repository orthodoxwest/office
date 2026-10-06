//! Synthetic collisions protect rubric boundaries outside the live calendar.
use crate::{
    commemoration::*,
    model::{CommemorationClass, MonthDay, OctaveClass},
    occurrence::*,
    *,
};
use std::sync::Arc;

fn feast(id: &str, rank: Rank, category: Category) -> FeastRef {
    Arc::new(Feast::synthetic(id, id, rank, Color::White, category))
}
fn ids(feasts: &[FeastRef]) -> Vec<&str> {
    feasts.iter().map(|f| f.id.as_str()).collect()
}

#[test]
fn precedence_rules_explain_each_tiebreak() {
    let privileged = feast("privileged", Rank::PrivilegedFeria, Category::Feria);
    let double = feast("double", Rank::Double, Category::Martyr);
    let second = feast("second", Rank::Double2ndClass, Category::Martyr);
    let corpus = feast("corpus-christi-octave-day-2", Rank::SemiDouble, Category::Lord);
    let sunday = feast("ordinary-sunday", Rank::SemiDouble, Category::Sunday);
    let greater = feast("greater", Rank::GreaterDouble, Category::Martyr);
    let mut moveable = (*double).clone();
    moveable.date_rule = Some("easter+1".into());
    let moveable = Arc::new(moveable);
    let lord = feast("lord", Rank::Double, Category::Lord);
    for (challenger, incumbent, wins, rule) in [
        (&privileged, &double, true, "privileged-feria-below-second-class"),
        (&second, &privileged, true, "second-class-over-privileged-feria"),
        (&corpus, &double, true, "corpus-octave-precedence"),
        (&sunday, &corpus, true, "sunday-or-first-class-over-corpus-octave"),
        (&second, &greater, true, "higher-rank"),
        (&sunday, &double, true, "sunday-rank-boost"),
        (&moveable, &double, true, "temporal-tiebreak"),
        (&lord, &double, true, "lord-tiebreak"),
        (&double, &double, false, "equal-precedence-possession"),
    ] {
        let (got, decision) = compare_feast_precedence_with_decision(challenger, incumbent);
        assert_eq!(got, wins, "{rule}");
        assert_eq!(decision.rule, format!("occurrence:{rule}"));
    }
}

#[test]
fn corpus_octave_transfers_second_class_but_yields_to_sunday_and_first_class() {
    let octave = feast("corpus-christi-octave-day-2", Rank::SemiDouble, Category::Lord);
    for (rank, category, wins, transfer) in [
        (Rank::Double, Category::Martyr, false, false),
        (Rank::Double2ndClass, Category::BlessedVirgin, false, true),
        (Rank::SemiDouble, Category::Sunday, true, false),
        (Rank::Double1stClass, Category::Apostle, true, false),
    ] {
        let contender = feast("contender", rank, category);
        let (day, out) = resolve_day(Date::new(2026, 6, 12), &[octave.clone(), contender.clone()], Season::Pentecost, Color::Green, &[]);
        assert!(Arc::ptr_eq(day.celebration.as_ref().unwrap(), if wins { &contender } else { &octave }));
        assert_eq!(out.len(), usize::from(transfer));
        if transfer {
            assert!(Arc::ptr_eq(&out[0], &contender));
        }
    }
}

#[test]
fn commemoration_hierarchy_is_independent_of_input_order() {
    let mut corpus = (*feast("high-octave-day-4", Rank::SemiDouble, Category::Lord)).clone();
    corpus.octave_class = OctaveClass::PrivilegedSecond;
    let mut ascension = (*feast("low-octave-day-4", Rank::SemiDouble, Category::Lord)).clone();
    ascension.octave_class = OctaveClass::PrivilegedThird;
    let mut friday = (*feast("friday", Rank::SemiDouble, Category::Feria)).clone();
    friday.commemoration_class = CommemorationClass::PostAscensionFeria;
    let mut vigil = (*feast("vigil", Rank::Simple, Category::Feria)).clone();
    vigil.is_vigil = true;
    vigil.vigil_of = Some("example".into());
    let mut simple_octave = (*feast("terminal", Rank::Simple, Category::Martyr)).clone();
    simple_octave.octave_class = OctaveClass::Simple;
    let expected = vec![
        feast("sunday", Rank::SemiDouble, Category::Sunday),
        Arc::new(corpus),
        feast("ember-example", Rank::PrivilegedFeria, Category::Feria),
        feast("example-octave-day", Rank::GreaterDouble, Category::Apostle),
        feast("greater", Rank::GreaterDouble, Category::Confessor),
        feast("double", Rank::Double, Category::Confessor),
        Arc::new(ascension),
        Arc::new(friday),
        feast("common-octave-day-3", Rank::SemiDouble, Category::Apostle),
        feast("feria", Rank::Commemoration, Category::Feria),
        Arc::new(vigil),
        Arc::new(simple_octave),
        feast("simple", Rank::Simple, Category::Virgin),
        feast("memorial", Rank::Commemoration, Category::Martyr),
    ];
    // Deterministic permutations without a random-number dependency.
    let mut input = expected.clone();
    let mut state = 399_u64;
    for _ in 0..100 {
        for i in (1..input.len()).rev() {
            state = state.wrapping_mul(6364136223846793005).wrapping_add(1);
            input.swap(i, (state as usize) % (i + 1));
        }
        let original = ids(&input);
        let actual = order_commemorations(&input, &OrderContext { season: Some(Season::Advent), ..Default::default() });
        assert_eq!(ids(&actual), ids(&expected));
        assert_eq!(ids(&input), original);
    }
    for season in [Season::Advent, Season::Septuagesima, Season::Lent, Season::Passiontide] {
        let (mut day, _) = resolve_day(Date::new(2026, 3, 11), &[], season, Color::Violet, &[]);
        day.commemorations = vec![expected[5].clone(), expected[13].clone()];
        day.feria_commemoration = Some(expected[9].clone());
        let actual = lauds_commemorations(&day);
        assert_eq!(
            ids(&actual),
            if matches!(season, Season::Lent | Season::Passiontide) {
                vec!["feria", "double", "memorial"]
            } else {
                vec!["double", "feria", "memorial"]
            }
        );
    }
}

#[test]
fn companions_stay_grouped_and_concurrent_office_precedes_the_cap() {
    let parent = feast("parent", Rank::GreaterDouble, Category::Apostle);
    let mut companion = (*feast("companion", Rank::Commemoration, Category::Apostle)).clone();
    companion.companion_of = Some("parent".into());
    let companion = Arc::new(companion);
    let equal = feast("equal", Rank::GreaterDouble, Category::Confessor);
    let concurrent = feast("concurrent", Rank::Commemoration, Category::Martyr);
    let ordered = order_commemorations(
        &[parent.clone(), equal.clone(), companion.clone(), concurrent.clone()],
        &OrderContext { concurrent: Some(concurrent.clone()), ..Default::default() },
    );
    assert_eq!(ids(&ordered), ["concurrent", "parent", "companion", "equal"]);
    let ordered = order_commemorations(
        &[equal, companion, concurrent.clone()],
        &OrderContext { concurrent: Some(concurrent), winner: Some(parent), ..Default::default() },
    );
    assert_eq!(ids(&ordered), ["concurrent", "companion", "equal"]);
    let mut input: Vec<_> =
        (0..MAX_COMMEMORATIONS_PER_DAY).map(|i| feast(&format!("memorial-{i}"), Rank::Commemoration, Category::Martyr)).collect();
    input.push(feast("sunday", Rank::SemiDouble, Category::Sunday));
    let (ordered, _) = ordered_commemorations(None, &input, OrderContext::default());
    assert_eq!(ordered.len(), MAX_COMMEMORATIONS_PER_DAY);
    assert_eq!(ordered[0].id, "sunday");
    // A principal's companion precedes a greater double, both through the
    // ordering entry point and at Lauds.
    let parent = feast("apostolic-office", Rank::Double, Category::Apostle);
    let mut companion = (*feast("companion", Rank::Commemoration, Category::Apostle)).clone();
    companion.companion_of = Some(parent.id.clone());
    let input = vec![feast("greater", Rank::GreaterDouble, Category::Confessor), Arc::new(companion)];
    let (ordered, _) = ordered_commemorations(Some(&parent), &input, OrderContext::default());
    assert_eq!(ids(&ordered), ["companion", "greater"]);
    let (mut day, _) = resolve_day(Date::new(2026, 7, 7), &[], Season::Pentecost, Color::Green, &[]);
    day.celebration = Some(parent);
    day.commemorations = input;
    assert_eq!(ids(&lauds_commemorations(&day)), ["companion", "greater"]);
}

#[test]
fn memorial_suppression_keeps_protected_scopes() {
    let memorial = feast("memorial", Rank::Commemoration, Category::Martyr);
    // Diurnal VIII, pp. xxviii–xxix. Easter and Pentecost Monday–Tuesday omit
    // Memorials in every ordo 2017–2026 (#380); St Joseph's Solemnity follows
    // the Primary Feast list (#138, #378).
    for (id, rank, category, suppressed) in [
        ("lord", Rank::Double1stClass, Category::Lord, true),
        ("saint", Rank::Double1stClass, Category::Confessor, true),
        ("marian", Rank::Double1stClass, Category::BlessedVirgin, true),
        ("second", Rank::Double2ndClass, Category::Lord, false),
        ("sunday", Rank::Double1stClass, Category::Sunday, false),
        ("feria", Rank::Double1stClass, Category::Feria, false),
        ("easter-sunday-octave-day-6", Rank::Double1stClass, Category::Lord, false),
        ("pentecost-octave-day-5", Rank::Double1stClass, Category::Lord, false),
        ("low-sunday", Rank::Double1stClass, Category::Lord, false),
        ("easter-monday", Rank::Double1stClass, Category::Lord, true),
        ("easter-tuesday", Rank::Double1stClass, Category::Lord, true),
        ("pentecost-octave-day-2", Rank::Double1stClass, Category::Lord, true),
        ("pentecost-octave-day-3", Rank::Double1stClass, Category::Lord, true),
        ("solemnity-st-joseph", Rank::Double1stClass, Category::Confessor, true),
    ] {
        let winner = feast(id, rank, category);
        let (kept, decisions) = ordered_commemorations(Some(&winner), std::slice::from_ref(&memorial), OrderContext::default());
        assert_eq!(kept.is_empty(), suppressed, "{id}: {decisions:?}");
        if suppressed {
            assert!(decisions.iter().any(|d| d.rule == "commemoration:memorial-under-first-class-feast"));
        }
    }
}

#[test]
fn first_class_feasts_admit_only_privileged_octaves() {
    // Diurnal X and General Rubrics XIV.4 (#378): no St George octave on St
    // Joseph's Solemnity, no St John Baptist octave on Ss Peter and Paul; a
    // Double II Class (St Mark) and a Sunday keep the common octave.
    let common = feast("st-george-octave-day-7", Rank::SemiDouble, Category::Martyr);
    let mut privileged = (*feast("corpus-christi-octave-day-3", Rank::SemiDouble, Category::Lord)).clone();
    privileged.is_privileged_octave_day = true;
    let privileged = Arc::new(privileged);
    for (winner, comm, kept) in [
        (feast("solemnity-st-joseph", Rank::Double1stClass, Category::Confessor), &common, false),
        (feast("ss-peter-paul", Rank::Double1stClass, Category::Apostle), &common, false),
        (feast("ss-peter-paul", Rank::Double1stClass, Category::Apostle), &privileged, true),
        (feast("st-mark", Rank::Double2ndClass, Category::Apostle), &common, true),
        // The Visitation's own rubric (Diurnal p. 557) drops the octave.
        (feast("visitation-bvm", Rank::Double2ndClass, Category::BlessedVirgin), &common, false),
        (feast("some-sunday", Rank::Double1stClass, Category::Sunday), &common, true),
    ] {
        let (out, decisions) = ordered_commemorations(Some(&winner), std::slice::from_ref(comm), OrderContext::default());
        assert_eq!(!out.is_empty(), kept, "{} under {}: {decisions:?}", comm.id, winner.id);
        if !kept {
            assert!(decisions.iter().any(|d| d.rule == "commemoration:common-octave-under-first-class-feast"));
        }
    }
}

#[test]
fn joseph_solemnity_suppresses_doubles_like_a_primary_feast() {
    // Fr Jason's #138 ruling alone (no ordo has a Double on the day): St
    // Joseph's Solemnity follows the Primary Feast list.
    let double = feast("st-pius-v", Rank::Double, Category::Confessor);
    let apostle = feast("st-john-latin-gate", Rank::GreaterDouble, Category::Apostle);
    let joseph = feast("solemnity-st-joseph", Rank::Double1stClass, Category::Confessor);
    let (kept, decisions) = primary_feast_doubles(Some(&joseph), vec![double.clone(), apostle.clone()]);
    assert!(kept.is_empty(), "{:?}", ids(&kept));
    assert!(decisions.iter().any(|d| d.rule == "commemoration:double-under-primary-feast-of-our-lord"));
    let other = feast("assumption", Rank::Double1stClass, Category::BlessedVirgin);
    let (kept, _) = primary_feast_doubles(Some(&other), vec![double, apostle]);
    assert_eq!(kept.len(), 2);
}

#[test]
fn only_trinity_and_corpus_christi_keep_an_occurring_apostle() {
    // The ordos keep Barnabas on Trinity (2017, 2023) and Corpus Christi
    // (2026) and St Paul on Trinity (2024); the ruling's exception goes no
    // further (#138, #379).
    let barnabas = feast("st-barnabas", Rank::GreaterDouble, Category::Apostle);
    let basil = feast("st-basil", Rank::GreaterDouble, Category::ConfessorDoctor);
    for (id, want) in
        [("corpus-christi", vec!["st-barnabas"]), ("trinity-sunday", vec!["st-barnabas"]), ("pentecost", vec![]), ("ascension", vec![])]
    {
        let mut w = (*feast(id, Rank::Double1stClass, Category::Lord)).clone();
        w.primary_of_our_lord = true;
        let (kept, _) = primary_feast_doubles(Some(&w), vec![barnabas.clone(), basil.clone()]);
        assert_eq!(ids(&kept), want, "{id}");
    }
}

#[test]
fn transfers_continue_through_a_blocked_year_boundary() {
    let fixed = |id, rank, color, category, month, day| {
        let mut f = Feast::synthetic(id, id, rank, color, category);
        f.fixed = Some(MonthDay { month, day });
        Arc::new(f)
    };
    let data = CalendarData {
        feasts: vec![
            fixed("year-end-winner", Rank::Double1stClass, Color::White, Category::Lord, 12, 31),
            fixed("cross-year-transfer", Rank::Double2ndClass, Color::Red, Category::Martyr, 12, 31),
            fixed("jan1-blocker", Rank::Double1stClass, Color::White, Category::Lord, 1, 1),
        ],
        penitential_rules: vec![],
    };
    let cal = build_calendar(2027, &data).unwrap();
    assert_eq!(cal.days[0].celebration.as_ref().unwrap().id, "jan1-blocker");
    assert_eq!(cal.days[1].celebration.as_ref().unwrap().id, "cross-year-transfer");
    for (index, rule) in [(0, "occurrence:transfer-in"), (0, "occurrence:transfer-out"), (1, "occurrence:transfer-in")] {
        assert!(cal.days[index].occurrence_decisions.iter().any(|d| d.rule == rule), "{index}: {rule}");
    }
}

#[test]
fn occurrence_passes_season_to_commemoration_order_with_and_without_winner() {
    let memorial = feast("memorial", Rank::Commemoration, Category::Martyr);
    let feria = feast("feria", Rank::Commemoration, Category::Feria);
    let winner = feast("winner", Rank::Double, Category::Confessor);
    for principal in [false, true] {
        let mut candidates = vec![memorial.clone(), feria.clone()];
        if principal {
            candidates.push(winner.clone());
        }
        for (season, expected) in [(Season::Lent, ["feria", "memorial"]), (Season::Pentecost, ["memorial", "feria"])] {
            let (day, _) = resolve_day(Date::new(2026, 3, 11), &candidates, season, Color::Violet, &[]);
            assert_eq!(day.celebration.is_some(), principal);
            assert_eq!(ids(&day.commemorations), expected);
        }
    }
}
