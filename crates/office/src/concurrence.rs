//! Vespers concurrence: which office owns each evening, and the evening's commemorations (General
//! Rubrics XIII–XIV).

use std::sync::Arc;

use calendar::builder::seasonal_feria_commemoration;
use calendar::commemoration::{
    OrderContext, apostle_kept_on_primary_feast, cap_commemorations, dedupe_commemorations, order_commemorations, ordered_commemorations,
};
use calendar::computus::MoveableDates;
use calendar::model::FERIA_COMMEMORATION_ID;
use calendar::occurrence::compare_feast_precedence;
use calendar::traits::{
    is_anticipated_sunday, is_apostolic_companion_commemoration, is_day_within_octave, is_double_or_above, is_ember_day, is_octave_day,
    is_penitential_feria_season, is_privileged_octave_commemoration, is_rogation_day, is_saturday_bvm, is_sunday, is_sunday_first_class,
    is_vigil, octave_parent_id, same_octave_days,
};
use calendar::{CalendarDay, Category, Color, Decision, Feast, FeastRef, Rank, Season, Weekday};

/// Which office owns Vespers on an evening. A Vespers split at the Chapter is
/// not a separate owner: the incoming office owns the hour and only the
/// psalmody stays behind (see [`VespersDesignation::psalmody_from_preceding`]).
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum VespersOwner {
    /// Neither adjacent celebration has rights to this evening.
    NotApplicable,
    IIOfPreceding,
    IOfFollowing,
}

impl VespersOwner {
    pub fn as_str(self) -> &'static str {
        match self {
            VespersOwner::NotApplicable => "not-applicable",
            VespersOwner::IIOfPreceding => "ii-of-preceding",
            VespersOwner::IOfFollowing => "i-of-following",
        }
    }
}

/// The office that owns an evening's Vespers and the evening's commemorations.
#[derive(Clone, Debug)]
pub struct VespersDesignation {
    pub owner: VespersOwner,
    pub feast: Option<FeastRef>,
    pub color: Option<Color>,
    pub season: Option<Season>,
    /// The following day's octave context when it owns I Vespers.
    pub within_octave_of: Option<String>,
    pub rule: String,
    pub decisions: Vec<Decision>,
    pub commemorations: Vec<FeastRef>,
    /// The following celebration when its office is commemorated at II
    /// Vespers (its antiphon and versicle then come from I Vespers, XIV.14).
    pub following_office_commemoration_id: Option<String>,
    /// Commemorated offices that belong to the following day: each begins
    /// with its own I-Vespers antiphon and versicle (XIV.14; 2026 ordo
    /// 24 January, 23 March, 10 November).
    pub incoming_commemoration_ids: Vec<String>,
    /// The octave whose office is celebrated tomorrow when this is II Vespers.
    pub following_office_octave_of: Option<String>,
    /// A Vespers split at the Chapter: psalmody from the outgoing office.
    pub psalmody_from_preceding: bool,
    /// The evening before All Souls: Vespers of the Dead follows.
    pub appended_office_of_the_dead: bool,
    pub appended_feast: Option<FeastRef>,
}

impl VespersDesignation {
    /// An evening no office owns, with no commemorations: the zero value.
    pub fn unowned() -> VespersDesignation {
        VespersDesignation::new(VespersOwner::NotApplicable, "")
    }

    fn new(owner: VespersOwner, rule: &str) -> VespersDesignation {
        VespersDesignation {
            owner,
            feast: None,
            color: None,
            season: None,
            within_octave_of: None,
            rule: rule.to_string(),
            decisions: Vec::new(),
            commemorations: Vec::new(),
            following_office_commemoration_id: None,
            incoming_commemoration_ids: Vec::new(),
            following_office_octave_of: None,
            psalmody_from_preceding: false,
            appended_office_of_the_dead: false,
            appended_feast: None,
        }
    }
}

fn decision(rule: &str, outcome: &str, detail: &str) -> Decision {
    Decision::new(rule, outcome, detail)
}

fn same(a: &Option<FeastRef>, b: &FeastRef) -> bool {
    a.as_ref().is_some_and(|a| Arc::ptr_eq(a, b))
}

/// Whether the office begins at I Vespers. Ferias never do (XIII.18).
pub fn has_first_vespers(f: &Feast) -> bool {
    if f.id == "vigil-epiphany" {
        return true; // XIV.9: the vigil exception
    }
    if f.is_category(Category::Feria) {
        return false;
    }
    match f.id.as_str() {
        "holy-thursday" | "good-friday" | "holy-saturday" => return false,
        "saturday-office-bvm" => return true,
        _ => {}
    }
    if f.is_category(Category::Sunday) {
        return true; // IV.7
    }
    if f.rank == Rank::Simple {
        return true; // XIII.17; begins only at the Chapter
    }
    f.rank.weight() >= Rank::Double.weight()
}

/// General Rubrics III: an incoming Simple takes Vespers from the Chapter.
fn splits_vespers_at_chapter(fol: &Feast) -> bool {
    fol.rank == Rank::Simple
}

/// Whether the office has II Vespers. Simples have none (XIII.17); ferias
/// cannot concur (XIII.18).
pub fn has_second_vespers(f: &Feast) -> bool {
    if f.id == "friday-after-ascension-octave" {
        return true; // Diurnal p. 394
    }
    if f.is_category(Category::Sunday) {
        return true;
    }
    if matches!(f.rank, Rank::Simple | Rank::Commemoration) {
        return false;
    }
    if f.is_category(Category::Feria) {
        return false;
    }
    // A day within an octave is a semidouble office with the feast's II
    // Vespers (General Rubrics II.1, VII.6; #417).
    if is_day_within_octave(f) {
        return true;
    }
    f.rank.weight() >= Rank::Double.weight()
}

fn incoming_feria_excluded_at_vespers(f: &Feast) -> bool {
    f.is_category(Category::Feria) && f.id != "vigil-epiphany"
}

/// Saturday evening is always the Sunday's I Vespers, so a feria displaced on
/// Saturday has no Vespers to commemorate (2026 ordo 21 March, 12 December).
fn saturday_feria_without_vespers(day: &CalendarDay, f: &Feast) -> bool {
    f.is_category(Category::Feria) && day.date.weekday() == Weekday::Saturday
}

/// XIV.9: Ember days, Rogation Monday, and common vigils have Lauds only.
fn occurrence_commemorated_at_first_vespers(comm: &Feast) -> (bool, &'static str) {
    if is_ember_day(comm) || is_rogation_day(comm) || is_vigil(comm) {
        return (false, "commemoration:first-vespers-feria-or-vigil-lauds-only");
    }
    (true, "commemoration:first-vespers-occurrence-included")
}

/// Wednesday to Saturday within the Octaves of Easter and Pentecost: days
/// within a I Class privileged octave, semidouble (General Rubrics II.1,
/// VII.3), not Doubles I Class. Other feasts are commemorated on them
/// (Diurnal VII), Memorials at I Vespers and Lauds, Doubles at both Vespers
/// and Lauds (XIV.9; Diurnal VIII): every ordo 2017–2026.
fn semidouble_privileged_octave_day(w: &Feast) -> bool {
    is_day_within_octave(w)
        && w.rank == Rank::Double1stClass
        && !matches!(w.id.as_str(), "pentecost-octave-day-2" | "pentecost-octave-day-3")
}

/// Easter and Pentecost Monday and Tuesday keep an occurring Double at Lauds
/// (Fr Jason's #138 ruling), and the ordos give it both Vespers too: Boniface
/// 2017 and 2023, Ephrem 2019, Alban 2021, Basil 2022, St John before the Latin
/// Gate 2024. Columba's Monday Vespers 2025 is the one line without it (#380).
fn double_kept_at_second_vespers(w: &Feast) -> bool {
    semidouble_privileged_octave_day(w)
        || matches!(w.id.as_str(), "easter-monday" | "easter-tuesday" | "pentecost-octave-day-2" | "pentecost-octave-day-3")
}

/// II Vespers that commemorate the following day's Double: those days, and the
/// feast itself on the eve of its Monday (Boniface 2017 and 2023, Columba 2025
/// and St John before the Latin Gate 2024 at Whitsun and Easter Vespers; Leo
/// 2018, Michael 2024 and Barnabas 2025 at the Tuesday's; #380).
fn incoming_double_at_second_vespers(w: &Feast) -> bool {
    double_kept_at_second_vespers(w) || matches!(w.id.as_str(), "easter-sunday" | "pentecost")
}

/// Whether a Lauds commemoration remains at II Vespers of the winning office.
fn occurrence_commemorated_at_second_vespers(winner: Option<&Feast>, comm: &Feast) -> (bool, &'static str) {
    second_vespers_commemoration(winner, comm, false)
}

/// Whether one of the following day's commemorations survives at II Vespers
/// of a Double II Class or above: the occurrence rules, without the #558
/// allowance, which keeps only today's simplified Doubles.
fn incoming_commemorated_at_second_vespers(winner: Option<&Feast>, comm: &Feast) -> bool {
    second_vespers_commemoration(winner, comm, true).0
}

fn second_vespers_commemoration(winner: Option<&Feast>, comm: &Feast, incoming: bool) -> (bool, &'static str) {
    // XIV.9: Advent and Lenten ferias keep I and II Vespers as well as Lauds,
    // even at a Double I Class (the Annunciation in Lent, 2018–2026 ordos;
    // St Tikhon 2017 and 2023; St George 2021).
    if comm.id == FERIA_COMMEMORATION_ID || comm.id == "privileged-lenten-feria" {
        return (true, "commemoration:second-vespers-seasonal-feria");
    }
    // Needs ruling (#642): the ordos also commemorate the Ember days of Advent
    // and Lent, against XIV.9 (Ember days "only at Lauds"): II Vespers of St
    // Gregory on Ember Wednesday (2025 ordo 12 March, "For as Jonas") and of
    // St Thomas on Ember Friday (2018 ordo 21 Dec, "O Day-spring").
    if advent_or_lenten_ember_day(comm) {
        return (true, "commemoration:second-vespers-seasonal-ember-day");
    }
    if is_ember_day(comm) || is_rogation_day(comm) || is_vigil(comm) {
        return (false, "commemoration:second-vespers-feria-or-vigil-lauds-only");
    }
    // A Sunday anticipated on a Saturday Double of the I or II Class, "or
    // some other Feast of XII Lessons", is commemorated "at I Vespers ... and
    // at Lauds" (IV.4-5; XIV.9), and "nothing is said of the Sunday at II
    // Vespers" even when it is only commemorated (Notes on the Tables 12):
    // not at II Vespers of the Purification, 2 February 2075 and 2086 (#656).
    if !incoming && is_anticipated_sunday(comm) {
        return (false, "commemoration:second-vespers-anticipated-sunday-exclusion");
    }
    if matches!(comm.rank, Rank::Commemoration | Rank::Simple) {
        if is_apostolic_companion_commemoration(comm) && winner.is_none_or(|w| w.rank != Rank::Double1stClass) {
            return (true, "commemoration:second-vespers-apostolic-companion");
        }
        return (false, "commemoration:second-vespers-memorial-or-simple");
    }
    if is_privileged_octave_commemoration(comm) {
        return (true, "commemoration:second-vespers-privileged-octave");
    }
    if let Some(w) = winner {
        // Trinity and Corpus Christi keep an occurring Apostle through II
        // Vespers as well as Lauds (2023 and 2024 ordos, 2026 ordo 11 June;
        // #379).
        if apostle_kept_on_primary_feast(w, comm) {
            return (true, "commemoration:second-vespers-apostle-on-primary-feast");
        }
        if double_kept_at_second_vespers(w) && comm.rank.weight() >= Rank::Double.weight() {
            return (true, "commemoration:second-vespers-double-within-easter-pentecost-octave");
        }
        // XIV.5: a Double impeded by a Sunday or a privileged feria is
        // commemorated at I and II Vespers and Lauds, also when that Sunday or
        // feria is I Class (Patrick on Ash Wednesday 2021, Isidore and Leo on
        // Lent Sundays 2021, Cuthbert 2022).
        if w.rank == Rank::Double1stClass
            && (w.is_category(Category::Sunday) || w.is_category(Category::Feria))
            && comm.rank.weight() >= Rank::Double.weight()
            && !is_day_within_octave(comm)
        {
            return (true, "commemoration:second-vespers-double-on-first-class-sunday-or-feria");
        }
        // XIV.9 keeps a simplified Double at both Vespers "except on all I
        // Class Doubles", and Diurnal §X keeps it at Lauds only; but the
        // ordos commemorate today's Double at II Vespers of a Double I Class
        // other than the Primary Feasts of Our Lord and St Joseph's Solemnity
        // (#378): Our Lady of Sorrows at St Tikhon 2017 and 2023 and St
        // George 2021, Athanasius at St George 2022, and the Doubles within
        // Easter and Pentecost weeks 2017-2025 (#558). Not the following
        // day's Doubles (no St Paul at Ss Peter & Paul, 2019 and 2022) nor an
        // octave day.
        if !incoming
            && w.rank == Rank::Double1stClass
            && !w.primary_of_our_lord
            && w.id != "solemnity-st-joseph"
            && matches!(comm.rank, Rank::Double | Rank::GreaterDouble)
            && !is_day_within_octave(comm)
            && !is_octave_day(comm)
            && comm.octave_of.is_none()
        {
            return (true, "commemoration:second-vespers-simplified-double-on-first-class-feast");
        }
        if w.rank == Rank::Double1stClass && !comm.is_category(Category::Sunday) {
            return (false, "commemoration:second-vespers-first-class-exclusion");
        }
        if is_day_within_octave(comm) && w.rank.weight() >= Rank::Double2ndClass.weight() {
            return (false, "commemoration:second-vespers-day-within-octave-exclusion");
        }
    }
    (true, "commemoration:second-vespers-included")
}

/// Excludes next-day offices whose coverage does not reach the preceding
/// evening (XIV.7-9).
fn following_office_commemorated_at_second_vespers(winner: Option<&Feast>, following: &CalendarDay) -> (bool, &'static str) {
    let Some(feast) = following.celebration.as_deref() else {
        return (false, "commemoration:following-office-at-second-vespers-nil");
    };
    // XIII.16: two offices of the same mystery of the Lord admit no
    // commemoration of the less worthy: the Holy Name, given at the
    // Circumcision (Luke 2:21), at its II Vespers (2017-2024 ordos, "No
    // Comm."; #557).
    if winner.is_some_and(|w| w.id == "circumcision") && feast.id == "holy-name-jesus" {
        return (false, "commemoration:same-mystery-at-second-vespers");
    }
    if let Some(w) = winner
        && w.rank.weight() >= Rank::Double2ndClass.weight()
        && !w.is_category(Category::Sunday)
        && is_day_within_octave(feast)
        && !is_privileged_octave_commemoration(feast)
    {
        return (false, "commemoration:following-common-octave-at-second-vespers");
    }
    if incoming_feria_excluded_at_vespers(feast) {
        return (false, "commemoration:following-feria-not-at-second-vespers");
    }
    if matches!(feast.rank, Rank::Simple | Rank::Commemoration) {
        // Saturday BVM is Simple, but the AWRV ordo commemorates it at II
        // Vespers of a Friday Double or Greater Double (2026 ordo 2 Oct
        // Guardian Angels, 2025 ordo 24 Oct Raphael, 2022 ordo 25 Nov
        // Catherine; #542). Doubles of the 1st and 2nd class admit no
        // commemoration of it (2022 ordo 28 Oct Simon & Jude, 2019 ordo 26
        // July Anne, 2018 ordo 2 Feb Purification).
        if is_saturday_bvm(feast) && winner.is_some_and(|w| w.rank.weight() < Rank::Double2ndClass.weight()) {
            return (true, "commemoration:following-office-at-second-vespers-saturday-bvm");
        }
        return (false, "commemoration:following-office-at-second-vespers-simple-or-memorial");
    }
    (true, "commemoration:following-office-at-second-vespers-included")
}

fn octave_celebration_parent(day: &CalendarDay) -> Option<&str> {
    let c = day.celebration.as_deref()?;
    if c.has_octave {
        return Some(&c.id);
    }
    // The day's own octave, even when another octave overlaps it (St George
    // within Easter week: 2022 and 2025 ordos, "No Comm.").
    if let Some(parent) = octave_parent_id(c) {
        return Some(parent);
    }
    let parent = day.within_octave_of.as_deref()?;
    if c.id.starts_with(&format!("{parent}-octave-day")) {
        return Some(parent);
    }
    None
}

fn same_octave_office(preceding: &CalendarDay, following: &CalendarDay) -> bool {
    matches!(octave_celebration_parent(preceding), Some(p) if Some(p) == octave_celebration_parent(following))
}

/// "A Feria in Advent, in Septuagesimatide, or in Lent" (Diurnal §X), with
/// the Ember days of Advent and Lent (2021 ordo 24 March, 2022 18 March).
fn penitential_season_feria(f: &Feast) -> bool {
    f.id == FERIA_COMMEMORATION_ID || f.id == "privileged-lenten-feria" || advent_or_lenten_ember_day(f)
}

fn advent_or_lenten_ember_day(f: &Feast) -> bool {
    f.id.starts_with("lent-ember-") || f.id.starts_with("advent-ember-")
}

/// Ember Saturday's evening is the Sunday's I Vespers, which do not
/// commemorate it (2026 ordo 19 Dec).
fn ember_saturday(f: &Feast) -> bool {
    f.id.ends_with("-ember-saturday")
}

/// XIV.7-8 applied to the office displaced by I Vespers of the following.
fn outgoing_commemorated_at_first_vespers(winner: Option<&Feast>, loser: &Feast) -> (bool, &'static str) {
    // A Sunday office anticipated on Saturday ends at None: its evening is the
    // next Sunday's I Vespers, which do not commemorate it (2025 and 2026
    // ordos, 7 February; 2021 ordo, 20 November).
    if is_anticipated_sunday(loser) {
        return (false, "commemoration:first-vespers-anticipated-sunday-exclusion");
    }
    let first_class = winner.is_some_and(|w| w.rank == Rank::Double1stClass);
    if first_class && penitential_season_feria(loser) {
        // Such a feria stays at I Vespers of a Double I Class (Diurnal §X; the
        // Annunciation in 2017, 2021–2023, 2025 and 2026). A first-class
        // Sunday or feria is no Double: the Lenten Saturday before it is not
        // commemorated.
        if winner.is_some_and(|w| w.is_category(Category::Sunday) || w.is_category(Category::Feria)) {
            return (false, "commemoration:first-vespers-first-class-seasonal-feria-exclusion");
        }
        return (true, "commemoration:first-vespers-first-class-seasonal-feria");
    }
    if loser.id == FERIA_COMMEMORATION_ID {
        return (true, "commemoration:first-vespers-seasonal-feria");
    }
    if first_class && loser.is_category(Category::Feria) && loser.rank == Rank::PrivilegedFeria {
        return (false, "commemoration:first-vespers-first-class-seasonal-feria-exclusion");
    }
    if !has_second_vespers(loser) {
        if is_day_within_octave(loser) {
            if winner.is_some_and(|w| w.rank.weight() >= Rank::Double2ndClass.weight()) {
                return (false, "commemoration:first-vespers-day-within-octave-exclusion");
            }
            return (true, "commemoration:first-vespers-day-within-octave");
        }
        if loser.is_category(Category::Feria)
            && loser.rank == Rank::PrivilegedFeria
            && !is_ember_day(loser)
            && !is_rogation_day(loser)
            && !is_vigil(loser)
        {
            return (true, "commemoration:first-vespers-seasonal-feria");
        }
        // Needs ruling (#642): XIV.9 commemorates an Ember day "only at
        // Lauds", but the ordos commemorate an Advent or Lenten one at a
        // following feast's I Vespers, like the season's other ferias (XIV.8):
        // 2017 and 2019 ordos 20 Dec (St Thomas), 2022 ordo 18 March (St
        // Joseph) and 14 Dec (the Conception's octave day), 2025 ordo 17 Dec
        // (the Expectation). Ember Saturday is left to the Sunday.
        if advent_or_lenten_ember_day(loser) && !ember_saturday(loser) && winner.is_some() {
            return (true, "commemoration:first-vespers-seasonal-ember-day");
        }
        return (false, "commemoration:first-vespers-office-ended-at-none");
    }
    let Some(w) = winner else {
        return (true, "commemoration:first-vespers-concurrence");
    };
    if w.rank == Rank::Double1stClass {
        // Diurnal §X (pp. xxix–xxx): the preceding day is commemorated "only
        // if it were a Sunday (except at I Vespers of the Nativity and
        // Epiphany of Our Lord), or a Privileged Octave, or a Double I or II
        // Class, or a Feria in Advent, in Septuagesimatide, or in Lent". The
        // fuller English XIV.7 reads "but not of" there, against the Table of
        // Concurrence (p. xlv) and the ordos: IV after Pentecost before Ss
        // Peter and Paul (2026), Low Sunday before St Tikhon (2018, 2026),
        // Simon and Jude before Christ the King (2017, 2023), the Holy Name
        // before the Epiphany (2025); #396.
        if loser.is_category(Category::Sunday) {
            if w.id == "christmas" || w.id == "epiphany" {
                return (false, "commemoration:first-vespers-nativity-epiphany-sunday-exclusion");
            }
            return (true, "commemoration:first-vespers-first-class-sunday");
        }
        // A day within a common octave is not on that list (2026 ordo, the
        // St George octave before St Joseph's Solemnity; #378).
        if is_day_within_octave(loser) && !is_privileged_octave_commemoration(loser) {
            return (false, "commemoration:first-vespers-first-class-common-octave-exclusion");
        }
        // The Easter and Pentecost octaves end at None of Saturday (2026 ordo
        // 18 April and 6 June), and the Triduum is never commemorated.
        if loser.is_category(Category::Feria)
            || matches!(loser.id.as_str(), "holy-thursday" | "good-friday" | "holy-saturday")
            || (is_day_within_octave(loser) && loser.rank.weight() >= Rank::Double2ndClass.weight())
        {
            return (false, "commemoration:first-vespers-first-class-exclusion");
        }
        if loser.rank.weight() >= Rank::Double2ndClass.weight() {
            return (true, "commemoration:first-vespers-first-class-double");
        }
        // A Greater or Lesser Double is not on the list either, but the ordos
        // are split: St Gabriel before the Annunciation every year, against
        // Doubles dropped before the Ascension, Pentecost and Ss Peter and
        // Paul. It stays pending a ruling.
    }
    if w.rank == Rank::Double2ndClass {
        // XIV.8: "on the Feast of the Circumcision, of a Sunday or any Greater
        // or Lesser Double" (St Sylvester, every ordo 2017-2026).
        if w.id == "circumcision" && (loser.is_category(Category::Sunday) || loser.rank.is_double()) {
            return (false, "commemoration:first-vespers-circumcision-exclusion");
        }
        if is_day_within_octave(loser) {
            return (false, "commemoration:first-vespers-second-class-octave-exclusion");
        }
    }
    (true, "commemoration:first-vespers-concurrence")
}

/// Whether a Lesser Double, impeded today and commemorated at Lauds, keeps its
/// commemoration at the following office's I Vespers. XIV.9 keeps simplified
/// Doubles at both Vespers "except on all I Class Doubles", but the ordos
/// keep them only where the evening's office admits them (#556):
/// - not before a Double I or II Class: Pentecost and Trinity; St Joseph
///   and St Benedict 2025-2026; St Matthew 2017, 2019 and 2023-2026;
/// - not before a Sunday when today's office ended at None (XIII.17-18): an
///   anticipated Sunday (Clement 2019, Romuald 2026; Edmund 2021 dissents),
///   an Ember Saturday (Januarius 2026) or a vigil (Alban 2024);
/// - otherwise kept: Lent Saturdays before the Sunday (Patrick 2018,
///   Cuthbert 2021, Cyril 2023, Isidore 2026; Cyril 2017 dissents), the
///   Corpus Christi octave (2018, 2021), and an Ember or Rogation day before
///   a Double (Januarius 2018, Augustine 2025).
fn impeded_double_at_first_vespers(winner: Option<&Feast>, impeder: Option<&Feast>) -> (bool, &'static str) {
    let Some(w) = winner else {
        return (false, "commemoration:outgoing-below-greater-double");
    };
    if impeder.is_some_and(|p| p.rank == Rank::Double1stClass && !p.is_category(Category::Sunday) && !p.is_category(Category::Feria)) {
        return (false, "commemoration:impeded-double-first-class-lauds-only");
    }
    if w.is_category(Category::Sunday) {
        let ended_at_none = impeder.is_some_and(|p| is_anticipated_sunday(p) || is_ember_day(p) || is_rogation_day(p) || is_vigil(p));
        if ended_at_none {
            return (false, "commemoration:impeded-double-office-ended-at-none");
        }
    } else if w.rank.weight() >= Rank::Double2ndClass.weight() {
        return (false, "commemoration:impeded-double-before-first-or-second-class");
    }
    (true, "commemoration:impeded-double-at-first-vespers")
}

/// Which office wins when II Vespers of `prec` concurs with I Vespers of
/// `fol` (XIII.2-17, with the parish ordo's resolutions).
pub fn concurrence_winner(prec: &Feast, fol: &Feast) -> (VespersOwner, &'static str) {
    use VespersOwner::{IIOfPreceding, IOfFollowing};
    // 1. Greater Sundays of the I Class.
    if is_sunday_first_class(prec) {
        // Advent I keeps its II Vespers before St Andrew, commemorating him
        // (2025 and 2026 ordos; Fr Jason's 2026 direction on #62), as the
        // Revised Table of Concurrence (Diurnal pp. xlvi-xlvii) gives a
        // following II Class feast no I Vespers. The original Table (p. xlv)
        // and XIII.6 say otherwise, and the ordos are split for the Lent and
        // Low Sundays, which stay on the original Table.
        if prec.id == "advent-sunday-1" && fol.rank.weight() < Rank::Double1stClass.weight() {
            return (IIOfPreceding, "concurrence:advent-sunday-1-vs-class-ii");
        }
        if fol.rank.weight() >= Rank::Double2ndClass.weight() && !fol.is_moveable() && prec.id != "easter-sunday" && prec.id != "pentecost"
        {
            return (IOfFollowing, "concurrence:greater-sunday-vs-class-i-ii");
        }
        return (IIOfPreceding, "concurrence:greater-sunday");
    }
    if is_sunday_first_class(fol) {
        if prec.rank.weight() >= Rank::Double2ndClass.weight() && !prec.is_moveable() && fol.id != "easter-sunday" && fol.id != "pentecost"
        {
            return (IIOfPreceding, "concurrence:greater-sunday-vs-class-i-ii");
        }
        return (IOfFollowing, "concurrence:greater-sunday");
    }
    // 2-3. A feast against another Sunday.
    if is_sunday(prec) != is_sunday(fol) {
        let (feast, feast_wins, sunday_wins, feast_is_prec) =
            if is_sunday(prec) { (fol, IOfFollowing, IIOfPreceding, false) } else { (prec, IIOfPreceding, IOfFollowing, true) };
        if feast.rank.weight() >= Rank::Double2ndClass.weight() {
            return (feast_wins, "concurrence:class-i-ii-vs-sunday");
        }
        if feast_is_prec && is_octave_day(feast) {
            return (feast_wins, "concurrence:octave-day-in-possession-vs-sunday");
        }
        // The Octave Day of the Epiphany takes I Vespers from the Sunday
        // within its Octave, commemorating II Vespers of the Sunday, whether
        // that Sunday falls on 12 January or is said on Saturday the 12th
        // because the Octave Day is a Sunday (Diurnal p. 231; 2019 ordo,
        // 12 January; 2025 ordo, 12 January) (#651). Not other octave days:
        // the Sunday keeps II Vespers before the Octave Day of Ss Peter and
        // Paul (2026 ordo, 5 July).
        if feast.id == "epiphany-octave-day" {
            return (feast_wins, "concurrence:epiphany-octave-day-vs-sunday");
        }
        return (sunday_wins, "concurrence:sunday-below-class-ii");
    }
    // 5. An octave day against a Double below II Class (XIII.10).
    if is_octave_day(prec) != is_octave_day(fol) {
        let (other, octave_wins, other_wins) =
            if is_octave_day(fol) { (prec, IOfFollowing, IIOfPreceding) } else { (fol, IIOfPreceding, IOfFollowing) };
        if other.rank.weight() >= Rank::Double2ndClass.weight() {
            return (other_wins, "concurrence:class-i-ii-vs-octave-day");
        }
        if is_double_or_above(other) {
            return (octave_wins, "concurrence:octave-day-vs-double");
        }
    }
    // 6. Octave day against octave day: the worthier (XIII.11).
    if is_octave_day(prec) && is_octave_day(fol) {
        if compare_feast_precedence(prec, fol) {
            return (IIOfPreceding, "concurrence:octave-day-vs-octave-day");
        }
        return (IOfFollowing, "concurrence:octave-day-vs-octave-day");
    }
    // 7. A Double against a day within an octave or the Saturday BVM.
    if is_double_or_above(prec) && (is_day_within_octave(fol) || is_saturday_bvm(fol)) {
        return (IIOfPreceding, "concurrence:double-vs-octave-or-saturday-bvm");
    }
    // A day within a common octave keeps its II Vespers before a Double II
    // Class, commemorating it (2026 ordo 24 April, Day II of St George's
    // octave before St Mark; Fr Jason's direction on #62, which reads the
    // Revised Table as refusing I Vespers to a following II Class feast).
    // The only such evening in the 2017-2026 ordos; the original Table
    // (Diurnal p. xlv) gives the following office.
    if fol.rank == Rank::Double2ndClass && !is_sunday(fol) && is_day_within_octave(prec) && !is_privileged_octave_commemoration(prec) {
        return (IIOfPreceding, "concurrence:common-octave-day-vs-class-ii");
    }
    if is_double_or_above(fol) && (is_day_within_octave(prec) || is_saturday_bvm(prec)) {
        return (IOfFollowing, "concurrence:double-vs-octave-or-saturday-bvm");
    }
    // 8. The worthier office; equal II Class feasts keep the preceding.
    if compare_feast_precedence(prec, fol) {
        return (IIOfPreceding, "concurrence:general-precedence");
    }
    if !compare_feast_precedence(fol, prec) && prec.rank.weight() >= Rank::Double2ndClass.weight() {
        return (IIOfPreceding, "concurrence:equal-second-class");
    }
    (IOfFollowing, "concurrence:equal-following")
}

/// The commemorations proper to an evening: the concurrence loser, then the
/// following day's occurrence commemorations, subject to XIV.7-9.
fn boundary_commemorations(
    winner: Option<&FeastRef>,
    loser: Option<&FeastRef>,
    preceding: &CalendarDay,
    following: &CalendarDay,
    second_vespers: bool,
    same_octave: bool,
) -> (Vec<FeastRef>, Vec<Decision>) {
    let w = winner.map(|w| &**w);
    let suppress_incoming = second_vespers
        && w.is_some_and(|w| {
            w.rank.weight() >= Rank::Double2ndClass.weight() && !w.is_category(Category::Sunday) && !incoming_double_at_second_vespers(w)
        })
        && loser.is_none_or(|l| l.id != "vigil-epiphany");
    let suppressed = |c: &Feast| {
        if !suppress_incoming || c.is_category(Category::Sunday) || c.is_category(Category::Feria) {
            return false;
        }
        !incoming_commemorated_at_second_vespers(w, c)
    };

    let mut comms: Vec<FeastRef> = Vec::new();
    let mut decisions = Vec::new();
    let mut loser_included = false;
    let mut concurrent = loser.cloned();
    if let Some(l) = loser {
        if same_octave {
            decisions.push(decision("commemoration:same-octave-boundary", "suppressed", &l.id));
        } else {
            let (included, rule) = if !second_vespers {
                outgoing_commemorated_at_first_vespers(w, l)
            } else {
                following_office_commemorated_at_second_vespers(w, following)
            };
            if included {
                comms.push(l.clone());
                loser_included = !second_vespers;
            }
            decisions.push(decision(rule, if included { "included" } else { "suppressed" }, &l.id));
        }
    }
    if !second_vespers {
        let mut c = preceding.feria_commemoration.clone();
        // A free seasonal feria still survives at the next feast's I Vespers
        // (Scholastica, Diurnal p. 476; 2026 ordo Feb 9).
        if c.is_none()
            && preceding.celebration.is_none()
            && w.is_some_and(|w| !w.is_category(Category::Sunday))
            && preceding.date.weekday() != Weekday::Sunday
            && is_penitential_feria_season(preceding.season)
        {
            c = Some(Arc::new(seasonal_feria_commemoration(preceding, &MoveableDates::compute(preceding.date.year()))));
        }
        if let Some(c) = c.filter(|c| {
            let saturday = saturday_feria_without_vespers(preceding, c);
            if saturday {
                decisions.push(decision("commemoration:saturday-feria-without-vespers", "suppressed", &c.id));
            }
            !saturday
        }) {
            let (included, rule) = outgoing_commemorated_at_first_vespers(w, &c);
            if included {
                comms.push(c.clone());
                if preceding.celebration.is_none() {
                    concurrent = Some(c.clone());
                }
            }
            decisions.push(decision(rule, if included { "included" } else { "suppressed" }, &c.id));
        }
        for c in &preceding.commemorations {
            if c.is_category(Category::Feria) || is_day_within_octave(c) || is_octave_day(c) {
                continue;
            }
            if is_apostolic_companion_commemoration(c) && loser_included {
                comms.push(c.clone());
                decisions.push(decision("commemoration:outgoing-apostolic-companion", "included", &c.id));
                continue;
            }
            if c.rank == Rank::Double {
                let (kept, rule) = impeded_double_at_first_vespers(w, preceding.celebration.as_deref());
                if !kept {
                    decisions.push(decision(rule, "suppressed", &c.id));
                    continue;
                }
            } else if c.rank.weight() < Rank::GreaterDouble.weight() {
                decisions.push(decision("commemoration:outgoing-below-greater-double", "suppressed", &c.id));
                continue;
            }
            let (included, rule) = outgoing_commemorated_at_first_vespers(w, c);
            if included {
                comms.push(c.clone());
            }
            decisions.push(decision(rule, if included { "included" } else { "suppressed" }, &c.id));
        }
    }
    for c in &following.commemorations {
        if second_vespers && is_day_within_octave(c) && octave_celebration_parent(preceding) == octave_parent_id(c) {
            decisions.push(decision("commemoration:same-octave-boundary", "suppressed", &c.id));
            continue;
        }
        if incoming_feria_excluded_at_vespers(c) {
            decisions.push(decision("commemoration:incoming-feria-not-at-vespers-boundary", "suppressed", &c.id));
            continue;
        }
        if !second_vespers
            && is_day_within_octave(c)
            && w.is_some_and(|w| w.rank.weight() >= Rank::Double2ndClass.weight() && !w.is_category(Category::Sunday))
        {
            decisions.push(decision("commemoration:first-vespers-day-within-octave-exclusion", "suppressed", &c.id));
            continue;
        }
        // Diurnal §VIII and §X (pp. xxix-xxx), XIV.8-9: a Simple Office,
        // Memorial or Simple Octave Day is "not commemorated at I Vespers" of
        // a Double II Class (#138 ruling item 2; 2026 ordo 2 May, 1 July,
        // 5 August, 14 September). The 2026 ordo keeps one on 24 July (St
        // Christopher, printed in every ordo but 2024), 7 September (Hadrian,
        // 2023's line word for word) and 3 January; the app follows the
        // rubric until clergy rule (#390).
        if !second_vespers
            && matches!(c.rank, Rank::Simple | Rank::Commemoration)
            && !is_apostolic_companion_commemoration(c)
            && w.is_some_and(|w| w.rank == Rank::Double2ndClass && !w.is_category(Category::Sunday))
        {
            decisions.push(decision("commemoration:first-vespers-second-class-memorial-exclusion", "suppressed", &c.id));
            continue;
        }
        if !second_vespers
            && loser_included
            && let Some(l) = loser.filter(|l| same_octave_days(l, c))
        {
            // The following octave day supersedes today's day within the
            // octave (XIII.16; 2025 ordo 5 July, "Comm. Oct. ('Peter the
            // Apostle' 558; Col. 560)"; #622).
            if !(is_octave_day(c) && is_day_within_octave(l)) {
                decisions.push(decision("commemoration:first-vespers-duplicate-octave-day", "suppressed", &c.id));
                continue;
            }
            comms.retain(|x| x.id != l.id);
            loser_included = false;
            decisions.push(decision("commemoration:octave-day-supersedes-day-within", "suppressed", &l.id));
        }
        let (included, rule) = occurrence_commemorated_at_first_vespers(c);
        if !included {
            decisions.push(decision(rule, "suppressed", &c.id));
        } else if !suppressed(c) {
            comms.push(c.clone());
            decisions.push(decision(rule, "included", &c.id));
        } else {
            decisions.push(decision("commemoration:incoming-at-second-vespers", "suppressed", &c.id));
        }
    }
    // The preceding day always exists here, so it supplies the season.
    let incoming: Vec<FeastRef> = following.celebration.iter().chain(&following.commemorations).cloned().collect();
    let (finalized, final_decisions) = ordered_commemorations(
        winner,
        &comms,
        OrderContext {
            season: Some(preceding.season),
            concurrent,
            incoming,
            incoming_season: Some(following.season),
            ..OrderContext::default()
        },
    );
    decisions.extend(final_decisions);
    (finalized, decisions)
}

fn second_vespers_commemorations(
    winner: &FeastRef,
    day: &CalendarDay,
    following: &CalendarDay,
    boundary: Vec<FeastRef>,
    boundary_decisions: Vec<Decision>,
) -> (Vec<FeastRef>, Vec<Decision>) {
    let following_office_id = following.celebration.as_ref().map(|c| c.id.as_str());
    let is_following_office = |c: &FeastRef| Some(c.id.as_str()) == following_office_id;
    let occurrence: Vec<FeastRef> = day.commemorations.iter().chain(&day.feria_commemoration).cloned().collect();
    let mut comms = Vec::new();
    let mut decisions = boundary_decisions;

    // Keep a continuing octave once, as today's commemoration.
    let mut filtered = Vec::with_capacity(boundary.len());
    let mut concurrent_octave: Option<FeastRef> = None;
    for comm in boundary {
        let mut duplicate = false;
        if is_day_within_octave(&comm) {
            for current in &occurrence {
                if is_day_within_octave(current)
                    && same_octave_days(current, &comm)
                    && occurrence_commemorated_at_second_vespers(Some(winner), current).0
                {
                    duplicate = true;
                    if is_following_office(&comm) {
                        concurrent_octave = Some(current.clone());
                    }
                    break;
                }
            }
        }
        if duplicate {
            decisions.push(decision("commemoration:duplicate-octave-boundary", "suppressed", &comm.id));
        } else {
            filtered.push(comm);
        }
    }
    let boundary = filtered;
    // The concurrent following office is commemorated first (XIV.14).
    comms.extend(boundary.iter().filter(|c| is_following_office(c)).cloned());
    if let Some(c) = &concurrent_octave {
        comms.push(c.clone());
    }
    for comm in &occurrence {
        // The following octave day supersedes today's day within the same
        // octave (XIII.16; 2026 ordo 5 July, 2018 ordo 29 April, 2021 ordo
        // 7 November).
        if is_day_within_octave(comm) && boundary.iter().any(|b| is_octave_day(b) && same_octave_days(b, comm)) {
            decisions.push(decision("commemoration:octave-day-supersedes-day-within", "suppressed", &comm.id));
            continue;
        }
        if saturday_feria_without_vespers(day, comm) {
            decisions.push(decision("commemoration:saturday-feria-without-vespers", "suppressed", &comm.id));
            continue;
        }
        let (included, rule) = occurrence_commemorated_at_second_vespers(Some(winner), comm);
        if included {
            if !same(&concurrent_octave, comm) {
                comms.push(comm.clone());
            }
            decisions.push(decision(rule, "included", &comm.id));
        } else {
            decisions.push(decision(rule, "suppressed", &comm.id));
        }
    }
    comms.extend(boundary.iter().filter(|c| !is_following_office(c)).cloned());
    let mut concurrent = concurrent_octave;
    for comm in boundary.iter().filter(|c| is_following_office(c)) {
        concurrent = Some(comm.clone());
    }
    let (finalized, final_decisions) = ordered_commemorations(
        Some(winner),
        &comms,
        OrderContext {
            season: Some(day.season),
            concurrent,
            incoming: boundary,
            incoming_season: Some(following.season),
            ..OrderContext::default()
        },
    );
    decisions.extend(final_decisions);
    (finalized, decisions)
}

/// An evening on which neither adjacent celebration owns I/II Vespers: the
/// current office stays, with both days' eligible commemorations (XIV.9).
fn no_owner_commemorations(preceding: &CalendarDay, following: &CalendarDay) -> (Vec<FeastRef>, Vec<Decision>) {
    let mut current = Vec::new();
    let mut incoming = Vec::new();
    let mut decisions = Vec::new();
    for comm in &preceding.commemorations {
        if saturday_feria_without_vespers(preceding, comm) {
            decisions.push(decision("commemoration:saturday-feria-without-vespers", "suppressed", &comm.id));
            continue;
        }
        let (included, rule) = occurrence_commemorated_at_second_vespers(preceding.celebration.as_deref(), comm);
        if included {
            current.push(comm.clone());
        }
        decisions.push(decision(rule, if included { "included" } else { "suppressed" }, &comm.id));
    }
    for comm in &following.commemorations {
        let (included, rule) = occurrence_commemorated_at_first_vespers(comm);
        // Holy Wednesday does not commemorate Thursday's feria (2026 ordo 8 April).
        if included && incoming_feria_excluded_at_vespers(comm) {
            decisions.push(decision("commemoration:incoming-feria-not-at-vespers-boundary", "suppressed", &comm.id));
            continue;
        }
        if included {
            incoming.push(comm.clone());
            decisions.push(decision("commemoration:incoming-at-unowned-vespers", "included", &comm.id));
        } else {
            decisions.push(decision(rule, "suppressed", &comm.id));
        }
    }
    let (current, current_decisions) = ordered_commemorations(
        preceding.celebration.as_ref(),
        &current,
        OrderContext { season: Some(preceding.season), ..OrderContext::default() },
    );
    let (incoming, incoming_decisions) = ordered_commemorations(
        following.celebration.as_ref(),
        &incoming,
        OrderContext { season: Some(following.season), ..OrderContext::default() },
    );
    decisions.extend(current_decisions);
    decisions.extend(incoming_decisions);
    let all: Vec<FeastRef> = current.into_iter().chain(incoming.iter().cloned()).collect();
    let (combined, dedupe_decisions) = dedupe_commemorations(None, &all);
    let combined = order_commemorations(
        &combined,
        &OrderContext {
            season: Some(preceding.season),
            winner: preceding.celebration.clone(),
            incoming,
            incoming_season: Some(following.season),
            ..OrderContext::default()
        },
    );
    let (combined, cap_decisions) = cap_commemorations(combined);
    decisions.extend(dedupe_decisions);
    decisions.extend(cap_decisions);
    (combined, decisions)
}

/// The Vespers designation for the evening between `preceding` and
/// `following`. A plain feria (no celebration) has no Vespers rights of its
/// own, but the following day's I Vespers still applies.
pub fn resolve_concurrence(preceding: &CalendarDay, following: &CalendarDay) -> VespersDesignation {
    let mut d = resolve_concurrence_owner(preceding, following);
    let incoming: Vec<&str> = following.celebration.iter().chain(&following.commemorations).map(|f| f.id.as_str()).collect();
    d.incoming_commemoration_ids = d.commemorations.iter().filter(|c| incoming.contains(&c.id.as_str())).map(|c| c.id.clone()).collect();
    d
}

fn resolve_concurrence_owner(preceding: &CalendarDay, following: &CalendarDay) -> VespersDesignation {
    // All Souls ends at None; Vespers are of the displaced All Saints octave.
    // Needs ruling (#662): these take II Vespers of the feast, as the 2023 and
    // 2026 ordos print ("As of II Vesp. of Feast"); Diurnal p. 654 says "as at
    // I Vespers of the Feast", and the 2017-2022 ordos ("As of the Feast,
    // except: Mag. Ant. 'O how glorious'") may mean that too.
    if let Some(octave) = all_souls_octave_vespers_office(preceding) {
        let mut synth = preceding.clone();
        synth.celebration = Some(octave.clone());
        synth.color = octave.color;
        synth.commemorations.retain(|c| c.id != octave.id);
        let mut result = resolve_concurrence(&synth, following);
        if result.owner == VespersOwner::NotApplicable {
            result.owner = VespersOwner::IIOfPreceding;
            result.feast = Some(octave.clone());
            result.color = Some(octave.color);
            result.season = Some(preceding.season);
            result.rule = "concurrence:all-souls-ends-at-none".to_string();
        }
        return result;
    }

    let prec = preceding.celebration.as_ref();
    let fol = following.celebration.as_ref();
    let prec_has_ii = prec.is_some_and(|p| has_second_vespers(p));
    let fol_has_i = fol.is_some_and(|f| has_first_vespers(f));
    let same_octave = same_octave_office(preceding, following);

    let first_vespers = |rule: &str, fol: &FeastRef| {
        let (comms, decisions) = boundary_commemorations(Some(fol), prec, preceding, following, false, same_octave);
        let mut d = VespersDesignation::new(VespersOwner::IOfFollowing, rule);
        d.feast = Some(fol.clone());
        d.color = Some(following.color);
        d.season = Some(following.season);
        d.within_octave_of = following.within_octave_of.clone();
        d.commemorations = comms;
        d.decisions = decisions;
        apply_chapter_split(d, fol, preceding)
    };
    let second_vespers = |rule: &str, prec: &FeastRef| {
        let (boundary, boundary_decisions) = boundary_commemorations(Some(prec), fol, preceding, following, true, same_octave);
        let (comms, decisions) = second_vespers_commemorations(prec, preceding, following, boundary, boundary_decisions);
        let mut d = VespersDesignation::new(VespersOwner::IIOfPreceding, rule);
        d.feast = Some(prec.clone());
        d.color = Some(preceding.color);
        d.season = Some(preceding.season);
        d.following_office_octave_of = octave_celebration_parent(following).map(str::to_string);
        d.commemorations = comms;
        d.decisions = decisions;
        d
    };

    match (prec, fol) {
        (Some(p), Some(f)) if prec_has_ii && fol_has_i => {
            let (winner, rule) = concurrence_winner(p, f);
            match winner {
                VespersOwner::IIOfPreceding => {
                    let mut d = second_vespers(rule, p);
                    if d.commemorations.iter().any(|c| c.id == f.id) {
                        d.following_office_commemoration_id = Some(f.id.clone());
                    }
                    d
                }
                VespersOwner::IOfFollowing | VespersOwner::NotApplicable => first_vespers(rule, f),
            }
        }
        (_, Some(f)) if !prec_has_ii && fol_has_i => first_vespers("concurrence:following-only", f),
        (Some(p), _) if prec_has_ii => second_vespers("concurrence:preceding-only", p),
        _ => {
            let (comms, decisions) = no_owner_commemorations(preceding, following);
            let mut d = VespersDesignation::new(VespersOwner::NotApplicable, "concurrence:neither-office-has-rights");
            d.commemorations = comms;
            d.decisions = decisions;
            d
        }
    }
}

/// The All Saints octave day that All Souls displaced.
fn all_souls_octave_vespers_office(day: &CalendarDay) -> Option<&FeastRef> {
    if day.celebration.as_ref()?.id != "all-souls" {
        return None;
    }
    day.commemorations.iter().find(|c| c.id.starts_with("all-saints-octave-day"))
}

/// Marks a designation whose incoming Simple begins only at the Chapter. The
/// colour follows the outgoing office when the day has an office of its own.
fn apply_chapter_split(mut d: VespersDesignation, fol: &Feast, preceding: &CalendarDay) -> VespersDesignation {
    if !splits_vespers_at_chapter(fol) {
        return d;
    }
    d.psalmody_from_preceding = true;
    if preceding.celebration.is_some() {
        d.color = Some(preceding.color);
    }
    d.decisions.push(decision("concurrence:simple-begins-at-chapter", "split", &fol.name));
    d
}

/// The designation for each evening of `days`, where `following` is the day
/// after the last.
pub fn resolve_vespers(days: &[CalendarDay], following: &CalendarDay) -> Vec<VespersDesignation> {
    (0..days.len())
        .map(|i| {
            let next = days.get(i + 1).unwrap_or(following);
            let mut v = resolve_concurrence(&days[i], next);
            if let Some(c) = &next.celebration
                && c.id == "all-souls"
            {
                v.appended_office_of_the_dead = true;
                v.appended_feast = Some(c.clone());
                v.decisions.push(decision("vespers:appended-office-of-the-dead", "included", "all-souls"));
            }
            v
        })
        .collect()
}

#[cfg(test)]
mod tests;
