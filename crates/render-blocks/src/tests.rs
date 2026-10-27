use std::path::Path;

use calendar::{CalendarData, Date, MoveableDates};
use liturgy::{
    ChantScore, ElementType, OfficeElement, OfficeHour, OfficeSection, Posture, PostureAnchor, PostureCue, PrayerForm, RubricSpan,
    ScoreSpan, Unrepeated, VoiceSpan,
};
use office::{Engine, HOUR_NAMES};
use tools::fs::FsData;

use super::*;

fn elem(kind: ElementType, text: &str) -> OfficeElement {
    OfficeElement::new(kind, text)
}

fn blocks(e: &OfficeElement) -> Vec<Block> {
    element_blocks(std::slice::from_ref(e))
}

fn styles(b: &Block) -> Vec<RunStyle> {
    b.runs.iter().map(|r| r.style).collect()
}

#[test]
fn pointed_verses_mark_the_mediant_and_number_the_gutter() {
    let out = blocks(&elem(
        ElementType::Canticle,
        "Song of the Three Children\n\n\
         O ALL ye Works of the Lord, bless ye the Lord: * praise him, and magnify him forever.\n\
         2 O ye Angels of the Lord, bless ye the Lord: * O ye Heavens, bless ye the Lord.\n",
    ));
    let verses: Vec<&Block> = out.iter().filter(|b| b.kind == BlockKind::Verse).collect();
    assert_eq!(verses.len(), 2, "{out:#?}");
    assert!(verses[0].drop_cap && !verses[1].drop_cap);
    assert_eq!(verses[1].marker, "2");
    // The drop-cap opening is softened, as on the web.
    assert!(verses[0].plain_text().starts_with("O all ye Works"), "{}", verses[0].plain_text());
    assert_eq!(styles(verses[1]), [RunStyle::Plain, RunStyle::Mediant, RunStyle::Plain, RunStyle::Plain]);
    assert_eq!(verses[1].runs[1].text, "\u{a0}*");
}

#[test]
fn posture_cues_fall_at_their_anchors() {
    let mut psalm = elem(ElementType::Psalm, "Psalm 95\n\nO COME, let us sing * unto the Lord.\n2 Let us come * before his presence.\n");
    psalm.postures = vec![
        PostureCue::new(Posture::Sit, PostureAnchor::AfterMediant(0)),
        PostureCue::new(Posture::Stand, PostureAnchor::AfterMediant(1)),
    ];
    let mut gloria =
        elem(ElementType::PsalmDoxology, "Glory be to the Father, * and to the Son;\nAs it was in the beginning, * world without end.");
    gloria.postures = vec![
        PostureCue::new(Posture::Bow, PostureAnchor::BeforeVerse(0)),
        PostureCue::new(Posture::StandUpright, PostureAnchor::BeforeVerse(1)),
    ];
    let out = element_blocks(&[psalm, gloria]);
    let verses: Vec<&Block> = out.iter().filter(|b| b.kind == BlockKind::Verse).collect();
    let cues = |b: &Block| b.runs.iter().filter(|r| r.style == RunStyle::Posture).map(|r| r.text.clone()).collect::<Vec<_>>();
    assert_eq!(verses.len(), 2, "{out:#?}");
    assert!(verses[0].plain_text().ends_with("\u{a0}* Sit. unto the Lord."), "{}", verses[0].plain_text());
    assert_eq!(cues(verses[1]), ["Stand."]);
    let gloria = out.last().unwrap();
    assert_eq!(gloria.kind, BlockKind::GloriaPatri);
    assert_eq!(styles(gloria).iter().filter(|s| **s == RunStyle::Break).count(), 1, "{gloria:#?}");
    assert_eq!(cues(gloria), ["Bow.", "Stand upright."]);
    assert!(gloria.plain_text().starts_with("Bow. Glory be"), "{}", gloria.plain_text());
}

#[test]
fn an_announced_antiphon_is_its_own_kind() {
    let mut announced = elem(ElementType::Antiphon, "Let my prayer * O Lord, enter into thy presence.");
    announced.announce = true;
    let full = elem(ElementType::Antiphon, "Let my prayer * O Lord, enter into thy presence.");
    let out = element_blocks(&[announced, full]);
    assert_eq!(out.iter().map(|b| b.kind).collect::<Vec<_>>(), [BlockKind::AnnouncedAntiphon, BlockKind::Antiphon]);
    assert_eq!(out[0].plain_text(), "Ant. Let my prayer.");
    // The unrepeated-words note follows its antiphon's setting.
    let psalm = || {
        let mut p = elem(ElementType::Psalm, "Psalm 144\n\nBLESSED be the Lord my strength * who teacheth my hands to war.\n");
        p.unrepeated = Some(Unrepeated { words: 2, named: String::new() });
        p
    };
    let note = |announce: bool| {
        let mut antiphon = elem(ElementType::Antiphon, "Blessed be * the Lord my strength and my fortress.");
        antiphon.announce = announce;
        element_blocks(&[antiphon, psalm()])[1].kind
    };
    assert_eq!(note(true), BlockKind::AnnouncementNote);
    assert_eq!(note(false), BlockKind::AntiphonNote);
}

#[test]
fn secret_words_and_the_cross_are_their_own_runs() {
    let mut e = elem(ElementType::Prayer, "Our Father. And lead us not ✠ into temptation.");
    e.voice = vec![VoiceSpan::new("Our Father. ", true, None), VoiceSpan::new("And lead us not ✠ into temptation.", false, None)];
    let out = blocks(&e);
    assert_eq!(out.len(), 1);
    assert_eq!(styles(&out[0]), [RunStyle::Plain, RunStyle::Secret, RunStyle::Cross, RunStyle::Secret]);
    // Prayed words quoted in a rubric are their own runs too.
    let mut e = elem(ElementType::Rubric, "Then is said Glory be.");
    e.rubric_spans =
        vec![RubricSpan { text: "Then is said ".into(), prayed: false }, RubricSpan { text: "Glory be.".into(), prayed: true }];
    assert_eq!(styles(&blocks(&e)[0]), [RunStyle::Plain, RunStyle::Prayed]);
}

/// A rubric's glossed term is a run of its own carrying its note, as the web's `.gloss-term`;
/// quoted prayer words are never glossed.
#[test]
fn a_rubric_s_glossed_term_carries_its_note() {
    let mut e = elem(ElementType::Rubric, "Our Father is said secretly.");
    e.rubric_spans =
        vec![RubricSpan { text: "Our Father".into(), prayed: true }, RubricSpan { text: " is said secretly.".into(), prayed: false }];
    let out = blocks(&e);
    let texts: Vec<&str> = out[0].runs.iter().map(|r| r.text.as_str()).collect();
    assert_eq!(texts, ["Our Father", " is said ", "secretly", "."]);
    let glossed: Vec<&str> = out[0].runs.iter().filter_map(|r| r.gloss.map(|g| g.title)).collect();
    assert_eq!(glossed, ["Secretly"]);
    assert_eq!(out[0].runs[2].style, RunStyle::Plain);

    let whole = blocks(&elem(ElementType::Rubric, "Our Father entirely in secret. Then confession is made:"));
    let gloss = whole[0].runs.iter().find_map(|r| r.gloss).expect("glossed");
    assert_eq!(gloss.title, "Entirely in secret");
    assert!(blocks(&elem(ElementType::Rubric, "All kneel."))[0].runs.iter().all(|r| r.gloss.is_none()));
}

#[test]
fn a_hymn_s_rubrics_stand_in_its_column_and_its_amen_folds_in() {
    let out = blocks(&elem(
        ElementType::Hymn,
        "/:The first stanza of the following hymn is said kneeling.:/\n\nStar of ocean fairest,\n\n/:Stand.:/\n\nVirgin thou immortal,\n",
    ));
    let kinds: Vec<BlockKind> = out.iter().map(|b| b.kind).collect();
    assert_eq!(kinds, [BlockKind::Heading, BlockKind::HymnRubric, BlockKind::Stanza, BlockKind::HymnRubric, BlockKind::Stanza]);
    // A closing Amen folds into the last stanza.
    let out = blocks(&elem(ElementType::Hymn, "Now that the daylight fills the sky,\nWe lift our hearts to God on high,\n\nAmen."));
    assert_eq!(out[0].kind, BlockKind::Heading);
    let stanzas: Vec<&Block> = out.iter().filter(|b| b.kind == BlockKind::Stanza).collect();
    assert_eq!(stanzas.len(), 1, "{out:#?}");
    assert!(stanzas[0].drop_cap);
    assert!(stanzas[0].runs.iter().any(|r| r.style == RunStyle::Break));
    assert!(stanzas[0].plain_text().ends_with("Amen."));
}

#[test]
fn compline_opens_with_a_heading_and_collapsible_sections_do_not() {
    let hour = OfficeHour {
        form: PrayerForm::Private,
        date: Date::new(2026, 1, 5),
        hour: "Compline".into(),
        title: "Compline".into(),
        season: None,
        feast: String::new(),
        rank: None,
        color: None,
        sections: vec![
            OfficeSection { label: String::new(), collapsible: false, elements: vec![elem(ElementType::Prayer, "Pray, Father.")] },
            OfficeSection { label: "Before the Office".into(), collapsible: true, elements: vec![elem(ElementType::Prayer, "Open.")] },
        ],
        decisions: Vec::new(),
    };
    let sections = hour_sections(&hour);
    assert_eq!(sections[0].blocks[0].plain_text(), "Opening");
    assert_eq!(sections[1].blocks[0].kind, BlockKind::Paragraph);
}

/// The text of rendered HTML, entities decoded.
fn html_text(html: &str) -> String {
    let mut out = String::new();
    let mut in_tag = false;
    for c in html.chars() {
        match c {
            '<' => in_tag = true,
            '>' => in_tag = false,
            c if !in_tag => out.push(c),
            _ => {}
        }
    }
    out.replace("&#34;", "\"").replace("&#39;", "'").replace("&lt;", "<").replace("&gt;", ">").replace("&amp;", "&")
}

fn squeeze(s: &str) -> String {
    s.chars().filter(|c| !c.is_whitespace()).collect()
}

/// Every word the web shows, the blocks show, in the same order: the two
/// renderers share one grammar, so a native app cannot silently drop text.
#[test]
fn blocks_carry_the_same_text_as_the_web() {
    let src = FsData::new(Path::new(env!("CARGO_MANIFEST_DIR")).join("../../data"));
    let engine = Engine::load(&src).expect("engine");
    let data = CalendarData::load(&src).expect("calendar data");
    let year = 2026;
    let days = tools::year::office_days(&data, year).expect("office days");
    let moveable = MoveableDates::compute(year);
    let mut checked = 0;
    for (i, day) in days.iter().enumerate() {
        // Every day in private prayer; the led forms on a weekly sample.
        let forms: &[PrayerForm] = if i % 7 == 0 { &PrayerForm::ALL } else { &[PrayerForm::Private] };
        for &form in forms {
            for hour_name in HOUR_NAMES {
                let hour = engine.compose_hour(hour_name, day, &moveable, form).expect("compose");
                for section in &hour.sections {
                    let html = render_html::html::render_section_elements(&section.elements);
                    let section_blocks = element_blocks(&section.elements);
                    // A score's words (the Marian antiphon's, say) are read either way.
                    for on in [false, true] {
                        let (native, web) = (shown(&section_blocks, on), web_words(&html, on));
                        assert_eq!(native, web, "{hour_name} {} {} section {:?} chant {on}", day.date, form.as_str(), section.label);
                    }
                    // Every rubric term the web glosses, the apps gloss too.
                    let glossed = section_blocks.iter().flat_map(|b| &b.runs).filter(|r| r.gloss.is_some()).count();
                    assert_eq!(glossed, html.matches("class=\"gloss-term\"").count(), "{hour_name} {} glosses", day.date);
                    checked += 1;
                }
            }
        }
    }
    assert!(checked > 10_000, "only {checked} sections checked");
}

const HYMN: &str = "To thee before the close of day,\nCreator of the world, we pray\n\nFrom all ill dreams defend our eyes,\nFrom nightly fears and fantasies;\n\nAmen.";
const MARIAN: &str = "Hail, holy Queen, * Mother of mercy,\nour life, our sweetness and our hope.\n\nV. Pray for us, O holy Mother of God.\nR. That we may be made worthy.\n\nLet us pray.\n\nAlmighty, everlasting God, who didst prepare.";

fn scored(kind: ElementType, text: &str, gabc: &str, sets: ScoreSpan) -> OfficeElement {
    let mut e = elem(kind, text);
    if kind == ElementType::Antiphon {
        e.label = "Salve Regina".into();
        e.slot_ref = "marian-antiphon".into();
    }
    e.score = Some(ChantScore { gabc: gabc.into(), sets });
    e
}

/// The words a front shows with Chant off, or (`on`) the words its scores set and the text around them.
fn shown(out: &[Block], on: bool) -> String {
    let hidden = if on { ChantVariant::Off } else { ChantVariant::On };
    out.iter().filter(|b| b.chant != hidden).map(|b| squeeze(&b.plain_text())).collect()
}

/// The web's words as a reader sees them with Chant `on` or off: with it on, each score by its
/// stanza number and label, the words it replaces left out.
fn web_words(html: &str, on: bool) -> String {
    let mut out = String::new();
    let mut rest = html;
    while let Some(i) = rest.find('<') {
        out.push_str(&squeeze(&html_text(&rest[..i])));
        rest = &rest[i..];
        let tag = &rest[..rest.find('>').unwrap() + 1];
        let name: String = tag[1..].chars().take_while(char::is_ascii_alphanumeric).collect();
        let score = on && tag.contains("class=\"chant-score");
        let hidden = if on { "data-chant-variant=\"off\"" } else { "data-chant-variant=\"on\"" };
        if score || tag.contains(hidden) {
            let end = element_end(rest, &name);
            if score {
                let element = &rest[..end];
                if let Some(n) = element.split("class=\"chant-stanza-number\" aria-hidden=\"true\">").nth(1) {
                    out.push_str(n.split('<').next().unwrap());
                }
                out.push_str(&squeeze(&html_text(tag.split("aria-label=\"").nth(1).unwrap().split('"').next().unwrap())));
            }
            rest = &rest[end..];
        } else {
            rest = &rest[tag.len()..];
        }
    }
    out.push_str(&squeeze(&html_text(rest)));
    out
}

/// Where the `name` element that `html` opens with closes.
fn element_end(html: &str, name: &str) -> usize {
    let (open, close) = (format!("<{name}"), format!("</{name}>"));
    let mut depth = 0;
    let mut i = 0;
    while i < html.len() {
        if html[i..].starts_with(&open) {
            depth += 1;
        } else if html[i..].starts_with(&close) {
            depth -= 1;
            if depth == 0 {
                return i + close.len();
            }
        }
        i += html[i..].chars().next().map_or(1, char::len_utf8);
    }
    html.len()
}

#[test]
fn a_hymn_s_tune_stands_before_the_stanza_it_sets() {
    let gabc = "name: t;\n%%\n(c4) To(g) thee(h) be(j)fore(h) the(g) close(h) of(g) day,(g.) (;) Cre(g)a(h)tor(j) of(h) the(g) world,(h) we(g) pray(g.) (::)";
    let e = scored(ElementType::Hymn, HYMN, gabc, ScoreSpan::FirstStanza);
    let out = blocks(&e);
    let kinds: Vec<(BlockKind, ChantVariant)> = out.iter().map(|b| (b.kind, b.chant)).collect();
    assert_eq!(
        kinds,
        [
            (BlockKind::Heading, ChantVariant::Always),
            (BlockKind::Score, ChantVariant::On),
            (BlockKind::Stanza, ChantVariant::Off),
            (BlockKind::Stanza, ChantVariant::Always)
        ]
    );
    assert!(out[1].drop_cap && out[1].score == gabc);
    assert_eq!(out[1].plain_text(), "To thee before the close of day, Creator of the world, we pray");
}

#[test]
fn a_whole_hymn_is_a_score_per_stanza_numbered_after_the_first() {
    let gabc = "name: t;\n%%\n(c4) To(g) thee(h) be(j)fore(h) the(g) close(h) of(g) day,(g.) (::) From(g) all(h) ill(j) dreams(h) de(g)fend(h) our(g) eyes,(g.) (::) A(gh)men.(g) (::)";
    let out = blocks(&scored(ElementType::Hymn, HYMN, gabc, ScoreSpan::WholeHymn));
    let scores: Vec<&Block> = out.iter().filter(|b| b.kind == BlockKind::Score).collect();
    assert_eq!(scores.len(), 2);
    assert!(scores[0].drop_cap && scores[0].marker.is_empty());
    assert!(!scores[1].drop_cap && scores[1].marker == "2.");
    assert!(scores[1].score.ends_with("A(gh)men.(g) (::)"), "{}", scores[1].score);
    assert!(out.iter().filter(|b| b.kind == BlockKind::Stanza).all(|b| b.chant == ChantVariant::Off));
}

/// With Chant off a scored piece reads as it did unscored; with it on, the blocks name what the
/// web's scores set, around the same unscored text.
#[test]
fn scored_blocks_carry_the_web_s_words_either_way() {
    let hymn_gabc = "name: t;\n%%\n(c4) To(g) thee(h) (::) From(g) all(h) A(g)men.(g) (::)";
    let marian_gabc = "name: t;\n%%\n(c4) Hail,(g) ho(h)ly(g) Queen.(g) (::)";
    for e in [
        scored(ElementType::Hymn, HYMN, hymn_gabc, ScoreSpan::FirstStanza),
        scored(ElementType::Hymn, HYMN, hymn_gabc, ScoreSpan::WholeHymn),
        scored(ElementType::Antiphon, MARIAN, marian_gabc, ScoreSpan::Antiphon),
    ] {
        let unscored = OfficeElement { score: None, ..e.clone() };
        let out = blocks(&e);
        assert_eq!(shown(&out, false), shown(&blocks(&unscored), false), "{:?}", e.score);
        let html = render_html::html::render_section_elements(std::slice::from_ref(&e));
        assert_eq!(shown(&out, false), squeeze(&html_text(&render_html::html::render_section_elements(&[unscored]))));
        assert_eq!(shown(&out, true), web_words(&html, true), "{:?}", e.score);
    }
}
