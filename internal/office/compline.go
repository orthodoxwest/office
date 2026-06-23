package office

import (
	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/texts"
)

// ComplineComposer composes the hour of Compline.
type ComplineComposer struct {
	Moveable *calendar.MoveableDates
}

// SetMoveable sets the moveable feast dates for preces.
func (c *ComplineComposer) SetMoveable(m *calendar.MoveableDates) {
	c.Moveable = m
}

// Compose builds a complete Compline hour for the given day.
func (c *ComplineComposer) Compose(day *models.CalendarDay, sections []HourSection, corpus *texts.TextCorpus) (*models.OfficeHour, error) {
	hour := &models.OfficeHour{
		Date:   day.Date,
		Hour:   "Compline",
		Title:  "Compline",
		Season: day.Season,
		Color:  day.Color,
	}

	if day.Celebration != nil {
		hour.Feast = day.Celebration.Name
	}

	for _, section := range sections {
		// Evaluate section condition
		if section.Condition != "" && !evaluateCondition(section.Condition, day, c.Moveable) {
			continue
		}

		var elems []models.OfficeElement
		for _, elem := range section.Elements {
			if elem.Type == "marian" && elem.Ref == "seasonal" {
				ref := "ordinary/marian/" + day.MarianAntiphon
				oe := models.OfficeElement{
					Type:  models.Antiphon,
					Text:  corpus.Get(ref),
					Label: marianLabel(day.MarianAntiphon),
				}
				if oe.Text == "" {
					oe.Text = "[Text not found: " + ref + "]"
				}
				elems = append(elems, oe)
				continue
			}

			elems = append(elems, resolveHourElement(day, "compline", elem, corpus))
		}
		hour.Sections = append(hour.Sections, models.OfficeSection{
			Label:       section.Label,
			Collapsible: section.Collapsible,
			Elements:    elems,
		})
	}

	return hour, nil
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
