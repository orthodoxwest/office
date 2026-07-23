package office

import (
	"fmt"
	"strings"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/texts"
)

// LaudsComposer composes the hour of Lauds.
type LaudsComposer struct{}

// Compose builds a complete Lauds hour for the given day.
func (l *LaudsComposer) Compose(day *models.CalendarDay, sections []HourSection, corpus *texts.TextCorpus, moveable *calendar.MoveableDates) (*models.OfficeHour, error) {
	return composeMajorHour(day, sections, corpus, moveable, majorHourOptions{
		hourName: "lauds",
		title:    "Lauds",
	})
}

// addCommemorations returns commemoration elements for each commemorated feast.
func addCommemorations(day *models.CalendarDay, hourName string, corpus *texts.TextCorpus) []models.OfficeElement {
	comms := day.Commemorations

	// The occurring privileged feria is commemorated at Lauds when a feast takes
	// the office on a penitential weekday, ahead of any sanctoral commemoration.
	// Its Vespers commemoration is concurrence-dependent and handled separately.
	if hourName == "lauds" && day.FeriaCommemoration != nil {
		comms = append([]*models.Feast{day.FeriaCommemoration}, comms...)
	}

	var elems []models.OfficeElement
	for _, comm := range comms {
		lookup := func(ref string) (string, string) {
			if isSynthesizedFeria(comm) {
				return lookupFeriaCommemoration(day, comm, hourName, ref, corpus)
			}
			if hourName == "vespers" && comm.ID == day.FollowingOfficeCommemorationID {
				return lookupFollowingOfficeCommemoration(comm, day.Season, ref, corpus)
			}
			return lookupCommemoration(comm, day.Season, hourName, ref, corpus)
		}
		elems = append(elems, models.OfficeElement{
			Type: models.Heading,
			Text: fmt.Sprintf("Commemoration of %s", comm.CommemorationName()),
		})

		// Antiphon: feast-specific or fallback
		antText, antSrc := lookup("commemoration-antiphon")
		elems = append(elems, models.OfficeElement{
			Type:       models.Antiphon,
			Text:       antText,
			SlotRef:    "commemoration-antiphon",
			SourceRef:  antSrc,
			SourceRefs: compactRefs([]string{antSrc}),
		})

		// Versicle + Response
		versText, versSrc := lookup("commemoration-versicle")
		elems = append(elems, models.OfficeElement{
			Type:       models.Versicle,
			Text:       versText,
			SlotRef:    "commemoration-versicle",
			SourceRef:  versSrc,
			SourceRefs: compactRefs([]string{versSrc}),
		})

		// Collect
		collectText, collectSrc := lookup("commemoration-collect")
		elems = append(elems, models.OfficeElement{
			Type:       models.Collect,
			Text:       collectText,
			SlotRef:    "commemoration-collect",
			SourceRef:  collectSrc,
			SourceRefs: compactRefs([]string{collectSrc}),
		})
	}
	return elems
}

// lookupFollowingOfficeCommemoration resolves a following celebration
// commemorated at II Vespers. Its Antiphon and versicle are those of I
// Vespers, while its collect follows the normal commemoration lookup (XIV.14).
func lookupFollowingOfficeCommemoration(feast *models.Feast, season models.Season, ref string, corpus *texts.TextCorpus) (string, string) {
	var candidates []string
	switch ref {
	case "commemoration-antiphon":
		candidates = []string{"magnificat-antiphon-first", "magnificat-antiphon"}
	case "commemoration-versicle":
		candidates = []string{"versicle-first-vespers", "versicle-vespers"}
	}
	for _, candidate := range candidates {
		if text, source := lookupCommemoration(feast, season, "vespers", candidate, corpus); text != "" &&
			!strings.HasPrefix(text, "[") {
			return text, source
		}
	}
	return lookupCommemoration(feast, season, "vespers", ref, corpus)
}

// lookupFeriaCommemoration resolves the commemoration slots for the synthesized
// occurring feria, which has no proper of its own: the gospel-canticle
// antiphon comes from the governing week's Proper when available, the little
// versicle from the Psalter, and the collect from the governing Sunday carried
// on ProperID. Falls back to the generic ordinary slot when a preferred source
// is missing.
func lookupFeriaCommemoration(day *models.CalendarDay, feast *models.Feast, hourName, ref string, corpus *texts.TextCorpus) (string, string) {
	switch ref {
	case "commemoration-antiphon":
		antSlot := "benedictus-antiphon"
		if hourName == "vespers" {
			antSlot = "magnificat-antiphon"
		}
		if day != nil && feast.ProperID != "" {
			antRef := "proper/" + feast.ProperID + "/" + antSlot + "-" +
				strings.ToLower(civilWeekday(day).String())
			if text := corpus.Get(antRef); text != "" {
				return text, antRef
			}
		}
		antRef := "ordinary/" + hourName + "/" + antSlot
		if text := corpus.Get(antRef); text != "" {
			return text, antRef
		}
	case "commemoration-versicle":
		if text := corpus.Get("ordinary/" + hourName + "/versicle"); text != "" {
			return text, "ordinary/" + hourName + "/versicle"
		}
	case "commemoration-collect":
		if feast.ProperID != "" {
			collectRef := "proper/" + feast.ProperID + "/collect"
			if text := corpus.Get(collectRef); text != "" {
				return text, collectRef
			}
		}
	}

	// Generic ordinary fallback (no proper name to substitute for a feria).
	ordinaryRef := "ordinary/" + hourName + "/" + ref
	if text := corpus.Get(ordinaryRef); text != "" {
		return text, ordinaryRef
	}
	return fmt.Sprintf("[%s: %s]", ref, feast.ID), ref
}

// lookupTemporalCommemoration resolves the commemoration slots for a de Tempore
// commemoration — a Sunday, Ember day, or vigil — that is not the synthesized
// occurring feria. These never take the saint-shaped "O holy N." antiphon or
// "The Lord hath chosen him" versicle: the antiphon is the day's own
// gospel-canticle antiphon (the feast's proper if it has one, else the
// Psalter's), the versicle is the hour's little versicle, and the collect is the
// feast's proper collect.
func lookupTemporalCommemoration(feast *models.Feast, hourName, ref string, corpus *texts.TextCorpus) (string, string) {
	// A de Tempore feast may carry its own dedicated commemoration slot (e.g. the
	// Vigil of the Epiphany's "While all things were in quiet silence"); prefer it.
	for _, feastID := range feastProperIDs(feast) {
		if text := corpus.Get("proper/" + feastID + "/" + ref); text != "" {
			return text, "proper/" + feastID + "/" + ref
		}
	}

	switch ref {
	case "commemoration-antiphon":
		// No dedicated commemoration antiphon: use the day's own gospel-canticle
		// antiphon (the feast's proper if it has one, else the Psalter's).
		antSlot := "benedictus-antiphon"
		if hourName == "vespers" {
			antSlot = "magnificat-antiphon"
		}
		for _, feastID := range feastProperIDs(feast) {
			properRef := "proper/" + feastID + "/" + antSlot
			if text := corpus.Get(properRef); text != "" {
				return text, properRef
			}
		}
		if text := corpus.Get("ordinary/" + hourName + "/" + antSlot); text != "" {
			return text, "ordinary/" + hourName + "/" + antSlot
		}
	case "commemoration-versicle":
		if text := corpus.Get("ordinary/" + hourName + "/versicle"); text != "" {
			return text, "ordinary/" + hourName + "/versicle"
		}
	case "commemoration-collect":
		for _, feastID := range feastProperIDs(feast) {
			collectRef := "proper/" + feastID + "/collect"
			if text := corpus.Get(collectRef); text != "" {
				return text, collectRef
			}
		}
	}

	// Generic ordinary fallback (no proper name to substitute for a de Tempore day).
	ordinaryRef := "ordinary/" + hourName + "/" + ref
	if text := corpus.Get(ordinaryRef); text != "" {
		return text, ordinaryRef
	}
	return fmt.Sprintf("[%s: %s]", ref, feast.ID), ref
}

// commemorationFallbackSlots returns the hour-appropriate content slots that
// a sanctoral commemoration uses when a dedicated commemoration-* section is
// missing. Per the ferial office books, each commemoration is the proper
// gospel-canticle antiphon, the hour's little versicle, and the collect of
// the feast or common.
func commemorationFallbackSlots(hourName, ref string) []string {
	switch ref {
	case "commemoration-antiphon":
		if hourName == "vespers" {
			return []string{"magnificat-antiphon", "magnificat-antiphon-first"}
		}
		return []string{"benedictus-antiphon"}
	case "commemoration-versicle":
		if hourName == "vespers" {
			return []string{"versicle-vespers", "versicle-lauds", "versicle"}
		}
		return []string{"versicle-lauds", "versicle"}
	case "commemoration-collect":
		return []string{"collect"}
	default:
		return nil
	}
}

// lookupCommemoration looks up a commemoration text, trying in order:
// feast-specific proper, commons (paschal then regular), ordinary fallback.
// Applies N. substitution using the feast's ProperName.
// Returns the text and the corpus ref it was resolved from.
func lookupCommemoration(feast *models.Feast, season models.Season, hourName, ref string, corpus *texts.TextCorpus) (string, string) {
	// The synthesized occurring feria has no proper of its own and must never
	// fall through to the saint-shaped fallbacks (which would leave an unfilled
	// "N."). It takes its Antiphon and versicle from the Psalter and its collect
	// from the governing Sunday. Ember days and vigils are real feasts with
	// their own propers and are resolved by the generic path below.
	if isSynthesizedFeria(feast) {
		return lookupFeriaCommemoration(nil, feast, hourName, ref, corpus)
	}

	// Sundays, Ember days, and vigils are de Tempore: their commemoration takes
	// the day's own gospel-canticle antiphon and the hour's little versicle, never
	// the saint-shaped fallbacks (which would leave an unfilled "N.").
	if feast.Category == models.CategorySunday || feast.Category == models.CategoryFeria {
		return lookupTemporalCommemoration(feast, hourName, ref, corpus)
	}

	properName := feastProperName(feast)

	// 1. Feast-specific dedicated slot (hour-qualified first).
	for _, feastID := range feastProperIDs(feast) {
		prefix := "proper/" + feastID + "/"
		if text, resolved := lookupSectionText(prefix, season, hourName, ref, corpus); text != "" {
			return substituteProperName(text, properName), resolved
		}
	}

	// 1b. Feast-specific content fallbacks (gospel antiphon / hour versicle / collect).
	for _, fallback := range commemorationFallbackSlots(hourName, ref) {
		for _, feastID := range feastProperIDs(feast) {
			prefix := "proper/" + feastID + "/"
			if text, resolved := lookupSectionText(prefix, season, hourName, fallback, corpus); text != "" {
				return substituteProperName(text, properName), resolved
			}
		}
	}

	// 2. Commons dedicated slot (paschal, then regular).
	if text, resolved := lookupCommonsText(feast.Category, season, hourName, ref, corpus); text != "" {
		return substituteProperName(text, properName), resolved
	}

	// 2b. Commons content fallbacks.
	for _, fallback := range commemorationFallbackSlots(hourName, ref) {
		if text, resolved := lookupCommonsText(feast.Category, season, hourName, fallback, corpus); text != "" {
			return substituteProperName(text, properName), resolved
		}
	}

	// 3. Ordinary fallback (hour-specific)
	ordinaryRef := "ordinary/" + hourName + "/" + ref
	if text := corpus.Get(ordinaryRef); text != "" {
		return substituteProperName(text, properName), ordinaryRef
	}

	return fmt.Sprintf("[%s: %s]", ref, feast.ID), ref
}
