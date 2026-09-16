package e2e

import (
	"strings"
	"testing"

	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/office"
)

// The 2026 ordo, p. 56, appoints the supplement collect for Tikhon's
// transferred feast at I/II Vespers, Lauds and the little Hours. The named
// April 7 supplement supplies "O Lord God of the nations". Prime and
// Compline retain their ordinary collects; the neighboring feria/Sunday
// must not acquire the saint's collect.
func TestTikhonSupplementCollectAppointments(t *testing.T) {
	engine, err := office.NewEngine(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	yd := buildYear(t, 2026)
	const proper = "proper/st-tikhon-moscow/collect"
	for _, tc := range []struct {
		date, hour string
		want       bool
	}{
		{"2026-04-19", "vespers", true},
		{"2026-04-20", "lauds", true},
		{"2026-04-20", "terce", true},
		{"2026-04-20", "sext", true},
		{"2026-04-20", "none", true},
		{"2026-04-20", "vespers", true},
		{"2026-04-19", "lauds", false},
		{"2026-04-20", "prime", false},
		{"2026-04-20", "compline", false},
		{"2026-04-21", "lauds", false},
	} {
		for _, form := range models.PrayerForms {
			t.Run(tc.date+"/"+tc.hour+"/"+string(form), func(t *testing.T) {
				hour, err := engine.ComposeHourWithOptions(tc.hour, dayFor(t, yd, tc.date), yd.moveable, office.ComposeOptions{Form: form})
				if err != nil {
					t.Fatal(err)
				}
				if tc.want {
					assertPrincipalSource(t, hour, "collect", proper)
				}
				count := 0
				for _, section := range hour.Sections {
					for _, elem := range section.Elements {
						if elem.SourceRef != proper {
							continue
						}
						count++
						if elem.Type != models.Collect || elem.IsCommemoration ||
							!strings.HasPrefix(elem.Text, "O Lord God of the nations,") ||
							!strings.Contains(elem.Text, "blessed Saint Tikhon, thy Bishop and Confessor") ||
							strings.Contains(elem.Text, "Through.") {
							t.Errorf("unexpected supplement collect: %+v", elem)
						}
					}
				}
				want := 0
				if tc.want {
					want = 1
				}
				if count != want {
					t.Errorf("found %d Tikhon collects, want %d", count, want)
				}
			})
		}
	}
}
