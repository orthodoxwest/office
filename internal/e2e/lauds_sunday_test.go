package e2e

import (
	"slices"
	"strings"
	"testing"

	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/office"
)

// Diurnal pp. 39 and 41 appoint a distinct Sunday chapter, responsory,
// and versicle. The 2026 ordo p. 98 explicitly refers September 13 to these
// pages. A verified ferial text is not a valid fallback for these slots.
func TestSundayLaudsOrdinaryAppointments(t *testing.T) {
	eng, err := office.NewEngine(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	yd := buildYear(t, 2026)
	for _, date := range []string{"2026-01-18", "2026-09-13", "2026-09-20", "2026-10-04"} {
		for _, form := range models.PrayerForms {
			t.Run(date+"/"+string(form), func(t *testing.T) {
				hour, err := eng.ComposeHourWithOptions("lauds", dayFor(t, yd, date), yd.moveable, office.ComposeOptions{Form: form})
				if err != nil {
					t.Fatal(err)
				}
				var found []string
				for _, section := range hour.Sections {
					for _, elem := range section.Elements {
						var incipit string
						switch elem.SlotRef {
						case "chapter":
							incipit = "Blessing, and glory"
						case "short-responsory":
							incipit = "Incline my heart, O God"
						case "versicle":
							incipit = "The Lord is King"
						default:
							continue
						}
						found = append(found, elem.SlotRef)
						want := "ordinary/lauds/" + elem.SlotRef + "-sunday"
						if elem.SourceRef != want || !strings.Contains(elem.Text, incipit) {
							t.Errorf("%s: source = %s, text = %q; want %s beginning %q", elem.SlotRef, elem.SourceRef, elem.Text, want, incipit)
						}
					}
				}
				if !slices.Equal(found, []string{"chapter", "short-responsory", "versicle"}) {
					t.Errorf("Sunday slots missing, duplicated, or out of order: %v", found)
				}
			})
		}
	}
}

// The ordinary Sunday entries must yield to appointed proper/seasonal texts
// and leave weekday Lauds alone. Diurnal p. 39 retains the Sunday responsory
// through Quinquagesima; p. 40 gives those Sundays their own versicle.
func TestSundayLaudsOrdinaryExceptions(t *testing.T) {
	eng, err := office.NewEngine(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	yd := buildYear(t, 2026)
	for _, tc := range []struct {
		date                          string
		chapter, responsory, versicle string
	}{
		{"2026-09-17", "ordinary/lauds/chapter", "ordinary/lauds/short-responsory", "ordinary/lauds/versicle"},
		{"2026-02-08", "proper/septuagesima/chapter-lauds", "ordinary/lauds/short-responsory-sunday", "proper/septuagesima/versicle-lauds"},
		{"2026-03-01", "proper/lent-sunday-1/chapter-lauds", "seasonal/lent/short-responsory-lauds", "proper/lent-sunday-1/versicle-lauds"},
		{"2026-04-26", "seasonal/easter/chapter-lauds", "seasonal/easter/short-responsory-lauds", "seasonal/easter/versicle-lauds"},
		{"2026-06-07", "proper/trinity-sunday/chapter-lauds", "proper/trinity-sunday/short-responsory-lauds", "proper/trinity-sunday/versicle-lauds"},
		{"2026-06-14", "proper/pentecost-sunday-2/chapter-lauds", "proper/pentecost-sunday-2/short-responsory-lauds", "proper/pentecost-sunday-2/versicle-lauds"},
		{"2026-11-01", "proper/all-saints/chapter-lauds", "proper/all-saints/short-responsory-lauds", "proper/all-saints/versicle-lauds"},
	} {
		t.Run(tc.date, func(t *testing.T) {
			hour, err := eng.ComposeHour("lauds", dayFor(t, yd, tc.date), yd.moveable)
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, section := range hour.Sections {
				for _, elem := range section.Elements {
					if slices.Contains([]string{"chapter", "short-responsory", "versicle"}, elem.SlotRef) {
						got = append(got, elem.SourceRef)
					}
				}
			}
			if want := []string{tc.chapter, tc.responsory, tc.versicle}; !slices.Equal(got, want) {
				t.Errorf("appointed sources = %v, want %v", got, want)
			}
		})
	}
}
