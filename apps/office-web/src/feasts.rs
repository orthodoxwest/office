//! Finding a feast: the ordo's search ("/calendar/find") and a page per feast ("/feast/{id}"),
//! both read from the calendar alone (presentation::feast) but for the feast page's texts.

use axum::body::Body;
use axum::http::{Response, StatusCode};
use std::sync::Arc;

use calendar::Date;
use office::feast::{DayOf, FeastTexts, feast_texts};
use presentation::feast::{
    FEAST_PAGE_YEARS, FeastEntry, Keeping, MAX_RESULTS, YEARS_HEADING, YearKeeping, date_lead, feast_date, feast_subtitle, is_entry,
    is_feast_id, is_usual, keeping_class, keeping_label, keeping_note, kept_by_all_note, monastic_note, next_keeping, next_label,
    rank_tone, result_meta, results_count, row_date, search_years, texts_keeping, texts_source, whole_office_label,
};
use presentation::{date_slug, long_date};
use render_html::links::{feast_link, find_link, home_from_feast};
use render_html::view::{Chrome, FeastPageData, FeastResult, FeastTextRow, FeastYearRow, FindData, LinkRow, NextKeeping};

use crate::Server;
use crate::handlers::{Req, html, render_failed};
use crate::web_time::now_in;

/// The longest query the search reads; a longer one is cut there.
const MAX_QUERY: usize = 100;

/// A year's row, leading to that day's page with the way back to the feast.
/// `usual` leaves its status unsaid, as the feast page does for a year kept as the feast usually is.
fn year_row(id: &str, k: &YearKeeping, usual: bool) -> FeastYearRow {
    FeastYearRow {
        year: k.year,
        date: k.date.map(row_date).unwrap_or_default(),
        href: k.date.map(|d| home_from_feast(&date_slug(d), id)).unwrap_or_default(),
        label: keeping_label(&k.keeping).into(),
        class: keeping_class(&k.keeping).into(),
        usual,
        note: keeping_note(&k.keeping),
        color: k.color.as_str().into(),
    }
}

impl Server {
    /// How `entry` is kept in each of `years`.
    fn keepings(&self, entry: &FeastEntry, years: impl Iterator<Item = i32>) -> Result<Vec<YearKeeping>, String> {
        years.map(|y| self.cache.keeping(y).map(|index| index.keeping(entry))).collect()
    }

    /// "/calendar/find?q=…": the feasts whose names hold the query, each with how it is kept
    /// this year and next.
    pub fn find(&self, req: &Req) -> Response<Body> {
        let raw = req.query.get("q").trim();
        let query: String = raw.chars().take(MAX_QUERY).collect();
        let now = now_in(&self.user_location(req)).0;
        let year = now.year();
        let found = self.cache.feasts().search(&query);
        let mut results = Vec::new();
        for e in found.iter().take(MAX_RESULTS) {
            let years = match self.keepings(e, [year, year + 1].into_iter()) {
                Ok(y) => y,
                Err(e) => return self.error_page(req, StatusCode::INTERNAL_SERVER_ERROR, &format!("error building calendar: {e}")),
            };
            results.push(FeastResult {
                name: e.name.clone(),
                href: feast_link(&e.id),
                rank: e.rank.abbrev().into(),
                rank_full: e.rank.display_name().into(),
                rank_class: match rank_tone(e.rank) {
                    "" => String::new(),
                    tone => format!("is-{tone}"),
                },
                meta: result_meta(e),
                years: years.iter().map(|k| year_row(&e.id, k, false)).collect(),
            });
        }
        let data = FindData {
            chrome: Chrome {
                page: "find".into(),
                nav_date: date_slug(now),
                site: req.site.into(),
                canonical: find_link(""),
                // Each search's results are one link from the page itself.
                noindex: !query.is_empty(),
                ..Chrome::default()
            },
            subtitle: search_years(year),
            count: results_count(&query, found.len()),
            query,
            results,
        };
        match self.pages.find(&data) {
            Ok(body) => html(StatusCode::OK, body),
            Err(e) => render_failed(&e),
        }
    }

    /// "/feast/{id}": a feast's rank and date, how it is kept in the next eight years, and its
    /// texts. "/feast" alone is the search.
    pub fn feast(&self, req: &Req) -> Response<Body> {
        let path = req.path.trim_matches('/');
        let id = path.strip_prefix("feast").unwrap_or(path).trim_start_matches('/');
        if id.is_empty() {
            return crate::http::redirect(&find_link(""), StatusCode::FOUND);
        }
        let Some(entry) = is_feast_id(id).then(|| self.cache.feasts().get(id)).flatten() else {
            return self.not_found_page(req);
        };
        let now = now_in(&self.user_location(req)).0;
        let failed = |e: String| self.error_page(req, StatusCode::INTERNAL_SERVER_ERROR, &format!("error building calendar: {e}"));
        // The years begin with this one while its keeping is still ahead.
        let first = match self.keepings(entry, std::iter::once(now.year())) {
            Ok(k) if k[0].date.is_some_and(|d| d < now) => now.year() + 1,
            Ok(_) => now.year(),
            Err(e) => return failed(e),
        };
        let years = match self.keepings(entry, first..first + FEAST_PAGE_YEARS) {
            Ok(y) => y,
            Err(e) => return failed(e),
        };
        let next = next_keeping(&years, now);
        // The texts as the office says them the first of these years the feast is kept, else
        // commemorated.
        let said = texts_keeping(&years).and_then(|k| Some((k.date?, matches!(k.keeping, Keeping::Kept | Keeping::Transferred { .. }))));
        let texts = match said {
            Some((d, _)) => match self.texts_on(entry, d) {
                Ok(t) => t,
                Err(e) => return failed(e),
            },
            None => FeastTexts::default(),
        };
        let kept_by_all = self.cache.feasts().kept_by_all(entry);
        let whole_office = next
            .filter(|k| matches!(k.keeping, Keeping::Kept | Keeping::Transferred { .. }))
            .and_then(|k| k.date)
            .map(|d| LinkRow { label: whole_office_label(d), href: home_from_feast(&date_slug(d), &entry.id) });
        let data = FeastPageData {
            chrome: Chrome {
                page: "feast".into(),
                nav_date: date_slug(now),
                site: req.site.into(),
                canonical: feast_link(&entry.id),
                ..Chrome::default()
            },
            name: entry.name.clone(),
            subtitle: feast_subtitle(entry),
            date: feast_date(entry),
            date_lead: date_lead(entry).into(),
            monastic_note: if entry.monastic.is_some() { monastic_note(entry) } else { String::new() },
            kept_by_all: kept_by_all.map(|e| LinkRow { label: e.name.clone(), href: feast_link(&e.id) }),
            kept_by_all_note: kept_by_all.map(kept_by_all_note).unwrap_or_default(),
            next: next.and_then(|k| {
                let d = k.date?;
                Some(NextKeeping {
                    label: next_label(&k.keeping).into(),
                    date: long_date(d),
                    href: home_from_feast(&date_slug(d), &entry.id),
                    color: k.color.as_str().into(),
                })
            }),
            years_heading: YEARS_HEADING.into(),
            years: years.iter().map(|k| year_row(&entry.id, k, is_usual(entry, &k.keeping))).collect(),
            texts_source: said.map(|(d, kept)| texts_source(texts.proper, texts.common, d, kept)).unwrap_or_default(),
            texts: texts
                .texts
                .into_iter()
                .map(|t| FeastTextRow { label: t.label, text: t.text, common: t.common, prayer: t.prayer })
                .collect(),
            whole_office,
        };
        match self.pages.feast(&data) {
            Ok(body) => html(StatusCode::OK, body),
            Err(e) => render_failed(&e),
        }
    }

    /// The feast's texts as the office says them on `date`, with the Vespers of the evening before.
    fn texts_on(&self, entry: &FeastEntry, date: Date) -> Result<FeastTexts, String> {
        let year = self.cache.get(date.year())?;
        let eve = date.add_days(-1);
        let eve_year = if eve.year() == date.year() { Arc::clone(&year) } else { self.cache.get(eve.year())? };
        let Some(day) = year.days.iter().find(|d| d.date == date) else { return Ok(FeastTexts::default()) };
        let eve = eve_year.days.iter().find(|d| d.date == eve).map(|day| DayOf { day, moveable: &eve_year.moveable });
        let is_it = |id: &str, name: &str| is_entry(entry, id, name);
        Ok(feast_texts(&self.engine, &is_it, eve, DayOf { day, moveable: &year.moveable }))
    }
}

#[cfg(test)]
mod tests {
    use axum::http::{HeaderMap, Uri, header};

    use super::*;
    use crate::test_server;

    fn get(path: &str) -> (StatusCode, HeaderMap, String) {
        use tower::ServiceExt;
        let runtime = tokio::runtime::Builder::new_current_thread().build().unwrap();
        let request = axum::extract::Request::builder().uri(path.parse::<Uri>().unwrap()).body(Body::empty()).unwrap();
        let resp = runtime.block_on(test_server().router().oneshot(request)).unwrap();
        let (parts, body) = resp.into_parts();
        let bytes = runtime.block_on(axum::body::to_bytes(body, usize::MAX)).unwrap();
        (parts.status, parts.headers, String::from_utf8(bytes.to_vec()).unwrap().replace("&#x2f;", "/"))
    }

    fn page(path: &str) -> String {
        let (status, _, body) = get(path);
        assert_eq!(status, StatusCode::OK, "{path}");
        body
    }

    #[test]
    fn search_lists_each_match_with_this_year_and_next() {
        let body = page("/calendar/find?q=raphael");
        assert_eq!(body.matches(r#"<article class="find-feast">"#).count(), 3);
        let brooklyn = body.find(r#"href="/feast/st-raphael-of-brooklyn""#).unwrap();
        assert!(brooklyn < body.find(r#"href="/feast/st-raphael-archangel""#).unwrap(), "the best match leads");
        assert!(body.contains(r#"<p class="find-count" role="status">3 feasts</p>"#));
        assert!(body.contains(r#"value="raphael""#));
        assert!(body.contains(r#"<meta name="robots" content="noindex">"#), "a search's results stay out of search engines");
        assert_eq!(body.matches(r#"class="find-year "#).count(), 6, "two years for each feast");
        assert!(body.contains("&amp;from=st-raphael-of-brooklyn"), "each year leads to its day, with the way back");
    }

    #[test]
    fn search_before_a_query_and_without_a_match() {
        let body = page("/calendar/find");
        assert!(!body.contains("find-count") && !body.contains("noindex"));
        assert!(body.contains(" autofocus"));
        let none = page("/calendar/find?q=zzzzzz");
        assert!(none.contains("No feast or saint by that name"));
        assert!(page("/calendar/find?q=Candlemas").contains(r#"href="/feast/purification-bvm""#));
        assert_eq!(get("/calendar/find/extra").0, StatusCode::NOT_FOUND);
        let long = format!("/calendar/find?q={}", "a".repeat(5000));
        assert_eq!(get(&long).0, StatusCode::OK);
    }

    #[test]
    fn a_feast_page_gives_its_years_and_its_texts() {
        let body = page("/feast/st-raphael-of-brooklyn");
        assert!(body.contains("<h1>St Raphael of Brooklyn</h1>"));
        assert!(body.contains("Greater Double · White<"));
        assert!(body.contains(">The next eight years</h2>"));
        assert_eq!(body.matches(r#"class="find-year "#).count(), FEAST_PAGE_YEARS as usize);
        assert!(body.contains("From the Common of a Confessor and Bishop."));
        // Septuagesima's I Vespers take his evening, where he is commemorated.
        for label in ["At I Vespers · Magnificat", "At Lauds · Benedictus", "At II Vespers · Antiphon", "Collect"] {
            assert!(body.contains(&format!("<h3>{label} <span")), "{label}");
        }
        assert!(body.contains("blessed Raphael, thy Confessor and Bishop") && body.contains("world without end"));
        assert!(body.contains(r#"class="feast-next""#), "a feast kept every few years has a next keeping");
        let collect = body.find("<h3>Collect").unwrap();
        assert!(collect < body.find("<h3>At I Vespers").unwrap(), "the collect, the day's, comes before the texts of single hours");
        assert!(collect < body.find(">The next eight years</h2>").unwrap(), "the texts come before the years");
        // A year kept as usual leaves "Kept" unsaid; a year he is only commemorated says so.
        assert!(!body.contains("<b>Kept</b>") && body.contains("<b>Commemorated</b>"));
        assert!(body.contains(r#"<link rel="canonical" href="#) && !body.contains("noindex"));
    }

    #[test]
    fn a_monastic_observance_says_so_and_leads_to_the_feast_everyone_keeps() {
        let body = page("/feast/solemnity-st-benedict");
        assert!(body.contains("Monastics &amp; Oblates only"));
        assert!(body.contains("Monastic Diurnal pp. 560–564"));
        assert!(body.contains(r#"Everyone keeps <a href="/feast/st-benedict">"#));
        assert!(!body.contains("Proper of the feast"), "its office is in the Monastic Diurnal, not this app");
        // Its years say what the parish keeps, not "Monastics & Oblates" eight times over.
        assert!(body.contains("the parish keeps") && !body.contains("<b>Monastics &amp; Oblates</b>"));
        let everyone = body.find("Everyone keeps").unwrap();
        assert!(everyone < body.find("The ordo keeps this observance").unwrap(), "the feast everyone keeps leads");
    }

    #[test]
    fn feast_addresses() {
        assert_eq!(get("/feast/no-such-feast").0, StatusCode::NOT_FOUND);
        assert_eq!(get("/feast/St-Mark").0, StatusCode::NOT_FOUND);
        assert_eq!(get("/feast/st-mark/extra").0, StatusCode::NOT_FOUND);
        let (status, headers, _) = get("/feast");
        assert_eq!((status, headers[header::LOCATION].to_str().unwrap()), (StatusCode::FOUND, "/calendar/find"));
    }

    #[test]
    fn days_lead_to_their_feasts_and_back() {
        let home = page("/?date=2026-02-27&from=st-raphael-of-brooklyn");
        assert!(home.contains(r#"<a class="from-feast" href="/feast/st-raphael-of-brooklyn">&lsaquo; St Raphael of Brooklyn</a>"#));
        assert!(home.contains(r#"<a class="feast-link" href="/feast/st-raphael-of-brooklyn">"#), "the day's commemoration leads to it");
        assert!(!page("/?date=2026-02-27&from=..%2Fnope").contains("from-feast"));
        assert!(
            page("/?date=2027-02-27").contains(r#"<a class="feast-link" href="/feast/st-raphael-of-brooklyn" title="About this feast">"#)
        );
        let ordo = page("/calendar/2026/02");
        assert!(ordo.contains(r#"action="/calendar/find""#));
        assert!(ordo.contains(r#"<a class="feast-link" href="/feast/st-raphael-of-brooklyn">"#));
    }

    #[test]
    fn the_sitemap_names_every_feast() {
        let body = page("/sitemap.xml");
        assert!(body.contains("/calendar/find</loc>") && body.contains("/feast/st-raphael-of-brooklyn</loc>"));
    }
}
