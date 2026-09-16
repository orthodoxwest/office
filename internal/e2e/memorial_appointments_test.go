package e2e

import (
	"testing"

	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/office"
)

// Diurnal VIII (pp. xxviii-xxix) and 2026 ordo pp. 67-68, 109-110,
// 127: the first-class feasts suppress their occurring Memorials at
// I Vespers and Lauds. The occurring Sunday at Christ the King survives.
func TestFirstClassMemorialAppointments(t *testing.T) {
	engine, err := office.NewEngine(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	yd := buildYear(t, 2026)
	for _, tc := range []struct {
		date, hour, comm string
		want             bool
	}{
		{"2026-05-30", "vespers", "comm-05-31-st-petronilla-virgin", false},
		{"2026-05-31", "lauds", "comm-05-31-st-petronilla-virgin", false},
		{"2026-10-24", "vespers", "comm-extra-10-25-ss-chrysanthus-and-daria-martyrs", false},
		{"2026-10-25", "lauds", "comm-extra-10-25-ss-chrysanthus-and-daria-martyrs", false},
		{"2026-12-24", "vespers", "comm-12-25-st-anastasia-virgin-and-martyr", false},
		{"2026-12-25", "lauds", "comm-12-25-st-anastasia-virgin-and-martyr", false},
		{"2026-10-24", "vespers", "pentecost-sunday-21", true},
		{"2026-10-25", "lauds", "pentecost-sunday-21", true},
		{"2026-10-25", "vespers", "pentecost-sunday-21", true},
		// Later privileged-octave weekdays retain Memorials (ordo p. 55).
		{"2026-04-17", "lauds", "comm-extra-04-17-st-anicetus-pope-and-martyr", true},
		// The ordo p. 96 retains Hadrian at D2 I Vespers. The source
		// conflict with VIII is held separately in #390.
		{"2026-09-07", "vespers", "comm-09-08-st-hadrian-martyr", true},
		// Preserve unresolved scopes (#378-380), without certifying them.
		{"2026-04-13", "lauds", "comm-04-13-st-hermengild-martyr", true},
		{"2026-04-14", "lauds", "comm-04-14-st-justin-martyr", true},
		{"2026-06-02", "lauds", "comm-extra-06-02-ss-marcellinus-peter-and-erasmus-martyrs", true},
		{"2026-04-29", "lauds", "st-george-octave-day-7", true},
		{"2026-06-11", "lauds", "st-barnabas", true},
		{"2026-06-11", "vespers", "st-barnabas", false},
	} {
		for _, form := range models.PrayerForms {
			t.Run(tc.date+"/"+tc.hour+"/"+tc.comm+"/"+string(form), func(t *testing.T) {
				hour, err := engine.ComposeHourWithOptions(tc.hour, dayFor(t, yd, tc.date), yd.moveable, office.ComposeOptions{Form: form})
				if err != nil {
					t.Fatal(err)
				}
				collects := 0
				for _, section := range hour.Sections {
					for _, elem := range section.Elements {
						if elem.CommemorationOwnerID != tc.comm {
							continue
						}
						if !tc.want {
							t.Errorf("suppressed commemoration still rendered: %+v", elem)
						}
						if elem.Type == models.Collect && elem.IsCommemoration {
							collects++
						}
					}
				}
				if tc.want && collects != 1 {
					t.Errorf("retained commemoration has %d collects, want 1", collects)
				}
			})
		}
	}
}
