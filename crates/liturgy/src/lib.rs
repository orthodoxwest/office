//! The document model: a composed hour of sections and elements, with speaker roles and rubric
//! spans. Every composer produces it and every renderer consumes it. Empty text fields represent
//! absence; the dump serializes them as null.

use calendar::{Color, Date, Decision, Season};

/// The kind of a liturgical element.
#[derive(Clone, Copy, Debug, PartialEq, Eq, Hash)]
pub enum ElementType {
    Rubric,
    Versicle,
    Prayer,
    Psalm,
    Canticle,
    Antiphon,
    Hymn,
    Chapter,
    Collect,
    Response,
    Blessing,
    Heading,
    Doxology,
    PsalmDoxology,
    Preces,
    /// Non-scriptural prose, such as the daily Martyrology.
    Reading,
    /// The seasonal Alleluia after the opening versicle; never an antiphon.
    OpeningAcclamation,
    /// The Responsory Breve after a chapter.
    ShortResponsory,
    /// A short exchange with named speakers, such as the Kyrie.
    Dialogue,
    /// The office's corporate form of the Lord's Prayer.
    CorporateLordPrayer,
}

impl ElementType {
    pub fn as_str(self) -> &'static str {
        match self {
            ElementType::Rubric => "rubric",
            ElementType::Versicle => "versicle",
            ElementType::Prayer => "prayer",
            ElementType::Psalm => "psalm",
            ElementType::Canticle => "canticle",
            ElementType::Antiphon => "antiphon",
            ElementType::Hymn => "hymn",
            ElementType::Chapter => "chapter",
            ElementType::Collect => "collect",
            ElementType::Response => "response",
            ElementType::Blessing => "blessing",
            ElementType::Heading => "heading",
            ElementType::Doxology => "doxology",
            ElementType::PsalmDoxology => "psalm-doxology",
            ElementType::Preces => "preces",
            ElementType::Reading => "reading",
            ElementType::OpeningAcclamation => "opening-acclamation",
            ElementType::ShortResponsory => "short-responsory",
            ElementType::Dialogue => "dialogue",
            ElementType::CorporateLordPrayer => "corporate-lord-prayer",
        }
    }

    pub fn is_psalmody(self) -> bool {
        matches!(self, ElementType::Psalm | ElementType::Canticle)
    }
}

/// A participant in a structured corporate prayer.
#[derive(Clone, Copy, Debug, PartialEq, Eq, Hash)]
pub enum VoiceRole {
    Officiant,
    Response,
    Priest,
    All,
}

impl VoiceRole {
    pub fn as_str(self) -> &'static str {
        match self {
            VoiceRole::Officiant => "officiant",
            VoiceRole::Response => "response",
            VoiceRole::Priest => "priest",
            VoiceRole::All => "all",
        }
    }

    /// The speaker's name for readers unfamiliar with liturgical sigils.
    pub fn label(self) -> &'static str {
        match self {
            VoiceRole::Officiant => "Leader",
            VoiceRole::Response => "People",
            VoiceRole::Priest => "Priest",
            VoiceRole::All => "All",
        }
    }
}

/// A stretch of prayer text with spoken or silent delivery and, for shared
/// prayers, a speaker.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct VoiceSpan {
    pub text: String,
    pub spoken: bool,
    pub role: Option<VoiceRole>,
}

impl VoiceSpan {
    pub fn new(text: impl Into<String>, spoken: bool, role: Option<VoiceRole>) -> VoiceSpan {
        VoiceSpan { text: text.into(), spoken, role }
    }
}

/// A run of a rubric; `prayed` marks words quoted for recitation.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct RubricSpan {
    pub text: String,
    pub prayed: bool,
}

/// A congregational posture, as the parish booklets cue it in red.
#[derive(Clone, Copy, Debug, PartialEq, Eq, Hash)]
pub enum Posture {
    Sit,
    Stand,
    Bow,
    StandUpright,
}

impl Posture {
    pub fn as_str(self) -> &'static str {
        match self {
            Posture::Sit => "sit",
            Posture::Stand => "stand",
            Posture::Bow => "bow",
            Posture::StandUpright => "stand-upright",
        }
    }

    /// The printed cue.
    pub fn cue(self) -> &'static str {
        match self {
            Posture::Sit => "Sit.",
            Posture::Stand => "Stand.",
            Posture::Bow => "Bow.",
            Posture::StandUpright => "Stand upright.",
        }
    }
}

/// Where a posture cue falls, counted in the verses `corpus::parse_psalm`
/// yields for a psalm or canticle, or in the lines of a psalm doxology.
#[derive(Clone, Copy, Debug, PartialEq, Eq, Hash)]
pub enum PostureAnchor {
    /// After the verse's " * " mediant, or at its end when it has none.
    AfterMediant(usize),
    /// Before the verse begins.
    BeforeVerse(usize),
}

#[derive(Clone, Copy, Debug, PartialEq, Eq, Hash)]
pub struct PostureCue {
    pub posture: Posture,
    pub at: PostureAnchor,
}

impl PostureCue {
    pub fn new(posture: Posture, at: PostureAnchor) -> PostureCue {
        PostureCue { posture, at }
    }
}

/// The cues at one anchor, in order.
pub fn posture_cues_at(cues: &[PostureCue], at: PostureAnchor) -> impl Iterator<Item = &'static str> + '_ {
    cues.iter().filter(move |c| c.at == at).map(|c| c.posture.cue())
}

/// One element of a composed hour, with presentation and source metadata.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct OfficeElement {
    pub leader_slot: String,
    pub kind: ElementType,
    pub text: String,
    pub label: String,
    pub incipit: String,
    pub rubric: String,
    pub voice: Vec<VoiceSpan>,
    pub rubric_spans: Vec<RubricSpan>,
    /// Congregational posture cues within a psalm, canticle or psalm doxology.
    pub postures: Vec<PostureCue>,
    pub slot_ref: String,
    pub source_ref: String,
    pub source_refs: Vec<String>,
    pub commemoration_owner_id: String,
    pub is_commemoration: bool,
    pub announce: bool,
}

impl OfficeElement {
    pub fn new(kind: ElementType, text: impl Into<String>) -> OfficeElement {
        OfficeElement {
            leader_slot: String::new(),
            kind,
            text: text.into(),
            label: String::new(),
            incipit: String::new(),
            rubric: String::new(),
            voice: Vec::new(),
            rubric_spans: Vec::new(),
            postures: Vec::new(),
            slot_ref: String::new(),
            source_ref: String::new(),
            source_refs: Vec::new(),
            commemoration_owner_id: String::new(),
            is_commemoration: false,
            announce: false,
        }
    }

    /// What a renderer prints: an announced antiphon's opening words, or the
    /// whole text.
    pub fn display_text(&self) -> String {
        if self.kind == ElementType::Antiphon && self.announce {
            return antiphon_announcement(&self.text);
        }
        self.text.clone()
    }

    /// The speaker turns, when every span is spoken by a named role and the
    /// spans reassemble the text exactly.
    pub fn speaker_turns(&self) -> Option<&[VoiceSpan]> {
        let mut text = String::new();
        for span in &self.voice {
            if !span.spoken || span.role.is_none() {
                return None;
            }
            text.push_str(&span.text);
        }
        (text == self.text).then_some(&self.voice[..])
    }
}

/// The words said when an antiphon is only announced before a psalm: those
/// before the first mediant mark (`*`, `†`, `‡`), closed with a period. Text
/// with no such mark is returned whole.
pub fn antiphon_announcement(text: &str) -> String {
    let text = text.trim();
    if text.is_empty() {
        return String::new();
    }
    if let Some(i) = text.find(['*', '†', '‡'])
        && i > 0
    {
        let incipit = text[..i].trim().trim_end_matches([',', ';', ':']);
        // Only terminal ASCII punctuation is recognized; a closing ’ counts as unpunctuated.
        if incipit.as_bytes().last().is_some_and(|b| b".?!".contains(b)) {
            return incipit.to_string();
        }
        return format!("{incipit}.");
    }
    text.to_string()
}

/// A group of elements; `collapsible` renders as a closed disclosure.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct OfficeSection {
    pub label: String,
    pub collapsible: bool,
    pub elements: Vec<OfficeElement>,
}

/// How the office is prayed: privately or led by clergy.
#[derive(Clone, Copy, Debug, PartialEq, Eq, Hash)]
pub enum PrayerForm {
    Private,
    Deacon,
    Priest,
}

impl PrayerForm {
    pub const ALL: [PrayerForm; 3] = [PrayerForm::Private, PrayerForm::Deacon, PrayerForm::Priest];

    pub fn as_str(self) -> &'static str {
        match self {
            PrayerForm::Private => "private",
            PrayerForm::Deacon => "deacon",
            PrayerForm::Priest => "priest",
        }
    }

    /// An empty value selects private prayer.
    pub fn parse(value: &str) -> Result<PrayerForm, String> {
        if value.is_empty() {
            return Ok(PrayerForm::Private);
        }
        PrayerForm::ALL
            .into_iter()
            .find(|f| f.as_str() == value)
            .ok_or_else(|| format!("invalid prayer form {}: choose private, deacon, or priest", data_format::quote(value)))
    }

    pub fn label(self) -> &'static str {
        match self {
            PrayerForm::Private => "Private",
            PrayerForm::Deacon => "Deacon",
            PrayerForm::Priest => "Priest",
        }
    }
}

/// A composed hour ready for rendering.
#[derive(Clone, Debug)]
pub struct OfficeHour {
    pub form: PrayerForm,
    pub date: Date,
    pub hour: String,
    pub title: String,
    pub season: Option<Season>,
    pub feast: String,
    pub color: Option<Color>,
    pub sections: Vec<OfficeSection>,
    pub decisions: Vec<Decision>,
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn announcements() {
        for (text, want) in [
            ("O Lord, * make haste.", "O Lord."),
            ("Who is this? * Alleluia.", "Who is this?"),
            ("Alleluia, alleluia, alleluia.", "Alleluia, alleluia, alleluia."),
            ("* leading mark", "* leading mark"),
            ("Thy servant’ † more", "Thy servant’."),
            ("", ""),
        ] {
            assert_eq!(antiphon_announcement(text), want, "{text}");
        }
    }

    #[test]
    fn prayer_forms() {
        assert_eq!(PrayerForm::parse(""), Ok(PrayerForm::Private));
        assert_eq!(PrayerForm::parse("priest"), Ok(PrayerForm::Priest));
        assert!(PrayerForm::parse("bishop").is_err());
    }
}
