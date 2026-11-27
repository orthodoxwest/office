//! Page templates with native MiniJinja HTML autoescaping and explicit URL filters
//! (see [`crate::escape`]).

use minijinja::syntax::SyntaxConfig;
use minijinja::value::{Serde, Value};
use minijinja::{AutoEscape, Environment};

use crate::escape::{url_norm, url_part, url_start};
use crate::html::{render_feast_hymn, render_flowing_block, render_liturgical_block, render_section_heading_in, typeset};
use crate::leader::{LeaderSection, leader_sections};
use crate::links::{home_link, hour_link, nav_link};
use crate::usage::UsageData;
use crate::view::{
    AboutData, CalendarData, ErrorData, FeastPageData, FindData, HomeData, HourData, NotFoundData, PrivacyData, RemindersData,
};
use presentation::{season_label, title_case};

const TEMPLATES: [(&str, &str); 13] = [
    ("layout.html", include_str!("../templates/layout.html")),
    ("macros.html", include_str!("../templates/macros.html")),
    ("home.html", include_str!("../templates/home.html")),
    ("hour.html", include_str!("../templates/hour.html")),
    ("calendar.html", include_str!("../templates/calendar.html")),
    ("find.html", include_str!("../templates/find.html")),
    ("feast.html", include_str!("../templates/feast.html")),
    ("reminders.html", include_str!("../templates/reminders.html")),
    ("privacy.html", include_str!("../templates/privacy.html")),
    ("about.html", include_str!("../templates/about.html")),
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
        env.set_syntax(SyntaxConfig::builder().keep_trailing_newline(true).build().map_err(|e| format!("building template syntax: {e}"))?);
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
        // A JSON string literal that stays inside its <script> (the home's WebSite data).
        env.add_filter("script_string", |s: String| Value::from_safe_string(script_string(&s)));
        env.add_filter("titlecase", |s: String| title_case(&s));
        env.add_filter("season_label", |s: String| season_label(&s));
        // A feast's antiphon, versicle or collect, set as the hours set it (℣/℟, the mediant).
        env.add_filter("feast_text", |s: String| Value::from_safe_string(render_liturgical_block(&s)));
        // A feast's collect, flowing as the hours set it.
        env.add_filter("feast_prayer", |s: String| Value::from_safe_string(render_flowing_block(&s)));
        // A feast's hymn, in stanzas as the hours set it.
        env.add_filter("feast_hymn", |s: String| Value::from_safe_string(render_feast_hymn(&s)));
        let w = &presentation::feast::FEAST_WORDS;
        env.add_global(
            "feast_words",
            Value::from_pairs([
                ("find_title", w.find_title),
                ("find_prompt", w.find_prompt),
                ("every_year", w.every_year),
                ("days_hours", w.days_hours),
                ("texts_heading", w.texts_heading),
                ("common_mark", w.common_mark),
                ("whole_office", w.whole_office),
                ("everyone_keeps", w.everyone_keeps),
                ("monastic_heading", w.monastic_heading),
                ("todays_hours", w.todays_hours),
            ]),
        );
        env.add_function("nav_link", |base: String, date: String| nav_link(&base, &date));
        env.add_function("home_link", |date: String| home_link(&date));
        env.add_function("hour_link", |hour: String, date: String| hour_link(&hour, &date));
        env.add_function("static", move |name: String| asset_url(&name));
        env.add_function("section_heading", |label: String, bare: Option<bool>| {
            Value::from_safe_string(render_section_heading_in(&label, bare.unwrap_or(false)))
        });
        env.add_global("chant", false);
        env.add_global("martyrology_hint", presentation::MARTYROLOGY_HINT);
        env.add_global("chant_hint", presentation::CHANT_HINT);
        env.add_global("previous_day", presentation::PREVIOUS_DAY);
        env.add_global("next_day", presentation::NEXT_DAY);
        // The ordo's § and fish, named in the note they open, as the apps name them.
        env.add_global("fast_day", presentation::FAST_DAY);
        env.add_global("abstinence_day", presentation::ABSTINENCE_DAY);
        env.add_global("open_this_day", presentation::OPEN_THIS_DAY);
        // How long an hour keeps the reader's place (hour.html's head).
        env.add_global("place_kept_minutes", presentation::PLACE_KEPT_MINUTES);
        // The About page's names in the menu and on home, as the apps set them.
        env.add_global("about_menu", presentation::about::ABOUT_MENU);
        env.add_global("about_introduction", presentation::about::ABOUT_INTRODUCTION);
        // The colophon's line on the native apps (presentation::stores); iOS joins it once
        // APP_STORE_ID is set.
        env.add_global("apps_lead", presentation::stores::APPS_LEAD);
        env.add_global("apps_apple", presentation::stores::APPS_APPLE);
        env.add_global("play_url", presentation::stores::PLAY_URL);
        env.add_global("app_store_id", presentation::stores::APP_STORE_ID.unwrap_or(""));
        env.add_global("app_store_url", presentation::stores::app_store_url().unwrap_or_default());
        env.add_global("play_label", presentation::stores::PLAY_LABEL);
        env.add_global("app_store_label", presentation::stores::APP_STORE_LABEL);
        env.add_global("about_apps", presentation::stores::ABOUT_APPS);
        env.add_global("reminders_app", presentation::stores::REMINDERS_APP);
        env.add_global("reminders_calendar", presentation::stores::REMINDERS_CALENDAR);
        env.add_global("reminders_calendar_lead", presentation::stores::REMINDERS_CALENDAR_LEAD);
        env.add_global("reminders_calendar_intro", presentation::stores::REMINDERS_CALENDAR_INTRO);
        // The office being prayed from each clock hour, as a script literal for home's pre-paint mark
        // of the hour ([start, slug, name, day offset], home.html).
        env.add_global("hour_schedule", Value::from_safe_string(hour_schedule_literal()));
        // The in-hour "How to read" key and screen gloss notes (`presentation::gloss`).
        env.add_global("reading_key", reading_key_value());
        env.add_global("ordo_gloss", gloss_value(&presentation::gloss::ORDO_GLOSS));
        env.add_global("tabula_temporaria_gloss", gloss_value(&presentation::gloss::TABULA_TEMPORARIA_GLOSS));
        env.add_global("ember_days_gloss", gloss_value(&presentation::gloss::EMBER_DAYS_GLOSS));
        env.add_global("ordo_gloss_ben", gloss_value(&presentation::gloss::ORDO_BENEDICTUS_GLOSS));
        env.add_global("ordo_gloss_mag", gloss_value(&presentation::gloss::ORDO_MAGNIFICAT_GLOSS));
        env.add_global("ordo_gloss_com", gloss_value(&presentation::gloss::ORDO_COMMEMORATION_GLOSS));
        env.add_global("ordo_gloss_preces", gloss_value(&presentation::gloss::PRECES_GLOSS));
        env.add_global("ordo_gloss_suffrage", gloss_value(&presentation::gloss::SUFFRAGE_GLOSS));
        env.add_global("ordo_gloss_cross", gloss_value(&presentation::gloss::COMMEMORATION_OF_CROSS_GLOSS));
        env.add_global("monastic_gloss", gloss_value(&presentation::gloss::MONASTIC_OBSERVANCE_GLOSS));
        env.add_global("proper_gloss", gloss_value(&presentation::gloss::PROPER_GLOSS));
        env.add_global("common_gloss", gloss_value(&presentation::gloss::COMMON_GLOSS));
        env.add_global("octave_gloss", gloss_value(&presentation::gloss::OCTAVE_GLOSS));
        env.add_global("versicle_gloss", gloss_value(&presentation::gloss::VERSICLE_GLOSS));
        env.add_filter("vespers_note_gloss", |s: String| presentation::gloss::vespers_note_gloss(&s).map(gloss_value));
        env.add_filter("season_gloss", |s: String| presentation::gloss::season_gloss(&s).map(gloss_value));
        env.add_filter("day_name_gloss", |s: String| presentation::gloss::day_name_gloss(&s).map(gloss_value));
        env.add_filter("penitential_gloss", |s: String| presentation::gloss::penitential_gloss(&s).map(gloss_value));
        env.add_filter("rank_gloss", |s: String| presentation::gloss::rank_gloss(&s).map(gloss_value));
        env.add_filter("feast_text_gloss", |s: String| presentation::gloss::feast_text_gloss(&s).map(gloss_value));
        Ok(Pages { env })
    }

    /// Offers the Chant setting in the menu: the data holds a score for a hymn or Marian
    /// antiphon (office::chant::offers_chant).
    #[must_use]
    pub fn with_chant(mut self, offered: bool) -> Pages {
        self.env.add_global("chant", offered);
        self
    }

    fn render<T: serde::Serialize>(&self, template: &str, data: &T) -> Result<String, String> {
        let tmpl = self.env.get_template(template).map_err(|e| e.to_string())?;
        tmpl.render(Value::from(Serde(data))).map_err(|e| format!("rendering {template}: {e:#}"))
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

    pub fn find(&self, data: &FindData) -> Result<String, String> {
        self.render("find.html", data)
    }

    pub fn feast(&self, data: &FeastPageData) -> Result<String, String> {
        self.render("feast.html", data)
    }

    pub fn reminders(&self, data: &RemindersData) -> Result<String, String> {
        self.render("reminders.html", data)
    }

    pub fn about(&self, data: &AboutData) -> Result<String, String> {
        self.render("about.html", data)
    }

    pub fn privacy(&self, data: &PrivacyData) -> Result<String, String> {
        self.render("privacy.html", data)
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
    let same = |a: &LeaderSection, b: &LeaderSection| a.label == b.label && a.collapsible == b.collapsible && a.bare == b.bare;
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
    for (passage, (section, htmls)) in rows.into_iter().enumerate() {
        let base = htmls.iter().find(|(c, _)| *c == "off").map(|(_, h)| h.clone());
        let Some(base) = base else {
            // A variant's own section: its heading goes with it.
            let choices: Vec<&str> = htmls.iter().map(|(c, _)| *c).collect();
            let heading = if section.label.is_empty() { String::new() } else { render_section_heading_in(&section.label, section.bare) };
            let html = after_prime_variant(&format!("{heading}{}", htmls[0].1), &choices, passage);
            out.push(LeaderSection { label: String::new(), collapsible: section.collapsible, bare: section.bare, html });
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
            groups.iter().map(|(choices, html)| after_prime_variant(html, choices, passage)).collect()
        };
        out.push(LeaderSection { html, ..section });
    }
    Ok(out)
}

/// The After Prime choices, as the setting stores them.
const AFTER_PRIME_CHOICES: [&str; 3] = ["off", "on", "chapter"];

/// One reading of a passage, for the After Prime choices that read it: the reader's choice
/// (`data-martyrology` on the root) shows the variant whose choices name it. `passage` numbers
/// the passage, so the page can tell a passage's other readings from a neighbouring passage's
/// (the place kept across a change of setting goes to the same passage read another way).
fn after_prime_variant(html: &str, choices: &[&str], passage: usize) -> String {
    format!("<div data-martyrology-variant=\"{}\" data-martyrology-passage=\"{passage}\">{html}</div>", choices.join(" "))
}

// Template escaping and the undated brand link.
/// `presentation::CURRENT_HOUR_SCHEDULE` as a JavaScript array literal: `[[0,"compline","Compline",-1],…]`.
fn hour_schedule_literal() -> String {
    let entries: Vec<String> = presentation::CURRENT_HOUR_SCHEDULE
        .iter()
        .map(|(start, slug, name, offset)| format!("[{start},\"{slug}\",\"{name}\",{offset}]"))
        .collect();
    format!("[{}]", entries.join(","))
}

fn gloss_value(g: &presentation::gloss::Gloss) -> Value {
    minijinja::context! {
        title => g.title,
        note => g.note,
    }
}

fn reading_key_value() -> Value {
    use presentation::gloss::{
        READING_KEY_FOOT_AFTER, READING_KEY_FOOT_BEFORE, READING_KEY_FOOT_LINK, READING_KEY_ITEMS, READING_KEY_LABEL, READING_KEY_TITLE,
    };
    let items: Vec<Value> = READING_KEY_ITEMS
        .iter()
        .map(|item| {
            minijinja::context! {
                mark => item.mark,
                spoken => item.spoken,
                style => item.style.name(),
                text => item.text,
            }
        })
        .collect();
    minijinja::context! {
        label => READING_KEY_LABEL,
        title => READING_KEY_TITLE,
        items => items,
        foot_before => READING_KEY_FOOT_BEFORE,
        foot_link => READING_KEY_FOOT_LINK,
        foot_after => READING_KEY_FOOT_AFTER,
    }
}

/// `s` as a JSON string literal, quoted, safe inside a `<script>` element: `<`, `>` and `&` are
/// escaped too, so no `</script>` or comment opener can close it early.
fn script_string(s: &str) -> String {
    let mut out = String::with_capacity(s.len() + 2);
    out.push('"');
    for c in s.chars() {
        match c {
            '"' => out.push_str("\\\""),
            '\\' => out.push_str("\\\\"),
            '<' | '>' | '&' | '\u{2028}' | '\u{2029}' => out.push_str(&format!("\\u{:04x}", c as u32)),
            c if (c as u32) < 0x20 => out.push_str(&format!("\\u{:04x}", c as u32)),
            c => out.push(c),
        }
    }
    out.push('"');
    out
}

#[cfg(test)]
mod tests {
    use super::{Pages, TEMPLATES, script_string};
    use crate::links::static_url;

    fn source(name: &str) -> &'static str {
        TEMPLATES.iter().find(|(n, _)| *n == name).map(|(_, s)| *s).unwrap()
    }

    #[test]
    fn script_strings_cannot_close_their_script() {
        assert_eq!(script_string("https://example.org/"), "\"https://example.org/\"");
        assert_eq!(script_string("a\"</script><!--\\"), "\"a\\\"\\u003c/script\\u003e\\u003c!--\\\\\"");
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
    fn about_lights_the_hour_home_lights_and_ends_with_its_invitation() {
        use crate::view::{AboutData, Chrome};
        let pages = Pages::new(|name| static_url(name, "test")).unwrap();
        let today = calendar::Date::new(2026, 11, 13);
        let chrome = || Chrome { page: "about".into(), ..Chrome::default() };
        let afternoon = pages.about(&AboutData::new(chrome(), today, 15)).unwrap();
        assert_eq!(afternoon.matches("about-hour is-current").count(), 1);
        assert!(afternoon.contains(r#"<div class="about-hour is-current" data-hour="none">"#));
        assert!(afternoon.contains(r#"<a class="pray-now" href="&#x2f;none&#x2f;2026-11-13">Pray None</a>"#));
        // The door is the page's last word, after the last paragraph.
        let door = afternoon.find("about-door").unwrap();
        assert!(afternoon.find("issues/new\">issue tracker</a>").unwrap() < door);
        // From midnight to 2am home lights no hour, and invites to yesterday's Compline.
        let late = pages.about(&AboutData::new(chrome(), today, 1)).unwrap();
        assert!(!late.contains("is-current"));
        assert!(late.contains(r#"<a class="pray-now" href="&#x2f;compline&#x2f;2026-11-12">Pray Compline</a>"#));
    }

    #[test]
    fn colophon_names_only_the_apps_that_are_in_a_store() {
        use crate::view::{AboutData, Chrome};
        let pages = Pages::new(|name| static_url(name, "test")).unwrap();
        let chrome = Chrome { page: "about".into(), ..Chrome::default() };
        let html = pages.about(&AboutData::new(chrome, calendar::Date::new(2026, 11, 13), 15)).unwrap();
        let footer = &html[html.find("<footer>").unwrap()..];
        assert!(footer.contains(r#"data-store="android"><a href="https:&#x2f;&#x2f;play.google.com&#x2f;store&#x2f;apps&#x2f;details?id=org.orthodoxwest.office">Android</a>"#));
        // iPhone, Safari's banner and the line for an iPhone reader come with the listing.
        let ios = presentation::stores::APP_STORE_ID.is_some();
        assert_eq!(footer.contains(r#"data-store="ios""#), ios);
        assert_eq!(footer.contains("app-stores has-ios"), ios);
        assert_eq!(html.contains(r#"name="apple-itunes-app""#), ios);
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

    // Home marks the device's hour before its first paint from presentation's schedule, so the
    // inline script and the server cannot disagree on a boundary.
    #[test]
    fn home_marks_the_hour_from_the_shared_schedule() {
        let pages = Pages::new(|name| static_url(name, "test")).unwrap();
        let html = pages.home(&crate::view::HomeData { date_slug: "2026-11-13".into(), ..Default::default() }).unwrap();
        let literal = super::hour_schedule_literal();
        assert!(literal.starts_with(r#"[[0,"compline","Compline",-1],[2,"lauds","Lauds",0],"#), "{literal}");
        assert_eq!(literal.matches('[').count(), presentation::CURRENT_HOUR_SCHEDULE.len() + 1);
        assert!(html.contains(&format!("{literal}.forEach(")), "home's pre-paint script reads the schedule");
    }
}
