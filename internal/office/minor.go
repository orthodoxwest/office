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
	Name     string
	Moveable *calendar.MoveableDates
}

// SetMoveable sets the moveable feast dates for preces calculation.
func (m *MinorHourComposer) SetMoveable(mv *calendar.MoveableDates) {
	m.Moveable = mv
}

// Compose builds a complete minor hour for the given day.
func (m *MinorHourComposer) Compose(day *models.CalendarDay, sections []HourSection, corpus *texts.TextCorpus) (*models.OfficeHour, error) {
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
		if section.Condition != "" && !evaluateCondition(section.Condition, day, m.Moveable) {
			continue
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

// isWeekdayMatch checks whether the day matches a weekday-* condition.
func isWeekdayMatch(condition string, day *models.CalendarDay) bool {
	weekdayName := strings.TrimPrefix(condition, "weekday-")
	switch weekdayName {
	case "sunday":
		return day.Date.Weekday() == time.Sunday
	case "monday":
		return day.Date.Weekday() == time.Monday
	case "tuesday":
		return day.Date.Weekday() == time.Tuesday
	case "wednesday":
		return day.Date.Weekday() == time.Wednesday
	case "thursday":
		return day.Date.Weekday() == time.Thursday
	case "friday":
		return day.Date.Weekday() == time.Friday
	case "saturday":
		return day.Date.Weekday() == time.Saturday
	default:
		return false
	}
}
