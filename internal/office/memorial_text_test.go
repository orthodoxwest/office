package office

import (
	"strings"
	"testing"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/texts"
)

func TestMemorialCommemorationAppointments(t *testing.T) {
	// Diurnal pp.xxix,1*–3*; 2026 ordo pp.29,66: a Memorial begins
	// at its own I Vespers, even when the principal office is II Vespers.
	engine, err := NewEngine("../../data")
	if err != nil {
		t.Fatal(err)
	}
	days, err := calendar.BuildCalendar(2026, "../../data")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ date, hour, name, ant, verse string }{
		{"01-15", "vespers", "Marcellus", "This is a Martyr", "V. Thou hast crowned him"},
		{"05-25", "vespers", "Eleutherius", "Light perpetual", "V. O ye holy and righteous"},
		{"05-26", "vespers", "John I", "Light perpetual", "V. O ye holy and righteous"},
		{"05-26", "lauds", "Eleutherius", "Daughters of Jerusalem", "V. Right dear"},
		{"05-27", "lauds", "John I", "Daughters of Jerusalem", "V. Right dear"},
	} {
		for _, form := range models.PrayerForms {
			t.Run(tc.date+"/"+tc.hour+"/"+string(form), func(t *testing.T) {
				for i := range days {
					day := &days[i]
					if day.Date.Format("01-02") != tc.date {
						continue
					}
					hour, err := engine.ComposeHourWithOptions(tc.hour, day, calendar.ComputeMoveableDates(2026), ComposeOptions{Form: form})
					if err != nil {
						t.Fatal(err)
					}
					owner := ""
					seen := 0
					for _, s := range hour.Sections {
						for _, e := range s.Elements {
							if e.Type == models.Heading && e.IsCommemoration && strings.Contains(e.Text, tc.name) {
								owner = e.CommemorationOwnerID
							}
							if owner == "" || e.CommemorationOwnerID != owner {
								continue
							}
							want := map[string]string{"commemoration-antiphon": tc.ant, "commemoration-versicle": tc.verse}[e.SlotRef]
							if want == "" {
								continue
							}
							seen++
							if !strings.HasPrefix(e.Text, want) {
								t.Errorf("%s = %q, want %q", e.SlotRef, e.Text, want)
							}
							trace := engine.TraceCommemorationResolution(day, tc.hour, e.SlotRef, e.SourceRef, owner)
							if trace.OwnerID != owner || trace.FirstVespers != (tc.hour == "vespers") {
								t.Errorf("trace = %#v", trace)
							}
						}
					}
					if seen != 2 {
						t.Fatalf("found %d Memorial antiphon/versicle slots", seen)
					}
				}
			})
		}
	}
}

func TestMemorialProperAndCompanionBoundaries(t *testing.T) {
	corpus := texts.NewTestCorpus(map[string]string{
		"proper/named/commemoration-antiphon-vespers":   "Named appointment",
		"proper/named/commemoration-collect":            "Named collect",
		"commons/martyr/magnificat-antiphon-first":      "First antiphon",
		"commons/martyr/commemoration-antiphon-vespers": "Second antiphon",
		"commons/martyr/versicle-first-vespers":         "First verse",
		"commons/martyr/commemoration-versicle-vespers": "Second verse",
		"commons/martyr/commemoration-collect":          "Common collect",
	})
	for _, tc := range []struct {
		name, proper, companion string
		rank                    models.Rank
		ant, verse, collect     string
	}{
		{"memorial", "", "", models.Commemoration, "First antiphon", "First verse", "Common collect"},
		{"proper", "named", "", models.Commemoration, "Named appointment", "First verse", "Named collect"},
		{"outgoing double", "", "", models.Double, "Second antiphon", "Second verse", "Common collect"},
		{"perpetual companion", "", "principal", models.Commemoration, "Second antiphon", "Second verse", "Common collect"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			comm := &models.Feast{ID: "example", ProperID: tc.proper, Rank: tc.rank, CompanionOf: tc.companion, Category: models.CategoryMartyr}
			day := &models.CalendarDay{Commemorations: []*models.Feast{comm}}
			elems := addCommemorations(day, "vespers", corpus, true)
			for _, e := range elems {
				want := map[string]string{"commemoration-antiphon": tc.ant, "commemoration-versicle": tc.verse, "commemoration-collect": tc.collect}[e.SlotRef]
				if want != "" && e.Text != want {
					t.Errorf("%s = %q, want %q", e.SlotRef, e.Text, want)
				}
			}
		})
	}
}

func TestPaschalMartyrCommemorationsKeepTheirCommon(t *testing.T) {
	engine, err := NewEngine("../../data")
	if err != nil {
		t.Fatal(err)
	}
	for _, category := range []models.FeastCategory{models.CategoryMartyr, models.CategoryMartyrs, models.CategoryBishopMartyr} {
		feast := &models.Feast{ID: "example", Category: category, Rank: models.Double}
		for _, tc := range []struct{ hour, slot, want string }{
			{"lauds", "commemoration-antiphon", "Daughters of Jerusalem"},
			{"lauds", "commemoration-versicle", "V. Right dear"},
			{"vespers", "commemoration-antiphon", "O ye holy and righteous"},
			{"vespers", "commemoration-versicle", "V. Right dear"},
		} {
			got, ref := lookupCommemoration(feast, models.Easter, tc.hour, tc.slot, engine.corpus)
			if !strings.HasPrefix(got, tc.want) || !strings.HasPrefix(ref, "commons/"+string(category)+"-paschal/") {
				t.Errorf("%s %s %s = %q (%s)", category, tc.hour, tc.slot, got, ref)
			}
		}
	}
}

func TestPaschalCommonCommemorationsKeepSeasonalTexts(t *testing.T) {
	// Dedicated ordinary slots must not shadow seasonal Common texts.
	engine, err := NewEngine("../../data")
	if err != nil {
		t.Fatal(err)
	}
	for _, category := range []models.FeastCategory{
		models.CategoryApostle, models.CategoryEvangelist, models.CategoryConfessor,
		models.CategoryConfessorBishop, models.CategoryConfessorDoctor,
		models.CategoryVirgin, models.CategoryVirginMartyr, models.CategoryHolyWoman,
	} {
		feast := &models.Feast{ID: "example", Rank: models.Commemoration, Category: category}
		for _, hour := range []string{"lauds", "vespers"} {
			for _, slot := range []string{"commemoration-antiphon", "commemoration-versicle"} {
				text, source := lookupCommemoration(feast, models.Easter, hour, slot, engine.corpus)
				if !strings.Contains(strings.ToLower(text), "alleluia") || !strings.HasPrefix(source, "commons/"+string(category)+"-paschal/") {
					t.Errorf("%s %s %s lost its Paschal form: %q (%s)", category, hour, slot, text, source)
				}
				if hour == "vespers" {
					text, source = lookupFollowingOfficeCommemoration(feast, models.Easter, slot, engine.corpus)
					if !strings.Contains(strings.ToLower(text), "alleluia") || !strings.HasPrefix(source, "commons/"+string(category)+"-paschal/") {
						t.Errorf("%s first Vespers %s lost its Paschal form: %q (%s)", category, slot, text, source)
					}
				}
			}
		}
	}
}
