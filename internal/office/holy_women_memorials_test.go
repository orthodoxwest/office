package office

import (
	"strings"
	"testing"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
)

func TestHolyWomenMemorialAppointments(t *testing.T) {
	// 2026 Ordo pp.60,90,113; Diurnal pp.5*,53*,525,591.
	engine, err := NewEngine("../../data")
	if err != nil {
		t.Fatal(err)
	}
	days, err := calendar.BuildCalendar(2026, "../../data")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ date, hour, id, ant, verse, collect string }{
		{"05-04", "lauds", "comm-extra-05-04-st-monica-widow", "Give her", "V. Full of grace", "O God, the comforter"},
		{"08-17", "vespers", "comm-08-18-st-helen-empress", "The kingdom of heaven", "V. In thy grace", "O Lord Jesus Christ"},
		{"08-18", "lauds", "comm-08-18-st-helen-empress", "Give her", "V. Full of grace", "O Lord Jesus Christ"},
		{"11-04", "vespers", "comm-11-05-st-elizabeth-mother-of-st-john-baptist", "The kingdom of heaven", "V. In thy grace", "Hear us, O God our Saviour"},
		{"11-05", "lauds", "comm-11-05-st-elizabeth-mother-of-st-john-baptist", "Give her", "V. Full of grace", "Hear us, O God our Saviour"},
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
							want := map[string]string{"commemoration-antiphon": tc.ant, "commemoration-versicle": tc.verse, "commemoration-collect": tc.collect}[e.SlotRef]
							if want == "" {
								continue
							}
							seen++
							if !strings.HasPrefix(e.Text, want) {
								t.Errorf("%s = %q, want %q", e.SlotRef, e.Text, want)
							}
							if tc.date == "05-04" && e.Type != models.Collect && !strings.Contains(e.Text, "alleluia") {
								t.Errorf("Monica's Paschal %s lost its Alleluia: %q", e.Type, e.Text)
							}
							if tc.date == "11-05" && e.Type == models.Collect && !strings.Contains(e.Text, "blessed Elizabeth,") {
								t.Errorf("Elizabeth's name not substituted: %q", e.Text)
							}
						}
					}
				}
				if seen != 3 {
					t.Fatalf("found %d commemoration slots, want 3", seen)
				}
			})
		}
	}
	if ref, ok := conclusionRefFor("proper/st-helen/collect", engine.corpus); !ok || ref != "shared/formulas/collect-conclusion-who-livest" {
		t.Errorf("Helen's collect must conclude in the second person: %q, %v", ref, ok)
	}
}
