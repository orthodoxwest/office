//! Page templates with native MiniJinja HTML autoescaping and explicit URL filters
//! (see [`crate::escape`]).

use minijinja::value::Value;
use minijinja::{AutoEscape, Environment};

use crate::escape::{url_norm, url_part, url_start};
use crate::html::{render_section_heading, typeset};
use crate::leader::leader_sections;
use crate::links::{home_link, hour_link, nav_link};
use crate::usage::UsageData;
use crate::view::{CalendarData, ErrorData, HomeData, HourData, NotFoundData, RemindersData};
use presentation::{season_label, title_case};

const TEMPLATES: [(&str, &str); 9] = [
    ("layout.html", include_str!("../templates/layout.html")),
    ("macros.html", include_str!("../templates/macros.html")),
    ("home.html", include_str!("../templates/home.html")),
    ("hour.html", include_str!("../templates/hour.html")),
    ("calendar.html", include_str!("../templates/calendar.html")),
    ("reminders.html", include_str!("../templates/reminders.html")),
    ("404.html", include_str!("../templates/404.html")),
    ("error.html", include_str!("../templates/error.html")),
    ("usage.html", include_str!("../templates/usage.html")),
];

/// The parsed page templates, shareable for the life of the process.
pub struct Pages {
    env: Environment<'static>,
}

impl Pages {
    /// Parses the embedded templates. `asset_url` maps a static file name to
    /// its stamped URL, so a deploy that changes CSS or JS produces new URLs
    /// and one that leaves them alone keeps them cached.
    pub fn new(asset_url: impl Fn(&str) -> String + Send + Sync + 'static) -> Result<Pages, String> {
        let mut env = Environment::new();
        env.set_keep_trailing_newline(true);
        // Native HTML autoescaping also preserves trusted macro and fragment output.
        env.set_auto_escape_callback(|_| AutoEscape::Html);
        env.set_undefined_behavior(minijinja::UndefinedBehavior::Strict);
        for (name, source) in TEMPLATES {
            env.add_template(name, source).map_err(|e| format!("parsing {name} template: {e}"))?;
        }
        env.add_filter("url", |s: String| url_start(&s));
        env.add_filter("urlnorm", |s: String| url_norm(&s));
        env.add_filter("urlpart", |s: String| url_part(&s));
        env.add_filter("typeset", |s: String| typeset(&s));
        env.add_filter("titlecase", |s: String| title_case(&s));
        env.add_filter("season_label", |s: String| season_label(&s));
        env.add_function("nav_link", |base: String, date: String| nav_link(&base, &date));
        env.add_function("home_link", |date: String| home_link(&date));
        env.add_function("hour_link", |hour: String, date: String| hour_link(&hour, &date));
        env.add_function("static", move |name: String| asset_url(&name));
        env.add_function("section_heading", |label: String| Value::from_safe_string(render_section_heading(&label)));
        Ok(Pages { env })
    }

    fn render<T: serde::Serialize>(&self, template: &str, data: &T) -> Result<String, String> {
        let tmpl = self.env.get_template(template).map_err(|e| e.to_string())?;
        tmpl.render(Value::from_serialize(data)).map_err(|e| format!("rendering {template}: {e:#}"))
    }

    pub fn home(&self, data: &HomeData) -> Result<String, String> {
        self.render("home.html", data)
    }

    /// A composed hour. With leader forms, the forms' sections are aligned
    /// into one page.
    /// `martyrology` is the same hour composed with the Martyrology read at Prime, or empty when
    /// there is none to read: a section it changes carries both readings, and the reader's
    /// setting shows one of them, so the page itself never depends on the setting.
    pub fn hour(
        &self,
        data: &mut HourData,
        forms: &[(liturgy::PrayerForm, &liturgy::OfficeHour)],
        martyrology: &[(liturgy::PrayerForm, &liturgy::OfficeHour)],
    ) -> Result<String, String> {
        if !data.leader_forms.is_empty() {
            let mut sections = leader_sections(forms)?;
            if !martyrology.is_empty() {
                let read = leader_sections(martyrology)?;
                if read.len() != sections.len() {
                    return Err("martyrology section mismatch".into());
                }
                for (section, read) in sections.iter_mut().zip(read) {
                    if read.label != section.label || read.collapsible != section.collapsible {
                        return Err("martyrology section mismatch".into());
                    }
                    if read.html != section.html {
                        section.martyrology_html = read.html;
                    }
                }
            }
            data.leader_sections = sections;
        }
        self.render("hour.html", data)
    }

    pub fn calendar(&self, data: &CalendarData) -> Result<String, String> {
        self.render("calendar.html", data)
    }

    pub fn reminders(&self, data: &RemindersData) -> Result<String, String> {
        self.render("reminders.html", data)
    }

    pub fn not_found(&self, data: &NotFoundData) -> Result<String, String> {
        self.render("404.html", data)
    }

    pub fn error_page(&self, data: &ErrorData) -> Result<String, String> {
        self.render("error.html", data)
    }

    pub fn usage(&self, data: &UsageData) -> Result<String, String> {
        self.render("usage.html", data)
    }
}

// Page markup and seasonal presentation contracts.
#[cfg(test)]
mod tests {
    use super::{Pages, TEMPLATES};
    use crate::links::static_url;
    use crate::view::{CalendarData, Chrome, HomeData};

    fn source(name: &str) -> &'static str {
        TEMPLATES.iter().find(|(n, _)| *n == name).map(|(_, s)| *s).unwrap()
    }

    #[track_caller]
    fn has_all(body: &str, wants: &[&str]) {
        for want in wants {
            assert!(body.contains(want), "template is missing {want:?}");
        }
    }

    fn at(body: &str, needle: &str) -> usize {
        body.find(needle).unwrap_or_else(|| panic!("missing {needle:?}"))
    }

    #[test]
    fn templates_escape_values_and_preserve_explicit_markup() {
        let mut pages = Pages::new(|name| static_url(name, "test")).unwrap();
        pages
            .env
            .add_template("boundary.html", r#"<p title="{{ text }}">{{ text }}</p><a href="{{ link|url }}">Link</a>{{ markup|safe }}"#)
            .unwrap();
        let html = pages
            .env
            .get_template("boundary.html")
            .unwrap()
            .render(minijinja::context! {
                text => r#"" onmouseover="alert(1)"><script>alert(2)</script> + & café"#,
                link => "javascript:alert(3)",
                markup => "<strong>Trusted &amp; text</strong>",
            })
            .unwrap();
        assert!(html.contains(r#"title="&quot; onmouseover=&quot;alert(1)&quot;&gt;&lt;script&gt;"#));
        assert!(html.contains(" + &amp; café"));
        assert!(!html.contains("<script>") && !html.contains("javascript:"));
        assert!(html.contains(r##"href="#invalid-url""##));
        assert!(html.ends_with("<strong>Trusted &amp; text</strong>"));
        assert!(pages.env.render_str("{{ missing }}", ()).is_err());
    }

    #[test]
    fn layout_includes_stamped_navigation_and_assets() {
        has_all(
            source("layout.html"),
            &[
                r#"static("style.css")"#,
                r#"static("app.js")"#,
                r#"data-nav="home""#,
                r#"data-nav="hour""#,
                r#"data-hour="lauds""#,
                r#"data-nav="calendar""#,
            ],
        );
    }

    #[test]
    fn layout_includes_text_size_control() {
        let body = source("layout.html");
        // Preferences render once at the foot of the phone menu and once in the
        // wide header's Settings disclosure; CSS shows one copy per width, and
        // no page ends in them.
        has_all(
            body,
            &[
                r#"import prefs"#,
                r#"class="site-prefs menu-prefs""#,
                r#"<details class="site-settings">"#,
                r#"class="site-prefs settings-prefs""#,
            ],
        );
        assert!(at(body, "menu-prefs") < at(body, "</details>"), "phone preferences sit inside the site menu");
        assert!(at(body, "settings-prefs") < at(body, "</header>"), "wide preferences sit in the header");
        assert!(!body[at(body, "<footer>")..].contains("prefs()"), "the footer carries no preferences");
        has_all(
            source("macros.html"),
            &[
                r#"class="theme-switch" role="group" aria-label="Appearance""#,
                r#"class="text-size-switch" role="group" aria-label="Text size""#,
                r#"data-text-size-choice="small""#,
                r#"data-text-size-choice="default""#,
                r#"data-text-size-choice="large""#,
                r#"aria-label="Smaller text""#,
                r#"aria-label="Default text size""#,
                r#"aria-label="Larger text""#,
                r#"title="Smaller text""#,
                r#"title="Larger text""#,
                r#"aria-pressed="false""#,
            ],
        );
        has_all(
            body,
            &[
                r#"localStorage.getItem("office-text-size")"#,
                r#"setAttribute("data-text-size", s)"#,
                r#"removeAttribute("data-text-size")"#,
            ],
        );
        // The pre-paint script runs before the stylesheet, or the size flashes.
        assert!(at(body, r#"localStorage.getItem("office-text-size")"#) < at(body, r#"static("style.css")"#));
        assert!(!body.contains("text-size=") || body.contains("data-text-size="), "text size is never stamped onto URLs");
    }

    #[test]
    fn reminders_expose_quiet_subscription_actions() {
        let body = source("reminders.html");
        has_all(
            body,
            &[
                r#"data-reminder-hour="{{ h.slug }}""#,
                r#"id="reminder-webcal" class="reminder-subscribe""#,
                "Subscribe in calendar app",
                r#"id="reminder-copy" class="reminder-copy""#,
                r#"role="status" aria-live="polite""#,
                r#"class="reminder-address""#,
                r#"class="reminder-help""#,
            ],
        );
        assert!(!body.contains(r#"id="reminder-copy" class="date-submit""#));
    }

    #[test]
    fn home_groups_hours_without_breaking_client_selectors() {
        let body = source("home.html");
        has_all(
            body,
            &[
                r#"class="home-hero day-color-{{ color }}""#,
                r#"class="home-prayer-card""#,
                r#"class="pray-now""#,
                r#"class="home-hour-links""#,
                "home-hour-group-morning",
                "home-hour-group-day",
                "home-hour-group-evening",
                r#"id="home-hours-morning">"#,
                "<span>Morning</span>",
                r#"id="home-hours-day">"#,
                "<span>Day</span>",
                r#"id="home-hours-evening">"#,
                "<span>Evening</span>",
                r#"data-hour="{{ h.slug }}""#,
                r#"aria-current="time""#,
                "home-hour-link-name",
                r#"class="not-today-notice""#,
                "Go to today",
            ],
        );
        let (day, prayer, meta) =
            (at(body, r#"class="home-summary"#), at(body, r#"class="home-prayer-card""#), at(body, r#"class="home-day-meta""#));
        assert!(day < prayer && prayer < meta, "day identity, prayer invitation, then date control");
        let (notice, date_nav) = (at(body, r#"class="not-today-notice""#), at(body, r#"class="hour-date-nav home-date-nav""#));
        assert!(notice < prayer && notice < date_nav, "the not-today notice leads the day");
    }

    #[test]
    fn layout_brand_is_today_home() {
        let body = source("layout.html");
        // The brand must not carry the page's date, which pins an overnight
        // app to yesterday.
        has_all(body, &[r#"data-nav-home="today""#, r#"nav_link("/", "")"#, "not show_today"]);
        assert!(!body.contains(r#"nav_link("/", d)"#));
        assert!(!body.contains(r#"{% if p == "home" %} active{% endif %}"#), "not every home page is current");
    }

    #[test]
    fn hour_not_today_notice_is_outside_date_nav() {
        let body = source("hour.html");
        has_all(body, &[r#"class="not-today-notice""#, "Go to today"]);
        assert!(at(body, r#"class="not-today-notice""#) < at(body, r#"class="hour-date-nav""#));
    }

    #[test]
    fn calendar_fish_uses_sprite() {
        // The fish instance is a macro in macros.html.
        assert!(source("calendar.html").contains(r#"id="icon-fish""#), "the calendar defines one fish symbol");
        assert!(source("macros.html").contains(r##"<use href="#icon-fish"/>"##), "fish instances use the symbol");
        let paths: usize = TEMPLATES.iter().map(|(_, s)| s.matches("M1 6 C5 1.2").count()).sum();
        assert_eq!(paths, 1, "the fish path is defined once");
    }

    #[test]
    fn layout_stamps_season_class_on_body() {
        let pages = Pages::new(|name| static_url(name, "test")).unwrap();
        for (class, want) in [
            ("season-passiontide", r#"<body class="page-home season-passiontide">"#),
            ("season-eastertide", r#"<body class="page-home season-eastertide">"#),
            // No stray trailing space without a season class.
            ("", r#"<body class="page-home">"#),
        ] {
            let data = HomeData {
                chrome: Chrome { page: "home".into(), season_class: class.into(), ..Chrome::default() },
                date_str: "Sunday, March 22, 2026".into(),
                date_slug: "2026-03-22".into(),
                ..HomeData::default()
            };
            let html = pages.home(&data).unwrap();
            assert!(html.contains(want), "home body tag is not {want:?}");
        }
    }

    /// The year calendar spans every season, so it is never tinted.
    #[test]
    fn calendar_page_stays_season_neutral() {
        let pages = Pages::new(|name| static_url(name, "test")).unwrap();
        let data = CalendarData { chrome: Chrome { page: "calendar".into(), ..Chrome::default() }, year: 2026, ..CalendarData::default() };
        let html = pages.calendar(&data).unwrap();
        assert!(html.contains(r#"<body class="page-calendar">"#));
        assert!(!html.contains("season-passiontide") && !html.contains("season-eastertide"));
    }
}
