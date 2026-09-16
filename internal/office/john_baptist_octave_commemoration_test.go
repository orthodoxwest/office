package office

import (
	"strings"
	"testing"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
)

func TestJohnBaptistOctaveCommemorationAntiphons(t *testing.T) {
	// Diurnal pp.541-542, 2026 ordo pp.75-76. The feast's I-Vespers
	// psalm antiphon is distinct from either gospel-canticle antiphon.
	engine, err := NewEngine("../../data")
	if err != nil {
		t.Fatal(err)
	}
	if got := engine.corpus.Get("proper/nativity-john-baptist/psalm-antiphon-1-first-vespers"); !strings.HasPrefix(got, "He shall go before him") {
		t.Fatalf("I-Vespers psalm antiphon changed: %q", got)
	}
	days, err := calendar.BuildCalendar(2026, "../../data")
	if err != nil {
		t.Fatal(err)
	}
	for _, form := range models.PrayerForms {
		for _, hour := range []string{"lauds", "vespers"} {
			seen := 0
			for i := range days {
				day := &days[i]
				if date := day.Date.Format("01-02"); date < "06-25" || date > "06-30" {
					continue
				}
				h, err := engine.ComposeHourWithOptions(hour, day, calendar.ComputeMoveableDates(2026), ComposeOptions{Form: form})
				if err != nil {
					t.Fatal(err)
				}
				for _, s := range h.Sections {
					for _, e := range s.Elements {
						if e.SlotRef != "commemoration-antiphon" || !strings.HasPrefix(e.CommemorationOwnerID, "nativity-john-baptist-octave-day-") {
							continue
						}
						seen++
						want := "The mouth of Zacharias"
						if hour == "vespers" {
							want = "The child * that is born unto us"
						}
						if !strings.HasPrefix(e.Text, want) {
							t.Errorf("%s/%s/%s antiphon = %q, want %q", day.Date.Format("2006-01-02"), hour, form, e.Text, want)
						}
					}
				}
			}
			if seen == 0 {
				t.Fatalf("no John Baptist octave commemoration at %s/%s", hour, form)
			}
		}
	}
}
