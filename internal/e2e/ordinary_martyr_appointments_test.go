package e2e

import (
	"testing"
	"time"

	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/office"
	"github.com/orthodoxwest/office/internal/texts"
)

// Diurnal pp. 23*, 27*: literal little-hour pairs, including the response
// following the Sext chapter across the column break.
func TestOrdinaryMartyrVersicleAppointments(t *testing.T) {
	engine, err := office.NewEngine(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	corpus, err := texts.LoadTexts(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	wording := map[string]map[string]string{
		"bishop-martyr": {
			"terce": "V. Thou hast crowned him with glory and honour, O Lord.\nR. And madest him to have dominion of the works of thy hands.",
			"sext":  "V. Thou hast set upon his head, O Lord.\nR. A crown of pure gold.",
			"none":  "V. His honour is great in thy salvation.\nR. Glory and great worship shalt thou lay upon him.",
		},
		"martyrs": {
			"terce": "V. Be glad, O ye righteous, and rejoice in the Lord.\nR. And be joyful, all ye that are true of heart.",
			"sext":  "V. Let the righteous rejoice before God.\nR. Let them also be merry and joyful.",
			"none":  "V. The righteous live for evermore.\nR. Their reward also is with the Lord.",
		},
	}
	for _, tc := range []struct {
		category       models.FeastCategory
		common, target string
	}{
		{models.CategoryMartyr, "martyr", "bishop-martyr"},
		{models.CategoryBishopMartyr, "bishop-martyr", "bishop-martyr"},
		{models.CategoryMartyrs, "martyrs", "martyrs"},
	} {
		t.Run(tc.common, func(t *testing.T) {
			day := &models.CalendarDay{Date: time.Date(2026, 6, 22, 0, 0, 0, 0, time.UTC), Season: models.Pentecost,
				Celebration: &models.Feast{ID: "common-probe", Category: tc.category}}
			for _, name := range []string{"terce", "sext", "none"} {
				for _, form := range []models.PrayerForm{"private", "deacon", "priest"} {
					hour, err := engine.ComposeHourWithOptions(name, day, nil, office.ComposeOptions{Form: form})
					if err != nil {
						t.Fatal(err)
					}
					wantRef := "commons/" + tc.common + "/versicle-" + name
					target := "commons/" + tc.target + "/versicle-" + name
					wantText, canonical := wording[tc.target][name], corpus.CanonicalRef(target)
					if wantText == "" || canonical == "" {
						t.Fatal("missing reviewed Martyr target", target)
					}
					found := 0
					for _, section := range hour.Sections {
						for _, elem := range section.Elements {
							if elem.SlotRef != "versicle" || elem.IsCommemoration {
								continue
							}
							found++
							if elem.Type != models.Versicle || elem.SourceRef != wantRef || elem.Text != wantText ||
								len(elem.SourceRefs) != 1 || elem.SourceRefs[0] != canonical {
								t.Errorf("%s/%s: got %+v; want %s -> %s", name, form, elem, wantRef, canonical)
							}
						}
					}
					if found != 1 {
						t.Fatalf("%s/%s: found %d versicles", name, form, found)
					}
				}
			}
		})
	}
}
