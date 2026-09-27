//! Text-to-HTML conversion of composed office elements: psalm verses,
//! liturgical blocks, hymns, and the typography every run of display text
//! passes through. Ported from Go's `render/html.go` and `render/typeset.go`.

use corpus::lines::{BlockKind, BlockLine, PsalmItem, hymn_rubric_stanza, hymn_rubric_text, parse_block, parse_hymn, parse_psalm};
use liturgy::{ElementType, OfficeElement, VoiceRole, VoiceSpan};

use crate::escape::html_escape_string;

// Sigil column classes: a single mark (℣. / ℟.) takes the narrow gutter;
// spelled-out words take their own class so CSS can move them above the text
// on small screens.
const SIGIL_CLASS: &str = "sigil";
const SIGIL_WORD_CLASS: &str = "sigil sigil-word";
const SIGIL_ALL_CLASS: &str = "sigil sigil-all";

/// Words that begin with an eliding apostrophe rather than an opening quote.
const ELIDED_WORDS: [&str; 9] = ["mid", "midst", "tis", "twas", "twere", "gainst", "neath", "tween", "twixt"];

// PORT(inherited): Go's unicode.IsLetter / IsDigit / IsUpper / IsSpace are
// general-category tests; these Rust properties agree on the corpus.
fn is_letter(c: char) -> bool {
    c.is_alphabetic()
}

fn is_digit(c: char) -> bool {
    c.is_ascii_digit() || (!c.is_ascii() && c.is_numeric())
}

/// Curly quotes and apostrophes for the corpus's typewriter punctuation, and
/// an en dash between two digits (a verse range). Display text only.
pub fn typeset(s: &str) -> String {
    if !s.contains(['\'', '"', '-']) {
        return s.to_string();
    }
    let mut b = String::with_capacity(s.len() + 8);
    let mut prev: Option<char> = None;
    for (i, r) in s.char_indices() {
        // Go decodes past the end as U+FFFD, which is neither letter nor digit.
        let next = s[i + r.len_utf8()..].chars().next().unwrap_or('\u{FFFD}');
        match r {
            '\'' => {
                let after_word = prev.is_some_and(|p| is_letter(p) || is_digit(p) || is_closing_punct(p));
                if after_word || (is_letter(next) && starts_elision(&s[i + 1..])) {
                    b.push('’');
                } else {
                    b.push('‘');
                }
            }
            '"' => {
                if prev.is_none_or(|p| p.is_whitespace() || is_opening_punct(p)) {
                    b.push('“');
                } else {
                    b.push('”');
                }
            }
            '-' if prev.is_some_and(is_digit) && is_digit(next) => b.push('–'),
            _ => b.push(r),
        }
        prev = Some(r);
    }
    b
}

fn starts_elision(rest: &str) -> bool {
    let end = rest.find(|c: char| !is_letter(c)).unwrap_or(rest.len());
    ELIDED_WORDS.contains(&rest[..end].to_lowercase().as_str())
}

fn is_opening_punct(c: char) -> bool {
    "([{‘“—–-/".contains(c)
}

fn is_closing_punct(c: char) -> bool {
    ".,;:!?)]}’”".contains(c)
}

/// The escape for every run of display text in an hour.
pub fn esc_text(s: &str) -> String {
    html_escape_string(&typeset(s))
}

/// Escaped text with the ✠ cross styled.
fn esc_cross(s: &str) -> String {
    esc_text(s).replace('✠', "<span class=\"cross\">✠</span>")
}

/// Title-cases a leading run of ALL-CAPS words so a CSS drop cap takes only
/// the initial: "GOD be merciful" → "God be merciful".
fn soften_drop_cap_opening(s: &str) -> String {
    let mut out = String::with_capacity(s.len());
    let mut changed = false;
    let mut rest = s;
    loop {
        let ws = rest.len() - rest.trim_start().len();
        out.push_str(&rest[..ws]);
        rest = &rest[ws..];
        if rest.is_empty() {
            break;
        }
        let end = rest.find(char::is_whitespace).unwrap_or(rest.len());
        let word = &rest[..end];
        let letter_end = word.char_indices().rev().find(|(_, c)| is_letter(*c)).map_or(0, |(i, c)| i + c.len_utf8());
        if letter_end == 0 {
            out.push_str(rest);
            return out;
        }
        let (letters, trail) = word.split_at(letter_end);
        if !letters.chars().all(|c| is_letter(c) && c.is_uppercase()) {
            out.push_str(rest);
            return out;
        }
        if letters.chars().count() >= 2 {
            let mut chars = letters.chars();
            out.push(chars.next().expect("non-empty"));
            for c in chars {
                // Go's unicode.ToLower maps one rune to one rune.
                let mut lower = c.to_lowercase();
                match (lower.next(), lower.next()) {
                    (Some(l), None) => out.push(l),
                    _ => out.push(c),
                }
            }
            out.push_str(trail);
            changed = true;
        } else {
            out.push_str(word);
        }
        rest = &rest[end..];
    }
    if changed { out } else { s.to_string() }
}

/// A section's elements, a psalm or canticle absorbing the doxology after it.
pub fn render_section_elements(elems: &[OfficeElement]) -> String {
    let mut sb = String::new();
    let mut i = 0;
    while i < elems.len() {
        let elem = &elems[i];
        let mut doxology = "";
        if i + 1 < elems.len()
            && matches!(elem.kind, ElementType::Psalm | ElementType::Canticle)
            && elems[i + 1].kind == ElementType::PsalmDoxology
        {
            doxology = &elems[i + 1].text;
            i += 1;
        }
        sb.push_str(&render_office_element(elem, doxology));
        i += 1;
    }
    sb
}

/// A section heading, separating a commemoration's rubric from its feast.
pub fn render_section_heading(label: &str) -> String {
    if let Some(name) = label.strip_prefix("Commemoration of ").filter(|n| !n.is_empty()) {
        return format!(
            "<h2 class=\"section-heading commemoration-heading\"><span class=\"commemoration-kicker\">Commemoration of</span> <span class=\"commemoration-name\">{}</span></h2>",
            esc_text(name)
        );
    }
    format!("<h2 class=\"section-heading\">{}</h2>", esc_text(label))
}

fn render_office_element(elem: &OfficeElement, doxology: &str) -> String {
    let mut sb = String::new();
    match elem.kind {
        ElementType::Heading => sb.push_str(&render_section_heading(&elem.text)),
        ElementType::Rubric => sb.push_str(&render_rubric(elem)),
        ElementType::OpeningAcclamation => {
            sb.push_str("<p class=\"opening-acclamation\">");
            sb.push_str(&chant_line_html(&elem.text));
            sb.push_str("</p>");
        }
        ElementType::Antiphon => {
            if elem.label.is_empty() {
                let class = if elem.announce { "antiphon antiphon-announce" } else { "antiphon" };
                sb.push_str(&format!("<p class=\"{class}\"><em>Ant.</em> "));
                sb.push_str(&chant_line_html(&elem.display_text()));
                sb.push_str("</p>");
            } else {
                sb.push_str("<div class=\"marian-antiphon\"><h3 class=\"item-label\" lang=\"la\">");
                sb.push_str(&esc_text(&elem.label));
                sb.push_str("</h3>");
                sb.push_str(&render_marian_antiphon(&elem.text));
                sb.push_str("</div>");
            }
        }
        ElementType::Psalm | ElementType::Canticle => {
            sb.push_str(&format!("<div class=\"{}\">", elem.kind.as_str()));
            if !elem.label.is_empty() {
                sb.push_str("<h3 class=\"item-label\">");
                sb.push_str(&esc_text(&elem.label));
                if !elem.incipit.is_empty() {
                    sb.push_str("<span class=\"label-sep\" aria-hidden=\"true\"> · </span>");
                    sb.push_str("<span class=\"psalm-incipit\" lang=\"la\">");
                    sb.push_str(&esc_text(&elem.incipit));
                    sb.push_str("</span>");
                }
                sb.push_str("</h3>");
            }
            sb.push_str(&render_psalm_verses(&elem.text));
            if !doxology.is_empty() {
                sb.push_str(&render_gloria_patri(doxology));
            }
            sb.push_str("</div>");
        }
        ElementType::Hymn => {
            sb.push_str("<div class=\"hymn\"><h2 class=\"section-heading\">Hymn</h2>");
            if !elem.label.is_empty() {
                sb.push_str("<p class=\"hymn-title\" lang=\"la\">");
                sb.push_str(&esc_text(&elem.label));
                sb.push_str("</p>");
            }
            sb.push_str(&render_hymn_stanzas(&elem.text));
            sb.push_str("</div>");
        }
        ElementType::Versicle | ElementType::Response | ElementType::Blessing | ElementType::Doxology | ElementType::Dialogue => {
            sb.push_str(&render_liturgical_block(&elem.text));
        }
        ElementType::ShortResponsory => sb.push_str(&render_block(&elem.text, Mode::PreserveLines, true)),
        ElementType::CorporateLordPrayer => sb.push_str(&render_corporate_lord_prayer(elem)),
        ElementType::Collect => {
            sb.push_str("<div class=\"collect\">");
            if elem.voice.is_empty() {
                sb.push_str(&render_flowing_block(&elem.text));
            } else {
                sb.push_str(&render_voice_block(&elem.voice, Mode::Flow));
            }
            sb.push_str("</div>");
        }
        ElementType::Prayer | ElementType::Reading => {
            if let Some(turns) = elem.speaker_turns().filter(|t| !t.is_empty()) {
                for turn in turns {
                    let role = turn.role.expect("speaker turns carry roles");
                    sb.push_str(&format!("<div class=\"prayer-turn\" data-speaker=\"{}\"><p class=\"prayer-speaker\">", role.as_str()));
                    sb.push_str(role.label());
                    sb.push_str("</p>");
                    sb.push_str(&render_flowing_block(&turn.text));
                    sb.push_str("</div>");
                }
            } else if !elem.voice.is_empty() && elem.voice.iter().all(|s| s.role.is_none()) {
                sb.push_str(&render_voice_block(&elem.voice, Mode::Flow));
            } else {
                sb.push_str(&render_flowing_block(&elem.text));
            }
        }
        ElementType::Chapter => {
            sb.push_str("<div class=\"chapter\"><h2 class=\"section-heading\">Chapter</h2>");
            if !elem.label.is_empty() {
                sb.push_str("<p class=\"chapter-ref\">");
                sb.push_str(&esc_text(&elem.label));
                sb.push_str("</p>");
            }
            sb.push_str(&render_flowing_block(&elem.text));
            sb.push_str("</div>");
        }
        ElementType::Preces => {
            sb.push_str("<div class=\"preces\">");
            sb.push_str(&render_liturgical_block(&elem.text));
            sb.push_str("</div>");
        }
        ElementType::PsalmDoxology => sb.push_str(&render_gloria_patri(&elem.text)),
    }
    sb
}

/// A psalm or canticle. A scripture reference precedes the verses so the
/// first verse is the first child of `.psalm-verses` and takes the drop cap.
pub fn render_psalm_verses(text: &str) -> String {
    let psalm = parse_psalm(text);
    let mut sb = String::new();
    if !psalm.scripture_ref.is_empty() {
        sb.push_str(&format!("<p class=\"scripture-ref\">{}</p>", esc_text(&psalm.scripture_ref)));
    }
    sb.push_str("<div class=\"psalm-verses\">");
    let mut drop_cap_next = true;
    for item in &psalm.items {
        match item {
            PsalmItem::Section { heading } => {
                sb.push_str("</div>");
                sb.push_str(&format!("<p class=\"canticle-section\">{}</p>", esc_text(heading)));
                sb.push_str("<div class=\"psalm-verses\">");
                drop_cap_next = true;
            }
            PsalmItem::Gloria { first, second } => {
                sb.push_str("<p class=\"verse\">");
                sb.push_str(&esc_text(first));
                if !second.is_empty() {
                    sb.push_str(" <span class=\"mediant\">*</span> ");
                    sb.push_str(&esc_text(second));
                }
                sb.push_str("</p>");
                drop_cap_next = false;
            }
            PsalmItem::Verse { number, first, second } => {
                let first = if drop_cap_next {
                    drop_cap_next = false;
                    soften_drop_cap_opening(first)
                } else {
                    first.clone()
                };
                if number.is_empty() {
                    sb.push_str("<p class=\"verse\">");
                } else {
                    sb.push_str(&format!(
                        "<p class=\"verse numbered\"><span class=\"verse-num\">{}</span><span class=\"verse-body\">",
                        esc_text(number)
                    ));
                }
                sb.push_str(&esc_cross(&first));
                if !second.is_empty() {
                    sb.push_str(" <span class=\"mediant\">*</span> ");
                    sb.push_str(&esc_cross(second));
                }
                if !number.is_empty() {
                    sb.push_str("</span>");
                }
                sb.push_str("</p>");
            }
        }
    }
    sb.push_str("</div>");
    sb
}

#[derive(Clone, Copy, PartialEq, Eq)]
enum Mode {
    PreserveLines,
    Flow,
    PreserveFirstBlock,
}

/// Blessings, doxologies, and preces keep their prose line breaks.
pub fn render_liturgical_block(text: &str) -> String {
    render_block(text, Mode::PreserveLines, false)
}

fn render_flowing_block(text: &str) -> String {
    render_block(text, Mode::Flow, false)
}

fn render_rubric(elem: &OfficeElement) -> String {
    let mut sb = String::from("<p class=\"rubric\">");
    if elem.rubric_spans.is_empty() {
        sb.push_str(&esc_text(&elem.text));
    } else {
        for span in &elem.rubric_spans {
            if span.prayed {
                sb.push_str(&format!("<span class=\"rubric-prayed\">{}</span>", esc_cross(&span.text)));
            } else {
                sb.push_str(&esc_text(&span.text));
            }
        }
    }
    sb.push_str("</p>");
    sb
}

/// The corporate Lord's Prayer, split between officiant and people; any
/// incomplete partition falls back to an ordinary prayer.
fn render_corporate_lord_prayer(elem: &OfficeElement) -> String {
    let (mut officiant, mut response) = (String::new(), String::new());
    for span in &elem.voice {
        match span.role {
            Some(VoiceRole::Officiant) => officiant.push_str(&span.text),
            Some(VoiceRole::Response) => response.push_str(&span.text),
            Some(VoiceRole::Priest | VoiceRole::All) | None => {}
        }
    }
    if officiant.is_empty() || response.is_empty() {
        return render_flowing_block(&elem.text);
    }
    let flow = |s: &str| s.split('\n').map(str::trim).filter(|l| !l.is_empty()).collect::<Vec<_>>().join(" ");
    format!(
        "<div class=\"liturgical-block corporate-lord-prayer\"><p class=\"plain-line corporate-lord-prayer-officiant\">{}</p><p class=\"response-line corporate-lord-prayer-response\"><span class=\"{SIGIL_CLASS}\">℟.</span><span class=\"sigil-text\">{}</span></p></div>",
        chant_line_html(&flow(&officiant)),
        chant_line_html(&flow(&response))
    )
}

/// A prayer partitioned into spoken and silent spans.
fn render_voice_block(spans: &[VoiceSpan], mode: Mode) -> String {
    let text: String = spans.iter().map(|s| s.text.as_str()).collect();
    let mut spoken_at = Vec::with_capacity(text.len());
    for span in spans {
        spoken_at.extend(std::iter::repeat_n(span.spoken, span.text.len()));
    }
    render_voiced_block(&text, &spoken_at, mode)
}

/// `text` as runs of spoken and secret spans, by byte.
fn emit_voiced(sb: &mut String, text: &str, offset: usize, spoken_at: &[bool]) {
    let bytes = text.as_bytes();
    let mut i = 0;
    while i < bytes.len() {
        let spoken = spoken_at[offset + i];
        let mut j = i + 1;
        while j < bytes.len() && spoken_at[offset + j] == spoken {
            j += 1;
        }
        let class = if spoken { "spoken-text" } else { "secret-text" };
        // Span boundaries fall between whole characters of the composed text.
        sb.push_str(&format!("<span class=\"{class}\">{}</span>", esc_cross(&text[i..j])));
        i = j;
    }
}

fn render_voiced_block(text: &str, spoken_at: &[bool], mode: Mode) -> String {
    let mut sb = String::from("<div class=\"liturgical-block\">");
    let mut prose: Vec<BlockLine> = Vec::new();
    let mut prose_blocks = 0;
    let mut pending_gap = false;
    let emit_gap = |sb: &mut String, pending: &mut bool| {
        if *pending {
            sb.push_str("<div class=\"liturgical-gap\"></div>");
            *pending = false;
        }
    };
    let flush = |sb: &mut String, prose: &mut Vec<BlockLine>, blocks: &mut usize, pending: &mut bool| {
        if prose.is_empty() {
            return;
        }
        emit_gap(sb, pending);
        sb.push_str("<p class=\"plain-line\">");
        let preserve = mode == Mode::PreserveLines || (mode == Mode::PreserveFirstBlock && *blocks == 0);
        for (i, l) in prose.iter().enumerate() {
            if i > 0 {
                sb.push_str(if preserve { "<br>" } else { " " });
            }
            emit_voiced(sb, &l.text, l.offset, spoken_at);
        }
        sb.push_str("</p>");
        prose.clear();
        *blocks += 1;
    };
    for line in parse_block(text) {
        let (line_class, mark_class, sigil) = match line.kind {
            BlockKind::Gap => {
                flush(&mut sb, &mut prose, &mut prose_blocks, &mut pending_gap);
                pending_gap = true;
                continue;
            }
            BlockKind::ScriptureRef => {
                flush(&mut sb, &mut prose, &mut prose_blocks, &mut pending_gap);
                emit_gap(&mut sb, &mut pending_gap);
                sb.push_str(&format!("<p class=\"scripture-ref\">{}</p>", esc_text(&line.text)));
                continue;
            }
            BlockKind::Versicle => ("versicle-line", SIGIL_CLASS, "℣."),
            BlockKind::Response => ("response-line", SIGIL_CLASS, "℟."),
            BlockKind::Blessing => ("versicle-line", SIGIL_WORD_CLASS, "Blessing."),
            // PORT(inherited): Go's voiced renderer has no All: case; the
            // line is prose.
            BlockKind::All | BlockKind::Prose => {
                prose.push(line);
                continue;
            }
        };
        flush(&mut sb, &mut prose, &mut prose_blocks, &mut pending_gap);
        emit_gap(&mut sb, &mut pending_gap);
        sb.push_str(&format!("<p class=\"{line_class}\"><span class=\"{mark_class}\">{sigil}</span><span class=\"sigil-text\">"));
        emit_voiced(&mut sb, &line.text, line.offset, spoken_at);
        sb.push_str("</span></p>");
    }
    flush(&mut sb, &mut prose, &mut prose_blocks, &mut pending_gap);
    sb.push_str("</div>");
    sb
}

/// A Marian antiphon: its sung lines preserved, its collect flowing.
fn render_marian_antiphon(text: &str) -> String {
    const INVITATION: &str = "\n\nLet us pray.\n\n";
    match text.split_once(INVITATION) {
        Some((chant, collect)) if !collect.trim().is_empty() => format!(
            "{}<div class=\"liturgical-gap\"></div><p class=\"plain-line\">Let us pray.</p><div class=\"liturgical-gap\"></div><div class=\"collect\">{}</div>",
            render_block(chant, Mode::PreserveFirstBlock, false),
            render_flowing_block(collect)
        ),
        _ => render_block(text, Mode::PreserveFirstBlock, false),
    }
}

/// A line of liturgical text with its " * " mediant styled.
pub fn chant_line_html(line: &str) -> String {
    if let Some((before, after)) = line.split_once(" * ") {
        return format!("{} <span class=\"mediant\">*</span> {}", esc_cross(before), esc_cross(after));
    }
    if let Some(before) = line.strip_suffix(" *") {
        return format!("{} <span class=\"mediant\">*</span>", esc_cross(before));
    }
    esc_cross(line)
}

fn render_block(text: &str, mode: Mode, short_responsory: bool) -> String {
    let mut sb = String::from("<div class=\"liturgical-block\">");
    let mut prose: Vec<String> = Vec::new();
    let mut prose_blocks = 0;
    let mut pending_gap = false;
    let emit_gap = |sb: &mut String, pending: &mut bool| {
        if *pending {
            sb.push_str("<div class=\"liturgical-gap\"></div>");
            *pending = false;
        }
    };
    let flush = |sb: &mut String, prose: &mut Vec<String>, blocks: &mut usize, pending: &mut bool| {
        if prose.is_empty() {
            return;
        }
        emit_gap(sb, pending);
        if mode == Mode::PreserveFirstBlock && *blocks == 0 {
            // The sung Marian antiphon's opening pair shares one block so a
            // two-line drop cap can sit beside both lines.
            let open_n = prose.len().min(2);
            if open_n > 0 {
                sb.push_str("<p class=\"chant-line chant-line-opening\">");
                for (i, l) in prose[..open_n].iter().enumerate() {
                    if i > 0 {
                        sb.push_str("<br>");
                    }
                    sb.push_str(&chant_line_html(l));
                }
                sb.push_str("</p>");
            }
            for l in &prose[open_n..] {
                sb.push_str(&format!("<p class=\"chant-line\">{}</p>", chant_line_html(l)));
            }
            prose.clear();
            *blocks += 1;
            return;
        }
        sb.push_str("<p class=\"plain-line\">");
        for (i, l) in prose.iter().enumerate() {
            if i > 0 {
                sb.push_str(if mode == Mode::PreserveLines { "<br>" } else { " " });
            }
            sb.push_str(&chant_line_html(l));
        }
        sb.push_str("</p>");
        prose.clear();
        *blocks += 1;
    };
    let mut first_response = true;
    for line in parse_block(text) {
        let (line_class, mark_class, sigil) = match line.kind {
            BlockKind::Gap => {
                flush(&mut sb, &mut prose, &mut prose_blocks, &mut pending_gap);
                pending_gap = true;
                continue;
            }
            BlockKind::ScriptureRef => {
                flush(&mut sb, &mut prose, &mut prose_blocks, &mut pending_gap);
                emit_gap(&mut sb, &mut pending_gap);
                sb.push_str(&format!("<p class=\"scripture-ref\">{}</p>", esc_text(&line.text)));
                continue;
            }
            BlockKind::Versicle => ("versicle-line", SIGIL_CLASS, "℣."),
            BlockKind::Response if short_responsory && first_response => {
                first_response = false;
                flush(&mut sb, &mut prose, &mut prose_blocks, &mut pending_gap);
                emit_gap(&mut sb, &mut pending_gap);
                sb.push_str(&format!(
                    "<p class=\"response-line short-responsory-opening\"><span class=\"sigil-text\">{}</span></p>",
                    chant_line_html(&line.text)
                ));
                continue;
            }
            BlockKind::Response => {
                first_response = false;
                ("response-line", SIGIL_CLASS, "℟.")
            }
            BlockKind::Blessing => ("versicle-line", SIGIL_WORD_CLASS, "Blessing."),
            BlockKind::All => ("all-line", SIGIL_ALL_CLASS, "All:"),
            BlockKind::Prose => {
                prose.push(line.text);
                continue;
            }
        };
        flush(&mut sb, &mut prose, &mut prose_blocks, &mut pending_gap);
        emit_gap(&mut sb, &mut pending_gap);
        sb.push_str(&format!(
            "<p class=\"{line_class}\"><span class=\"{mark_class}\">{sigil}</span><span class=\"sigil-text\">{}</span></p>",
            chant_line_html(&line.text)
        ));
    }
    flush(&mut sb, &mut prose, &mut prose_blocks, &mut pending_gap);
    sb.push_str("</div>");
    sb
}

/// A hymn: one paragraph per stanza, one span per metrical line, a
/// standalone Amen folded into the verse before it.
pub fn render_hymn_stanzas(text: &str) -> String {
    let hymn = parse_hymn(text);
    let mut sb = String::from("<div class=\"hymn-verses\">");
    if !hymn.title.is_empty() {
        match hymn_rubric_text(&hymn.title) {
            Some(rubric) => write_hymn_rubric(&mut sb, rubric),
            None => sb.push_str(&format!("<p class=\"hymn-latin\" lang=\"la\">{}</p>", esc_text(&hymn.title))),
        }
    }
    let mut opening = true;
    for (i, stanza) in hymn.stanzas.iter().enumerate() {
        if is_hymn_amen(stanza) && i > 0 {
            continue;
        }
        if let Some(rubrics) = hymn_rubric_stanza(stanza) {
            for rubric in rubrics {
                write_hymn_rubric(&mut sb, rubric);
            }
            continue;
        }
        let class = if opening { "hymn-stanza hymn-stanza-opening" } else { "hymn-stanza" };
        opening = false;
        sb.push_str(&format!("<p class=\"{class}\">"));
        let join_amen = hymn.stanzas.get(i + 1).is_some_and(|s| is_hymn_amen(s));
        for (j, line) in stanza.iter().enumerate() {
            sb.push_str("<span class=\"hymn-line\">");
            sb.push_str(&esc_cross(line));
            if join_amen && j == stanza.len() - 1 {
                sb.push_str(&format!("<span class=\"hymn-amen\">{}</span>", esc_cross(&hymn.stanzas[i + 1][0])));
            }
            sb.push_str("</span>");
        }
        sb.push_str("</p>");
    }
    sb.push_str("</div>");
    sb
}

fn write_hymn_rubric(sb: &mut String, rubric: &str) {
    sb.push_str(&format!("<p class=\"rubric hymn-rubric\">{}</p>", esc_text(rubric)));
}

fn is_hymn_amen(stanza: &[String]) -> bool {
    stanza.len() == 1 && stanza[0].trim().trim_end_matches(['.', '!', ' ']).eq_ignore_ascii_case("amen")
}

/// The Gloria Patri as two pointed lines.
pub fn render_gloria_patri(text: &str) -> String {
    let trimmed = text.trim();
    let lines: Vec<&str> = trimmed.split('\n').collect();
    let (line1, line2) = if lines.len() >= 2 { (lines[0].trim(), lines[1].trim()) } else { (text.trim(), "") };
    let mut sb = String::from("<p class=\"gloria-patri\">");
    sb.push_str(&chant_line_html(line1));
    if !line2.is_empty() {
        sb.push_str("<br>");
        sb.push_str(&chant_line_html(line2));
    }
    sb.push_str("</p>");
    sb
}

#[cfg(test)]
#[path = "html_tests.rs"]
mod html_tests;
