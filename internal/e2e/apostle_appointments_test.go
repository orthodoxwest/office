package e2e

import (
	"testing"

	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/office"
)

// Diurnal pp. 12*, 19*: each little hour has a printed V./R., including
// one alleluia on each Paschal line. These expectations come from the pages,
// not reduction of a responsory. The 2026 ordo confirms use on pp. 59, 92.
func TestApostleLittleHourAppointmentsAcrossCalendars(t *testing.T) {
	engine, err := office.NewEngine(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	ordinary := map[string]string{
		"terce": "V. Their sound is gone out into all lands.\nR. And their words into the ends of the world.",
		"sext":  "V. Thou shalt make them princes in all lands.\nR. They shall remember thy Name, O Lord.",
		"none":  "V. Exceedingly honoured are thy friends, O God.\nR. Firmly stablished is their princedom.",
	}
	paschal := map[string]string{
		"terce": "V. O ye holy and righteous, rejoice in the Lord, alleluia.\nR. God hath chosen you to him to be his inheritance, alleluia.",
		"sext":  "V. Light perpetual shall shine upon thy Saints, O Lord, alleluia.\nR. And an ageless eternity, alleluia.",
		"none":  "V. Everlasting joy shall be upon their heads, alleluia.\nR. They shall obtain joy and gladness, alleluia.",
	}
	for _, year := range []int{2026, 2027, 2032} {
		yd := buildYear(t, year)
		checked := map[bool]int{}
		for i := range yd.days {
			day := &yd.days[i]
			if day.Celebration == nil || day.Celebration.Category != models.CategoryApostle {
				continue
			}
			// Its explicit proper responsories remain a separate migration.
			// TestCompositionRequirements preserves that precedence separately.
			if day.Celebration.ID == "conversion-st-paul" {
				continue
			}
			isPaschal := day.Season == models.Easter
			prefix, wording := "commons/apostle/", ordinary
			if isPaschal {
				prefix, wording = "commons/apostle-paschal/", paschal
			}
			for _, name := range []string{"terce", "sext", "none"} {
				for _, form := range []models.PrayerForm{"private", "deacon", "priest"} {
					hour, err := engine.ComposeHourWithOptions(name, day, yd.moveable, office.ComposeOptions{Form: form})
					if err != nil {
						t.Fatal(err)
					}
					found := 0
					for _, section := range hour.Sections {
						for _, elem := range section.Elements {
							if elem.SlotRef != "versicle" || elem.IsCommemoration {
								continue
							}
							found++
							wantRef := prefix + "versicle-" + name
							canonical := wantRef
							if name == "terce" {
								canonical = "commons/apostle/versicle-first-vespers"
								if isPaschal {
									canonical = "commons/martyr-paschal/versicle-first-vespers"
								}
							}
							if elem.Type != models.Versicle || elem.Text != wording[name] || elem.SourceRef != wantRef ||
								len(elem.SourceRefs) != 1 || elem.SourceRefs[0] != canonical {
								t.Errorf("%s %s %s: verse = %+v; want %s (%s)", day.Date.Format("2006-01-02"), name, form, elem, wantRef, canonical)
							}
						}
					}
					if found != 1 {
						t.Fatalf("%s %s: found %d versicles", day.Date, name, found)
					}
					checked[isPaschal]++
				}
			}
		}
		if checked[false] == 0 || checked[true] == 0 {
			t.Fatalf("%d: missing ordinary or Paschal exercise: %v", year, checked)
		}
	}
}
