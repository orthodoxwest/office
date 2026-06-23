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
	var elems []models.OfficeElement
	for _, comm := range day.Commemorations {
		elems = append(elems, models.OfficeElement{
			Type: models.Heading,
			Text: fmt.Sprintf("Commemoration of %s", comm.Name),
		})

		// Antiphon: feast-specific or fallback
		antText := lookupCommemoration(comm, day.Season, hourName, "commemoration-antiphon", corpus)
		elems = append(elems, models.OfficeElement{
			Type: models.Antiphon,
			Text: antText,
		})

		// Versicle + Response
		versText := lookupCommemoration(comm, day.Season, hourName, "commemoration-versicle", corpus)
		elems = append(elems, models.OfficeElement{
			Type: models.Versicle,
			Text: versText,
		})

		// Collect
		collectText := lookupCommemoration(comm, day.Season, hourName, "commemoration-collect", corpus)
		elems = append(elems, models.OfficeElement{
			Type: models.Collect,
			Text: collectText,
		})
	}
	return elems
}

// lookupCommemoration looks up a commemoration text, trying in order:
// feast-specific proper, commons (paschal then regular), ordinary fallback.
// Applies N. substitution using the feast's ProperName.
func lookupCommemoration(feast *models.Feast, season models.Season, hourName, ref string, corpus *texts.TextCorpus) string {
	// 1. Feast-specific
	for _, feastID := range feastProperIDs(feast) {
		feastRef := "proper/" + feastID + "/" + ref
		if text := corpus.Get(feastRef); text != "" {
			return substituteProperName(text, feast.ProperName)
		}
	}

	// 1b. For commemoration-collect, fall back to the feast's own collect.
	if ref == "commemoration-collect" {
		for _, feastID := range feastProperIDs(feast) {
			if text := corpus.Get("proper/" + feastID + "/collect"); text != "" {
				return substituteProperName(text, feast.ProperName)
			}
		}
	}

	// 2. Commons (paschal, then regular)
	if text, _ := lookupCommonsText(feast.Category, season, "", ref, corpus); text != "" {
		return substituteProperName(text, feast.ProperName)
	}

	// 3. Ordinary fallback (hour-specific)
	ordinaryRef := "ordinary/" + hourName + "/" + ref
	if text := corpus.Get(ordinaryRef); text != "" {
		return substituteProperName(text, feast.ProperName)
	}

	return fmt.Sprintf("[%s: %s]", ref, feast.ID)
}
