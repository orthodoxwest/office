//! Finding a feast: the ordo's search ("/calendar/find") and a page per feast ("/feast/{id}"),
//! both read from the calendar alone (presentation::feast) but for the feast page's texts.

use axum::body::Body;
use axum::http::{Response, StatusCode};
use calendar::{Date, Season};
use office::feast::feast_texts;
use presentation::feast::{
    FEAST_PAGE_YEARS, FeastEntry, Keeping, YEARS_HEADING, YearKeeping, feast_date, feast_subtitle, is_feast_id, keeping_class,
    keeping_label, keeping_note, kept_by_all_note, monastic_note, next_keeping, next_label, rank_tone, result_meta, results_count,
    row_date, search_years, texts_source, whole_office_label,
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
fn year_row(id: &str, k: &YearKeeping) -> FeastYearRow {
    FeastYearRow {
        year: k.year,
        date: k.date.map(row_date).unwrap_or_default(),
        href: k.date.map(|d| home_from_feast(&date_slug(d), id)).unwrap_or_default(),
        label: keeping_label(&k.keeping).into(),
        class: keeping_class(&k.keeping).into(),
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
        let mut results = Vec::new();
        for e in self.cache.feasts().search(&query) {
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
                years: years.iter().map(|k| year_row(&e.id, k)).collect(),
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
            count: if query.is_empty() { String::new() } else { results_count(results.len()) },
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
        // The texts as said on the day it is next kept, in that day's season; a feast never kept
        // in these years takes the season of its date in the first of them.
        let season = match next.and_then(|k| k.date).or_else(|| years.iter().find_map(|k| k.date)) {
            Some(d) => match self.season_of(d) {
                Ok(s) => s,
                Err(e) => return failed(e),
            },
            None => None,
        };
        let season = season.unwrap_or(Season::Pentecost);
        let texts = match (&entry.monastic, self.cache.data().feasts.iter().find(|f| f.id == entry.id)) {
            (None, Some(feast)) => feast_texts(feast, season, &self.engine.texts),
            _ => Default::default(),
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
            years: years.iter().map(|k| year_row(&entry.id, k)).collect(),
            texts_source: texts_source(entry.category, texts.proper, texts.common),
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

    fn season_of(&self, date: Date) -> Result<Option<Season>, String> {
        let index = self.cache.keeping(date.year())?;
        Ok(index.day(date).map(|d| d.season))
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
        assert!(body.contains("Greater Double · White · Bishop &amp; Confessor"));
        assert!(body.contains(">The next eight years</h2>"));
        assert_eq!(body.matches(r#"class="find-year "#).count(), FEAST_PAGE_YEARS as usize);
        assert!(body.contains("From the Common of a Confessor and Bishop."));
        for label in ["At I Vespers · Magnificat", "At Lauds · Benedictus", "At II Vespers · Magnificat", "Collect"] {
            assert!(body.contains(&format!("<h3>{label} <span")), "{label}");
        }
        assert!(body.contains("blessed Raphael, thy Confessor and Bishop") && body.contains("world without end"));
        assert!(body.contains(r#"class="feast-next "#), "a feast kept every few years has a next keeping");
        assert!(body.contains(r#"<link rel="canonical" href="#) && !body.contains("noindex"));
    }

    #[test]
    fn a_monastic_observance_says_so_and_leads_to_the_feast_everyone_keeps() {
        let body = page("/feast/solemnity-st-benedict");
        assert!(body.contains("Monastics &amp; Oblates only"));
        assert!(body.contains("Monastic Diurnal pp. 560–564"));
        assert!(body.contains(r#"Everyone keeps <a href="/feast/st-benedict">"#));
        assert!(!body.contains("Proper of the feast"), "its office is in the Monastic Diurnal, not this app");
        assert!(body.contains("is-monastic"));
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
