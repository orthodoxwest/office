package calendar

import (
	"testing"
	"time"
)

func TestComputeMoveableDates2026(t *testing.T) {
	m := ComputeMoveableDates(2026)

	// Easter 2026 (Julian paschalion) = April 12
	assertDate(t, "Easter", m.Easter, 2026, 4, 12)

	// Pre-Lent
	assertDate(t, "Septuagesima", m.Septuagesima, 2026, 2, 8)
	assertDate(t, "Sexagesima", m.Sexagesima, 2026, 2, 15)
	assertDate(t, "Quinquagesima", m.Quinquagesima, 2026, 2, 22)

	// Lent
	assertDate(t, "AshWednesday", m.AshWednesday, 2026, 2, 25)
	assertDate(t, "Lent1", m.Lent1, 2026, 3, 1)
	assertDate(t, "PalmSunday", m.PalmSunday, 2026, 4, 5)

	// Holy Week
	assertDate(t, "GoodFriday", m.GoodFriday, 2026, 4, 10)
	assertDate(t, "HolySaturday", m.HolySaturday, 2026, 4, 11)

	// Post-Easter
	assertDate(t, "Ascension", m.Ascension, 2026, 5, 21)
	assertDate(t, "Pentecost", m.Pentecost, 2026, 5, 31)
	assertDate(t, "TrinitySunday", m.TrinitySunday, 2026, 6, 7)
	assertDate(t, "CorpusChristi", m.CorpusChristi, 2026, 6, 11)

	// Advent 2026: Christmas is Friday, so Advent 4 = Dec 20
	assertDate(t, "Advent4", m.Advent4, 2026, 12, 20)
	assertDate(t, "Advent3", m.Advent3, 2026, 12, 13)
	assertDate(t, "Advent2", m.Advent2, 2026, 12, 6)
	assertDate(t, "Advent1", m.Advent1, 2026, 11, 29)
}

func TestAdventWhenChristmasIsSunday(t *testing.T) {
	// In 2022, Christmas is Sunday — Advent 4 should be Dec 18
	m := ComputeMoveableDates(2022)
	assertDate(t, "Advent4", m.Advent4, 2022, 12, 18)
	assertDate(t, "Advent1", m.Advent1, 2022, 11, 27)
}

func assertDate(t *testing.T, name string, got time.Time, year, month, day int) {
	t.Helper()
	want := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("%s = %v, want %v", name, got.Format("2006-01-02"), want.Format("2006-01-02"))
	}
}
