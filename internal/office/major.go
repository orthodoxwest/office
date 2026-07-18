package office

import (
	"fmt"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/texts"
)

type majorHourOptions struct {
	hourName  string
	title     string
	officeDay func(*models.CalendarDay) *models.CalendarDay
}

func composeMajorHour(
	day *models.CalendarDay,
	sections []HourSection,
	corpus *texts.TextCorpus,
	moveable *calendar.MoveableDates,
	opts majorHourOptions,
) (*models.OfficeHour, error) {
	if day == nil {
		return nil, fmt.Errorf("calendar day is nil")
	}

	officeDay := day
	if opts.officeDay != nil {
		officeDay = opts.officeDay(day)
	}
	if officeDay == nil {
		return nil, fmt.Errorf("major hour office day is nil")
	}

	hour := &models.OfficeHour{
		Date:   day.Date,
		Hour:   opts.title,
		Title:  opts.title,
		Season: officeDay.Season,
		Color:  officeDay.Color,
	}

	if officeDay.Celebration != nil {
		hour.Feast = officeDay.Celebration.Name
	}

	for _, section := range sections {
		if section.Condition != "" {
			included := evaluateHourSectionCondition(section, officeDay, moveable, corpus)
			recordConditionDecision(hour, section.Condition, included, section.Name)
			if !included {
				continue
			}
		}

		var elems []models.OfficeElement
		for _, elem := range section.Elements {
			switch elem.Type {
			case "commemorations":
				elems = append(elems, addCommemorations(officeDay, opts.hourName, corpus)...)
			case "proper-psalmody":
				psalmody, _, err := resolveVespersPsalmody(officeDay, corpus)
				if err != nil {
					return nil, err
				}
				elems = append(elems, composeResolvedPsalmody(officeDay, opts.hourName, psalmody, corpus)...)
			default:
				elems = append(elems, resolveHourElement(officeDay, opts.hourName, elem, corpus))
			}
		}

		hour.Sections = append(hour.Sections, models.OfficeSection{
			Label:       section.Label,
			Collapsible: section.Collapsible,
			Elements:    elems,
		})
	}

	return hour, nil
}

func recordConditionDecision(hour *models.OfficeHour, condition string, included bool, section string) {
	outcome := "omitted"
	if included {
		outcome = "included"
	}
	hour.Decisions = append(hour.Decisions, models.CompositionDecision{
		Rule:    "condition:" + condition,
		Outcome: outcome,
		Detail:  section,
	})
}
