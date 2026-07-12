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
	if day == nil {
		return day
	}

	officeDay := *day
	if day.Vespers.Owner == models.VespersNotApplicable || day.Vespers.Feast == nil {
		// No adjacent celebration owns Vespers. The office remains today's,
		// but its occurrence commemorations belong to tomorrow rather than
		// carrying today's Lauds commemorations one evening late (XIV.9).
		officeDay.Commemorations = day.Vespers.Commemorations
		return &officeDay
	}

	officeDay.Celebration = day.Vespers.Feast
	officeDay.Color = day.Vespers.Color
	if day.Vespers.Season != "" {
		officeDay.Season = day.Vespers.Season
	}

	switch day.Vespers.Owner {
	case models.VespersIOfFollowing:
		// Vespers belongs liturgically to tomorrow's feast; only the outgoing
		// office (today's celebration, if any) is commemorated (XIII.2-17).
		officeDay.Date = day.Date.Add(24 * time.Hour)
		officeDay.Commemorations = day.Vespers.Commemorations
		officeDay.Tempora = ""
		officeDay.WithinOctaveOf = ""
		officeDay.FirstVespers = true
	case models.VespersIIOfPreceding:
		// Vespers is of today's own feast; the incoming feast that lost the
		// concurrence is commemorated in addition to today's commemorations.
		if len(day.Vespers.Commemorations) > 0 {
			comms := make([]*models.Feast, 0, len(day.Commemorations)+len(day.Vespers.Commemorations))
			comms = append(comms, day.Commemorations...)
			comms = append(comms, day.Vespers.Commemorations...)
			officeDay.Commemorations = comms
		}
	}

	return &officeDay
}
