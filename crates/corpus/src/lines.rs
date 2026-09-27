//! The corpus line grammar every renderer understands (Go's
//! `internal/texts/lines.go`):
//!
//! ```text
//! !Isaiah 55:1          a scripture reference
//! [section: Part II]    a canticle section break
//! V. O God, make speed  a versicle, response, or blessing sigil
//! 1. Be merciful * unto us
//!                       a psalm verse: optional number, " * " mediant
//! Glory be to the Father, …
//! as it was in the beginning, …
//!                       the two-line Gloria Patri
//! /:Said kneeling.:/    a hymn-embedded rubric; delimiters are markup
//! [Ad Laudes]           a bracketed title artifact, dropped
//! ```

/// One item of a parsed psalm or canticle body.
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum PsalmItem {
    /// A (possibly numbered) verse, split at its " * " mediant; `second` is
    /// empty when the line has no mediant.
    Verse { number: String, first: String, second: String },
    /// A canticle section break.
    Section { heading: String },
    /// The two-line Gloria Patri, left unsplit; `second` is empty when the
    /// closing "as it was…" line is missing.
    Gloria { first: String, second: String },
}

/// A parsed psalm or canticle.
#[derive(Clone, Debug, Default, PartialEq, Eq)]
pub struct PsalmText {
    pub scripture_ref: String,
    pub items: Vec<PsalmItem>,
}

/// Parses a psalm or canticle body. The title block — everything above the
/// first blank line or numbered verse — is dropped except for a "!" scripture
/// reference. A "Glory be…" line opens a Gloria that the next "as it was…"
/// line closes; anything else intervening leaves it open where it stands.
pub fn parse_psalm(text: &str) -> PsalmText {
    let lines: Vec<&str> = text.split('\n').collect();
    let mut parsed = PsalmText::default();
    let mut content_start = lines.len();
    for (i, line) in lines.iter().enumerate() {
        let trimmed = line.trim();
        if split_leading_verse_number(trimmed).2 {
            content_start = i;
            break;
        }
        if trimmed.is_empty() {
            content_start = i + 1;
            break;
        }
        if let Some(r) = trimmed.strip_prefix('!') {
            parsed.scripture_ref = r.to_string();
        }
    }

    let mut gloria: Option<usize> = None;
    for line in &lines[content_start..] {
        let line = line.trim();
        if line.is_empty() {
            continue;
        }
        if let Some(heading) = section_heading(line) {
            gloria = None;
            parsed.items.push(PsalmItem::Section { heading: heading.to_string() });
            continue;
        }
        if let Some(i) = gloria.take()
            && line.to_lowercase().starts_with("as it was")
        {
            if let PsalmItem::Gloria { second, .. } = &mut parsed.items[i] {
                *second = line.to_string();
            }
            continue;
        }
        if line.starts_with("Glory be") {
            parsed.items.push(PsalmItem::Gloria { first: line.to_string(), second: String::new() });
            gloria = Some(parsed.items.len() - 1);
            continue;
        }
        let (number, body, _) = split_leading_verse_number(line);
        let (first, second) = body.split_once(" * ").unwrap_or((body, ""));
        parsed.items.push(PsalmItem::Verse { number: number.to_string(), first: first.to_string(), second: second.to_string() });
    }
    parsed
}

/// A "[section: Heading]" canticle break.
fn section_heading(line: &str) -> Option<&str> {
    let inner = line.strip_prefix("[section:")?.strip_suffix(']')?;
    Some(inner.trim())
}

/// Peels a leading verse number ("2. Text" or Benedicite-style "2 Text"),
/// at most three digits.
pub fn split_leading_verse_number(line: &str) -> (&str, &str, bool) {
    let b = line.as_bytes();
    let i = b.iter().take_while(|c| c.is_ascii_digit()).count();
    if i == 0 || i > 3 {
        return ("", line, false);
    }
    if i + 1 < b.len() && b[i] == b'.' && b[i + 1] == b' ' {
        return (&line[..i], line[i + 2..].trim(), true);
    }
    if i < b.len() && b[i] == b' ' {
        return (&line[..i], line[i + 1..].trim(), true);
    }
    ("", line, false)
}

/// The kind of one line of a liturgical block (collects, chapters, prayers,
/// blessings, preces, Marian antiphons).
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum BlockKind {
    /// An ordinary line; the corpus hard-wraps prose at sense lines.
    Prose,
    /// A blank separator line.
    Gap,
    /// A "!" reference line.
    ScriptureRef,
    Versicle,
    Response,
    Blessing,
    /// An "All: " line, spoken by everyone together.
    All,
}

/// One parsed line of a block. `offset` is the byte offset of `text` in the
/// parsed string, so spoken and silent spans can be mapped back onto it.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct BlockLine {
    pub kind: BlockKind,
    pub text: String,
    pub offset: usize,
}

const BLOCK_SIGILS: [(&str, BlockKind); 4] =
    [("V. ", BlockKind::Versicle), ("R. ", BlockKind::Response), ("Blessing. ", BlockKind::Blessing), ("All: ", BlockKind::All)];

/// Parses a liturgical block into classified lines. Bracketed title artifacts
/// ("[Ad Laudes]") are dropped.
pub fn parse_block(text: &str) -> Vec<BlockLine> {
    let mut parsed = Vec::new();
    let mut offset = 0;
    for raw in text.split('\n') {
        let line_start = offset;
        offset += raw.len() + 1;
        let line = raw.trim();
        if line.is_empty() {
            parsed.push(BlockLine { kind: BlockKind::Gap, text: String::new(), offset: line_start });
            continue;
        }
        let start = line_start + raw.find(line).unwrap_or(0);
        if line.starts_with('[') && line.ends_with(']') && line.len() >= 2 && line[1..line.len() - 1].contains(' ') {
            continue;
        }
        if let Some(r) = line.strip_prefix('!') {
            parsed.push(BlockLine { kind: BlockKind::ScriptureRef, text: r.to_string(), offset: start + 1 });
            continue;
        }
        if let Some((prefix, kind)) = BLOCK_SIGILS.iter().find(|(p, _)| line.starts_with(p)) {
            parsed.push(BlockLine { kind: *kind, text: line[prefix.len()..].to_string(), offset: start + prefix.len() });
            continue;
        }
        parsed.push(BlockLine { kind: BlockKind::Prose, text: line.to_string(), offset: start });
    }
    parsed
}

/// A parsed hymn: its Latin incipit, when the source carries one, and its
/// stanzas.
#[derive(Clone, Debug, Default, PartialEq, Eq)]
pub struct Hymn {
    pub title: String,
    pub stanzas: Vec<Vec<String>>,
}

/// Peels a hymn's Latin incipit: a single line standing alone above a blank
/// line. A multi-line first block, or a /:rubric:/, is left in the body.
pub fn split_hymn_title(text: &str) -> (&str, &str) {
    let Some((first, rest)) = text.split_once("\n\n") else { return ("", text) };
    let first = first.trim();
    if first.contains('\n') || hymn_rubric_text(first).is_some() {
        return ("", text);
    }
    (first, rest.trim())
}

/// Parses a hymn into its title and blank-line-separated stanzas.
pub fn parse_hymn(text: &str) -> Hymn {
    let (title, body) = split_hymn_title(text.trim());
    let mut parsed = Hymn { title: title.to_string(), stanzas: Vec::new() };
    let mut stanza: Vec<String> = Vec::new();
    for line in body.split('\n') {
        let trimmed = line.trim();
        if trimmed.is_empty() {
            if !stanza.is_empty() {
                parsed.stanzas.push(std::mem::take(&mut stanza));
            }
        } else {
            stanza.push(trimmed.to_string());
        }
    }
    if !stanza.is_empty() {
        parsed.stanzas.push(stanza);
    }
    parsed
}

/// The instruction inside a /:...:/ hymn rubric, delimiters stripped.
pub fn hymn_rubric_text(line: &str) -> Option<&str> {
    let s = line.trim();
    if s.len() < 4 {
        return None;
    }
    let inner = s.strip_prefix("/:")?.strip_suffix(":/")?.trim();
    (!inner.is_empty()).then_some(inner)
}

/// The instructions of a stanza whose every line is a /:...:/ rubric.
pub fn hymn_rubric_stanza(stanza: &[String]) -> Option<Vec<&str>> {
    if stanza.is_empty() {
        return None;
    }
    stanza.iter().map(|l| hymn_rubric_text(l)).collect()
}

#[cfg(test)]
mod tests {
    use super::*;

    fn verse(number: &str, first: &str, second: &str) -> PsalmItem {
        PsalmItem::Verse { number: number.into(), first: first.into(), second: second.into() }
    }

    #[test]
    fn leading_verse_numbers() {
        for (line, num, rest, ok) in [
            ("2. That thy way may be known", "2", "That thy way may be known", true),
            ("10. Make me a clean heart", "10", "Make me a clean heart", true),
            ("2 O ye Angels of the Lord", "2", "O ye Angels of the Lord", true),
            ("20 Blessed art thou, O Lord", "20", "Blessed art thou, O Lord", true),
            ("O ALL ye Works of the Lord", "", "O ALL ye Works of the Lord", false),
            ("2.No space after period", "", "2.No space after period", false),
        ] {
            assert_eq!(split_leading_verse_number(line), (num, rest, ok), "{line}");
        }
    }

    #[test]
    fn psalm_title_block_and_verses() {
        let psalm = parse_psalm(
            "Psalm 67\n!Deus misereatur\n\n1. God be merciful unto us * and bless us.\n2 O ye Angels of the Lord, bless ye the Lord.\n\
             An unnumbered line * with a mediant.\nGlory be to the Father, and to the Son,\n\
             as it was in the beginning, is now, and ever shall be.\n",
        );
        assert_eq!(psalm.scripture_ref, "Deus misereatur");
        assert_eq!(
            psalm.items,
            vec![
                verse("1", "God be merciful unto us", "and bless us."),
                verse("2", "O ye Angels of the Lord, bless ye the Lord.", ""),
                verse("", "An unnumbered line", "with a mediant."),
                PsalmItem::Gloria {
                    first: "Glory be to the Father, and to the Son,".into(),
                    second: "as it was in the beginning, is now, and ever shall be.".into()
                },
            ]
        );
    }

    #[test]
    fn canticle_sections() {
        let psalm = parse_psalm(
            "Song of the Three Children\n\n1. O all ye Works of the Lord.\n[section: The Second Part]\n2. O ye Angels of the Lord.\n",
        );
        assert_eq!(psalm.scripture_ref, "");
        assert_eq!(psalm.items.len(), 3);
        assert_eq!(psalm.items[1], PsalmItem::Section { heading: "The Second Part".into() });
    }

    #[test]
    fn dangling_gloria_stays_in_place() {
        let psalm = parse_psalm("Psalm 1\n\nGlory be to the Father,\n1. Blessed is the man.\n");
        assert_eq!(
            psalm.items,
            vec![
                PsalmItem::Gloria { first: "Glory be to the Father,".into(), second: String::new() },
                verse("1", "Blessed is the man.", "")
            ]
        );
    }

    #[test]
    fn block_classifies_lines() {
        let block = parse_block(
            "[Ad Laudes]\n!Romans 13\nProse one,\nprose two.\n\nV. O Lord, hear my prayer.\nR. And let my cry come unto thee.\n\
             All: Kyrie, eleison.\nBlessing. May the Lord bless us.\n",
        );
        use BlockKind::*;
        let kinds: Vec<BlockKind> = block.iter().map(|l| l.kind).collect();
        assert_eq!(kinds, vec![ScriptureRef, Prose, Prose, Gap, Versicle, Response, All, Blessing, Gap]);
        assert_eq!(block[0].text, "Romans 13");
        assert_eq!(block[4].text, "O Lord, hear my prayer.");
        assert_eq!(block[6].text, "Kyrie, eleison.");
        assert_eq!(block[7].text, "May the Lord bless us.");
    }

    #[test]
    fn block_offsets_point_at_their_text() {
        let source = "  Let us pray.\n\n\tV. O Lord, hear my prayer.\n!Psalm 102\n";
        for line in parse_block(source).iter().filter(|l| l.kind != BlockKind::Gap) {
            assert_eq!(&source[line.offset..line.offset + line.text.len()], line.text);
        }
    }

    #[test]
    fn hymn_title_only_when_it_stands_alone() {
        let titled = parse_hymn(
            "Te lucis ante terminum\n\nTo thee, before the close of day,\nCreator of the world, we pray.\n\nFrom all ill dreams defend our eyes.\n",
        );
        assert_eq!(titled.title, "Te lucis ante terminum");
        assert_eq!(titled.stanzas.len(), 2);
        assert_eq!(titled.stanzas[0], ["To thee, before the close of day,", "Creator of the world, we pray."]);
        let peeled =
            parse_hymn("To thee, before the close of day,\nCreator of the world, we pray.\n\nFrom all ill dreams defend our eyes.\n");
        assert_eq!(peeled.title, "");
        assert_eq!(peeled.stanzas.len(), 2);
        assert_eq!(peeled.stanzas[0][0], "To thee, before the close of day,");
        let single = parse_hymn("O Trinity of blessed light.\n");
        assert_eq!((single.title.as_str(), single.stanzas.len(), single.stanzas[0].len()), ("", 1, 1));
    }

    #[test]
    fn hymn_rubrics() {
        for (line, want) in [
            (
                "/:The first stanza of the following hymn is said kneeling.:/",
                Some("The first stanza of the following hymn is said kneeling."),
            ),
            ("  /:Stand and bow.:/  ", Some("Stand and bow.")),
            ("/:  Said kneeling.  :/", Some("Said kneeling.")),
            ("/::/", None),
            ("/:   :/", None),
            (":/not a rubric/:", None),
            ("The first stanza is said kneeling.", None),
            ("/:unterminated", None),
        ] {
            assert_eq!(hymn_rubric_text(line), want, "{line}");
        }
        let rubric = "/:The first stanza of the following hymn is said kneeling.:/";
        let peeled = parse_hymn(&format!("{rubric}\n\nStar of ocean fairest,\nMother, God who barest.\n"));
        assert_eq!(peeled.title, "");
        assert_eq!(peeled.stanzas[0][0], rubric);
        let titled = parse_hymn(&format!("Ave, maris stella\n\n{rubric}\n\nStar of ocean fairest,\nMother, God who barest.\n"));
        assert_eq!(titled.title, "Ave, maris stella");
        assert_eq!(titled.stanzas[0][0], rubric);
        assert_eq!(hymn_rubric_stanza(&peeled.stanzas[0]), Some(vec!["The first stanza of the following hymn is said kneeling."]));
        assert_eq!(hymn_rubric_stanza(&peeled.stanzas[1]), None);
    }
}
