package office

import (
	"strings"
	"testing"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
)

func TestChristmasOctaveVespersCommemoration(t *testing.T) {
	// Diurnal p.197 and 2026 ordo pp.127-129: the octave commemoration
	// uses the feast's II-Vespers antiphon/verse, even when St Sylvester
	// owns I Vespers. It must not predict the Nativity as "tomorrow".
	engine, err := NewEngine("../../data")
	if err != nil {
		t.Fatal(err)
	}
	for _, year := range []int{2024, 2025, 2026, 2027, 2028} {
		days, err := calendar.BuildCalendar(year, "../../data")
		if err != nil {
			t.Fatal(err)
		}
		seen := 0
		for i := range days {
			day := &days[i]
			if date := day.Date.Format("01-02"); date < "12-26" || date > "12-30" {
				continue
			}
			for _, form := range models.PrayerForms {
				h, err := engine.ComposeHourWithOptions("vespers", day, calendar.ComputeMoveableDates(year), ComposeOptions{Form: form})
				if err != nil {
					t.Fatal(err)
				}
				counts := map[string]int{}
				for _, s := range h.Sections {
					for _, e := range s.Elements {
						if !strings.HasPrefix(e.CommemorationOwnerID, "christmas-octave-day-") {
							continue
						}
						counts[e.SlotRef]++
						switch e.SlotRef {
						case "commemoration-antiphon":
							seen++
							if !strings.HasPrefix(e.Text, "Today * the Christ is born") {
								t.Errorf("%s/%s antiphon = %q", day.Date.Format("2006-01-02"), form, e.Text)
							}
						case "commemoration-versicle":
							if !strings.HasPrefix(e.Text, "V. The Lord declared, alleluia.") {
								t.Errorf("%s/%s verse = %q", day.Date.Format("2006-01-02"), form, e.Text)
							}
						case "commemoration-collect":
							if !strings.HasPrefix(e.Text, "Grant, we beseech thee, Almighty God") {
								t.Errorf("unexpected octave collect: %q", e.Text)
							}
						}
					}
				}
				for _, slot := range []string{"commemoration-antiphon", "commemoration-versicle", "commemoration-collect"} {
					if counts[slot] > 1 {
						t.Errorf("%s/%s repeated octave %s", day.Date.Format("2006-01-02"), form, slot)
					}
				}
			}
		}
		if seen == 0 {
			t.Fatalf("%d supplied no octave commemoration to check", year)
		}
	}
}
