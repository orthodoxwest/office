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

func TestDateFixedAdventBenedictusAppointment(t *testing.T) {
	corpus := texts.NewTestCorpus(map[string]string{
		"seasonal/advent/benedictus-antiphon-december-21": "Be not afraid",
		"seasonal/advent/benedictus-antiphon-december-23": "Behold all things are fulfilled",
		"proper/test/benedictus-antiphon":                 "Own antiphon",
		"proper/test/magnificat-antiphon":                 "Own Magnificat",
	})
	for _, tc := range []struct {
		name, date, hour, slot string
		season                 models.Season
		category               models.FeastCategory
		fixed                  bool
	}{
		{"December 21 Advent Sunday", "2025-12-21", "lauds", "benedictus-antiphon", models.Advent, models.CategorySunday, true},
		{"December 21 feria", "2026-12-21", "lauds", "benedictus-antiphon", models.Advent, models.CategoryFeria, true},
		{"December 21 saint retains its office", "2026-12-21", "lauds", "benedictus-antiphon", models.Advent, models.CategoryApostle, false},
		{"December 22 feria", "2026-12-22", "lauds", "benedictus-antiphon", models.Advent, models.CategoryFeria, false},
		{"Wednesday feria", "2026-12-23", "lauds", "benedictus-antiphon", models.Advent, models.CategoryFeria, true},
		{"Thursday feria", "2027-12-23", "lauds", "benedictus-antiphon", models.Advent, models.CategoryFeria, true},
		{"Advent Sunday", "2029-12-23", "lauds", "benedictus-antiphon", models.Advent, models.CategorySunday, true},
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
				want = "seasonal/advent/benedictus-antiphon-december-" + tc.date[8:]
			}
			if ref != want {
				t.Fatalf("selected %s, want %s", ref, want)
			}
		})
	}
}

func TestAdventDateCommemorationAntiphon(t *testing.T) {
	corpus := texts.NewTestCorpus(map[string]string{
		"seasonal/advent/magnificat-antiphon-december-17": "O Wisdom",
		"seasonal/advent/magnificat-antiphon-december-20": "O Key of David",
		"seasonal/advent/magnificat-antiphon-december-21": "O Day-spring",
		"seasonal/advent/benedictus-antiphon-december-21": "Be not afraid",
		"seasonal/advent/benedictus-antiphon-december-23": "Behold all things are fulfilled",
	})
	for _, tc := range []struct {
		name, date, hour, ref string
		firstVespers          bool
		season                models.Season
		category              models.FeastCategory
		want                  string
	}{
		// 2026 ordo: Dec 21 II Vespers of St Thomas, "Comm. Sun. ('O Day-Spring')".
		{"II Vespers takes the evening's O antiphon", "2026-12-21", "vespers", "commemoration-antiphon", false, models.Advent, models.CategoryFeria, "seasonal/advent/magnificat-antiphon-december-21"},
		// 2026 ordo: Dec 20 I Vespers of St Thomas, "Comm. Fer. ('O Key of David')".
		{"I Vespers steps back to the evening's date", "2026-12-21", "vespers", "commemoration-antiphon", true, models.Advent, models.CategorySunday, "seasonal/advent/magnificat-antiphon-december-20"},
		{"first O antiphon", "2029-12-17", "vespers", "commemoration-antiphon", false, models.Advent, models.CategoryFeria, "seasonal/advent/magnificat-antiphon-december-17"},
		{"before the O antiphons", "2026-12-16", "vespers", "commemoration-antiphon", false, models.Advent, models.CategoryFeria, ""},
		{"I Vespers of December 17", "2026-12-17", "vespers", "commemoration-antiphon", true, models.Advent, models.CategoryFeria, ""},
		// Diurnal p. 176; 2024 and 2026 ordos, Dec 21 Lauds "Comm. Fer. ('Be not afraid')".
		{"December 21 Lauds", "2026-12-21", "lauds", "commemoration-antiphon", false, models.Advent, models.CategoryFeria, "seasonal/advent/benedictus-antiphon-december-21"},
		{"December 21 Lauds of an Ember day", "2024-12-21", "lauds", "commemoration-antiphon", false, models.Advent, models.CategoryFeria, "seasonal/advent/benedictus-antiphon-december-21"},
		{"December 23 Lauds", "2026-12-23", "lauds", "commemoration-antiphon", false, models.Advent, models.CategoryFeria, "seasonal/advent/benedictus-antiphon-december-23"},
		{"other Lauds dates keep the weekday antiphon", "2026-12-22", "lauds", "commemoration-antiphon", false, models.Advent, models.CategoryFeria, ""},
		{"a saint's commemoration keeps its own antiphon", "2026-12-21", "vespers", "commemoration-antiphon", false, models.Advent, models.CategoryMartyr, ""},
		{"only the antiphon slot", "2026-12-21", "vespers", "commemoration-versicle", false, models.Advent, models.CategoryFeria, ""},
		{"other hours", "2026-12-21", "prime", "commemoration-antiphon", false, models.Advent, models.CategoryFeria, ""},
		{"other seasons", "2026-12-21", "vespers", "commemoration-antiphon", false, models.Christmas, models.CategoryFeria, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			date, err := time.Parse("2006-01-02", tc.date)
			if err != nil {
				t.Fatal(err)
			}
			day := &models.CalendarDay{Date: date, Season: tc.season, FirstVespers: tc.firstVespers}
			comm := &models.Feast{ID: "comm", Category: tc.category}
			_, ref := adventDateCommemorationAntiphon(day, comm, tc.hour, tc.ref, corpus)
			if tc.want == "" {
				if text, _ := adventDateCommemorationAntiphon(day, comm, tc.hour, tc.ref, corpus); text != "" {
					t.Fatalf("selected %s, want no date-fixed antiphon", ref)
				}
				return
			}
			if ref != tc.want {
				t.Fatalf("selected %s, want %s", ref, tc.want)
			}
		})
	}
}
