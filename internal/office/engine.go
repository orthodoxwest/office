// Package office implements the office composition engine.
package office

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/texts"
)

// HourComposer composes a specific liturgical hour for a given calendar day.
type HourComposer interface {
	Compose(day *models.CalendarDay, sections []HourSection, corpus *texts.TextCorpus, moveable *calendar.MoveableDates) (*models.OfficeHour, error)
}

// Engine loads hour definitions and text corpus, then delegates to hour-specific composers.
type Engine struct {
	corpus      *texts.TextCorpus
	composers   map[string]HourComposer
	definitions map[string][]HourSection
}

// NewEngine creates an office engine, loading the text corpus and compiling
// all hour definitions from dataDir once for immutable concurrent reuse.
func NewEngine(dataDir string) (*Engine, error) {
	corpus, err := texts.LoadTexts(dataDir)
	if err != nil {
		return nil, fmt.Errorf("loading text corpus: %w", err)
	}

	e := &Engine{
		corpus:      corpus,
		composers:   make(map[string]HourComposer),
		definitions: make(map[string][]HourSection, len(hourNames)),
	}

	e.composers["compline"] = &ComplineComposer{}
	e.composers["prime"] = &PrimeComposer{}
	e.composers["lauds"] = &LaudsComposer{}
	e.composers["vespers"] = &VespersComposer{}
	e.composers["terce"] = &MinorHourComposer{Name: "Terce"}
	e.composers["sext"] = &MinorHourComposer{Name: "Sext"}
	e.composers["none"] = &MinorHourComposer{Name: "None"}
	for _, hourName := range hourNames {
		defPath := filepath.Join(dataDir, "office", hourName+".txt")
		sections, err := ParseHourDefinition(defPath)
		if err != nil {
			return nil, fmt.Errorf("parsing %s definition: %w", hourName, err)
		}
		e.definitions[hourName] = sections
	}

	return e, nil
}

// ComposeHour composes the named hour for the given calendar day.
func (e *Engine) ComposeHour(hourName string, day *models.CalendarDay, moveable *calendar.MoveableDates) (*models.OfficeHour, error) {
	composer, ok := e.composers[hourName]
	if !ok {
		return nil, fmt.Errorf("unknown hour: %s", hourName)
	}

	sections := e.definitions[hourName]
	hour, err := composer.Compose(day, sections, e.corpus, moveable)
	if err != nil {
		return nil, fmt.Errorf("composing %s: %w", hourName, err)
	}

	dropEmptySections(hour)
	canonicalizeSourceRefs(hour, e.corpus)
	collapseUniformAntiphons(hour)
	markPsalmDoxologies(hour)
	markAnnouncedAntiphons(hour, day, hourName)
	appendContextDecisions(hour, day, hourName, moveable)
	return hour, nil
}

// canonicalizeSourceRefs makes review dependencies follow corpus aliases.
// SourceRef remains the key selected by composition so resolution-tier
// diagnostics continue to distinguish proper, commons, seasonal, and ordinary
// selection; SourceRefs carries the canonical texts that humans must review.
func canonicalizeSourceRefs(hour *models.OfficeHour, corpus *texts.TextCorpus) {
	canonical := func(ref string) string {
		if resolved := corpus.CanonicalRef(ref); resolved != "" {
			return resolved
		}
		return ref
	}
	for si := range hour.Sections {
		for ei := range hour.Sections[si].Elements {
			elem := &hour.Sections[si].Elements[ei]
			originalRefs := elem.SourceRefs
			if len(originalRefs) == 0 && elem.SourceRef != "" {
				originalRefs = []string{elem.SourceRef}
			}
			refs := make([]string, len(originalRefs))
			for i, ref := range originalRefs {
				refs[i] = canonical(ref)
			}
			elem.SourceRefs = compactRefs(refs)
		}
	}
}

func appendContextDecisions(hour *models.OfficeHour, day *models.CalendarDay, hourName string, moveable *calendar.MoveableDates) {
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

	// Structural dispositions are recorded only on hours that render the
	// corresponding element, so sign-off credit cannot cover unread sections.
	// Preces: Prime and Compline. Suffrage: Lauds and Vespers. Marian: Lauds,
	// Vespers, and Compline.
	switch hourName {
	case "prime":
		_, precesReason := precesDisposition(day, moveable)
		add("preces", precesReason, "")
	case "compline":
		_, precesReason := precesDisposition(complineOfficeDay(day), moveable)
		add("preces", precesReason, "")
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
		officeDay := vespersOfficeDay(day)
		appendEveningOfficeContextDecisions(hour, officeDay)
		// Suffrage and Marian selection follow the office that owns Vespers.
		_, suffrageReason := suffrageDisposition(officeDay, moveable)
		add("suffrage", suffrageReason, "")
		addMarianDecisions(add, officeDay, hourName)
	} else if hourName == "lauds" {
		_, suffrageReason := suffrageDisposition(day, moveable)
		add("suffrage", suffrageReason, "")
		addMarianDecisions(add, day, hourName)
	} else if hourName == "compline" {
		officeDay := complineOfficeDay(day)
		appendEveningOfficeContextDecisions(hour, officeDay)
		addMarianDecisions(add, officeDay, hourName)
	}

	// Record whether psalm/canticle antiphons are doubled at this hour so the
	// decision is visible in review explain / assurance traces.
	if antiphonsDoubled(day, hourName) {
		add("antiphon:doubling", "doubled", "")
	} else {
		add("antiphon:doubling", "announced", "")
	}
}

// MarianBoundary outcomes for plan features (rule "marian:boundary").
const (
	MarianBoundaryCivilDay                    = "civil-day"
	MarianBoundaryPurificationVespersOverride = "purification-vespers-override"
)

func addMarianDecisions(add func(rule, outcome, detail string), day *models.CalendarDay, hourName string) {
	key, boundary := marianAntiphonSelection(day, hourName)
	add("marian:selection", key, "")
	add("marian:boundary", boundary, "")
}

// appendEveningOfficeContextDecisions describes the synthetic office day that
// actually drove Vespers or Compline composition. The existing context:* and
// occurrence decisions remain civil-day provenance; these distinct rule IDs
// make the two frames explicit without changing the meaning of the
// review-facing API.
func appendEveningOfficeContextDecisions(hour *models.OfficeHour, officeDay *models.CalendarDay) {
	add := func(rule, outcome, detail string) {
		hour.Decisions = append(hour.Decisions, models.CompositionDecision{Rule: rule, Outcome: outcome, Detail: detail})
	}

	add("office-context:season", string(officeDay.Season), "")
	add("office-context:weekday", strings.ToLower(officeDay.Date.Weekday().String()), "")
	if officeDay.Celebration == nil {
		add("office-context:office", "feria", "")
	} else {
		add("office-context:office", "celebration", officeDay.Celebration.ID)
		add("office-context:rank", string(officeDay.Celebration.Rank), "")
		add("office-context:category", string(officeDay.Celebration.Category), "")
	}
	add("office-context:commemorations", fmt.Sprintf("%d", len(officeDay.Commemorations)), "")
	if officeDay.WithinOctaveOf != "" {
		add("office-context:octave", "within", officeDay.WithinOctaveOf)
	} else {
		add("office-context:octave", "outside", "")
	}
	if officeDay.FirstVespers {
		add("office-context:first-vespers", "yes", "")
	} else {
		add("office-context:first-vespers", "no", "")
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

// antiphonsDoubled reports whether psalm and canticle antiphons are said
// entire both before and after at this hour. General Rubrics I.4 / XXIV.8:
// only Vespers, Matins, and Lauds of a Double office double the antiphons;
// at every other hour, and in a non-Double office, the antiphon is merely
// begun before and said entire after.
func antiphonsDoubled(day *models.CalendarDay, hourName string) bool {
	if hourName != "lauds" && hourName != "vespers" {
		return false
	}
	officeDay := day
	if hourName == "vespers" {
		officeDay = vespersOfficeDay(day)
	}
	if officeDay == nil || officeDay.Celebration == nil {
		return false
	}
	return officeDay.Celebration.Rank.IsDouble()
}

// markAnnouncedAntiphons flags the opening half of each psalm/canticle
// antiphon frame when the office does not double antiphons at this hour.
// Text stays whole; renderers print models.AntiphonAnnouncement via DisplayText.
func markAnnouncedAntiphons(hour *models.OfficeHour, day *models.CalendarDay, hourName string) {
	if hour == nil || antiphonsDoubled(day, hourName) {
		return
	}
	for si := range hour.Sections {
		elems := hour.Sections[si].Elements
		for i := range elems {
			if elems[i].Type != models.Antiphon {
				continue
			}
			if isBeforePsalmodyAntiphon(elems, i) {
				elems[i].Announce = true
			}
		}
	}
}

// isBeforePsalmodyAntiphon reports whether elems[i] is the antiphon said
// immediately before a psalm or canticle (as opposed to the full repeat after
// the Gloria, a commemoration antiphon, the Marian antiphon, or the opening
// Alleluia). Composition always places the opening antiphon directly before
// the psalm/canticle, so only the next element is considered.
func isBeforePsalmodyAntiphon(elems []models.OfficeElement, i int) bool {
	if i < 0 || i >= len(elems) || elems[i].Type != models.Antiphon {
		return false
	}
	if i+1 >= len(elems) {
		return false
	}
	switch elems[i+1].Type {
	case models.Psalm, models.Canticle:
		return true
	default:
		// Closing antiphon (next is another ant, heading, etc.), Gloria, or
		// non-psalmody material such as a commemoration versicle.
		return false
	}
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
	// Retain the legacy partly-secret form for callers outside the office
	// definitions. The corporate form owns its complete response, including
	// Amen, and is intentionally not trimmed.
	if elem.Type == "partly-secret-prayer" {
		text = strings.TrimSuffix(text, " Amen.")
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
	oe := models.OfficeElement{
		Type:       elemType,
		Text:       text,
		Label:      label,
		SourceRef:  elem.Ref,
		SourceRefs: []string{elem.Ref},
	}
	if elemType == models.Psalm || elemType == models.Canticle {
		oe.Incipit = corpus.Incipit(elem.Ref)
	}
	switch elem.Type {
	case "secret-prayer":
		oe.Voice = buildPrayerVoice(elem.Ref, text, false)
	case "partly-secret-prayer":
		oe.Voice = buildPrayerVoice(elem.Ref, text, true)
	case "corporate-lord-prayer":
		oe.Voice = buildCorporateLordPrayerVoice(elem.Ref, text)
	case "rubric":
		oe.RubricSpans = buildRubricSpans(elem.Ref, text)
	}
	return oe
}

// marianAntiphonSelection returns the seasonal Marian antiphon corpus slug and
// the boundary branch for the given civil/office day and hour. Alma Redemptoris
// continues through II Vespers of the Purification; Ave Regina begins at
// Compline on February 2.
func marianAntiphonSelection(day *models.CalendarDay, hourName string) (key, boundary string) {
	if day == nil {
		return "", MarianBoundaryCivilDay
	}
	key = day.MarianAntiphon
	if hourName == "vespers" && day.Date.Month() == time.February && day.Date.Day() == 2 {
		return "alma-redemptoris-christmas", MarianBoundaryPurificationVespersOverride
	}
	return key, MarianBoundaryCivilDay
}

// marianAntiphonKey is the slug-only helper used by resolvers.
func marianAntiphonKey(day *models.CalendarDay, hourName string) string {
	key, _ := marianAntiphonSelection(day, hourName)
	return key
}

// resolveMarianElement resolves the seasonal Marian antiphon (with its versicle,
// response, and collect) for the given day.
func resolveMarianElement(day *models.CalendarDay, hourName string, corpus *texts.TextCorpus) models.OfficeElement {
	key := marianAntiphonKey(day, hourName)
	ref := "ordinary/marian/" + key
	oe := models.OfficeElement{
		Type:       models.Antiphon,
		Text:       corpus.Get(ref),
		Label:      marianLabel(key),
		SlotRef:    "marian-antiphon",
		SourceRef:  ref,
		SourceRefs: []string{ref},
	}
	if oe.Text == "" {
		oe.Text = "[Text not found: " + ref + "]"
	}
	return oe
}

// appendHourElement resolves elem and appends it, unless the corpus omits the
// element for this office (texts.OmitMarker), in which case nothing is added.
func appendHourElement(elems []models.OfficeElement, day *models.CalendarDay, hourName string, elem HourElement, corpus *texts.TextCorpus) []models.OfficeElement {
	return appendResolved(elems, resolveHourElement(day, hourName, elem, corpus))
}

// appendResolved appends an already-resolved element unless the corpus omits
// it. Every composer path that can produce an omission goes through here, so
// the marker has one place to be dropped rather than one per composer.
func appendResolved(elems []models.OfficeElement, oe models.OfficeElement) []models.OfficeElement {
	if texts.IsOmitted(oe.Text) {
		return elems
	}
	return append(elems, oe)
}

// dropEmptySections removes sections left with no elements, which happens when
// the corpus omits every element a section holds. Their labels would otherwise
// render as empty headings.
func dropEmptySections(hour *models.OfficeHour) {
	kept := hour.Sections[:0]
	for _, section := range hour.Sections {
		if len(section.Elements) > 0 {
			kept = append(kept, section)
		}
	}
	hour.Sections = kept
}

// resolveHourElement converts a HourElement to an OfficeElement, applying proper resolution
// for proper-* element types and falling through to resolveElement for all others.
// An element the corpus omits resolves with texts.OmitMarker as its text; use
// appendHourElement to drop it rather than render it.
func resolveHourElement(day *models.CalendarDay, hourName string, elem HourElement, corpus *texts.TextCorpus) models.OfficeElement {
	switch elem.Type {
	case "marian":
		if elem.Ref == "seasonal" {
			return resolveMarianElement(day, hourName, corpus)
		}
		return resolveElement(elem, corpus)
	case "gloria-patri":
		// Ref "office" defers to the office of the day: the Gloria Patri, or
		// "Rest eternal" in the Office of the Dead. Only sections shared across
		// every office need it; ones that run in a single office name their
		// text outright.
		if elem.Ref == doxologyRefPerOffice {
			elem.Ref = psalmDoxologyRef(day)
		}
		return resolveElement(elem, corpus)
	case "proper-antiphon":
		text, src := resolveProperText(day, hourName, elem.Ref, corpus)
		return sourcedElement(models.OfficeElement{Type: models.Antiphon, Text: text, SlotRef: elem.Ref, SourceRef: src}, src)
	case "proper-opening-acclamation":
		text, src := resolveProperText(day, hourName, elem.Ref, corpus)
		return sourcedElement(models.OfficeElement{Type: models.OpeningAcclamation, Text: text, SlotRef: elem.Ref, SourceRef: src}, src)
	case "proper-collect":
		text, src := resolveProperCollectText(day, hourName, corpus)
		// The collect of the day is always the first of the hour's run, so it
		// is always concluded (XXXIII.5).
		text, refs := applyConclusion(text, src, corpus)
		elem := sourcedElement(models.OfficeElement{Type: models.Collect, Text: text, SlotRef: "collect", SourceRef: src}, src)
		elem.SourceRefs = compactRefs(refs)
		return elem
	case "proper-hymn":
		text, src := resolveProperText(day, hourName, elem.Ref, corpus)
		if texts.IsOmitted(text) {
			// Alone among the proper-* cases, the hymn decorates its text
			// (the doxology substitution below), which would append wording
			// after the marker and defeat the drop in appendResolved.
			return sourcedElement(models.OfficeElement{Type: models.Hymn, Text: text, SlotRef: elem.Ref, SourceRef: src}, src)
		}
		refs := []string{src}
		doxologyRef := "hymn-doxology"
		if usesAscensionHymnDoxology(day) {
			doxologyRef = "hymn-doxology-ascension"
		}
		if dox, doxRef := resolveProperText(day, hourName, doxologyRef, corpus); strings.HasPrefix(doxRef, "seasonal/") {
			text = substituteHymnDoxology(text, dox)
			refs = append(refs, doxRef)
		}
		title, body := extractHymnTitle(text)
		return models.OfficeElement{Type: models.Hymn, Text: body, Label: title, SlotRef: elem.Ref, SourceRef: src, SourceRefs: compactRefs(refs)}
	case "proper-responsory":
		text, src := resolveProperText(day, hourName, elem.Ref, corpus)
		return sourcedElement(models.OfficeElement{Type: models.Response, Text: text, SlotRef: elem.Ref, SourceRef: src}, src)
	case "proper-short-responsory":
		text, src := resolveProperText(day, hourName, elem.Ref, corpus)
		return sourcedElement(models.OfficeElement{Type: models.ShortResponsory, Text: text, SlotRef: elem.Ref, SourceRef: src}, src)
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

// usesAscensionHymnDoxology reports whether the Ascensiontide ending replaces
// a hymn's ordinary or Easter ending. The calendar's Easter season runs until
// Pentecost, so the date boundary is the moveable Ascension feast itself.
func usesAscensionHymnDoxology(day *models.CalendarDay) bool {
	if day == nil || day.Season != models.Easter {
		return false
	}
	ascension := calendar.ComputeMoveableDates(day.Date.Year()).Ascension
	return !day.Date.Before(ascension)
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
	case "prayer", "secret-prayer", "partly-secret-prayer":
		return models.Prayer
	case "corporate-lord-prayer":
		return models.CorporateLordPrayer
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
	case "proper-opening-acclamation":
		return models.OpeningAcclamation
	case "proper-collect":
		return models.Collect
	case "proper-hymn":
		return models.Hymn
	case "proper-responsory":
		return models.Response
	case "proper-short-responsory":
		return models.ShortResponsory
	case "dialogue":
		return models.Dialogue
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
	return texts.SplitHymnTitle(text)
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

// TraceProperResolution describes a dynamic slot selected by ComposeHour.
// Vespers is evaluated against its liturgical owner, just as ComposeHour does.
// It does not perform a second resolution and cannot affect composed output.
func (e *Engine) TraceProperResolution(day *models.CalendarDay, hourName, ref, selectedRef string) ProperResolutionTrace {
	if hourName == "vespers" {
		day = vespersOfficeDay(day)
	}
	return traceProperResolution(day, hourName, ref, selectedRef, e.corpus)
}

// UnknownCommemorationOwnerReason marks a commemoration trace whose owner
// could not be found among the day's commemorations. The trace fails closed
// with no owner, which on its own is indistinguishable from a legitimately
// unowned day (a feria with no Celebration), so the reason carries the
// distinction the OwnerID column cannot.
const UnknownCommemorationOwnerReason = "unknown-commemoration-owner"

// TraceCommemorationResolution traces a generated commemoration slot. An
// empty owner ID is intentionally not allowed to fall back to the principal
// celebration.
func (e *Engine) TraceCommemorationResolution(day *models.CalendarDay, hourName, ref, selectedRef, ownerID string) ProperResolutionTrace {
	if hourName == "vespers" {
		day = vespersOfficeDay(day)
	}
	ownerDay, found := commemorationOwnerDay(day, ownerID)
	if ownerDay != nil {
		// The office day's FirstVespers describes the incoming celebration,
		// never the commemoration, so it must be re-derived for the owner.
		ownerDay.FirstVespers = found && hourName == "vespers" &&
			commemorationTakesFirstVespers(day, ownerDay.Celebration, ref)
	}
	trace := traceProperResolution(ownerDay, hourName, ref, selectedRef, e.corpus)
	if !found {
		trace.Reason = UnknownCommemorationOwnerReason
	}
	return trace
}

// commemorationTakesFirstVespers reports whether a commemoration at Vespers is
// taken from its owner's I Vespers rather than its II Vespers. It mirrors the
// composer: on an evening that is I Vespers of the following feast the office
// day carries FirstVespers, but the commemorations there are of the outgoing
// office, whose II Vespers they are. Only the incoming office's commemoration
// (XIII.2-17) and a Sunday commemorated at Saturday II Vespers (XIV.14) begin
// with their own I-Vespers text, and only in the slots the composer prefers it
// for — a collect is the same at either Vespers.
func commemorationTakesFirstVespers(day *models.CalendarDay, comm *models.Feast, ref string) bool {
	if day == nil || comm == nil {
		return false
	}
	if comm.ID != "" && comm.ID == day.FollowingOfficeCommemorationID {
		return ref == "commemoration-antiphon" || ref == "commemoration-versicle"
	}
	return isSaturdaySecondVespersSundayCommemoration(day, comm, "vespers", ref)
}

// commemorationOwnerDay changes only the celebration used for proper tracing.
// Composition has already selected the text; this keeps inventory ownership
// tied to the actual commemoration Feast, including its ProperID redirect.
// It reports whether the owner was found; an unfound owner fails closed with
// no celebration at all.
func commemorationOwnerDay(day *models.CalendarDay, ownerID string) (*models.CalendarDay, bool) {
	if day == nil {
		return day, false
	}
	// An empty owner must fail closed before the search: it would otherwise
	// match the first commemoration whose Feast has no ID, which is exactly
	// the case IsCommemoration exists to keep off the principal celebration.
	if ownerID == "" {
		unowned := *day
		unowned.Celebration = nil
		return &unowned, false
	}
	for _, feast := range day.Commemorations {
		if feast != nil && feast.ID == ownerID {
			ownerDay := *day
			ownerDay.Celebration = feast
			return &ownerDay, true
		}
	}
	if day.FeriaCommemoration != nil && day.FeriaCommemoration.ID == ownerID {
		ownerDay := *day
		ownerDay.Celebration = day.FeriaCommemoration
		return &ownerDay, true
	}
	// An explicit owner that is not present in the transformed commemoration
	// list must fail closed; attributing it to the principal celebration would
	// make inventory rows appear to belong to the wrong feast.
	ownerDay := *day
	ownerDay.Celebration = nil
	return &ownerDay, false
}
