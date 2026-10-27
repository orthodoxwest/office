//! Finding a feast: every feast and observance a reader can look up, how each is kept in a given
//! year, and the words every front uses to say so. Search and the feast page read the calendar,
//! never the printed ordo, so a feast printed under a shorter name, reduced to a commemoration,
//! transferred, or not kept at all is still found and told truly.

use std::collections::HashMap;

use calendar::commemoration::normalize_commemoration_name;
use calendar::model::MonthDay;
use calendar::{CalendarData, CalendarDay, Category, Color, Date, Rank};

use crate::{MONTHS, day_name};

/// Familiar names the calendar's own names do not carry, by feast id.
pub const ALIASES: [(&str, &str); 14] = [
    ("Candlemas", "purification-bvm"),
    ("Meeting of the Lord", "purification-bvm"),
    ("Lady Day", "annunciation"),
    ("Michaelmas", "dedication-st-michael"),
    ("Lammas", "chains-st-peter"),
    ("All Hallows", "all-saints"),
    ("Christmas", "christmas"),
    ("Easter", "easter-sunday"),
    ("Pascha", "easter-sunday"),
    ("Whitsunday", "pentecost"),
    ("Holy Cross Day", "exaltation-holy-cross"),
    ("Theophany", "epiphany"),
    ("Dormition", "assumption-bvm"),
    ("Good Friday", "good-friday"),
];

/// How many matches a search lists.
pub const MAX_RESULTS: usize = 40;

/// A query shorter than this lists nothing.
pub const MIN_QUERY: usize = 2;

/// An observance the ordo keeps for monastics and oblates only.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct MonasticNote {
    /// Where its office is found ("Proper Office, Monastic Diurnal pp. 560–564"), or empty.
    pub office: String,
}

/// A feast a reader can look up.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct FeastEntry {
    pub id: String,
    pub name: String,
    pub rank: Rank,
    pub color: Color,
    pub category: Option<Category>,
    /// Its date, for a fixed feast.
    pub fixed: Option<MonthDay>,
    /// Set for an observance kept by monastics and oblates only.
    pub monastic: Option<MonasticNote>,
    /// Normalized names it answers to: its own, then its aliases.
    keys: Vec<String>,
}

/// Every feast and observance a reader can look up, in calendar order.
#[derive(Clone, Debug, Default)]
pub struct FeastIndex {
    entries: Vec<FeastEntry>,
}

impl FeastIndex {
    /// The calendar's feasts and the ordo's monastic observances. A dated entry that only
    /// repeats a greater feast of the same date and name ("St Patrick, Bishop & Confessor" as a
    /// commemoration beside St Patrick) is the calendar's own duplicate, never kept apart from
    /// it, and is left out.
    pub fn new(data: &CalendarData) -> FeastIndex {
        let mut entries: Vec<FeastEntry> = Vec::new();
        for f in &data.feasts {
            let entry = FeastEntry {
                id: f.id.clone(),
                name: f.name.clone(),
                rank: f.rank,
                color: f.color,
                category: f.category,
                fixed: f.fixed,
                monastic: None,
                keys: vec![normalize(&f.name)],
            };
            match entries.iter().position(|e| duplicates(e, &entry)) {
                Some(i) if entry.rank.weight() > entries[i].rank.weight() => entries[i] = entry,
                Some(_) => {}
                None => entries.push(entry),
            }
        }
        for m in &data.monastic {
            entries.push(FeastEntry {
                id: m.id.clone(),
                name: m.name.clone(),
                rank: m.rank,
                color: Color::White,
                category: None,
                fixed: Some(m.fixed),
                monastic: Some(MonasticNote { office: m.office.clone().unwrap_or_default() }),
                keys: vec![normalize(&m.name)],
            });
        }
        for (alias, id) in ALIASES {
            if let Some(e) = entries.iter_mut().find(|e| e.id == id) {
                e.keys.push(normalize(alias));
            }
        }
        entries.sort_by_key(|e| e.fixed.map_or((13, 0), |f| (f.month, f.day)));
        FeastIndex { entries }
    }

    pub fn get(&self, id: &str) -> Option<&FeastEntry> {
        self.entries.iter().find(|e| e.id == id)
    }

    pub fn len(&self) -> usize {
        self.entries.len()
    }

    pub fn is_empty(&self) -> bool {
        self.entries.is_empty()
    }

    /// The feast a printed name names ("St Paul" in a day's commemorations), nearest `near`
    /// when several share the name and within [`NAME_REACH`] days of it; `None` when no feast
    /// is named so.
    pub fn by_name(&self, name: &str, near: Date) -> Option<&FeastEntry> {
        let key = normalize_commemoration_name(name);
        if key.is_empty() {
            return None;
        }
        let distance = |e: &FeastEntry| {
            e.fixed.map_or(0, |f| {
                let d = Date::new(near.year(), f.month as i32, f.day as i32).days_since(near).abs();
                d.min(365 - d)
            })
        };
        self.entries
            .iter()
            .filter(|e| e.monastic.is_none() && normalize_commemoration_name(&e.name) == key)
            .filter(|e| distance(e) <= NAME_REACH)
            .min_by_key(|e| distance(e))
    }

    /// The feast a day keeps or commemorates under `id` and `name`: its own entry, else the
    /// feast of that name near `date` (a generated octave day, a commemoration printed under its
    /// saint's name). Every front links a day's feast and commemorations by this.
    pub fn find(&self, id: &str, name: &str, date: Date) -> Option<&FeastEntry> {
        self.get(id).filter(|e| e.monastic.is_none()).or_else(|| self.by_name(name, date))
    }

    /// For an observance kept by monastics and oblates only, the feast of the same saint that
    /// everyone keeps ("St Benedict, Abbot" beside the Solemnity of St Benedict).
    pub fn kept_by_all(&self, entry: &FeastEntry) -> Option<&FeastEntry> {
        entry.monastic.as_ref()?;
        let own = normalize_commemoration_name(&entry.name);
        self.entries.iter().filter(|e| e.monastic.is_none() && e.fixed.is_some()).find(|e| {
            let saint = normalize_commemoration_name(e.name.split(',').next().unwrap_or_default());
            !saint.is_empty() && own.ends_with(&saint) && own.len() > saint.len()
        })
    }

    /// Every entry, in calendar order.
    pub fn entries(&self) -> &[FeastEntry] {
        &self.entries
    }

    /// The entries whose names (or aliases) hold every word of the query, each word the start of
    /// a word of the name: best match first, then the greater feast, then the calendar's order.
    pub fn search(&self, query: &str) -> Vec<&FeastEntry> {
        let q = normalize(query);
        let words: Vec<&str> = q.split(' ').filter(|w| !w.is_empty()).collect();
        if q.chars().filter(|c| c.is_alphanumeric()).count() < MIN_QUERY {
            return Vec::new();
        }
        let mut found: Vec<(u8, i32, usize, &FeastEntry)> = self
            .entries
            .iter()
            .enumerate()
            .filter_map(|(order, e)| {
                let score = e.keys.iter().enumerate().filter_map(|(k, key)| match_score(key, &q, &words, k > 0)).min()?;
                Some((score, -e.rank.weight(), order, e))
            })
            .collect();
        found.sort_by_key(|(score, rank, order, _)| (*score, *rank, *order));
        found.into_iter().take(MAX_RESULTS).map(|(.., e)| e).collect()
    }
}

/// How well a normalized name answers the query, lower being better; `None` when it does not.
fn match_score(key: &str, query: &str, words: &[&str], alias: bool) -> Option<u8> {
    let key_words: Vec<&str> = key.split(' ').collect();
    if !words.iter().all(|w| key_words.iter().any(|k| k.starts_with(w))) {
        return None;
    }
    let base = if key == query {
        0
    } else if key.starts_with(query) || key.strip_prefix("st ").is_some_and(|k| k.starts_with(query)) {
        1
    } else {
        2
    };
    Some(if alias { base + 3 } else { base })
}

/// Two dated entries of the same date whose names are the same saint.
fn duplicates(a: &FeastEntry, b: &FeastEntry) -> bool {
    let (Some(fa), Some(fb)) = (a.fixed, b.fixed) else { return false };
    if fa != fb {
        return false;
    }
    let (x, y) = (normalize_commemoration_name(&a.name), normalize_commemoration_name(&b.name));
    !x.is_empty() && !y.is_empty() && (x == y || x.contains(&y) || y.contains(&x))
}

/// Lowercase words without accents or punctuation, "saint" and its abbreviations as "st", and
/// "&" as "and", so a reader's spelling meets the calendar's.
pub fn normalize(s: &str) -> String {
    let mut out = String::with_capacity(s.len());
    for c in s.chars() {
        let c = fold_accent(c);
        if c == '&' {
            out.push_str(" and ");
        } else if c.is_alphanumeric() {
            out.extend(c.to_lowercase());
        } else {
            out.push(' ');
        }
    }
    out.split_whitespace()
        .map(|w| match w {
            "saint" | "saints" | "sts" | "ss" => "st",
            w => w,
        })
        .collect::<Vec<_>>()
        .join(" ")
}

fn fold_accent(c: char) -> char {
    match c {
        'à' | 'á' | 'â' | 'ã' | 'ä' | 'å' => 'a',
        'À' | 'Á' | 'Â' | 'Ã' | 'Ä' | 'Å' => 'A',
        'è' | 'é' | 'ê' | 'ë' => 'e',
        'È' | 'É' | 'Ê' | 'Ë' => 'E',
        'ì' | 'í' | 'î' | 'ï' => 'i',
        'Ì' | 'Í' | 'Î' | 'Ï' => 'I',
        'ò' | 'ó' | 'ô' | 'õ' | 'ö' => 'o',
        'Ò' | 'Ó' | 'Ô' | 'Õ' | 'Ö' => 'O',
        'ù' | 'ú' | 'û' | 'ü' => 'u',
        'Ù' | 'Ú' | 'Û' | 'Ü' => 'U',
        'ç' => 'c',
        'Ç' => 'C',
        'ñ' => 'n',
        'Ñ' => 'N',
        '’' | '‘' => '\'',
        c => c,
    }
}

/// How a feast is kept in a year.
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum Keeping {
    /// It is the day's office.
    Kept,
    /// It is the day's office on another day than its own.
    Transferred { from: Date },
    /// The day's office is another's, and this feast is commemorated in it.
    Commemorated { holder: String },
    /// Monastics and oblates keep it; the parish keeps the day's own office.
    Monastic { parish: String },
    /// Neither kept nor commemorated: `holder` keeps its date.
    NotKept { holder: String },
}

/// A feast's keeping in one year: the date (none when it is not kept and has no date of its
/// own) and the colour of that day's office.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct YearKeeping {
    pub year: i32,
    pub date: Option<Date>,
    pub color: Color,
    pub keeping: Keeping,
}

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
enum Seen {
    Kept(usize),
    Commemorated(usize),
    Monastic(usize),
}

/// One year's days, indexed by the feasts they keep, commemorate, or bracket for monastics.
pub struct YearIndex {
    year: i32,
    days: Vec<CalendarDay>,
    seen: HashMap<String, Seen>,
    /// The same, by name: a feast the calendar keeps under another id of the same name (a
    /// generated octave day, a commemoration printed as its saint's name).
    by_name: HashMap<String, Vec<Seen>>,
}

/// How far from its own date a feast found only by name may be.
const NAME_REACH: i32 = 14;

impl YearIndex {
    /// `days` are the year's days in order, 1 January first.
    pub fn new(year: i32, days: Vec<CalendarDay>) -> YearIndex {
        let mut seen: HashMap<String, Seen> = HashMap::new();
        let mut by_name: HashMap<String, Vec<Seen>> = HashMap::new();
        for (i, d) in days.iter().enumerate() {
            if let Some(c) = &d.celebration {
                let prior = seen.get(&c.id).copied();
                if !matches!(prior, Some(Seen::Kept(_))) {
                    seen.insert(c.id.clone(), Seen::Kept(i));
                }
                by_name.entry(normalize_commemoration_name(&c.name)).or_default().push(Seen::Kept(i));
            }
            for c in &d.commemorations {
                seen.entry(c.id.clone()).or_insert(Seen::Commemorated(i));
                by_name.entry(normalize_commemoration_name(&c.name)).or_default().push(Seen::Commemorated(i));
            }
            for m in &d.monastic {
                seen.entry(m.id.clone()).or_insert(Seen::Monastic(i));
            }
        }
        YearIndex { year, days, seen, by_name }
    }

    pub fn year(&self) -> i32 {
        self.year
    }

    /// The year's day for `date`, when it falls in this year.
    pub fn day(&self, date: Date) -> Option<&CalendarDay> {
        (date.year() == self.year).then(|| self.days.get(date.ordinal() as usize - 1)).flatten()
    }

    /// How `entry` is kept this year.
    pub fn keeping(&self, entry: &FeastEntry) -> YearKeeping {
        let at = |i: usize, keeping: Keeping| {
            let d = &self.days[i];
            YearKeeping { year: self.year, date: Some(d.date), color: d.color, keeping }
        };
        let own = entry.fixed.map(|f| Date::new(self.year, f.month as i32, f.day as i32)).filter(|d| d.year() == self.year);
        match self.seen.get(&entry.id).copied().or_else(|| self.by_name(entry, own)).as_ref() {
            Some(Seen::Kept(i)) => {
                let date = self.days[*i].date;
                match own {
                    Some(from) if from != date && !leap_shift(from, date) => at(*i, Keeping::Transferred { from }),
                    _ => at(*i, Keeping::Kept),
                }
            }
            Some(Seen::Commemorated(i)) => at(*i, Keeping::Commemorated { holder: day_name(&self.days[*i]) }),
            Some(Seen::Monastic(i)) => at(*i, Keeping::Monastic { parish: day_name(&self.days[*i]) }),
            None => match own.and_then(|d| self.days.get(d.ordinal() as usize - 1)) {
                Some(d) => {
                    YearKeeping { year: self.year, date: Some(d.date), color: d.color, keeping: Keeping::NotKept { holder: day_name(d) } }
                }
                None => {
                    YearKeeping { year: self.year, date: None, color: entry.color, keeping: Keeping::NotKept { holder: String::new() } }
                }
            },
        }
    }
}

impl YearIndex {
    /// The keeping of a day bearing the entry's name: kept before commemorated, then nearest its
    /// own date, which it must lie within [`NAME_REACH`] days of.
    fn by_name(&self, entry: &FeastEntry, own: Option<Date>) -> Option<Seen> {
        if entry.monastic.is_some() {
            return None;
        }
        let found = self.by_name.get(&normalize_commemoration_name(&entry.name))?;
        let index = |s: &Seen| match s {
            Seen::Kept(i) | Seen::Commemorated(i) | Seen::Monastic(i) => *i,
        };
        let distance = |s: &Seen| own.map_or(0, |own| self.days[index(s)].date.days_since(own).abs());
        found
            .iter()
            .filter(|s| own.is_none() || distance(s) <= NAME_REACH)
            .min_by_key(|s| (!matches!(s, Seen::Kept(_)), distance(s)))
            .copied()
    }
}

/// In a leap year the feasts of 24–28 February move a day later (the Roman leap-day shift):
/// the feast keeps its own day, not a transferred one.
fn leap_shift(own: Date, kept: Date) -> bool {
    calendar::date::is_leap_year(own.year()) && own.month() == 2 && own.day() >= 24 && kept == own.add_days(1)
}

// ---- Words ----

/// The fixed words of the search and the feast page, as every front sets them.
pub struct FeastWords {
    /// The search's title, and the way back to it.
    pub find_title: &'static str,
    /// Before a fixed feast's date: "Kept on February 27".
    pub kept_on: &'static str,
    /// The search field's prompt.
    pub find_prompt: &'static str,
    /// A result's link to its feast's page.
    pub every_year: &'static str,
    /// Over a feast's years, under their heading.
    pub years_hint: &'static str,
    /// The next keeping's way to that day.
    pub days_hours: &'static str,
    /// The heading over a feast's texts.
    pub texts_heading: &'static str,
    /// The mark on a text from the Common.
    pub common_mark: &'static str,
    /// Before the link to the day the texts are said in full.
    pub whole_office: &'static str,
    /// Before the feast everyone keeps, on a monastic observance's page.
    pub everyone_keeps: &'static str,
    /// The heading of a monastic observance's note.
    pub monastic_heading: &'static str,
}

pub const FEAST_WORDS: FeastWords = FeastWords {
    find_title: "Find a feast",
    kept_on: "Kept on",
    find_prompt: "Find a feast or saint",
    every_year: "Every year",
    years_hint: "Each date opens that day, with its hours.",
    days_hours: "The day’s hours",
    texts_heading: "Proper of the feast",
    common_mark: "Common",
    whole_office: "The whole office:",
    everyone_keeps: "Everyone keeps",
    monastic_heading: "Monastics & Oblates only",
};

/// The status a year's row leads with.
pub fn keeping_label(k: &Keeping) -> &'static str {
    match k {
        Keeping::Kept => "Kept",
        Keeping::Transferred { .. } => "Transferred",
        Keeping::Commemorated { .. } => "Commemorated",
        Keeping::Monastic { .. } => "Monastics & Oblates",
        Keeping::NotKept { .. } => "Not kept",
    }
}

/// A short word for styling the status: "kept", "transferred", "commemorated", "monastic",
/// "not-kept".
pub fn keeping_class(k: &Keeping) -> &'static str {
    match k {
        Keeping::Kept => "kept",
        Keeping::Transferred { .. } => "transferred",
        Keeping::Commemorated { .. } => "commemorated",
        Keeping::Monastic { .. } => "monastic",
        Keeping::NotKept { .. } => "not-kept",
    }
}

/// The line under the status: why it is kept so, or empty.
pub fn keeping_note(k: &Keeping) -> String {
    match k {
        Keeping::Kept => String::new(),
        Keeping::Transferred { from } => format!("from {}", month_day(*from)),
        Keeping::Commemorated { holder } => format!("{holder} holds the day"),
        Keeping::Monastic { parish } => format!("the parish keeps {}", parish_office(parish)),
        Keeping::NotKept { holder } if holder.is_empty() => String::new(),
        Keeping::NotKept { holder } => format!("{holder} holds the day"),
    }
}

/// "Feria" reads as "a feria" in a sentence.
fn parish_office(name: &str) -> String {
    if name == "Feria" || name.ends_with(" feria") { format!("a {}", name.to_lowercase()) } else { name.to_string() }
}

/// "February 27".
pub fn month_day(d: Date) -> String {
    format!("{} {}", MONTHS[d.month() as usize - 1], d.day())
}

/// "Fri, Feb 27", a row's date.
pub fn row_date(d: Date) -> String {
    format!("{}, {} {}", &d.weekday().name()[..3], &MONTHS[d.month() as usize - 1][..3], d.day())
}

/// Where a feast falls: "February 27", or "A moveable feast".
pub fn feast_date(e: &FeastEntry) -> String {
    match e.fixed {
        Some(f) => format!("{} {}", MONTHS[f.month as usize - 1], f.day),
        None => "A moveable feast".to_string(),
    }
}

/// Whom or what the feast honours, as the feast page's subtitle names it; empty when the
/// category says nothing a reader needs.
pub fn category_label(c: Option<Category>) -> &'static str {
    match c {
        Some(Category::Lord) => "Feast of Our Lord",
        Some(Category::BlessedVirgin) => "Feast of Our Lady",
        Some(Category::Angel) => "Archangel",
        Some(Category::Apostle) => "Apostle",
        Some(Category::Evangelist) => "Evangelist",
        Some(Category::Martyr) => "Martyr",
        Some(Category::Martyrs) => "Martyrs",
        Some(Category::BishopMartyr) => "Bishop & Martyr",
        Some(Category::VirginMartyr) => "Virgin & Martyr",
        Some(Category::ConfessorBishop) => "Bishop & Confessor",
        Some(Category::ConfessorDoctor) => "Confessor & Doctor",
        Some(Category::Confessor) => "Confessor",
        Some(Category::Virgin) => "Virgin",
        Some(Category::HolyWoman) => "Holy Woman",
        Some(Category::Dedication) => "Dedication",
        Some(Category::Feria) | Some(Category::Sunday) | None => "",
    }
}

/// The Common a category's texts come from, as the Diurnal names it.
pub fn common_name(c: Category) -> &'static str {
    match c {
        Category::Lord => "Common of Feasts of Our Lord",
        Category::BlessedVirgin => "Common of Feasts of the Blessed Virgin Mary",
        Category::Angel => "Common of Angels",
        Category::Apostle | Category::Evangelist => "Common of Apostles and Evangelists",
        Category::Martyr => "Common of One Martyr",
        Category::Martyrs => "Common of Many Martyrs",
        Category::BishopMartyr => "Common of a Bishop and Martyr",
        Category::VirginMartyr | Category::Virgin => "Common of Virgins",
        Category::ConfessorBishop => "Common of a Confessor and Bishop",
        Category::ConfessorDoctor => "Common of a Confessor and Doctor",
        Category::Confessor => "Common of a Confessor not a Bishop",
        Category::HolyWoman => "Common of Holy Women not Virgins",
        Category::Dedication => "Common of the Dedication of a Church",
        Category::Feria | Category::Sunday => "",
    }
}

/// A search result's line under its name: "February 27 · Bishop & Confessor".
pub fn result_meta(e: &FeastEntry) -> String {
    let what = if e.monastic.is_some() { "Monastics & Oblates only" } else { category_label(e.category) };
    if what.is_empty() { feast_date(e) } else { format!("{} · {what}", feast_date(e)) }
}

/// How the ordo marks a rank: "greater" (red, a greater double and above), "double" (blue), or
/// empty.
pub fn rank_tone(r: Rank) -> &'static str {
    match r {
        Rank::Double1stClass | Rank::Double2ndClass | Rank::GreaterDouble => "greater",
        Rank::Double => "double",
        Rank::SemiDouble | Rank::PrivilegedFeria | Rank::Simple | Rank::Commemoration => "",
    }
}

/// The feast page's heading over its years.
pub const YEARS_HEADING: &str = "The next eight years";

/// The feast page's link to the whole office of the day it is next kept.
pub fn whole_office_label(d: Date) -> String {
    format!("the hours of {}", crate::long_date(d))
}

/// The feast page's subtitle: "Greater Double · White · Bishop & Confessor".
pub fn feast_subtitle(e: &FeastEntry) -> String {
    let mut parts = vec![e.rank.display_name().to_string()];
    match &e.monastic {
        Some(_) => parts.push("Monastics & Oblates only".into()),
        None => {
            parts.push(crate::title_case(e.color.as_str()));
            let c = category_label(e.category);
            if !c.is_empty() {
                parts.push(c.into());
            }
        }
    }
    parts.join(" · ")
}

/// The feast page's note for a monastic observance.
pub fn monastic_note(e: &FeastEntry) -> String {
    let office = e.monastic.as_ref().map(|m| m.office.as_str()).unwrap_or_default();
    let mut note = "The ordo keeps this observance for monastics and oblates only. Parishes keep the day's own office, and that is what this app's hours show.".to_string();
    if !office.is_empty() {
        note.push_str(&format!(" Its office: {office}."));
    }
    note
}

/// The feast page's pointer from a monastic observance to the feast everyone keeps.
pub fn kept_by_all_note(e: &FeastEntry) -> String {
    format!("on {}.", feast_date(e))
}

/// The feast page's heading over the next date it is kept, by how it is kept then.
pub fn next_label(k: &Keeping) -> &'static str {
    match k {
        Keeping::Kept | Keeping::Transferred { .. } => "Next kept",
        Keeping::Commemorated { .. } => "Next commemorated",
        Keeping::Monastic { .. } => "Next kept by monastics and oblates",
        Keeping::NotKept { .. } => "",
    }
}

/// The next of `years` on or after `today` when the feast is kept, commemorated, or bracketed
/// for monastics.
pub fn next_keeping(years: &[YearKeeping], today: Date) -> Option<&YearKeeping> {
    years.iter().filter(|y| !matches!(y.keeping, Keeping::NotKept { .. })).find(|y| y.date.is_some_and(|d| d >= today))
}

/// The feast page's line over its texts. `proper` and `common` say whether any text came from
/// the feast's own proper and from its Common.
pub fn texts_source(category: Option<Category>, proper: bool, common: bool) -> String {
    let common_name = category.map(common_name).unwrap_or_default();
    match (proper, common && !common_name.is_empty()) {
        (true, true) => format!("Proper to the feast; texts marked Common are from the {common_name}."),
        (false, true) => format!("From the {common_name}."),
        _ => "Proper to the feast.".to_string(),
    }
}

/// How many years the feast page lists; [`YEARS_HEADING`] names it.
pub const FEAST_PAGE_YEARS: i32 = 8;

/// The search page's summary of what it found.
pub fn results_count(n: usize) -> String {
    match n {
        0 => "No feast or saint by that name".into(),
        1 => "1 feast".into(),
        n if n >= MAX_RESULTS => format!("The first {n} feasts"),
        n => format!("{n} feasts"),
    }
}

/// The search page's subtitle for the two years it covers.
pub fn search_years(year: i32) -> String {
    format!("Feasts & commemorations, {year} and {}", year + 1)
}

/// The feast page's address segment for an id: ids are already lowercase words and hyphens.
pub fn is_feast_id(id: &str) -> bool {
    !id.is_empty() && id.len() <= 120 && id.chars().all(|c| c.is_ascii_lowercase() || c.is_ascii_digit() || c == '-')
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn normalize_meets_the_readers_spelling() {
        assert_eq!(normalize("Ss. Peter & Paul"), "st peter and paul");
        assert_eq!(normalize("Saint Rapha\u{eb}l"), "st raphael");
        assert_eq!(normalize("  St   Mary  Magdalene, Penitent "), "st mary magdalene penitent");
    }

    #[test]
    fn match_score_prefers_whole_and_leading_matches() {
        let words = |q: &'static str| q.split(' ').collect::<Vec<_>>();
        assert_eq!(match_score("st raphael", "st raphael", &words("st raphael"), false), Some(0));
        assert_eq!(match_score("st raphael of brooklyn", "raphael", &words("raphael"), false), Some(1));
        assert_eq!(match_score("translation of st raphael", "raphael", &words("raphael"), false), Some(2));
        assert_eq!(match_score("candlemas", "candle", &words("candle"), true), Some(4));
        assert_eq!(match_score("st raphael", "gabriel", &words("gabriel"), false), None);
    }

    #[test]
    fn words_for_each_keeping() {
        let d = Date::new(2027, 4, 25);
        assert_eq!(keeping_note(&Keeping::Transferred { from: d }), "from April 25");
        assert_eq!(keeping_note(&Keeping::Commemorated { holder: "Quinquagesima Sunday".into() }), "Quinquagesima Sunday holds the day");
        assert_eq!(keeping_note(&Keeping::Monastic { parish: "Feria".into() }), "the parish keeps a feria");
        assert_eq!(keeping_note(&Keeping::Kept), "");
        assert_eq!(row_date(Date::new(2027, 2, 27)), "Sat, Feb 27");
        assert_eq!(texts_source(Some(Category::ConfessorBishop), false, true), "From the Common of a Confessor and Bishop.");
        assert_eq!(texts_source(Some(Category::Apostle), true, false), "Proper to the feast.");
    }

    /// The data directory, read as the calendar loader reads it.
    struct Data;

    impl calendar::DataSource for Data {
        fn display_path(&self, rel: &str) -> String {
            format!("../../data/{rel}")
        }
        fn read(&self, rel: &str) -> Result<Option<String>, String> {
            match std::fs::read_to_string(format!("../../data/{rel}")) {
                Ok(s) => Ok(Some(s)),
                Err(e) if e.kind() == std::io::ErrorKind::NotFound => Ok(None),
                Err(e) => Err(e.to_string()),
            }
        }
        fn walk(&self, rel: &str) -> Result<Vec<(String, Vec<u8>)>, String> {
            fn visit(dir: &std::path::Path, rel: &str, out: &mut Vec<(String, Vec<u8>)>) -> std::io::Result<()> {
                let mut names: Vec<_> = std::fs::read_dir(dir)?.map(|e| e.map(|e| e.file_name())).collect::<Result<_, _>>()?;
                names.sort();
                for name in names {
                    let path = dir.join(&name);
                    let rel = format!("{rel}/{}", name.to_string_lossy());
                    if path.is_dir() {
                        visit(&path, &rel, out)?;
                    } else {
                        out.push((rel, std::fs::read(&path)?));
                    }
                }
                Ok(())
            }
            let mut out = Vec::new();
            visit(std::path::Path::new(&format!("../../data/{rel}")), rel, &mut out).map_err(|e| e.to_string())?;
            Ok(out)
        }
    }

    fn setup() -> (CalendarData, FeastIndex) {
        let data = CalendarData::load(&Data).unwrap();
        let index = FeastIndex::new(&data);
        (data, index)
    }

    fn year(data: &CalendarData, y: i32) -> YearIndex {
        YearIndex::new(y, calendar::build_calendar(y, data).unwrap().days)
    }

    #[test]
    fn every_alias_names_a_feast() {
        let (_, index) = setup();
        for (alias, id) in ALIASES {
            assert!(index.get(id).is_some(), "{alias} names {id}, which is not a feast");
        }
        assert_eq!(index.search("candlemas")[0].id, "purification-bvm");
    }

    #[test]
    fn a_dated_duplicate_gives_way_to_the_feast() {
        let (_, index) = setup();
        assert!(index.get("st-patrick").is_some());
        assert!(index.get("comm-extra-03-17-st-patrick-bishop-and-confessor").is_none());
        assert_eq!(index.search("patrick").len(), 1);
    }

    #[test]
    fn search_finds_every_raphael_best_first() {
        let (_, index) = setup();
        let ids: Vec<&str> = index.search("raphael").iter().map(|e| e.id.as_str()).collect();
        assert_eq!(ids[0], "st-raphael-of-brooklyn");
        assert_eq!(ids.len(), 3, "{ids:?}");
        assert!(index.search("r").is_empty(), "a single letter lists nothing");
        assert!(index.search("st raphael brook").iter().all(|e| e.name.contains("Brooklyn")));
    }

    #[test]
    fn each_way_a_feast_is_kept() {
        let (data, index) = setup();
        let (y26, y27) = (year(&data, 2026), year(&data, 2027));
        let raphael = index.get("st-raphael-of-brooklyn").unwrap();
        assert_eq!(y26.keeping(raphael).keeping, Keeping::Commemorated { holder: "Friday after Ash Wednesday".into() });
        assert_eq!(y27.keeping(raphael).keeping, Keeping::Kept);
        assert_eq!(y27.keeping(raphael).date, Some(Date::new(2027, 2, 27)));
        // Pascha falls on 2 May 2027.
        let athanasius = index.get("st-athanasius").unwrap();
        assert!(
            matches!(&y27.keeping(athanasius).keeping, Keeping::NotKept { holder } if !holder.is_empty()),
            "{:?}",
            y27.keeping(athanasius)
        );
        assert_eq!(y26.keeping(athanasius).keeping, Keeping::Kept);
        // 25 April 2027 is Palm Sunday.
        let mark = y27.keeping(index.get("st-mark").unwrap());
        assert_eq!(mark.keeping, Keeping::Transferred { from: Date::new(2027, 4, 25) });
        let benedict = index.get("solemnity-st-benedict").unwrap();
        assert_eq!(y26.keeping(benedict).keeping, Keeping::Monastic { parish: "Saturday Office of the B.V.M.".into() });
        assert!(feast_subtitle(benedict).contains("Monastics & Oblates only"));
        assert_eq!(index.kept_by_all(benedict).map(|e| e.id.as_str()), Some("st-benedict"));
        assert_eq!(index.by_name("St Raphael of Brooklyn", Date::new(2026, 2, 27)).map(|e| e.id.as_str()), Some("st-raphael-of-brooklyn"));
    }

    #[test]
    fn feast_ids_are_plain() {
        assert!(is_feast_id("st-raphael-of-brooklyn"));
        assert!(!is_feast_id("../etc"));
        assert!(!is_feast_id(""));
    }
}
