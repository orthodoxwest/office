//! What search engines read before the pages: `robots.txt` and the sitemap.
//!
//! The sitemap lists the pages worth finding, all of them undated but the ordo: today's day and
//! hours, this year's ordo, reminders, privacy and about. Each dated day and hour is one link from those and carries
//! `noindex` (see `Chrome::noindex`), so the endless run of dates stays out of search results.

use axum::body::Body;
use axum::http::{Response, StatusCode, header};

use render_html::links::calendar_year_link;

use crate::http::{response, set};

/// The undated pages; the ordo's year is added per request.
const PAGES: [&str; 11] =
    ["/", "/lauds", "/prime", "/terce", "/sext", "/none", "/vespers", "/compline", "/reminders", "/privacy", "/about"];

/// Crawl everything a reader sees; skip the usage report, the beacon and the reminder feed.
pub fn robots(site: &str) -> Response<Body> {
    let body = format!("User-agent: *\nDisallow: /admin/\nDisallow: /api/\nDisallow: /office.ics\n\nSitemap: {site}/sitemap.xml\n");
    text(body, "text/plain; charset=utf-8")
}

/// `/calendar` itself only redirects to the month, so the sitemap names `year`'s ordo.
pub fn sitemap(site: &str, year: i32) -> Response<Body> {
    let site = site.replace('&', "&amp;").replace('<', "&lt;").replace('>', "&gt;");
    let mut body =
        String::from("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<urlset xmlns=\"http://www.sitemaps.org/schemas/sitemap/0.9\">\n");
    for page in PAGES.iter().map(|p| p.to_string()).chain([calendar_year_link(year)]) {
        body.push_str(&format!("  <url><loc>{site}{page}</loc></url>\n"));
    }
    body.push_str("</urlset>\n");
    text(body, "application/xml; charset=utf-8")
}

fn text(body: String, content_type: &str) -> Response<Body> {
    let mut resp = response(StatusCode::OK, body);
    set(&mut resp, header::CONTENT_TYPE, content_type);
    set(&mut resp, header::CACHE_CONTROL, "public, max-age=86400");
    resp
}
