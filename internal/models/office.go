package models

import (
	"strings"
	"time"
)

// ElementType identifies the kind of liturgical element.
type ElementType string

const (
	Rubric        ElementType = "rubric"
	Versicle      ElementType = "versicle"
	Prayer        ElementType = "prayer"
	Psalm         ElementType = "psalm"
	Canticle      ElementType = "canticle"
	Antiphon      ElementType = "antiphon"
	Hymn          ElementType = "hymn"
	Chapter       ElementType = "chapter"
	Collect       ElementType = "collect"
	Response      ElementType = "response"
	Blessing      ElementType = "blessing"
	Heading       ElementType = "heading"
	Doxology      ElementType = "doxology"
	PsalmDoxology ElementType = "psalm-doxology"
	Preces        ElementType = "preces"
	// OpeningAcclamation is the seasonal Alleluia (or its Septuagesima
	// replacement) which follows the opening versicle. It is not an antiphon
	// and therefore never takes the rubrical "Ant." sigil or psalm-frame
	// presentation rules.
	OpeningAcclamation ElementType = "opening-acclamation"
	// ShortResponsory is the Responsory Breve after a chapter. Its opening
	// response is conventionally marked by a dropped initial, while later
	// responses retain their ℟. sigils.
	ShortResponsory ElementType = "short-responsory"
	// Dialogue is a short exchange with named speakers, such as the Kyrie.
	Dialogue ElementType = "dialogue"
	// CorporateLordPrayer identifies the office's corporate form of the Lord's
	// Prayer, whose officiant and response portions must not leak into the
	// private/secret use of the shared prayer text.
	CorporateLordPrayer ElementType = "corporate-lord-prayer"
)

// VoiceSpan is a contiguous stretch of prayer text with a spoken/silent delivery.
// Secret prayers are composed of an aloud incipit, a silent body, and optionally
// an aloud tail (e.g. the pre-collect Our Father). Nil/empty Voice means the
// whole Text is spoken normally.
type VoiceSpan struct {
	Text   string
	Spoken bool
	Role   VoiceRole
}

// VoiceRole identifies a participant when a spoken span belongs to a
// structured corporate prayer. The empty role preserves ordinary/private
// spoken and secret prayer behaviour.
type VoiceRole string

const (
	VoiceOfficiant VoiceRole = "officiant"
	VoiceResponse  VoiceRole = "response"
)

// RubricSpan is one semantically distinct run in a rubric. Prayed is limited
// to words which the rubric quotes for recitation; the surrounding instruction
// remains rubricated. It deliberately does not try to parse arbitrary prose.
type RubricSpan struct {
	Text   string
	Prayed bool
}

// OfficeElement represents a single element in an office hour.
// Incipit is the Latin incipit a printed diurnal sets beside a psalm or
// canticle label ("Psalm 67 · Deus misereatur"); it is empty for every other
// element type. Unlike SourceRef it is displayed text, so it takes part in
// review hashes.
//
// SlotRef is set for elements that went through proper resolution. SourceRef
// is the primary corpus key that supplied the rendered text. SourceRefs is the
// complete dependency set when composition used more than one entry (for
// example, a proper hymn with a seasonal doxology). These fields are excluded
// from review hashes and never rendered.
//
// Voice, when non-empty, is a presentation partition of Text into spoken and
// silent spans. The concatenation of span texts must equal Text. Plain-text
// and golden output use Text only; HTML/TeX may style Voice.
//
// Announce, on an Antiphon, means the element is the opening half of a
// psalm/canticle frame and the office is not Double at this hour: print only
// the announcement form (DisplayText), not the full Text. Text itself always
// holds the complete antiphon so review hashes, provenance, and the matching
// after-antiphon stay whole. Announce is presentation-only and is not part of
// the review content hash.
type OfficeElement struct {
	Type        ElementType
	Text        string
	Label       string
	Incipit     string
	Rubric      string
	Voice       []VoiceSpan
	RubricSpans []RubricSpan
	SlotRef     string
	SourceRef   string
	SourceRefs  []string
	Announce    bool
}

// DisplayText is the string a renderer should print for this element. For an
// announced antiphon it is the words before the mediant, closed as a sentence;
// otherwise it is Text.
func (e OfficeElement) DisplayText() string {
	if e.Type == Antiphon && e.Announce {
		return AntiphonAnnouncement(e.Text)
	}
	return e.Text
}

// AntiphonAnnouncement returns the form said when an antiphon is merely
// announced before a psalm: the words before the first mediant asterisk (*)
// or dagger (†), closed with a period. When the corpus text has no such mark,
// the full text is returned — there is no safe partial cut without a pointing
// cue.
func AntiphonAnnouncement(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	if i := strings.IndexAny(text, "*†‡"); i > 0 {
		incipit := strings.TrimRight(strings.TrimSpace(text[:i]), ",;:")
		if incipit != "" && strings.ContainsAny(incipit[len(incipit)-1:], ".?!") {
			return incipit
		}
		return incipit + "."
	}
	// No mark, or no incipit before a leading mark, leaves the full text intact.
	return text
}

// CompositionDecision records one machine-readable choice made while an hour
// was composed. Rule is a stable identifier, Outcome is the selected branch,
// and Detail is optional reviewer-facing context.
type CompositionDecision struct {
	Rule    string `json:"rule"`
	Outcome string `json:"outcome"`
	Detail  string `json:"detail,omitempty"`
}

// OfficeSection groups related elements within an office hour.
// If Collapsible is true the section should be rendered as a collapsed disclosure widget.
type OfficeSection struct {
	Label       string
	Collapsible bool
	Elements    []OfficeElement
}

// OfficeHour represents a fully composed office hour ready for rendering.
type OfficeHour struct {
	Date      time.Time
	Hour      string
	Title     string
	Season    Season
	Feast     string
	Color     Color
	Sections  []OfficeSection
	Decisions []CompositionDecision
}
