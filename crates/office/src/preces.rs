//! Preces, the Suffrage of All Saints, and the Commemoration of the Cross.

use calendar::{Category, Feast, MoveableDates, Rank, Season, Weekday};

use crate::day::Day;
use crate::lauds_psalmody::uses_weekday_lauds_psalmody;
use crate::proper::feast_proper_ids;
use crate::texts::OfficeTexts;

pub const SATURDAY_OFFICE_BVM_ID: &str = "saturday-office-bvm";

/// Ferial-category celebrations that nevertheless have a Double office.
fn is_double_feria_office(id: &str) -> bool {
    matches!(id, "all-souls" | "vigil-nativity" | "vigil-pentecost")
}

/// Whether Lauds takes the festal psalms.
pub fn uses_festal_lauds_psalmody(day: &Day, t: &OfficeTexts) -> bool {
    let Some(c) = day.celebration.as_deref() else { return false };
    if feast_proper_ids(c).iter().any(|id| t.get(&format!("proper/{id}/lauds-psalmody")) == "festal") {
        return true;
    }
    !uses_weekday_lauds_psalmody(day, t) && !c.is_category(Category::Feria) && !c.is_category(Category::Sunday)
}

pub const PRECES_SAID: &str = "said";
pub const PRECES_SUPPRESSED_DOUBLE_OFFICE: &str = "suppressed:double-office";
pub const PRECES_SUPPRESSED_WITHIN_OCTAVE: &str = "suppressed:within-octave";
pub const PRECES_SUPPRESSED_OCTAVE_DAY: &str = "suppressed:octave-day";
pub const PRECES_SUPPRESSED_DOUBLE_COMMEMORATION: &str = "suppressed:double-commemoration";
pub const PRECES_SUPPRESSED_OCTAVE_COMMEMORATION: &str = "suppressed:octave-commemoration";
pub const PRECES_SUPPRESSED_FRIDAY_AFTER_ASCENSION: &str = "suppressed:friday-after-ascension-octave";
pub const PRECES_SUPPRESSED_VIGIL_EPIPHANY: &str = "suppressed:vigil-epiphany";
pub const PRECES_SUPPRESSED_EASTER_SUNDAY: &str = "suppressed:easter-sunday";

/// Whether the preces are said, and the structural reason (§XXXVII; #15).
pub fn preces_disposition(day: Option<&Day>, moveable: Option<&MoveableDates>) -> (bool, &'static str) {
    let Some(day) = day else { return (true, PRECES_SAID) };
    if celebration_has_double_office(day.celebration.as_deref()) {
        return (false, PRECES_SUPPRESSED_DOUBLE_OFFICE);
    }
    if day.within_octave_of.is_some() {
        return (false, PRECES_SUPPRESSED_WITHIN_OCTAVE);
    }
    if day.celebration.as_deref().is_some_and(|c| c.id.contains("octave-day")) {
        return (false, PRECES_SUPPRESSED_OCTAVE_DAY);
    }
    for comm in &day.commemorations {
        if comm.rank.weight() >= Rank::Double.weight() {
            return (false, PRECES_SUPPRESSED_DOUBLE_COMMEMORATION);
        }
        if comm.id.contains("-octave-") {
            return (false, PRECES_SUPPRESSED_OCTAVE_COMMEMORATION);
        }
    }
    if let Some(m) = moveable
        && day.date == m.easter.add_days(47)
    {
        return (false, PRECES_SUPPRESSED_FRIDAY_AFTER_ASCENSION);
    }
    if day.date.month() == 1 && day.date.day() == 5 {
        return (false, PRECES_SUPPRESSED_VIGIL_EPIPHANY);
    }
    if day.season == Season::Easter && day.date.weekday() == Weekday::Sunday {
        return (false, PRECES_SUPPRESSED_EASTER_SUNDAY);
    }
    (true, PRECES_SAID)
}

pub fn should_say_preces(day: Option<&Day>, moveable: Option<&MoveableDates>) -> bool {
    preces_disposition(day, moveable).0
}

/// Office form, as opposed to occurrence precedence: penitential Sundays and
/// privileged ferias keep Sunday or ferial offices.
fn celebration_has_double_office(feast: Option<&Feast>) -> bool {
    let Some(f) = feast else { return false };
    if f.rank.weight() < Rank::Double.weight() {
        return false;
    }
    match f.category {
        Some(Category::Sunday) => false,
        Some(Category::Feria) => is_double_feria_office(&f.id),
        _ => true,
    }
}

fn office_allows_customary_suffrage(day: &Day) -> bool {
    let Some(c) = day.celebration.as_deref() else { return true };
    c.id == SATURDAY_OFFICE_BVM_ID || c.is_category(Category::Sunday) || c.is_category(Category::Feria)
}

fn within_suffrage_season(day: &Day) -> bool {
    match day.season {
        Season::Epiphany => !(day.date.month() == 1 && (7..=13).contains(&day.date.day())),
        Season::Septuagesima | Season::Lent | Season::Pentecost => true,
        Season::Advent | Season::Christmas | Season::Passiontide | Season::Easter => false,
    }
}

fn commemoration_suppresses_suffrage(comm: &Feast) -> bool {
    comm.rank.weight() >= Rank::Double.weight() || comm.id.contains("octave")
}

pub const SUFFRAGE_SAID: &str = "said";
pub const SUFFRAGE_SUPPRESSED_NON_CUSTOMARY: &str = "suppressed:non-customary-office";
pub const SUFFRAGE_SUPPRESSED_WITHIN_OCTAVE: &str = "suppressed:within-octave";
pub const SUFFRAGE_SUPPRESSED_OUT_OF_SEASON: &str = "suppressed:out-of-season";
pub const SUFFRAGE_SUPPRESSED_COMMEMORATION: &str = "suppressed:commemoration";
pub const SUFFRAGE_SUPPRESSED_ALL_SAINTS_VIGIL: &str = "suppressed:vigil-of-all-saints";

/// Whether the Suffrage of All Saints is said, and why.
pub fn suffrage_disposition(day: Option<&Day>) -> (bool, &'static str) {
    let Some(day) = day else { return (false, SUFFRAGE_SUPPRESSED_OUT_OF_SEASON) };
    if !office_allows_customary_suffrage(day) {
        return (false, SUFFRAGE_SUPPRESSED_NON_CUSTOMARY);
    }
    // Diurnal p. 636: "At Lauds the Suffrage of All Saints is not said." The
    // 2017–2019 ordos agree; the later "Suff." descends from the 2021 line for
    // the vigil anticipated to Saturday (#471).
    if day.celebration.as_deref().is_some_and(|c| c.id == "vigil-of-all-saints") {
        return (false, SUFFRAGE_SUPPRESSED_ALL_SAINTS_VIGIL);
    }
    if day.within_octave_of.is_some() {
        return (false, SUFFRAGE_SUPPRESSED_WITHIN_OCTAVE);
    }
    if !within_suffrage_season(day) {
        return (false, SUFFRAGE_SUPPRESSED_OUT_OF_SEASON);
    }
    if day.commemorations.iter().any(|c| commemoration_suppresses_suffrage(c)) {
        return (false, SUFFRAGE_SUPPRESSED_COMMEMORATION);
    }
    (true, SUFFRAGE_SAID)
}

pub fn should_say_suffrage(day: Option<&Day>) -> bool {
    suffrage_disposition(day).0
}

/// The Commemoration of the Cross, from Monday after Low Sunday through the
/// Vigil of the Ascension, in offices below Double (Diurnal p. 146). Like the
/// suffrage it gives way to a commemorated Double or octave; every ordo pairs
/// those commemorations with "No Comm. HC" (#356).
pub fn should_say_cross_commemoration(day: &Day, moveable: Option<&MoveableDates>) -> bool {
    let Some(m) = moveable else { return false };
    if !office_allows_customary_suffrage(day) || day.within_octave_of.is_some() || day.season != Season::Easter {
        return false;
    }
    if day.commemorations.iter().any(|c| commemoration_suppresses_suffrage(c)) {
        return false;
    }
    let since = day.date.days_since(m.easter);
    (8..=38).contains(&since)
}

/// The office or a commemoration is of the Blessed Virgin: the Suffrage then
/// omits its invocation of her.
pub fn uses_bvm_suffrage_form(day: &Day) -> bool {
    day.celebration.as_deref().is_some_and(|c| c.is_category(Category::BlessedVirgin))
        || day.commemorations.iter().any(|c| c.is_category(Category::BlessedVirgin))
}
