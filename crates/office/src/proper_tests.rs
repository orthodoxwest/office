use calendar::{Category, Rank, Season, Weekday};

use crate::proper::{
    derive_proper_name_from_title, feast_proper_name, resolve_proper_collect_text, resolve_proper_text, substitute_proper_name,
};
use crate::testutil::{celebrating, date, day, feast, texts, zero_date};

fn resolve(d: &crate::Day, hour: &str, r: &str, t: &crate::texts::OfficeTexts) -> (String, String) {
    resolve_proper_text(d, hour, r, t)
}

#[test]
fn commons() {
    let t = texts(&[
        ("commons/martyr/antiphon", "Martyr common antiphon"),
        ("commons/martyr/antiphon-lauds", "Martyr common lauds antiphon"),
        ("ordinary/lauds/antiphon", "Ordinary antiphon"),
    ]);
    let d = celebrating(date(2026, 3, 16), Season::Lent, feast("unknown-feast", Some(Category::Martyr)));
    assert_eq!(resolve(&d, "lauds", "antiphon", &t).0, "Martyr common lauds antiphon");
    let t2 = texts(&[("commons/martyr/antiphon", "Martyr common antiphon"), ("ordinary/lauds/antiphon", "Ordinary antiphon")]);
    assert_eq!(resolve(&d, "lauds", "antiphon", &t2).0, "Martyr common antiphon");
}

#[test]
fn hour_qualified() {
    let t = texts(&[
        ("proper/christmas/antiphon-lauds", "Christmas lauds antiphon"),
        ("proper/christmas/antiphon", "Christmas generic antiphon"),
        ("ordinary/lauds/antiphon", "Ordinary antiphon"),
    ]);
    let d = celebrating(date(2026, 12, 25), Season::Christmas, feast("christmas", None));
    assert_eq!(resolve(&d, "lauds", "antiphon", &t).0, "Christmas lauds antiphon");
}

#[test]
fn greater_antiphon_boundaries() {
    let t = texts(&[
        ("seasonal/advent/magnificat-antiphon-december-16", "Out-of-range December 16 antiphon"),
        ("seasonal/advent/magnificat-antiphon-december-17", "O Wisdom"),
        ("seasonal/advent/magnificat-antiphon-december-18", "O Adonai"),
        ("seasonal/advent/magnificat-antiphon-december-19", "O Root of Jesse"),
        ("seasonal/advent/magnificat-antiphon-december-23", "O Emmanuel"),
        ("seasonal/advent/magnificat-antiphon-december-24", "Out-of-range December 24 antiphon"),
        ("ordinary/vespers/magnificat-antiphon", "Ordinary Vespers antiphon"),
        ("ordinary/lauds/magnificat-antiphon", "Ordinary Lauds antiphon"),
    ]);
    let ordinary = ("Ordinary Vespers antiphon", "ordinary/vespers/magnificat-antiphon");
    for (name, (m, dd), season, hour, first, want) in [
        (
            "December 17 included",
            (12, 17),
            Season::Advent,
            "vespers",
            false,
            ("O Wisdom", "seasonal/advent/magnificat-antiphon-december-17"),
        ),
        (
            "December 23 included",
            (12, 23),
            Season::Advent,
            "vespers",
            false,
            ("O Emmanuel", "seasonal/advent/magnificat-antiphon-december-23"),
        ),
        ("December 16 excluded", (12, 16), Season::Advent, "vespers", false, ordinary),
        ("December 24 excluded", (12, 24), Season::Advent, "vespers", false, ordinary),
        (
            "first Vespers uses preceding evening",
            (12, 18),
            Season::Advent,
            "vespers",
            true,
            ("O Wisdom", "seasonal/advent/magnificat-antiphon-december-17"),
        ),
        ("first Vespers shifts outside the window", (12, 17), Season::Advent, "vespers", true, ordinary),
        ("Lauds excluded", (12, 17), Season::Advent, "lauds", false, ("Ordinary Lauds antiphon", "ordinary/lauds/magnificat-antiphon")),
        ("non-Advent season excluded", (12, 17), Season::Christmas, "vespers", false, ordinary),
        ("non-December month excluded", (11, 17), Season::Advent, "vespers", false, ordinary),
    ] {
        let mut d = day(date(2026, m, dd), season);
        d.first_vespers = first;
        let got = resolve(&d, hour, "magnificat-antiphon", &t);
        assert_eq!((got.0.as_str(), got.1.as_str()), want, "{name}");
    }
}

#[test]
fn greater_antiphon_precedence() {
    let t = texts(&[
        ("seasonal/advent/magnificat-antiphon-december-18", "O Adonai"),
        ("proper/test-sunday/magnificat-antiphon", "Sunday proper antiphon"),
        ("proper/test-feria/magnificat-antiphon", "Feria proper antiphon"),
        ("proper/test-saint/magnificat-antiphon", "Saint proper antiphon"),
        ("commons/martyr/magnificat-antiphon", "Martyr common antiphon"),
    ]);
    let o = ("O Adonai", "seasonal/advent/magnificat-antiphon-december-18");
    for (id, cat, want) in [
        ("test-sunday", Category::Sunday, o),
        ("test-feria", Category::Feria, o),
        ("test-saint", Category::Martyr, ("Saint proper antiphon", "proper/test-saint/magnificat-antiphon")),
        ("common-only-saint", Category::Martyr, o),
    ] {
        let d = celebrating(date(2026, 12, 18), Season::Advent, feast(id, Some(cat)));
        let got = resolve(&d, "vespers", "magnificat-antiphon", &t);
        assert_eq!((got.0.as_str(), got.1.as_str()), want, "{id}");
    }
}

#[test]
fn proper_id_redirects() {
    let t = texts(&[("proper/pentecost-sunday-23/collect", "XXIII Pentecost collect")]);
    let mut f = feast("epiphany-sunday-7", Some(Category::Sunday));
    f.proper_id = Some("pentecost-sunday-23".into());
    let d = celebrating(date(2026, 2, 21), Season::Epiphany, f);
    assert_eq!(resolve(&d, "lauds", "collect", &t), ("XXIII Pentecost collect".into(), "proper/pentecost-sunday-23/collect".into()));

    let t = texts(&[
        ("proper/epiphany-sunday-within-octave/benedictus-antiphon", "The Child Jesus remained"),
        ("proper/epiphany-sunday-1/benedictus-antiphon", "First Sunday after Epiphany"),
    ]);
    let mut f = feast("epiphany-sunday-1", Some(Category::Sunday));
    f.proper_id = Some("epiphany-sunday-within-octave".into());
    let d = celebrating(date(2026, 1, 11), Season::Epiphany, f);
    assert_eq!(
        resolve(&d, "lauds", "benedictus-antiphon", &t),
        ("The Child Jesus remained".into(), "proper/epiphany-sunday-within-octave/benedictus-antiphon".into())
    );
}

#[test]
fn privileged_feria_uses_weekday_temporal_text() {
    let t = texts(&[
        ("proper/lent-sunday-2/benedictus-antiphon", "Sunday Benedictus antiphon"),
        ("proper/lent-sunday-2/benedictus-antiphon-wednesday", "Wednesday Benedictus antiphon"),
    ]);
    let mut f = feast("privileged-lenten-feria", Some(Category::Feria));
    f.rank = Rank::PrivilegedFeria;
    f.proper_id = Some("lent-sunday-2".into());
    let mut d = celebrating(date(2026, 3, 11), Season::Lent, f);
    d.temporal_week_id = Some("lent-sunday-2".into());
    assert_eq!(
        resolve(&d, "lauds", "benedictus-antiphon", &t),
        ("Wednesday Benedictus antiphon".into(), "proper/lent-sunday-2/benedictus-antiphon-wednesday".into())
    );
}

#[test]
fn named_privileged_feria_uses_own_proper() {
    for (id, weekday) in
        [("lent-ember-wednesday", Weekday::Wednesday), ("lent-ember-friday", Weekday::Friday), ("lent-ember-saturday", Weekday::Saturday)]
    {
        let proper_ref = format!("proper/{id}/benedictus-antiphon");
        let weekly = format!("proper/lent-sunday-1/benedictus-antiphon-{}", weekday.name().to_lowercase());
        let t = texts(&[(&proper_ref, "Named Ember antiphon"), (&weekly, "Weekly feria antiphon")]);
        let mut f = feast(id, Some(Category::Feria));
        f.rank = Rank::PrivilegedFeria;
        let mut d = celebrating(date(2026, 3, 1 + weekday.number()), Season::Lent, f);
        d.temporal_week_id = Some("lent-sunday-1".into());
        assert_eq!(resolve(&d, "lauds", "benedictus-antiphon", &t), ("Named Ember antiphon".into(), proper_ref.clone()), "{id}");
    }
}

#[test]
fn weekday_ordinary() {
    let t = texts(&[("ordinary/lauds/hymn-monday", "Monday lauds hymn"), ("ordinary/lauds/hymn", "Generic lauds hymn")]);
    assert_eq!(resolve(&day(date(2026, 3, 16), Season::Lent), "lauds", "hymn", &t).0, "Monday lauds hymn");

    let t = texts(&[
        ("ordinary/terce/psalm-antiphon", "Weekday Terce antiphon"),
        ("ordinary/terce/psalm-antiphon-sunday", "Sunday Terce antiphon"),
        ("ordinary/terce/psalm-antiphon-monday", "Monday Terce antiphon"),
        ("ordinary/terce/chapter", "Weekday Terce chapter"),
        ("ordinary/terce/chapter-sunday", "Sunday Terce chapter"),
    ]);
    for (dd, r, want) in [
        (19, "psalm-antiphon-2", ("Sunday Terce antiphon", "ordinary/terce/psalm-antiphon-sunday")),
        (20, "psalm-antiphon-2", ("Monday Terce antiphon", "ordinary/terce/psalm-antiphon-monday")),
        (21, "psalm-antiphon-2", ("Weekday Terce antiphon", "ordinary/terce/psalm-antiphon")),
        (19, "chapter", ("Sunday Terce chapter", "ordinary/terce/chapter-sunday")),
        (21, "chapter", ("Weekday Terce chapter", "ordinary/terce/chapter")),
    ] {
        let got = resolve(&day(date(2026, 7, dd), Season::Pentecost), "terce", r, &t);
        assert_eq!((got.0.as_str(), got.1.as_str()), want, "{dd} {r}");
    }
}

#[test]
fn sunday_first_vespers_uses_saturday_psalm_antiphon() {
    let mut d = celebrating(date(2026, 3, 22), Season::Lent, feast("lent-sunday-4", Some(Category::Sunday)));
    d.first_vespers = true;
    let t = texts(&[
        ("proper/lent-sunday-4/psalm-antiphon-1", "Sunday Lauds antiphon"),
        ("ordinary/vespers/psalm-antiphon-1-saturday", "Saturday Vespers antiphon"),
    ]);
    assert_eq!(
        resolve(&d, "vespers", "psalm-antiphon-1", &t),
        ("Saturday Vespers antiphon".into(), "ordinary/vespers/psalm-antiphon-1-saturday".into())
    );
    let t = texts(&[
        ("proper/lent-sunday-4/psalm-antiphon-1-vespers", "Sunday Vespers antiphon"),
        ("ordinary/vespers/psalm-antiphon-1-saturday", "Saturday Vespers antiphon"),
    ]);
    assert_eq!(
        resolve(&d, "vespers", "psalm-antiphon-1", &t),
        ("Sunday Vespers antiphon".into(), "proper/lent-sunday-4/psalm-antiphon-1-vespers".into())
    );
    let t = texts(&[
        ("proper/easter-sunday-3/psalm-antiphon-1", "Sunday Lauds antiphon"),
        ("seasonal/easter/psalm-antiphon-1", "Alleluia, alleluia, alleluia."),
        ("ordinary/vespers/psalm-antiphon-1-saturday", "Saturday Vespers antiphon"),
    ]);
    let mut d = celebrating(date(2026, 3, 22), Season::Easter, feast("easter-sunday-3", Some(Category::Lord)));
    d.first_vespers = true;
    assert_eq!(
        resolve(&d, "vespers", "psalm-antiphon-1", &t),
        ("Alleluia, alleluia, alleluia.".into(), "seasonal/easter/psalm-antiphon-1".into())
    );
}

#[test]
fn seasonal_first_vespers_only_before_sunday() {
    let t = texts(&[
        ("proper/st-joseph/short-responsory-vespers", "Proper feast responsory"),
        ("seasonal/lent/short-responsory-first-vespers", "Saturday Lent responsory"),
        ("ordinary/vespers/short-responsory", "Ordinary responsory"),
    ]);
    let mut weekday_feast = celebrating(date(2026, 3, 19), Season::Lent, feast("st-joseph", Some(Category::Confessor)));
    weekday_feast.first_vespers = true;
    assert_eq!(
        resolve(&weekday_feast, "vespers", "short-responsory", &t),
        ("Proper feast responsory".into(), "proper/st-joseph/short-responsory-vespers".into())
    );
    let mut sunday = celebrating(date(2026, 3, 22), Season::Lent, feast("passion-sunday", Some(Category::Sunday)));
    sunday.first_vespers = true;
    assert_eq!(
        resolve(&sunday, "vespers", "short-responsory", &t),
        ("Saturday Lent responsory".into(), "seasonal/lent/short-responsory-first-vespers".into())
    );
}

#[test]
fn shared_fallback_and_not_found() {
    let d = day(date(2026, 3, 16), Season::Lent);
    assert_eq!(resolve(&d, "lauds", "collect", &texts(&[("ordinary/shared/collect", "Shared collect")])).0, "Shared collect");
    assert_eq!(resolve(&d, "lauds", "missing-ref", &texts(&[])), ("[Proper text not found: missing-ref]".into(), "missing-ref".into()));
}

#[test]
fn proper_names() {
    assert_eq!(substitute_proper_name("feast of N. the saint", ""), "feast of N. the saint");
    assert_eq!(substitute_proper_name("feast of N. the saint", "Ambrose"), "feast of Ambrose the saint");
    assert_eq!(derive_proper_name_from_title("St Agatha, Virgin & Martyr"), "Agatha");
    let mut f = feast("x", None);
    f.name = "St Agatha, Virgin & Martyr".into();
    assert_eq!(feast_proper_name(&f), "Agatha");
    f.proper_name = Some("Agatha of Catania".into());
    assert_eq!(feast_proper_name(&f), "Agatha of Catania");
}

#[test]
fn minor_hours_reuse_lauds_collect() {
    let t = texts(&[("proper/st-x/collect-lauds", "Lauds collect"), ("proper/st-x/collect-terce", "Terce collect")]);
    let d = celebrating(date(2026, 6, 16), Season::Pentecost, feast("st-x", Some(Category::Confessor)));
    for hour in ["terce", "sext", "none"] {
        assert_eq!(resolve_proper_collect_text(&d, hour, &t), ("Lauds collect".into(), "proper/st-x/collect-lauds".into()), "{hour}");
    }
}

#[test]
fn n_substitution() {
    let t = texts(&[("commons/confessor/collect", "O God, bless N. thy confessor"), ("ordinary/lauds/antiphon", "O holy N., pray for us")]);
    let mut f = feast("st-benedict", Some(Category::Confessor));
    f.proper_name = Some("Benedict".into());
    assert_eq!(resolve(&celebrating(date(2026, 3, 21), Season::Lent, f), "lauds", "collect", &t).0, "O God, bless Benedict thy confessor");
    let f = feast("st-benedict", Some(Category::Confessor));
    assert_eq!(resolve(&celebrating(date(2026, 3, 21), Season::Lent, f), "lauds", "collect", &t).0, "O God, bless N. thy confessor");
    let mut f = feast("st-unknown", None);
    f.proper_name = Some("Patrick".into());
    assert_eq!(resolve(&celebrating(date(2026, 3, 21), Season::Pentecost, f), "lauds", "antiphon", &t).0, "O holy Patrick, pray for us");
}

#[test]
fn paschal_commons() {
    let t = texts(&[
        ("commons/apostle-paschal/antiphon", "Paschal apostle antiphon, alleluia"),
        ("commons/apostle/antiphon", "Regular apostle antiphon"),
        ("commons/apostle/collect", "Regular apostle collect"),
    ]);
    let easter = celebrating(date(2026, 4, 20), Season::Easter, feast("st-mark", Some(Category::Apostle)));
    assert_eq!(resolve(&easter, "lauds", "antiphon", &t).0, "Paschal apostle antiphon, alleluia");
    assert_eq!(resolve(&easter, "lauds", "collect", &t).0, "Regular apostle collect");
    let lent = celebrating(date(2026, 3, 16), Season::Lent, feast("st-mark", Some(Category::Apostle)));
    assert_eq!(resolve(&lent, "lauds", "antiphon", &t).0, "Regular apostle antiphon");
}

#[test]
fn paschal_commons_use_explicit_hymns() {
    let t = crate::testutil::live_texts();
    for (category, hour, r, want) in [
        (Category::Martyr, "lauds", "hymn", "Thou foll'west, Martyr of thy God"),
        (Category::Martyrs, "lauds", "hymn", "All glorious King of Martyrs thou"),
        (Category::Martyrs, "vespers", "hymn", "All glorious King of Martyrs thou"),
        (Category::BishopMartyr, "lauds", "hymn", "Thou foll'west, Martyr of thy God"),
        (Category::BishopMartyr, "vespers", "hymn", "Of all thy warrior Saints"),
        (Category::Evangelist, "lauds", "hymn", "In this our bright and Paschal day"),
        (Category::Evangelist, "vespers", "hymn", "Th'Apostles' hearts were full of pain"),
        (Category::Confessor, "lauds", "hymn", "O Jesu, Crown above the sky"),
        (Category::Confessor, "vespers", "hymn", "This the Confessor of the Lord"),
        (Category::ConfessorDoctor, "lauds", "hymn", "Jesu, the world's Redeemer"),
        (Category::ConfessorDoctor, "vespers", "hymn", "This the Confessor of the Lord"),
        (Category::Virgin, "lauds", "hymn", "Jesu, the Virgins'"),
        (Category::Virgin, "vespers", "hymn", "Jesu, the Virgins'"),
        (Category::VirginMartyr, "lauds", "hymn", "Jesu, the Virgins'"),
        (Category::VirginMartyr, "vespers", "hymn", "Jesu, the Virgins'"),
        (Category::HolyWoman, "lauds", "hymn", "High let us all our voices raise"),
        (Category::HolyWoman, "vespers", "hymn", "High let us all our voices raise"),
        (Category::Martyrs, "vespers", "psalm-antiphon-4", "Then shall the righteous"),
        (Category::Martyrs, "vespers", "short-responsory", "Light perpetual shall shine"),
        (Category::Martyrs, "vespers", "versicle-first", "O ye holy and righteous"),
    ] {
        let mut d = celebrating(date(2026, 4, 23), Season::Easter, feast("test-martyrs", Some(category)));
        let mut r = r;
        if r == "versicle-first" {
            d.first_vespers = true;
            r = "versicle";
        }
        let (text, source) = resolve(&d, hour, r, t);
        assert!(text.contains(want), "{category} {hour} {r}: {text:?}");
        assert!(source.starts_with(&format!("commons/{category}-paschal/")), "{category} {hour} {r}: {source}");
    }
}

#[test]
fn seasonal_hour_qualified_and_indexed_fallbacks() {
    let t = texts(&[
        ("seasonal/lent/antiphon-lauds", "Lent lauds antiphon"),
        ("seasonal/lent/antiphon", "Lent generic antiphon"),
        ("ordinary/lauds/antiphon", "Ordinary antiphon"),
    ]);
    assert_eq!(resolve(&day(date(2026, 3, 16), Season::Lent), "lauds", "antiphon", &t).0, "Lent lauds antiphon");

    let t = texts(&[
        ("proper/st-mark/psalm-antiphon", "Generic proper antiphon"),
        ("commons/apostle/psalm-antiphon", "Generic common antiphon"),
        ("ordinary/lauds/psalm-antiphon", "Generic ordinary antiphon"),
        ("ordinary/lauds/psalm-antiphon-2-sunday", "Weekday-specific indexed antiphon"),
        ("ordinary/vespers/psalm-antiphon", "Generic vespers antiphon"),
        ("seasonal/pentecost/psalm-antiphon", "Generic seasonal antiphon"),
        ("seasonal/pentecost/psalm-antiphon-lauds", "Hour-qualified seasonal antiphon"),
    ]);
    let d = celebrating(zero_date(), Season::Pentecost, feast("st-mark", None));
    assert_eq!(resolve(&d, "lauds", "psalm-antiphon-2", &t), ("Generic proper antiphon".into(), "proper/st-mark/psalm-antiphon".into()));
    let d = celebrating(zero_date(), Season::Lent, feast("st-john", Some(Category::Apostle)));
    assert_eq!(resolve(&d, "lauds", "psalm-antiphon-2", &t), ("Generic common antiphon".into(), "commons/apostle/psalm-antiphon".into()));
    let d = day(zero_date(), Season::Pentecost);
    assert_eq!(
        resolve(&d, "vespers", "psalm-antiphon-2", &t),
        ("Generic seasonal antiphon".into(), "seasonal/pentecost/psalm-antiphon".into())
    );
}

#[test]
fn minor_hour_collects_and_prime() {
    let t = texts(&[
        ("ordinary/lauds/collect", "Lauds collect"),
        ("ordinary/terce/collect", "Terce collect"),
        ("ordinary/sext/collect", "Sext collect"),
        ("ordinary/none/collect", "None collect"),
        ("ordinary/prime/collect", "Prime collect"),
    ]);
    let d = day(zero_date(), Season::Lent);
    for hour in ["terce", "sext", "none"] {
        assert_eq!(resolve_proper_collect_text(&d, hour, &t), ("Lauds collect".into(), "ordinary/lauds/collect".into()));
    }
    assert_eq!(resolve_proper_collect_text(&d, "prime", &t), ("Prime collect".into(), "ordinary/prime/collect".into()));
}

#[test]
fn feria_uses_sunday_collect() {
    let t = texts(&[
        ("proper/pentecost-sunday-12/collect", "Sunday collect"),
        ("ordinary/lauds/collect", "Daily Lauds collect"),
        ("ordinary/vespers/collect", "Daily Vespers collect"),
        ("ordinary/prime/collect", "Prime collect"),
        ("proper/st-bartholomew/collect", "Feast collect"),
        ("commons/apostle/collect", "Apostle common collect"),
    ]);
    let mut feria = day(date(2026, 8, 25), Season::Pentecost);
    feria.temporal_week_id = Some("pentecost-sunday-12".into());
    let sunday = ("Sunday collect".to_string(), "proper/pentecost-sunday-12/collect".to_string());
    for hour in ["lauds", "vespers", "terce"] {
        assert_eq!(resolve_proper_collect_text(&feria, hour, &t), sunday, "{hour}");
    }
    assert_eq!(resolve_proper_collect_text(&feria, "prime", &t), ("Prime collect".into(), "ordinary/prime/collect".into()));
    let mut d = celebrating(date(2026, 8, 24), Season::Pentecost, feast("st-bartholomew", Some(Category::Apostle)));
    d.temporal_week_id = Some("pentecost-sunday-12".into());
    assert_eq!(resolve_proper_collect_text(&d, "lauds", &t), ("Feast collect".into(), "proper/st-bartholomew/collect".into()));
    let mut d = celebrating(date(2026, 8, 26), Season::Pentecost, feast("unknown-apostle", Some(Category::Apostle)));
    d.temporal_week_id = Some("pentecost-sunday-12".into());
    assert_eq!(resolve_proper_collect_text(&d, "lauds", &t), ("Apostle common collect".into(), "commons/apostle/collect".into()));
    let d = day(date(2026, 1, 8), Season::Epiphany);
    assert_eq!(resolve_proper_collect_text(&d, "lauds", &t), ("Daily Lauds collect".into(), "ordinary/lauds/collect".into()));
}

#[test]
fn easter_overrides() {
    let t = texts(&[
        ("seasonal/easter/short-responsory-lauds", "Paschal lauds short responsory"),
        ("seasonal/easter/short-responsory-vespers", "Paschal vespers short responsory"),
        ("seasonal/easter/versicle-lauds", "Paschal lauds versicle"),
        ("seasonal/easter/versicle-vespers", "Paschal vespers versicle"),
        ("ordinary/lauds/short-responsory", "Ordinary lauds short responsory"),
        ("ordinary/vespers/short-responsory", "Ordinary vespers short responsory"),
        ("ordinary/lauds/versicle", "Ordinary lauds versicle"),
        ("ordinary/vespers/versicle", "Ordinary vespers versicle"),
    ]);
    let d = day(zero_date(), Season::Easter);
    for (hour, r, want) in [
        ("lauds", "short-responsory", "seasonal/easter/short-responsory-lauds"),
        ("vespers", "short-responsory", "seasonal/easter/short-responsory-vespers"),
        ("lauds", "versicle", "seasonal/easter/versicle-lauds"),
        ("vespers", "versicle", "seasonal/easter/versicle-vespers"),
    ] {
        assert_eq!(resolve(&d, hour, r, &t).1, want);
    }
    let t = texts(&[
        ("proper/example/short-responsory-lauds", "Regular feast response"),
        ("proper/example-paschal/short-responsory-lauds", "Paschal feast response"),
        ("seasonal/easter/short-responsory-lauds", "Generic Easter response"),
    ]);
    let mut d = celebrating(zero_date(), Season::Easter, feast("example", None));
    assert_eq!(
        resolve(&d, "lauds", "short-responsory", &t),
        ("Paschal feast response".into(), "proper/example-paschal/short-responsory-lauds".into())
    );
    d.season = Season::Lent;
    assert_eq!(
        resolve(&d, "lauds", "short-responsory", &t),
        ("Regular feast response".into(), "proper/example/short-responsory-lauds".into())
    );
}
