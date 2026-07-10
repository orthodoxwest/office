// Package office implements the office composition engine.
package office

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/texts"
)

// HourComposer composes a specific liturgical hour for a given calendar day.
type HourComposer interface {
	Compose(day *models.CalendarDay, sections []HourSection, corpus *texts.TextCorpus) (*models.OfficeHour, error)
}

// MoveableReceiver is implemented by composers that need moveable feast dates.
type MoveableReceiver interface {
	SetMoveable(m *calendar.MoveableDates)
}

// Engine loads hour definitions and text corpus, then delegates to hour-specific composers.
type Engine struct {
	dataDir   string
	corpus    *texts.TextCorpus
	composers map[string]HourComposer
}

// NewEngine creates a new office engine, loading the text corpus from dataDir.
func NewEngine(dataDir string) (*Engine, error) {
	corpus, err := texts.LoadTexts(dataDir)
	if err != nil {
		return nil, fmt.Errorf("loading text corpus: %w", err)
	}

	e := &Engine{
		dataDir:   dataDir,
		corpus:    corpus,
		composers: make(map[string]HourComposer),
	}

	e.composers["compline"] = &ComplineComposer{}
	e.composers["prime"] = &PrimeComposer{}
	e.composers["lauds"] = &LaudsComposer{}
	e.composers["vespers"] = &VespersComposer{}
	e.composers["terce"] = &MinorHourComposer{Name: "Terce"}
	e.composers["sext"] = &MinorHourComposer{Name: "Sext"}
	e.composers["none"] = &MinorHourComposer{Name: "None"}

	return e, nil
}

// ComposeHour composes the named hour for the given calendar day.
func (e *Engine) ComposeHour(hourName string, day *models.CalendarDay, moveable *calendar.MoveableDates) (*models.OfficeHour, error) {
	composer, ok := e.composers[hourName]
	if !ok {
		return nil, fmt.Errorf("unknown hour: %s", hourName)
	}

	// Set moveable dates on composers that need them
	if mr, ok := composer.(MoveableReceiver); ok {
		mr.SetMoveable(moveable)
	}

	defPath := filepath.Join(e.dataDir, "office", hourName+".txt")
	sections, err := ParseHourDefinition(defPath)
	if err != nil {
		return nil, fmt.Errorf("parsing hour definition: %w", err)
	}

	hour, err := composer.Compose(day, sections, e.corpus)
	if err != nil {
		return nil, fmt.Errorf("composing %s: %w", hourName, err)
	}

	collapseUniformAntiphons(hour)
	markPsalmDoxologies(hour)
	appendContextDecisions(hour, day, hourName)
	return hour, nil
}

func appendContextDecisions(hour *models.OfficeHour, day *models.CalendarDay, hourName string) {
	add := func(rule, outcome, detail string) {
		hour.Decisions = append(hour.Decisions, models.CompositionDecision{Rule: rule, Outcome: outcome, Detail: detail})
	}

	add("context:season", string(day.Season), "")
	add("context:weekday", strings.ToLower(day.Date.Weekday().String()), "")
	add("occurrence", day.ResolutionRule, "")
	hour.Decisions = append(hour.Decisions, day.OccurrenceDecisions...)
	if day.Celebration == nil {
		add("context:office", "feria", "")
	} else {
		add("context:office", "celebration", day.Celebration.ID)
		add("context:rank", string(day.Celebration.Rank), "")
		add("context:category", string(day.Celebration.Category), "")
	}
	add("context:commemorations", fmt.Sprintf("%d", len(day.Commemorations)), "")
	if day.WithinOctaveOf != "" {
		add("context:octave", "within", day.WithinOctaveOf)
	} else {
		add("context:octave", "outside", "")
	}
	if day.FeriaCommemoration != nil {
		add("context:feria-commemoration", "present", day.FeriaCommemoration.ProperID)
	}
	if hourName == "vespers" {
		owner := "not-applicable"
		switch day.Vespers.Owner {
		case models.VespersIIOfPreceding:
			owner = "second-of-preceding"
		case models.VespersIOfFollowing:
			owner = "first-of-following"
		}
		add("vespers:owner", owner, "")
		add("vespers:rule", day.Vespers.Rule, "")
		hour.Decisions = append(hour.Decisions, day.Vespers.Decisions...)
	}
}

// collapseUniformAntiphons renders psalm groups "under one antiphon": within
// a run of adjacent psalm-bearing sections, a stretch of three or more
// consecutive antiphon elements sharing the same text (e.g. the paschal
// Alleluia, or one antiphon spanning a split psalm) is reduced to its first
// and last occurrence — said before the first psalm of the group and in full
// after the last, not repeated between. Doubled framing around a single
// psalm (two occurrences) is left alone, as are differing antiphons.
func collapseUniformAntiphons(hour *models.OfficeHour) {
	type pos struct{ section, elem int }

	for start := 0; start < len(hour.Sections); start++ {
		if !sectionHasPsalmody(hour.Sections[start]) {
			continue
		}
		end := start
		for end+1 < len(hour.Sections) && sectionHasPsalmody(hour.Sections[end+1]) {
			end++
		}

		// Collect antiphon positions across the run in order.
		var ants []pos
		for si := start; si <= end; si++ {
			for i, el := range hour.Sections[si].Elements {
				if el.Type == models.Antiphon {
					ants = append(ants, pos{si, i})
				}
			}
		}

		// Drop the interior of each maximal same-text group of length >= 3.
		drop := map[pos]bool{}
		text := func(p pos) string { return hour.Sections[p.section].Elements[p.elem].Text }
		for lo := 0; lo < len(ants); {
			hi := lo
			for hi+1 < len(ants) && text(ants[hi+1]) == text(ants[lo]) {
				hi++
			}
			if hi-lo+1 >= 3 {
				for _, p := range ants[lo+1 : hi] {
					drop[p] = true
				}
			}
			lo = hi + 1
		}
		if len(drop) > 0 {
			for si := start; si <= end; si++ {
				elems := hour.Sections[si].Elements
				kept := elems[:0]
				for i := range elems {
					if !drop[pos{si, i}] {
						kept = append(kept, elems[i])
					}
				}
				hour.Sections[si].Elements = kept
			}
		}
		start = end
	}
}

// sectionHasPsalmody reports whether a section contains a psalm or canticle.
func sectionHasPsalmody(s models.OfficeSection) bool {
	for _, el := range s.Elements {
		if el.Type == models.Psalm || el.Type == models.Canticle {
			return true
		}
	}
	return false
}

// markPsalmDoxologies promotes any Doxology element that immediately follows
// a Psalm or Canticle to PsalmDoxology, so the renderer can style it like a
// psalm verse (with * mediant). Doxologies in other positions (e.g. embedded
// in versicle or responsory blocks) are left as plain Doxology.
func markPsalmDoxologies(hour *models.OfficeHour) {
	for si := range hour.Sections {
		elems := hour.Sections[si].Elements
		for i := range elems {
			if elems[i].Type != models.Doxology {
				continue
			}
			if i > 0 {
				prev := elems[i-1].Type
				if prev == models.Psalm || prev == models.Canticle {
					elems[i].Type = models.PsalmDoxology
				}
			}
		}
	}
}

// resolveElement converts a single HourElement into an OfficeElement by looking up text.
func resolveElement(elem HourElement, corpus *texts.TextCorpus) models.OfficeElement {
	text := corpus.Get(elem.Ref)
	if text == "" {
		text = fmt.Sprintf("[Text not found: %s]", elem.Ref)
	}

	elemType := mapElementType(elem.Type)
	label := formatLabel(elem.Type, elem.Ref)
	if elemType == models.Chapter {
		ref, body := extractChapterRef(text)
		return models.OfficeElement{Type: models.Chapter, Text: body, Label: ref, SourceRef: elem.Ref, SourceRefs: []string{elem.Ref}}
	}
	if elemType == models.Preces {
		return models.OfficeElement{Type: models.Preces, Text: text, Label: "Preces", SourceRef: elem.Ref, SourceRefs: []string{elem.Ref}}
	}
	return models.OfficeElement{
		Type:       elemType,
		Text:       text,
		Label:      label,
		SourceRef:  elem.Ref,
		SourceRefs: []string{elem.Ref},
	}
}

// resolveMarianElement resolves the seasonal Marian antiphon (with its versicle,
// response, and collect) for the given day.
func resolveMarianElement(day *models.CalendarDay, corpus *texts.TextCorpus) models.OfficeElement {
	ref := "ordinary/marian/" + day.MarianAntiphon
	oe := models.OfficeElement{
		Type:       models.Antiphon,
		Text:       corpus.Get(ref),
		Label:      marianLabel(day.MarianAntiphon),
		SourceRef:  ref,
		SourceRefs: []string{ref},
	}
	if oe.Text == "" {
		oe.Text = "[Text not found: " + ref + "]"
	}
	return oe
}

// resolveHourElement converts a HourElement to an OfficeElement, applying proper resolution
// for proper-* element types and falling through to resolveElement for all others.
func resolveHourElement(day *models.CalendarDay, hourName string, elem HourElement, corpus *texts.TextCorpus) models.OfficeElement {
	switch elem.Type {
	case "marian":
		if elem.Ref == "seasonal" {
			return resolveMarianElement(day, corpus)
		}
		return resolveElement(elem, corpus)
	case "proper-antiphon":
		text, src := resolveProperText(day, hourName, elem.Ref, corpus)
		return sourcedElement(models.OfficeElement{Type: models.Antiphon, Text: text, SlotRef: elem.Ref, SourceRef: src}, src)
	case "proper-collect":
		text, src := resolveProperCollectText(day, hourName, corpus)
		return sourcedElement(models.OfficeElement{Type: models.Collect, Text: text, SlotRef: "collect", SourceRef: src}, src)
	case "proper-hymn":
		text, src := resolveProperText(day, hourName, elem.Ref, corpus)
		refs := []string{src}
		if dox, doxRef := resolveProperText(day, hourName, "hymn-doxology", corpus); strings.HasPrefix(doxRef, "seasonal/") {
			text = substituteHymnDoxology(text, dox)
			refs = append(refs, doxRef)
		}
		title, body := extractHymnTitle(text)
		return models.OfficeElement{Type: models.Hymn, Text: body, Label: title, SlotRef: elem.Ref, SourceRef: src, SourceRefs: compactRefs(refs)}
	case "proper-responsory":
		text, src := resolveProperText(day, hourName, elem.Ref, corpus)
		return sourcedElement(models.OfficeElement{Type: models.Response, Text: text, SlotRef: elem.Ref, SourceRef: src}, src)
	case "proper-versicle":
		text, src := resolveProperText(day, hourName, elem.Ref, corpus)
		return sourcedElement(models.OfficeElement{Type: models.Versicle, Text: text, SlotRef: elem.Ref, SourceRef: src}, src)
	case "proper-chapter":
		text, src := resolveProperText(day, hourName, elem.Ref, corpus)
		ref, body := extractChapterRef(text)
		return sourcedElement(models.OfficeElement{Type: models.Chapter, Text: body, Label: ref, SlotRef: elem.Ref, SourceRef: src}, src)
	default:
		return resolveElement(elem, corpus)
	}
}

func sourcedElement(elem models.OfficeElement, refs ...string) models.OfficeElement {
	elem.SourceRefs = compactRefs(refs)
	return elem
}

func compactRefs(refs []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(refs))
	for _, ref := range refs {
		if ref == "" || seen[ref] {
			continue
		}
		seen[ref] = true
		out = append(out, ref)
	}
	return out
}

// mapElementType converts hour definition type strings to model ElementType constants.
func mapElementType(t string) models.ElementType {
	switch t {
	case "psalm":
		return models.Psalm
	case "canticle":
		return models.Canticle
	case "hymn":
		return models.Hymn
	case "antiphon":
		return models.Antiphon
	case "versicle":
		return models.Versicle
	case "response":
		return models.Response
	case "prayer":
		return models.Prayer
	case "preces":
		return models.Preces
	case "gloria-patri":
		return models.Doxology
	case "rubric":
		return models.Rubric
	case "chapter":
		return models.Chapter
	case "collect":
		return models.Collect
	case "blessing":
		return models.Blessing
	case "marian":
		return models.Antiphon
	case "proper-antiphon":
		return models.Antiphon
	case "proper-collect":
		return models.Collect
	case "proper-hymn":
		return models.Hymn
	case "proper-responsory":
		return models.Response
	case "proper-versicle":
		return models.Versicle
	case "proper-chapter":
		return models.Chapter
	case "commemorations":
		return models.Rubric
	default:
		return models.Rubric
	}
}

// formatLabel produces a human-readable label from a type and ref.
func formatLabel(elemType, ref string) string {
	// Extract the last path component as the label
	parts := strings.Split(ref, "/")
	name := parts[len(parts)-1]
	name = strings.ReplaceAll(name, "-", " ")

	switch elemType {
	case "psalm":
		// "psalms/004" → "Psalm 4"
		name = strings.TrimLeft(name, "0")
		if name == "" {
			name = "0"
		}
		return "Psalm " + name
	case "canticle":
		return titleCase(name)
	case "hymn":
		return titleCase(name)
	default:
		return ""
	}
}

// extractHymnTitle splits a hymn text into a Latin title and body.
// If the text begins with a single-line block followed by a blank line,
// that line is the title; the remainder is the body. Otherwise title is empty.
func extractHymnTitle(text string) (title, body string) {
	firstBlock, rest, found := strings.Cut(text, "\n\n")
	if !found {
		return "", text
	}
	firstBlock = strings.TrimSpace(firstBlock)
	if strings.ContainsRune(firstBlock, '\n') {
		// Multi-line first block — not a title
		return "", text
	}
	return firstBlock, strings.TrimSpace(rest)
}

// extractChapterRef splits a chapter text into a scripture reference and body.
// If the first line starts with "!", that line (without the "!") is the reference;
// the remainder (after stripping the leading blank line) is the body.
func extractChapterRef(text string) (ref, body string) {
	first, rest, found := strings.Cut(text, "\n")
	first = strings.TrimSpace(first)
	if found && strings.HasPrefix(first, "!") {
		return first[1:], strings.TrimSpace(rest)
	}
	return "", text
}

// substituteHymnDoxology replaces the last stanza of a hymn body with a
// seasonal doxology. It only acts on hymns ending with "Amen." (L.M. pattern).
func substituteHymnDoxology(hymnText, doxology string) string {
	trimmed := strings.TrimSpace(hymnText)
	if !strings.HasSuffix(trimmed, "Amen.") {
		return hymnText
	}
	idx := strings.LastIndex(trimmed, "\n\n")
	if idx < 0 {
		return hymnText
	}
	return trimmed[:idx+2] + strings.TrimSpace(doxology)
}

func titleCase(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}
