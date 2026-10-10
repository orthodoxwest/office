//! Vespers concurrence and decision-trace tests. Unless specified otherwise, synthetic days use a
//! Monday in the season after Pentecost.

use std::sync::Arc;

use calendar::model::{FERIA_COMMEMORATION_ID, Penitential};
use calendar::{CalendarDay, Category, Color, Date, Decision, Feast, FeastRef, Rank, Season};

use super::*;

fn feast(id: &str, rank: Rank, category: Category) -> Feast {
    let mut f = Feast::synthetic(id, "", rank, Color::White, category);
    f.category = Some(category);
    f
}

fn f(id: &str, rank: Rank, category: Category) -> FeastRef {
    Arc::new(feast(id, rank, category))
}

fn named(id: &str, name: &str, rank: Rank, category: Category) -> FeastRef {
    let mut x = feast(id, rank, category);
    x.name = name.to_string();
    Arc::new(x)
}

fn with(id: &str, rank: Rank, category: Category, edit: impl FnOnce(&mut Feast)) -> FeastRef {
    let mut x = feast(id, rank, category);
    edit(&mut x);
    Arc::new(x)
}

fn day(celebration: Option<&FeastRef>, commemorations: &[&FeastRef]) -> CalendarDay {
    CalendarDay {
        date: Date::new(2026, 6, 1),
        season: Season::Pentecost,
        tempora: None,
        celebration: celebration.cloned(),
        commemorations: commemorations.iter().map(|c| (*c).clone()).collect(),
        color: Color::White,
        notes: None,
        resolution_rule: String::new(),
        occurrence_decisions: Vec::new(),
        feria_commemoration: None,
        temporal_week_id: None,
        within_octave_of: None,
        penitential: Penitential::default(),
        monastic: Vec::new(),
    }
}

fn ids(list: &[FeastRef]) -> Vec<&str> {
    list.iter().map(|c| c.id.as_str()).collect()
}

fn same_list(got: &[FeastRef], want: &[&FeastRef]) -> bool {
    got.len() == want.len() && got.iter().zip(want).all(|(a, b)| Arc::ptr_eq(a, b))
}

fn assert_trace_rule(decisions: &[Decision], rule: &str) {
    assert!(decisions.iter().any(|d| d.rule == rule), "missing trace rule {rule} in {decisions:?}");
}

use VespersOwner::{IIOfPreceding, IOfFollowing, NotApplicable};

fn winner(prec: &Feast, fol: &Feast) -> VespersOwner {
    concurrence_winner(prec, fol).0
}

#[test]
fn concurrence_winner_table() {
    let double = feast("some-double", Rank::Double, Category::Martyr);
    let lesser_sunday = feast("epiphany-sunday-2", Rank::SemiDouble, Category::Sunday);
    let greater_sunday = feast("advent-sunday-1", Rank::Double2ndClass, Category::Sunday);
    // XIII.3-4: a Double below II Class yields to any Sunday.
    assert_eq!(winner(&double, &lesser_sunday), IOfFollowing);
    assert_eq!(winner(&double, &greater_sunday), IOfFollowing);
    // XIII.6: I Class Double vs Greater Sunday.
    let christmas = feast("christmas", Rank::Double1stClass, Category::Lord);
    assert_eq!(winner(&christmas, &feast("advent-sunday-4", Rank::Double2ndClass, Category::Sunday)), IIOfPreceding);
    // A sanctoral II Class Double takes it from a Greater Sunday both ways.
    let benedict = feast("st-benedict", Rank::Double2ndClass, Category::Confessor);
    let lent_sunday = feast("lent-sunday-1", Rank::Double2ndClass, Category::Sunday);
    assert_eq!(winner(&benedict, &lent_sunday), IIOfPreceding);
    assert_eq!(winner(&lent_sunday, &benedict), IOfFollowing);
    // Advent I keeps its II Vespers before St Andrew, who keeps his before
    // Advent I (2019, 2024, 2025, 2026 ordos; #62).
    let andrew = feast("st-andrew", Rank::Double2ndClass, Category::Apostle);
    assert_eq!(winner(&greater_sunday, &andrew), IIOfPreceding);
    assert_eq!(winner(&andrew, &greater_sunday), IIOfPreceding);
    let first_class = feast("some-double-1st", Rank::Double1stClass, Category::Confessor);
    assert_eq!(winner(&greater_sunday, &first_class), IOfFollowing);
    // A moveable temporal II Class office does not.
    let mut moveable = feast("moveable-temporal-office", Rank::Double2ndClass, Category::Lord);
    moveable.date_rule = Some("easter+10".to_string());
    assert_eq!(winner(&greater_sunday, &moveable), IIOfPreceding);
    assert_eq!(winner(&moveable, &greater_sunday), IOfFollowing);
    // Even a Greater Double of the Lord yields to a lesser Sunday.
    let lord = feast("exaltation-holy-cross", Rank::GreaterDouble, Category::Lord);
    let sunday15 = feast("pentecost-sunday-15", Rank::SemiDouble, Category::Sunday);
    assert_eq!(winner(&sunday15, &lord), IIOfPreceding);
    assert_eq!(winner(&lord, &sunday15), IOfFollowing);
    // XIII.6: II Class Double vs lesser Sunday.
    assert_eq!(winner(&feast("holy-name-jesus", Rank::Double2ndClass, Category::Lord), &lesser_sunday), IIOfPreceding);
    // XIII.12: Double vs day within an octave.
    assert_eq!(winner(&double, &feast("epiphany-octave-day-3", Rank::SemiDouble, Category::Lord)), IIOfPreceding);
    // A day within a common octave keeps II Vespers before a II Class Double
    // (2026 ordo 24 April, St George's octave before St Mark; #62), not
    // before a I Class one or a lesser Double.
    let george_octave = feast("st-george-octave-day-2", Rank::SemiDouble, Category::Martyr);
    let mark = feast("st-mark", Rank::Double2ndClass, Category::Apostle);
    assert_eq!(winner(&george_octave, &mark), IIOfPreceding);
    assert_eq!(winner(&george_octave, &first_class), IOfFollowing);
    assert_eq!(winner(&george_octave, &double), IOfFollowing);
    // XIII.9 with X.1(c): a primary II Class Double is worthier than a
    // secondary one of the Lord, either way round (2021, 2022, 2024 ordos; #557).
    let mut cross = feast("finding-holy-cross", Rank::Double2ndClass, Category::Lord);
    cross.secondary = true;
    let apostles = feast("ss-philip-james", Rank::Double2ndClass, Category::Apostle);
    assert_eq!(winner(&apostles, &cross), IIOfPreceding);
    assert_eq!(winner(&cross, &apostles), IOfFollowing);
}

#[test]
fn octave_day_vs_double() {
    let double = feast("some-double", Rank::Double, Category::Martyr);
    let greater_oct = feast("epiphany-octave-day", Rank::GreaterDouble, Category::Lord);
    assert_eq!(winner(&double, &greater_oct), IOfFollowing);
    assert_eq!(winner(&greater_oct, &double), IIOfPreceding);
    let greater_double = feast("some-greater-double", Rank::GreaterDouble, Category::Martyr);
    let lesser_oct = feast("some-octave-day", Rank::Double, Category::Confessor);
    assert_eq!(winner(&greater_double, &lesser_oct), IOfFollowing);
    assert_eq!(winner(&lesser_oct, &greater_double), IIOfPreceding);
    let second = feast("visitation-bvm", Rank::Double2ndClass, Category::BlessedVirgin);
    assert_eq!(winner(&greater_oct, &second), IOfFollowing);
    // XIII.11: the worthier octave day.
    assert_eq!(winner(&greater_oct, &feast("st-stephen-octave-day", Rank::GreaterDouble, Category::Martyr)), IIOfPreceding);
}

#[test]
fn privileged_days() {
    let easter = feast("easter-sunday", Rank::Double1stClass, Category::Lord);
    let double = feast("some-double-1st", Rank::Double1stClass, Category::Apostle);
    assert_eq!(winner(&easter, &double), IIOfPreceding);
    assert_eq!(winner(&double, &easter), IOfFollowing);
    // St Tikhon's I Vespers over Low Sunday (2026 ordo).
    assert_eq!(winner(&feast("low-sunday", Rank::Double1stClass, Category::Lord), &double), IOfFollowing);
}

#[test]
fn octave_days_keep_second_vespers_but_not_under_a_first_class_feast() {
    // General Rubrics II.1, VII.6 (#417): a semidouble day within an octave
    // has the feast's II Vespers.
    let day5 = f("st-george-octave-day-5", Rank::SemiDouble, Category::Martyr);
    let day6 = f("st-george-octave-day-6", Rank::SemiDouble, Category::Martyr);
    assert!(has_second_vespers(&day5));
    let mut prec = day(Some(&day5), &[]);
    prec.within_octave_of = Some("st-george".to_string());
    let mut fol = day(Some(&day6), &[]);
    fol.within_octave_of = Some("st-george".to_string());
    assert_ne!(resolve_concurrence(&prec, &fol).owner, NotApplicable);

    // Diurnal §X: the common octave is not commemorated at I Vespers of St
    // Joseph's Solemnity (2026 ordo, 28 April; #378).
    let joseph = f("solemnity-st-joseph", Rank::Double1stClass, Category::Confessor);
    let r = resolve_concurrence(&day(Some(&day6), &[]), &day(Some(&joseph), &[]));
    assert_eq!(r.owner, IOfFollowing);
    assert!(r.commemorations.is_empty(), "{:?}", ids(&r.commemorations));
    assert_trace_rule(&r.decisions, "commemoration:first-vespers-first-class-common-octave-exclusion");
}

#[test]
fn first_class_first_vespers_commemorates_the_outgoing_sunday() {
    // Diurnal §X; 2022 ordo (Assumption), 2026 ordo 28 June (#396).
    let sunday = f("pentecost-sunday-4", Rank::SemiDouble, Category::Sunday);
    let peter_paul = f("ss-peter-paul", Rank::Double1stClass, Category::Apostle);
    let r = resolve_concurrence(&day(Some(&sunday), &[]), &day(Some(&peter_paul), &[]));
    assert_eq!(r.owner, IOfFollowing);
    assert!(same_list(&r.commemorations, &[&sunday]), "{:?}", ids(&r.commemorations));
    assert_trace_rule(&r.decisions, "commemoration:first-vespers-first-class-sunday");
    let epiphany = f("epiphany", Rank::Double1stClass, Category::Lord);
    let r = resolve_concurrence(&day(Some(&sunday), &[]), &day(Some(&epiphany), &[]));
    assert!(r.commemorations.is_empty(), "{:?}", ids(&r.commemorations));
}

#[test]
fn saturday_bvm_commemorated_at_friday_double_ii_vespers() {
    // 2026 ordo 2 Oct: II Vespers of Holy Guardian Angels (Gd) with Comm. BVM
    // when Saturday is the Saturday Office of Our Lady (#542).
    let angels = f("guardian-angels", Rank::GreaterDouble, Category::Angel);
    let bvm = f("saturday-office-bvm", Rank::Simple, Category::BlessedVirgin);
    let r = resolve_concurrence(&day(Some(&angels), &[]), &day(Some(&bvm), &[]));
    assert_eq!(r.owner, IIOfPreceding);
    assert_eq!(r.rule, "concurrence:double-vs-octave-or-saturday-bvm");
    assert!(same_list(&r.commemorations, &[&bvm]), "{:?}", ids(&r.commemorations));
    assert_eq!(r.following_office_commemoration_id.as_deref(), Some("saturday-office-bvm"));
    assert_trace_rule(&r.decisions, "commemoration:following-office-at-second-vespers-saturday-bvm");

    // Ordinary Double (not only Greater) keeps the same commemorations.
    let ordinary = f("some-double", Rank::Double, Category::Martyr);
    let r = resolve_concurrence(&day(Some(&ordinary), &[]), &day(Some(&bvm), &[]));
    assert_eq!(r.owner, IIOfPreceding);
    assert!(same_list(&r.commemorations, &[&bvm]), "{:?}", ids(&r.commemorations));
    assert_eq!(r.following_office_commemoration_id.as_deref(), Some("saturday-office-bvm"));
}

#[test]
fn saturday_bvm_not_commemorated_at_second_class_ii_vespers() {
    // 2022 ordo 28 Oct: II Vespers of Ss Simon & Jude (D2) / No Comm. before
    // the Saturday Office of Our Lady.
    let apostles = f("simon-jude", Rank::Double2ndClass, Category::Apostle);
    let bvm = f("saturday-office-bvm", Rank::Simple, Category::BlessedVirgin);
    let r = resolve_concurrence(&day(Some(&apostles), &[]), &day(Some(&bvm), &[]));
    assert_eq!(r.owner, IIOfPreceding);
    assert!(r.commemorations.is_empty(), "{:?}", ids(&r.commemorations));
    assert_trace_rule(&r.decisions, "commemoration:following-office-at-second-vespers-simple-or-memorial");
}

#[test]
fn whitsun_and_easter_vespers_commemorate_the_next_days_double() {
    // 2017 and 2023 ordos 4 June: Whitsun II Vespers with Comm. Boniface, whose
    // feast falls on the Monday; 2018 ordo 10 April: Easter Tuesday II Vespers
    // with Comm. Leo (#380). A Primary Feast of Our Lord keeps it to Lauds (#558).
    let pentecost = with("pentecost", Rank::Double1stClass, Category::Lord, |x| x.primary_of_our_lord = true);
    let whit_monday = f("pentecost-octave-day-2", Rank::Double1stClass, Category::Lord);
    let easter_tuesday = f("easter-tuesday", Rank::Double1stClass, Category::Lord);
    let easter_wednesday = f("easter-sunday-octave-day-4", Rank::Double1stClass, Category::Lord);
    let boniface = f("st-boniface", Rank::Double, Category::Martyr);
    let leo = f("st-leo-i", Rank::Double, Category::ConfessorDoctor);
    let r = resolve_concurrence(&day(Some(&pentecost), &[]), &day(Some(&whit_monday), &[&boniface]));
    assert_eq!(r.owner, IIOfPreceding);
    assert!(same_list(&r.commemorations, &[&boniface]), "{:?}", ids(&r.commemorations));
    let r = resolve_concurrence(&day(Some(&easter_tuesday), &[]), &day(Some(&easter_wednesday), &[&leo]));
    assert!(same_list(&r.commemorations, &[&leo]), "{:?}", ids(&r.commemorations));
    let primary = with("first-class", Rank::Double1stClass, Category::Lord, |x| x.primary_of_our_lord = true);
    let r = resolve_concurrence(&day(Some(&primary), &[]), &day(Some(&whit_monday), &[&boniface]));
    assert!(r.commemorations.is_empty(), "{:?}", ids(&r.commemorations));
}

#[test]
fn first_class_second_vespers_keep_only_todays_double() {
    // 2019 and 2022 ordos 29 June: II Vespers of Ss Peter & Paul without the
    // following Commemoration of St Paul. The #558 allowance keeps today's
    // simplified Double, not the following day's.
    let peter_paul = f("ss-peter-paul", Rank::Double1stClass, Category::Apostle);
    let paul =
        with("commemoration-st-paul-apostle", Rank::GreaterDouble, Category::Apostle, |x| x.octave_of = Some("ss-peter-paul".to_string()));
    let sunday = f("pentecost-sunday-2", Rank::SemiDouble, Category::Sunday);
    let r = resolve_concurrence(&day(Some(&peter_paul), &[]), &day(Some(&sunday), &[&paul]));
    assert_eq!(r.owner, IIOfPreceding);
    assert!(same_list(&r.commemorations, &[&sunday]), "{:?}", ids(&r.commemorations));
}

#[test]
fn simple_preceding_and_nil_days() {
    let simple = f("some-simple", Rank::Simple, Category::Confessor);
    let double = f("some-double", Rank::Double, Category::Martyr);
    // A Simple ends at None and is not commemorated (XIII.17, XIV.9).
    let r = resolve_concurrence(&day(Some(&simple), &[]), &day(Some(&double), &[]));
    assert_eq!(r.owner, IOfFollowing);
    assert!(Arc::ptr_eq(r.feast.as_ref().unwrap(), &double));
    assert!(r.commemorations.is_empty());
    // An incoming Simple is excluded at II Vespers (XIV.7-8).
    let r = resolve_concurrence(&day(Some(&double), &[]), &day(Some(&simple), &[]));
    assert_eq!(r.owner, IIOfPreceding);
    assert!(r.commemorations.is_empty());
    // A plain feria still yields to the following I Vespers.
    let r = resolve_concurrence(&day(None, &[]), &day(Some(&double), &[]));
    assert_eq!(r.owner, IOfFollowing);
    assert!(r.commemorations.is_empty());
    let r = resolve_concurrence(&day(Some(&double), &[]), &day(None, &[]));
    assert_eq!(r.owner, IIOfPreceding);
    assert!(r.commemorations.is_empty());
}

#[test]
fn occurrence_at_second_vespers() {
    let first = feast("first-class", Rank::Double1stClass, Category::Lord);
    let primary = (*with("corpus-christi", Rank::Double1stClass, Category::Lord, |x| x.primary_of_our_lord = true)).clone();
    let trinity = (*with("trinity-sunday", Rank::Double1stClass, Category::Lord, |x| x.primary_of_our_lord = true)).clone();
    let pentecost = (*with("pentecost", Rank::Double1stClass, Category::Lord, |x| x.primary_of_our_lord = true)).clone();
    let easter_monday = feast("easter-monday", Rank::Double1stClass, Category::Lord);
    let whit_tuesday = feast("pentecost-octave-day-3", Rank::Double1stClass, Category::Lord);
    let joseph = feast("solemnity-st-joseph", Rank::Double1stClass, Category::Confessor);
    let second = feast("second-class", Rank::Double2ndClass, Category::Apostle);
    let greater = feast("greater-double", Rank::GreaterDouble, Category::Confessor);
    let companion = || {
        let mut c = feast("companion", Rank::Commemoration, Category::Apostle);
        c.companion_of = Some("example-apostle".to_string());
        c
    };
    let vigil = || {
        let mut v = feast("vigil-st-lawrence", Rank::Simple, Category::Feria);
        v.is_vigil = true;
        v
    };
    for (w, comm, want) in [
        (&greater, feast("memorial", Rank::Commemoration, Category::Martyr), false),
        (&greater, feast("simple", Rank::Simple, Category::Martyr), false),
        (&greater, feast("september-ember-wednesday", Rank::PrivilegedFeria, Category::Feria), false),
        (&greater, feast("rogation-monday", Rank::PrivilegedFeria, Category::Feria), false),
        (&greater, vigil(), false),
        (&greater, companion(), true),
        (&first, companion(), false),
        (&greater, feast("named-companion", Rank::Commemoration, Category::Apostle), false),
        (&first, feast("sunday", Rank::SemiDouble, Category::Sunday), true),
        // A simplified Double stays at Vespers of a Double I Class other than
        // a Primary Feast of Our Lord or St Joseph's Solemnity (#558).
        (&first, feast("double", Rank::Double, Category::Martyr), true),
        (&primary, feast("double", Rank::Double, Category::Martyr), false),
        (&joseph, feast("double", Rank::Double, Category::Martyr), false),
        (&first, feast("st-george-octave-day", Rank::Double, Category::Martyr), false),
        // An Apostle stays through II Vespers of Trinity and Corpus Christi
        // only (Barnabas and St Paul; #379), not of other Primary Feasts.
        (&primary, feast("st-barnabas", Rank::GreaterDouble, Category::Apostle), true),
        (&trinity, feast("commemoration-st-paul-apostle", Rank::GreaterDouble, Category::Apostle), true),
        (&pentecost, feast("st-barnabas", Rank::GreaterDouble, Category::Apostle), false),
        // Easter and Pentecost Monday and Tuesday keep it (2017–2024 ordos; #380).
        (&easter_monday, feast("st-john-latin-gate", Rank::GreaterDouble, Category::Apostle), true),
        (&whit_tuesday, feast("st-alban", Rank::Double, Category::Martyr), true),
        (&whit_tuesday, feast("memorial", Rank::Commemoration, Category::Martyr), false),
        (&primary, feast("st-basil", Rank::GreaterDouble, Category::ConfessorDoctor), false),
        (&first, feast("st-barnabas", Rank::GreaterDouble, Category::Apostle), true),
        (&second, feast("ss-peter-paul-octave-day-3", Rank::SemiDouble, Category::Martyr), false),
        (&greater, feast("double", Rank::Double, Category::Martyr), true),
        (&greater, feast("privileged-lenten-feria", Rank::PrivilegedFeria, Category::Feria), true),
        (&second, feast(FERIA_COMMEMORATION_ID, Rank::Commemoration, Category::Feria), true),
    ] {
        assert_eq!(occurrence_commemorated_at_second_vespers(Some(w), &comm).0, want, "{} under {}", comm.id, w.id);
    }
}

#[test]
fn outgoing_apostolic_companion() {
    let outgoing = f("chair-peter", Rank::GreaterDouble, Category::Apostle);
    let companion =
        with("commemoration-paul", Rank::Commemoration, Category::Apostle, |x| x.companion_of = Some("example-apostle".to_string()));
    let ordinary = named("named-only-companion", "Commemoration of St Paul", Rank::Commemoration, Category::Apostle);
    let sunday = f("lesser-sunday", Rank::SemiDouble, Category::Sunday);
    let r = resolve_concurrence(&day(Some(&outgoing), &[&companion, &ordinary]), &day(Some(&sunday), &[]));
    assert_eq!(r.owner, IOfFollowing);
    assert!(same_list(&r.commemorations, &[&outgoing, &companion]), "{:?}", ids(&r.commemorations));
    assert_trace_rule(&r.decisions, "commemoration:outgoing-apostolic-companion");

    // It follows a Double II Class into I Vespers of a Double I Class
    // (Diurnal §X; #396)...
    let second = f("second-class", Rank::Double2ndClass, Category::Apostle);
    let first = f("first-class", Rank::Double1stClass, Category::Lord);
    let r = resolve_concurrence(&day(Some(&second), &[&companion]), &day(Some(&first), &[]));
    assert_eq!(r.owner, IOfFollowing);
    assert!(same_list(&r.commemorations, &[&second, &companion]), "{:?}", ids(&r.commemorations));

    // ...and is not orphaned when the outgoing feast itself is suppressed.
    let circumcision = f("circumcision", Rank::Double2ndClass, Category::Lord);
    let r = resolve_concurrence(&day(Some(&outgoing), &[&companion]), &day(Some(&circumcision), &[]));
    assert_eq!(r.owner, IOfFollowing);
    assert!(r.commemorations.is_empty(), "{:?}", ids(&r.commemorations));
}

#[test]
fn outgoing_at_first_vespers() {
    let first = feast("first-class", Rank::Double1stClass, Category::Lord);
    let second = feast("second-class", Rank::Double2ndClass, Category::Apostle);
    let circumcision = feast("circumcision", Rank::Double2ndClass, Category::Lord);
    let double = feast("double", Rank::Double, Category::Martyr);
    let christmas = feast("christmas", Rank::Double1stClass, Category::Lord);
    let lent_sunday = feast("lent-sunday-2", Rank::Double1stClass, Category::Sunday);
    let ash_wednesday = feast("ash-wednesday", Rank::Double1stClass, Category::Feria);
    let sunday = feast("sunday", Rank::SemiDouble, Category::Sunday);
    for (w, loser, want) in [
        (&double, feast("simple", Rank::Simple, Category::Confessor), false),
        (&double, feast("feria", Rank::SemiDouble, Category::Feria), false),
        (&double, feast("epiphany-octave-day-3", Rank::SemiDouble, Category::Martyr), true),
        (&second, feast("epiphany-octave-day-3", Rank::SemiDouble, Category::Martyr), false),
        (&second, feast("easter-sunday-octave-day-4", Rank::Double1stClass, Category::Lord), false),
        (&second, feast("feria", Rank::SemiDouble, Category::Feria), false),
        (&double, feast("privileged-lenten-feria", Rank::PrivilegedFeria, Category::Feria), true),
        // An Ember day of Advent or Lent is kept at a following feast's I
        // Vespers like any Advent or Lenten feria (2022 ordo 14 Dec and
        // 18 March; #642), but not at the Sunday's after Ember Saturday
        // (2026 ordo 19 Dec), and the September Ember days keep to Lauds.
        (&second, feast("advent-ember-wednesday", Rank::PrivilegedFeria, Category::Feria), true),
        (&double, feast("advent-ember-friday", Rank::PrivilegedFeria, Category::Feria), true),
        (&second, feast("lent-ember-friday", Rank::PrivilegedFeria, Category::Feria), true),
        (&sunday, feast("advent-ember-saturday", Rank::PrivilegedFeria, Category::Feria), false),
        (&second, feast("september-ember-wednesday", Rank::PrivilegedFeria, Category::Feria), false),
        // Diurnal §X: at I Vespers of a Double I Class, a Feria of Advent,
        // Septuagesima or Lent (Ember days included) and a Double I or II Class
        // are commemorated (#396), but the Lenten Saturday is not at I Vespers
        // of a first-class Sunday, nor on the eve of Ash Wednesday, and the
        // September Ember days keep to Lauds.
        (&first, feast("privileged-lenten-feria", Rank::PrivilegedFeria, Category::Feria), true),
        (&first, feast("lent-ember-wednesday", Rank::PrivilegedFeria, Category::Feria), true),
        (&first, feast(FERIA_COMMEMORATION_ID, Rank::Commemoration, Category::Feria), true),
        (&lent_sunday, feast("privileged-lenten-feria", Rank::PrivilegedFeria, Category::Feria), false),
        (&ash_wednesday, feast(FERIA_COMMEMORATION_ID, Rank::Commemoration, Category::Feria), false),
        (&first, feast("september-ember-saturday", Rank::PrivilegedFeria, Category::Feria), false),
        (&first, second.clone(), true),
        (&first, feast("low-sunday", Rank::Double1stClass, Category::Lord), true),
        // The Easter and Pentecost octaves end at None of Saturday, and the
        // Triduum is never commemorated.
        (&first, feast("easter-sunday-octave-day-7", Rank::Double1stClass, Category::Lord), false),
        (&first, feast("pentecost-octave-day-7", Rank::Double1stClass, Category::Lord), false),
        (&first, feast("holy-saturday", Rank::Double1stClass, Category::Lord), false),
        // Diurnal §X: the Sunday is commemorated except before the Nativity
        // and the Epiphany (#396).
        (&first, sunday.clone(), true),
        (&christmas, sunday.clone(), false),
        // A common octave is not commemorated at I Vespers of a Double I
        // Class; a privileged one is (#378, #417).
        (&first, feast("st-george-octave-day-6", Rank::SemiDouble, Category::Martyr), false),
        (
            &first,
            (*with("corpus-christi-octave-day-2", Rank::SemiDouble, Category::Lord, |x| x.is_privileged_octave_day = true)).clone(),
            true,
        ),
        (&circumcision, sunday.clone(), false),
        (&circumcision, feast("greater-double", Rank::GreaterDouble, Category::Martyr), false),
        (&circumcision, double.clone(), false),
        (&second, double.clone(), true),
    ] {
        assert_eq!(outgoing_commemorated_at_first_vespers(Some(w), &loser).0, want, "{} under {}", loser.id, w.id);
    }
}

#[test]
fn first_vespers_retains_free_seasonal_feria() {
    for (name, date, week, season, rank, want) in [
        ("Scholastica", "2026-02-09", "septuagesima", Season::Septuagesima, Rank::Double2ndClass, true),
        ("Advent weekday", "2026-12-03", "advent-sunday-1", Season::Advent, Rank::Double, true),
        ("first class feast (Diurnal §X)", "2026-02-09", "septuagesima", Season::Septuagesima, Rank::Double1stClass, true),
        ("ordinary feria", "2026-09-07", "pentecost-sunday-14", Season::Pentecost, Rank::Double, false),
        ("Sunday is not a feria", "2026-02-08", "septuagesima", Season::Septuagesima, Rank::Double, false),
    ] {
        let date = Date::parse(date).unwrap();
        let mut prec = day(None, &[]);
        prec.date = date;
        prec.season = season;
        prec.temporal_week_id = Some(week.to_string());
        let following_feast = f("following", rank, Category::Virgin);
        let mut fol = day(Some(&following_feast), &[]);
        fol.date = date.add_days(1);
        let r = resolve_concurrence(&prec, &fol);
        assert_eq!(r.commemorations.len() == 1, want, "{name}: {:?}", ids(&r.commemorations));
        if want {
            assert_eq!(r.commemorations[0].id, FERIA_COMMEMORATION_ID, "{name}");
            assert_eq!(r.commemorations[0].proper_id.as_deref(), Some(week), "{name}");
        }
        assert!(prec.feria_commemoration.is_none());
    }
}

#[test]
fn impeded_lesser_double_at_first_vespers() {
    // #556: the ordo cases are listed on impeded_double_at_first_vespers.
    let double = f("impeded-double", Rank::Double, Category::Confessor);
    let lent_saturday = f("privileged-lenten-feria", Rank::PrivilegedFeria, Category::Feria);
    let anticipated = f("pentecost-sunday-23-anticipated", Rank::SemiDouble, Category::Sunday);
    let ember = f("september-ember-saturday", Rank::PrivilegedFeria, Category::Feria);
    let octave_day = with("corpus-christi-octave-day-3", Rank::SemiDouble, Category::Lord, |x| x.is_privileged_octave_day = true);
    let lent_sunday = f("lent-sunday-3", Rank::Double1stClass, Category::Sunday);
    let sunday = f("pentecost-sunday-24", Rank::SemiDouble, Category::Sunday);
    let lesser = f("following-double", Rank::Double, Category::Martyr);
    let second = f("second-class", Rank::Double2ndClass, Category::Apostle);
    for (name, impeder, following, want) in [
        ("Lent Saturday before the Sunday (Isidore 2026)", &lent_saturday, &lent_sunday, true),
        ("privileged octave before a Double", &octave_day, &lesser, true),
        ("Ember day before a Double (Januarius 2018)", &ember, &lesser, true),
        ("Double II Class (St Joseph 2026)", &lent_saturday, &second, false),
        ("anticipated Sunday (Romuald 2026)", &anticipated, &sunday, false),
        ("Ember Saturday before a Sunday (Januarius 2026)", &ember, &sunday, false),
    ] {
        let r = resolve_concurrence(&day(Some(impeder), &[&double]), &day(Some(following), &[]));
        assert_eq!(r.owner, IOfFollowing, "{name}");
        assert_eq!(r.commemorations.iter().any(|c| c.id == double.id), want, "{name}: {:?}", ids(&r.commemorations));
    }
}

#[test]
fn memorial_not_at_first_vespers_of_second_class_double() {
    // #390: Diurnal §VIII and §X; 2026 ordo 2 May, 1 July, 5 August and
    // 14 September (but not 24 July, 7 September or 3 January).
    let memorial = f("memorial", Rank::Commemoration, Category::Martyr);
    let simple = f("simple", Rank::Simple, Category::Martyr);
    let companion =
        with("commemoration-paul", Rank::Commemoration, Category::Apostle, |x| x.companion_of = Some("example-apostle".to_string()));
    let prec = f("preceding-feria", Rank::SemiDouble, Category::Feria);
    for (rank, category, want) in [
        (Rank::Double2ndClass, Category::BlessedVirgin, false),
        (Rank::GreaterDouble, Category::BlessedVirgin, true),
        (Rank::Double2ndClass, Category::Sunday, true),
    ] {
        let fol = f("following", rank, category);
        let r = resolve_concurrence(&day(Some(&prec), &[]), &day(Some(&fol), &[&memorial, &simple]));
        assert_eq!(r.owner, IOfFollowing);
        assert_eq!(r.commemorations.iter().any(|c| c.id == "memorial"), want, "{rank:?} {category:?}: {:?}", ids(&r.commemorations));
        assert_eq!(r.commemorations.iter().any(|c| c.id == "simple"), want, "{rank:?} {category:?}: {:?}", ids(&r.commemorations));
    }
    // The apostolic companion is part of the feast, not a Memorial.
    let fol = f("following", Rank::Double2ndClass, Category::Apostle);
    let r = resolve_concurrence(&day(Some(&prec), &[]), &day(Some(&fol), &[&companion]));
    assert!(r.commemorations.iter().any(|c| c.id == "commemoration-paul"), "{:?}", ids(&r.commemorations));
}
