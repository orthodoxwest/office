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
    is_apostolic_companion_commemoration, is_day_within_octave, is_double_or_above, is_ember_day, is_octave_day,
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

/// Whether a Lauds commemoration remains at II Vespers of the winning office.
fn occurrence_commemorated_at_second_vespers(winner: Option<&Feast>, comm: &Feast) -> (bool, &'static str) {
    if comm.id == FERIA_COMMEMORATION_ID {
        return (true, "commemoration:second-vespers-seasonal-feria");
    }
    if is_ember_day(comm) || is_rogation_day(comm) || is_vigil(comm) {
        return (false, "commemoration:second-vespers-feria-or-vigil-lauds-only");
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
    f.id == FERIA_COMMEMORATION_ID
        || f.id == "privileged-lenten-feria"
        || f.id.starts_with("lent-ember-")
        || f.id.starts_with("advent-ember-")
}

/// XIV.7-8 applied to the office displaced by I Vespers of the following.
fn outgoing_commemorated_at_first_vespers(winner: Option<&Feast>, loser: &Feast) -> (bool, &'static str) {
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
        if w.id == "circumcision" && (loser.is_category(Category::Sunday) || loser.rank.weight() >= Rank::GreaterDouble.weight()) {
            return (false, "commemoration:first-vespers-circumcision-exclusion");
        }
        if is_day_within_octave(loser) {
            return (false, "commemoration:first-vespers-second-class-octave-exclusion");
        }
    }
    (true, "commemoration:first-vespers-concurrence")
}

/// Which office wins when II Vespers of `prec` concurs with I Vespers of
/// `fol` (XIII.2-17, with the parish ordo's resolutions).
pub fn concurrence_winner(prec: &Feast, fol: &Feast) -> (VespersOwner, &'static str) {
    use VespersOwner::{IIOfPreceding, IOfFollowing};
    // 1. Greater Sundays of the I Class.
    if is_sunday_first_class(prec) {
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
        && w.is_some_and(|w| w.rank.weight() >= Rank::Double2ndClass.weight() && !w.is_category(Category::Sunday))
        && loser.is_none_or(|l| l.id != "vigil-epiphany");
    let suppressed = |c: &Feast| {
        if !suppress_incoming || c.is_category(Category::Sunday) || c.is_category(Category::Feria) {
            return false;
        }
        !occurrence_commemorated_at_second_vespers(w, c).0
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
            if c.rank.weight() < Rank::GreaterDouble.weight() {
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
        if !second_vespers && loser_included && loser.is_some_and(|l| same_octave_days(l, c)) {
            decisions.push(decision("commemoration:first-vespers-duplicate-octave-day", "suppressed", &c.id));
            continue;
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
