package e2e

import (
	"slices"
	"strings"
	"testing"

	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/office"
)

// 2026 ordo pp. 29, 70, 86 appoints the Abbot collect for Anthony, the
// printed proper for Columba, and Stephen's collect with "finding" in place
// of "birthday". These appointments include evening commemorations.
func TestSourceCollectAppointments(t *testing.T) {
	engine, err := office.NewEngine(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	yd := buildYear(t, 2026)
	for _, feast := range []struct {
		id, eve, date, following, phrase, conclusion string
		eveComm, eveningComm                         bool
	}{
		{"st-anthony-egypt", "2026-01-16", "2026-01-17", "2026-01-18", "the intercession of blessed Anthony thine Abbot", "through", false, true},
		{"st-columba-iona", "2026-06-08", "2026-06-09", "2026-06-10", "where thy holy Abbot Columba shineth like a star before thee", "through", false, false},
		{"finding-st-stephen", "2026-08-02", "2026-08-03", "2026-08-04", "we celebrate the finding of him", "who-liveth", true, false},
	} {
		proper := "proper/" + feast.id + "/collect"
		for _, tc := range []struct {
			date, hour string
			want, comm bool
		}{
			{feast.eve, "vespers", true, feast.eveComm},
			{feast.date, "lauds", true, false},
			{feast.date, "terce", true, false},
			{feast.date, "sext", true, false},
			{feast.date, "none", true, false},
			{feast.date, "vespers", true, feast.eveningComm},
			{feast.eve, "lauds", false, false},
			{feast.date, "prime", false, false},
			{feast.date, "compline", false, false},
			{feast.following, "lauds", false, false},
		} {
			for _, form := range models.PrayerForms {
				t.Run(feast.id+"/"+tc.date+"/"+tc.hour+"/"+string(form), func(t *testing.T) {
					hour, err := engine.ComposeHourWithOptions(tc.hour, dayFor(t, yd, tc.date), yd.moveable, office.ComposeOptions{Form: form})
					if err != nil {
						t.Fatal(err)
					}
					count := 0
					for _, section := range hour.Sections {
						for _, elem := range section.Elements {
							if elem.SourceRef != proper {
								continue
							}
							count++
							if elem.Type != models.Collect || elem.IsCommemoration != tc.comm ||
								(tc.comm && elem.CommemorationOwnerID != feast.id) ||
								!strings.Contains(elem.Text, feast.phrase) || strings.Contains(elem.Text, "N.") {
								t.Errorf("incorrect appointed collect: %+v", elem)
							}
							// Anthony is followed by Prisca in the shared commemoration
							// collect chain, so his body has no separate conclusion.
							if (!tc.comm || feast.conclusion == "who-liveth") &&
								!slices.Contains(elem.SourceRefs, "shared/formulas/collect-conclusion-"+feast.conclusion) {
								t.Errorf("incorrect collect conclusion: %+v", elem)
							}
						}
					}
					want := 0
					if tc.want {
						want = 1
					}
					if count != want {
						t.Errorf("found %d appointed collects, want %d", count, want)
					}
				})
			}
		}
	}
	// The August adaptation must not change the December feast's collect.
	for _, form := range models.PrayerForms {
		hour, err := engine.ComposeHourWithOptions("lauds", dayFor(t, yd, "2026-12-26"), yd.moveable, office.ComposeOptions{Form: form})
		if err != nil {
			t.Fatal(err)
		}
		assertPrincipalSource(t, hour, "collect", "proper/st-stephen/collect")
		for _, section := range hour.Sections {
			for _, elem := range section.Elements {
				if elem.SourceRef == "proper/st-stephen/collect" &&
					(!strings.Contains(elem.Text, "we celebrate the birthday of him") || strings.Contains(elem.Text, "finding")) {
					t.Errorf("December collect changed: %+v", elem)
				}
			}
		}
	}
}
