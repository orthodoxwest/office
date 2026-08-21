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
	// psalmodyDay supplies the psalms and their antiphons when they belong to a
	// different office than the rest of the hour — a Vespers split at the
	// Chapter. Defaults to officeDay.
	psalmodyDay func(*models.CalendarDay) *models.CalendarDay
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

	psalmodyDay := officeDay
	if opts.psalmodyDay != nil {
		if pd := opts.psalmodyDay(day); pd != nil {
			psalmodyDay = pd
		}
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

	// The sections that are said must be known up front: whether a
	// commemoration's collect is the last of the hour's run depends on whether
	// a Suffrage or Commemoration of the Cross follows it (XXXIII.5), and those
	// sections are conditional.
	included := make([]bool, len(sections))
	for i, section := range sections {
		included[i] = section.Condition == "" ||
			evaluateHourSectionCondition(section, officeDay, moveable, corpus)
	}
	moreCollects := collectRunContinues(sections, included)

	for i, section := range sections {
		if section.Condition != "" {
			recordConditionDecision(hour, section.Condition, included[i], section.Name)
			if !included[i] {
				continue
			}
		}

		var elems []models.OfficeElement
		for _, elem := range section.Elements {
			switch elem.Type {
			case "commemorations":
				elems = append(elems, addCommemorations(officeDay, opts.hourName, corpus, moreCollects[i])...)
			case "proper-psalmody":
				psalmody, _, err := resolveVespersPsalmody(psalmodyDay, corpus)
				if err != nil {
					return nil, err
				}
				elems = append(elems, composeResolvedPsalmody(psalmodyDay, opts.hourName, psalmody, corpus)...)
			default:
				elems = appendHourElement(elems, officeDay, opts.hourName, elem, corpus)
			}
		}

		// A section whose every element the corpus omits carries no office; its
		// label would otherwise render as an empty heading.
		if len(elems) == 0 {
			continue
		}

		hour.Sections = append(hour.Sections, models.OfficeSection{
			Label:       section.Label,
			Collapsible: section.Collapsible,
			Elements:    elems,
		})
	}

	return hour, nil
}

// collectRunContinues reports, for each section, whether a further collect of
// the hour's run is said after it.
//
// XXXIII.3 marks the end of the run with "The Lord be with you", which is the
// blessing element; XXXIII.5 concludes only the first and the last collect of
// the run. The Monastic Diurnal prints the boundary explicitly at the ordinary
// of Saturday Vespers (p. 144): the collect of the day, "then the
// Commemorations, if any occur, are made as required by the rubrics, and the
// Suffrage of All Saints, or, in Paschaltide, the Commemoration of the Cross,
// is said as set forth below. After the last Collect is said: V. The Lord be
// with you." So the Suffrage and the Cross are members of the run, not separate
// devotions, and a commemoration collect they follow is a middle collect.
func collectRunContinues(sections []HourSection, included []bool) []bool {
	end := len(sections)
	for i, section := range sections {
		if included[i] && sectionHasElementType(section, "blessing") {
			end = i
			break
		}
	}

	res := make([]bool, len(sections))
	later := false
	for i := end - 1; i >= 0; i-- {
		res[i] = later
		if included[i] && (sectionHasElementType(sections[i], "collect") ||
			sectionHasElementType(sections[i], "proper-collect")) {
			later = true
		}
	}
	return res
}

func sectionHasElementType(section HourSection, elemType string) bool {
	for _, elem := range section.Elements {
		if elem.Type == elemType {
			return true
		}
	}
	return false
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
