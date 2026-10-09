//! Occurrence: which of a day's candidates takes the office, which are
//! commemorated, and which are transferred.

use std::sync::Arc;

use crate::commemoration::{OrderContext, is_privileged_feast, ordered_commemorations, primary_feast_doubles};
use crate::date::{Date, Weekday};
use crate::model::{CalendarDay, Category, Color, Decision, Feast, FeastRef, Penitential, Rank, Season};
use crate::traits::{is_ember_day, is_privileged_feria};

/// The precedence key (higher wins). Sundays below Greater Double are
/// boosted to Greater Double.
fn sort_key(f: &Feast) -> [i32; 4] {
    let mut weight = f.rank.weight();
    if f.is_category(Category::Sunday) && weight < Rank::GreaterDouble.weight() {
        weight = Rank::GreaterDouble.weight();
    }
    // X.1(c): a primary feast before a secondary one of the same rite, ahead
    // of the Lord's personal dignity (X.1(d)): Ss Philip and James before the
    // Finding of the Holy Cross (2021 and 2024 ordos), St Mark after it (2022),
    // the Circumcision before the Holy Name (2017-2024 ordos); #557.
    [weight, i32::from(!f.secondary), i32::from(f.is_moveable()), i32::from(f.is_category(Category::Lord))]
}

fn is_corpus_octave_day(f: &Feast) -> bool {
    f.id.starts_with("corpus-christi-octave-day")
}

/// Whether challenger `a` wins over incumbent `b`.
pub fn compare_feast_precedence(a: &Feast, b: &Feast) -> bool {
    compare_feast_precedence_with_decision(a, b).0
}

pub fn compare_feast_precedence_with_decision(a: &Feast, b: &Feast) -> (bool, Decision) {
    let detail = format!("challenger={}; incumbent={}", a.id, b.id);
    let decision =
        |rule: &str, wins: bool| (wins, Decision::new(rule, if wins { "challenger-wins" } else { "incumbent-holds" }, detail.as_str()));

    // Privileged ferias of the second class take the office over every feast
    // below a Double of the second class. So an Advent Ember Day takes the
    // office from a Greater Double Octave Day, which is commemorated (Table of
    // Occurrence, rubrics p. 60: Common Octave Day and Ember Day, 4; XIII.16,
    // p. 64): 15 December 2021, 2027, 2032. The 2021 ordo keeps the Octave of
    // the Conception, commemorating the feria; awaiting a ruling (#638).
    let (a_pf, b_pf) = (is_privileged_feria(a), is_privileged_feria(b));
    if a_pf != b_pf {
        let other = if b_pf { a } else { b };
        let privileged_wins = other.rank.weight() < Rank::Double2ndClass.weight();
        return match (a_pf, privileged_wins) {
            (true, true) => decision("occurrence:privileged-feria-below-second-class", true),
            (true, false) => decision("occurrence:second-class-over-privileged-feria", false),
            (false, true) => decision("occurrence:privileged-feria-below-second-class", false),
            (false, false) => decision("occurrence:second-class-over-privileged-feria", true),
        };
    }

    let (a_corpus, b_corpus) = (is_corpus_octave_day(a), is_corpus_octave_day(b));
    if a_corpus != b_corpus {
        // Sundays and first-class feasts outrank Corpus octave days.
        if a_corpus {
            if b.is_category(Category::Sunday) || b.rank == Rank::Double1stClass {
                return decision("occurrence:sunday-or-first-class-over-corpus-octave", false);
            }
            return decision("occurrence:corpus-octave-precedence", true);
        }
        if a.is_category(Category::Sunday) || a.rank == Rank::Double1stClass {
            return decision("occurrence:sunday-or-first-class-over-corpus-octave", true);
        }
        return decision("occurrence:corpus-octave-precedence", false);
    }

    let (ak, bk) = (sort_key(a), sort_key(b));
    if ak[0] != bk[0] {
        let boosted = |f: &Feast| f.is_category(Category::Sunday) && f.rank.weight() < Rank::GreaterDouble.weight();
        let rule = if boosted(a) || boosted(b) { "occurrence:sunday-rank-boost" } else { "occurrence:higher-rank" };
        return decision(rule, ak[0] > bk[0]);
    }
    if ak[1] != bk[1] {
        return decision("occurrence:primary-tiebreak", ak[1] > bk[1]);
    }
    if ak[2] != bk[2] {
        return decision("occurrence:temporal-tiebreak", ak[2] > bk[2]);
    }
    if ak[3] != bk[3] {
        return decision("occurrence:lord-tiebreak", ak[3] > bk[3]);
    }
    decision("occurrence:equal-precedence-possession", false)
}

fn resolved_day_color(winner: Option<&Feast>, season: Season, season_color: Color) -> (Color, Decision) {
    let Some(w) = winner else {
        return (season_color, Decision::new("color:resolution", "seasonal-feria", season_color.as_str()));
    };
    // In Lent and Passiontide, lesser-rank sanctoral observances use the
    // seasonal color (not in Septuagesimatide: 2026 ordo, St Scholastica).
    if matches!(season, Season::Lent | Season::Passiontide) && w.rank.weight() < Rank::Double2ndClass.weight() {
        return (
            season_color,
            Decision::new("color:resolution", "penitential-season-over-lesser-feast", format!("{}={season_color}", w.id)),
        );
    }
    (w.color, Decision::new("color:resolution", "celebration-color", format!("{}={}", w.id, w.color)))
}

/// General Rubrics VI.2: a common vigil in Advent, Lent, or on an Ember Day
/// has neither office nor commemoration; nor, after Septuagesima, St
/// Matthias's (Diurnal p. 481), the only vigil that can fall there.
fn exclude_seasonal_vigils(candidates: Vec<FeastRef>, season: Season) -> (Vec<FeastRef>, Vec<Decision>) {
    let excluded = matches!(season, Season::Advent | Season::Septuagesima | Season::Lent | Season::Passiontide)
        || candidates.iter().any(|f| is_ember_day(f));
    if !excluded {
        return (candidates, Vec::new());
    }
    let mut filtered = Vec::with_capacity(candidates.len());
    let mut decisions = Vec::new();
    for c in candidates {
        if c.is_vigil && c.rank == Rank::Simple {
            decisions.push(Decision::new("commemoration:vigil-seasonal-exclusion", "suppressed", c.id.as_str()));
            continue;
        }
        filtered.push(c);
    }
    (filtered, decisions)
}

/// Whether a displaced feast is transferred rather than commemorated: II
/// Class Doubles and above, and All Souls from a Sunday.
fn should_transfer_out(f: &Feast, date: Date) -> bool {
    if f.id == "all-souls" && date.weekday() == Weekday::Sunday {
        return true;
    }
    f.rank.weight() >= Rank::Double2ndClass.weight() && !f.is_category(Category::Sunday)
}

fn transfer_out_outcome(f: &Feast, date: Date) -> &'static str {
    if f.id == "all-souls" && date.weekday() == Weekday::Sunday {
        return "all-souls-from-sunday";
    }
    "second-class-or-higher"
}

fn feast_ids(feasts: &[FeastRef]) -> String {
    feasts.iter().map(|f| f.id.as_str()).collect::<Vec<_>>().join(",")
}

fn day(date: Date, season: Season) -> CalendarDay {
    CalendarDay {
        date,
        season,
        tempora: None,
        celebration: None,
        commemorations: Vec::new(),
        color: season.color(),
        notes: None,
        resolution_rule: String::new(),
        occurrence_decisions: Vec::new(),
        feria_commemoration: None,
        temporal_week_id: None,
        within_octave_of: None,
        penitential: Penitential::default(),
        monastic: Vec::new(),
    }
}

/// Resolves one day's candidates (plus feasts transferred in). Returns the
/// day and the feasts it transfers out.
pub fn resolve_day(
    date: Date,
    candidates: &[FeastRef],
    season: Season,
    season_color: Color,
    transferred_in: &[FeastRef],
) -> (CalendarDay, Vec<FeastRef>) {
    // XI.7: a transferred feast goes to the next day free of an occurrent
    // Sunday, so on a Sunday it waits without competing (2021 ordo: the
    // Visitation passes over the Sunday within the Corpus Christi octave).
    let (held, transferred_in): (Vec<FeastRef>, Vec<FeastRef>) =
        transferred_in.iter().cloned().partition(|_| date.weekday() == Weekday::Sunday);
    let transferred_in = transferred_in.as_slice();
    let mut all: Vec<FeastRef> = candidates.iter().chain(transferred_in).cloned().collect();
    let mut transfers_out = held.clone();
    let mut decisions = vec![Decision::new(
        "occurrence:resolution-mode",
        "start",
        format!("candidates={}; transferred-in={}", candidates.len(), transferred_in.len()),
    )];
    let (filtered, vigil_decisions) = exclude_seasonal_vigils(all, season);
    all = filtered;
    decisions.extend(vigil_decisions);
    if !held.is_empty() {
        decisions.push(Decision::new("occurrence:transfer-in", "held-over-sunday", feast_ids(&held)));
    }
    if !transferred_in.is_empty() {
        decisions.push(Decision::new("occurrence:transfer-in", "considered", feast_ids(transferred_in)));
    }
    let mut result = day(date, season);
    result.color = season_color;

    if all.is_empty() {
        let (_, color_decision) = resolved_day_color(None, season, season_color);
        decisions.push(Decision::new("occurrence:resolution-mode", "no-candidates", ""));
        decisions.push(color_decision);
        result.resolution_rule = "occurrence:no-candidates".to_string();
        result.occurrence_decisions = decisions;
        return (result, transfers_out);
    }

    if all.iter().all(|f| f.rank == Rank::Commemoration) {
        let (comms, comm_decisions) = ordered_commemorations(None, &all, OrderContext { season: Some(season), ..OrderContext::default() });
        let (_, color_decision) = resolved_day_color(None, season, season_color);
        decisions.push(Decision::new("occurrence:resolution-mode", "commemorations-only", ""));
        decisions.extend(comm_decisions);
        decisions.push(color_decision);
        result.commemorations = comms;
        result.resolution_rule = "occurrence:commemorations-only".to_string();
        result.occurrence_decisions = decisions;
        return (result, transfers_out);
    }

    let privileged: Vec<FeastRef> = all.iter().filter(|f| is_privileged_feast(f)).cloned().collect();
    let is_privileged_mode = !privileged.is_empty();
    let pool = if is_privileged_mode {
        decisions.push(Decision::new("occurrence:resolution-mode", "privileged-fixed-day", feast_ids(&privileged)));
        &privileged
    } else {
        decisions.push(Decision::new("occurrence:resolution-mode", "general-precedence", ""));
        &all
    };
    let is_transferred = |f: &FeastRef| transferred_in.iter().any(|t| Arc::ptr_eq(t, f));
    let mut winner = pool[0].clone();
    for f in &pool[1..] {
        let (mut wins, mut decision) = compare_feast_precedence_with_decision(f, &winner);
        // XI.7: a transferred feast waits for a day free of a I or II Class
        // Double (2024 ordo: the Nativity of St John Baptist passes over the
        // Visitation). XI.8: of equal transferred feasts the one whose own day
        // comes first goes first (2019, 2021 and 2022 ordos: St Mark, then Ss
        // Philip and James, then the Finding of the Holy Cross).
        if wins && is_transferred(f) {
            let occupied = !is_transferred(&winner) && winner.rank.weight() >= Rank::Double2ndClass.weight();
            let earlier_equal = is_transferred(&winner) && sort_key(f)[0] == sort_key(&winner)[0];
            if occupied || earlier_equal {
                wins = false;
                let outcome = if occupied { "day-occupied" } else { "earlier-own-day" };
                decision = Decision::new("occurrence:transfer-order", outcome, format!("challenger={}; incumbent={}", f.id, winner.id));
            }
        }
        decisions.push(decision);
        if wins {
            winner = f.clone();
        }
    }

    let mut comms = Vec::new();
    for f in &all {
        if Arc::ptr_eq(f, &winner) {
            continue;
        }
        if is_privileged_mode && is_privileged_feast(f) {
            decisions.push(Decision::new("occurrence:other-privileged-day", "suppressed", f.id.as_str()));
            continue;
        }
        if should_transfer_out(f, date) {
            transfers_out.push(f.clone());
            decisions.push(Decision::new("occurrence:transfer-out", transfer_out_outcome(f, date), f.id.as_str()));
        } else if f.rank.weight() >= Rank::Commemoration.weight() {
            comms.push(f.clone());
            decisions.push(Decision::new("occurrence:loser-disposition", "commemorated", f.id.as_str()));
        }
    }
    let (comms, primary_decisions) = primary_feast_doubles(Some(&winner), comms);
    decisions.extend(primary_decisions);
    let (comms, comm_decisions) =
        ordered_commemorations(Some(&winner), &comms, OrderContext { season: Some(season), ..OrderContext::default() });
    decisions.extend(comm_decisions);
    let (color, color_decision) = resolved_day_color(Some(&winner), season, season_color);
    decisions.push(color_decision);

    result.celebration = Some(winner);
    result.commemorations = comms;
    result.color = color;
    result.resolution_rule = if is_privileged_mode { "occurrence:privileged-day" } else { "occurrence:general-precedence" }.to_string();
    result.occurrence_decisions = decisions;
    (result, transfers_out)
}
