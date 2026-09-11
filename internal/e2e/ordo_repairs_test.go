package e2e

import (
	"slices"
	"strings"
	"testing"

	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/office"
)

// Check the appointed slots and hour boundaries directly: a year snapshot
// alone can accept an incorrect common fallback when its hashes are refreshed.
func Test2026OrdoRepairAppointments(t *testing.T) {
	eng, err := office.NewEngine(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	yd := buildYear(t, 2026)
	compose := func(name, date string) *models.OfficeHour {
		t.Helper()
		hour, err := eng.ComposeHour(name, dayFor(t, yd, date), yd.moveable)
		if err != nil {
			t.Fatal(err)
		}
		return hour
	}
	has := func(hour *models.OfficeHour, ref string) bool {
		for _, section := range hour.Sections {
			for _, elem := range section.Elements {
				if elem.SourceRef == ref || slices.Contains(elem.SourceRefs, ref) {
					return true
				}
			}
		}
		return false
	}
	for _, tc := range []struct{ date, hour, ref string }{
		{"2026-01-14", "vespers", "proper/st-maurus/magnificat-antiphon-first"},
		{"2026-01-15", "lauds", "proper/st-maurus/magnificat-antiphon-first"},
		{"2026-01-15", "vespers", "proper/st-maurus/magnificat-antiphon"},
		{"2026-02-09", "vespers", "proper/st-scholastica/magnificat-antiphon-first"},
		{"2026-10-06", "vespers", "proper/holy-rosary-bvm/magnificat-antiphon-first"},
		{"2026-01-30", "vespers", "proper/saturday-office-bvm-christmastide/magnificat-antiphon"},
		{"2026-05-15", "vespers", "proper/saturday-office-bvm-paschal/magnificat-antiphon"},
		{"2026-09-25", "vespers", "proper/saturday-office-bvm/magnificat-antiphon"},
		{"2026-10-09", "vespers", "proper/saturday-office-bvm/magnificat-antiphon"},
		{"2026-10-16", "vespers", "proper/saturday-office-bvm/magnificat-antiphon"},
		{"2026-11-13", "vespers", "proper/saturday-office-bvm/magnificat-antiphon"},
		{"2026-05-29", "vespers", "proper/ascension-sunday-within-octave/magnificat-antiphon"},
	} {
		t.Run(tc.date+"/"+tc.hour, func(t *testing.T) {
			if !has(compose(tc.hour, tc.date), tc.ref) {
				t.Errorf("missing appointed text %s", tc.ref)
			}
		})
	}
	scholastica := dayFor(t, yd, "2026-02-09")
	if len(scholastica.Vespers.Commemorations) != 1 || scholastica.Vespers.Commemorations[0].ID != models.FeriaCommemorationID {
		t.Errorf("Scholastica: missing outgoing feria: %+v", scholastica.Vespers.Commemorations)
	}
	friday := dayFor(t, yd, "2026-05-29")
	if friday.Celebration == nil || friday.Celebration.Rank != models.SemiDouble || friday.Vespers.Owner != models.VespersIIOfPreceding {
		t.Errorf("Friday after Ascension octave: incorrect calendar context: %+v", friday)
	}
	for _, date := range []string{"2026-05-24", "2026-05-29", "2026-05-30"} {
		hour := compose("lauds", date)
		for _, ref := range []string{"psalms/093", "psalms/100", "canticles/benedicite", "proper/ascension-sunday-within-octave/benedictus-antiphon"} {
			if !has(hour, ref) {
				t.Errorf("%s Lauds missing %s", date, ref)
			}
		}
	}
	for _, date := range []string{"2026-05-29", "2026-05-30"} {
		for _, name := range []string{"terce", "sext", "none"} {
			if !has(compose(name, date), "proper/ascension/short-responsory-"+name) {
				t.Errorf("%s %s missing Ascension versicle", date, name)
			}
		}
		for _, comm := range dayFor(t, yd, date).Commemorations {
			if strings.Contains(comm.ID, "ascension") {
				t.Errorf("%s must not commemorate Ascension", date)
			}
		}
	}
	if dayFor(t, yd, "2026-05-30").Vespers.Feast.ID != "pentecost" {
		t.Error("Pentecost must retain its First Vespers")
	}
	for _, tc := range []struct{ date, day string }{{"2026-09-16", "wednesday"}, {"2026-09-18", "friday"}, {"2026-09-19", "saturday"}} {
		proper := "proper/september-ember-" + tc.day + "/collect-lauds"
		for _, name := range []string{"lauds", "terce", "sext", "none"} {
			if !has(compose(name, tc.date), proper) {
				t.Errorf("%s %s missing Ember collect", tc.date, name)
			}
		}
		if got := compose("vespers", tc.date).Color; got != models.Green {
			t.Errorf("%s Vespers color = %s, want green", tc.date, got)
		}
		if dayFor(t, yd, tc.date).Color != models.Violet {
			t.Errorf("%s daytime Ember color was changed", tc.date)
		}
		for _, name := range []string{"prime", "vespers", "compline"} {
			if has(compose(name, tc.date), proper) {
				t.Errorf("%s %s incorrectly uses Ember collect", tc.date, name)
			}
		}
	}
}

func TestAntiphonParityAppointments(t *testing.T) {
	eng, err := office.NewEngine(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	yd := buildYear(t, 2026)
	for _, tc := range []struct{ date, hour, ref string }{
		{"2026-02-06", "vespers", "ordinary/vespers/magnificat-antiphon-friday"},
		{"2026-02-13", "vespers", "proper/septuagesima/magnificat-antiphon-friday"},
		{"2026-02-19", "vespers", "proper/sexagesima/magnificat-antiphon-thursday"},
		{"2026-02-20", "vespers", "proper/sexagesima/magnificat-antiphon-friday"},
		{"2026-05-04", "vespers", "proper/easter-sunday-3/magnificat-antiphon-monday"},
		{"2026-05-05", "lauds", "proper/easter-sunday-3/benedictus-antiphon-tuesday"},
		{"2026-05-05", "vespers", "proper/st-john-latin-gate/magnificat-antiphon-first"},
		{"2026-05-06", "vespers", "proper/st-john-latin-gate/magnificat-antiphon-first"},
		{"2026-12-22", "lauds", "proper/advent-sunday-4/benedictus-antiphon-tuesday"},
		{"2026-12-23", "lauds", "proper/advent-sunday-4/benedictus-antiphon-friday"},
		{"2026-12-23", "vespers", "seasonal/advent/magnificat-antiphon-december-23"},
	} {
		t.Run(tc.date+"/"+tc.hour, func(t *testing.T) {
			hour, err := eng.ComposeHour(tc.hour, dayFor(t, yd, tc.date), yd.moveable)
			if err != nil {
				t.Fatal(err)
			}
			for _, section := range hour.Sections {
				for _, elem := range section.Elements {
					if elem.SourceRef == tc.ref || slices.Contains(elem.SourceRefs, tc.ref) {
						return
					}
				}
			}
			t.Errorf("missing appointed antiphon %s", tc.ref)
		})
	}
}
