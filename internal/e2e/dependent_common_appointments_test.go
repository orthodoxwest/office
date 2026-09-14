package e2e

import (
	"testing"
	"time"

	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/office"
	"github.com/orthodoxwest/office/internal/texts"
)

// Diurnal pp. 6*, 513, 626 send Evangelists to the Apostle commons; pp.
// 31*-33* send One or Many Martyrs in Paschaltide there with no little-hour
// chapter/versicle exception. These controlled common offices also exercise
// Many Martyrs, which has no principal Paschal occurrence in the sampled years.
func TestDependentCommonVersicleAppointments(t *testing.T) {
	engine, err := office.NewEngine(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	corpus, err := texts.LoadTexts(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		category models.FeastCategory
		season   models.Season
		common   string
		target   string
	}{
		{models.CategoryEvangelist, models.Pentecost, "evangelist", "apostle"},
		{models.CategoryEvangelist, models.Easter, "evangelist-paschal", "apostle-paschal"},
		{models.CategoryMartyr, models.Easter, "martyr-paschal", "apostle-paschal"},
		{models.CategoryMartyrs, models.Easter, "martyrs-paschal", "apostle-paschal"},
		{models.CategoryBishopMartyr, models.Easter, "bishop-martyr-paschal", "apostle-paschal"},
	} {
		t.Run(tc.common, func(t *testing.T) {
			date := time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC)
			if tc.season != models.Easter {
				date = time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC)
			}
			day := &models.CalendarDay{Date: date, Season: tc.season,
				Celebration: &models.Feast{ID: "common-probe", Category: tc.category}}
			for _, name := range []string{"terce", "sext", "none"} {
				for _, form := range []models.PrayerForm{"private", "deacon", "priest"} {
					hour, err := engine.ComposeHourWithOptions(name, day, nil, office.ComposeOptions{Form: form})
					if err != nil {
						t.Fatal(err)
					}
					wantRef := "commons/" + tc.common + "/versicle-" + name
					target := "commons/" + tc.target + "/versicle-" + name
					wantText, canonical := corpus.Get(target), corpus.CanonicalRef(target)
					if wantText == "" || canonical == "" {
						t.Fatal("missing reviewed Apostle target", target)
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
