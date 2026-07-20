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
			if elem.Type == "proper-versicle" && elem.Ref == "versicle" {
				elems = append(elems, resolveMinorHourVersicle(day, strings.ToLower(m.Name), corpus))
				continue
			}
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

// resolveMinorHourVersicle follows the Monastic Diurnal's Little Hours
// structure: the chapter is followed by a simple versicle, not a short
// responsory. The seeded feast/common corpus records the same two texts in
// Responsory Breve form, so non-ferial selections are reduced to their opening
// response and verse. Ordinary Sunday/weekday forms are stored directly as
// versicles because their texts differ from the seeded ordinary responsories.
func resolveMinorHourVersicle(day *models.CalendarDay, hourName string, corpus *texts.TextCorpus) models.OfficeElement {
	responsory, responsoryRef := resolveProperText(day, hourName, "short-responsory", corpus)
	ordinaryResponsory := "ordinary/" + hourName + "/short-responsory"
	if responsoryRef == ordinaryResponsory {
		text, ref := resolveProperText(day, hourName, "versicle", corpus)
		text = decorateMinorHourVersicle(day, text)
		return sourcedElement(models.OfficeElement{
			Type:      models.Versicle,
			Text:      text,
			SlotRef:   "versicle",
			SourceRef: ref,
		}, ref)
	}

	text, ok := shortResponsoryVersicle(responsory)
	if !ok {
		text = "[Little Hours versicle not found: " + responsoryRef + "]"
	} else {
		text = decorateMinorHourVersicle(day, text)
	}
	return sourcedElement(models.OfficeElement{
		Type:      models.Versicle,
		Text:      text,
		SlotRef:   "versicle",
		SourceRef: responsoryRef,
	}, responsoryRef)
}

func decorateMinorHourVersicle(day *models.CalendarDay, text string) string {
	if day == nil || day.Season != models.Easter {
		return text
	}
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "V. ") && !strings.HasPrefix(trimmed, "R. ") {
			continue
		}
		prefix := trimmed[:3]
		body := strings.TrimSpace(trimmed[3:])
		body = stripTrailingAlleluias(body)
		lines[i] = prefix + body + ", alleluia."
	}
	return strings.Join(lines, "\n")
}

func stripTrailingAlleluias(text string) string {
	text = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(text), "."))
	for {
		lower := strings.ToLower(text)
		switch {
		case strings.HasSuffix(lower, ", alleluia"):
			text = strings.TrimSpace(text[:len(text)-len(", alleluia")])
		case strings.HasSuffix(lower, " alleluia"):
			text = strings.TrimSpace(text[:len(text)-len(" alleluia")])
		default:
			return strings.TrimRight(text, " ,;:")
		}
	}
}

func shortResponsoryVersicle(responsory string) (string, bool) {
	var versicle, response string
	for _, line := range strings.Split(responsory, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case versicle == "" && strings.HasPrefix(line, "R. "):
			versicle = strings.TrimSpace(strings.ReplaceAll(strings.TrimPrefix(line, "R. "), "*", ""))
		case versicle != "" && strings.HasPrefix(line, "V. "):
			response = strings.TrimSpace(strings.TrimPrefix(line, "V. "))
		}
		if versicle != "" && response != "" {
			break
		}
	}
	if versicle == "" || response == "" {
		return "", false
	}
	return "V. " + strings.Join(strings.Fields(versicle), " ") +
		"\nR. " + response, true
}

// civilWeekday returns the weekday used by the psalter and weekday ordinary.
// At I Vespers the office day has advanced to the following feast, while the
// psalter and weekday ordinary retain the civil evening on which Vespers is
// recited. Most feasts use proper psalmody, but the same rule matters when a
// proper explicitly appoints ferial psalmody.
func civilWeekday(day *models.CalendarDay) time.Weekday {
	date := day.Date
	if day.FirstVespers {
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
