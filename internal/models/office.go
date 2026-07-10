package models

import "time"

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

// OfficeElement represents a single element in an office hour.
// SlotRef is set for elements that went through proper resolution. SourceRef
// is the primary corpus key that supplied the rendered text. SourceRefs is the
// complete dependency set when composition used more than one entry (for
// example, a proper hymn with a seasonal doxology). These fields are excluded
// from review hashes and never rendered.
type OfficeElement struct {
	Type       ElementType
	Text       string
	Label      string
	Rubric     string
	SlotRef    string
	SourceRef  string
	SourceRefs []string
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
