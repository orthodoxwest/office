package office

import (
	"testing"
	"time"

	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/texts"
)

func TestAnticipatedSundayFirstVespersAntiphon(t *testing.T) {
	corpus := texts.NewTestCorpus(map[string]string{
		"proper/epiphany-sunday-5/magnificat-antiphon":       "Sunday gospel",
		"proper/epiphany-sunday-5/magnificat-antiphon-first": "Sunday first Vespers",
		"ordinary/vespers/magnificat-antiphon-friday":        "Friday psalter",
		"ordinary/vespers/magnificat-antiphon-saturday":      "Saturday psalter",
	})
	for _, tc := range []struct {
		name, id, want string
		day            int
	}{
		{"anticipated on Saturday", "epiphany-sunday-5-anticipated", "ordinary/vespers/magnificat-antiphon-friday", 7},
		{"ordinary Sunday", "epiphany-sunday-5", "proper/epiphany-sunday-5/magnificat-antiphon-first", 8},
	} {
		t.Run(tc.name, func(t *testing.T) {
			day := &models.CalendarDay{
				Date: time.Date(2026, 2, tc.day, 0, 0, 0, 0, time.UTC), FirstVespers: true,
				Season:      models.Epiphany,
				Celebration: &models.Feast{ID: tc.id, ProperID: "epiphany-sunday-5", Category: models.CategorySunday},
			}
			_, ref := resolveProperText(day, "vespers", "magnificat-antiphon", corpus)
			if ref != tc.want {
				t.Fatalf("selected %s, want %s", ref, tc.want)
			}
		})
	}
}

func TestDecember23BenedictusAppointment(t *testing.T) {
	const fixed = "seasonal/advent/benedictus-antiphon-december-23"
	corpus := texts.NewTestCorpus(map[string]string{
		fixed:                             "Fixed December 23 antiphon",
		"proper/test/benedictus-antiphon": "Own antiphon",
		"proper/test/magnificat-antiphon": "Own Magnificat",
	})
	for _, tc := range []struct {
		name, date, hour, slot string
		season                 models.Season
		category               models.FeastCategory
		fixed                  bool
	}{
		{"Wednesday feria", "2026-12-23", "lauds", "benedictus-antiphon", models.Advent, models.CategoryFeria, true},
		{"Thursday feria", "2027-12-23", "lauds", "benedictus-antiphon", models.Advent, models.CategoryFeria, true},
		{"Advent Sunday", "2029-12-23", "lauds", "benedictus-antiphon", models.Advent, models.CategorySunday, true},
		{"preceding day", "2026-12-22", "lauds", "benedictus-antiphon", models.Advent, models.CategoryFeria, false},
		{"Nativity vigil", "2026-12-24", "lauds", "benedictus-antiphon", models.Advent, models.CategoryFeria, false},
		{"other month", "2026-11-23", "lauds", "benedictus-antiphon", models.Advent, models.CategoryFeria, false},
		{"other season", "2026-12-23", "lauds", "benedictus-antiphon", models.Christmas, models.CategoryFeria, false},
		{"saint retains its office", "2026-12-23", "lauds", "benedictus-antiphon", models.Advent, models.CategoryMartyr, false},
		{"other hour", "2026-12-23", "vespers", "benedictus-antiphon", models.Advent, models.CategoryFeria, false},
		{"other canticle", "2026-12-23", "lauds", "magnificat-antiphon", models.Advent, models.CategoryFeria, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			date, err := time.Parse("2006-01-02", tc.date)
			if err != nil {
				t.Fatal(err)
			}
			day := &models.CalendarDay{Date: date, Season: tc.season,
				Celebration: &models.Feast{ID: "test", Category: tc.category}}
			_, ref := resolveProperText(day, tc.hour, tc.slot, corpus)
			want := "proper/test/" + tc.slot
			if tc.fixed {
				want = fixed
			}
			if ref != want {
				t.Fatalf("selected %s, want %s", ref, want)
			}
		})
	}
}
