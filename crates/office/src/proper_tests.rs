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
