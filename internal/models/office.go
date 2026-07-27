package models

import (
	"strings"
	"time"
	"unicode/utf8"
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
)

// VoiceSpan is a contiguous stretch of prayer text with a spoken/silent delivery.
// Secret prayers are composed of an aloud incipit, a silent body, and optionally
// an aloud tail (e.g. the pre-collect Our Father). Nil/empty Voice means the
// whole Text is spoken normally.
type VoiceSpan struct {
	Text   string
	Spoken bool
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
	Type       ElementType
	Text       string
	Label      string
	Incipit    string
	Rubric     string
	Voice      []VoiceSpan
	SlotRef    string
	SourceRef  string
	SourceRefs []string
	Announce   bool
}

// DisplayText is the string a renderer should print for this element. For an
// announced antiphon it is the text through the mediant asterisk or dagger;
// otherwise it is Text.
func (e OfficeElement) DisplayText() string {
	if e.Type == Antiphon && e.Announce {
		return AntiphonAnnouncement(e.Text)
	}
	return e.Text
}

// AntiphonAnnouncement returns the form said when an antiphon is merely
// announced before a psalm: the words through the first mediant asterisk (*)
// or dagger (†). When the corpus text has no such mark, the full text is
// returned — there is no safe partial cut without a pointing cue.
//
// A single left-to-right scan picks the earliest mark. Comparing Index results
// across marker strings left an equivalent boundary mutant (i < cut vs i <= cut)
// that no test can kill for distinct one-rune markers.
func AntiphonAnnouncement(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	for i, r := range text {
		switch r {
		case '*', '†', '‡':
			end := i + utf8.RuneLen(r)
			return strings.TrimRight(text[:end], " \t")
		}
	}
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
