//! The Office engine: loads the corpus and hour definitions, dispatches to the hour composers, and
//! applies the passes every hour shares.

use std::collections::{HashMap, HashSet};

use calendar::traits::octave_parent_id;
use calendar::{Category, DataSource, Decision, Feast, MoveableDates, Season};
use liturgy::{ElementType, OfficeElement, OfficeHour, PrayerForm, VoiceSpan};

use crate::conclusion::apply_conclusion;
use crate::concurrence::VespersOwner;
use crate::day::Day;
use crate::hourdef::{HourElement, HourSection, parse_hour_definition, uses_triduum_form};
use crate::proper::{resolve_proper_collect_text, resolve_proper_text};
use crate::psalmody::{DOXOLOGY_REF_PER_OFFICE, VESPERS_OF_THE_DEAD_LABEL, psalm_doxology_ref, says_psalm_doxology};
use crate::texts::{OfficeTexts, load_texts};
use crate::{compline, lauds_psalmody, major, minor, preces, prime, rubric, vespers, voice};

/// The seven hours, in canonical order.
pub const HOUR_NAMES: [&str; 7] = ["lauds", "prime", "terce", "sext", "none", "vespers", "compline"];
pub const HOLY_SATURDAY_VESPERS_DEFINITION: &str = "vespers-holy-saturday";

/// Every hour definition file, including exceptional forms.
pub fn hour_definition_names() -> Vec<&'static str> {
    HOUR_NAMES.iter().copied().chain([HOLY_SATURDAY_VESPERS_DEFINITION]).collect()
}

/// Per-composition choices. The default is private prayer without the
/// optional Martyrology at Prime.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct ComposeOptions {
    pub form: PrayerForm,
    /// Read the next day's Martyrology at Prime (a reader's setting).
    pub martyrology: bool,
}

impl Default for ComposeOptions {
    fn default() -> ComposeOptions {
        ComposeOptions { form: PrayerForm::Private, martyrology: false }
    }
}

/// The loaded corpus and hour definitions, immutable and shareable.
pub struct Engine {
    pub texts: OfficeTexts,
    definitions: HashMap<String, Vec<HourSection>>,
}

impl Engine {
    /// Loads everything under the data directory.
    pub fn load(src: &dyn DataSource) -> Result<Engine, String> {
        let texts = load_texts(src).map_err(|e| format!("loading text corpus: {e}"))?;
        if texts.scopes.is_none() {
            return Err(format!("missing required appointment scopes: {}", src.display_path("appointment-scopes.json")));
        }
        let mut definitions = HashMap::new();
        for name in hour_definition_names() {
            let rel = format!("office/{name}.txt");
            let path = src.display_path(&rel);
            let content = src.read(&rel)?.ok_or_else(|| format!("parsing {name} definition: {path} does not exist"))?;
            let sections = parse_hour_definition(&path, &content).map_err(|e| format!("parsing {name} definition: {e}"))?;
            definitions.insert(name.to_string(), sections);
        }
        Ok(Engine { texts, definitions })
    }

    /// Composes the named hour for the given day in the given prayer form.
    pub fn compose_hour(&self, hour_name: &str, day: &Day, moveable: &MoveableDates, form: PrayerForm) -> Result<OfficeHour, String> {
        self.compose_hour_with_options(hour_name, day, moveable, &ComposeOptions { form, martyrology: false })
    }

    /// Composes with the reader's choices. They stay local to this
    /// composition; the engine is unchanged.
    pub fn compose_hour_with_options(
        &self,
        hour_name: &str,
        day: &Day,
        moveable: &MoveableDates,
        options: &ComposeOptions,
    ) -> Result<OfficeHour, String> {
        let form = options.form;
        if !HOUR_NAMES.contains(&hour_name) {
            return Err(format!("unknown hour: {hour_name}"));
        }
        let definition =
            if hour_name == "vespers" && vespers::is_holy_saturday_vespers(day) { HOLY_SATURDAY_VESPERS_DEFINITION } else { hour_name };
        let sections = &self.definitions[definition];
        let t = &self.texts;
        let composed = match hour_name {
            "lauds" => major::compose_major_hour(day, sections, t, Some(moveable), &major::MajorHourOptions::lauds()),
            "vespers" => vespers::compose_vespers(day, sections, t, Some(moveable)),
            "prime" => Ok(prime::compose_prime(day, sections, t, Some(moveable), options.martyrology)),
            "compline" => Ok(compline::compose_compline(day, sections, t, Some(moveable))),
            "terce" => Ok(minor::compose_minor_hour("Terce", day, sections, t, Some(moveable))),
            "sext" => Ok(minor::compose_minor_hour("Sext", day, sections, t, Some(moveable))),
            _ => Ok(minor::compose_minor_hour("None", day, sections, t, Some(moveable))),
        };
        let mut hour = composed.map_err(|e| format!("composing {hour_name}: {e}"))?;
        if definition != hour_name {
            hour.decisions.push(Decision::new(
                "context:office-form",
                "holy-saturday-vigil",
                "Diurnal pp. 360–361, Vespers apart from Mass",
            ));
        }
        crate::leader::apply_leader(&mut hour, form, t)?;
        drop_empty_sections(&mut hour);
        canonicalize_source_refs(&mut hour, t);
        collapse_uniform_antiphons(&mut hour);
        mark_psalm_doxologies(&mut hour);
        crate::posture::mark_postures(&mut hour, day, hour_name);
        mark_announced_antiphons(&mut hour, day, hour_name);
        crate::unrepeated::mark_unrepeated_openings(&mut hour);
        append_context_decisions(&mut hour, day, hour_name, moveable);
        // A missing corpus entry is a composition failure, never rendered text:
        // the 28-year golden digest then fails before a regression ships.
        if let Some(marker) = hour.sections.iter().flat_map(|s| &s.elements).find_map(|e| unresolved_marker(&e.text)) {
            return Err(format!("composing {hour_name} for {}: unresolved text {marker}", day.date));
        }
        Ok(hour)
    }
}

/// Review dependencies follow corpus aliases.
fn canonicalize_source_refs(hour: &mut OfficeHour, t: &OfficeTexts) {
    for section in &mut hour.sections {
        for elem in &mut section.elements {
            let original = if elem.source_refs.is_empty() && !elem.source_ref.is_empty() {
                vec![elem.source_ref.clone()]
            } else {
                elem.source_refs.clone()
            };
            let refs: Vec<String> = original.iter().map(|r| t.canonical_ref(r).unwrap_or(r).to_string()).collect();
            elem.source_refs = compact_refs(refs);
        }
    }
}

fn weekday_lower(day: &Day) -> String {
    day.date.weekday().name().to_lowercase()
}

fn append_context_decisions(hour: &mut OfficeHour, day: &Day, hour_name: &str, moveable: &MoveableDates) {
    let d = &mut hour.decisions;
    let add = |d: &mut Vec<Decision>, rule: &str, outcome: &str, detail: &str| d.push(Decision::new(rule, outcome, detail));
    add(d, "context:season", day.season.as_str(), "");
    add(d, "context:weekday", &weekday_lower(day), "");
    add(d, "occurrence", &day.resolution_rule, "");
    d.extend(day.occurrence_decisions.iter().cloned());
    match day.celebration.as_deref() {
        None => add(d, "context:office", "feria", ""),
        Some(c) => {
            add(d, "context:office", "celebration", &c.id);
            add(d, "context:rank", c.rank.as_str(), "");
            add(d, "context:category", c.category.map_or("", |c| c.as_str()), "");
        }
    }
    add(d, "context:commemorations", &day.commemorations.len().to_string(), "");
    match &day.within_octave_of {
        Some(o) => add(d, "context:octave", "within", o),
        None => add(d, "context:octave", "outside", ""),
    }
    if let Some(f) = &day.feria_commemoration {
        add(d, "context:feria-commemoration", "present", f.proper_id.as_deref().unwrap_or(""));
    }
    match hour_name {
        "prime" => add(d, "preces", preces::preces_disposition(Some(day), Some(moveable)).1, ""),
        "compline" => add(d, "preces", preces::preces_disposition(Some(&compline::compline_office_day(day)), Some(moveable)).1, ""),
        _ => {}
    }
    match hour_name {
        "vespers" => {
            let owner = match day.vespers.owner {
                VespersOwner::NotApplicable => "not-applicable",
                VespersOwner::IIOfPreceding => "second-of-preceding",
                VespersOwner::IOfFollowing => "first-of-following",
            };
            add(d, "vespers:owner", owner, "");
            add(d, "vespers:rule", &day.vespers.rule, "");
            d.extend(day.vespers.decisions.iter().cloned());
            let office_day = vespers::vespers_office_day(day);
            append_evening_office_context_decisions(d, &office_day);
            add(d, "suffrage", preces::suffrage_disposition(Some(&office_day)).1, "");
            add_marian_decisions(d, &office_day, hour_name);
        }
        "lauds" => {
            add(d, "suffrage", preces::suffrage_disposition(Some(day)).1, "");
            add_marian_decisions(d, day, hour_name);
        }
        "compline" => {
            let office_day = compline::compline_office_day(day);
            append_evening_office_context_decisions(d, &office_day);
            add_marian_decisions(d, &office_day, hour_name);
        }
        _ => {}
    }
    let outcome = if antiphons_doubled(day, hour_name) { "doubled" } else { "announced" };
    add(d, "antiphon:doubling", outcome, "");
}

pub const MARIAN_BOUNDARY_CIVIL_DAY: &str = "civil-day";
pub const MARIAN_BOUNDARY_PURIFICATION_VESPERS_OVERRIDE: &str = "purification-vespers-override";

fn add_marian_decisions(d: &mut Vec<Decision>, day: &Day, hour_name: &str) {
    let (key, boundary) = marian_antiphon_selection(day, hour_name);
    d.push(Decision::new("marian:selection", key, ""));
    d.push(Decision::new("marian:boundary", boundary, ""));
}

/// Describes the synthetic office day that drove Vespers or Compline.
fn append_evening_office_context_decisions(d: &mut Vec<Decision>, office_day: &Day) {
    let add = |d: &mut Vec<Decision>, rule: &str, outcome: &str, detail: &str| d.push(Decision::new(rule, outcome, detail));
    add(d, "office-context:season", office_day.season.as_str(), "");
    add(d, "office-context:weekday", &weekday_lower(office_day), "");
    match office_day.celebration.as_deref() {
        None => add(d, "office-context:office", "feria", ""),
        Some(c) => {
            add(d, "office-context:office", "celebration", &c.id);
            add(d, "office-context:rank", c.rank.as_str(), "");
            add(d, "office-context:category", c.category.map_or("", |c| c.as_str()), "");
        }
    }
    add(d, "office-context:commemorations", &office_day.commemorations.len().to_string(), "");
    match &office_day.within_octave_of {
        Some(o) => add(d, "office-context:octave", "within", o),
        None => add(d, "office-context:octave", "outside", ""),
    }
    add(d, "office-context:first-vespers", if office_day.first_vespers { "yes" } else { "no" }, "");
}

/// Psalm groups "under one antiphon": within a run of psalm-bearing
/// sections, three or more consecutive equal antiphons keep only the first
/// and the last.
fn collapse_uniform_antiphons(hour: &mut OfficeHour) {
    let has_psalmody = |s: &liturgy::OfficeSection| s.elements.iter().any(|e| e.kind.is_psalmody());
    let mut start = 0;
    while start < hour.sections.len() {
        if !has_psalmody(&hour.sections[start]) {
            start += 1;
            continue;
        }
        let mut end = start;
        while end + 1 < hour.sections.len() && has_psalmody(&hour.sections[end + 1]) {
            end += 1;
        }
        let ants: Vec<(usize, usize)> = (start..=end)
            .flat_map(|si| {
                hour.sections[si].elements.iter().enumerate().filter(|(_, e)| e.kind == ElementType::Antiphon).map(move |(i, _)| (si, i))
            })
            .collect();
        let text = |p: (usize, usize)| &hour.sections[p.0].elements[p.1].text;
        let mut drop: HashSet<(usize, usize)> = HashSet::new();
        let mut lo = 0;
        while lo < ants.len() {
            let mut hi = lo;
            while hi + 1 < ants.len() && text(ants[hi + 1]) == text(ants[lo]) {
                hi += 1;
            }
            if hi - lo + 1 >= 3 {
                drop.extend(ants[lo + 1..hi].iter().copied());
            }
            lo = hi + 1;
        }
        if !drop.is_empty() {
            for si in start..=end {
                let elems = std::mem::take(&mut hour.sections[si].elements);
                hour.sections[si].elements =
                    elems.into_iter().enumerate().filter(|(i, _)| !drop.contains(&(si, *i))).map(|(_, e)| e).collect();
            }
        }
        start = end + 1;
    }
}

/// Whether psalm and canticle antiphons are doubled: only Lauds and Vespers
/// of a Double office (General Rubrics I.4 / XXIV.8).
pub fn antiphons_doubled(day: &Day, hour_name: &str) -> bool {
    if hour_name != "lauds" && hour_name != "vespers" {
        return false;
    }
    let office_day = if hour_name == "vespers" { vespers::vespers_office_day(day) } else { day.clone() };
    office_day.celebration.as_deref().is_some_and(|c| c.rank.is_double())
}

/// Flags the opening antiphon of each psalm when antiphons are not doubled.
/// Vespers of the Dead is said as a Double.
fn mark_announced_antiphons(hour: &mut OfficeHour, day: &Day, hour_name: &str) {
    let doubled = antiphons_doubled(day, hour_name);
    let mut in_dead = false;
    for section in &mut hour.sections {
        if section.label == VESPERS_OF_THE_DEAD_LABEL {
            in_dead = true;
        }
        if doubled || in_dead {
            continue;
        }
        let n = section.elements.len();
        for i in 0..n {
            if section.elements[i].kind == ElementType::Antiphon && i + 1 < n && section.elements[i + 1].kind.is_psalmody() {
                section.elements[i].announce = true;
            }
        }
    }
}

/// A doxology directly after a psalm or canticle is a psalm doxology.
fn mark_psalm_doxologies(hour: &mut OfficeHour) {
    for section in &mut hour.sections {
        for i in 1..section.elements.len() {
            if section.elements[i].kind == ElementType::Doxology && section.elements[i - 1].kind.is_psalmody() {
                section.elements[i].kind = ElementType::PsalmDoxology;
            }
        }
    }
}

/// Resolves a plain element by looking up its text.
pub fn resolve_element(elem: &HourElement, t: &OfficeTexts) -> OfficeElement {
    let mut text = t.get(&elem.reference).to_string();
    if text.is_empty() {
        text = format!("[Text not found: {}]", elem.reference);
    }
    // Only the legacy partly-secret Pater omits its final Amen.
    if elem.kind == "partly-secret-prayer"
        && elem.reference.ends_with("/our-father")
        && let Some(stripped) = text.strip_suffix(" Amen.")
    {
        text = stripped.to_string();
    }
    let kind = map_element_type(&elem.kind);
    let label = format_label(&elem.kind, &elem.reference);
    let with_source = |mut e: OfficeElement| {
        e.source_ref = elem.reference.clone();
        e.source_refs = vec![elem.reference.clone()];
        e
    };
    if kind == ElementType::Chapter {
        let (r, body) = extract_chapter_ref(&text);
        let mut e = OfficeElement::new(ElementType::Chapter, body);
        e.label = r;
        return with_source(e);
    }
    if kind == ElementType::Preces {
        let mut e = OfficeElement::new(ElementType::Preces, text);
        e.label = "Preces".to_string();
        return with_source(e);
    }
    let mut oe = OfficeElement::new(kind, text);
    oe.label = label;
    oe = with_source(oe);
    if kind.is_psalmody() {
        oe.incipit = t.incipit(&elem.reference).unwrap_or("").to_string();
    }
    match elem.kind.as_str() {
        "officiant-greeting" => oe.leader_slot = "greeting".to_string(),
        "officiant-confession" => oe.leader_slot = "confession".to_string(),
        "officiant-opening" => oe.leader_slot = "opening".to_string(),
        "secret-prayer" => oe.voice = voice::build_prayer_voice(&elem.reference, &oe.text, false),
        // The Triduum's concluding Our Father is entirely silent (p. 313).
        "silent-prayer" => oe.voice = vec![VoiceSpan::new(oe.text.clone(), false, None)],
        "partly-secret-prayer" => oe.voice = voice::build_prayer_voice(&elem.reference, &oe.text, true),
        "corporate-lord-prayer" => oe.voice = voice::build_corporate_lord_prayer_voice(&elem.reference, &oe.text),
        "rubric" => oe.rubric_spans = rubric::build_rubric_spans(&elem.reference, &oe.text),
        _ => {}
    }
    oe
}

/// The seasonal Marian antiphon slug and its boundary branch. Alma
/// Redemptoris continues through II Vespers of the Purification.
pub fn marian_antiphon_selection<'a>(day: &'a Day, hour_name: &str) -> (&'a str, &'static str) {
    if hour_name == "vespers" && day.date.month() == 2 && day.date.day() == 2 {
        return ("alma-redemptoris-christmas", MARIAN_BOUNDARY_PURIFICATION_VESPERS_OVERRIDE);
    }
    (&day.marian_antiphon, MARIAN_BOUNDARY_CIVIL_DAY)
}

fn resolve_marian_element(day: &Day, hour_name: &str, t: &OfficeTexts) -> OfficeElement {
    let (key, _) = marian_antiphon_selection(day, hour_name);
    let r = format!("ordinary/marian/{key}");
    let mut text = t.get(&r).to_string();
    if text.is_empty() {
        text = format!("[Text not found: {r}]");
    }
    let mut oe = OfficeElement::new(ElementType::Antiphon, text);
    oe.label = compline::marian_label(key).to_string();
    oe.slot_ref = "marian-antiphon".to_string();
    oe.source_ref = r.clone();
    oe.source_refs = vec![r];
    oe
}

/// Resolves and appends an element unless the corpus omits it.
pub fn append_hour_element(elems: &mut Vec<OfficeElement>, day: &Day, hour_name: &str, elem: &HourElement, t: &OfficeTexts) {
    append_resolved(elems, resolve_hour_element(day, hour_name, elem, t));
}

/// Appends an already-resolved element unless it is an omission.
pub fn append_resolved(elems: &mut Vec<OfficeElement>, oe: OfficeElement) {
    if !corpus::is_omitted(&oe.text) {
        elems.push(oe);
    }
}

fn drop_empty_sections(hour: &mut OfficeHour) {
    hour.sections.retain(|s| !s.elements.is_empty());
}

fn element(kind: ElementType, text: String, slot_ref: &str, src: &str) -> OfficeElement {
    let mut e = OfficeElement::new(kind, text);
    e.slot_ref = slot_ref.to_string();
    e.source_ref = src.to_string();
    e.source_refs = compact_refs(vec![src.to_string()]);
    e
}

/// Resolves an hour element, with proper resolution for the `proper-*` types.
/// An omitted element resolves with the omission marker as its text.
pub fn resolve_hour_element(day: &Day, hour_name: &str, elem: &HourElement, t: &OfficeTexts) -> OfficeElement {
    let r = elem.reference.as_str();
    match elem.kind.as_str() {
        "marian" => {
            if r == "seasonal" {
                resolve_marian_element(day, hour_name, t)
            } else {
                resolve_element(elem, t)
            }
        }
        "gloria-patri" => {
            if !says_psalm_doxology(day, hour_name) {
                let mut e = OfficeElement::new(ElementType::Doxology, corpus::OMIT_MARKER);
                e.slot_ref = r.to_string();
                return e;
            }
            if r == DOXOLOGY_REF_PER_OFFICE {
                return resolve_element(&HourElement::new(&elem.kind, psalm_doxology_ref(day)), t);
            }
            resolve_element(elem, t)
        }
        "proper-antiphon" => {
            // Diurnal pp. 652–654: the weekday psalms of Prime and the
            // Little Hours of the Dead are said without antiphons.
            if crate::psalmody::is_office_of_the_dead(day) && matches!(hour_name, "prime" | "terce" | "sext" | "none") {
                return OfficeElement::new(ElementType::Antiphon, corpus::OMIT_MARKER);
            }
            let (mut text, mut src) = resolve_proper_text(day, hour_name, r, t);
            if hour_name == "lauds" && r.starts_with("psalm-antiphon-") && lauds_psalmody::uses_weekday_lauds_psalmody(day, t) {
                (text, src) = lauds_psalmody::weekday_lauds_antiphon(day, r, t);
                if text.is_empty() {
                    text = format!("[Weekday Lauds antiphon not found: {src}]");
                }
            }
            element(ElementType::Antiphon, text, r, &src)
        }
        "proper-opening-acclamation" => {
            // Diurnal p. 235: Alleluia is said for the last time at Vespers
            // of the Saturday before Septuagesima, and Compline begins the
            // season's "Praise be to thee", even when the Saturday's own
            // feast (St Matthias, 2035) keeps the day in the season before.
            let eve_key = match hour_name {
                _ if r != "alleluia" || !day.is_septuagesima_eve() => None,
                "vespers" => Some("ordinary/shared/alleluia"),
                "compline" => Some("seasonal/septuagesima/alleluia"),
                _ => None,
            };
            let (text, src) = match eve_key {
                Some(key) => (t.get(key).to_string(), key.to_string()),
                None => resolve_proper_text(day, hour_name, r, t),
            };
            element(ElementType::OpeningAcclamation, text, r, &src)
        }
        "proper-collect" => {
            let (text, src) = resolve_proper_collect_text(day, hour_name, t);
            let body = text.trim_end_matches('\n').to_string();
            // The collect of the day is always concluded (XXXIII.5).
            let (text, refs) = apply_conclusion(&text, &src, t);
            let mut e = element(ElementType::Collect, text.clone(), "collect", &src);
            e.source_refs = compact_refs(refs.clone());
            if uses_triduum_form(day, hour_name) && refs.len() == 2 {
                // The body is said in a low voice; the conclusion is silent
                // with no aloud response (Diurnal p. 313).
                let conclusion = text.strip_prefix(&format!("{body}\n")).unwrap_or(&text).replace("\nR. Amen.", "\nAmen.");
                e.text = format!("{body}\n{conclusion}");
                e.voice = vec![VoiceSpan::new(format!("{body}\n"), true, None), VoiceSpan::new(conclusion, false, None)];
            }
            e
        }
        "proper-hymn" => {
            let (mut text, src) = resolve_proper_text(day, hour_name, r, t);
            if corpus::is_omitted(&text) {
                return element(ElementType::Hymn, text, r, &src);
            }
            let mut refs = vec![src.clone()];
            // A proper `hymn-doxology` entry is the feast's own ending for
            // hymns of the metre (Our Lady of Sorrows, Diurnal pp. 505, 602);
            // @omit marks a hymn whose ending "is never changed".
            let (own_dox, own_ref) = resolve_proper_text(day, hour_name, "hymn-doxology", t);
            let (dox, dox_ref) = match own_ref.starts_with("proper/") {
                true => (own_dox, own_ref),
                false => hymn_doxology(day, hour_name, t),
            };
            let found = dox_ref.starts_with("seasonal/") || dox_ref.starts_with("proper/");
            if found && !corpus::is_omitted(&dox) && has_common_metre_ending(&text) {
                text = substitute_hymn_doxology(&text, &dox);
                refs.push(dox_ref);
            }
            let (title, body) = corpus::lines::split_hymn_title(&text);
            let mut e = OfficeElement::new(ElementType::Hymn, body);
            e.label = title.to_string();
            e.slot_ref = r.to_string();
            e.source_ref = src;
            e.source_refs = compact_refs(refs);
            e
        }
        "proper-responsory" => {
            let (text, src) = resolve_proper_text(day, hour_name, r, t);
            element(ElementType::Response, text, r, &src)
        }
        "proper-short-responsory" => {
            let (text, src) = resolve_proper_text(day, hour_name, r, t);
            element(ElementType::ShortResponsory, text, r, &src)
        }
        "proper-versicle" => {
            let (text, src) = resolve_proper_text(day, hour_name, r, t);
            element(ElementType::Versicle, text, r, &src)
        }
        "proper-chapter" => {
            let (text, src) = resolve_proper_text(day, hour_name, r, t);
            let (label, body) = extract_chapter_ref(&text);
            let mut e = element(ElementType::Chapter, body, r, &src);
            e.label = label;
            e
        }
        "versicle" if r == BENEDICAMUS_REF && says_paschal_benedicamus(day, hour_name) => {
            resolve_element(&HourElement::new(&elem.kind, BENEDICAMUS_ALLELUIA_REF), t)
        }
        _ => resolve_element(elem, t),
    }
}

const BENEDICAMUS_REF: &str = "shared/leader/benedicamus-domino";
const BENEDICAMUS_ALLELUIA_REF: &str = "shared/formulas/benedicamus-domino-alleluia";

/// Diurnal p. 365: "Let us bless the Lord, alleluia, alleluia" at Lauds and
/// Vespers from Easter Day "only through Lauds of Saturday before Low
/// Sunday"; that Saturday's Vespers are I Vespers of Low Sunday (2026 ordo
/// p. 18).
fn says_paschal_benedicamus(day: &Day, hour_name: &str) -> bool {
    let easter = MoveableDates::compute(day.date.year()).easter;
    matches!(hour_name, "lauds" | "vespers") && day.date >= easter && day.date < easter.add_days(7)
}

/// The seasonal hymn ending (Diurnal p. 3). Eastertide's "To thee who, dead,
/// again dost live" ends "all Hymns of the same metre through None of the
/// Vigil of the Ascension, except those which have a proper Ending" (p. 364),
/// then the Ascensiontide ending until Pentecost, and the Pentecost ending
/// through its Octave. I Vespers and Compline belong to the following day's
/// office.
fn hymn_doxology_ref(day: &Day) -> &'static str {
    let dates = MoveableDates::compute(day.date.year());
    match day.season {
        Season::Easter if day.date >= dates.ascension => "hymn-doxology-ascension",
        Season::Easter => "hymn-doxology-easter",
        Season::Pentecost if day.date < dates.pentecost.add_days(7) => "hymn-doxology-pentecost",
        Season::Advent
        | Season::Christmas
        | Season::Epiphany
        | Season::Septuagesima
        | Season::Lent
        | Season::Passiontide
        | Season::Pentecost => "hymn-doxology",
    }
}

/// General Rubrics XXIII.4: "All honour, laud, and glory be, O Jesu,
/// Virgin-born, to thee" ends the hymns (b) on Corpus Christi and throughout
/// its Octave, and (c) "whenever the Office is of Blessed Mary", as the
/// Diurnal repeats on her feasts and at the Saturday Office (pp. 414, 448,
/// 69*). Paschaltide keeps its own ending (2026 ordo p. 64, the Saturday
/// Office "Easter dox."; XXIII.4(c) says "even in Paschaltide", ruling
/// pending in #635), and so does Advent, when the Office is of the Season
/// (XXIII.10; Diurnal p. 448), which the owner's category already excludes.
/// A saint's feast or a Sunday within these Octaves keeps the ordinary
/// ending, as the ordos print it (St Joachim, 2023–2026), though XXIII.4
/// gives saints' hymns the Octave's ending (ruling pending in #635).
fn hymn_doxology(day: &Day, hour_name: &str, t: &OfficeTexts) -> (String, String) {
    const NATIVITY: &str = "seasonal/christmas/hymn-doxology";
    let reference = hymn_doxology_ref(day);
    if reference == "hymn-doxology" && day.celebration.as_deref().is_some_and(is_of_our_lady_or_corpus_christi) {
        return (t.get(NATIVITY).to_string(), NATIVITY.to_string());
    }
    // XXIII.5, Diurnal p. 3: the Epiphany ending is said "on Epiphany and
    // throughout the Octave", through 13 January (2026 ordo, "Epiph. dox."
    // through the Octave Day only); afterwards the hymns keep their own (#636).
    if day.season == Season::Epiphany && (day.date.month(), day.date.day()) > (1, 13) {
        return (String::new(), String::new());
    }
    resolve_proper_text(day, hour_name, reference, t)
}

/// The Sunday within the Octave of Corpus Christi is always the II Sunday
/// after Pentecost, whose octave context may name SS Peter and Paul instead.
fn is_of_our_lady_or_corpus_christi(c: &Feast) -> bool {
    const CORPUS_CHRISTI: &str = "corpus-christi";
    c.is_category(Category::BlessedVirgin)
        || [CORPUS_CHRISTI, "pentecost-sunday-2"].contains(&c.id.as_str())
        || octave_parent_id(c) == Some(CORPUS_CHRISTI)
}

/// The seasonal endings are iambic dimeter quatrains and replace only an
/// ending of that metre: four lines of about eight syllables. Sapphic
/// (11.11.11.5), trochaic (8.7.8.7.8.7) and shorter-lined hymns keep theirs.
fn has_common_metre_ending(hymn: &str) -> bool {
    let Some(body) = hymn.trim().strip_suffix("Amen.") else { return false };
    let Some(stanza) = body.trim_end().rsplit("\n\n").next() else { return false };
    let lines: Vec<&str> = stanza.lines().map(str::trim).filter(|l| !l.is_empty()).collect();
    lines.len() == 4 && lines.iter().all(|l| (7..=9).contains(&syllables(l)))
}

/// A rough English syllable count: vowel groups per word, less a silent
/// final "e", "es" or "ed". Close enough to tell 8 from 6 or 11.
fn syllables(line: &str) -> usize {
    line.split(|c: char| !c.is_ascii_alphabetic() && c != '\'' && c != '\u{2019}')
        .map(|w| w.to_ascii_lowercase().replace(['\'', '\u{2019}'], ""))
        .filter(|w| !w.is_empty())
        .map(|w| {
            let is_vowel = |c: char| "aeiouy".contains(c);
            let mut groups = 0;
            let mut prev = false;
            for c in w.chars() {
                let v = is_vowel(c);
                if v && !prev {
                    groups += 1;
                }
                prev = v;
            }
            let silent = (w.ends_with('e') && !w.ends_with("le") && !w.ends_with("ee"))
                || (w.ends_with("es") && !["ses", "xes", "zes", "ches", "shes", "ces", "ges"].iter().any(|s| w.ends_with(s)))
                || (w.ends_with("ed") && !w.ends_with("ted") && !w.ends_with("ded"));
            if silent && groups > 1 { groups - 1 } else { groups.max(1) }
        })
        .sum()
}

/// De-duplicates refs, dropping empty ones, keeping first occurrences.
pub fn compact_refs(refs: Vec<String>) -> Vec<String> {
    let mut seen = HashSet::new();
    refs.into_iter().filter(|r| !r.is_empty() && seen.insert(r.clone())).collect()
}

fn map_element_type(kind: &str) -> ElementType {
    match kind {
        "psalm" => ElementType::Psalm,
        "canticle" => ElementType::Canticle,
        "hymn" | "proper-hymn" => ElementType::Hymn,
        "antiphon" | "marian" | "proper-antiphon" => ElementType::Antiphon,
        "versicle" | "officiant-greeting" | "officiant-opening" | "proper-versicle" => ElementType::Versicle,
        "response" | "proper-responsory" => ElementType::Response,
        "prayer" | "secret-prayer" | "silent-prayer" | "partly-secret-prayer" | "officiant-confession" => ElementType::Prayer,
        "corporate-lord-prayer" => ElementType::CorporateLordPrayer,
        "preces" => ElementType::Preces,
        "gloria-patri" => ElementType::Doxology,
        "chapter" | "proper-chapter" => ElementType::Chapter,
        "collect" | "proper-collect" => ElementType::Collect,
        "blessing" => ElementType::Blessing,
        "proper-opening-acclamation" => ElementType::OpeningAcclamation,
        "proper-short-responsory" => ElementType::ShortResponsory,
        "dialogue" => ElementType::Dialogue,
        // Unknown types (and "commemorations") are rubrics.
        _ => ElementType::Rubric,
    }
}

/// A label from the ref's last path component: "Psalm 4", or a title-cased
/// canticle or hymn name.
fn format_label(kind: &str, reference: &str) -> String {
    let name = reference.rsplit('/').next().unwrap_or("").replace('-', " ");
    match kind {
        "psalm" => {
            let n = name.trim_start_matches('0');
            format!("Psalm {}", if n.is_empty() { "0" } else { n })
        }
        "canticle" | "hymn" => title_case(&name),
        _ => String::new(),
    }
}

/// A leading "!" line is the chapter's scripture reference.
pub fn extract_chapter_ref(text: &str) -> (String, String) {
    if let Some((first, rest)) = text.split_once('\n') {
        let first = first.trim();
        if let Some(r) = first.strip_prefix('!') {
            return (r.to_string(), rest.trim().to_string());
        }
    }
    (String::new(), text.to_string())
}

/// Replaces a hymn's final verse stanza and closing Amen with the complete
/// seasonal doxology. Amen may stand alone after a blank line.
fn substitute_hymn_doxology(hymn: &str, doxology: &str) -> String {
    let Some(without_amen) = hymn.trim().strip_suffix("Amen.") else {
        return hymn.to_string();
    };
    // Find the verse stanza before the Amen, not an Amen-only final block.
    let trimmed = without_amen.trim_end();
    match trimmed.rfind("\n\n") {
        None => hymn.to_string(),
        Some(i) => format!("{}{}", &trimmed[..i + 2], doxology.trim()),
    }
}

pub fn title_case(s: &str) -> String {
    s.split_whitespace()
        .map(|w| {
            let mut c = w.chars();
            match c.next() {
                Some(f) => f.to_uppercase().chain(c).collect(),
                None => String::new(),
            }
        })
        .collect::<Vec<String>>()
        .join(" ")
}

/// The first "[… not found: …]" marker in rendered text. The composer renders
/// such a marker instead of failing when no corpus text resolves; callers that
/// check whole calendars (the golden digest, the audit sweep) treat it as an
/// error.
pub fn unresolved_marker(text: &str) -> Option<&str> {
    let mid = text.find(" not found: ")?;
    let start = text[..mid].rfind('[')?;
    let end = mid + text[mid..].find(']')?;
    Some(&text[start..=end])
}

#[cfg(test)]
#[path = "engine_tests.rs"]
mod behavior_tests;
