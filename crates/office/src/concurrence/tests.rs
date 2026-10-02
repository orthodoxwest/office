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
    assert_eq!(winner(&benedict, &greater_sunday), IIOfPreceding);
    assert_eq!(winner(&greater_sunday, &benedict), IOfFollowing);
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
fn simple_and_feria_have_no_second_vespers() {
    assert!(!has_second_vespers(&feast("some-simple", Rank::Simple, Category::Martyr)));
    assert!(!has_second_vespers(&feast("some-feria", Rank::SemiDouble, Category::Feria)));
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
fn saturday_bvm_yields_to_sunday() {
    let bvm = f("saturday-office-bvm", Rank::Simple, Category::BlessedVirgin);
    let sunday = f("pentecost-sunday-5", Rank::SemiDouble, Category::Sunday);
    assert!(!has_second_vespers(&bvm));
    let r = resolve_concurrence(&day(Some(&bvm), &[]), &day(Some(&sunday), &[]));
    assert_eq!(r.owner, IOfFollowing);
    assert!(Arc::ptr_eq(r.feast.as_ref().unwrap(), &sunday));
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
fn two_ferias() {
    let current = named("current-memorial", "Current Memorial", Rank::Commemoration, Category::Martyr);
    let incoming = named("incoming-memorial", "Incoming Memorial", Rank::Commemoration, Category::Martyr);
    let r = resolve_concurrence(&day(None, &[&current]), &day(None, &[&incoming]));
    assert_eq!(r.owner, NotApplicable);
    assert!(same_list(&r.commemorations, &[&incoming]), "{:?}", ids(&r.commemorations));
    assert_trace_rule(&r.decisions, "commemoration:incoming-at-unowned-vespers");
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
fn occurrence_at_first_vespers() {
    for (comm, want) in [
        (feast("memorial", Rank::Commemoration, Category::Martyr), true),
        (feast("double", Rank::Double, Category::Martyr), true),
        (feast("september-ember-wednesday", Rank::PrivilegedFeria, Category::Feria), false),
        (feast("rogation-monday", Rank::PrivilegedFeria, Category::Feria), false),
        ((*with("comm-extra-08-22-vigil-of-st-bartholomew", Rank::Commemoration, Category::Feria, |x| x.is_vigil = true)).clone(), false),
        (feast("vigil-looking-memorial", Rank::Commemoration, Category::Martyr), true),
    ] {
        assert_eq!(occurrence_commemorated_at_first_vespers(&comm).0, want, "{}", comm.id);
    }
}

#[test]
fn occurrence_at_second_vespers() {
    let first = feast("first-class", Rank::Double1stClass, Category::Lord);
    let primary = (*with("corpus-christi", Rank::Double1stClass, Category::Lord, |x| x.primary_of_our_lord = true)).clone();
    let trinity = (*with("trinity-sunday", Rank::Double1stClass, Category::Lord, |x| x.primary_of_our_lord = true)).clone();
    let pentecost = (*with("pentecost", Rank::Double1stClass, Category::Lord, |x| x.primary_of_our_lord = true)).clone();
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
        (&first, feast("double", Rank::Double, Category::Martyr), false),
        // An Apostle stays through II Vespers of Trinity and Corpus Christi
        // only (Barnabas and St Paul; #379), not of other Primary Feasts.
        (&primary, feast("st-barnabas", Rank::GreaterDouble, Category::Apostle), true),
        (&trinity, feast("commemoration-st-paul-apostle", Rank::GreaterDouble, Category::Apostle), true),
        (&pentecost, feast("st-barnabas", Rank::GreaterDouble, Category::Apostle), false),
        (&primary, feast("st-basil", Rank::GreaterDouble, Category::ConfessorDoctor), false),
        (&first, feast("st-barnabas", Rank::GreaterDouble, Category::Apostle), false),
        (&second, feast("ss-peter-paul-octave-day-3", Rank::SemiDouble, Category::Martyr), false),
        (&greater, feast("double", Rank::Double, Category::Martyr), true),
        (&greater, feast("privileged-lenten-feria", Rank::PrivilegedFeria, Category::Feria), true),
        (&second, feast(FERIA_COMMEMORATION_ID, Rank::Commemoration, Category::Feria), true),
    ] {
        assert_eq!(occurrence_commemorated_at_second_vespers(Some(w), &comm).0, want, "{} under {}", comm.id, w.id);
    }
}

#[test]
fn second_vespers_filters_occurrence_commemorations() {
    let w = f("winner", Rank::GreaterDouble, Category::Confessor);
    let memorial = f("memorial", Rank::Commemoration, Category::Martyr);
    let simplified = f("simplified-double", Rank::Double, Category::Martyr);
    let seasonal = f("lenten-feria", Rank::PrivilegedFeria, Category::Feria);
    let displaced = f(FERIA_COMMEMORATION_ID, Rank::Commemoration, Category::Feria);
    let mut prec = day(Some(&w), &[&memorial, &simplified, &seasonal]);
    prec.feria_commemoration = Some(displaced.clone());
    let r = resolve_concurrence(&prec, &day(None, &[]));
    assert_eq!(r.owner, IIOfPreceding);
    assert!(same_list(&r.commemorations, &[&simplified, &seasonal, &displaced]), "{:?}", ids(&r.commemorations));
    assert!(r.decisions.iter().any(|d| d.rule == "commemoration:second-vespers-memorial-or-simple"
        && d.outcome == "suppressed"
        && d.detail.as_deref() == Some("memorial")));
}

#[test]
fn following_office_first_at_second_vespers() {
    let w = f("chair-peter", Rank::Double2ndClass, Category::Apostle);
    let following = f("st-matthias", Rank::Double2ndClass, Category::Apostle);
    let doctor = f("generic-confessor-doctor", Rank::Double, Category::ConfessorDoctor);
    let companion =
        with("commemoration-st-paul", Rank::Commemoration, Category::Apostle, |x| x.companion_of = Some("chair-peter".to_string()));
    let feria = f(FERIA_COMMEMORATION_ID, Rank::Commemoration, Category::Feria);
    let mut prec = day(Some(&w), &[&doctor, &companion]);
    prec.feria_commemoration = Some(feria.clone());
    let r = resolve_concurrence(&prec, &day(Some(&following), &[]));
    assert!(same_list(&r.commemorations, &[&following, &companion, &doctor, &feria]), "{:?}", ids(&r.commemorations));
    assert_eq!(r.following_office_commemoration_id.as_deref(), Some("st-matthias"));
}

#[test]
fn feria_boundaries() {
    // The following day's feria does not begin at I Vespers.
    let outgoing = named("privileged-lenten-feria", "Wednesday after Lent II", Rank::PrivilegedFeria, Category::Feria);
    let incoming = named("privileged-lenten-feria", "Thursday after Lent II", Rank::PrivilegedFeria, Category::Feria);
    let gregory = f("st-gregory", Rank::GreaterDouble, Category::ConfessorDoctor);
    let r = resolve_concurrence(&day(Some(&outgoing), &[]), &day(Some(&gregory), &[&incoming]));
    assert!(same_list(&r.commemorations, &[&outgoing]), "{:?}", ids(&r.commemorations));
    assert_trace_rule(&r.decisions, "commemoration:incoming-feria-not-at-vespers-boundary");

    // A displaced Advent feria is kept at the following I Vespers.
    let ambrose = f("st-ambrose", Rank::GreaterDouble, Category::ConfessorDoctor);
    let conception = f("conception-bvm", Rank::Double2ndClass, Category::BlessedVirgin);
    let feria = named(FERIA_COMMEMORATION_ID, "Monday after Advent II", Rank::Commemoration, Category::Feria);
    let mut prec = day(Some(&ambrose), &[]);
    prec.feria_commemoration = Some(feria.clone());
    let r = resolve_concurrence(&prec, &day(Some(&conception), &[]));
    assert_eq!(r.owner, IOfFollowing);
    assert!(same_list(&r.commemorations, &[&ambrose, &feria]), "{:?}", ids(&r.commemorations));
    assert_trace_rule(&r.decisions, "commemoration:first-vespers-seasonal-feria");

    // The following feria is not commemorated at II Vespers.
    let gregory2 = f("st-gregory", Rank::Double2ndClass, Category::ConfessorDoctor);
    let current = named(FERIA_COMMEMORATION_ID, "Thursday after Lent II", Rank::Commemoration, Category::Feria);
    let following = named("privileged-lenten-feria", "Friday after Lent II", Rank::PrivilegedFeria, Category::Feria);
    let mut prec = day(Some(&gregory2), &[]);
    prec.feria_commemoration = Some(current.clone());
    let r = resolve_concurrence(&prec, &day(Some(&following), &[]));
    assert!(same_list(&r.commemorations, &[&current]), "{:?}", ids(&r.commemorations));
    assert_trace_rule(&r.decisions, "commemoration:following-feria-not-at-second-vespers");
}

#[test]
fn vigil_of_epiphany_exception() {
    let w = f("holy-name-jesus", Rank::Double2ndClass, Category::Lord);
    let vigil = with("vigil-epiphany", Rank::SemiDouble, Category::Feria, |x| x.is_vigil = true);
    let telesphorus = f("st-telesphorus", Rank::Commemoration, Category::BishopMartyr);
    let r = resolve_concurrence(&day(Some(&w), &[]), &day(Some(&vigil), &[&telesphorus]));
    assert!(same_list(&r.commemorations, &[&vigil, &telesphorus]), "{:?}", ids(&r.commemorations));
    assert_eq!(r.following_office_commemoration_id.as_deref(), Some("vigil-epiphany"));
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
fn no_owner_combines_hour_eligible_commemorations() {
    let current_memorial = f("current-memorial", Rank::Commemoration, Category::Martyr);
    let current_double = f("current-double", Rank::Double, Category::Martyr);
    let incoming_memorial = f("incoming-memorial", Rank::Commemoration, Category::Martyr);
    let incoming_vigil = with("comm-extra-vigil", Rank::Commemoration, Category::Feria, |x| {
        x.name = "Vigil of an Apostle".to_string();
        x.is_vigil = true;
    });
    let r = resolve_concurrence(&day(None, &[&current_memorial, &current_double]), &day(None, &[&incoming_memorial, &incoming_vigil]));
    assert_eq!(r.owner, NotApplicable);
    assert!(same_list(&r.commemorations, &[&current_double, &incoming_memorial]), "{:?}", ids(&r.commemorations));
    assert_trace_rule(&r.decisions, "commemoration:second-vespers-included");
    assert_trace_rule(&r.decisions, "commemoration:incoming-at-unowned-vespers");
    assert_trace_rule(&r.decisions, "commemoration:first-vespers-feria-or-vigil-lauds-only");
}

#[test]
fn same_octave_boundary() {
    let parent = with("octave-feast", Rank::Double1stClass, Category::Lord, |x| {
        x.name = "Octave Feast".to_string();
        x.has_octave = true;
    });
    let next = named("octave-feast-octave-day-2", "Day II within the Octave Feast", Rank::Double1stClass, Category::Lord);
    let mut following = day(Some(&next), &[]);
    following.within_octave_of = Some("octave-feast".to_string());
    let r = resolve_concurrence(&day(Some(&parent), &[]), &following);
    assert!(r.commemorations.is_empty(), "{:?}", ids(&r.commemorations));
    assert_trace_rule(&r.decisions, "commemoration:same-octave-boundary");

    // A distinct occurring office inside the octave is not the octave office.
    let saint = named("saint", "Saint", Rank::Double, Category::Martyr);
    let sunday = named("sunday", "Sunday within the Octave", Rank::SemiDouble, Category::Sunday);
    let mut prec = day(Some(&saint), &[]);
    prec.within_octave_of = Some("octave-feast".to_string());
    let mut fol = day(Some(&sunday), &[]);
    fol.within_octave_of = Some("octave-feast".to_string());
    let r = resolve_concurrence(&prec, &fol);
    assert!(same_list(&r.commemorations, &[&saint]), "{:?}", ids(&r.commemorations));
    assert!(!r.decisions.iter().any(|d| d.rule == "commemoration:same-octave-boundary"));
}

#[test]
fn octave_celebration_parent_easter_week() {
    let easter = with("easter-sunday", Rank::Double1stClass, Category::Lord, |x| x.has_octave = true);
    assert_eq!(octave_celebration_parent(&day(Some(&easter), &[])), Some("easter-sunday"));
    for (id, want) in [("easter-monday", Some("easter-sunday")), ("easter-tuesday", Some("easter-sunday")), ("annunciation-bvm", None)] {
        let c = with(id, Rank::Double1stClass, Category::Lord, |x| x.octave_of = want.map(str::to_string));
        // An overlapping octave (St George) must not hide the day's own octave.
        let mut d = day(Some(&c), &[]);
        d.within_octave_of = Some("st-george".to_string());
        assert_eq!(octave_celebration_parent(&d), want, "{id}");
    }
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
        (&circumcision, double.clone(), true),
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
fn free_seasonal_feria_ends_before_sunday_vespers() {
    let mut prec = day(None, &[]);
    prec.date = Date::new(2026, 2, 21);
    prec.season = Season::Septuagesima;
    prec.temporal_week_id = Some("sexagesima".to_string());
    let quinquagesima = f("quinquagesima", Rank::SemiDouble, Category::Sunday);
    let mut fol = day(Some(&quinquagesima), &[]);
    fol.date = Date::new(2026, 2, 22);
    assert!(resolve_concurrence(&prec, &fol).commemorations.is_empty());
}

#[test]
fn second_vespers_retains_following_octave_office() {
    let sunday = f("sunday", Rank::SemiDouble, Category::Sunday);
    for (name, following, want) in [
        ("weekday without I Vespers", Some(f("example-octave-day-5", Rank::SemiDouble, Category::Lord)), Some("example")),
        ("terminal day with I Vespers", Some(f("example-octave-day", Rank::Double, Category::Lord)), Some("example")),
        ("occurring saint inside octave", Some(f("saint", Rank::Double, Category::Confessor)), None),
        ("unnamed feria", None, None),
    ] {
        let mut prec = day(Some(&sunday), &[]);
        prec.within_octave_of = Some("example".to_string());
        let mut fol = day(following.as_ref(), &[]);
        fol.within_octave_of = Some("example".to_string());
        let r = resolve_concurrence(&prec, &fol);
        assert_eq!(r.owner, IIOfPreceding, "{name}");
        assert_eq!(r.following_office_octave_of.as_deref(), want, "{name}");
    }
}

#[test]
fn boundary_trace_rules() {
    let w = f("winner", Rank::Double2ndClass, Category::Martyr);
    let loser = f("loser", Rank::Simple, Category::Martyr);
    let incoming = f("incoming", Rank::Commemoration, Category::Martyr);
    let following = day(Some(&loser), &[&incoming]);
    // The preceding day has no octave office.
    let (comms, decisions) = boundary_commemorations(Some(&w), Some(&loser), &day(None, &[]), &following, true, false);
    assert!(comms.is_empty(), "{:?}", ids(&comms));
    assert_trace_rule(&decisions, "commemoration:following-office-at-second-vespers-simple-or-memorial");
    assert_trace_rule(&decisions, "commemoration:incoming-at-second-vespers");

    let r = resolve_concurrence(&day(Some(&w), &[]), &day(None, &[&incoming]));
    assert_eq!(r.rule, "concurrence:preceding-only");
    assert_trace_rule(&r.decisions, "commemoration:incoming-at-second-vespers");

    let r = resolve_concurrence(&day(None, &[]), &day(None, &[&incoming]));
    assert_eq!(r.owner, NotApplicable);
    assert!(same_list(&r.commemorations, &[&incoming]));
    assert_trace_rule(&r.decisions, "commemoration:incoming-at-unowned-vespers");
}
