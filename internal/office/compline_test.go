package office

import (
	"testing"
	"time"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
)

func makeDay(year int, month time.Month, day int, celebration *models.Feast, comms []*models.Feast, withinOctave string) *models.CalendarDay {
	return &models.CalendarDay{
		Date:           time.Date(year, month, day, 0, 0, 0, 0, time.UTC),
		Season:         models.Lent,
		Celebration:    celebration,
		Commemorations: comms,
		Color:          models.Violet,
		WithinOctaveOf: withinOctave,
	}
}

func TestShouldSayPreces(t *testing.T) {
	moveable := calendar.ComputeMoveableDates(2026)

	tests := []struct {
		name string
		day  *models.CalendarDay
		want bool
	}{
		{
			name: "ferial day — preces",
			day:  makeDay(2026, 3, 11, nil, nil, ""),
			want: true,
		},
		{
			name: "simple feast — preces",
			day: makeDay(2026, 3, 11,
				&models.Feast{ID: "test", Rank: models.Simple}, nil, ""),
			want: true,
		},
		{
			name: "semi-double feast — preces",
			day: makeDay(2026, 3, 15,
				&models.Feast{ID: "test", Rank: models.SemiDouble}, nil, ""),
			want: true,
		},
		{
			name: "double feast — no preces",
			day: makeDay(2026, 3, 15,
				&models.Feast{ID: "test", Rank: models.Double}, nil, ""),
			want: false,
		},
		{
			name: "1st class feast — no preces",
			day: makeDay(2026, 4, 12,
				&models.Feast{ID: "easter-sunday", Rank: models.Double1stClass}, nil, ""),
			want: false,
		},
		{
			name: "Sunday with elevated precedence rank — preces",
			day: makeDay(2026, 3, 15,
				&models.Feast{ID: "lent-sunday-3", Rank: models.Double1stClass, Category: models.CategorySunday}, nil, ""),
			want: true,
		},
		{
			name: "feria with elevated precedence rank — preces",
			day: makeDay(2026, 2, 25,
				&models.Feast{ID: "ash-wednesday", Rank: models.Double1stClass, Category: models.CategoryFeria}, nil, ""),
			want: true,
		},
		{
			name: "elevated Sunday with double commemoration — no preces",
			day: makeDay(2026, 3, 15,
				&models.Feast{ID: "lent-sunday-3", Rank: models.Double1stClass, Category: models.CategorySunday},
				[]*models.Feast{{ID: "test", Rank: models.Double}}, ""),
			want: false,
		},
		{
			name: "Vigil of Pentecost is a Double feria — no preces",
			day: makeDay(2026, 5, 30,
				&models.Feast{ID: "vigil-pentecost", Rank: models.Double1stClass, Category: models.CategoryFeria}, nil, ""),
			want: false,
		},
		{
			name: "Vigil of the Nativity is a Double feria — no preces",
			day: makeDay(2026, 12, 24,
				&models.Feast{ID: "vigil-nativity", Rank: models.Double1stClass, Category: models.CategoryFeria}, nil, ""),
			want: false,
		},
		{
			name: "All Souls is a Double feria — no preces",
			day: makeDay(2026, 11, 2,
				&models.Feast{ID: "all-souls", Rank: models.Double, Category: models.CategoryFeria}, nil, ""),
			want: false,
		},
		{
			name: "within octave — no preces",
			day:  makeDay(2026, 4, 13, nil, nil, "easter-sunday"),
			want: false,
		},
		{
			name: "simple octave-day office (Jan 2, Octave Day of St Stephen) — no preces",
			day: makeDay(2026, 1, 2,
				&models.Feast{ID: "octave-day-st-stephen", Rank: models.Simple}, nil, ""),
			want: false,
		},
		{
			name: "double commemoration — no preces",
			day: makeDay(2026, 3, 11, nil,
				[]*models.Feast{{ID: "test", Rank: models.Double}}, ""),
			want: false,
		},
		{
			name: "octave commemoration — no preces",
			day: makeDay(2026, 3, 11, nil,
				[]*models.Feast{{ID: "test-octave-day-3", Rank: models.SemiDouble}}, ""),
			want: false,
		},
		{
			name: "vigil of Epiphany (Jan 5) — no preces",
			day:  makeDay(2026, 1, 5, nil, nil, ""),
			want: false,
		},
		{
			name: "Friday after Ascension octave — no preces",
			day: makeDay(2026, 5, 29, // Easter+47 = Apr 12 + 47 = May 29
				nil, nil, ""),
			want: false,
		},
		{
			name: "Eastertide Sunday (IV after Easter) — no preces",
			day: func() *models.CalendarDay {
				d := makeDay(2026, 5, 10,
					&models.Feast{ID: "easter-sunday-4", Rank: models.SemiDouble, Category: models.CategorySunday},
					nil, "")
				d.Season = models.Easter
				return d
			}(),
			want: false,
		},
		{
			name: "Eastertide feria — preces",
			day: func() *models.CalendarDay {
				d := makeDay(2026, 5, 11, nil, nil, "")
				d.Season = models.Easter
				return d
			}(),
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldSayPreces(tt.day, moveable)
			if got != tt.want {
				t.Errorf("shouldSayPreces() = %v, want %v", got, tt.want)
			}
		})
	}
}
