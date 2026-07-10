package office

import (
	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/texts"
)

// PrimeComposer composes the hour of Prime.
type PrimeComposer struct {
	Moveable *calendar.MoveableDates
}

// SetMoveable sets the moveable feast dates for preces calculation.
func (p *PrimeComposer) SetMoveable(m *calendar.MoveableDates) {
	p.Moveable = m
}

// Compose builds a complete Prime hour for the given day.
func (p *PrimeComposer) Compose(day *models.CalendarDay, sections []HourSection, corpus *texts.TextCorpus) (*models.OfficeHour, error) {
	hour := &models.OfficeHour{
		Date:   day.Date,
		Hour:   "Prime",
		Title:  "Prime",
		Season: day.Season,
		Color:  day.Color,
	}

	if day.Celebration != nil {
		hour.Feast = day.Celebration.Name
	}

	for _, section := range sections {
		if section.Condition != "" {
			included := evaluateCondition(section.Condition, day, p.Moveable)
			recordConditionDecision(hour, section.Condition, included, section.Name)
			if !included {
				continue
			}
		}

		var elems []models.OfficeElement
		for _, elem := range section.Elements {
			elems = append(elems, resolveHourElement(day, "prime", elem, corpus))
		}
		hour.Sections = append(hour.Sections, models.OfficeSection{
			Label:       section.Label,
			Collapsible: section.Collapsible,
			Elements:    elems,
		})
	}

	return hour, nil
}
