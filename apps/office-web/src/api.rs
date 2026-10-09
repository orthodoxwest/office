//! The public read-only JSON API: the ordo's month as data, for other sites
//! to lay out in their own way. Paths carry a version, `/api/v1/…`, and a field
//! once published keeps its name and meaning within that version.

use axum::body::Body;
use axum::http::{Response, StatusCode, header};
use render_html::links::{calendar_link, hour_link};
use render_html::view::{CommemorationRow, DayRow};
use serde::Serialize;

use crate::Server;
use crate::handlers::{ORDERED_HOURS, Req};
use crate::http::{response, set};

/// `GET /api/v1/ordo/{year}/{month}`.
#[derive(Debug, Serialize)]
struct MonthBody {
    year: i32,
    month: u32,
    days: Vec<ApiDay>,
}

/// One day of the ordo.
#[derive(Debug, Serialize)]
struct ApiDay {
    /// "2026-10-09".
    date: String,
    /// "Friday".
    weekday: String,
    /// The day's office: "St Denis and Companions, Martyrs".
    name: String,
    /// The ordo's abbreviation ("D", "Sd", "Fer") and its full name.
    rank: String,
    rank_name: String,
    /// The liturgical colour: "white", "red", "green", "violet", "rose" or "black".
    color: String,
    fast: bool,
    abstinence: bool,
    /// Feasts commemorated on the day, by name.
    commemorations: Vec<String>,
    /// Observances the ordo brackets "(Monastics & Oblates Only)".
    monastic: Vec<ApiMonastic>,
    lauds: ApiMajorHour,
    vespers: ApiMajorHour,
    /// The preces at Prime and the Little Hours.
    hours_preces: bool,
    /// Pages on this site for the day: the ordo row and each hour.
    links: ApiLinks,
}

#[derive(Debug, Serialize)]
struct ApiMonastic {
    name: String,
    rank: String,
    rank_name: String,
    /// Where its office is found, or empty.
    office: String,
}

/// Lauds or Vespers as the ordo digests it.
#[derive(Debug, Serialize)]
struct ApiMajorHour {
    /// The Benedictus or Magnificat antiphon's incipit.
    gospel_antiphon: String,
    preces: bool,
    suffrage: bool,
    commemorations: Vec<ApiCommemoration>,
    /// The ordo's precedence note ("Vespers of the following"), or empty. Vespers only.
    #[serde(skip_serializing_if = "String::is_empty")]
    note: String,
}

#[derive(Debug, Serialize)]
struct ApiCommemoration {
    name: String,
    /// The commemoration antiphon's incipit.
    incipit: String,
}

#[derive(Debug, Serialize)]
struct ApiLinks {
    calendar: String,
    lauds: String,
    prime: String,
    terce: String,
    sext: String,
    none: String,
    vespers: String,
    compline: String,
}

fn comms(rows: &[CommemorationRow]) -> Vec<ApiCommemoration> {
    rows.iter().map(|c| ApiCommemoration { name: c.name.clone(), incipit: c.incipit.clone() }).collect()
}

fn api_day(row: &DayRow, site: &str) -> ApiDay {
    let date = row.date_slug.clone();
    let weekday = calendar::Date::parse(&date).map(|d| d.weekday().name().to_string()).unwrap_or_default();
    let at = |slug: &str| format!("{site}{}", hour_link(slug, &date));
    let [lauds, prime, terce, sext, none, vespers, compline] = ORDERED_HOURS.map(|(_, slug)| at(slug));
    ApiDay {
        weekday,
        name: row.feast_name.clone(),
        rank: row.rank.clone(),
        rank_name: row.rank_full.clone(),
        color: row.color.clone(),
        fast: row.fast,
        abstinence: row.abstinence,
        commemorations: row.commemorations.clone(),
        monastic: row
            .monastic
            .iter()
            .map(|m| ApiMonastic {
                name: m.heading.clone(),
                rank: m.rank.clone(),
                rank_name: m.rank_full.clone(),
                office: m.office.clone(),
            })
            .collect(),
        lauds: ApiMajorHour {
            gospel_antiphon: row.benedictus_antiphon.clone(),
            preces: row.lauds_preces,
            suffrage: row.lauds_suffrage,
            commemorations: comms(&row.lauds_comms),
            note: String::new(),
        },
        vespers: ApiMajorHour {
            gospel_antiphon: row.magnificat_antiphon.clone(),
            preces: row.vespers_preces,
            suffrage: row.vespers_suffrage,
            commemorations: comms(&row.vespers_comms),
            note: row.vespers_note.clone(),
        },
        hours_preces: row.hours_preces,
        links: ApiLinks { calendar: format!("{site}{}", calendar_link(&date)), lauds, prime, terce, sext, none, vespers, compline },
        date,
    }
}

/// A JSON reply any site may read from its pages. The ordo for a date changes
/// only when a correction is deployed, so caches may hold it for an hour.
fn json(status: StatusCode, body: String) -> Response<Body> {
    let mut resp = response(status, body);
    set(&mut resp, header::CONTENT_TYPE, "application/json; charset=utf-8");
    set(&mut resp, header::ACCESS_CONTROL_ALLOW_ORIGIN, "*");
    set(&mut resp, header::CACHE_CONTROL, if status == StatusCode::OK { "public, max-age=3600" } else { "no-cache" });
    resp
}

fn error(status: StatusCode, message: &str) -> Response<Body> {
    json(status, serde_json::json!({ "error": message }).to_string())
}

impl Server {
    pub(crate) fn api(&self, req: &Req) -> Response<Body> {
        let parts: Vec<&str> = req.path.trim_matches('/').split('/').collect();
        let ["api", "v1", "ordo", year, month] = parts[..] else {
            return error(StatusCode::NOT_FOUND, "No such endpoint. Try /api/v1/ordo/YYYY/MM.");
        };
        let year = match year.parse::<i32>() {
            Ok(y) if year.len() == 4 && (1..=9999).contains(&y) => y,
            _ => return error(StatusCode::NOT_FOUND, "The year must be four digits."),
        };
        let month = match month.parse::<u32>() {
            Ok(m) if month.len() == 2 && (1..=12).contains(&m) => m,
            _ => return error(StatusCode::NOT_FOUND, "The month must be two digits, 01 to 12."),
        };
        let data = match self.cache.month(year, month, &self.engine) {
            Ok(m) => m,
            Err(e) => return error(StatusCode::INTERNAL_SERVER_ERROR, &format!("error building calendar: {e}")),
        };
        let body = MonthBody { year, month, days: data.days.iter().map(|d| api_day(d, req.site)).collect() };
        match serde_json::to_string(&body) {
            Ok(s) => json(StatusCode::OK, s),
            Err(e) => error(StatusCode::INTERNAL_SERVER_ERROR, &e.to_string()),
        }
    }
}

#[cfg(test)]
mod tests {
    use axum::http::{HeaderMap, Uri};
    use serde_json::Value;
    use tower::ServiceExt;

    use super::*;
    use crate::test_server;

    fn get(path: &str) -> (StatusCode, HeaderMap, Value) {
        let runtime = tokio::runtime::Builder::new_current_thread().build().unwrap();
        let request = axum::extract::Request::builder().uri(path.parse::<Uri>().unwrap()).body(Body::empty()).unwrap();
        let resp = runtime.block_on(test_server().router().oneshot(request)).unwrap();
        let (parts, body) = resp.into_parts();
        let bytes = runtime.block_on(axum::body::to_bytes(body, usize::MAX)).unwrap();
        (parts.status, parts.headers, serde_json::from_slice(&bytes).unwrap())
    }

    /// A month carries every day, each with its office and links back to the site.
    #[test]
    fn month_lists_each_day() {
        let (status, headers, body) = get("/api/v1/ordo/2026/12");
        assert_eq!(status, StatusCode::OK);
        assert_eq!(headers[header::ACCESS_CONTROL_ALLOW_ORIGIN], "*");
        assert!(headers[header::CONTENT_TYPE].to_str().unwrap().starts_with("application/json"));
        let days = body["days"].as_array().unwrap();
        assert_eq!(days.len(), 31);
        let christmas = &days[24];
        assert_eq!(christmas["date"], "2026-12-25");
        assert_eq!(christmas["weekday"], "Friday");
        assert!(christmas["name"].as_str().unwrap().contains("Nativity"), "{christmas}");
        assert_eq!(christmas["color"], "white");
        assert!(!christmas["lauds"]["gospel_antiphon"].as_str().unwrap().is_empty());
        assert!(christmas["links"]["vespers"].as_str().unwrap().ends_with("/vespers/2026-12-25"));
    }

    #[test]
    fn rejects_malformed_paths() {
        for path in ["/api/v1/ordo/2026/13", "/api/v1/ordo/2026/9", "/api/v1/ordo/26/09", "/api/v1/ordo/2026", "/api/v2/ordo/2026/09"] {
            let (status, headers, body) = get(path);
            assert_eq!(status, StatusCode::NOT_FOUND, "{path}");
            assert_eq!(headers[header::ACCESS_CONTROL_ALLOW_ORIGIN], "*", "{path}");
            assert!(body["error"].is_string(), "{path}");
        }
    }
}
