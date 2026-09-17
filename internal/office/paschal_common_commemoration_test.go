package office

import (
	"strings"
	"testing"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
)

func TestPaschalCommonCommemorationAppointments(t *testing.T) {
	// Diurnal pp.3*–5*,17*–19*: the seasonal forms apply equally when
	// these offices are commemorated. Dedicated ordinary slots must not
	// shadow the Paschal antiphon or versicle.
	engine, err := NewEngine("../../data")
	if err != nil {
		t.Fatal(err)
	}
	for _, category := range []models.FeastCategory{
		models.CategoryApostle, models.CategoryEvangelist, models.CategoryConfessor,
		models.CategoryConfessorBishop, models.CategoryConfessorDoctor,
		models.CategoryVirgin, models.CategoryVirginMartyr,
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
	verse := engine.corpus.Get("commons/apostle-paschal/versicle-first-vespers")
	if strings.Count(strings.ToLower(verse), "alleluia") != 2 {
		t.Errorf("Diurnal p.17* appoints Alleluia in both V. and R.: %q", verse)
	}
}

func TestJosephAndAlexisCollectAppointments(t *testing.T) {
	// 2026 Ordo pp.46,61 explicitly appoints the alternate Confessor
	// collect at both the preceding Vespers and the day's Lauds.
	engine, err := NewEngine("../../data")
	if err != nil {
		t.Fatal(err)
	}
	days, err := calendar.BuildCalendar(2026, "../../data")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ date, hour, name string }{
		{"03-16", "vespers", "Joseph"}, {"03-17", "lauds", "Joseph"},
		{"05-06", "vespers", "Alexis"}, {"05-07", "lauds", "Alexis"},
	} {
		for _, form := range models.PrayerForms {
			t.Run(tc.date+"/"+tc.hour+"/"+string(form), func(t *testing.T) {
				seen := 0
				for i := range days {
					if days[i].Date.Format("01-02") != tc.date {
						continue
					}
					h, err := engine.ComposeHourWithOptions(tc.hour, &days[i], calendar.ComputeMoveableDates(2026), ComposeOptions{Form: form})
					if err != nil {
						t.Fatal(err)
					}
					for _, s := range h.Sections {
						for _, e := range s.Elements {
							if e.Type != models.Collect || !e.IsCommemoration || !strings.Contains(e.Text, "blessed "+tc.name) {
								continue
							}
							seen++
							if !strings.HasPrefix(e.Text, "Attend, O Lord") || !strings.Contains(e.Text, "blessed "+tc.name+", thy Confessor") {
								t.Errorf("wrong collect or name: %q", e.Text)
							}
						}
					}
				}
				if seen != 1 {
					t.Fatalf("found %d collects for %s, want exactly one", seen, tc.name)
				}
			})
		}
	}
}
