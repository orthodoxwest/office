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
// SlotRef and SourceRef are set only for elements that went through proper
// resolution (proper-* slots and commemoration texts): SlotRef is the ref
// requested by the hour definition (e.g. "hymn", "psalm-antiphon-1") and
// SourceRef is the corpus key the text actually resolved from, so audits can
// tell which tier (proper/commons/seasonal/ordinary) supplied a slot that
// could have carried a proper text. Both are excluded from review hashes and
// never rendered.
type OfficeElement struct {
	Type      ElementType
	Text      string
	Label     string
	Rubric    string
	SlotRef   string
	SourceRef string
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
	Date     time.Time
	Hour     string
	Title    string
	Season   Season
	Feast    string
	Color    Color
	Sections []OfficeSection
}
