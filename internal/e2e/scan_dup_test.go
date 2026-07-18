package e2e

import (
	"strings"
	"testing"
	"time"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/office"
)

// TestVespers2026PsalmodySweep guards the annual adversarial-review sweep:
// every Vespers must compose, no service may grow a fifth psalm, and corpus
// annotation lines must never reach rendered elements.
func TestVespers2026PsalmodySweep(t *testing.T) {
	eng, err := office.NewEngine(dataDir)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	days, err := calendar.BuildCalendar(2026, dataDir)
	if err != nil {
		t.Fatalf("BuildCalendar: %v", err)
	}
	moveable := calendar.ComputeMoveableDates(2026)
	for i := range days {
		day := &days[i]
		hour, err := eng.ComposeHour("vespers", day, moveable)
		if err != nil {
			t.Fatalf("ComposeHour(vespers, %s): %v", day.Date.Format(time.DateOnly), err)
		}
		psalms := 0
		for _, section := range hour.Sections {
			for _, elem := range section.Elements {
				if elem.Type == models.Psalm {
					psalms++
				}
				for _, line := range strings.Split(elem.Text, "\n") {
					if strings.HasPrefix(strings.TrimSpace(line), "#") {
						t.Errorf("Vespers on %s leaked corpus comment %q", day.Date.Format(time.DateOnly), line)
					}
				}
			}
		}
		if psalms > 4 {
			t.Errorf("Vespers on %s (%s) has %d psalms", day.Date.Format(time.DateOnly), feastID(day), psalms)
		}
	}
}

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
