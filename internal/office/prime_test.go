package office

import (
	"testing"
	"time"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
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
