package calendar

import "time"

// EmberSet is one of the four seasonal sets of Ember Days (Wed/Fri/Sat).
type EmberSet struct {
	Wed time.Time
	Fri time.Time
	Sat time.Time
}

// Tabula holds the "Tabula Temporaria" front-matter values the archdiocesan
// ordo prints for a year: the computus figures and the four Ember sets. The
// moveable feast dates themselves come from MoveableDates; this struct carries
// only the derived numbers that do not already live there.
type Tabula struct {
	Year                  int
	GoldenNumber          int
	DominicalLetter       string
	SundaysAfterEpiphany  int
	SundaysAfterPentecost int
	Spring                EmberSet // after the 1st Sunday of Lent
	Summer                EmberSet // after Pentecost (Whitsun)
	Autumn                EmberSet // after Holy Cross (Sept 14)
	Winter                EmberSet // after the 3rd Sunday of Advent
}

// ComputeTabula derives the Tabula Temporaria figures for a civil year. The
// paschalion is Julian (via ComputeMoveableDates); the computus figures
// (golden number, dominical letter) follow the civil Gregorian year, matching
// how the printed ordo lists them.
func ComputeTabula(year int) *Tabula {
	m := ComputeMoveableDates(year)

	return &Tabula{
		Year:                  year,
		GoldenNumber:          year%19 + 1,
		DominicalLetter:       dominicalLetter(year),
		SundaysAfterEpiphany:  countSundays(time.Date(year, 1, 6, 0, 0, 0, 0, time.UTC).AddDate(0, 0, 1), m.Septuagesima),
		SundaysAfterPentecost: countSundays(m.TrinitySunday, m.Advent1),
		Spring:                emberSet(firstWednesdayAfter(m.Lent1)),
		Summer:                emberSet(firstWednesdayAfter(m.Pentecost)),
		Autumn:                emberSet(firstWednesdayAfter(time.Date(year, 9, 14, 0, 0, 0, 0, time.UTC))),
		Winter:                emberSet(firstWednesdayAfter(m.Advent3)),
	}
}

// emberSet builds the Wed/Fri/Sat triple from the Ember Wednesday.
func emberSet(wed time.Time) EmberSet {
	return EmberSet{Wed: wed, Fri: wed.AddDate(0, 0, 2), Sat: wed.AddDate(0, 0, 3)}
}

// firstWednesdayAfter returns the first Wednesday strictly after d.
func firstWednesdayAfter(d time.Time) time.Time {
	offset := (int(time.Wednesday) - int(d.Weekday()) + 7) % 7
	if offset == 0 {
		offset = 7
	}
	return d.AddDate(0, 0, offset)
}

// countSundays counts the Sundays in the half-open interval [start, end).
func countSundays(start, end time.Time) int {
	// Advance to the first Sunday on or after start.
	offset := (int(time.Sunday) - int(start.Weekday()) + 7) % 7
	s := start.AddDate(0, 0, offset)
	n := 0
	for s.Before(end) {
		n++
		s = s.AddDate(0, 0, 7)
	}
	return n
}

// dominicalLetter returns the year's Sunday (dominical) letter. Letters A–G are
// assigned to dates from Jan 1 (A); the letter falling on Sundays is the
// dominical letter. In leap years Feb 29 carries no letter, shifting the
// March–December letter back by one; the ordo prints that (later) letter, so we
// compute the letter for a Sunday after March 1.
func dominicalLetter(year int) string {
	mar1 := time.Date(year, 3, 1, 0, 0, 0, 0, time.UTC)
	// First Sunday on or after March 1.
	offset := (int(time.Sunday) - int(mar1.Weekday()) + 7) % 7
	sunday := mar1.AddDate(0, 0, offset)
	// Day-of-year (1-based) of that Sunday.
	doy := sunday.YearDay()
	leap := isLeapYear(year)
	// Feb 29 (day 60 in a leap year) does not advance the letter.
	idx := doy - 1
	if leap {
		idx = doy - 2
	}
	idx = ((idx % 7) + 7) % 7
	return string(rune('A' + idx))
}

// Roman renders n as a Roman numeral (adequate for golden numbers 1–19 and
// small ordo counts).
func Roman(n int) string {
	if n <= 0 {
		return ""
	}
	vals := []int{1000, 900, 500, 400, 100, 90, 50, 40, 10, 9, 5, 4, 1}
	syms := []string{"M", "CM", "D", "CD", "C", "XC", "L", "XL", "X", "IX", "V", "IV", "I"}
	var b []byte
	for i, v := range vals {
		for n >= v {
			b = append(b, syms[i]...)
			n -= v
		}
	}
	return string(b)
}
