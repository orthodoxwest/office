package calendar

import (
	"testing"
	"time"

	"github.com/orthodoxwest/office/internal/models"
)

func TestDetermineSeason2026(t *testing.T) {
	m := ComputeMoveableDates(2026)

	tests := []struct {
		date   time.Time
		season models.Season
	}{
		// Christmas (Jan 1-5)
		{time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), models.Christmas},
		{time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC), models.Christmas},
		// Epiphany
		{time.Date(2026, 1, 6, 0, 0, 0, 0, time.UTC), models.Epiphany},
		{time.Date(2026, 2, 7, 0, 0, 0, 0, time.UTC), models.Epiphany},
		// Septuagesima (Feb 8)
		{time.Date(2026, 2, 8, 0, 0, 0, 0, time.UTC), models.Septuagesima},
		{time.Date(2026, 2, 24, 0, 0, 0, 0, time.UTC), models.Septuagesima},
		// Lent (Feb 25 = Ash Wednesday)
		{time.Date(2026, 2, 25, 0, 0, 0, 0, time.UTC), models.Lent},
		{time.Date(2026, 3, 28, 0, 0, 0, 0, time.UTC), models.Lent},
		// Passiontide (Mar 29 = Passion Sunday)
		{time.Date(2026, 3, 29, 0, 0, 0, 0, time.UTC), models.Passiontide},
		{time.Date(2026, 4, 11, 0, 0, 0, 0, time.UTC), models.Passiontide},
		// Easter (Apr 12)
		{time.Date(2026, 4, 12, 0, 0, 0, 0, time.UTC), models.Easter},
		{time.Date(2026, 5, 30, 0, 0, 0, 0, time.UTC), models.Easter},
		// Pentecost (May 31)
		{time.Date(2026, 5, 31, 0, 0, 0, 0, time.UTC), models.Pentecost},
		{time.Date(2026, 11, 28, 0, 0, 0, 0, time.UTC), models.Pentecost},
		// Advent (Nov 29)
		{time.Date(2026, 11, 29, 0, 0, 0, 0, time.UTC), models.Advent},
		{time.Date(2026, 12, 24, 0, 0, 0, 0, time.UTC), models.Advent},
		// Christmas (Dec 25-31)
		{time.Date(2026, 12, 25, 0, 0, 0, 0, time.UTC), models.Christmas},
		{time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC), models.Christmas},
	}

	for _, tt := range tests {
		got := DetermineSeason(tt.date, m)
		if got != tt.season {
			t.Errorf("DetermineSeason(%s) = %v, want %v", tt.date.Format("2006-01-02"), got, tt.season)
		}
	}
}
