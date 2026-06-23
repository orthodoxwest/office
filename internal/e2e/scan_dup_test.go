package e2e

import (
	"testing"
	"time"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/office"
)

// TestScanDuplicateSectionLabels composes every hour for every day across several
// years and flags any composed hour where a non-empty section Label appears more
// than once — the symptom of the Psalmody-Sunday/Psalmody-Festal overlap bug
// (overlapping section Conditions causing the same content to render twice).
func TestScanDuplicateSectionLabels(t *testing.T) {
	eng, err := office.NewEngine(dataDir)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	hours := []string{"lauds", "prime", "terce", "sext", "none", "vespers", "compline"}

	for _, year := range []int{2024, 2025, 2026, 2027, 2028} {
		days, err := calendar.BuildCalendar(year, dataDir)
		if err != nil {
			t.Fatalf("BuildCalendar(%d): %v", year, err)
		}
		moveable := calendar.ComputeMoveableDates(year)

		for i := range days {
			day := &days[i]
			for _, hourName := range hours {
				hour, err := eng.ComposeHour(hourName, day, moveable)
				if err != nil {
					t.Fatalf("ComposeHour(%s, %s): %v", hourName, day.Date.Format(time.DateOnly), err)
				}
				seen := map[string]int{}
				for _, sec := range hour.Sections {
					if sec.Label == "" {
						continue
					}
					seen[sec.Label]++
				}
				for label, count := range seen {
					if count > 1 {
						t.Errorf("%s on %s (%s): section label %q appears %d times",
							hourName, day.Date.Format(time.DateOnly), feastID(day), label, count)
					}
				}
			}
		}
	}
}

func feastID(day *models.CalendarDay) string {
	if day.Celebration == nil {
		return "<none>"
	}
	return day.Celebration.ID
}
