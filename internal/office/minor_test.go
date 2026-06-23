package office

import (
	"testing"
	"time"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
)

func TestMinorHourWeekdayCondition(t *testing.T) {
	composer := &MinorHourComposer{Name: "Terce"}

	tests := []struct {
		name      string
		date      time.Time
		condition string
		want      bool
	}{
		{"Sunday matches weekday-sunday", time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC), "weekday-sunday", true},
		{"Sunday does not match weekday-monday", time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC), "weekday-monday", false},
		{"Tuesday matches weekday-tuesday", time.Date(2026, 3, 17, 0, 0, 0, 0, time.UTC), "weekday-tuesday", true},
		{"Saturday matches weekday-saturday", time.Date(2026, 3, 21, 0, 0, 0, 0, time.UTC), "weekday-saturday", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			day := &models.CalendarDay{Date: tt.date}
			got := evaluateCondition(tt.condition, day, composer.Moveable)
			if got != tt.want {
				t.Errorf("evaluateCondition(%q) = %v, want %v", tt.condition, got, tt.want)
			}
		})
	}
}

func TestMinorHourPreces(t *testing.T) {
	moveable := calendar.ComputeMoveableDates(2026)
	composer := &MinorHourComposer{Name: "Sext", Moveable: moveable}

	tests := []struct {
		name string
		day  *models.CalendarDay
		want bool
	}{
		{
			name: "ferial day — preces",
			day:  makeDay(2026, 3, 16, nil, nil, ""),
			want: true,
		},
		{
			name: "double feast — no preces",
			day:  makeDay(2026, 3, 15, &models.Feast{ID: "test", Rank: models.Double}, nil, ""),
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
