package calendar

import (
	"testing"
	"time"
)

func TestJulianEaster(t *testing.T) {
	// Known Julian Easter dates (Gregorian calendar)
	tests := []struct {
		year int
		want time.Time
	}{
		{2024, time.Date(2024, 5, 5, 0, 0, 0, 0, time.UTC)},
		{2025, time.Date(2025, 4, 20, 0, 0, 0, 0, time.UTC)},
		{2026, time.Date(2026, 4, 12, 0, 0, 0, 0, time.UTC)},
		{2027, time.Date(2027, 5, 2, 0, 0, 0, 0, time.UTC)},
		{2028, time.Date(2028, 4, 16, 0, 0, 0, 0, time.UTC)},
		{2029, time.Date(2029, 4, 8, 0, 0, 0, 0, time.UTC)},
		{2030, time.Date(2030, 4, 28, 0, 0, 0, 0, time.UTC)},
	}

	for _, tt := range tests {
		got := JulianEaster(tt.year)
		if !got.Equal(tt.want) {
			t.Errorf("JulianEaster(%d) = %v, want %v", tt.year, got, tt.want)
		}
	}
}

func TestJulianEasterAlwaysSunday(t *testing.T) {
	for year := 2020; year <= 2050; year++ {
		easter := JulianEaster(year)
		if easter.Weekday() != time.Sunday {
			t.Errorf("JulianEaster(%d) = %v (%s), want Sunday", year, easter, easter.Weekday())
		}
	}
}
