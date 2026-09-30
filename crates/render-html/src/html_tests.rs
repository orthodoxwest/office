use liturgy::{ElementType, OfficeElement, Posture, PostureAnchor, PostureCue, RubricSpan, VoiceRole, VoiceSpan};

use crate::html::*;

fn elem(kind: ElementType, text: &str) -> OfficeElement {
    OfficeElement::new(kind, text)
}

fn render(e: &OfficeElement) -> String {
    render_office_element(e, None)
}

fn short_responsory(text: &str) -> String {
    render_block(text, Mode::PreserveLines, true)
}

#[track_caller]
fn has(html: &str, want: &str) {
    assert!(html.contains(want), "missing {want:?} in {html}");
}

#[track_caller]
fn lacks(html: &str, unwanted: &str) {
    assert!(!html.contains(unwanted), "unexpected {unwanted:?} in {html}");
}

#[test]
fn benedicite_space_numbered_verses() {
    let html = render_psalm_verses(
        "Song of the Three Children\n\n\
         O ALL ye Works of the Lord, bless ye the Lord: * praise him, and magnify him forever.\n\
         2 O ye Angels of the Lord, bless ye the Lord: * O ye Heavens, bless ye the Lord.\n\
         10 O let the Earth bless the Lord: * yea, let it praise him, and magnify him for ever.\n",
        &[],
    );
    has(&html, r#"<p class="verse numbered"><span class="verse-num">2</span>"#);
    has(&html, r#"<span class="verse-num">10</span>"#);
    lacks(&html, ">2 O ye Angels");
    // Drop-cap opening: single-letter O kept; multi-letter ALL softened.
    has(&html, "O All ye Works of the Lord");
    lacks(&html, "O ALL ye");
}

#[test]
fn soften_drop_cap_opening_cases() {
    let cases = [
        ("GOD be merciful unto us", "God be merciful unto us"),
        ("HAVE mercy upon me, O God", "Have mercy upon me, O God"),
        ("BLESSED are those", "Blessed are those"),
        ("WHEREWITHAL shall a young man", "Wherewithal shall a young man"),
        ("MY SOUL cleaveth to the dust", "My Soul cleaveth to the dust"),
        ("O GIVE thanks unto the Lord", "O Give thanks unto the Lord"),
        ("O ALL ye Works of the Lord", "O All ye Works of the Lord"),
        // Single-letter capitals are left alone.
        ("O God", "O God"),
        ("I will magnify thee", "I will magnify thee"),
        // Trailing punctuation is stripped from the letter run, then restored.
        ("GOD, be merciful", "God, be merciful"),
        ("BLESSED!", "Blessed!"),
        ("MY SOUL,", "My Soul,"),
        // Leading and trailing whitespace is preserved exactly.
        ("  GOD be merciful", "  God be merciful"),
        ("GOD be merciful  ", "God be merciful  "),
        ("  GOD  ", "  God  "),
        ("\tGOD be", "\tGod be"),
        // A pure-punctuation token ends the opening run without rewriting it.
        ("... GOD be", "... GOD be"),
        ("— GOD be", "— GOD be"),
        // Already mixed or sentence case: unchanged.
        ("Blessed be the Lord God of Israel", "Blessed be the Lord God of Israel"),
        ("That thy way may be known", "That thy way may be known"),
        ("God be merciful", "God be merciful"),
        // A non-letter inside the letter run fails all-caps.
        ("PSA6LM is", "PSA6LM is"),
        // Trailing digits are kept after the title-cased letter run.
        ("PSALM67 is", "Psalm67 is"),
        ("", ""),
        ("   ", "   "),
    ];
    for (input, want) in cases {
        assert_eq!(soften_drop_cap_opening(input), want, "soften_drop_cap_opening({input:?})");
    }
}

#[test]
fn psalm_softens_drop_cap_opening_only() {
    let html = render_psalm_verses(
        "Psalm 67\n\n\
         GOD be merciful unto us, and bless us * and shew us the light of his countenance.\n\
         2. That thy way may be known upon earth * thy saving health among all nations.\n",
        &[],
    );
    has(&html, ">God be merciful unto us");
    lacks(&html, ">GOD be merciful");
    // Numbered verses keep their source capitalisation.
    has(&html, "That thy way may be known upon earth");
}

/// Softening applies to the first verse of each block, including the first
/// after a mid-canticle section break, and never to later verses.
#[test]
fn psalm_softens_drop_cap_after_section_break() {
    let html = render_psalm_verses(
        "Benedicite\n\n\
         O ALL ye Works of the Lord, bless ye the Lord: * praise him forever.\n\
         2 O ye Angels of the Lord, bless ye the Lord: * O ye Heavens, bless ye the Lord.\n\
         [section: Part II]\n\
         O LET the Earth bless the Lord: * yea, let it praise him forever.\n\
         10 O ye Mountains and Hills, bless ye the Lord: * praise him forever.\n",
        &[],
    );
    has(&html, "O All ye Works of the Lord");
    has(&html, "O Let the Earth bless the Lord");
    lacks(&html, "O LET the Earth");
    lacks(&html, "O ALL ye Works");
    has(&html, "O ye Mountains and Hills");
}

#[test]
fn posture_cues_follow_their_mediants_and_precede_the_doxology_lines() {
    let mut psalm = elem(
        ElementType::Psalm,
        "Psalm 93\n\nTHE Lord is King * and hath put on glorious apparel.\n2. He hath made the round world so sure * that it cannot be moved.\n3. Without a mediant",
    );
    psalm.postures = vec![
        PostureCue::new(Posture::Sit, PostureAnchor::AfterMediant(0)),
        PostureCue::new(Posture::Stand, PostureAnchor::AfterMediant(2)),
        PostureCue::new(Posture::Bow, PostureAnchor::BeforeVerse(1)),
    ];
    let mut gloria = elem(
        ElementType::PsalmDoxology,
        "Glory be to the Father, * and to the Holy Ghost;\nAs it was in the beginning, * world without end. Amen.",
    );
    gloria.postures = vec![
        PostureCue::new(Posture::Bow, PostureAnchor::BeforeVerse(0)),
        PostureCue::new(Posture::StandUpright, PostureAnchor::BeforeVerse(1)),
    ];
    let html = render_section_elements(&[psalm, gloria]);
    has(&html, r#"<span class="mediant">*</span> <span class="posture">Sit.</span> and hath put on"#);
    has(&html, r#"<span class="verse-body"><span class="posture">Bow.</span> He hath made"#);
    has(&html, r#"Without a mediant <span class="posture">Stand.</span></span>"#);
    has(&html, r#"<span class="source-line"><span class="posture">Bow.</span> Glory be"#);
    has(&html, r#"<span class="source-line"><span class="posture">Stand upright.</span> As it was"#);
    // Without cues nothing is added.
    lacks(&render_psalm_verses("Psalm 93\n\nTHE Lord is King * and hath put on glorious apparel.\n", &[]), "posture");
}

#[test]
fn section_elements_merge_psalm_doxology_into_psalm_block() {
    let mut psalm = elem(ElementType::Psalm, "Psalm 67\n\n1. Be merciful unto us * and bless us.");
    psalm.label = "Psalm 67".into();
    let html = render_section_elements(&[psalm, elem(ElementType::PsalmDoxology, "Glory be to the Father,\nas it was in the beginning.")]);
    has(&html, r#"<div class="psalm">"#);
    has(&html, r#"<div class="psalm"><h3 class="item-label">Psalm 67</h3>"#);
    has(&html, r#"<div class="psalm-verses">"#);
    has(&html, r#"<p class="gloria-patri">"#);
    has(
        &html,
        r#"</div><p class="gloria-patri"><span class="source-line">Glory be to the Father,</span><span class="source-line">as it was"#,
    );
    assert!(html.ends_with("</p></div>"), "{html}");
}

#[test]
fn collect_reflows_prose_and_preserves_semantic_lines() {
    let html = render(&elem(
        ElementType::Collect,
        "Almighty God, who hast brought us\nto the beginning of this day.\n\nV. O Lord, hear my prayer.\nR. And let my cry come unto thee.",
    ));
    has(
        &html,
        r#"<div class="collect"><div class="liturgical-block"><p class="plain-line">Almighty God, who hast brought us to the beginning of this day.</p>"#,
    );
    lacks(&html, "brought us<br>to");
    has(&html, r#"<span class="sigil">℣.</span>"#);
    has(&html, r#"<span class="sigil">℟.</span>"#);
    has(&html, r#"<div class="liturgical-gap"></div>"#);
}

#[test]
fn prayer_reflows_source_lines() {
    let html = render(&elem(ElementType::Prayer, "Thy kingdom come.\nThy will be done."));
    has(&html, "Thy kingdom come. Thy will be done.");
}

#[test]
fn secret_prayer_voice_spans() {
    let mut e = elem(ElementType::Prayer, "Our Father, who art in heaven.\nThy kingdom come.");
    e.voice = vec![VoiceSpan::new("Our Father", true, None), VoiceSpan::new(", who art in heaven.\nThy kingdom come.", false, None)];
    let html = render(&e);
    has(&html, r#"<span class="spoken-text">Our Father</span>"#);
    has(&html, r#"<span class="secret-text">, who art in heaven.</span>"#);
    has(&html, r#"<span class="secret-text">Thy kingdom come.</span>"#);
    has(&html, r#"heaven.</span> <span class="secret-text">Thy kingdom"#);
}

#[test]
fn partly_secret_prayer_voice_spans() {
    let mut e = elem(ElementType::Prayer, "Our Father, middle.\nAnd lead us not into temptation,\nBut deliver us from evil.");
    e.voice = vec![
        VoiceSpan::new("Our Father", true, None),
        VoiceSpan::new(", middle.\n", false, None),
        VoiceSpan::new("And lead us not into temptation,\nBut deliver us from evil.", true, None),
    ];
    let html = render(&e);
    has(&html, r#"<span class="spoken-text">Our Father</span><span class="secret-text">, middle.</span>"#);
    has(&html, r#"<span class="spoken-text">And lead us not into temptation,</span>"#);
    has(&html, r#"<span class="spoken-text">But deliver us from evil.</span>"#);
}

#[test]
fn marian_antiphon_preserves_verse_and_reflows_prayer() {
    let text = "[Ave Regina Caelorum]\n\nQueen of the heavens, we hail thee,\nHail thee, Lady of all the Angels;\nTo thee the faithful send up their sighs.\n\nV. Vouchsafe that I may praise thee.\nR. Give me strength.\n\nLet us pray.\n\nGrant us, O merciful God, protection in our weakness:\nthat we may rise again from our sins.";
    let html = render_marian_antiphon(text);
    has(&html, r#"<div class="collect"><div class="liturgical-block"><p class="plain-line">Grant us"#);
    lacks(&html, r#"<p class="chant-line">Grant us"#);
    // The opening pair shares one block so a two-line drop cap can float
    // across both source lines; later verse lines stay discrete.
    has(&html, r#"<p class="chant-line chant-line-opening">Queen of the heavens, we hail thee,<br>Hail thee, Lady of all the Angels;</p>"#);
    has(&html, r#"<p class="chant-line">To thee the faithful send up their sighs.</p>"#);
    has(&html, "Grant us, O merciful God, protection in our weakness: that we may rise again from our sins.");
}

#[test]
fn marian_antiphon_styles_incipit_mediant() {
    let html =
        render_marian_antiphon("Mary we hail thee * Mother and Queen compassionate;\nMary our comfort, life, and hope, we hail thee.");
    has(
        &html,
        &format!(
            r#"<p class="chant-line chant-line-opening">Mary we hail thee{MEDIANT}Mother and Queen compassionate;<br>Mary our comfort, life, and hope, we hail thee.</p>"#
        ),
    );
}

#[test]
fn antiphon_styles_mediant() {
    let html = render(&elem(ElementType::Antiphon, "The Lord said * to my Lord: Sit thou at my right hand."));
    has(&html, &format!("The Lord said{MEDIANT}to my Lord: Sit thou at my right hand."));
    // The mark ends its half-verse; a wrap may follow it but never precede it.
    lacks(&html, r#" <span class="mediant">"#);
}

#[test]
fn announced_antiphon_prints_incipit_only() {
    let mut e = elem(ElementType::Antiphon, "Do away, O Lord, * mine offenses.");
    e.announce = true;
    let html = render(&e);
    has(&html, r#"class="antiphon antiphon-announce""#);
    has(&html, "Do away, O Lord.");
    lacks(&html, r#"class="mediant""#);
    lacks(&html, "mine offenses");
}

#[test]
fn announced_antiphon_preserves_terminal_punctuation() {
    let mut e =
        elem(ElementType::Antiphon, "I have yet many things to say unto you, but ye cannot bear them now. * Howbeit, when He is come.");
    e.announce = true;
    let html = render(&e);
    has(&html, "but ye cannot bear them now.");
    lacks(&html, "now..");
}

#[test]
fn response_styles_mediant() {
    let html = render_liturgical_block("R. Great is our Lord * and great is his power.");
    has(&html, &format!(r#"<span class="sigil-text">Great is our Lord{MEDIANT}and great is his power.</span>"#));
}

#[test]
fn opening_acclamation_is_not_an_antiphon() {
    let html = render(&elem(ElementType::OpeningAcclamation, "Praise be to thee, O Lord, King of eternal glory."));
    has(&html, r#"class="opening-acclamation""#);
    lacks(&html, "Ant.");
}

#[test]
fn rubric_keeps_quoted_prayer_words_black() {
    let mut e = elem(ElementType::Rubric, "Our Father is said secretly.");
    e.rubric_spans =
        vec![RubricSpan { text: "Our Father".into(), prayed: true }, RubricSpan { text: " is said secretly.".into(), prayed: false }];
    has(&render(&e), r#"<span class="rubric-prayed">Our Father</span> is said secretly."#);
}

#[test]
fn short_responsory_drops_only_first_response_sigil() {
    let html = short_responsory("R. The Lord hath set his love upon me.\nV. He shall deliver me.\nR. The Lord hath set his love upon me.");
    has(&html, r#"class="response-line short-responsory-opening"><span class="sigil-text">The Lord"#);
    assert_eq!(html.matches(">℟.</span>").count(), 1, "{html}");
}

#[test]
fn short_responsory_marks_its_block_for_the_dialogue_edge() {
    let html = short_responsory("R. Heal my soul.\nR. Heal my soul.\nGlory be to the Father.\nR. Heal my soul.");
    assert!(html.starts_with(r#"<div class="liturgical-block short-responsory">"#), "{html}");
    has(&html, r#"<p class="plain-line">Glory be to the Father.</p>"#);
    lacks(&render_liturgical_block("V. O Lord, hear my prayer."), "short-responsory");
}

#[test]
fn short_responsory_opening_versicle_keeps_an_ordinary_pair() {
    // Compline's slot holds a versicle and its response, not a responsory.
    let html = short_responsory("V. Keep us, O Lord, as the apple of an eye.\nR. Hide us under the shadow of thy wings.");
    lacks(&html, "short-responsory-opening");
    has(&html, r#"<span class="sigil">℟.</span><span class="sigil-text">Hide us"#);
}

#[test]
fn preserved_prose_lines_hang_as_source_lines() {
    let html =
        render_liturgical_block("Glory be to the Father, * and to the Holy Ghost;\nAs it was in the beginning, * world without end. Amen.");
    has(&html, r#"<p class="plain-line"><span class="source-line">Glory be to the Father,"#);
    has(&html, r#"Holy Ghost;</span><span class="source-line">As it was"#);
    lacks(&html, "<br>");
    let single = render_liturgical_block("Almighty God have mercy upon us.");
    has(&single, r#"<p class="plain-line">Almighty God have mercy upon us.</p>"#);
}

#[test]
fn dialogue_and_corporate_lord_prayer_roles() {
    let dialogue = render_liturgical_block("V. Kyrie, eleison.\nR. Christe, eleison.\nAll: Kyrie, eleison.");
    has(&dialogue, ">℣.</span>");
    has(&dialogue, ">℟.</span>");
    has(&dialogue, ">All:</span>");
    let mut prayer = elem(
        ElementType::CorporateLordPrayer,
        "Our Father, who art in heaven.\nAnd lead us not into temptation,\nBut deliver us from evil. Amen.",
    );
    prayer.voice = vec![
        VoiceSpan::new("Our Father, who art in heaven.\nAnd lead us not into temptation,\n", true, Some(VoiceRole::Officiant)),
        VoiceSpan::new("But deliver us from evil. Amen.", true, Some(VoiceRole::Response)),
    ];
    let html = render(&prayer);
    has(&html, r#"corporate-lord-prayer-officiant">Our Father"#);
    has(&html, r#">℟.</span><span class="sigil-text">But deliver us from evil. Amen."#);
    let fallback = render(&elem(ElementType::CorporateLordPrayer, "Our Father, who art in heaven."));
    lacks(&fallback, "corporate-lord-prayer-officiant");
    has(&fallback, r#"<p class="plain-line">Our Father, who art in heaven.</p>"#);
}

#[test]
fn versicle_styles_mediant() {
    let html = render_liturgical_block("V. Serve the Lord in fear: * and rejoice unto him with reverence.");
    has(&html, &format!(r#"<span class="sigil-text">Serve the Lord in fear:{MEDIANT}and rejoice unto him with reverence.</span>"#));
}

#[test]
fn hymn_stanzas_preserve_verse_lines() {
    let html = render_hymn_stanzas("Latin title\n\nFirst verse line,\nSecond verse line.\n\nAnother stanza.");
    has(
        &html,
        r#"<p class="hymn-stanza hymn-stanza-opening"><span class="hymn-line">First verse line,</span><span class="hymn-line">Second verse line.</span></p>"#,
    );
    lacks(&html, "<br>");
}

#[test]
fn hymn_joins_amen_coda_to_final_line() {
    let html = render_hymn_stanzas("Title\n\nFirst line,\nSecond line.\n\nAmen.");
    has(&html, r#"<span class="hymn-line">Second line.<span class="hymn-amen">Amen.</span></span>"#);
    lacks(&html, r#"<p class="hymn-stanza hymn-amen">"#);
    has(
        &html,
        r#"<p class="hymn-stanza hymn-stanza-opening"><span class="hymn-line">First line,</span><span class="hymn-line">Second line.<span class="hymn-amen">Amen.</span></span></p>"#,
    );
}

#[test]
fn hymn_keeps_an_opening_amen_and_attaches_only_later_coda() {
    let html = render_hymn_stanzas("Title\n\nAmen.\n\nSecond stanza.\n\nAmen.");
    has(&html, r#"<p class="hymn-stanza hymn-stanza-opening"><span class="hymn-line">Amen.</span></p>"#);
    has(&html, r#"<p class="hymn-stanza"><span class="hymn-line">Second stanza.<span class="hymn-amen">Amen.</span></span></p>"#);
    assert_eq!(html.matches(r#"class="hymn-amen""#).count(), 1, "{html}");
}

#[test]
fn is_hymn_amen_cases() {
    let s = |v: &[&str]| v.iter().map(|l| l.to_string()).collect::<Vec<_>>();
    let cases: [(&[&str], bool); 11] = [
        (&["Amen."], true),
        (&["Amen"], true),
        (&["amen!"], true),
        (&["AMEN."], true),
        (&["  Amen.  "], true),
        (&["Amen.", "Again."], false),
        (&["Amen, amen."], false),
        (&["So be it. Amen."], false),
        (&["First line,"], false),
        (&[], false),
        (&[""], false),
    ];
    for (stanza, want) in cases {
        assert_eq!(is_hymn_amen(&s(stanza)), want, "is_hymn_amen({stanza:?})");
    }
}

#[test]
fn hymn_does_not_mark_non_coda_amen() {
    // Amen glued to the last verse line is a corpus problem; the renderer
    // must not invent the class.
    lacks(&render_hymn_stanzas("Title\n\nLast line ends with Amen."), "hymn-amen");
}

#[test]
fn hymn_rubric_is_instruction_not_latin_title() {
    let html = render_hymn_stanzas(
        "/:The first stanza of the following hymn is said kneeling.:/\n\nStar of ocean fairest,\nMother, God who barest.\n",
    );
    has(&html, r#"<p class="rubric hymn-rubric">The first stanza of the following hymn is said kneeling.</p>"#);
    lacks(&html, "/:");
    lacks(&html, ":/");
    lacks(&html, "hymn-latin");
    lacks(&html, r#"lang="la""#);
    has(&html, r#"<p class="hymn-stanza hymn-stanza-opening"><span class="hymn-line">Star of ocean fairest,</span>"#);
}

#[test]
fn hymn_mid_hymn_rubric() {
    let html = render_hymn_stanzas(
        "Title\n\nThe royal banners forward go.\n\n/:The following stanza is said kneeling.:/\n\nO Cross, our one reliance, hail!\n",
    );
    has(&html, r#"<p class="hymn-latin" lang="la">Title</p>"#);
    has(&html, r#"<p class="rubric hymn-rubric">The following stanza is said kneeling.</p>"#);
    assert_eq!(html.matches("hymn-stanza-opening").count(), 1, "{html}");
    has(&html, r#"<p class="hymn-stanza hymn-stanza-opening"><span class="hymn-line">The royal banners forward go.</span></p>"#);
}

#[test]
fn blessing_uses_versicle_line() {
    let html = render_liturgical_block("Blessing. May the Almighty and merciful Lord grant us a quiet night.");
    // The spelled-out label takes the wide sigil column.
    has(&html, r#"<span class="sigil sigil-word">Blessing.</span>"#);
    has(&html, r#"class="versicle-line""#);
    has(&html, r#"<span class="sigil-text">May the Almighty and merciful Lord grant us a quiet night.</span>"#);
}

#[test]
fn commemoration_heading_preserves_name_and_escapes_markup() {
    let got = render_section_heading("Commemoration of St A & St B <test>");
    has(&got, r#"class="commemoration-kicker">Commemoration of</span> "#);
    has(&got, "St A &amp; St B &lt;test&gt;");
    assert_eq!(render_section_heading("Lauds"), r#"<h2 class="section-heading">Lauds</h2>"#);
}

#[test]
fn silent_triduum_prayers() {
    let mut prayer = elem(ElementType::Prayer, "Our Father, who art in heaven.");
    prayer.voice = vec![VoiceSpan::new(prayer.text.clone(), false, None)];
    let html = render(&prayer);
    lacks(&html, r#"class="spoken-text""#);
    has(&html, r#"<span class="secret-text">Our Father, who art in heaven.</span>"#);
    let mut collect = elem(ElementType::Collect, "Almighty God, behold thy family.\nWho with thee liveth.\nAmen.");
    collect.voice =
        vec![VoiceSpan::new("Almighty God, behold thy family.\n", true, None), VoiceSpan::new("Who with thee liveth.\nAmen.", false, None)];
    let html = render(&collect);
    for want in [
        r#"class="collect""#,
        r#"<span class="spoken-text">Almighty God, behold thy family.</span>"#,
        r#"<span class="secret-text">Who with thee liveth.</span>"#,
        r#"<span class="secret-text">Amen.</span>"#,
    ] {
        has(&html, want);
    }
}

fn psalm_67(incipit: &str) -> OfficeElement {
    let mut e = elem(ElementType::Psalm, "Psalm 67\n\n1. Be merciful unto us * and bless us.");
    e.label = "Psalm 67".into();
    e.incipit = incipit.into();
    e
}

#[test]
fn psalm_label_carries_latin_incipit() {
    let html = render_section_elements(&[psalm_67("Deus misereatur nostri")]);
    has(
        &html,
        concat!(
            r#"<h3 class="item-label">Psalm 67"#,
            "<span class=\"label-sep\" aria-hidden=\"true\">\u{a0}· </span>",
            r#"<span class="psalm-incipit" lang="la">Deus misereatur nostri</span></h3>"#
        ),
    );
}

/// Canticles are labeled by their incipit too.
#[test]
fn canticle_label_carries_latin_incipit() {
    let mut e = elem(ElementType::Canticle, "!Luke 1:46-55\n\n1. My soul doth magnify the Lord * and my spirit.");
    e.label = "Magnificat".into();
    e.incipit = "Magnificat anima mea Dominum".into();
    let html = render_section_elements(&[e]);
    has(&html, r#"<div class="canticle">"#);
    has(&html, r#"<span class="psalm-incipit" lang="la">Magnificat anima mea Dominum</span>"#);
}

/// Without a recorded incipit there is no empty span or orphaned separator.
#[test]
fn psalm_label_without_incipit_is_unchanged() {
    let html = render_section_elements(&[psalm_67("")]);
    has(&html, r#"<h3 class="item-label">Psalm 67</h3>"#);
    lacks(&html, "label-sep");
    lacks(&html, "psalm-incipit");
}

#[test]
fn psalm_incipit_is_escaped() {
    let html = render_section_elements(&[psalm_67("Deus <b>misereatur</b> & nostri")]);
    lacks(&html, "<b>misereatur</b>");
    has(&html, "&lt;b&gt;misereatur&lt;/b&gt; &amp; nostri");
}

#[test]
fn martyrology_reading_renders_paragraphs_without_chapter_heading() {
    let got = render(&elem(ElementType::Reading, "First notice.\n\nSecond notice."));
    lacks(&got, "Chapter");
    assert_eq!(got.matches("<p").count(), 2, "{got}");
    has(&got, "First notice.");
    has(&got, "Second notice.");
    let response = render(&elem(ElementType::Response, "R. Thanks be to God."));
    has(&response, "℟.");
    has(&response, "Thanks be to God.");
}

// Checks against the live Athanasian Creed and ordinary prayer texts.

fn live() -> tools::fs::FsData {
    tools::fs::FsData::new("../../data")
}

#[test]
fn athanasian_creed_corpus() {
    let texts = office::texts::load_texts(&live()).unwrap();
    let mut creed = elem(ElementType::Canticle, texts.get("proper/trinity-sunday/athanasian-creed"));
    creed.label = "Athanasian Creed".into();
    let html = render_section_elements(&[creed, elem(ElementType::PsalmDoxology, texts.get("ordinary/shared/gloria-patri"))]);
    for n in 1..=42 {
        has(&html, &format!(r#"<span class="verse-num">{n}</span>"#));
    }
    for want in [
        "Whosoever will be saved",
        "The Holy Ghost is of the Father through the Son:",
        "which except a man believe faithfully, he cannot be saved.",
        "Glory be to the Father",
    ] {
        has(&html, want);
    }
}

#[test]
fn composed_preces_creed_renders_silent_middle_and_spoken_tail() {
    let src = live();
    let engine = office::Engine::load(&src).unwrap();
    // A bare Pentecost-season day with no celebration.
    let day = office::Day {
        cal: calendar::CalendarDay {
            date: calendar::Date::new(2026, 9, 7),
            season: calendar::Season::Pentecost,
            tempora: None,
            celebration: None,
            commemorations: Vec::new(),
            color: calendar::Color::White,
            notes: None,
            resolution_rule: String::new(),
            occurrence_decisions: Vec::new(),
            feria_commemoration: None,
            temporal_week_id: None,
            within_octave_of: None,
            penitential: Default::default(),
        },
        marian_antiphon: "salve-regina".to_string(),
        vespers: office::concurrence::VespersDesignation::unowned(),
        first_vespers: false,
        following_office_commemoration_id: String::new(),
    };
    let moveable = calendar::MoveableDates::compute(2026);
    for name in ["prime", "compline"] {
        let hour = engine.compose_hour(name, &day, &moveable, liturgy::PrayerForm::Private).unwrap();
        let creeds: Vec<&OfficeElement> = hour
            .sections
            .iter()
            .filter(|s| s.label == "Preces")
            .flat_map(|s| &s.elements)
            .filter(|e| e.source_ref == "ordinary/shared/apostles-creed")
            .collect();
        assert!(!creeds.is_empty(), "{name} missing Creed");
        for e in creeds {
            let html = render(e);
            for want in [
                r#"class="spoken-text">I believe</span>"#,
                r#"class="secret-text"> in God"#,
                r#"class="spoken-text">The Resurrection of the body"#,
                "And the Life everlasting. Amen.</span>",
            ] {
                has(&html, want);
            }
        }
    }
}

#[test]
fn typeset_cases() {
    for (input, want) in [
        ("even unto Aaron's beard", "even unto Aaron’s beard"),
        ("the angels' song", "the angels’ song"),
        ("Safe on th' eternal shore.", "Safe on th’ eternal shore."),
        ("Disperse th'oppressive shades", "Disperse th’oppressive shades"),
        ("'Mid the Twelve, his chosen band,", "’Mid the Twelve, his chosen band,"),
        ("'Tis the season", "’Tis the season"),
        ("his name of 'the Watchful'.", "his name of ‘the Watchful’."),
        ("Whose name 'God's might' doth signify:", "Whose name ‘God’s might’ doth signify:"),
        ("'The Lord is risen from the dead.'", "‘The Lord is risen from the dead.’"),
        (r#"He said, "Peace be unto you.""#, "He said, “Peace be unto you.”"),
        ("Luke 1:46-55", "Luke 1:46–55"),
        ("2 Cor. 1:3-4", "2 Cor. 1:3–4"),
        ("eye-lids to slumber", "eye-lids to slumber"),
        ("resting-place", "resting-place"),
        ("No punctuation to change.", "No punctuation to change."),
    ] {
        assert_eq!(typeset(input), want, "typeset({input:?})");
    }
}

#[test]
fn esc_text_typesets_before_escaping() {
    assert_eq!(esc_text("David's <b>"), "David’s &lt;b&gt;");
}

#[test]
fn prayer_speaker_labels_keep_responses_in_order() {
    let mut e = elem(ElementType::Prayer, "Have mercy upon thee.\nR. Amen.");
    e.voice = vec![
        VoiceSpan::new("Have mercy upon thee.\n", true, Some(VoiceRole::Response)),
        VoiceSpan::new("R. Amen.", true, Some(VoiceRole::Priest)),
    ];
    let html = render(&e);
    has(&html, r#"data-speaker="response"><p class="prayer-speaker">People</p>"#);
    has(&html, r#"data-speaker="priest"><p class="prayer-speaker">Priest</p>"#);
    assert!(html.find("People</p>") < html.find("Have mercy"), "speaker must precede their words: {html}");
    assert!(html.find("Priest</p>") < html.find("Amen."), "speaker must precede their words: {html}");
    // Invalid metadata must not truncate the prayer or invent labels.
    e.voice.truncate(1);
    let html = render(&e);
    has(&html, "Amen.");
    lacks(&html, "prayer-speaker");
}
