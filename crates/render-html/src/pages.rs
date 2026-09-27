//! The page templates and their environment. Ported from Go's
//! `render/render.go`: the templates are the Go templates translated to
//! minijinja with identical markup, and the environment reproduces
//! html/template's escaping (see [`crate::escape`]).

use minijinja::value::Value;
use minijinja::{AutoEscape, Environment, Error, Output, State};

use crate::escape::{template_escape, url_norm, url_part, url_start};
use crate::html::{render_section_heading, typeset};
use crate::leader::leader_sections;
use crate::links::{calendar_year_link, home_link, hour_link, nav_link, static_url, title_case};
use crate::usage::UsageData;
use crate::view::{CalendarData, ErrorData, HomeData, HourData, NotFoundData, RemindersData};

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

/// Every value printed into a page is escaped as html/template escapes a
/// text node or quoted attribute, unless it is trusted markup.
fn go_formatter(out: &mut Output, _state: &State, value: &Value) -> Result<(), Error> {
    if value.is_safe() {
        write!(out, "{value}").map_err(Error::from)
    } else if value.is_none() || value.is_undefined() {
        Ok(())
    } else {
        out.write_str(&template_escape(&value.to_string())).map_err(Error::from)
    }
}

impl Pages {
    /// Parses the embedded templates. `version` stamps static asset URLs so
    /// a deploy that changes CSS or JS produces new URLs.
    pub fn new(version: &str) -> Result<Pages, String> {
        let mut env = Environment::new();
        env.set_keep_trailing_newline(true);
        // Macro output is trusted markup; the formatter escapes the rest.
        env.set_auto_escape_callback(|_| AutoEscape::Html);
        env.set_formatter(go_formatter);
        env.set_undefined_behavior(minijinja::UndefinedBehavior::Strict);
        for (name, source) in TEMPLATES {
            env.add_template(name, source).map_err(|e| format!("parsing {name} template: {e}"))?;
        }
        env.add_filter("url", |s: String| url_start(&s));
        env.add_filter("urlnorm", |s: String| url_norm(&s));
        env.add_filter("urlpart", |s: String| url_part(&s));
        env.add_filter("typeset", |s: String| typeset(&s));
        env.add_filter("titlecase", |s: String| title_case(&s));
        env.add_function("nav_link", |base: String, date: String| nav_link(&base, &date));
        env.add_function("home_link", |date: String| home_link(&date));
        env.add_function("hour_link", |hour: String, date: String| hour_link(&hour, &date));
        env.add_function("calendar_year_link", |year: i32| calendar_year_link(year));
        let version = version.to_string();
        env.add_function("static", move |name: String| static_url(&name, &version));
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
    /// into one page and the review banner follows the forms that need it.
    pub fn hour(&self, data: &mut HourData, forms: &[(liturgy::PrayerForm, &liturgy::OfficeHour)]) -> Result<String, String> {
        if !data.leader_forms.is_empty() {
            data.show_banner = false;
            data.banner_forms.clear();
            for form in &data.leader_forms {
                if form.show_banner {
                    data.show_banner = true;
                    data.banner_forms.push_str(&form.form);
                    data.banner_forms.push(' ');
                }
            }
            data.leader_sections = leader_sections(forms)?;
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
