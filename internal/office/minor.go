package office

import (
	"strings"
	"time"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/texts"
)

// MinorHourComposer composes the minor hours (Terce, Sext, None).
// All three share identical logic; the hour name and psalm assignments
// come from the hour definition file.
type MinorHourComposer struct {
	Name string
}

// Compose builds a complete minor hour for the given day.
func (m *MinorHourComposer) Compose(day *models.CalendarDay, sections []HourSection, corpus *texts.TextCorpus, moveable *calendar.MoveableDates) (*models.OfficeHour, error) {
	hour := &models.OfficeHour{
		Date:   day.Date,
		Hour:   m.Name,
		Title:  m.Name,
		Season: day.Season,
		Color:  day.Color,
	}

	if day.Celebration != nil {
		hour.Feast = day.Celebration.Name
	}

	for _, section := range sections {
		if section.Condition != "" {
			included := evaluateHourSectionCondition(section, day, moveable, corpus)
			recordConditionDecision(hour, section.Condition, included, section.Name)
			if !included {
				continue
			}
		}

		var elems []models.OfficeElement
		for _, elem := range section.Elements {
			elems = append(elems, resolveHourElement(day, strings.ToLower(m.Name), elem, corpus))
		}
		hour.Sections = append(hour.Sections, models.OfficeSection{
			Label:       section.Label,
			Collapsible: section.Collapsible,
			Elements:    elems,
		})
	}

	return hour, nil
}

// civilWeekday returns the weekday used by the psalter and weekday ordinary.
// At I Vespers the office day has advanced to the following feast, while the
// psalter and weekday ordinary retain the civil evening on which Vespers is
// recited. Most feasts use proper psalmody, but the same rule matters when a
// proper explicitly appoints ferial psalmody.
func civilWeekday(day *models.CalendarDay) time.Weekday {
	date := day.Date
	if day != nil && day.FirstVespers {
		date = date.AddDate(0, 0, -1)
	}
	return date.Weekday()
}

// isSundayFirstVespers uses the liturgical office date rather than the feast
// category: Low Sunday is categorized as a feast of the Lord, but its first
// Vespers is nevertheless recited on Saturday evening.
func isSundayFirstVespers(day *models.CalendarDay) bool {
	return day != nil && day.FirstVespers && day.Date.Weekday() == time.Sunday
}
