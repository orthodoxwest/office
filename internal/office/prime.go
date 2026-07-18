package office

import (
	"strings"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/texts"
)

// PrimeComposer composes the hour of Prime.
type PrimeComposer struct{}

// Compose builds a complete Prime hour for the given day.
func (p *PrimeComposer) Compose(day *models.CalendarDay, sections []HourSection, corpus *texts.TextCorpus, moveable *calendar.MoveableDates) (*models.OfficeHour, error) {
	hour := &models.OfficeHour{
		Date:   day.Date,
		Hour:   "Prime",
		Title:  "Prime",
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
			if elem.Type == "proper-antiphon" && elem.Ref == "psalm-antiphon-1" {
				elems = append(elems, resolvePrimePsalmAntiphon(day, corpus, moveable))
				continue
			}
			elems = append(elems, resolveHourElement(day, "prime", elem, corpus))
		}
		hour.Sections = append(hour.Sections, models.OfficeSection{
			Label:       section.Label,
			Collapsible: section.Collapsible,
			Elements:    elems,
		})
	}

	return hour, nil
}

// resolvePrimePsalmAntiphon follows Prime's antiphon rubric. Feasts and
// Sundays use the first antiphon from their Lauds proper or common. Ferias use
// the seasonal exceptions appointed for Prime, then the weekday psalter form.
func resolvePrimePsalmAntiphon(day *models.CalendarDay, corpus *texts.TextCorpus, moveable *calendar.MoveableDates) models.OfficeElement {
	const slot = "psalm-antiphon-1"
	if day == nil {
		return primePsalmAntiphonElement(slot, "", "")
	}

	// The Saturday Office has specially named festal Lauds slots in the
	// corpus; its first one is the antiphon used at Prime.
	if day.Celebration != nil && day.Celebration.ID == saturdayOfficeBVMID {
		key := "proper/saturday-office-bvm/saturday-psalm-antiphon-1"
		return primePsalmAntiphonElement(slot, key, corpus.Get(key))
	}

	ferial := day.Celebration == nil || day.Celebration.Category == models.CategoryFeria
	if !ferial {
		text, key := resolveProperText(day, "prime", slot, corpus)
		return primePsalmAntiphonElement(slot, key, text)
	}

	if moveable == nil {
		moveable = calendar.ComputeMoveableDates(day.Date.Year())
	}
	weekday := strings.ToLower(civilWeekday(day).String())
	weekdayKey := "ordinary/prime/" + slot + "-" + weekday
	key := weekdayKey

	switch day.Season {
	case models.Advent:
		// December 17-23 contains the six ferias before Christmas Eve
		// (the seventh day is Sunday). They use the first Lauds antiphon
		// of the occurrent feria; earlier ferias repeat the preceding
		// Sunday's first Lauds antiphon.
		if day.Date.Month() == 12 && day.Date.Day() >= 17 && day.Date.Day() <= 23 {
			key = "seasonal/advent/" + slot + "-prime-" + weekday
		} else if strings.HasPrefix(day.TemporalWeekID, "advent-sunday-") {
			key = "proper/" + day.TemporalWeekID + "/" + slot
		}
	case models.Lent:
		// Ash Wednesday through Saturday retain the weekday forms. The
		// Lenten antiphon begins on Monday after the first Sunday.
		if !day.Date.Before(moveable.Lent1.AddDate(0, 0, 1)) {
			key = "seasonal/lent/" + slot + "-prime"
		}
	case models.Passiontide:
		if !day.Date.Before(moveable.HolyMonday) {
			// The Holy Week ferias use the first antiphon from their own
			// Lauds propers.
			text, properKey := resolveProperText(day, "prime", slot, corpus)
			return primePsalmAntiphonElement(slot, properKey, text)
		}
		key = "seasonal/passiontide/" + slot + "-prime"
	case models.Easter:
		// The Paschal form runs only from Monday after Low Sunday through
		// the Vigil of the Ascension, not throughout the engine's broader
		// Easter season.
		if !day.Date.Before(moveable.LowSunday.AddDate(0, 0, 1)) && day.Date.Before(moveable.Ascension) {
			key = "seasonal/easter/" + slot
		}
	}

	return primePsalmAntiphonElement(slot, key, corpus.Get(key))
}

func primePsalmAntiphonElement(slot, key, text string) models.OfficeElement {
	if text == "" {
		text = "[Prime psalm antiphon not found: " + key + "]"
	}
	return sourcedElement(models.OfficeElement{
		Type:      models.Antiphon,
		Text:      text,
		SlotRef:   slot,
		SourceRef: key,
	}, key)
}
