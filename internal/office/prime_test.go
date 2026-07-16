package office

import (
	"testing"
	"time"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/texts"
)

func TestPrimeWeekdayCondition(t *testing.T) {
	composer := &PrimeComposer{}

	tests := []struct {
		name      string
		date      time.Time
		condition string
		want      bool
	}{
		{
			name:      "Sunday matches weekday-sunday",
			date:      time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC), // Sunday
			condition: "weekday-sunday",
			want:      true,
		},
		{
			name:      "Sunday does not match weekday-monday",
			date:      time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC), // Sunday
			condition: "weekday-monday",
			want:      false,
		},
		{
			name:      "Monday matches weekday-monday",
			date:      time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC), // Monday
			condition: "weekday-monday",
			want:      true,
		},
		{
			name:      "Tuesday matches weekday-tuesday",
			date:      time.Date(2026, 3, 17, 0, 0, 0, 0, time.UTC),
			condition: "weekday-tuesday",
			want:      true,
		},
		{
			name:      "Wednesday matches weekday-wednesday",
			date:      time.Date(2026, 3, 18, 0, 0, 0, 0, time.UTC),
			condition: "weekday-wednesday",
			want:      true,
		},
		{
			name:      "Thursday matches weekday-thursday",
			date:      time.Date(2026, 3, 19, 0, 0, 0, 0, time.UTC),
			condition: "weekday-thursday",
			want:      true,
		},
		{
			name:      "Friday matches weekday-friday",
			date:      time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC),
			condition: "weekday-friday",
			want:      true,
		},
		{
			name:      "Saturday matches weekday-saturday",
			date:      time.Date(2026, 3, 21, 0, 0, 0, 0, time.UTC),
			condition: "weekday-saturday",
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			day := &models.CalendarDay{Date: tt.date}
			got := evaluateCondition(tt.condition, day, composer.Moveable)
			if got != tt.want {
				t.Errorf("evaluateCondition(%q) = %v, want %v (weekday=%s)",
					tt.condition, got, tt.want, tt.date.Weekday())
			}
		})
	}
}

func TestPrimePreces(t *testing.T) {
	moveable := calendar.ComputeMoveableDates(2026)
	composer := &PrimeComposer{Moveable: moveable}

	tests := []struct {
		name string
		day  *models.CalendarDay
		want bool
	}{
		{
			name: "ferial day — preces",
			day:  makeDay(2026, 3, 16, nil, nil, ""), // Monday in Lent
			want: true,
		},
		{
			name: "double feast — no preces",
			day: makeDay(2026, 3, 15,
				&models.Feast{ID: "test", Rank: models.Double}, nil, ""),
			want: false,
		},
		{
			name: "Easter Sunday — no preces",
			day: makeDay(2026, 4, 12,
				&models.Feast{ID: "easter-sunday", Rank: models.Double1stClass}, nil, ""),
			want: false,
		},
		{
			name: "within octave — no preces",
			day:  makeDay(2026, 4, 13, nil, nil, "easter-sunday"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evaluateCondition("if-preces", tt.day, composer.Moveable)
			if got != tt.want {
				t.Errorf("evaluateCondition(if-preces) = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResolvePrimePsalmAntiphon(t *testing.T) {
	corpusTexts := map[string]string{
		"ordinary/prime/psalm-antiphon-1-monday":               "Monday",
		"ordinary/prime/psalm-antiphon-1-wednesday":            "Wednesday",
		"ordinary/prime/psalm-antiphon-1-friday":               "Friday",
		"proper/example-feast/psalm-antiphon-1":                "Feast proper",
		"proper/advent-sunday-1/psalm-antiphon-1":              "Advent I",
		"seasonal/advent/psalm-antiphon-1-prime-monday":        "Greater Advent feria",
		"seasonal/lent/psalm-antiphon-1-prime":                 "Lent",
		"seasonal/passiontide/psalm-antiphon-1-prime":          "Passion Week",
		"seasonal/easter/psalm-antiphon-1":                     "Paschaltide",
		"proper/holy-monday/psalm-antiphon-1":                  "Holy Monday",
		"proper/saturday-office-bvm/saturday-psalm-antiphon-1": "Saturday BVM",
	}
	corpus := texts.NewTestCorpus(corpusTexts)

	tests := []struct {
		name string
		day  *models.CalendarDay
		want string
		ref  string
	}{
		{
			name: "feast takes its first Lauds antiphon",
			day: &models.CalendarDay{
				Date:        time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC),
				Season:      models.Pentecost,
				Celebration: &models.Feast{ID: "example-feast", Category: models.CategoryConfessor},
			},
			want: "Feast proper",
			ref:  "proper/example-feast/psalm-antiphon-1",
		},
		{
			name: "Advent feria takes preceding Sunday first Lauds antiphon",
			day: &models.CalendarDay{
				Date:           time.Date(2026, 11, 30, 0, 0, 0, 0, time.UTC),
				Season:         models.Advent,
				TemporalWeekID: "advent-sunday-1",
			},
			want: "Advent I",
			ref:  "proper/advent-sunday-1/psalm-antiphon-1",
		},
		{
			name: "greater Advent feria takes its own Lauds antiphon",
			day: &models.CalendarDay{
				Date:           time.Date(2026, 12, 21, 0, 0, 0, 0, time.UTC),
				Season:         models.Advent,
				TemporalWeekID: "advent-sunday-4",
			},
			want: "Greater Advent feria",
			ref:  "seasonal/advent/psalm-antiphon-1-prime-monday",
		},
		{
			name: "Ash Wednesday through Saturday retain weekday form",
			day: &models.CalendarDay{
				Date:        time.Date(2026, 2, 25, 0, 0, 0, 0, time.UTC),
				Season:      models.Lent,
				Celebration: &models.Feast{ID: "ash-wednesday", Category: models.CategoryFeria},
			},
			want: "Wednesday",
			ref:  "ordinary/prime/psalm-antiphon-1-wednesday",
		},
		{
			name: "Lenten feria after Lent I takes fixed form",
			day:  &models.CalendarDay{Date: time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC), Season: models.Lent},
			want: "Lent",
			ref:  "seasonal/lent/psalm-antiphon-1-prime",
		},
		{
			name: "Passion Week takes fixed form",
			day:  &models.CalendarDay{Date: time.Date(2026, 3, 30, 0, 0, 0, 0, time.UTC), Season: models.Passiontide},
			want: "Passion Week",
			ref:  "seasonal/passiontide/psalm-antiphon-1-prime",
		},
		{
			name: "Holy Week feria takes its own first Lauds antiphon",
			day: &models.CalendarDay{
				Date:        time.Date(2026, 4, 6, 0, 0, 0, 0, time.UTC),
				Season:      models.Passiontide,
				Celebration: &models.Feast{ID: "holy-monday", Category: models.CategoryFeria},
			},
			want: "Holy Monday",
			ref:  "proper/holy-monday/psalm-antiphon-1",
		},
		{
			name: "Monday after Low Sunday begins Paschal form",
			day:  &models.CalendarDay{Date: time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC), Season: models.Easter},
			want: "Paschaltide",
			ref:  "seasonal/easter/psalm-antiphon-1",
		},
		{
			name: "after Ascension returns to weekday form",
			day:  &models.CalendarDay{Date: time.Date(2026, 5, 22, 0, 0, 0, 0, time.UTC), Season: models.Easter},
			want: "Friday",
			ref:  "ordinary/prime/psalm-antiphon-1-friday",
		},
		{
			name: "Saturday BVM outranks Paschaltide",
			day: &models.CalendarDay{
				Date:        time.Date(2026, 4, 18, 0, 0, 0, 0, time.UTC),
				Season:      models.Easter,
				Celebration: &models.Feast{ID: saturdayOfficeBVMID, Category: models.CategoryBlessedVirgin},
			},
			want: "Saturday BVM",
			ref:  "proper/saturday-office-bvm/saturday-psalm-antiphon-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolvePrimePsalmAntiphon(tt.day, corpus)
			if got.Text != tt.want || got.SourceRef != tt.ref {
				t.Fatalf("resolvePrimePsalmAntiphon() = (%q, %q), want (%q, %q)", got.Text, got.SourceRef, tt.want, tt.ref)
			}
			if got.Type != models.Antiphon || got.SlotRef != "psalm-antiphon-1" {
				t.Fatalf("resolved element metadata = (type %q, slot %q)", got.Type, got.SlotRef)
			}
		})
	}
}
