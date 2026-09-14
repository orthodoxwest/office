package e2e

import (
	"testing"

	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/office"
)

// Diurnal p. 67* prints these pairs; pp. 69*-71* send the Saturday
// chapters/versicles there, including the After Christmas and Paschal forms.
// The 2026 ordo confirms the chosen Hours on pp. 33, 63, 73, 77, 96, 97.
func TestBVMCommonVersicleAppointments(t *testing.T) {
	engine, err := office.NewEngine(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	yd := buildYear(t, 2026)
	ordinary := map[string]string{
		"terce": "V. In thy grace and in thy beauty.\nR. Go forth, ride prosperously, and reign.",
		"sext":  "V. God shall help her with his countenance.\nR. God is in the midst of her, therefore shall she not be removed.",
		"none":  "V. God hath chosen her and preferred her.\nR. He hath made her to dwell in his tabernacle.",
	}
	// Preserve the existing Paschal decoration of one alleluia per line.
	paschal := map[string]string{
		"terce": "V. In thy grace and in thy beauty, alleluia.\nR. Go forth, ride prosperously, and reign, alleluia.",
		"sext":  "V. God shall help her with his countenance, alleluia.\nR. God is in the midst of her, therefore shall she not be removed, alleluia.",
		"none":  "V. God hath chosen her and preferred her, alleluia.\nR. He hath made her to dwell in his tabernacle, alleluia.",
	}
	for _, date := range []string{"2026-01-31", "2026-05-16", "2026-06-20", "2026-07-02", "2026-09-08", "2026-09-12"} {
		day := dayFor(t, yd, date)
		for _, name := range []string{"terce", "sext", "none"} {
			for _, form := range []models.PrayerForm{"private", "deacon", "priest"} {
				hour, err := engine.ComposeHourWithOptions(name, day, yd.moveable, office.ComposeOptions{Form: form})
				if err != nil {
					t.Fatal(err)
				}
				want := ordinary[name]
				if date == "2026-05-16" {
					want = paschal[name]
				}
				ref := "commons/blessed-virgin/versicle-" + name
				canonical := ref
				if name == "terce" {
					canonical = "commons/virgin-martyr/versicle-first-vespers"
				}
				found := 0
				for _, section := range hour.Sections {
					for _, elem := range section.Elements {
						if elem.SlotRef != "versicle" || elem.IsCommemoration {
							continue
						}
						found++
						if elem.Type != models.Versicle || elem.Text != want || elem.SourceRef != ref ||
							len(elem.SourceRefs) != 1 || elem.SourceRefs[0] != canonical {
							t.Errorf("%s/%s/%s: got %+v; want %s -> %s", date, name, form, elem, ref, canonical)
						}
					}
				}
				if found != 1 {
					t.Fatalf("%s/%s/%s: found %d versicles", date, name, form, found)
				}
			}
		}
	}
}
