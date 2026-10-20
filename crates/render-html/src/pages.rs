//! Page templates with native MiniJinja HTML autoescaping and explicit URL filters
//! (see [`crate::escape`]).

use minijinja::value::Value;
use minijinja::{AutoEscape, Environment};

use crate::escape::{url_norm, url_part, url_start};
use crate::html::{render_section_heading, typeset};
use crate::leader::{LeaderSection, leader_sections, mark_after_prime};
use crate::links::{home_link, hour_link, nav_link};
use crate::usage::UsageData;
use crate::view::{BetaData, CalendarData, ErrorData, HomeData, HourData, NotFoundData, PrivacyData, RemindersData};
use presentation::{season_label, title_case};

const TEMPLATES: [(&str, &str); 11] = [
    ("layout.html", include_str!("../templates/layout.html")),
    ("macros.html", include_str!("../templates/macros.html")),
    ("home.html", include_str!("../templates/home.html")),
    ("hour.html", include_str!("../templates/hour.html")),
    ("calendar.html", include_str!("../templates/calendar.html")),
    ("reminders.html", include_str!("../templates/reminders.html")),
    ("privacy.html", include_str!("../templates/privacy.html")),
    ("beta.html", include_str!("../templates/beta.html")),
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
    /// `after_prime` holds Prime composed for the reader's other After Prime choices (`on`, the
    /// Martyrology read; `chapter`, the Capitular Office), each with every form, or nothing when
    /// a choice changes nothing that day. A section a choice changes carries each reading, a
    /// section it adds is carried for it alone, and the reader's setting shows one of them, so
    /// the page itself never depends on the setting.
    pub fn hour(
        &self,
        data: &mut HourData,
        forms: &[(liturgy::PrayerForm, &liturgy::OfficeHour)],
        after_prime: &[(&str, &[(liturgy::PrayerForm, &liturgy::OfficeHour)])],
    ) -> Result<String, String> {
        if !data.leader_forms.is_empty() {
            let sections = leader_sections(forms)?;
            let mut variants = Vec::new();
            for (choice, hours) in after_prime {
                variants.push((*choice, leader_sections(hours)?));
            }
            data.leader_sections = if variants.is_empty() { sections } else { after_prime_sections(sections, &variants)? };
        }
        self.render("hour.html", data)
    }

    pub fn calendar(&self, data: &CalendarData) -> Result<String, String> {
        self.render("calendar.html", data)
    }

    pub fn reminders(&self, data: &RemindersData) -> Result<String, String> {
        self.render("reminders.html", data)
    }

    pub fn privacy(&self, data: &PrivacyData) -> Result<String, String> {
        self.render("privacy.html", data)
    }

    pub fn beta(&self, data: &BetaData) -> Result<String, String> {
        self.render("beta.html", data)
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

/// One page's sections for every After Prime choice: `base` is Prime with the rubric (`off`), and
/// each variant is Prime for another choice. A variant's sections are the base's with, at most,
/// one run of its own inserted (the Capitular Office's, after the Martyrology).
fn after_prime_sections(base: Vec<LeaderSection>, variants: &[(&str, Vec<LeaderSection>)]) -> Result<Vec<LeaderSection>, String> {
    let same = |a: &LeaderSection, b: &LeaderSection| a.label == b.label && a.collapsible == b.collapsible;
    // Each row: the base section, or a section of a variant's own; its html under each choice.
    let mut rows: Vec<(LeaderSection, Vec<(&str, String)>)> = base
        .into_iter()
        .map(|s| {
            let html = s.html.clone();
            (s, vec![("off", html)])
        })
        .collect();
    let base_rows = rows.len();
    for (choice, sections) in variants {
        let n = base_rows;
        if sections.len() < n {
            return Err("after-prime section mismatch".into());
        }
        let based: Vec<usize> = (0..rows.len()).filter(|&i| rows[i].1.iter().any(|(c, _)| *c == "off")).collect();
        let mut prefix = 0;
        while prefix < n && same(&rows[based[prefix]].0, &sections[prefix]) {
            prefix += 1;
        }
        let mut suffix = 0;
        while prefix + suffix < n && same(&rows[based[n - 1 - suffix]].0, &sections[sections.len() - 1 - suffix]) {
            suffix += 1;
        }
        if prefix + suffix != n {
            return Err("after-prime section mismatch".into());
        }
        for i in 0..n {
            let section = if i < prefix { &sections[i] } else { &sections[sections.len() - n + i] };
            rows[based[i]].1.push((choice, section.html.clone()));
        }
        let own = &sections[prefix..sections.len() - suffix];
        let at = if prefix == 0 { 0 } else { based[prefix - 1] + 1 };
        for (k, section) in own.iter().enumerate() {
            rows.insert(at + k, (section.clone(), vec![(choice, section.html.clone())]));
        }
    }
    let mut out = Vec::new();
    for (section, htmls) in rows {
        let base = htmls.iter().find(|(c, _)| *c == "off").map(|(_, h)| h.clone());
        let Some(base) = base else {
            // A variant's own section: its heading goes with it.
            let choices: Vec<&str> = htmls.iter().map(|(c, _)| *c).collect();
            let heading = if section.label.is_empty() { String::new() } else { render_section_heading(&section.label) };
            let html = mark_after_prime(&format!("{heading}{}", htmls[0].1), &choices.join(" "));
            out.push(LeaderSection { label: String::new(), collapsible: section.collapsible, html });
            continue;
        };
        // Every choice reads the base where its own composition is missing or the same.
        let mut groups: Vec<(Vec<&str>, String)> = Vec::new();
        for choice in AFTER_PRIME_CHOICES {
            let html = htmls.iter().find(|(c, _)| *c == choice).map_or(&base, |(_, h)| h);
            match groups.iter_mut().find(|(_, h)| h == html) {
                Some((choices, _)) => choices.push(choice),
                None => groups.push((vec![choice], html.clone())),
            }
        }
        let html = if groups.len() == 1 {
            base
        } else {
            groups.iter().map(|(choices, html)| mark_after_prime(html, &choices.join(" "))).collect()
        };
        out.push(LeaderSection { html, ..section });
    }
    Ok(out)
}

/// The After Prime choices, as the setting stores them.
const AFTER_PRIME_CHOICES: [&str; 3] = ["off", "on", "chapter"];

// Template escaping and the undated brand link.
#[cfg(test)]
mod tests {
    use super::{Pages, TEMPLATES};
    use crate::links::static_url;

    fn source(name: &str) -> &'static str {
        TEMPLATES.iter().find(|(n, _)| *n == name).map(|(_, s)| *s).unwrap()
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
    fn layout_brand_is_today_home() {
        let body = source("layout.html");
        // The brand must not carry the page's date, which pins an overnight
        // app to yesterday.
        for want in [r#"data-nav-home="today""#, r#"nav_link("/", "")"#, "not show_today"] {
            assert!(body.contains(want), "template is missing {want:?}");
        }
        assert!(!body.contains(r#"nav_link("/", d)"#));
        assert!(!body.contains(r#"{% if p == "home" %} active{% endif %}"#), "not every home page is current");
    }
}
