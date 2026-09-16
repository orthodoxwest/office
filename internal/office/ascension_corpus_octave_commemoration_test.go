package office

import (
	"strings"
	"testing"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
)

func TestAscensionAndCorpusSundayOctaveCommemorations(t *testing.T) {
	// Diurnal pp.392, 419; 2026 ordo pp.66-67, 72.
	// Reusing festal psalmody does not replace the Sunday's octave prayer.
	engine, err := NewEngine("../../data")
	if err != nil {
		t.Fatal(err)
	}
	days, err := calendar.BuildCalendar(2026, "../../data")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		date, hour, owner, antiphon, verse, collect string
	}{
		{"05-24", "lauds", "ascension-octave-day-4", "I ascend", "V. The Lord hath prepared", "Grant, we beseech thee, Almighty God"},
		{"05-26", "lauds", "ascension-octave-day-6", "I ascend", "V. The Lord hath prepared", "Grant, we beseech thee, Almighty God"},
		{"05-27", "lauds", "ascension-octave-day-7", "I ascend", "V. The Lord hath prepared", "Grant, we beseech thee, Almighty God"},
		{"06-14", "lauds", "corpus-christi-octave-day-4", "I am * the living bread", "V. He maketh peace in thy borders", "O God, who in a wonderful Sacrament"},
	} {
		for _, form := range models.PrayerForms {
			t.Run(tc.date+"/"+tc.hour+"/"+string(form), func(t *testing.T) {
				var day *models.CalendarDay
				for i := range days {
					if days[i].Date.Format("01-02") == tc.date {
						day = &days[i]
						break
					}
				}
				hour, err := engine.ComposeHourWithOptions(tc.hour, day, calendar.ComputeMoveableDates(2026), ComposeOptions{Form: form})
				if err != nil {
					t.Fatal(err)
				}
				want := map[string]string{"commemoration-antiphon": tc.antiphon, "commemoration-versicle": tc.verse, "commemoration-collect": tc.collect}
				seen := map[string]int{}
				for _, section := range hour.Sections {
					for _, element := range section.Elements {
						if element.CommemorationOwnerID != tc.owner {
							continue
						}
						if prefix, ok := want[element.SlotRef]; ok {
							seen[element.SlotRef]++
							if !strings.HasPrefix(element.Text, prefix) {
								t.Errorf("%s = %q, want %q", element.SlotRef, element.Text, prefix)
							}
						}
					}
				}
				for slot := range want {
					if seen[slot] != 1 {
						t.Errorf("%s rendered %d times, want once", slot, seen[slot])
					}
				}
			})
		}
	}
}

func TestAscensionSundayEveningAntiphonHold(t *testing.T) {
	// #398: ordo p.66 appoints O King of glory, Diurnal p.393 Father.
	// Preserve the existing selection pending a ruling; this is a hold,
	// not an assertion that the psalm-antiphon fallback is correct.
	engine, err := NewEngine("../../data")
	if err != nil {
		t.Fatal(err)
	}
	feast := &models.Feast{ID: "ascension-octave-day-4", ProperID: "ascension", Category: models.CategoryLord}
	text, ref := lookupCommemoration(feast, models.Easter, "vespers", "commemoration-antiphon", engine.corpus)
	if !strings.HasPrefix(text, "Ye men of Galilee") || ref != "proper/ascension/commemoration-antiphon" {
		t.Fatalf("held selection changed to %q (%s)", text, ref)
	}
}
