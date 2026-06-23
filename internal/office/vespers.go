package office

import (
	"time"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/texts"
)

// VespersComposer composes the hour of Vespers.
type VespersComposer struct {
	Moveable *calendar.MoveableDates
}

// SetMoveable sets the moveable feast dates for preces calculation.
func (v *VespersComposer) SetMoveable(m *calendar.MoveableDates) {
	v.Moveable = m
}

// Compose builds a complete Vespers hour for the given day.
func (v *VespersComposer) Compose(day *models.CalendarDay, sections []HourSection, corpus *texts.TextCorpus) (*models.OfficeHour, error) {
	return composeMajorHour(day, sections, corpus, v.Moveable, majorHourOptions{
		hourName:  "vespers",
		title:     "Vespers",
		officeDay: vespersOfficeDay,
	})
}

func vespersOfficeDay(day *models.CalendarDay) *models.CalendarDay {
	if day == nil || day.Vespers.Owner == models.VespersNotApplicable || day.Vespers.Feast == nil {
		return day
	}

	officeDay := *day
	officeDay.Celebration = day.Vespers.Feast
	officeDay.Color = day.Vespers.Color
	if day.Vespers.Season != "" {
		officeDay.Season = day.Vespers.Season
	}

	if day.Vespers.Owner == models.VespersIOfFollowing {
		officeDay.Date = day.Date.Add(24 * time.Hour)
		officeDay.Commemorations = nil
		officeDay.Tempora = ""
		officeDay.WithinOctaveOf = ""
	}

	return &officeDay
}
