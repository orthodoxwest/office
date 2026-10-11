use calendar::{Category, Feast, MoveableDates, Rank, Season};

use crate::day::Day;
use crate::hourdef::Condition;
use crate::preces::*;
use crate::testutil::{arc, date, day, feast, texts};

fn ranked(id: &str, category: Option<Category>, rank: Rank) -> Feast {
    let mut f = feast(id, category);
    f.rank = rank;
    f
}

/// A Lenten violet day.
fn make_day(m: i32, d: i32, celebration: Option<Feast>, comms: Vec<Feast>, within: Option<&str>) -> Day {
    let mut x = day(date(2026, m, d), Season::Lent);
    x.color = calendar::Color::Violet;
    x.celebration = celebration.map(arc);
    x.commemorations = comms.into_iter().map(arc).collect();
    x.within_octave_of = within.map(str::to_string);
    x
}

fn suffrage_day(m: i32, d: i32, season: Season, celebration: Feast, comms: Vec<Feast>) -> Day {
    let mut x = day(date(2026, m, d), season);
    x.celebration = Some(arc(celebration));
    x.commemorations = comms.into_iter().map(arc).collect();
    x
}

fn eval(condition: &str, d: &Day, moveable: Option<&MoveableDates>) -> bool {
    Condition::parse(condition).is_ok_and(|c| c.evaluate(d, moveable, &texts(&[])))
}

#[test]
fn hour_conditions() {
    let m = MoveableDates::compute(2026);
    let plain = |mm, dd| day(date(2026, mm, dd), Season::Pentecost);
    let double = || ranked("test", None, Rank::Double);
    let with = |m, d, season, f: Option<Feast>| {
        let mut x = day(date(2026, m, d), season);
        x.celebration = f.map(arc);
        x
    };
    let feria = || feast("feria", Some(Category::Feria));
    let bvm = || feast("future-bvm-office", Some(Category::BlessedVirgin));
    for (condition, d, want) in [
        ("not-weekday-sunday", plain(3, 15), false),
        ("not-weekday-sunday", plain(3, 16), true),
        ("not-if-preces", make_day(3, 16, None, vec![], None), false),
        ("not-if-preces", make_day(3, 15, Some(double()), vec![], None), true),
        ("not-feast-easter-sunday", make_day(3, 16, Some(feast("some-feast", None)), vec![], None), true),
        ("not-feast-easter-sunday", make_day(4, 12, Some(feast("easter-sunday", None)), vec![], None), false),
        ("weekday-sunday,not-if-preces", make_day(3, 15, Some(double()), vec![], None), true),
        ("weekday-sunday,not-if-preces", make_day(3, 15, None, vec![], None), false),
        ("weekday-monday,weekday-monday", plain(3, 16), true),
        ("weekday-monday,weekday-sunday", plain(3, 16), false),
        ("feast-easter-sunday,weekday-sunday", make_day(4, 12, Some(feast("easter-sunday", None)), vec![], None), true),
        ("feast-easter-sunday,weekday-monday", make_day(4, 12, Some(feast("easter-sunday", None)), vec![], None), false),
        ("feast-christmas", make_day(12, 25, Some(feast("christmas", None)), vec![], None), true),
        ("feast-christmas", make_day(12, 26, Some(feast("st-stephen", None)), vec![], None), false),
        ("feast-christmas", make_day(3, 16, None, vec![], None), false),
        ("some-unknown-condition", plain(3, 16), false),
        ("not-some-unknown-condition", plain(3, 16), false),
        ("is-ferial", with(3, 16, Season::Lent, Some(feast("feria-lent-monday", Some(Category::Feria)))), true),
        ("is-ferial", with(3, 18, Season::Lent, Some(feast("st-cyril", Some(Category::ConfessorDoctor)))), false),
        ("is-ferial", with(3, 15, Season::Lent, Some(feast("sunday-lent", Some(Category::Sunday)))), false),
        ("is-ferial", with(3, 16, Season::Lent, None), true),
        ("season-easter", with(4, 15, Season::Easter, None), true),
        ("season-easter", with(3, 16, Season::Lent, None), false),
        ("season-passiontide", with(3, 30, Season::Passiontide, None), true),
        ("not-season-easter", with(3, 16, Season::Lent, None), true),
        ("is-ferial,not-season-easter", with(3, 16, Season::Lent, Some(feria())), true),
        ("is-ferial,season-easter", with(4, 21, Season::Easter, Some(feria())), true),
        ("bvm-suffrage-form", suffrage_day(1, 1, Season::Pentecost, bvm(), vec![]), true),
        ("bvm-suffrage-form", suffrage_day(1, 1, Season::Pentecost, feria(), vec![bvm()]), true),
        ("bvm-suffrage-form", suffrage_day(1, 1, Season::Pentecost, feria(), vec![]), false),
        ("not-bvm-suffrage-form", suffrage_day(1, 1, Season::Pentecost, feria(), vec![]), true),
    ] {
        assert_eq!(eval(condition, &d, Some(&m)), want, "{condition} on {}", d.date);
    }
}

#[test]
fn suffrage() {
    let sunday = |id: &str, rank| ranked(id, Some(Category::Sunday), rank);
    let memorial = |id: &str| ranked(id, None, Rank::Commemoration);
    for (name, d, want) in [
        ("Septuagesima Sunday", suffrage_day(2, 8, Season::Septuagesima, sunday("septuagesima", Rank::Double2ndClass), vec![]), true),
        (
            "Sexagesima with a memorial",
            suffrage_day(
                2,
                15,
                Season::Septuagesima,
                sunday("sexagesima", Rank::Double2ndClass),
                vec![memorial("comm-02-15-ss-faustinus-and-jovita-martyrs")],
            ),
            true,
        ),
        (
            "Lent Sunday with a memorial",
            suffrage_day(
                3,
                1,
                Season::Lent,
                sunday("lent-sunday-1", Rank::Double1stClass),
                vec![memorial("comm-03-01-st-david-of-wales-bishop-and-confessor")],
            ),
            true,
        ),
        ("Lent Sunday", suffrage_day(3, 15, Season::Lent, sunday("lent-sunday-3", Rank::Double1stClass), vec![]), true),
        (
            "Pentecost Sunday",
            suffrage_day(
                7,
                12,
                Season::Pentecost,
                sunday("pentecost-sunday-6", Rank::SemiDouble),
                vec![memorial("comm-07-12-ss-nabor-and-felix-martyrs")],
            ),
            true,
        ),
        (
            "simplified Double commemoration",
            suffrage_day(
                1,
                18,
                Season::Epiphany,
                sunday("epiphany-sunday-2", Rank::SemiDouble),
                vec![ranked("chair-peter-rome", None, Rank::GreaterDouble)],
            ),
            false,
        ),
        (
            "feast office",
            suffrage_day(7, 14, Season::Pentecost, ranked("some-semidouble-feast", Some(Category::Martyr), Rank::SemiDouble), vec![]),
            false,
        ),
    ] {
        assert_eq!(should_say_suffrage(Some(&d)), want, "{name}");
    }
    let mut within = suffrage_day(
        7,
        5,
        Season::Pentecost,
        sunday("pentecost-sunday-5", Rank::SemiDouble),
        vec![ranked("ss-peter-paul-octave-day-7", None, Rank::SemiDouble)],
    );
    within.within_octave_of = Some("ss-peter-paul".into());
    assert!(!should_say_suffrage(Some(&within)));
    assert_eq!(suffrage_disposition(None), (false, SUFFRAGE_SUPPRESSED_OUT_OF_SEASON));
    // Not within the Epiphany octave (7–13 January).
    for (d, want) in [(6, true), (7, false), (13, false), (14, true)] {
        let x = suffrage_day(1, d, Season::Epiphany, feast("feria", Some(Category::Feria)), vec![]);
        assert_eq!(should_say_suffrage(Some(&x)), want, "January {d}");
    }
}

#[test]
fn cross_commemoration() {
    let m = MoveableDates::compute(2026);
    let feria = suffrage_day(4, 21, Season::Easter, ranked("feria", Some(Category::Feria), Rank::Commemoration), vec![]);
    assert!(should_say_cross_commemoration(&feria, Some(&m)));
    let sunday = suffrage_day(5, 17, Season::Easter, ranked("easter-sunday-5", Some(Category::Sunday), Rank::SemiDouble), vec![]);
    assert!(should_say_cross_commemoration(&sunday, Some(&m)));
    let feast_office =
        suffrage_day(4, 21, Season::Easter, ranked("some-semidouble-feast", Some(Category::Martyr), Rank::SemiDouble), vec![]);
    assert!(!should_say_cross_commemoration(&feast_office, Some(&m)));
    let mut within = suffrage_day(4, 26, Season::Easter, ranked("easter-sunday-2", Some(Category::Sunday), Rank::SemiDouble), vec![]);
    within.within_octave_of = Some("st-george".into());
    assert!(!should_say_cross_commemoration(&within, Some(&m)));
    assert!(!should_say_cross_commemoration(&feria, None));

    // Diurnal p. 146: from Monday after Low Sunday through the Vigil of the
    // Ascension (Easter 2026 is 12 April; #356).
    let feria_on = |d: i32| {
        let (mm, dd) = if d <= 30 { (4, d) } else { (5, d - 30) };
        suffrage_day(mm, dd, Season::Easter, ranked("feria", Some(Category::Feria), Rank::Commemoration), vec![])
    };
    for (d, want) in [(19, false), (20, true), (50, true), (51, false)] {
        let x = feria_on(d);
        assert_eq!(should_say_cross_commemoration(&x, Some(&m)), want, "{}", x.date);
    }

    // "Except when there has been Commemoration of a Double, even in
    // concurrence, or an Office or Commemoration of any Octave"; a Memorial
    // does not suppress it (2026 ordo, 18–19 May).
    let rogation = || ranked("rogation-monday", Some(Category::Feria), Rank::PrivilegedFeria);
    let venantius = suffrage_day(5, 18, Season::Easter, rogation(), vec![ranked("st-venantius", Some(Category::Martyr), Rank::Double)]);
    assert!(!should_say_cross_commemoration(&venantius, Some(&m)));
    let octave = suffrage_day(
        4,
        26,
        Season::Easter,
        ranked("easter-sunday-2", Some(Category::Sunday), Rank::SemiDouble),
        vec![ranked("st-george-octave-day-4", Some(Category::Martyr), Rank::SemiDouble)],
    );
    assert!(!should_say_cross_commemoration(&octave, Some(&m)));
    let memorial = suffrage_day(
        5,
        19,
        Season::Easter,
        ranked("feria", Some(Category::Feria), Rank::Commemoration),
        vec![ranked("st-pudentiana", Some(Category::Virgin), Rank::Commemoration)],
    );
    assert!(should_say_cross_commemoration(&memorial, Some(&m)));
}

#[test]
fn vigil_of_all_saints_omits_the_suffrage() {
    // Diurnal p. 636: "At Lauds the Suffrage of All Saints is not said" (#471).
    let vigil = suffrage_day(10, 31, Season::Pentecost, ranked("vigil-of-all-saints", Some(Category::Feria), Rank::Simple), vec![]);
    assert_eq!(suffrage_disposition(Some(&vigil)), (false, SUFFRAGE_SUPPRESSED_ALL_SAINTS_VIGIL));
    let feria = suffrage_day(10, 30, Season::Pentecost, ranked("feria", Some(Category::Feria), Rank::Commemoration), vec![]);
    assert!(should_say_suffrage(Some(&feria)));
}

#[test]
fn should_say_preces_table() {
    let m = MoveableDates::compute(2026);
    let r = |id: &str, rank| ranked(id, None, rank);
    let rc = |id: &str, rank, c| ranked(id, Some(c), rank);
    let mut easter_sunday = make_day(5, 10, Some(rc("easter-sunday-4", Rank::SemiDouble, Category::Sunday)), vec![], None);
    easter_sunday.season = Season::Easter;
    let mut easter_feria = make_day(5, 11, None, vec![], None);
    easter_feria.season = Season::Easter;
    for (name, d, want) in [
        ("ferial day", make_day(3, 11, None, vec![], None), true),
        ("simple feast", make_day(3, 11, Some(r("test", Rank::Simple)), vec![], None), true),
        ("semi-double feast", make_day(3, 15, Some(r("test", Rank::SemiDouble)), vec![], None), true),
        ("double feast", make_day(3, 15, Some(r("test", Rank::Double)), vec![], None), false),
        ("1st class feast", make_day(4, 12, Some(r("easter-sunday", Rank::Double1stClass)), vec![], None), false),
        ("elevated Sunday", make_day(3, 15, Some(rc("lent-sunday-3", Rank::Double1stClass, Category::Sunday)), vec![], None), true),
        ("elevated feria", make_day(2, 25, Some(rc("ash-wednesday", Rank::Double1stClass, Category::Feria)), vec![], None), true),
        (
            "elevated Sunday with Double commemoration",
            make_day(3, 15, Some(rc("lent-sunday-3", Rank::Double1stClass, Category::Sunday)), vec![r("test", Rank::Double)], None),
            false,
        ),
        ("Vigil of Pentecost", make_day(5, 30, Some(rc("vigil-pentecost", Rank::Double1stClass, Category::Feria)), vec![], None), false),
        ("Vigil of the Nativity", make_day(12, 24, Some(rc("vigil-nativity", Rank::Double1stClass, Category::Feria)), vec![], None), false),
        ("All Souls", make_day(11, 2, Some(rc("all-souls", Rank::Double, Category::Feria)), vec![], None), false),
        ("within octave", make_day(4, 13, None, vec![], Some("easter-sunday")), false),
        ("simple octave-day office", make_day(1, 2, Some(r("octave-day-st-stephen", Rank::Simple)), vec![], None), false),
        ("double commemoration", make_day(3, 11, None, vec![r("test", Rank::Double)], None), false),
        ("octave commemoration", make_day(3, 11, None, vec![r("test-octave-day-3", Rank::SemiDouble)], None), false),
        ("vigil of Epiphany", make_day(1, 5, None, vec![], None), false),
        ("Friday after Ascension octave", make_day(5, 29, None, vec![], None), false),
        // Ordo day lines over Diurnal p. 7, pending a ruling (#660).
        ("Eastertide Sunday (#660)", easter_sunday, false),
        ("Eastertide feria", easter_feria, true),
    ] {
        assert_eq!(should_say_preces(Some(&d), Some(&m)), want, "{name}");
    }
}
