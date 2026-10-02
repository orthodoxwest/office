//! Platform-neutral layout of a composed hour for the native apps: each section a list of blocks,
//! each block a paragraph of styled runs. It follows the HTML renderer's line grammar (pointed
//! verses, ℣/℟ lines, hymn stanzas, spoken and secret spans), so Kotlin and Swift only map kinds
//! and styles onto their own text systems and never parse liturgical text themselves.
//!
//! Every run of display text passes through the same typography as the web and the booklet.

use corpus::lines::{
    BlockKind as LineKind, BlockLine, PsalmItem, hymn_rubric_stanza, hymn_rubric_text, parse_block, parse_hymn, parse_psalm,
};
use corpus::typography::{soften_drop_cap_opening, typeset};
use liturgy::{
    ElementType, OfficeElement, OfficeHour, PostureAnchor, PostureCue, RubricSpan, VoiceRole, VoiceSpan, posture_cues_at, split_words,
};

/// What a block is, which decides its paragraph style.
#[derive(Clone, Copy, Debug, PartialEq, Eq, Hash)]
pub enum BlockKind {
    /// A section or item heading ("Hymn", "Chapter", a section's label).
    Heading,
    /// A commemoration heading: a `Kicker` run, then the feast's name.
    CommemorationHeading,
    /// A psalm or canticle label, its Latin incipit as a `Latin` run.
    ItemLabel,
    /// A hymn's or Marian antiphon's Latin title.
    LatinTitle,
    /// A chapter's citation.
    ChapterRef,
    /// A scripture reference before verses or inside a block.
    ScriptureRef,
    /// A heading inside a long canticle.
    CanticleSection,
    /// Rubric text, red except for `Prayed` runs.
    Rubric,
    /// A rubric that annotates the antiphon above it (a psalm's opening words
    /// that the antiphon has just said are not repeated): set as a rubric,
    /// close under the antiphon's words, past its marker.
    AntiphonNote,
    /// The same note under an announced antiphon: centred beneath its words,
    /// as the web's `.antiphon-announce + .unrepeated-note`.
    AnnouncementNote,
    /// An antiphon; the marker is "Ant.".
    Antiphon,
    /// An antiphon only announced before its psalm (its opening words): set
    /// centred over the psalm's label, as the web's `.antiphon-announce`.
    AnnouncedAntiphon,
    /// A pointed psalm or canticle verse; the marker is its printed number, if any.
    Verse,
    /// The Gloria Patri after a psalm: two pointed lines separated by a `Break`
    /// run, set on the verses' edge, each line's wrap hanging beneath its start.
    GloriaPatri,
    /// Prose, or preserved lines separated by `Break` runs.
    Paragraph,
    /// A sung line of a Marian antiphon.
    ChantLine,
    /// A versicle; the marker is ℣. or "Blessing.".
    Versicle,
    /// A response; the marker is ℟., or empty for a short responsory's opening.
    Response,
    /// A line said by all; the marker is "All:".
    All,
    /// The speaker of the turn that follows ("Leader", "People").
    Speaker,
    /// A hymn stanza, its lines separated by `Break` runs.
    Stanza,
    /// Vertical space between groups of lines; it has no runs.
    Gap,
}

/// How a run of text is set within its block.
#[derive(Clone, Copy, Debug, PartialEq, Eq, Hash)]
pub enum RunStyle {
    Plain,
    /// The pointing asterisk between half-verses, bound to the half it ends.
    Mediant,
    /// The ✠ sign of the cross.
    Cross,
    /// Words quoted in a rubric for recitation, set as prayer, not rubric.
    Prayed,
    /// Words not said aloud: a secret prayer's silent part, or a psalm's
    /// opening words that its antiphon has just said. Set in the `unsaid` tone.
    Secret,
    /// A Latin title or incipit.
    Latin,
    /// The small lead-in of a commemoration heading.
    Kicker,
    /// A congregational posture cue in the psalmody ("Sit.", "Bow."), set as a small red rubric.
    Posture,
    /// A preserved line break; its text is "\n".
    Break,
}

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Run {
    pub text: String,
    pub style: RunStyle,
}

impl Run {
    fn new(text: impl Into<String>, style: RunStyle) -> Run {
        Run { text: text.into(), style }
    }
}

/// One paragraph of an hour.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Block {
    pub kind: BlockKind,
    /// A sigil, verse number, or "Ant." set in the gutter before the text.
    pub marker: String,
    /// An opening that takes an initial, as the web marks them: a psalm's or
    /// canticle's first verse, a hymn's first stanza, a Marian antiphon's sung
    /// opening, and the first line of a chapter, a spoken collect, the corporate
    /// Lord's Prayer, or a short responsory (whose initial is raised, not dropped).
    pub drop_cap: bool,
    /// The first block of a liturgical element: layout keeps more air between
    /// elements than between the lines of one.
    pub starts_element: bool,
    pub runs: Vec<Run>,
}

impl Block {
    fn new(kind: BlockKind, runs: Vec<Run>) -> Block {
        Block { kind, marker: String::new(), drop_cap: false, starts_element: false, runs }
    }

    fn marked(kind: BlockKind, marker: &str, runs: Vec<Run>) -> Block {
        Block { kind, marker: marker.to_string(), drop_cap: false, starts_element: false, runs }
    }

    /// The block's words as read: marker, then runs.
    pub fn plain_text(&self) -> String {
        let mut s = self.marker.clone();
        if !s.is_empty() && !self.runs.is_empty() {
            s.push(' ');
        }
        s.extend(self.runs.iter().map(|r| r.text.as_str()));
        s
    }
}

/// A section of an hour; a collapsible section opens closed under its label.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct SectionBlocks {
    pub label: String,
    pub collapsible: bool,
    pub blocks: Vec<Block>,
}

/// The hour's sections as blocks. An uncollapsible labelled section opens with
/// its heading, and Compline's unlabelled first section is the Opening, as on
/// the web.
pub fn hour_sections(hour: &OfficeHour) -> Vec<SectionBlocks> {
    let compline = hour.hour.eq_ignore_ascii_case("compline");
    hour.sections
        .iter()
        .enumerate()
        .map(|(i, section)| {
            let mut blocks = Vec::new();
            if !section.collapsible {
                if !section.label.is_empty() {
                    blocks.push(section_heading(&section.label));
                } else if compline && i == 0 {
                    blocks.push(section_heading("Opening"));
                }
            }
            blocks.extend(element_blocks(&section.elements));
            SectionBlocks { label: section.label.clone(), collapsible: section.collapsible, blocks }
        })
        .collect()
}

/// A section's elements, a psalm or canticle absorbing the doxology after it.
pub fn element_blocks(elems: &[OfficeElement]) -> Vec<Block> {
    let mut out = Vec::new();
    let mut i = 0;
    while i < elems.len() {
        let elem = &elems[i];
        let mut doxology = None;
        if elem.kind.is_psalmody() && elems.get(i + 1).is_some_and(|next| next.kind == ElementType::PsalmDoxology) {
            doxology = Some(&elems[i + 1]);
            i += 1;
        }
        let start = out.len();
        push_element(&mut out, elem, doxology);
        if let Some(first) = out.get_mut(start) {
            first.starts_element = true;
        }
        i += 1;
    }
    out
}

/// A section heading, separating a commemoration's lead-in from its feast.
pub fn section_heading(label: &str) -> Block {
    if let Some(name) = label.strip_prefix("Commemoration of ").filter(|n| !n.is_empty()) {
        return Block::new(
            BlockKind::CommemorationHeading,
            vec![Run::new("Commemoration of", RunStyle::Kicker), Run::new(" ", RunStyle::Plain), Run::new(typeset(name), RunStyle::Plain)],
        );
    }
    Block::new(BlockKind::Heading, text_runs(label))
}

fn push_element(out: &mut Vec<Block>, elem: &OfficeElement, doxology: Option<&OfficeElement>) {
    match elem.kind {
        ElementType::Heading => out.push(section_heading(&elem.text)),
        ElementType::Rubric => out.push(rubric(elem)),
        ElementType::OpeningAcclamation => out.push(Block::new(BlockKind::Paragraph, chant_runs(&elem.text))),
        ElementType::Antiphon => {
            if elem.label.is_empty() {
                let kind = if elem.announce { BlockKind::AnnouncedAntiphon } else { BlockKind::Antiphon };
                out.push(Block::marked(kind, "Ant.", chant_runs(&elem.display_text())));
            } else {
                out.push(Block::new(BlockKind::LatinTitle, text_runs_styled(&elem.label, RunStyle::Latin)));
                marian_antiphon(out, &elem.text);
            }
        }
        ElementType::Psalm | ElementType::Canticle => {
            let unrepeated = elem.unrepeated_rubric();
            if !unrepeated.is_empty() {
                let announced = out.last().is_some_and(|b| b.kind == BlockKind::AnnouncedAntiphon);
                let kind = if announced { BlockKind::AnnouncementNote } else { BlockKind::AntiphonNote };
                out.push(Block { kind, ..rubric_block(&unrepeated) });
            }
            if !elem.label.is_empty() {
                let mut runs = text_runs(&elem.label);
                if !elem.incipit.is_empty() {
                    runs.push(Run::new("\u{a0}· ", RunStyle::Plain));
                    runs.push(Run::new(typeset(&elem.incipit), RunStyle::Latin));
                }
                out.push(Block::new(BlockKind::ItemLabel, runs));
            }
            psalm_verses(out, &elem.text, &elem.postures, elem.unrepeated.as_ref().map_or(0, |u| u.words));
            if let Some(doxology) = doxology.filter(|d| !d.text.is_empty()) {
                out.push(gloria_patri(&doxology.text, &doxology.postures));
            }
        }
        ElementType::Hymn => {
            out.push(section_heading("Hymn"));
            if !elem.label.is_empty() {
                out.push(Block::new(BlockKind::LatinTitle, text_runs_styled(&elem.label, RunStyle::Latin)));
            }
            hymn_stanzas(out, &elem.text);
        }
        ElementType::Versicle | ElementType::Response | ElementType::Blessing | ElementType::Doxology | ElementType::Dialogue => {
            liturgical_block(out, &elem.text, Mode::PreserveLines, false);
        }
        ElementType::ShortResponsory => {
            let start = out.len();
            liturgical_block(out, &elem.text, Mode::PreserveLines, true);
            mark_opening(out, start, |b| b.kind == BlockKind::Response && b.marker.is_empty());
        }
        ElementType::CorporateLordPrayer => {
            let start = out.len();
            corporate_lord_prayer(out, elem);
            mark_opening(out, start, |b| b.kind == BlockKind::Paragraph);
        }
        ElementType::Collect => {
            if elem.voice.is_empty() {
                let start = out.len();
                liturgical_block(out, &elem.text, Mode::Flow, false);
                mark_opening(out, start, |b| b.kind == BlockKind::Paragraph);
            } else {
                voice_block(out, &elem.voice, Mode::Flow);
            }
        }
        ElementType::Prayer | ElementType::Reading => {
            if let Some(turns) = elem.speaker_turns().filter(|t| !t.is_empty()) {
                for turn in turns {
                    let role = turn.role.expect("speaker turns carry roles");
                    out.push(Block::new(BlockKind::Speaker, vec![Run::new(role.label(), RunStyle::Plain)]));
                    liturgical_block(out, &turn.text, Mode::Flow, false);
                }
            } else if !elem.voice.is_empty() && elem.voice.iter().all(|s| s.role.is_none()) {
                voice_block(out, &elem.voice, Mode::Flow);
            } else {
                liturgical_block(out, &elem.text, Mode::Flow, false);
            }
        }
        ElementType::Chapter => {
            out.push(section_heading("Chapter"));
            if !elem.label.is_empty() {
                out.push(Block::new(BlockKind::ChapterRef, text_runs(&elem.label)));
            }
            let start = out.len();
            liturgical_block(out, &elem.text, Mode::Flow, false);
            mark_opening(out, start, |b| matches!(b.kind, BlockKind::Paragraph | BlockKind::Versicle));
        }
        ElementType::Preces => liturgical_block(out, &elem.text, Mode::PreserveLines, false),
        ElementType::PsalmDoxology => out.push(gloria_patri(&elem.text, &elem.postures)),
    }
}

/// Marks the first block from `start` as an opening when it is the kind that
/// takes one; a reference or gap first means the opening is not a text line.
fn mark_opening(out: &mut [Block], start: usize, takes_initial: impl Fn(&Block) -> bool) {
    if let Some(first) = out.get_mut(start).filter(|b| takes_initial(b)) {
        first.drop_cap = true;
    }
}

/// Typeset display text as one plain run.
fn text_runs(s: &str) -> Vec<Run> {
    text_runs_styled(s, RunStyle::Plain)
}

fn text_runs_styled(s: &str, style: RunStyle) -> Vec<Run> {
    vec![Run::new(typeset(s), style)]
}

/// Typeset text in `style`, each ✠ its own run.
fn cross_runs(s: &str, style: RunStyle) -> Vec<Run> {
    let text = typeset(s);
    let mut runs = Vec::new();
    for (i, part) in text.split('✠').enumerate() {
        if i > 0 {
            runs.push(Run::new("✠", RunStyle::Cross));
        }
        if !part.is_empty() {
            runs.push(Run::new(part, style));
        }
    }
    runs
}

/// The pointing asterisk, a no-break space binding it to the half it ends.
fn push_mediant(runs: &mut Vec<Run>, trailing_space: bool) {
    runs.push(Run::new("\u{a0}*", RunStyle::Mediant));
    if trailing_space {
        runs.push(Run::new(" ", RunStyle::Plain));
    }
}

/// A line of liturgical text with its " * " mediant marked.
fn chant_runs(line: &str) -> Vec<Run> {
    if let Some((before, after)) = line.split_once(" * ") {
        let mut runs = cross_runs(before, RunStyle::Plain);
        push_mediant(&mut runs, true);
        runs.extend(cross_runs(after, RunStyle::Plain));
        return runs;
    }
    if let Some(before) = line.strip_suffix(" *") {
        let mut runs = cross_runs(before, RunStyle::Plain);
        push_mediant(&mut runs, false);
        return runs;
    }
    cross_runs(line, RunStyle::Plain)
}

fn rubric(elem: &OfficeElement) -> Block {
    if elem.rubric_spans.is_empty() {
        return Block::new(BlockKind::Rubric, text_runs(&elem.text));
    }
    rubric_block(&elem.rubric_spans)
}

fn rubric_block(spans: &[RubricSpan]) -> Block {
    let mut runs = Vec::new();
    for span in spans {
        if span.prayed {
            runs.extend(cross_runs(&span.text, RunStyle::Prayed));
        } else {
            runs.extend(text_runs(&span.text));
        }
    }
    Block::new(BlockKind::Rubric, runs)
}

/// Posture cues at one anchor, each a run followed by a space.
fn push_postures(runs: &mut Vec<Run>, cues: &[PostureCue], at: PostureAnchor) {
    for cue in posture_cues_at(cues, at) {
        runs.push(Run::new(cue, RunStyle::Posture));
        runs.push(Run::new(" ", RunStyle::Plain));
    }
}

/// Runs of a half-verse, its first `words` words muted as not said; returns
/// the words still to mute.
fn push_unrepeated(runs: &mut Vec<Run>, s: &str, words: usize) -> usize {
    let (unsaid, rest, taken) = split_words(s, words);
    if !unsaid.is_empty() {
        runs.extend(cross_runs(unsaid, RunStyle::Secret));
    }
    if !rest.is_empty() {
        runs.extend(cross_runs(rest, RunStyle::Plain));
    }
    words - taken
}

/// A psalm or canticle: any scripture reference, then its verses, with posture cues at their anchors.
/// The first `unrepeated` words, which the antiphon has just said, are muted.
fn psalm_verses(out: &mut Vec<Block>, text: &str, postures: &[PostureCue], mut unrepeated: usize) {
    let psalm = parse_psalm(text);
    if !psalm.scripture_ref.is_empty() {
        out.push(Block::new(BlockKind::ScriptureRef, text_runs(&psalm.scripture_ref)));
    }
    let mut drop_cap_next = true;
    let mut verse = 0;
    for item in &psalm.items {
        match item {
            PsalmItem::Section { heading } => {
                out.push(Block::new(BlockKind::CanticleSection, text_runs(heading)));
                drop_cap_next = true;
            }
            PsalmItem::Gloria { first, second } => {
                let mut runs = text_runs(first);
                if !second.is_empty() {
                    push_mediant(&mut runs, true);
                    runs.extend(text_runs(second));
                }
                out.push(Block::new(BlockKind::Verse, runs));
                drop_cap_next = false;
            }
            PsalmItem::Verse { number, first, second } => {
                let drop_cap = drop_cap_next;
                drop_cap_next = false;
                let first = if drop_cap { soften_drop_cap_opening(first) } else { first.clone() };
                let mut runs = Vec::new();
                push_postures(&mut runs, postures, PostureAnchor::BeforeVerse(verse));
                unrepeated = push_unrepeated(&mut runs, &first, unrepeated);
                let mut after_mediant = Vec::new();
                push_postures(&mut after_mediant, postures, PostureAnchor::AfterMediant(verse));
                if !second.is_empty() {
                    push_mediant(&mut runs, true);
                    runs.extend(after_mediant);
                    unrepeated = push_unrepeated(&mut runs, second, unrepeated);
                } else if !after_mediant.is_empty() {
                    // Without a mediant the cue ends the verse.
                    after_mediant.pop();
                    runs.push(Run::new(" ", RunStyle::Plain));
                    runs.extend(after_mediant);
                }
                verse += 1;
                let mut block = Block::marked(BlockKind::Verse, &typeset(number), runs);
                block.drop_cap = drop_cap;
                out.push(block);
            }
        }
    }
}

#[derive(Clone, Copy, PartialEq, Eq)]
enum Mode {
    PreserveLines,
    Flow,
    PreserveFirstBlock,
}

/// Collects prose lines between sigil lines, and the gaps between groups.
struct Prose<T> {
    lines: Vec<T>,
    blocks: usize,
    pending_gap: bool,
}

impl<T> Prose<T> {
    fn new() -> Prose<T> {
        Prose { lines: Vec::new(), blocks: 0, pending_gap: false }
    }

    fn emit_gap(&mut self, out: &mut Vec<Block>) {
        if self.pending_gap {
            out.push(Block::new(BlockKind::Gap, Vec::new()));
            self.pending_gap = false;
        }
    }
}

/// Joins lines' runs, with a `Break` between preserved lines or a space between flowing ones.
fn join_lines(lines: impl IntoIterator<Item = Vec<Run>>, preserve: bool) -> Vec<Run> {
    let mut runs = Vec::new();
    for (i, line) in lines.into_iter().enumerate() {
        if i > 0 {
            runs.push(if preserve { Run::new("\n", RunStyle::Break) } else { Run::new(" ", RunStyle::Plain) });
        }
        runs.extend(line);
    }
    runs
}

/// The block of a sigil line: ℣., ℟., "Blessing.", or "All:".
fn sigil_line(kind: LineKind) -> Option<(BlockKind, &'static str)> {
    match kind {
        LineKind::Versicle => Some((BlockKind::Versicle, "℣.")),
        LineKind::Response => Some((BlockKind::Response, "℟.")),
        LineKind::Blessing => Some((BlockKind::Versicle, "Blessing.")),
        LineKind::All => Some((BlockKind::All, "All:")),
        LineKind::Prose | LineKind::Gap | LineKind::ScriptureRef => None,
    }
}

fn flush_prose(out: &mut Vec<Block>, prose: &mut Prose<String>, mode: Mode) {
    if prose.lines.is_empty() {
        return;
    }
    prose.emit_gap(out);
    let lines = std::mem::take(&mut prose.lines);
    if mode == Mode::PreserveFirstBlock && prose.blocks == 0 {
        // The sung Marian antiphon's opening pair shares one block so a
        // two-line drop cap can sit beside both lines.
        let open_n = lines.len().min(2);
        let mut opening = Block::new(BlockKind::ChantLine, join_lines(lines[..open_n].iter().map(|l| chant_runs(l)), true));
        opening.drop_cap = true;
        out.push(opening);
        for l in &lines[open_n..] {
            out.push(Block::new(BlockKind::ChantLine, chant_runs(l)));
        }
    } else {
        let preserve = mode == Mode::PreserveLines && lines.len() > 1;
        out.push(Block::new(BlockKind::Paragraph, join_lines(lines.iter().map(|l| chant_runs(l)), preserve)));
    }
    prose.blocks += 1;
}

/// Versicles, responses, and prose; a short responsory opens with its response unmarked.
fn liturgical_block(out: &mut Vec<Block>, text: &str, mode: Mode, short_responsory: bool) {
    let mut prose = Prose::new();
    // A responsory opens with its response (℟. br.). One that opens with a
    // versicle, such as Compline's Keep us / Hide us, is an ordinary pair.
    let mut opening = short_responsory;
    for line in parse_block(text) {
        let at_opening = opening && !matches!(line.kind, LineKind::Gap | LineKind::ScriptureRef);
        if at_opening {
            opening = false;
        }
        match line.kind {
            LineKind::Gap => {
                flush_prose(out, &mut prose, mode);
                prose.pending_gap = true;
            }
            LineKind::ScriptureRef => {
                flush_prose(out, &mut prose, mode);
                prose.emit_gap(out);
                out.push(Block::new(BlockKind::ScriptureRef, text_runs(&line.text)));
            }
            LineKind::Prose => prose.lines.push(line.text),
            LineKind::Response if at_opening => {
                flush_prose(out, &mut prose, mode);
                prose.emit_gap(out);
                out.push(Block::new(BlockKind::Response, chant_runs(&line.text)));
            }
            LineKind::Versicle | LineKind::Response | LineKind::Blessing | LineKind::All => {
                let (kind, marker) = sigil_line(line.kind).expect("sigil line");
                flush_prose(out, &mut prose, mode);
                prose.emit_gap(out);
                out.push(Block::marked(kind, marker, chant_runs(&line.text)));
            }
        }
    }
    flush_prose(out, &mut prose, mode);
}

/// `text` as runs of spoken and secret words, by byte offset into the prayer.
fn voiced_runs(text: &str, offset: usize, spoken_at: &[bool]) -> Vec<Run> {
    let bytes = text.as_bytes();
    let mut runs = Vec::new();
    let mut i = 0;
    while i < bytes.len() {
        let spoken = spoken_at[offset + i];
        let mut j = i + 1;
        while j < bytes.len() && spoken_at[offset + j] == spoken {
            j += 1;
        }
        // Span boundaries fall between whole characters of the composed text.
        runs.extend(cross_runs(&text[i..j], if spoken { RunStyle::Plain } else { RunStyle::Secret }));
        i = j;
    }
    runs
}

/// A prayer partitioned into spoken and secret spans.
fn voice_block(out: &mut Vec<Block>, spans: &[VoiceSpan], mode: Mode) {
    let text: String = spans.iter().map(|s| s.text.as_str()).collect();
    let mut spoken_at = Vec::with_capacity(text.len());
    for span in spans {
        spoken_at.extend(std::iter::repeat_n(span.spoken, span.text.len()));
    }
    let mut prose: Prose<BlockLine> = Prose::new();
    let flush = |out: &mut Vec<Block>, prose: &mut Prose<BlockLine>| {
        if prose.lines.is_empty() {
            return;
        }
        prose.emit_gap(out);
        let lines = std::mem::take(&mut prose.lines);
        let preserve = lines.len() > 1 && (mode == Mode::PreserveLines || (mode == Mode::PreserveFirstBlock && prose.blocks == 0));
        out.push(Block::new(BlockKind::Paragraph, join_lines(lines.iter().map(|l| voiced_runs(&l.text, l.offset, &spoken_at)), preserve)));
        prose.blocks += 1;
    };
    for line in parse_block(&text) {
        match line.kind {
            LineKind::Gap => {
                flush(out, &mut prose);
                prose.pending_gap = true;
            }
            LineKind::ScriptureRef => {
                flush(out, &mut prose);
                prose.emit_gap(out);
                out.push(Block::new(BlockKind::ScriptureRef, text_runs(&line.text)));
            }
            // An `All:` line is prose in a voiced prayer.
            LineKind::All | LineKind::Prose => prose.lines.push(line),
            LineKind::Versicle | LineKind::Response | LineKind::Blessing => {
                let (kind, marker) = sigil_line(line.kind).expect("sigil line");
                flush(out, &mut prose);
                prose.emit_gap(out);
                out.push(Block::marked(kind, marker, voiced_runs(&line.text, line.offset, &spoken_at)));
            }
        }
    }
    flush(out, &mut prose);
}

/// A Marian antiphon: its sung lines preserved, its collect flowing.
fn marian_antiphon(out: &mut Vec<Block>, text: &str) {
    const INVITATION: &str = "\n\nLet us pray.\n\n";
    match text.split_once(INVITATION) {
        Some((chant, collect)) if !collect.trim().is_empty() => {
            liturgical_block(out, chant, Mode::PreserveFirstBlock, false);
            out.push(Block::new(BlockKind::Gap, Vec::new()));
            out.push(Block::new(BlockKind::Paragraph, vec![Run::new("Let us pray.", RunStyle::Plain)]));
            out.push(Block::new(BlockKind::Gap, Vec::new()));
            liturgical_block(out, collect, Mode::Flow, false);
        }
        _ => liturgical_block(out, text, Mode::PreserveFirstBlock, false),
    }
}

/// The corporate Lord's Prayer, split between officiant and people; any
/// incomplete partition falls back to an ordinary prayer.
fn corporate_lord_prayer(out: &mut Vec<Block>, elem: &OfficeElement) {
    let (mut officiant, mut response) = (String::new(), String::new());
    for span in &elem.voice {
        match span.role {
            Some(VoiceRole::Officiant) => officiant.push_str(&span.text),
            Some(VoiceRole::Response) => response.push_str(&span.text),
            Some(VoiceRole::Priest | VoiceRole::All) | None => {}
        }
    }
    if officiant.is_empty() || response.is_empty() {
        liturgical_block(out, &elem.text, Mode::Flow, false);
        return;
    }
    let flow = |s: &str| s.split('\n').map(str::trim).filter(|l| !l.is_empty()).collect::<Vec<_>>().join(" ");
    out.push(Block::new(BlockKind::Paragraph, chant_runs(&flow(&officiant))));
    out.push(Block::marked(BlockKind::Response, "℟.", chant_runs(&flow(&response))));
}

/// A hymn: one block per stanza, a standalone Amen folded into the stanza before it.
fn hymn_stanzas(out: &mut Vec<Block>, text: &str) {
    let hymn = parse_hymn(text);
    if !hymn.title.is_empty() {
        match hymn_rubric_text(&hymn.title) {
            Some(r) => out.push(Block::new(BlockKind::Rubric, text_runs(r))),
            None => out.push(Block::new(BlockKind::LatinTitle, text_runs_styled(&hymn.title, RunStyle::Latin))),
        }
    }
    let mut opening = true;
    for (i, stanza) in hymn.stanzas.iter().enumerate() {
        if is_hymn_amen(stanza) && i > 0 {
            continue;
        }
        if let Some(rubrics) = hymn_rubric_stanza(stanza) {
            out.extend(rubrics.into_iter().map(|r| Block::new(BlockKind::Rubric, text_runs(r))));
            continue;
        }
        let mut runs = join_lines(stanza.iter().map(|l| cross_runs(l, RunStyle::Plain)), true);
        if let Some(amen) = hymn.stanzas.get(i + 1).filter(|s| is_hymn_amen(s)) {
            runs.push(Run::new(" ", RunStyle::Plain));
            runs.extend(cross_runs(&amen[0], RunStyle::Plain));
        }
        let mut block = Block::new(BlockKind::Stanza, runs);
        block.drop_cap = opening;
        opening = false;
        out.push(block);
    }
}

fn is_hymn_amen(stanza: &[String]) -> bool {
    stanza.len() == 1 && stanza[0].trim().trim_end_matches(['.', '!', ' ']).eq_ignore_ascii_case("amen")
}

/// The Gloria Patri as two pointed lines, each after its posture cues.
fn gloria_patri(text: &str, postures: &[PostureCue]) -> Block {
    let lines: Vec<&str> = text.trim().split('\n').collect();
    let (first, second) = if lines.len() >= 2 { (lines[0].trim(), lines[1].trim()) } else { (text.trim(), "") };
    let cued = |n: usize, line: &str| {
        let mut runs = Vec::new();
        push_postures(&mut runs, postures, PostureAnchor::BeforeVerse(n));
        runs.extend(chant_runs(line));
        runs
    };
    let runs = if second.is_empty() { cued(0, first) } else { join_lines([cued(0, first), cued(1, second)], true) };
    Block::new(BlockKind::GloriaPatri, runs)
}

#[cfg(test)]
mod tests;
