package calendar

import (
	"testing"
	"time"
)

func TestDetermineMarianAntiphon(t *testing.T) {
	moveable := ComputeMoveableDates(2026)

	date := func(month time.Month, day int) time.Time {
		return time.Date(2026, month, day, 0, 0, 0, 0, time.UTC)
	}

	tests := []struct {
		name string
		date time.Time
		want string
	}{
		// alma-redemptoris-advent: day before Advent 1 through Dec 24
		// 2026: Advent 1 = Nov 29, so day before = Nov 28
		{"day before Advent 1", date(11, 28), "alma-redemptoris-advent"},
		{"Advent 1", date(11, 29), "alma-redemptoris-advent"},
		{"mid Advent", date(12, 15), "alma-redemptoris-advent"},
		{"Dec 24", date(12, 24), "alma-redemptoris-advent"},

		// alma-redemptoris-christmas: Dec 25 through Feb 1
		{"Christmas Day", date(12, 25), "alma-redemptoris-christmas"},
		{"Dec 31", date(12, 31), "alma-redemptoris-christmas"},
		{"Jan 1", date(1, 1), "alma-redemptoris-christmas"},
		{"Jan 31", date(1, 31), "alma-redemptoris-christmas"},
		{"Feb 1", date(2, 1), "alma-redemptoris-christmas"},

		// ave-regina-caelorum: Feb 2 through Holy Wednesday
		// 2026: Easter = Apr 14 (Julian+13), Holy Wednesday = Apr 8
		{"Feb 2", date(2, 2), "ave-regina-caelorum"},
		{"mid Lent", date(3, 15), "ave-regina-caelorum"},
		{"Holy Wednesday", moveable.HolyWednesday, "ave-regina-caelorum"},

		// regina-caeli: Holy Saturday through Saturday of Pentecost octave
		// 2026: Holy Saturday = Apr 11, Pentecost = Jun 2 (Easter+49), octave Sat = Jun 8
		{"Holy Saturday", moveable.HolySaturday, "regina-caeli"},
		{"Easter Sunday", moveable.Easter, "regina-caeli"},
		{"Ascension", moveable.Ascension, "regina-caeli"},
		{"Pentecost Saturday", moveable.Pentecost.AddDate(0, 0, 6), "regina-caeli"},

		// salve-regina: Trinity Sunday through 2 days before Advent 1
		// Note: day before Trinity = Pentecost octave Saturday, which is still Regina Caeli.
		{"day before Trinity (= Pentecost oct Sat)", moveable.TrinitySunday.AddDate(0, 0, -1), "regina-caeli"},
		{"Trinity Sunday", moveable.TrinitySunday, "salve-regina"},
		{"mid summer", date(7, 15), "salve-regina"},
		{"Nov 1", date(11, 1), "salve-regina"},
		{"2 days before Advent 1", moveable.Advent1.AddDate(0, 0, -2), "salve-regina"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetermineMarianAntiphon(tt.date, moveable)
			if got != tt.want {
				t.Errorf("DetermineMarianAntiphon(%s) = %s, want %s",
					tt.date.Format("2006-01-02"), got, tt.want)
			}
		})
	}
}

func TestDetermineMarianAntiphonBoundaries2027(t *testing.T) {
	moveable := ComputeMoveableDates(2027)

	tests := []struct {
		name string
		date time.Time
		want string
	}{
		{"Feb 2", time.Date(2027, 2, 2, 0, 0, 0, 0, time.UTC), "ave-regina-caelorum"},
		{"Holy Saturday", moveable.HolySaturday, "regina-caeli"},
		{"Pentecost octave Sat", moveable.Pentecost.AddDate(0, 0, 6), "regina-caeli"},
		{"day before Trinity (= Pentecost oct Sat)", moveable.TrinitySunday.AddDate(0, 0, -1), "regina-caeli"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetermineMarianAntiphon(tt.date, moveable)
			if got != tt.want {
				t.Errorf("DetermineMarianAntiphon(%s) = %s, want %s",
					tt.date.Format("2006-01-02"), got, tt.want)
			}
		})
	}
}
