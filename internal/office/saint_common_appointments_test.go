package office

import (
	"strings"
	"testing"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
)

func TestMargaretAndMaximusAppointments(t *testing.T) {
	// Current Ordo pp.70,89: Margaret uses the Holy Women Common;
	// Maximus takes the alternate Confessor collect (Diurnal p.45*).
	engine, err := NewEngine("../../data")
	if err != nil {
		t.Fatal(err)
	}
	days, err := calendar.BuildCalendar(2026, "../../data")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		date, hour, comm, slot, prefix string
	}{
		{"06-09", "vespers", "comm-06-10-st-margaret-of-scotland-queen-and-widow", "commemoration-antiphon", "The kingdom of heaven"},
		{"06-10", "lauds", "comm-06-10-st-margaret-of-scotland-queen-and-widow", "commemoration-antiphon", "Give her"},
		{"08-13", "lauds", "", "collect", "Attend, O Lord, upon our supplications"},
		{"08-13", "terce", "", "collect", "Attend, O Lord, upon our supplications"},
		{"08-13", "sext", "", "collect", "Attend, O Lord, upon our supplications"},
		{"08-13", "none", "", "collect", "Attend, O Lord, upon our supplications"},
		{"08-13", "vespers", "", "collect", "Attend, O Lord, upon our supplications"},
		{"08-13", "vespers", "comm-08-14-st-eusebius-priest-and-confessor", "commemoration-collect", "O God, who makest us glad"},
	} {
		for _, form := range models.PrayerForms {
			t.Run(tc.date+"/"+tc.hour+"/"+tc.comm+"/"+string(form), func(t *testing.T) {
				found := false
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
							if e.SlotRef != tc.slot || e.CommemorationOwnerID != tc.comm {
								continue
							}
							found = true
							if !strings.HasPrefix(e.Text, tc.prefix) {
								t.Errorf("%s = %q, want prefix %q", tc.slot, e.Text, tc.prefix)
							}
							if tc.comm == "" && !strings.Contains(e.Text, "blessed Maximus") {
								t.Errorf("collect lacks the saint's name: %q", e.Text)
							}
						}
					}
				}
				if !found {
					t.Fatal("appointed element not found")
				}
			})
		}
	}
}

func TestPaschalHolyWomanCommemoration(t *testing.T) {
	// Diurnal p.5* supplies Paschal Alleluias for both the antiphon and
	// versicle. Dedicated ordinary commemoration slots must not hide them.
	engine, err := NewEngine("../../data")
	if err != nil {
		t.Fatal(err)
	}
	feast := &models.Feast{ID: "example", Category: models.CategoryHolyWoman, Rank: models.Commemoration}
	for _, hour := range []string{"lauds", "vespers"} {
		for _, slot := range []string{"commemoration-antiphon", "commemoration-versicle"} {
			text, source := lookupCommemoration(feast, models.Easter, hour, slot, engine.corpus)
			if !strings.Contains(text, "alleluia") || !strings.HasPrefix(source, "commons/holy-woman-paschal/") {
				t.Errorf("%s %s lost its Paschal form: %q (%s)", hour, slot, text, source)
			}
		}
	}
}
