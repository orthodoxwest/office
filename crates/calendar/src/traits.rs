//! Predicates over feasts shared by occurrence (here) and concurrence (the office crate). Many
//! classify generated feasts by their ID shape.

use crate::model::{Category, Feast, Rank, Season};

/// The Greater Sundays of the I Class.
pub const SUNDAYS_FIRST_CLASS: [&str; 10] = [
    "advent-sunday-1",
    "lent-sunday-1",
    "lent-sunday-2",
    "lent-sunday-3",
    "laetare-sunday",
    "passion-sunday",
    "palm-sunday",
    "easter-sunday",
    "low-sunday",
    "pentecost",
];

pub fn is_sunday_first_class(f: &Feast) -> bool {
    SUNDAYS_FIRST_CLASS.contains(&f.id.as_str())
}

/// The terminal octave day ("…-octave-day").
pub fn is_octave_day(f: &Feast) -> bool {
    f.id.ends_with("-octave-day")
}

/// A non-terminal day within an octave ("…-octave-day-N").
pub fn is_day_within_octave(f: &Feast) -> bool {
    match f.id.rfind("-octave-day-") {
        None => false,
        Some(idx) => {
            let suffix = &f.id[idx + "-octave-day-".len()..];
            !suffix.is_empty() && suffix.bytes().all(|c| c.is_ascii_digit())
        }
    }
}

pub fn is_privileged_octave_commemoration(f: &Feast) -> bool {
    f.is_privileged_octave_day
}

/// The parent feast ID of a generated octave day, if `f` is one.
pub fn octave_parent_id(f: &Feast) -> Option<&str> {
    if let Some(parent) = f.octave_of.as_deref() {
        return Some(parent);
    }
    if is_octave_day(f) {
        return f.id.strip_suffix("-octave-day");
    }
    if is_day_within_octave(f) {
        return f.id.rfind("-octave-day-").map(|idx| &f.id[..idx]);
    }
    None
}

pub fn same_octave_days(a: &Feast, b: &Feast) -> bool {
    matches!(octave_parent_id(a), Some(p) if Some(p) == octave_parent_id(b))
}

pub fn is_double_or_above(f: &Feast) -> bool {
    f.rank.weight() >= Rank::Double.weight()
}

pub fn is_saturday_bvm(f: &Feast) -> bool {
    f.id == "saturday-office-bvm"
}

pub fn is_sunday(f: &Feast) -> bool {
    f.is_category(Category::Sunday)
}

/// A Sunday office anticipated on Saturday (General Rubrics IV.4-5).
pub fn is_anticipated_sunday(f: &Feast) -> bool {
    f.id.ends_with("-anticipated") && f.is_category(Category::Sunday)
}

pub fn is_ember_day(f: &Feast) -> bool {
    f.id.contains("ember-")
}

pub fn is_rogation_day(f: &Feast) -> bool {
    f.id == "rogation-monday"
}

pub fn is_vigil(f: &Feast) -> bool {
    f.is_vigil
}

/// The privileged Vigils of the General Rubrics' rank table: the Nativity and
/// Pentecost (I class) and the Epiphany (II class). All others are common.
pub fn is_privileged_vigil(f: &Feast) -> bool {
    f.is_vigil && matches!(f.id.as_str(), "vigil-nativity" | "vigil-pentecost" | "vigil-epiphany")
}

/// The perpetual commemoration of St Peter or St Paul kept on the other
/// apostle's feasts.
pub fn is_apostolic_companion_commemoration(f: &Feast) -> bool {
    f.companion_of.is_some()
}

pub fn is_privileged_feria(f: &Feast) -> bool {
    f.is_category(Category::Feria) && f.rank == Rank::PrivilegedFeria
}

/// Seasons whose occurring ferias are privileged and commemorated at Lauds
/// when a feast takes the office.
pub fn is_penitential_feria_season(season: Season) -> bool {
    match season {
        Season::Advent | Season::Septuagesima | Season::Lent | Season::Passiontide => true,
        Season::Christmas | Season::Epiphany | Season::Easter | Season::Pentecost => false,
    }
}
