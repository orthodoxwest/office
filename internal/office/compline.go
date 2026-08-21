package office

import (
	"strings"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/texts"
)

// ComplineComposer composes the hour of Compline.
type ComplineComposer struct{}

// Compose builds a complete Compline hour for the given day.
func (c *ComplineComposer) Compose(day *models.CalendarDay, sections []HourSection, corpus *texts.TextCorpus, moveable *calendar.MoveableDates) (*models.OfficeHour, error) {
	officeDay := complineOfficeDay(day)
	hour := &models.OfficeHour{
		Date:   day.Date,
		Hour:   "Compline",
		Title:  "Compline",
		Season: officeDay.Season,
		Color:  officeDay.Color,
	}

	if officeDay.Celebration != nil {
		hour.Feast = officeDay.Celebration.Name
	}

	for _, section := range sections {
		sectionDay := complineSectionDay(day, officeDay, section)
		if section.Condition != "" {
			included := evaluateHourSectionCondition(section, sectionDay, moveable, corpus)
			recordConditionDecision(hour, section.Condition, included, section.Name)
			if !included {
				continue
			}
		}

		var elems []models.OfficeElement
		for _, elem := range section.Elements {
			elems = appendHourElement(elems, sectionDay, "compline", elem, corpus)
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

// complineOfficeDay returns the office that owns the evening. When Vespers is
// I Vespers of the following celebration, that celebration also supplies
// Compline's feast, colour, season, proper hymn ending, and preces discipline.
// vespersOfficeDay deliberately retains the civil day's MarianAntiphon field,
// whose value already models the distinct Compline handoffs.
func complineOfficeDay(day *models.CalendarDay) *models.CalendarDay {
	return vespersOfficeDay(day)
}

// complineSectionDay preserves the exceptional Holy Saturday form of
// Compline. Its Nunc dimittis and omissions are keyed to the civil evening,
// even though Easter's I Vespers now supplies the surrounding office context.
func complineSectionDay(civilDay, officeDay *models.CalendarDay, section HourSection) *models.CalendarDay {
	if civilDay != nil && civilDay.Celebration != nil &&
		civilDay.Celebration.ID == "holy-saturday" &&
		strings.Contains(section.Condition, "feast-holy-saturday") {
		return civilDay
	}
	return officeDay
}

// marianLabel returns a display label for a Marian antiphon corpus key.
func marianLabel(key string) string {
	switch key {
	case "alma-redemptoris-advent", "alma-redemptoris-christmas":
		return "Alma Redemptoris Mater"
	case "ave-regina-caelorum":
		return "Ave Regina Caelorum"
	case "regina-caeli":
		return "Regina Caeli"
	case "salve-regina":
		return "Salve Regina"
	default:
		return ""
	}
}
