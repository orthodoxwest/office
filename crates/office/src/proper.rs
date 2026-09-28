//! Proper text resolution: feast proper, Common, weekly temporal texts, seasonal default, weekday
//! ordinary, ordinary, shared.

use calendar::{Category, Feast, MoveableDates, Season};

use crate::day::Day;
use crate::texts::OfficeTexts;

/// Strips a trailing "-N" numeric suffix ("psalm-antiphon-2" → "psalm-antiphon").
pub fn base_proper_ref(reference: &str) -> &str {
    let b = reference.as_bytes();
    let mut i = b.len();
    while i > 0 && b[i - 1].is_ascii_digit() {
        i -= 1;
    }
    if i > 0 && i < b.len() && b[i - 1] == b'-' {
        return &reference[..i - 1];
    }
    reference
}

pub fn ref_candidates(reference: &str) -> Vec<String> {
    let base = base_proper_ref(reference);
    if base == reference { vec![reference.to_string()] } else { vec![reference.to_string(), base.to_string()] }
}

pub fn hour_ref_candidates(hour_name: &str, reference: &str) -> Vec<String> {
    ref_candidates(reference).into_iter().map(|c| format!("{c}-{hour_name}")).collect()
}

/// The proper IDs to try for a feast, in order.
pub fn feast_proper_ids(feast: &Feast) -> Vec<String> {
    if feast.id.is_empty() {
        return Vec::new();
    }
    match &feast.proper_id {
        Some(p) if *p != feast.id => {
            // Days within an octave try day-specific texts, then the set,
            // then the parent; other redirects win outright.
            if let Some(i) = feast.id.find("-octave-day")
                && i > 0
            {
                return vec![feast.id.clone(), p.clone(), feast.id[..i].to_string()];
            }
            vec![p.clone(), feast.id.clone()]
        }
        _ => vec![feast.id.clone()],
    }
}

pub fn feast_proper_ids_opt(feast: Option<&Feast>) -> Vec<String> {
    feast.map(feast_proper_ids).unwrap_or_default()
}

/// An engine-generated feria with no proper of its own.
pub fn is_synthesized_feria(feast: &Feast) -> bool {
    feast.id == "privileged-lenten-feria" || feast.id == calendar::model::FERIA_COMMEMORATION_ID
}

/// An ordinary numbered Sunday after Epiphany or Pentecost (resumed and
/// anticipated included), excluding those always within an octave.
pub fn is_per_annum_sunday(f: Option<&Feast>) -> bool {
    let Some(f) = f else { return false };
    if !f.is_category(Category::Sunday) {
        return false;
    }
    for id in feast_proper_ids(f) {
        if id == "epiphany-sunday-1" || id == "pentecost-sunday-2" {
            return false;
        }
        if id.starts_with("epiphany-sunday-") || id.starts_with("pentecost-sunday-") {
            return true;
        }
    }
    false
}

/// The first non-empty text among `prefix + reference`.
pub fn first_text<S: AsRef<str>>(t: &OfficeTexts, prefix: &str, refs: &[S]) -> (String, String) {
    for r in refs {
        let key = format!("{prefix}{}", r.as_ref());
        let text = t.get(&key);
        if !text.is_empty() {
            return (text.to_string(), key);
        }
    }
    (String::new(), String::new())
}

/// A weekday appointment within a season, above the season's shared text.
pub fn lookup_seasonal_text(day: &Day, hour_name: &str, reference: &str, t: &OfficeTexts) -> (String, String) {
    let prefix = format!("seasonal/{}/", day.season);
    let weekday = day.civil_weekday_name();
    for candidate in hour_ref_candidates(hour_name, reference) {
        let key = format!("{prefix}{candidate}-{weekday}");
        let text = t.get(&key);
        if !text.is_empty() {
            return (text.to_string(), key);
        }
    }
    lookup_section_text(&prefix, None, hour_name, reference, t)
}

pub(crate) fn season_ref_candidates(refs: &[String], season: Option<Season>) -> Vec<String> {
    match season {
        None => Vec::new(),
        Some(s) => refs.iter().map(|r| format!("{r}-{s}")).collect(),
    }
}

/// Resolves a section within one tier: season-qualified, then
/// hour-qualified, then generic.
pub fn lookup_section_text(prefix: &str, season: Option<Season>, hour_name: &str, reference: &str, t: &OfficeTexts) -> (String, String) {
    let hour_candidates = if hour_name.is_empty() { Vec::new() } else { hour_ref_candidates(hour_name, reference) };
    let ref_cands = ref_candidates(reference);
    for list in [season_ref_candidates(&hour_candidates, season), season_ref_candidates(&ref_cands, season), hour_candidates] {
        let found = first_text(t, prefix, &list);
        if !found.0.is_empty() {
            return found;
        }
    }
    first_text(t, prefix, &ref_cands)
}

/// Replaces the placeholder "N." with the saint's name.
pub fn substitute_proper_name(text: &str, name: &str) -> String {
    if name.is_empty() { text.to_string() } else { text.replace("N.", name) }
}

/// The name used for "N.": the explicit ProperName, else derived from the title.
pub fn feast_proper_name(feast: &Feast) -> String {
    match &feast.proper_name {
        Some(n) => n.clone(),
        None => derive_proper_name_from_title(&feast.name),
    }
}

/// "St Apollinaris of Ravenna, Bishop & Martyr" → "Apollinaris of Ravenna".
pub fn derive_proper_name_from_title(name: &str) -> String {
    let mut name = name.trim();
    if name.is_empty() {
        return String::new();
    }
    for prefix in ["Commemoration of ", "The Second Feast of ", "The Feast of ", "Vigil of "] {
        if let Some(rest) = name.strip_prefix(prefix) {
            name = rest;
            break;
        }
    }
    for prefix in ["Ss. ", "Ss ", "SS. ", "SS ", "St. ", "St ", "Saints ", "Saint "] {
        if let Some(rest) = name.strip_prefix(prefix) {
            name = rest;
            break;
        }
    }
    const MARKERS: [&str; 17] = [
        ", Bishop",
        ", Abbot",
        ", Priest",
        ", Virgin",
        ", Martyr",
        ", Confessor",
        ", Pope",
        ", Doctor",
        ", Apostle",
        ", Evangelist",
        ", Widow",
        ", King",
        ", Queen",
        ", Hermit",
        ", Deacon",
        ", Monk",
        ", Nun",
    ];
    if let Some(cut) = MARKERS.iter().filter_map(|m| name.find(m)).min() {
        name = &name[..cut];
    }
    name.trim().to_string()
}

/// The Common tier: paschal variant in Eastertide, then the regular Common.
pub fn lookup_commons_text(
    category: Option<Category>,
    season: Season,
    hour_name: &str,
    reference: &str,
    t: &OfficeTexts,
) -> (String, String) {
    let Some(category) = category else { return (String::new(), String::new()) };
    let cat = category.as_str();
    if season == Season::Easter {
        let found = lookup_section_text(&format!("commons/{cat}-paschal/"), None, hour_name, reference, t);
        if !found.0.is_empty() {
            return found;
        }
    }
    lookup_section_text(&format!("commons/{cat}/"), Some(season), hour_name, reference, t)
}

/// The collect of the Little Hours is the collect of Lauds.
pub fn resolve_proper_collect_text(day: &Day, hour_name: &str, t: &OfficeTexts) -> (String, String) {
    match hour_name {
        "terce" | "sext" | "none" => resolve_proper_text(day, "lauds", "collect", t),
        _ => resolve_proper_text(day, hour_name, "collect", t),
    }
}

fn proper_name_of(day: &Day) -> String {
    match day.celebration.as_deref() {
        // A vigil's collect may be its Common's "N." form (Diurnal p. 7*).
        Some(c) if c.is_vigil => feast_proper_name(c),
        Some(c) => c.proper_name.clone().unwrap_or_default(),
        None => String::new(),
    }
}

fn ferial_vespers_antiphon(day: &Day, hour_name: &str, reference: &str, t: &OfficeTexts) -> bool {
    hour_name == "vespers"
        && base_proper_ref(reference).starts_with("psalm-antiphon")
        && crate::psalmody::uses_weekday_vespers_antiphons(day, t)
}

/// Only the feast's own proper and its redirects.
pub fn lookup_feast_proper_text(day: &Day, hour_name: &str, reference: &str, t: &OfficeTexts) -> (String, String) {
    let hour_candidates = hour_ref_candidates(hour_name, reference);
    let ref_cands = ref_candidates(reference);
    let proper_name = proper_name_of(day);
    let Some(celebration) = day.celebration.as_deref() else { return (String::new(), String::new()) };
    if ferial_vespers_antiphon(day, hour_name, reference, t) || celebration.id.is_empty() || is_synthesized_feria(celebration) {
        return (String::new(), String::new());
    }
    for feast_id in feast_proper_ids(celebration) {
        if day.season == Season::Easter {
            let prefix = format!("proper/{feast_id}-paschal/");
            for list in [&hour_candidates, &ref_cands] {
                let (text, resolved) = first_text(t, &prefix, list);
                if !text.is_empty() {
                    return (substitute_proper_name(&text, &proper_name), resolved);
                }
            }
        }
        let (text, resolved) = lookup_section_text(&format!("proper/{feast_id}/"), Some(day.season), hour_name, reference, t);
        if !text.is_empty() {
            return (substitute_proper_name(&text, &proper_name), resolved);
        }
    }
    (String::new(), String::new())
}

fn is_sunday_or_feria(f: Option<&Feast>) -> bool {
    match f {
        None => true,
        Some(f) => f.is_category(Category::Sunday) || f.is_category(Category::Feria),
    }
}

/// Resolves a proper text through every tier; returns the text and the key
/// it came from, or a "[Proper text not found: …]" marker.
pub fn resolve_proper_text(day: &Day, hour_name: &str, reference: &str, t: &OfficeTexts) -> (String, String) {
    // At I Vespers prefer a "-first" variant within the proper office.
    if day.first_vespers && hour_name == "vespers" && !reference.ends_with("-first") {
        let (text, resolved) = resolve_proper_text(day, hour_name, &format!("{reference}-first"), t);
        if !text.starts_with("[Proper text not found") {
            // A Common's I Vespers text cannot displace the feast's own
            // antiphon shared by both Vespers.
            if resolved.starts_with("commons/") {
                let own = lookup_feast_proper_text(day, hour_name, reference, t);
                if !own.0.is_empty() {
                    return own;
                }
            }
            return (text, resolved);
        }
    }

    let hour_candidates = hour_ref_candidates(hour_name, reference);
    let ref_cands = ref_candidates(reference);
    let weekday = day.civil_weekday_name();
    let proper_name = proper_name_of(day);
    let ferial_vespers_antiphon = ferial_vespers_antiphon(day, hour_name, reference, t);
    let celebration = day.celebration.as_deref();

    // Dec 21 and 23: a fixed Benedictus antiphon replaces the Advent one.
    if hour_name == "lauds"
        && reference == "benedictus-antiphon"
        && is_sunday_or_feria(celebration)
        && let Some(key) = advent_date_benedictus_ref(day)
    {
        let text = t.get(&key);
        if !text.is_empty() {
            return (text.to_string(), key);
        }
    }

    // 0. The Greater ("O") Antiphons at Vespers of December 17–23.
    let greater_antiphon = || -> (String, String) {
        if hour_name != "vespers" || !reference.starts_with("magnificat-antiphon") {
            return (String::new(), String::new());
        }
        let Some(o_day) = greater_antiphon_day(day) else { return (String::new(), String::new()) };
        for cand in &ref_cands {
            let date_ref = format!("seasonal/advent/{cand}-december-{o_day}");
            let text = t.get(&date_ref);
            if !text.is_empty() {
                return (text.to_string(), date_ref);
            }
        }
        (String::new(), String::new())
    };
    if is_sunday_or_feria(celebration) {
        let (text, date_ref) = greater_antiphon();
        if !text.is_empty() {
            return (substitute_proper_name(&text, &proper_name), date_ref);
        }
    }

    // 0.7. Saturday evening before a per-annum Sunday: the historia, the
    // Sunday's own -first proper, then the ferial Saturday antiphon.
    if hour_name == "vespers"
        && day.first_vespers
        && reference.starts_with("magnificat-antiphon")
        && reference.ends_with("-first")
        && is_per_annum_sunday(celebration)
    {
        if day.is_sunday_first_vespers() {
            if let Some(id) = crate::seasonal::historia_week_id(day.date) {
                let key = format!("proper/historia-{id}/magnificat-antiphon-first");
                let text = t.get(&key);
                if !text.is_empty() {
                    return (text.to_string(), key);
                }
            }
            for feast_id in feast_proper_ids_opt(celebration) {
                let key = format!("proper/{feast_id}/magnificat-antiphon-first");
                let text = t.get(&key);
                if !text.is_empty() {
                    return (text.to_string(), key);
                }
            }
        }
        let ferial = format!("ordinary/vespers/magnificat-antiphon-{weekday}");
        let text = t.get(&ferial);
        if !text.is_empty() {
            return (text.to_string(), ferial);
        }
    }

    // 0.8. I Vespers of a Sunday takes the Saturday psalter.
    if hour_name == "vespers"
        && day.is_sunday_first_vespers()
        && let Some(c) = celebration
        && reference.starts_with("psalm-antiphon")
        && !reference.ends_with("-first")
    {
        for feast_id in feast_proper_ids(c) {
            if day.season == Season::Easter {
                let (text, resolved) = first_text(t, &format!("proper/{feast_id}-paschal/"), &hour_candidates);
                if !text.is_empty() {
                    return (substitute_proper_name(&text, &proper_name), resolved);
                }
            }
            let (text, resolved) = first_text(t, &format!("proper/{feast_id}/"), &hour_candidates);
            if !text.is_empty() {
                return (substitute_proper_name(&text, &proper_name), resolved);
            }
        }
        if day.season == Season::Easter {
            let prefix = format!("seasonal/{}/", day.season);
            for list in [&hour_candidates, &ref_cands] {
                let found = first_text(t, &prefix, list);
                if !found.0.is_empty() {
                    return found;
                }
            }
        }
        for cand in &ref_cands {
            let saturday_ref = format!("ordinary/vespers/{cand}-saturday");
            let text = t.get(&saturday_ref);
            if !text.is_empty() {
                return (text.to_string(), saturday_ref);
            }
        }
    }

    // 1. Feast-specific proper.
    let own = lookup_feast_proper_text(day, hour_name, reference, t);
    if !own.0.is_empty() {
        return own;
    }

    // 1.5. The O antiphon outranks a feast's Commons antiphon.
    let (text, date_ref) = greater_antiphon();
    if !text.is_empty() {
        return (substitute_proper_name(&text, &proper_name), date_ref);
    }

    // 2. Common of Saints.
    if !ferial_vespers_antiphon
        && let Some(c) = celebration
        && !is_synthesized_feria(c)
    {
        let (text, resolved) = lookup_commons_text(c.category, day.season, hour_name, reference, t);
        if !text.is_empty() {
            return (substitute_proper_name(&text, &proper_name), resolved);
        }
    }

    // 2.5. Weekly temporal texts from the governing Sunday's proper.
    if let Some(week) = &day.temporal_week_id
        && weekday != "sunday"
    {
        let prefix = format!("proper/{week}/");
        let wd_refs: Vec<String> = ref_cands.iter().map(|c| format!("{c}-{weekday}")).collect();
        let (text, resolved) = first_text(t, &prefix, &wd_refs);
        if !text.is_empty() {
            return (substitute_proper_name(&text, &proper_name), resolved);
        }
        // A feria's collect is the Sunday collect.
        if base_proper_ref(reference) == "collect" && (hour_name == "lauds" || hour_name == "vespers") {
            let (text, resolved) = first_text(t, &prefix, &ref_cands);
            if !text.is_empty() {
                return (substitute_proper_name(&text, &proper_name), resolved);
            }
        }
    }

    // 3. Seasonal default. Seasonal "-first" entries model the Saturday
    // books before Sundays only.
    let seasonal_sunday_first = reference.ends_with("-first") && hour_name == "vespers";
    if (!seasonal_sunday_first || day.is_sunday_first_vespers()) && seasonal_appointment_applies(day, hour_name, reference, t) {
        let (text, resolved) = lookup_seasonal_text(day, hour_name, reference, t);
        if !text.is_empty() {
            return (substitute_proper_name(&text, &proper_name), resolved);
        }
    }

    // 4. Weekday ordinary; the Sunday Lauds hymn varies with summer.
    for cand in &ref_cands {
        let mut wd = weekday.clone();
        if hour_name == "lauds" && cand == "hymn" && weekday == "sunday" && crate::hymn::sunday_lauds_hymn_is_summer(day.date) {
            wd = "sunday-summer".to_string();
        }
        let weekday_ref = format!("ordinary/{hour_name}/{cand}-{wd}");
        let text = t.get(&weekday_ref);
        if !text.is_empty() {
            return (substitute_proper_name(text, &proper_name), weekday_ref);
        }
    }
    // 5. Ordinary.
    for cand in &ref_cands {
        let ordinary_ref = format!("ordinary/{hour_name}/{cand}");
        let text = t.get(&ordinary_ref);
        if !text.is_empty() {
            return (substitute_proper_name(text, &proper_name), ordinary_ref);
        }
    }
    // 6. Shared ordinary.
    for cand in &ref_cands {
        let shared_ref = format!("ordinary/shared/{cand}");
        let text = t.get(&shared_ref);
        if !text.is_empty() {
            return (substitute_proper_name(text, &proper_name), shared_ref);
        }
    }
    (format!("[Proper text not found: {reference}]"), reference.to_string())
}

/// The December date whose O Antiphon belongs to this evening, if Dec 17–23.
/// At I Vespers the office day carries tomorrow's date, so step back.
pub fn greater_antiphon_day(day: &Day) -> Option<u32> {
    if day.season != Season::Advent || day.date.month() != 12 {
        return None;
    }
    let mut o_day = day.date.day();
    if day.first_vespers {
        o_day -= 1;
    }
    (17..=23).contains(&o_day).then_some(o_day)
}

/// The date-fixed Benedictus antiphon of December 21 or 23.
pub fn advent_date_benedictus_ref(day: &Day) -> Option<String> {
    if day.season != Season::Advent || day.date.month() != 12 {
        return None;
    }
    let d = day.date.day();
    (d == 21 || d == 23).then(|| format!("seasonal/advent/benedictus-antiphon-december-{d}"))
}

/// The date-fixed Advent antiphon a de Tempore commemoration takes.
pub fn advent_date_commemoration_antiphon(day: &Day, comm: &Feast, hour_name: &str, reference: &str, t: &OfficeTexts) -> (String, String) {
    if reference != "commemoration-antiphon" || !(comm.is_category(Category::Sunday) || comm.is_category(Category::Feria)) {
        return (String::new(), String::new());
    }
    let key = match hour_name {
        "vespers" => match greater_antiphon_day(day) {
            Some(o) => format!("seasonal/advent/magnificat-antiphon-december-{o}"),
            None => return (String::new(), String::new()),
        },
        "lauds" => match advent_date_benedictus_ref(day) {
            Some(k) => k,
            None => return (String::new(), String::new()),
        },
        _ => return (String::new(), String::new()),
    };
    let text = t.get(&key);
    if text.is_empty() { (String::new(), String::new()) } else { (text.to_string(), key) }
}

/// Whether an appointment scope admits the seasonal fallback tier.
pub fn seasonal_appointment_applies(day: &Day, hour_name: &str, reference: &str, t: &OfficeTexts) -> bool {
    let Some(scope) = t.seasonal_appointment_scope(day.season, hour_name, base_proper_ref(reference)) else { return true };
    let easter = MoveableDates::compute(day.date.year()).easter;
    scope.allows(day.date, easter, day.civil_weekday(), day.is_ferial())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn base_refs() {
        assert_eq!(base_proper_ref("psalm-antiphon-2"), "psalm-antiphon");
        assert_eq!(base_proper_ref("psalm-antiphon"), "psalm-antiphon");
        assert_eq!(base_proper_ref("x-12"), "x");
        assert_eq!(base_proper_ref("12"), "12");
        assert_eq!(base_proper_ref("-3"), "");
        assert_eq!(ref_candidates("collect"), ["collect"]);
        assert_eq!(hour_ref_candidates("lauds", "psalm-antiphon-1"), ["psalm-antiphon-1-lauds", "psalm-antiphon-lauds"]);
    }

    #[test]
    fn proper_names() {
        for (title, want) in [
            ("St Apollinaris of Ravenna, Bishop & Martyr", "Apollinaris of Ravenna"),
            ("Ss. Marius, Martha, Audifax, & Abachum, Martyrs", "Marius, Martha, Audifax, & Abachum"),
            ("Commemoration of St Paul, Apostle", "Paul"),
            ("The Feast of the Holy Name", "the Holy Name"),
            ("", ""),
        ] {
            assert_eq!(derive_proper_name_from_title(title), want, "{title}");
        }
    }
}
