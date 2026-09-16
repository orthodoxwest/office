package office

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
)

func TestPaschalFerialLittleHourAntiphons(t *testing.T) {
	engine, err := NewEngine("../../data")
	if err != nil {
		t.Fatal(err)
	}
	// Diurnal p.374 prints four Alleluias at each little hour. The 2026
	// ordo pp.62–64 appoints that office, including Rogation and Ascension vigil.
	for _, year := range []int{2026, 2027, 2032} {
		days, err := calendar.BuildCalendar(year, "../../data")
		if err != nil {
			t.Fatal(err)
		}
		m := calendar.ComputeMoveableDates(year)
		n := 0
		for i := range days {
			day := &days[i]
			if day.Date.Weekday() == time.Sunday || day.Date.Before(m.LowSunday.AddDate(0, 0, 1)) || !day.Date.Before(m.Ascension) || (day.Celebration != nil && day.Celebration.Category != models.CategoryFeria) {
				continue
			}
			n++
			for _, form := range models.PrayerForms {
				for _, tc := range []struct{ hour, slot string }{{"prime", "1"}, {"terce", "2"}, {"sext", "3"}, {"none", "5"}} {
					h, err := engine.ComposeHourWithOptions(tc.hour, day, m, ComposeOptions{Form: form})
					if err != nil {
						t.Fatal(err)
					}
					ants := 0
					for _, s := range h.Sections {
						for _, e := range s.Elements {
							if e.Type != models.Antiphon || !strings.HasPrefix(e.SlotRef, "psalm-antiphon-") {
								continue
							}
							ants++
							if e.Text != "Alleluia, * alleluia, alleluia, alleluia." || e.SourceRef != "seasonal/easter/psalm-antiphon-"+tc.slot+"-"+tc.hour || !slices.Equal(e.SourceRefs, []string{"shared/formulas/paschal-little-hour-antiphon"}) {
								t.Errorf("%s/%s/%s: %+v", day.Date, tc.hour, form, e)
							}
						}
					}
					if ants != 2 {
						t.Errorf("%s/%s: %d antiphon frames", day.Date, tc.hour, ants)
					}
				}
			}
		}
		if n == 0 {
			t.Fatalf("no ferial appointments for %d", year)
		}
	}
}

func TestPaschalFerialAntiphonHourBoundaries(t *testing.T) {
	engine, err := NewEngine("../../data")
	if err != nil {
		t.Fatal(err)
	}
	// Include Saturday independently of whether the default calendar gives a
	// Saturday BVM office. P.374 distinguishes threefold Vespers from the Hours.
	for i := 0; i < 6; i++ {
		day := &models.CalendarDay{Date: time.Date(2026, 5, 4+i, 0, 0, 0, 0, time.UTC), Season: models.Easter}
		for _, hour := range []string{"prime", "terce", "sext", "none", "vespers"} {
			h, err := engine.ComposeHour(hour, day, calendar.ComputeMoveableDates(2026))
			if err != nil {
				t.Fatal(err)
			}
			want := 4
			if hour == "vespers" {
				want = 3
			}
			count := 0
			for _, s := range h.Sections {
				for _, e := range s.Elements {
					if e.Type == models.Antiphon && strings.HasPrefix(e.SlotRef, "psalm-antiphon-") {
						count++
						if strings.Count(strings.ToLower(e.Text), "alleluia") != want {
							t.Errorf("%s/%s: %+v", day.Date, hour, e)
						}
					}
				}
			}
			if count != 2 {
				t.Errorf("%s/%s: %d antiphon frames", day.Date, hour, count)
			}
		}
	}
	// The same seasonal data must not override a proper or a Common antiphon.
	for _, feast := range []models.Feast{
		{ID: "ascension", Rank: models.Double1stClass, Category: models.CategoryLord},
		{ID: "test-confessor", Rank: models.Double, Category: models.CategoryConfessor},
	} {
		day := &models.CalendarDay{Date: time.Date(2026, 5, 21, 0, 0, 0, 0, time.UTC), Season: models.Easter, Celebration: &feast}
		for _, hour := range []string{"prime", "terce", "sext", "none"} {
			h, err := engine.ComposeHour(hour, day, calendar.ComputeMoveableDates(2026))
			if err != nil {
				t.Fatal(err)
			}
			for _, s := range h.Sections {
				for _, e := range s.Elements {
					if e.Type == models.Antiphon && strings.HasPrefix(e.SlotRef, "psalm-antiphon-") && strings.HasPrefix(e.SourceRef, "seasonal/") {
						t.Errorf("%s/%s lost feast precedence: %+v", feast.ID, hour, e)
					}
				}
			}
		}
	}
}
