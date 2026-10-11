use calendar::{Category, Season};

use crate::proper::resolve_proper_text;
use crate::testutil::{celebrating, date, day, feast, texts};

fn resolve(d: &crate::Day, hour: &str, r: &str, t: &crate::texts::OfficeTexts) -> (String, String) {
    resolve_proper_text(d, hour, r, t)
}

#[test]
fn shared_fallback_and_not_found() {
    let d = day(date(2026, 3, 16), Season::Lent);
    assert_eq!(resolve(&d, "lauds", "collect", &texts(&[("ordinary/shared/collect", "Shared collect")])).0, "Shared collect");
    assert_eq!(resolve(&d, "lauds", "missing-ref", &texts(&[])), ("[Proper text not found: missing-ref]".into(), "missing-ref".into()));
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

/// #640: in Paschaltide a proper's `@use` into a Common reads the Common's
/// paschal form, unless the proper has its own paschal or Easter text; a text
/// the paschal Common lacks (a collect) and every hour outside Paschaltide
/// keep the redirect's own target.
#[test]
fn common_redirects_take_the_paschal_form_in_paschaltide() {
    let file = |rel_path: &str, content: &str| corpus::TextFile { rel_path: rel_path.into(), content: content.into() };
    let files = [
        file(
            "commons/confessor.txt",
            "[magnificat-antiphon-doctor]\nO Teacher.\n[collect]\nCollect.\n[benedictus-antiphon]\nWell done.\n[versicle]\nV. Plain.",
        ),
        file(
            "commons/confessor-paschal.txt",
            "[magnificat-antiphon-doctor]\nO Teacher, alleluia.\n[benedictus-antiphon]\nWell done, alleluia.\n[versicle]\nV. Paschal.",
        ),
        file(
            "proper/test-doctor.txt",
            "[magnificat-antiphon]\n@use commons/confessor/magnificat-antiphon-doctor\n[collect]\n@use commons/confessor/collect\n\
             [benedictus-antiphon]\n@use commons/confessor/benedictus-antiphon\n[benedictus-antiphon-easter]\nOwn Easter.\n\
             [versicle]\n@use commons/confessor/versicle",
        ),
        file("proper/test-doctor-paschal.txt", "[versicle]\nV. Own paschal."),
    ];
    let t = crate::texts::OfficeTexts::from_corpus(corpus::Corpus::load(&files, None, None).unwrap());
    let doctor = || feast("test-doctor", Some(Category::ConfessorDoctor));
    // Eastertide, and the Octave of Pentecost until None of Ember Saturday
    // (Pentecost 2026: 31 May).
    for (when, season, hour) in [
        (date(2026, 4, 23), Season::Easter, "vespers"),
        (date(2026, 6, 2), Season::Pentecost, "vespers"),
        (date(2026, 6, 6), Season::Pentecost, "none"),
    ] {
        let d = celebrating(when, season, doctor());
        let at = |r: &str| resolve(&d, hour, r, &t);
        let paschal = ("O Teacher, alleluia.".to_string(), "commons/confessor-paschal/magnificat-antiphon-doctor".to_string());
        assert_eq!(at("magnificat-antiphon"), paschal, "{when} {hour}");
        assert_eq!(at("collect"), ("Collect.".into(), "proper/test-doctor/collect".into()), "{when} {hour}");
        assert_eq!(at("versicle").0, "V. Own paschal.", "{when} {hour}");
        if season == Season::Easter {
            assert_eq!(at("benedictus-antiphon").0, "Own Easter.", "{when} {hour}");
        }
    }
    for (when, hour) in [(date(2026, 6, 6), "vespers"), (date(2026, 8, 1), "lauds")] {
        let d = celebrating(when, Season::Pentecost, doctor());
        let plain = ("O Teacher.".to_string(), "proper/test-doctor/magnificat-antiphon".to_string());
        assert_eq!(resolve(&d, hour, "magnificat-antiphon", &t), plain, "{when} {hour}");
        assert_eq!(resolve(&d, hour, "versicle", &t).0, "V. Plain.", "{when} {hour}");
    }
}
