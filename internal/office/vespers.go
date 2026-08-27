package office

import (
	"time"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/texts"
)

// VespersComposer composes the hour of Vespers.
type VespersComposer struct{}

// Compose builds a complete Vespers hour for the given day.
func (v *VespersComposer) Compose(day *models.CalendarDay, sections []HourSection, corpus *texts.TextCorpus, moveable *calendar.MoveableDates) (*models.OfficeHour, error) {
	if day != nil {
		if _, _, err := resolveVespersPsalmody(vespersPsalmodyDay(day), corpus); err != nil {
			return nil, err
		}
	}
	hour, err := composeMajorHour(day, sections, corpus, moveable, majorHourOptions{
		hourName:    "vespers",
		title:       "Vespers",
		officeDay:   vespersOfficeDay,
		psalmodyDay: vespersPsalmodyDay,
	})
	if err != nil {
		return nil, err
	}
	if day != nil && day.Vespers.AppendedOfficeOfTheDead {
		if err := appendVespersOfTheDead(hour, day, sections, corpus, moveable); err != nil {
			return nil, err
		}
	}
	return hour, nil
}

func appendVespersOfTheDead(hour *models.OfficeHour, day *models.CalendarDay, sections []HourSection, corpus *texts.TextCorpus, moveable *calendar.MoveableDates) error {
	deadHour, err := composeMajorHour(deadOfficeDay(day), sections, corpus, moveable, majorHourOptions{
		hourName:    "vespers",
		title:       "Vespers",
		officeDay:   vespersOfficeDay,
		psalmodyDay: vespersPsalmodyDay,
	})
	if err != nil {
		return err
	}
	rubricRef := "shared/formulas/appended-vespers-of-the-dead-rubric"
	if day.Celebration == nil || day.Celebration.ID != "all-saints" {
		rubricRef = "shared/formulas/appended-vespers-of-the-dead-rubric-optional"
	}
	rubric := resolveElement(HourElement{Type: "rubric", Ref: rubricRef}, corpus)
	hour.Sections = append(hour.Sections, models.OfficeSection{
		Label:    vespersOfTheDeadLabel,
		Elements: []models.OfficeElement{rubric},
	})
	hour.Sections = append(hour.Sections, deadHour.Sections...)
	hour.Decisions = append(hour.Decisions, deadHour.Decisions...)
	hour.Decisions = append(hour.Decisions, models.CompositionDecision{
		Rule:    "vespers:appended-office-of-the-dead",
		Outcome: "included",
		Detail:  "all-souls",
	})
	return nil
}

// vespersPsalmodyDay returns the office day that supplies the psalms and their
// antiphons. It differs from vespersOfficeDay only when the incoming office
// begins at the Chapter (General Rubrics III): the psalmody then stays with the
// outgoing office, so resolution uses today's celebration and civil date —
// which is what yields the weekday ferial psalms the ordo prints on those
// evenings ("Fri. Ps." on the eve of a Simple octave day).
func vespersPsalmodyDay(day *models.CalendarDay) *models.CalendarDay {
	if day == nil {
		return day
	}
	if !day.Vespers.PsalmodyFromPreceding {
		return vespersOfficeDay(day)
	}
	psalmodyDay := *day
	psalmodyDay.Commemorations = nil
	return &psalmodyDay
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
		officeDay.WithinOctaveOf = day.Vespers.WithinOctaveOf
		officeDay.FirstVespers = true
	case models.VespersIIOfPreceding:
		// Calendar resolution has already combined and filtered today's
		// occurrence commemorations with the incoming concurrence boundary.
		officeDay.Commemorations = day.Vespers.Commemorations
		officeDay.FollowingOfficeCommemorationID = day.Vespers.FollowingOfficeCommemorationID
	}

	return &officeDay
}
