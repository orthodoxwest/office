package calendar

import "time"

// MoveableDates holds all moveable dates for a liturgical year.
type MoveableDates struct {
	// Pre-Lent
	Septuagesima  time.Time
	Sexagesima    time.Time
	Quinquagesima time.Time

	// Lent
	AshWednesday  time.Time
	Lent1         time.Time
	Lent2         time.Time
	Lent3         time.Time
	Lent4         time.Time // Laetare Sunday
	PassionSunday time.Time
	PalmSunday    time.Time

	// Holy Week
	HolyMonday    time.Time
	HolyTuesday   time.Time
	HolyWednesday time.Time
	HolyThursday  time.Time
	GoodFriday    time.Time
	HolySaturday  time.Time

	// Easter
	Easter        time.Time
	EasterMonday  time.Time
	EasterTuesday time.Time

	// Post-Easter
	LowSunday     time.Time
	Ascension     time.Time
	Pentecost     time.Time
	TrinitySunday time.Time
	CorpusChristi time.Time

	// Advent
	Advent1 time.Time
	Advent2 time.Time
	Advent3 time.Time
	Advent4 time.Time
}

// ComputeMoveableDates computes all moveable dates for the given civil year.
func ComputeMoveableDates(year int) *MoveableDates {
	easter := JulianEaster(year)
	offset := func(days int) time.Time {
		return easter.AddDate(0, 0, days)
	}

	// Advent: 4th Sunday of Advent is the last Sunday before Christmas.
	christmas := time.Date(year, 12, 25, 0, 0, 0, 0, time.UTC)
	// Find the Sunday on or before Dec 24
	// time.Weekday: Sunday=0, Monday=1, ..., Saturday=6
	// Python: Monday=0 ... Sunday=6
	// Christmas weekday in Go: Sunday=0
	wd := christmas.Weekday()
	var advent4 time.Time
	if wd == time.Sunday {
		// Christmas is Sunday — 4th Advent Sunday is the week before
		advent4 = christmas.AddDate(0, 0, -7)
	} else {
		// Go back to the preceding Sunday
		// wd gives days since Sunday, so preceding Sunday is wd days back
		advent4 = christmas.AddDate(0, 0, -int(wd))
	}
	advent3 := advent4.AddDate(0, 0, -7)
	advent2 := advent4.AddDate(0, 0, -14)
	advent1 := advent4.AddDate(0, 0, -21)

	return &MoveableDates{
		Septuagesima:  offset(-63),
		Sexagesima:    offset(-56),
		Quinquagesima: offset(-49),

		AshWednesday:  offset(-46),
		Lent1:         offset(-42),
		Lent2:         offset(-35),
		Lent3:         offset(-28),
		Lent4:         offset(-21),
		PassionSunday: offset(-14),
		PalmSunday:    offset(-7),

		HolyMonday:    offset(-6),
		HolyTuesday:   offset(-5),
		HolyWednesday: offset(-4),
		HolyThursday:  offset(-3),
		GoodFriday:    offset(-2),
		HolySaturday:  offset(-1),

		Easter:        easter,
		EasterMonday:  offset(1),
		EasterTuesday: offset(2),

		LowSunday:     offset(7),
		Ascension:     offset(39),
		Pentecost:     offset(49),
		TrinitySunday: offset(56),
		CorpusChristi: offset(60),

		Advent1: advent1,
		Advent2: advent2,
		Advent3: advent3,
		Advent4: advent4,
	}
}
