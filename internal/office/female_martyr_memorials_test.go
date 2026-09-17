package office

import (
	"strings"
	"testing"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
)

func TestFemaleMartyrMemorialAntiphonsAndVersicles(t *testing.T) {
	// 2026 Ordo pp.93,118 explicitly appoints the Holy Women
	// commemoration forms from Diurnal p.5*, even though the saints
	// remain in the martyr text category for the unresolved collects.
	engine, err := NewEngine("../../data")
	if err != nil {
		t.Fatal(err)
	}
	days, err := calendar.BuildCalendar(2026, "../../data")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ date, hour, id, ant, verse string }{
		{"08-28", "vespers", "comm-08-29-st-sabina-martyr", "The kingdom of heaven", "V. In thy grace and in thy beauty"},
		{"08-29", "lauds", "comm-08-29-st-sabina-martyr", "Give her", "V. Full of grace are thy lips"},
		{"11-22", "vespers", "comm-11-23-st-felicitas-martyr", "The kingdom of heaven", "V. In thy grace and in thy beauty"},
		{"11-23", "lauds", "comm-11-23-st-felicitas-martyr", "Give her", "V. Full of grace are thy lips"},
	} {
		for _, form := range models.PrayerForms {
			t.Run(tc.date+"/"+tc.hour+"/"+string(form), func(t *testing.T) {
				seen := 0
				for i := range days {
					if days[i].Date.Format("01-02") != tc.date {
						continue
					}
					h, err := engine.ComposeHourWithOptions(tc.hour, &days[i], calendar.ComputeMoveableDates(2026), ComposeOptions{Form: form})
					if err != nil {
						t.Fatal(err)
					}
					for _, s := range h.Sections {
						for _, e := range s.Elements {
							if e.CommemorationOwnerID != tc.id {
								continue
							}
							want := map[string]string{"commemoration-antiphon": tc.ant, "commemoration-versicle": tc.verse}[e.SlotRef]
							if want == "" {
								continue
							}
							seen++
							if !strings.HasPrefix(e.Text, want) {
								t.Errorf("%s = %q, want %q", e.SlotRef, e.Text, want)
							}
						}
					}
				}
				if seen != 2 {
					t.Fatalf("found %d appointed antiphon/versicle slots, want 2", seen)
				}
			})
		}
	}
}
