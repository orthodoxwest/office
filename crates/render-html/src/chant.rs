//! Chant scores as HTML: neuma engraves a score's SVG on the server at one width measured in the
//! hymn's own text size, for a browser that can't run the engine, and carries its GABC; chant.js lays it out
//! again in the browser with neuma's module (static/neuma.mjs) at its column's width, as the
//! apps lay a score out at theirs, so a score fills its column and reflows with the page and the
//! reader's text size.
//!
//! A score sits beside the words it sets: the page carries both, marked `data-chant-variant`, and
//! the reader's Chant setting (`data-chant` on the root) shows one (style.css).

use std::collections::HashMap;
use std::sync::{Arc, Mutex, OnceLock};

use liturgy::ChantScore;
use neuma::{Chant, ChantOptions, Hyphens, Initial, LayoutOptions, SvgOptions};

use crate::escape::html_escape_string;
use crate::html::typeset;

/// Staff spaces per lyric em (neuma's default lyric size, GregorioTeX's 10 pt on its staff).
/// chant.js names the same.
const LYRIC_SPACES: f32 = 2.45;
/// SVG units per staff space (neuma's default scale). chant.js names the same.
const SCALE: f32 = 6.0;
/// The width, in lyric ems, of the score a page shows until its script lays the score out at
/// the column's width, or without script: a phone's column is 15 to 20 em at the default text
/// size, and the score scales down to fit a narrower one.
pub const FALLBACK_EM: u16 = 17;

/// A score in place of the words it sets. The group is announced by `words`, the text it sets,
/// typeset as the page's text is, rather than by its SVG.
pub fn score_html(gabc: &Arc<str>, words: &str) -> String {
    format!(
        "<div class=\"chant-score\" data-chant-variant=\"on\" role=\"img\" aria-label=\"{}\">{}</div>",
        html_escape_string(&typeset(words.trim())),
        engraved(gabc, true)
    )
}

/// A whole hymn's score set stanza by stanza (`ChantScore::stanza_parts`): each stanza from a new
/// line with its number, the first alone taking the initial. `stanzas` are the stanzas' words,
/// for their labels; parts that do not pair with them make one score, as written.
pub fn whole_hymn_html(score: &ChantScore, stanzas: &[String]) -> String {
    let parts = score.stanza_parts();
    if parts.len() != stanzas.len() {
        return score_html(&score.gabc, &stanzas.join(" "));
    }
    let mut out = String::from("<div class=\"chant-hymn\" data-chant-variant=\"on\">");
    for (i, (part, words)) in parts.iter().zip(stanzas).enumerate() {
        out.push_str(&format!(
            "<div class=\"chant-score chant-stanza\" role=\"img\" aria-label=\"{}\">{}{}</div>",
            html_escape_string(&typeset(words.trim())),
            if i > 0 { format!("<span class=\"chant-stanza-number\" aria-hidden=\"true\">{}.</span>", i + 1) } else { String::new() },
            engraved(&Arc::from(part.as_str()), i == 0)
        ));
    }
    out.push_str("</div>");
    out
}

/// The score's SVGs, engraved once per score and initial.
fn engraved(gabc: &Arc<str>, initial: bool) -> String {
    type Cache = Mutex<HashMap<(Arc<str>, bool), String>>;
    static CACHE: OnceLock<Cache> = OnceLock::new();
    let cache = CACHE.get_or_init(Default::default);
    let key = (gabc.clone(), initial);
    if let Some(svgs) = cache.lock().ok().and_then(|c| c.get(&key).cloned()) {
        return svgs;
    }
    let svgs = engrave(gabc, initial);
    if let Ok(mut c) = cache.lock() {
        c.insert(key, svgs.clone());
    }
    svgs
}

/// How the page sets a score, as Score.kt, Score.swift and chant.js do: a hyphen between every two
/// syllables, as English chant books mark them for a reader following the words; the initial
/// three quarters of the Solesmes size and flush with the margin, to sit with the page's text
/// initials; and the last staff ending at the final bar.
fn page_options() -> ChantOptions {
    ChantOptions::default()
        .with_hyphens(Hyphens::EverySyllable)
        .with_initial_scale(INITIAL_SCALE)
        .with_initial_margin(0.0)
        .with_short_last_staff(true)
}

/// The initial's size as a share of the Solesmes one, which runs from the staff's top line to
/// the lyrics (Score.kt, Score.swift and chant.js name the same).
const INITIAL_SCALE: f32 = 0.75;

/// The score at [`FALLBACK_EM`], sized in ems so that its lyrics are set at the text's size, with
/// what chant.js lays it out again from: its GABC, whether it takes the initial, and its prefix.
fn engrave(gabc: &str, initial: bool) -> String {
    let options = if initial { page_options() } else { page_options().with_initial(Initial::None).with_annotation(false) };
    let chant = Chant::with_options(gabc, options);
    // Ids and classes unique to this score, so scores on one page never share a definition.
    let prefix = format!("chant{:x}{}", fnv(gabc), if initial { "" } else { "n" });
    let units_per_em = LYRIC_SPACES * SCALE;
    let layout = chant.layout_with(f32::from(FALLBACK_EM) * units_per_em, &LayoutOptions::default().with_scale(SCALE));
    let svg = layout.svg_with(&SvgOptions::default().with_style(false).with_ids(false).with_prefix(prefix.clone()));
    let (w, h) = (layout_width(&svg), layout_height(&svg));
    format!(
        "<div class=\"chant-width\" data-gabc=\"{}\" data-chant-initial=\"{}\" data-chant-prefix=\"{prefix}\" \
         style=\"width:{:.3}em;aspect-ratio:{w:.2}/{h:.2}\">{}</div>",
        html_escape_string(gabc),
        u8::from(initial),
        w / units_per_em,
        strip_label(&svg)
    )
}

/// The `<svg>` without its own label: the score's group carries the words.
fn strip_label(svg: &str) -> String {
    let Some(start) = svg.find(" role=\"img\" aria-label=\"") else { return svg.to_string() };
    let rest = &svg[start + " role=\"img\" aria-label=\"".len()..];
    let Some(end) = rest.find('"') else { return svg.to_string() };
    format!("{} aria-hidden=\"true\" focusable=\"false\"{}", &svg[..start], &rest[end + 1..])
}

fn svg_attr(svg: &str, name: &str) -> f32 {
    let pat = format!(" {name}=\"");
    svg.find(&pat).and_then(|i| svg[i + pat.len()..].split('"').next()).and_then(|v| v.parse().ok()).unwrap_or(1.0)
}

fn layout_width(svg: &str) -> f32 {
    svg_attr(svg, "width")
}

fn layout_height(svg: &str) -> f32 {
    svg_attr(svg, "height")
}

/// A short stable hash for the score's id prefix.
fn fnv(s: &str) -> u32 {
    s.bytes().fold(0x811c_9dc5_u32, |h, b| (h ^ u32::from(b)).wrapping_mul(0x0100_0193))
}

#[cfg(test)]
mod tests {
    use super::*;
    use liturgy::ScoreSpan;

    const HYMN: &str = "name: Te lucis;\n%%\n(c4) TO(g) thee,(g) be(h)fore(j) the(h) close(g) of(h) day,(g.) (::)";

    #[test]
    fn a_score_carries_its_gabc_and_a_fallback_announced_by_its_words() {
        let html = score_html(&Arc::from(HYMN), "To thee, before the close of day,");
        assert!(html.starts_with(
            "<div class=\"chant-score\" data-chant-variant=\"on\" role=\"img\" aria-label=\"To thee, before the close of day,\">"
        ));
        assert!(html.contains(&format!(" data-gabc=\"{}\"", html_escape_string(HYMN))), "{html}");
        assert!(html.contains(" data-chant-initial=\"1\" data-chant-prefix=\"chant"), "{html}");
        assert_eq!(html.matches("<svg").count(), 1);
        assert_eq!(html.matches("aria-hidden=\"true\"").count(), 1);
        assert!(!html.contains("aria-label=\"TO"), "the SVG's own label is dropped");
        assert!(!html.contains("<style"), "style.css themes the score");
    }

    #[test]
    fn a_gabc_with_markup_in_it_stays_in_its_attribute() {
        let gabc = "name: \"x\" & <y>;\n%%\n(c4) A(g) <i>b</i>(h) (::)";
        let html = score_html(&Arc::from(gabc), "A b");
        assert!(html.contains("data-gabc=\"name: &#34;x&#34; &amp; &lt;y&gt;;"), "{html}");
        assert!(!html.contains("<i>b</i>"));
    }

    #[test]
    fn a_whole_hymn_parts_at_its_double_bars_keeping_the_amen() {
        let score = ChantScore {
            gabc: Arc::from("name: t;\n%%\n(c4) One(g) two.(g) (::) Three(g) four.(g) (::) A(gh)men.(g) (::)"),
            sets: ScoreSpan::WholeHymn,
        };
        let parts = score.stanza_parts();
        assert_eq!(parts.len(), 2, "{parts:?}");
        assert!(parts[1].starts_with("name: t;\n%%\n(c4) Three") && parts[1].ends_with("A(gh)men.(g) (::)"), "{}", parts[1]);
        let html = whole_hymn_html(&score, &["One two.".into(), "Three four. Amen.".into()]);
        assert_eq!(html.matches("class=\"chant-score chant-stanza\"").count(), 2);
        assert!(html.contains("<span class=\"chant-stanza-number\" aria-hidden=\"true\">2.</span>"));
        assert_eq!(html.matches(" data-chant-initial=\"1\"").count(), 1, "only the first stanza takes the initial");
        assert_eq!(html.matches(" data-chant-initial=\"0\"").count(), 1);
    }

    /// chant.js lays a score out again as the server does: at its lyric size and scale, with the
    /// page's options, the initial only where the server set one.
    #[test]
    fn chant_js_lays_a_score_out_as_the_server_does() {
        let js = include_str!("../../../apps/office-web/static/chant.js");
        for line in [
            format!("var LYRIC_SPACES = {LYRIC_SPACES};"),
            format!("var SCALE = {SCALE};"),
            format!("initialScale: {INITIAL_SCALE},"),
            "everyHyphen: true,".to_string(),
            "initialMargin: 0,".to_string(),
            "shortLastStaff: true,".to_string(),
            "style: false".to_string(),
        ] {
            assert!(js.contains(&line), "chant.js lacks {line:?}");
        }
        // The options chant.js names are page_options()'s.
        let style = page_options().style;
        assert_eq!(style.hyphens, Hyphens::EverySyllable);
        assert_eq!((style.initial_scale, style.initial_margin, style.short_last_staff), (INITIAL_SCALE, 0.0, true));
    }

    #[test]
    fn widths_are_set_in_lyric_ems() {
        let html = score_html(&Arc::from(HYMN), "x");
        assert!(html.contains("style=\"width:17.000em;"), "{html}");
    }
}
