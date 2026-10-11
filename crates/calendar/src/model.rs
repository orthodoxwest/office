//! Calendar types: ranks, colors, categories, seasons, feasts, and the
//! resolved calendar day every product shares.

use std::collections::BTreeMap;
use std::fmt;
use std::sync::Arc;

use crate::date::Date;

macro_rules! kebab_enum {
    ($(#[$meta:meta])* $name:ident, $what:literal { $($variant:ident => $text:literal),+ $(,)? }) => {
        $(#[$meta])*
        #[derive(Clone, Copy, Debug, PartialEq, Eq, Hash, PartialOrd, Ord)]
        pub enum $name { $($variant),+ }

        impl $name {
            pub const ALL: &'static [$name] = &[$($name::$variant),+];

            /// The kebab-case spelling used in the data files and the dump.
            pub fn as_str(self) -> &'static str {
                match self { $($name::$variant => $text),+ }
            }

            pub fn parse(s: &str) -> Result<$name, String> {
                match s {
                    $($text => Ok($name::$variant),)+
                    _ => Err(format!(concat!("invalid ", $what, ": {}"), data_format::quote(s))),
                }
            }
        }

        impl fmt::Display for $name {
            fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
                f.write_str(self.as_str())
            }
        }
    };
}

kebab_enum!(
    /// The pre-1962 ranking system for liturgical observances.
    Rank, "rank" {
        Double1stClass => "double-1st-class",
        Double2ndClass => "double-2nd-class",
        GreaterDouble => "greater-double",
        Double => "double",
        SemiDouble => "semi-double",
        PrivilegedFeria => "privileged-feria",
        Simple => "simple",
        Commemoration => "commemoration",
    }
);

impl Rank {
    /// Numeric weight for rank comparison (higher = more important).
    pub fn weight(self) -> i32 {
        match self {
            Rank::Double1stClass => 7,
            Rank::Double2ndClass => 6,
            Rank::GreaterDouble => 5,
            Rank::Double => 4,
            Rank::SemiDouble | Rank::PrivilegedFeria => 3,
            Rank::Simple => 2,
            Rank::Commemoration => 1,
        }
    }

    /// Any of the Double grades (General Rubrics I.4, XXIV.8).
    pub fn is_double(self) -> bool {
        match self {
            Rank::Double1stClass | Rank::Double2ndClass | Rank::GreaterDouble | Rank::Double => true,
            Rank::SemiDouble | Rank::PrivilegedFeria | Rank::Simple | Rank::Commemoration => false,
        }
    }

    pub fn abbrev(self) -> &'static str {
        match self {
            Rank::Double1stClass => "1cl",
            Rank::Double2ndClass => "2cl",
            Rank::GreaterDouble => "gd",
            Rank::Double => "d",
            Rank::SemiDouble => "sd",
            Rank::PrivilegedFeria => "f2",
            Rank::Simple => "s",
            Rank::Commemoration => "com",
        }
    }

    pub fn display_name(self) -> &'static str {
        match self {
            Rank::Double1stClass => "Double of the 1st Class",
            Rank::Double2ndClass => "Double of the 2nd Class",
            Rank::GreaterDouble => "Greater Double",
            Rank::Double => "Double",
            Rank::SemiDouble => "Semi-double",
            Rank::PrivilegedFeria => "Privileged Feria",
            Rank::Simple => "Simple",
            Rank::Commemoration => "Commemoration",
        }
    }
}

kebab_enum!(
    /// A liturgical color.
    Color, "color" {
        White => "white",
        Red => "red",
        Green => "green",
        Violet => "violet",
        Black => "black",
        Rose => "rose",
    }
);

impl Color {
    pub fn abbrev(self) -> &'static str {
        match self {
            Color::White => "w",
            Color::Red => "r",
            Color::Green => "g",
            Color::Violet => "v",
            Color::Black => "b",
            Color::Rose => "p",
        }
    }
}

kebab_enum!(
    /// The category of a feast, for common texts and precedence.
    Category, "feast category" {
        Lord => "lord",
        BlessedVirgin => "blessed-virgin",
        Angel => "angel",
        Apostle => "apostle",
        Evangelist => "evangelist",
        Martyr => "martyr",
        Martyrs => "martyrs",
        BishopMartyr => "bishop-martyr",
        VirginMartyr => "virgin-martyr",
        ConfessorBishop => "confessor-bishop",
        ConfessorDoctor => "confessor-doctor",
        Confessor => "confessor",
        Virgin => "virgin",
        HolyWoman => "holy-woman",
        Dedication => "dedication",
        Feria => "feria",
        Sunday => "sunday",
    }
);

kebab_enum!(
    /// A liturgical season.
    Season, "season" {
        Advent => "advent",
        Christmas => "christmas",
        Epiphany => "epiphany",
        Septuagesima => "septuagesima",
        Lent => "lent",
        Passiontide => "passiontide",
        Easter => "easter",
        Pentecost => "pentecost",
    }
);

impl Season {
    /// The season's default liturgical color.
    pub fn color(self) -> Color {
        match self {
            Season::Advent | Season::Septuagesima | Season::Lent | Season::Passiontide => Color::Violet,
            Season::Christmas | Season::Easter => Color::White,
            Season::Epiphany | Season::Pentecost => Color::Green,
        }
    }
}

/// General Rubrics VII.1-3 octave groups. The privileged classes are
/// Easter/Pentecost, Epiphany/Corpus Christi, and Nativity/Ascension/Sacred
/// Heart.
#[derive(Clone, Copy, Debug, PartialEq, Eq, Hash, Default)]
pub enum OctaveClass {
    /// The data files spell this as an absent or empty `OctaveClass`.
    #[default]
    Common,
    PrivilegedFirst,
    PrivilegedSecond,
    PrivilegedThird,
    Simple,
}

impl OctaveClass {
    /// The data-file spelling; `None` for the common class.
    pub fn as_str(self) -> Option<&'static str> {
        match self {
            OctaveClass::Common => None,
            OctaveClass::PrivilegedFirst => Some("privileged-first"),
            OctaveClass::PrivilegedSecond => Some("privileged-second"),
            OctaveClass::PrivilegedThird => Some("privileged-third"),
            OctaveClass::Simple => Some("simple"),
        }
    }

    pub fn parse(s: &str) -> Option<OctaveClass> {
        match s {
            "" => Some(OctaveClass::Common),
            "privileged-first" => Some(OctaveClass::PrivilegedFirst),
            "privileged-second" => Some(OctaveClass::PrivilegedSecond),
            "privileged-third" => Some(OctaveClass::PrivilegedThird),
            "simple" => Some(OctaveClass::Simple),
            _ => None,
        }
    }

    pub fn privileged(self) -> bool {
        match self {
            OctaveClass::PrivilegedFirst | OctaveClass::PrivilegedSecond | OctaveClass::PrivilegedThird => true,
            OctaveClass::Common | OctaveClass::Simple => false,
        }
    }
}

/// Named ferial exceptions in XIV.14. Ordinary Sundays, ferias, feasts, and
/// octaves derive their tier from their own traits.
#[derive(Clone, Copy, Debug, PartialEq, Eq, Hash, Default)]
pub enum CommemorationClass {
    #[default]
    Default,
    EpiphanyVigil,
    PostAscensionFeria,
}

impl CommemorationClass {
    pub fn as_str(self) -> Option<&'static str> {
        match self {
            CommemorationClass::Default => None,
            CommemorationClass::EpiphanyVigil => Some("epiphany-vigil"),
            CommemorationClass::PostAscensionFeria => Some("post-ascension-feria"),
        }
    }

    pub fn parse(s: &str) -> Option<CommemorationClass> {
        match s {
            "" => Some(CommemorationClass::Default),
            "epiphany-vigil" => Some(CommemorationClass::EpiphanyVigil),
            "post-ascension-feria" => Some(CommemorationClass::PostAscensionFeria),
            _ => None,
        }
    }
}

/// The synthetic feast ID of the occurring privileged feria commemorated at
/// Lauds on a penitential weekday feast day.
pub const FERIA_COMMEMORATION_ID: &str = "penitential-feria";

/// A fixed month and day.
#[derive(Clone, Copy, Debug, PartialEq, Eq, Hash)]
pub struct MonthDay {
    pub month: u32,
    pub day: u32,
}

/// The qualifier the ordo prints inside the brackets after an observance kept only by monastics
/// and oblates.
pub const MONASTIC_QUALIFIER: &str = "Monastics & Oblates Only";

/// An observance the ordo brackets "(Monastics & Oblates Only)": printed under the day's office
/// as a notation, never part of the parish calendar. The app does not compose its office; its rank
/// and office note are the ordo's words, carried for the reader who prays it from the Diurnal.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct MonasticObservance {
    pub id: String,
    pub name: String,
    pub rank: Rank,
    pub fixed: MonthDay,
    /// Where its office is found, e.g. "Proper Office, Monastic Diurnal pp. 560–564".
    pub office: Option<String>,
}

impl MonasticObservance {
    /// The bracketed line as the ordo prints it, without the brackets:
    /// "Solemnity of St Benedict (Monastics & Oblates Only)".
    pub fn heading(&self) -> String {
        format!("{} ({MONASTIC_QUALIFIER})", self.name)
    }
}

/// A liturgical feast or observance and its rubrical attributes.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Feast {
    pub id: String,
    pub name: String,
    pub rank: Rank,
    pub color: Color,
    pub category: Option<Category>,
    /// The given name used for "N." substitution in common texts.
    pub proper_name: Option<String>,
    /// Redirects proper lookup to another feast ID.
    pub proper_id: Option<String>,
    /// For moveable feasts: an Easter offset or a named rule.
    pub date_rule: Option<String>,
    /// For sanctoral feasts.
    pub fixed: Option<MonthDay>,
    pub has_octave: bool,
    pub has_vigil: bool,
    pub octave_class: OctaveClass,
    pub commemoration_class: CommemorationClass,
    pub is_privileged_octave_day: bool,
    pub is_vigil: bool,
    pub vigil_of: Option<String>,
    /// The octave this day continues when its ID does not say so (Easter
    /// Monday and Tuesday).
    pub octave_of: Option<String>,
    /// The ordo's names for this feast's generated octave days: a pattern for
    /// days II–VII ("{n}" the day's numeral, "{weekday}" its weekday), and
    /// single days 2–8 by number.
    pub octave_days: Option<String>,
    pub octave_day_names: BTreeMap<i32, String>,
    pub companion_of: Option<String>,
    pub primary_of_our_lord: bool,
    /// A secondary feast in the Diurnal's Table of the Rank of Feasts (General
    /// Rubrics X.1(c)); only the secondary feasts of Our Lord are marked so far.
    pub secondary: bool,
    pub only_with: Option<String>,
    pub skip_roman_leap_shift: bool,
    /// Free text in the data files ("base" unless given).
    pub source: Option<String>,
    /// Documentation only; never reaches output.
    pub notes: Option<String>,
}

impl Feast {
    /// A feast with every optional field unset and the "base" source cleared;
    /// synthesized observances fill in what they need.
    pub fn synthetic(id: impl Into<String>, name: impl Into<String>, rank: Rank, color: Color, category: Category) -> Feast {
        Feast {
            id: id.into(),
            name: name.into(),
            rank,
            color,
            category: Some(category),
            proper_name: None,
            proper_id: None,
            date_rule: None,
            fixed: None,
            has_octave: false,
            has_vigil: false,
            octave_class: OctaveClass::Common,
            commemoration_class: CommemorationClass::Default,
            is_privileged_octave_day: false,
            is_vigil: false,
            vigil_of: None,
            octave_of: None,
            octave_days: None,
            octave_day_names: BTreeMap::new(),
            companion_of: None,
            primary_of_our_lord: false,
            secondary: false,
            only_with: None,
            skip_roman_leap_shift: false,
            source: None,
            notes: None,
        }
    }

    pub fn is_fixed(&self) -> bool {
        self.fixed.is_some()
    }

    /// Whether the feast has a computed date rule. A date classification, not
    /// a statement about the temporal cycle.
    pub fn is_moveable(&self) -> bool {
        self.date_rule.is_some()
    }

    pub fn is_category(&self, c: Category) -> bool {
        self.category == Some(c)
    }

    /// The name after a "Com." prefix supplied by the composer.
    pub fn commemoration_name(&self) -> &str {
        self.name.strip_prefix("Commemoration of ").unwrap_or(&self.name)
    }
}

pub type FeastRef = Arc<Feast>;

/// Maps an empty string to `None`.
pub fn non_empty(s: impl Into<String>) -> Option<String> {
    let s = s.into();
    (!s.is_empty()).then_some(s)
}

/// One machine-readable choice. `rule` is a stable identifier, `outcome` the
/// selected branch, `detail` optional reviewer-facing context.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Decision {
    pub rule: String,
    pub outcome: String,
    pub detail: Option<String>,
}

impl Decision {
    pub fn new(rule: impl Into<String>, outcome: impl Into<String>, detail: impl Into<String>) -> Decision {
        Decision { rule: rule.into(), outcome: outcome.into(), detail: non_empty(detail) }
    }
}

/// Fasting and abstinence obligations for a day.
#[derive(Clone, Copy, Debug, Default, PartialEq, Eq)]
pub struct Penitential {
    pub fast: bool,
    pub abstinence: bool,
}

impl Penitential {
    pub fn is_empty(self) -> bool {
        !self.fast && !self.abstinence
    }

    /// The compact ordo marker used in text output.
    pub fn marker(self) -> String {
        let mut m = String::new();
        if self.abstinence {
            m.push('L');
        }
        if self.fast {
            m.push('§');
        }
        m
    }

    pub fn labels(self) -> Vec<&'static str> {
        let mut labels = Vec::new();
        if self.fast {
            labels.push("Fasting");
        }
        if self.abstinence {
            labels.push("Abstinence");
        }
        labels
    }
}

/// The observance of the day shared by every product: the Office, the Missal,
/// the vicariate calendar feed, and the ordo. Office-only resolution (Vespers
/// concurrence, the Marian antiphon) lives in the office crate.
#[derive(Clone, Debug)]
pub struct CalendarDay {
    pub date: Date,
    pub season: Season,
    pub tempora: Option<String>,
    pub celebration: Option<FeastRef>,
    pub commemorations: Vec<FeastRef>,
    pub color: Color,
    // The calendar builder leaves notes empty.
    pub notes: Option<String>,
    pub resolution_rule: String,
    pub occurrence_decisions: Vec<Decision>,
    /// The occurring privileged feria commemorated at Lauds (and eligible at
    /// II Vespers) when a feast takes the office on a penitential weekday.
    pub feria_commemoration: Option<FeastRef>,
    /// The temporal Sunday office governing this day's week.
    pub temporal_week_id: Option<String>,
    /// The parent feast ID when this day falls within an octave (days 1-8).
    pub within_octave_of: Option<String>,
    pub penitential: Penitential,
    /// Observances the ordo brackets for monastics and oblates on this date; notation only.
    pub monastic: Vec<MonasticObservance>,
}

impl CalendarDay {
    /// The office owner's category is feria, including unnamed ferias.
    pub fn is_ferial(&self) -> bool {
        match &self.celebration {
            None => true,
            Some(c) => c.is_category(Category::Feria),
        }
    }
}
