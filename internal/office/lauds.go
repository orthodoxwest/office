package office

import (
	"fmt"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/texts"
)

// LaudsComposer composes the hour of Lauds.
type LaudsComposer struct {
	Moveable *calendar.MoveableDates
}

// SetMoveable sets the moveable feast dates for preces calculation.
func (l *LaudsComposer) SetMoveable(m *calendar.MoveableDates) {
	l.Moveable = m
}

// Compose builds a complete Lauds hour for the given day.
func (l *LaudsComposer) Compose(day *models.CalendarDay, sections []HourSection, corpus *texts.TextCorpus) (*models.OfficeHour, error) {
	return composeMajorHour(day, sections, corpus, l.Moveable, majorHourOptions{
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
		elems = append(elems, models.OfficeElement{
			Type: models.Heading,
			Text: fmt.Sprintf("Commemoration of %s", comm.CommemorationName()),
		})

		// Antiphon: feast-specific or fallback
		antText, antSrc := lookupCommemoration(comm, day.Season, hourName, "commemoration-antiphon", corpus)
		elems = append(elems, models.OfficeElement{
			Type:      models.Antiphon,
			Text:      antText,
			SlotRef:   "commemoration-antiphon",
			SourceRef: antSrc,
		})

		// Versicle + Response
		versText, versSrc := lookupCommemoration(comm, day.Season, hourName, "commemoration-versicle", corpus)
		elems = append(elems, models.OfficeElement{
			Type:      models.Versicle,
			Text:      versText,
			SlotRef:   "commemoration-versicle",
			SourceRef: versSrc,
		})

		// Collect
		collectText, collectSrc := lookupCommemoration(comm, day.Season, hourName, "commemoration-collect", corpus)
		elems = append(elems, models.OfficeElement{
			Type:      models.Collect,
			Text:      collectText,
			SlotRef:   "commemoration-collect",
			SourceRef: collectSrc,
		})
	}
	return elems
}

// lookupFeriaCommemoration resolves the commemoration slots for the synthesized
// occurring feria, which has no proper of its own: Antiphon and versicle come
// "from the Psalter" (the ferial gospel-canticle antiphon and little versicle
// of the hour) and the collect from the governing Sunday carried on ProperID.
// Falls back to the generic ordinary slot for robustness when a preferred
// source is missing.
func lookupFeriaCommemoration(feast *models.Feast, hourName, ref string, corpus *texts.TextCorpus) (string, string) {
	switch ref {
	case "commemoration-antiphon":
		antRef := "ordinary/" + hourName + "/benedictus-antiphon"
		if hourName == "vespers" {
			antRef = "ordinary/vespers/magnificat-antiphon"
		}
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
	if feast.ID == models.FeriaCommemorationID || feast.Rank == models.PrivilegedFeria {
		return lookupFeriaCommemoration(feast, hourName, ref, corpus)
	}

	// Sundays, Ember days, and vigils are de Tempore: their commemoration takes
	// the day's own gospel-canticle antiphon and the hour's little versicle, never
	// the saint-shaped fallbacks (which would leave an unfilled "N.").
	if feast.Category == models.CategorySunday || feast.Category == models.CategoryFeria {
		return lookupTemporalCommemoration(feast, hourName, ref, corpus)
	}

	// 1. Feast-specific
	for _, feastID := range feastProperIDs(feast) {
		feastRef := "proper/" + feastID + "/" + ref
		if text := corpus.Get(feastRef); text != "" {
			return substituteProperName(text, feast.ProperName), feastRef
		}
	}

	// 1b. For commemoration-collect, fall back to the feast's own collect.
	if ref == "commemoration-collect" {
		for _, feastID := range feastProperIDs(feast) {
			collectRef := "proper/" + feastID + "/collect"
			if text := corpus.Get(collectRef); text != "" {
				return substituteProperName(text, feast.ProperName), collectRef
			}
		}
	}

	// 2. Commons (paschal, then regular)
	if text, resolved := lookupCommonsText(feast.Category, season, "", ref, corpus); text != "" {
		return substituteProperName(text, feast.ProperName), resolved
	}

	// 3. Ordinary fallback (hour-specific)
	ordinaryRef := "ordinary/" + hourName + "/" + ref
	if text := corpus.Get(ordinaryRef); text != "" {
		return substituteProperName(text, feast.ProperName), ordinaryRef
	}

	return fmt.Sprintf("[%s: %s]", ref, feast.ID), ref
}
