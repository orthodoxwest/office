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
		if section.Condition != "" {
			included := evaluateCondition(section.Condition, day, m.Moveable)
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

// isWeekdayMatch checks whether the day matches a weekday-* condition.
func isWeekdayMatch(condition string, day *models.CalendarDay) bool {
	weekdayName := strings.TrimPrefix(condition, "weekday-")
	weekday := civilWeekday(day)
	switch weekdayName {
	case "sunday":
		return weekday == time.Sunday
	case "monday":
		return weekday == time.Monday
	case "tuesday":
		return weekday == time.Tuesday
	case "wednesday":
		return weekday == time.Wednesday
	case "thursday":
		return weekday == time.Thursday
	case "friday":
		return weekday == time.Friday
	case "saturday":
		return weekday == time.Saturday
	default:
		return false
	}
}

// civilWeekday returns the weekday used by the psalter and weekday ordinary.
// At I Vespers of a Sunday the office day has advanced to Sunday, while the
// local Saturday Vespers books retain the Saturday psalter and ordinary.
func civilWeekday(day *models.CalendarDay) time.Weekday {
	date := day.Date
	if isSundayFirstVespers(day) {
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
