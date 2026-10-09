//! Commemoration eligibility, de-duplication, and XIV.14 ordering.

use std::collections::HashSet;
use std::sync::Arc;

use crate::model::{CalendarDay, Category, CommemorationClass, Decision, Feast, FeastRef, OctaveClass, Rank, Season};
use crate::traits::{
    is_apostolic_companion_commemoration, is_day_within_octave, is_double_or_above, is_ember_day, is_octave_day, is_rogation_day,
    is_sunday, is_sunday_first_class, is_vigil, same_octave_days,
};

/// The Lauds commemorations: the occurrence commemorations plus the displaced
/// seasonal feria, in XIV.14 order.
pub fn lauds_commemorations(day: &CalendarDay) -> Vec<FeastRef> {
    let mut comms = day.commemorations.clone();
    if let Some(f) = &day.feria_commemoration {
        comms.push(f.clone());
    }
    order_commemorations(&comms, &OrderContext { season: Some(day.season), winner: day.celebration.clone(), ..OrderContext::default() })
}

/// Ordering is separate from eligibility, duplicate selection, and text context. Feasts are
/// compared by identity (`Arc::ptr_eq`).
#[derive(Clone, Debug, Default)]
pub struct OrderContext {
    // Without a season, no seasonal eligibility rule matches.
    pub season: Option<Season>,
    pub winner: Option<FeastRef>,
    pub concurrent: Option<FeastRef>,
    pub incoming: Vec<FeastRef>,
    pub incoming_season: Option<Season>,
}

fn contains_ptr(list: &[FeastRef], f: &FeastRef) -> bool {
    list.iter().any(|x| Arc::ptr_eq(x, f))
}

/// Sorts complete office groups (an office followed by its companions) by
/// XIV.14 tier, keeping ties stable.
pub fn order_commemorations(comms: &[FeastRef], ctx: &OrderContext) -> Vec<FeastRef> {
    struct Group {
        office: FeastRef,
        companions: Vec<FeastRef>,
    }
    let present: HashSet<&str> = comms.iter().map(|f| f.id.as_str()).collect();
    let mut groups: Vec<Group> = comms
        .iter()
        .filter(|f| match &f.companion_of {
            None => true,
            Some(parent) => !present.contains(parent.as_str()),
        })
        .map(|f| Group { office: f.clone(), companions: Vec::new() })
        .collect();
    for g in &mut groups {
        for f in comms {
            if f.companion_of.as_deref() == Some(g.office.id.as_str()) {
                g.companions.push(f.clone());
            }
        }
    }
    let season = |f: &FeastRef| -> Option<Season> {
        if contains_ptr(&ctx.incoming, f) && ctx.incoming_season.is_some() {
            return ctx.incoming_season;
        }
        ctx.season
    };
    // XIV.14(a) promotes a Feast of the Lord over a lesser Sunday or the
    // Epiphany vigil, as a list-level promotion.
    let prefer_lord = comms.iter().any(|f| is_lesser_sunday(f, season(f)) || is_epiphany_vigil(f));
    let concurrent = |g: &Group| match &ctx.concurrent {
        None => false,
        Some(c) => Arc::ptr_eq(&g.office, c) || contains_ptr(&g.companions, c),
    };
    let tier = |f: &FeastRef| -> i32 {
        if let (Some(w), Some(parent)) = (&ctx.winner, &f.companion_of)
            && *parent == w.id
        {
            return -1;
        }
        commemoration_tier(f, season(f), prefer_lord)
    };
    groups.sort_by(|left, right| {
        let (cl, cr) = (concurrent(left), concurrent(right));
        if cl != cr {
            return if cl { std::cmp::Ordering::Less } else { std::cmp::Ordering::Greater };
        }
        tier(&left.office)
            .cmp(&tier(&right.office))
            // Same-tier dignity is not fully modeled (#402, #403).
            .then(right.office.rank.weight().cmp(&left.office.rank.weight()))
    });
    let mut ordered = Vec::with_capacity(comms.len());
    for g in groups {
        ordered.push(g.office);
        ordered.extend(g.companions);
    }
    ordered
}

fn is_epiphany_vigil(f: &Feast) -> bool {
    f.commemoration_class == CommemorationClass::EpiphanyVigil
}

fn is_lesser_sunday(f: &Feast, season: Option<Season>) -> bool {
    if !is_sunday(f) || is_sunday_first_class(f) {
        return false;
    }
    !matches!(season, Some(Season::Advent | Season::Septuagesima | Season::Lent | Season::Passiontide))
}

fn is_lent_or_passiontide(season: Option<Season>) -> bool {
    matches!(season, Some(Season::Lent | Season::Passiontide))
}

/// Tiers of General Rubrics XIV.14(a-n), pp.26-27, distinct from
/// occurrence precedence.
fn commemoration_tier(f: &Feast, season: Option<Season>, prefer_lord: bool) -> i32 {
    if is_sunday(f) {
        return if is_lesser_sunday(f, season) { 2 } else { 0 };
    }
    if is_epiphany_vigil(f) {
        return 2;
    }
    if prefer_lord && f.is_category(Category::Lord) && is_double_or_above(f) && !is_octave_day(f) && !is_day_within_octave(f) {
        return 1;
    }
    if is_day_within_octave(f) && f.octave_class == OctaveClass::PrivilegedSecond {
        return 3;
    }
    if is_ember_day(f) || is_rogation_day(f) || (f.is_category(Category::Feria) && !is_vigil(f) && is_lent_or_passiontide(season)) {
        return 4;
    }
    if is_octave_day(f) && f.rank == Rank::GreaterDouble {
        return 5;
    }
    if f.rank.weight() >= Rank::GreaterDouble.weight() {
        return 6;
    }
    if f.rank == Rank::Double {
        return 7;
    }
    if is_day_within_octave(f) && f.octave_class == OctaveClass::PrivilegedThird {
        return 8;
    }
    if f.commemoration_class == CommemorationClass::PostAscensionFeria {
        return 9;
    }
    if is_day_within_octave(f) {
        return 10;
    }
    if f.is_category(Category::Feria) && !is_vigil(f) && matches!(season, Some(Season::Advent | Season::Septuagesima)) {
        return 11;
    }
    if is_vigil(f) {
        return 12;
    }
    if f.rank == Rank::Simple && (is_octave_day(f) || f.octave_class == OctaveClass::Simple) {
        return 13;
    }
    if f.rank != Rank::Commemoration {
        return 14;
    }
    15
}

/// Days that always take the office regardless of conflicting feasts.
pub const PRIVILEGED_FEAST_IDS: [&str; 14] = [
    "ash-wednesday",
    "palm-sunday",
    "holy-monday",
    "holy-tuesday",
    "holy-wednesday",
    "holy-thursday",
    "good-friday",
    "holy-saturday",
    "easter-sunday",
    "easter-monday",
    "easter-tuesday",
    "low-sunday",
    "christmas",
    "vigil-nativity",
];

pub fn is_privileged_feast(f: &Feast) -> bool {
    PRIVILEGED_FEAST_IDS.contains(&f.id.as_str())
}

/// Folds a name for fuzzy duplicate detection.
pub fn normalize_commemoration_name(name: &str) -> String {
    // Use Unicode simple lowercase, mapping one character to one character.
    let mut n: String = name.trim().chars().map(|c| c.to_lowercase().next().unwrap_or(c)).collect();
    for (from, to) in [
        ("&", " and "),
        ("pope", "bishop"),
        (".", " "),
        (",", " "),
        (";", " "),
        ("(", " "),
        (")", " "),
        ("'", " "),
        ("’", " "),
        ("-", " "),
        ("commemoration of", " "),
        ("apostles", " "),
        ("apostle", " "),
        (" the ", " "),
    ] {
        n = n.replace(from, to);
    }
    // Trim surrounding Unicode whitespace, then collapse ASCII whitespace (`[\t\n\f\r ]`) to one
    // space.
    let mut out = String::with_capacity(n.len());
    let mut in_space = false;
    for c in n.trim().chars() {
        if matches!(c, '\t' | '\n' | '\u{c}' | '\r' | ' ') {
            if !in_space {
                out.push(' ');
            }
            in_space = true;
        } else {
            out.push(c);
            in_space = false;
        }
    }
    out
}

/// Drops commemorations whose normalized name matches the winner's or an
/// earlier commemoration's (equal or contained).
pub fn dedupe_commemorations(winner: Option<&Feast>, comms: &[FeastRef]) -> (Vec<FeastRef>, Vec<Decision>) {
    let winner_key = winner.map(|w| normalize_commemoration_name(&w.name)).unwrap_or_default();
    let same_or_contained = |a: &str, b: &str| {
        if a.is_empty() || b.is_empty() {
            return a == b;
        }
        a == b || a.contains(b) || b.contains(a)
    };
    let mut seen: Vec<String> = Vec::new();
    let mut deduped = Vec::new();
    let mut decisions = Vec::new();
    for comm in comms {
        let mut key = normalize_commemoration_name(&comm.name);
        if key.is_empty() {
            key = comm.id.clone();
        }
        if same_or_contained(&key, &winner_key) {
            decisions.push(Decision::new("commemoration:matches-winner", "suppressed", comm.id.as_str()));
            continue;
        }
        if seen.iter().any(|prior| same_or_contained(&key, prior)) {
            decisions.push(Decision::new("commemoration:duplicate-name", "suppressed", comm.id.as_str()));
            continue;
        }
        seen.push(key);
        deduped.push(comm.clone());
    }
    (deduped, decisions)
}

/// A sanity guard, not a rubric: the pre-1962 monastic rubrics do not cap
/// commemorations, and the 2026 ordo prints four at some Vespers.
pub const MAX_COMMEMORATIONS_PER_DAY: usize = 6;

fn suppresses_st_george_octave(winner: Option<&Feast>) -> bool {
    match winner {
        None => false,
        Some(w) => is_privileged_feast(w) || w.id.starts_with("easter-sunday-octave-day-"),
    }
}

/// Diurnal VIII: a Memorial is not commemorated on a Double I Class feast.
fn suppresses_memorials(winner: Option<&Feast>) -> bool {
    let Some(w) = winner else { return false };
    // Easter and Pentecost Monday and Tuesday omit their Memorials in every
    // ordo 2017–2026, several marked "omitted this year" (Diurnal VII; #380).
    if matches!(w.id.as_str(), "easter-monday" | "easter-tuesday" | "pentecost-octave-day-2" | "pentecost-octave-day-3") {
        return true;
    }
    if w.rank != Rank::Double1stClass
        || w.is_category(Category::Sunday)
        || w.is_category(Category::Feria)
        || w.is_vigil
        || is_day_within_octave(w)
    {
        return false;
    }
    // Low Sunday is printed Gd in the 2026 ordo.
    w.id != "low-sunday"
}

/// Fr Jason's #138 ruling: St Joseph's Solemnity suppresses ordinary
/// commemorations like a Primary Feast of Our Lord (2024 omits Romanus, 2026
/// the St George octave; #378).
fn suppresses_like_primary_feast(w: &Feast) -> bool {
    w.primary_of_our_lord || w.id == "solemnity-st-joseph"
}

/// General Rubrics XIV.4: "if the Double Feast is of the I or II Class, no
/// Commemoration may be made of a day within an Octave, unless the Octave is a
/// privileged one" (St George's octave on St Joseph's Solemnity, St John
/// Baptist's on Ss Peter and Paul: 2018–2026 ordos; #378). Doubles II Class
/// follow it too (#573): St George's octave on St Mark (No Comm. 2017, 2018,
/// 2023, 2025), the Assumption's on St Joachim (No Comm. 2017–2019 and 2026),
/// and the Visitation's own rubric (Diurnal p. 557). The 2026 St Mark and
/// 2021–2025 St Joachim Lauds lines that commemorate the octave are the
/// exceptions.
fn suppresses_common_octave(w: &Feast, comm: &Feast) -> bool {
    matches!(w.rank, Rank::Double1stClass | Rank::Double2ndClass)
        && !w.is_category(Category::Sunday)
        && is_day_within_octave(comm)
        && !comm.is_privileged_octave_day
}

fn commemoration_suppression(winner: Option<&Feast>, comm: &Feast) -> Option<Decision> {
    if let Some(only) = &comm.only_with
        && winner.is_none_or(|w| w.id != *only)
    {
        return Some(Decision::new("commemoration:only-with", "suppressed", format!("{} requires {only}", comm.id)));
    }
    let w = winner?;
    if comm.rank == Rank::Commemoration
        && !is_apostolic_companion_commemoration(comm)
        && !comm.is_category(Category::Sunday)
        && !comm.is_category(Category::Feria)
        && !comm.is_vigil
        && suppresses_memorials(Some(w))
    {
        return Some(Decision::new("commemoration:memorial-under-first-class-feast", "suppressed", comm.id.as_str()));
    }
    // General Rubrics VI.2: a Vigil falling on a Solemnity has not even a
    // Commemoration, "unless it is the Vigil of the Epiphany" (semi-double
    // here, so not caught). XIV.7 admits no Common Vigil in occurrence at a I
    // Class Double. Corpus Christi 2022, the transferred Nativity of St John
    // Baptist 2021 and the Vigil of Pentecost 2024 print no Comm. of the
    // Vigil (#616).
    if comm.is_vigil && comm.rank == Rank::Simple && w.rank == Rank::Double1stClass {
        return Some(Decision::new("commemoration:vigil-on-first-class-double", "suppressed", comm.id.as_str()));
    }
    if w.id.starts_with("pentecost-octave-day-") && comm.id.starts_with("whit-ember-") {
        return Some(Decision::new("commemoration:pentecost-ember", "suppressed", comm.id.as_str()));
    }
    if suppresses_st_george_octave(Some(w)) && comm.id.starts_with("st-george-octave-day") {
        return Some(Decision::new("commemoration:st-george-octave", "suppressed", comm.id.as_str()));
    }
    // XI.10 / XII.5: a feast of the same Person within its octave (St Paul on
    // 30 June) omits the commemoration of that octave (Diurnal p. 549).
    if is_day_within_octave(comm) && !is_day_within_octave(w) && same_octave_days(w, comm) {
        return Some(Decision::new("commemoration:same-person-octave", "suppressed", comm.id.as_str()));
    }
    if suppresses_common_octave(w, comm) {
        return Some(Decision::new("commemoration:common-octave-under-first-class-feast", "suppressed", comm.id.as_str()));
    }
    None
}

/// Fr Jason's #138 ruling keeps an occurring Apostle as a narrow local
/// exception, "not beyond what the Ordo prints": Barnabas on Trinity 2017 and
/// 2023 and on Corpus Christi 2026, St Paul on Trinity 2024, each at I
/// Vespers, Lauds and II Vespers (#379).
pub fn apostle_kept_on_primary_feast(winner: &Feast, comm: &Feast) -> bool {
    comm.is_category(Category::Apostle) && matches!(winner.id.as_str(), "trinity-sunday" | "corpus-christi")
}

/// General Rubrics X: at a Primary Feast of Our Lord, an occurring Greater or
/// Lesser Double is not commemorated (octaves excepted). St Joseph's Solemnity
/// is treated the same way (#138), and Trinity and Corpus Christi keep an
/// occurring Apostle (#379).
pub fn primary_feast_doubles(winner: Option<&Feast>, comms: Vec<FeastRef>) -> (Vec<FeastRef>, Vec<Decision>) {
    let Some(w) = winner.filter(|w| suppresses_like_primary_feast(w)) else {
        return (comms, Vec::new());
    };
    let mut kept = Vec::new();
    let mut decisions = Vec::new();
    for comm in comms {
        if matches!(comm.rank, Rank::GreaterDouble | Rank::Double)
            && !apostle_kept_on_primary_feast(w, &comm)
            && !comm.is_category(Category::Sunday)
            && !comm.is_category(Category::Feria)
            && !comm.is_vigil
            && !is_day_within_octave(&comm)
            && !is_octave_day(&comm)
        {
            decisions.push(Decision::new("commemoration:double-under-primary-feast-of-our-lord", "suppressed", comm.id.as_str()));
            continue;
        }
        kept.push(comm);
    }
    (kept, decisions)
}

/// Eligibility, then de-duplication, then XIV.14 order, then the cap.
pub fn ordered_commemorations(winner: Option<&FeastRef>, comms: &[FeastRef], ctx: OrderContext) -> (Vec<FeastRef>, Vec<Decision>) {
    let ctx = OrderContext { winner: winner.cloned(), ..ctx };
    let mut filtered = Vec::with_capacity(comms.len());
    let mut decisions = Vec::new();
    for comm in comms {
        match commemoration_suppression(winner.map(|w| &**w), comm) {
            Some(d) => decisions.push(d),
            None => filtered.push(comm.clone()),
        }
    }
    let (deduped, dedupe_decisions) = dedupe_commemorations(winner.map(|w| &**w), &filtered);
    decisions.extend(dedupe_decisions);
    let ordered = order_commemorations(&deduped, &ctx);
    let (capped, cap_decisions) = cap_commemorations(ordered);
    decisions.extend(cap_decisions);
    (capped, decisions)
}

pub fn cap_commemorations(mut comms: Vec<FeastRef>) -> (Vec<FeastRef>, Vec<Decision>) {
    if comms.len() <= MAX_COMMEMORATIONS_PER_DAY {
        return (comms, Vec::new());
    }
    let dropped: Vec<&str> = comms[MAX_COMMEMORATIONS_PER_DAY..].iter().map(|c| c.id.as_str()).collect();
    let decision = Decision::new("commemoration:cap", "truncated", dropped.join(","));
    comms.truncate(MAX_COMMEMORATIONS_PER_DAY);
    (comms, vec![decision])
}
