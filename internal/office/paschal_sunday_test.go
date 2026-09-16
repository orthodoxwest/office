package office

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/texts"
)

func TestPaschalSundayPsalmody(t *testing.T) {
	engine, err := NewEngine("../../data")
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{"low-sunday", "easter-sunday-2", "easter-sunday-3", "easter-sunday-4", "easter-sunday-5"}
	seen := map[string]bool{}
	// Diurnal pp.34–37,370–371,377,380,382,385. Actual calendars also
	// cover displacement by a feast (III Easter is displaced in 2026).
	for _, year := range []int{2026, 2027, 2032} {
		days, err := calendar.BuildCalendar(year, "../../data")
		if err != nil {
			t.Fatal(err)
		}
		for i := range days {
			day := &days[i]
			if day.Celebration == nil || !slices.Contains(ids, day.Celebration.ID) {
				continue
			}
			seen[day.Celebration.ID] = true
			for _, form := range models.PrayerForms {
				h, err := engine.ComposeHourWithOptions("lauds", day, calendar.ComputeMoveableDates(year), ComposeOptions{Form: form})
				if err != nil {
					t.Fatal(err)
				}
				var psalms []string
				var ants []models.OfficeElement
				cant, glorias := 0, 0
				for _, s := range h.Sections {
					for _, e := range s.Elements {
						if e.Type == models.Psalm {
							psalms = append(psalms, e.SourceRef)
						}
						if e.SourceRef == "canticles/benedicite" {
							cant++
						}
						if e.SourceRef == "ordinary/shared/gloria-patri" {
							glorias++
						}
						if e.Type == models.Antiphon && strings.HasPrefix(e.SlotRef, "psalm-antiphon-") {
							ants = append(ants, e)
						}
					}
				}
				if !slices.Equal(psalms, []string{"psalms/067", "psalms/093", "psalms/100", "psalms/063", "psalms/148", "psalms/149", "psalms/150"}) || cant != 1 || glorias != 6 {
					t.Fatalf("%s/%s: psalms %v, Benedicite %d, Glorias %d", day.Date, form, psalms, cant, glorias)
				}
				if len(ants) != 6 {
					t.Fatalf("%s/%s: %d antiphon frames", day.Date, form, len(ants))
				}
				for j, e := range ants {
					switch j {
					case 0, 1:
						if strings.Count(strings.ToLower(e.Text), "alleluia") != 9 || strings.Count(e.Text, ";") != 2 {
							t.Errorf("first group: %+v", e)
						}
					case 2, 3:
						if e.Text != "Christ is risen * from the grave, who delivered the three children from the burning fiery furnace, alleluia." {
							t.Errorf("Benedicite antiphon: %+v", e)
						}
					case 4, 5:
						if strings.Count(strings.ToLower(e.Text), "alleluia") != 3 {
							t.Errorf("Laudate: %+v", e)
						}
					}
				}
				// Prime must use its own fourfold antiphon, not borrow Lauds' ninefold.
				for _, hour := range []string{"prime", "terce", "sext", "none"} {
					minor, err := engine.ComposeHourWithOptions(hour, day, calendar.ComputeMoveableDates(year), ComposeOptions{Form: form})
					if err != nil {
						t.Fatal(err)
					}
					n := 0
					for _, s := range minor.Sections {
						for _, e := range s.Elements {
							if e.Type != models.Antiphon || !strings.HasPrefix(e.SlotRef, "psalm-antiphon-") {
								continue
							}
							n++
							if strings.Count(strings.ToLower(e.Text), "alleluia") != 4 || !strings.HasSuffix(e.SourceRef, "-"+hour) || !slices.Equal(e.SourceRefs, []string{"shared/formulas/paschal-little-hour-antiphon"}) {
								t.Errorf("%s/%s/%s: %+v", day.Date, hour, form, e)
							}
						}
					}
					if n != 2 {
						t.Errorf("%s/%s: %d antiphons", day.Date, hour, n)
					}
				}
			}
		}
	}
	for _, id := range ids {
		if !seen[id] {
			t.Errorf("no actual calendar case for %s", id)
		}
	}
}

func TestExplicitSundayLaudsPsalmody(t *testing.T) {
	day := &models.CalendarDay{Date: time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC), Season: models.Pentecost, Celebration: &models.Feast{ID: "test-sunday", ProperID: "shared-sunday", Rank: models.SemiDouble, Category: models.CategorySunday}}
	for _, key := range []string{"proper/test-sunday/lauds-psalmody", "proper/shared-sunday/lauds-psalmody"} {
		if !usesFestalLaudsPsalmody(day, texts.NewTestCorpus(map[string]string{key: "festal"})) {
			t.Errorf("ignored appointment %s", key)
		}
	}
	for _, corpus := range []*texts.TextCorpus{nil, texts.NewTestCorpus(map[string]string{})} {
		if usesFestalLaudsPsalmody(day, corpus) {
			t.Error("ordinary Sunday acquired festal psalmody without appointment")
		}
	}
	// Former ID exceptions must now come from data, including ProperID redirects.
	engine, err := NewEngine("../../data")
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"nativity-sunday-within-octave", "epiphany-sunday-within-octave", "ascension-sunday-within-octave"} {
		day.Celebration.ID, day.Celebration.ProperID = "redirected", id
		if !usesFestalLaudsPsalmody(day, engine.corpus) {
			t.Errorf("lost %s redirect", id)
		}
		if usesFestalLaudsPsalmody(day, texts.NewTestCorpus(map[string]string{})) {
			t.Errorf("retained hardcoded %s", id)
		}
	}
}
