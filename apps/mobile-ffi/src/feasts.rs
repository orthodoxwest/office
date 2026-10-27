//! Finding a feast, as the web's "/calendar/find" and "/feast/{id}" pages give it
//! (presentation::feast): the apps set the same words in their own idiom.

use std::sync::Arc;

use calendar::Date;
use corpus::typography::typeset;
use office::feast::{DayOf, FeastTexts, feast_texts};
use presentation::feast::{
    FEAST_PAGE_YEARS, FEAST_WORDS, FeastEntry, Keeping, MAX_RESULTS, YEARS_HEADING, YearIndex, YearKeeping, date_lead, feast_date,
    feast_subtitle, is_entry, is_feast_id, keeping_class, keeping_label, keeping_note, kept_by_all_note, monastic_note, next_keeping,
    next_label, rank_tone, result_meta, results_count, row_date, search_years, texts_keeping, texts_source, whole_office_label,
};
use presentation::long_date;

use crate::{BlockView, CivilDate, OfficeCore, OfficeError, failed};

/// The search's results: the feasts whose names hold the query, each kept this year and next.
#[derive(Clone, Debug, uniffi::Record)]
pub struct FindView {
    /// "Feasts & commemorations, 2026 and 2027".
    pub subtitle: String,
    /// "3 feasts", "No feast or saint by that name", or empty before a search.
    pub count: String,
    pub results: Vec<FeastResultView>,
}

/// A feast in the search results.
#[derive(Clone, Debug, uniffi::Record)]
pub struct FeastResultView {
    /// The feast page's id, for `feast_page`.
    pub id: String,
    pub name: String,
    /// "gd", and its full name "Greater Double".
    pub rank: String,
    pub rank_full: String,
    /// "greater" (red), "double" (blue), or empty, as the ordo marks ranks.
    pub rank_tone: String,
    /// "February 27 · Bishop & Confessor".
    pub meta: String,
    pub years: Vec<FeastYearView>,
}

/// A feast's keeping in one year, a row leading to that day.
#[derive(Clone, Debug, uniffi::Record)]
pub struct FeastYearView {
    pub year: i32,
    /// The day, or none when the feast has no date that year.
    pub date: Option<CivilDate>,
    /// "Sat, Feb 27", or empty.
    pub date_label: String,
    /// "Kept", "Commemorated", "Transferred", "Monastics & Oblates", "Not kept".
    pub label: String,
    /// "kept", "commemorated", "transferred", "monastic", "not-kept", for its colour.
    pub kind: String,
    /// "Friday after Ash Wednesday holds the day", or empty.
    pub note: String,
    /// The colour of that day's office.
    pub color: String,
}

/// A feast another links to by name.
#[derive(Clone, Debug, uniffi::Record)]
pub struct FeastLinkView {
    pub id: String,
    pub name: String,
}

/// The next day the feast is kept, leading to that day.
#[derive(Clone, Debug, uniffi::Record)]
pub struct NextKeepingView {
    /// "Next kept", "Next commemorated", …
    pub label: String,
    pub date: CivilDate,
    /// "Saturday, February 27, 2027".
    pub date_label: String,
    pub color: String,
}

/// One of the feast's texts, laid out as the hours lay out theirs.
#[derive(Clone, Debug, uniffi::Record)]
pub struct FeastTextView {
    /// "At Lauds · Benedictus", "Collect", …
    pub label: String,
    /// From the Common, marked so.
    pub common: bool,
    pub blocks: Vec<BlockView>,
}

/// The day whose hours say the feast's texts in full.
#[derive(Clone, Debug, uniffi::Record)]
pub struct WholeOfficeView {
    /// "the hours of Saturday, February 27, 2027".
    pub label: String,
    pub date: CivilDate,
}

/// A feast's page.
#[derive(Clone, Debug, uniffi::Record)]
pub struct FeastPageView {
    pub id: String,
    pub name: String,
    /// "Greater Double · White · Bishop & Confessor".
    pub subtitle: String,
    /// "February 27", or "A moveable feast".
    pub date: String,
    /// "Kept on" or "Commemorated on", set before a date of its own; empty for a date that moves.
    pub date_lead: String,
    /// For an observance kept by monastics and oblates only, what that means; else empty.
    pub monastic_note: String,
    /// For such an observance, the feast of the same saint everyone keeps, and "on March 21.".
    pub kept_by_all: Option<FeastLinkView>,
    pub kept_by_all_note: String,
    pub next: Option<NextKeepingView>,
    /// "The next eight years".
    pub years_heading: String,
    pub years: Vec<FeastYearView>,
    /// "From the Common of a Confessor and Bishop.", over the texts.
    pub texts_source: String,
    pub texts: Vec<FeastTextView>,
    pub whole_office: Option<WholeOfficeView>,
}

/// The fixed words of the search and the feast page.
#[derive(Clone, Debug, uniffi::Record)]
pub struct FeastWordsView {
    pub find_title: String,
    pub find_prompt: String,
    pub every_year: String,
    pub years_hint: String,
    pub days_hours: String,
    pub texts_heading: String,
    pub common_mark: String,
    pub whole_office: String,
    pub everyone_keeps: String,
    pub monastic_heading: String,
}

#[uniffi::export]
pub fn feast_words() -> FeastWordsView {
    let w = &FEAST_WORDS;
    FeastWordsView {
        find_title: w.find_title.into(),
        find_prompt: w.find_prompt.into(),
        every_year: w.every_year.into(),
        years_hint: w.years_hint.into(),
        days_hours: w.days_hours.into(),
        texts_heading: w.texts_heading.into(),
        common_mark: w.common_mark.into(),
        whole_office: w.whole_office.into(),
        everyone_keeps: w.everyone_keeps.into(),
        monastic_heading: w.monastic_heading.into(),
    }
}

fn year_view(k: &YearKeeping) -> FeastYearView {
    FeastYearView {
        year: k.year,
        date: k.date.map(CivilDate::from),
        date_label: k.date.map(row_date).unwrap_or_default(),
        label: keeping_label(&k.keeping).into(),
        kind: keeping_class(&k.keeping).into(),
        note: typeset(&keeping_note(&k.keeping)),
        color: k.color.as_str().into(),
    }
}

#[uniffi::export]
impl OfficeCore {
    /// The feasts whose names (or familiar names) hold `query`, each with how it is kept in
    /// `year` and the next. Builds two years' calendars, so call it off the main thread.
    pub fn find_feasts(&self, query: String, year: i32) -> Result<FindView, OfficeError> {
        let query: String = query.trim().chars().take(100).collect();
        let found = self.feasts.search(&query);
        let mut results = Vec::with_capacity(found.len().min(MAX_RESULTS));
        for e in found.iter().take(MAX_RESULTS) {
            let years = [year, year + 1].iter().map(|&y| self.keeping(y).map(|i| year_view(&i.keeping(e)))).collect::<Result<_, _>>()?;
            results.push(FeastResultView {
                id: e.id.clone(),
                name: typeset(&e.name),
                rank: e.rank.abbrev().into(),
                rank_full: e.rank.display_name().into(),
                rank_tone: rank_tone(e.rank).into(),
                meta: typeset(&result_meta(e)),
                years,
            });
        }
        Ok(FindView { subtitle: typeset(&search_years(year)), count: results_count(&query, found.len()), results })
    }

    /// The page of the feast `id` names, or none. `today` is the device's: the years begin with
    /// it while this year's keeping is still ahead.
    pub fn feast_page(&self, id: String, today: CivilDate) -> Result<Option<FeastPageView>, OfficeError> {
        let Some(entry) = is_feast_id(&id).then(|| self.feasts.get(&id)).flatten() else { return Ok(None) };
        let now = today.parse()?;
        let this_year = self.keeping(now.year())?.keeping(entry);
        let first = if this_year.date.is_some_and(|d| d < now) { now.year() + 1 } else { now.year() };
        let years: Vec<YearKeeping> =
            (first..first + FEAST_PAGE_YEARS).map(|y| self.keeping(y).map(|i| i.keeping(entry))).collect::<Result<_, _>>()?;
        let next = next_keeping(&years, now);
        // The texts as the office says them the first of these years the feast is kept, else
        // commemorated (the web's feast page).
        let said = texts_keeping(&years).and_then(|k| Some((k.date?, matches!(k.keeping, Keeping::Kept | Keeping::Transferred { .. }))));
        let texts = match said {
            Some((d, _)) => self.texts_on(entry, d)?,
            None => FeastTexts::default(),
        };
        let kept_by_all = self.feasts.kept_by_all(entry);
        Ok(Some(FeastPageView {
            id: entry.id.clone(),
            name: typeset(&entry.name),
            subtitle: typeset(&feast_subtitle(entry)),
            date: feast_date(entry),
            date_lead: date_lead(entry).into(),
            monastic_note: if entry.monastic.is_some() { typeset(&monastic_note(entry)) } else { String::new() },
            kept_by_all: kept_by_all.map(|e| FeastLinkView { id: e.id.clone(), name: typeset(&e.name) }),
            kept_by_all_note: kept_by_all.map(kept_by_all_note).unwrap_or_default(),
            next: next.and_then(|k| {
                let d = k.date?;
                Some(NextKeepingView {
                    label: next_label(&k.keeping).into(),
                    date: CivilDate::from(d),
                    date_label: long_date(d),
                    color: k.color.as_str().into(),
                })
            }),
            years_heading: YEARS_HEADING.into(),
            years: years.iter().map(year_view).collect(),
            texts_source: said.map(|(d, kept)| typeset(&texts_source(texts.proper, texts.common, d, kept))).unwrap_or_default(),
            texts: texts
                .texts
                .into_iter()
                .map(|t| FeastTextView {
                    label: t.label,
                    common: t.common,
                    blocks: render_blocks::text_blocks(&t.text, t.prayer).into_iter().map(BlockView::from).collect(),
                })
                .collect(),
            whole_office: next
                .filter(|k| matches!(k.keeping, Keeping::Kept | Keeping::Transferred { .. }))
                .and_then(|k| k.date)
                .map(|d| WholeOfficeView { label: whole_office_label(d), date: CivilDate::from(d) }),
        }))
    }
}

/// The feast pages reach this many years; a reader browsing past them starts afresh.
const MAX_KEEPING_YEARS: usize = 12;

impl OfficeCore {
    /// How every feast is kept in `year`: the calendar alone, with no hour composed.
    fn keeping(&self, year: i32) -> Result<Arc<YearIndex>, OfficeError> {
        if let Some(y) = self.keepings.lock().map_err(|e| failed(e.to_string()))?.get(&year) {
            return Ok(Arc::clone(y));
        }
        let built = Arc::new(YearIndex::new(year, calendar::build_calendar(year, &self.calendar).map_err(failed)?.days));
        let mut keepings = self.keepings.lock().map_err(|e| failed(e.to_string()))?;
        if keepings.len() >= MAX_KEEPING_YEARS {
            keepings.clear();
        }
        Ok(Arc::clone(keepings.entry(year).or_insert(built)))
    }

    /// The feast's texts as the office says them on `date`, with the Vespers of the evening before.
    fn texts_on(&self, entry: &FeastEntry, date: Date) -> Result<FeastTexts, OfficeError> {
        let eve = date.add_days(-1);
        let eve_year = self.year(eve.year())?;
        let year = if eve.year() == date.year() { Arc::clone(&eve_year) } else { self.year(date.year())? };
        let Some(day) = year.days.iter().find(|d| d.date == date) else { return Ok(FeastTexts::default()) };
        let eve = eve_year.days.iter().find(|d| d.date == eve).map(|day| DayOf { day, moveable: &eve_year.moveable });
        let is_it = |id: &str, name: &str| is_entry(entry, id, name);
        Ok(feast_texts(&self.engine, &is_it, eve, DayOf { day, moveable: &year.moveable }))
    }

    /// The page id of the feast a day keeps or commemorates as `id` and `name`, or empty.
    pub(crate) fn feast_id(&self, id: &str, name: &str, date: calendar::Date) -> String {
        self.feasts.find(id, name, date).map(|e| e.id.clone()).unwrap_or_default()
    }
}
