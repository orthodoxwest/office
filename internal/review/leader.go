package review

import (
	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/office"
)

// composeReviewForms visits every supported form and retains one witness for
// each distinct composition. Identical deacon/priest forms do not inflate
// review counts, and an omitted greeting (e.g. Triduum) creates no extra unit.
func composeReviewForms(engine *office.Engine, name string, day *models.CalendarDay, moveable *calendar.MoveableDates) ([]*models.OfficeHour, error) {
	var forms []*models.OfficeHour
	seen := map[string]bool{}
	for _, form := range models.PrayerForms {
		hour, err := engine.ComposeHourWithOptions(name, day, moveable, office.ComposeOptions{Form: form})
		if err != nil {
			return nil, err
		}
		hash := HashHour(hour)
		if !seen[hash] {
			forms = append(forms, hour)
			seen[hash] = true
		}
	}
	return forms, nil
}

func requestedPrayerForm(forms []models.PrayerForm) models.PrayerForm {
	if len(forms) != 0 {
		return forms[0]
	}
	return models.PrayerPrivate
}
