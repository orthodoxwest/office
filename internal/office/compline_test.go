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
