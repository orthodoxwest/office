package e2e

import (
	"strings"
	"testing"

	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/office"
)

// Diurnal pp. 206–207,213,240,584 appoint these chapters. The 2026 ordo
// explicitly sends Circumcision, Transfiguration and Nativity Sunday Hours
// to pp. 213,584,207 (ordo pp. 25,87,128). Sexagesima's Sunday psalter
// reference (ordo p. 38) does not give a different chapter appointment.
func TestProperLittleHourChapters(t *testing.T) {
	engine, err := office.NewEngine(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		feast, hour, words string
		dates              []string
	}{
		{"circumcision", "sext", "Thou, Lord, in the beginning", []string{"2026-01-01", "2027-01-01", "2032-01-01"}},
		{"circumcision", "none", "They shall perish, but thou remainest", []string{"2026-01-01", "2027-01-01", "2032-01-01"}},
		{"transfiguration", "none", "And he carried me away in the spirit", []string{"2026-08-06", "2027-08-06", "2032-08-06"}},
		{"sexagesima", "none", "Most gladly therefore will I rather glory", []string{"2026-02-15", "2027-03-07", "2032-03-07"}},
		{"nativity-sunday-within-octave", "terce", "The heir, as long as he is a child", []string{"2026-12-29", "2027-12-29", "2032-12-29"}},
		{"nativity-sunday-within-octave", "sext", "But when the fulness of the time was come", []string{"2026-12-29", "2027-12-29", "2032-12-29"}},
		{"nativity-sunday-within-octave", "none", "Wherefore thou art no more a servant", []string{"2026-12-29", "2027-12-29", "2032-12-29"}},
	}
	years := map[string]*yearData{"2026": buildYear(t, 2026), "2027": buildYear(t, 2027), "2032": buildYear(t, 2032)}
	for _, tc := range cases {
		for _, date := range tc.dates {
			for _, form := range []models.PrayerForm{"private", "deacon", "priest"} {
				t.Run(date+"/"+tc.hour+"/"+string(form), func(t *testing.T) {
					yd := years[date[:4]]
					day := dayFor(t, yd, date)
					if day.Celebration == nil || day.Celebration.ID != tc.feast {
						t.Fatalf("fixture must exercise %s", tc.feast)
					}
					hour, err := engine.ComposeHourWithOptions(tc.hour, day, yd.moveable, office.ComposeOptions{Form: form})
					if err != nil {
						t.Fatal(err)
					}
					chapter, verse, index, count := -1, -1, 0, 0
					for _, s := range hour.Sections {
						for _, e := range s.Elements {
							if e.SlotRef == "chapter" {
								chapter = index
								count++
								want := "proper/" + tc.feast + "/chapter-" + tc.hour
								if e.SourceRef != want || !strings.Contains(e.Text, tc.words) {
									t.Errorf("chapter = %s %q, want %s containing %q", e.SourceRef, e.Text, want, tc.words)
								}
								if !strings.HasSuffix(e.Text, "R. Thanks be to God.") {
									t.Errorf("chapter lacks its response: %q", e.Text)
								}
							}
							if e.SlotRef == "versicle" && verse < 0 {
								verse = index
							}
							index++
						}
					}
					if count != 1 || verse < 0 || chapter >= verse {
						t.Errorf("want one chapter before verse; count=%d positions=%d/%d", count, chapter, verse)
					}
				})
			}
		}
	}
}
