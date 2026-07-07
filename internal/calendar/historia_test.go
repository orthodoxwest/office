package calendar

import (
	"testing"
	"time"
)

func d(y int, m time.Month, day int) time.Time {
	return time.Date(y, m, day, 0, 0, 0, 0, time.UTC)
}

func TestHistoriaWeekID(t *testing.T) {
	cases := []struct {
		date time.Time
		want string
		why  string
	}{
		// 2026: Aug 1 is a Saturday, so the August scripture-month begins
		// Sunday Aug 2; the Aug 1 evening Magnificat is "Wisdom hath
		// builded" (august-1) per the 2026 ordo.
		{d(2026, time.August, 2), "august-1", "first Sunday of August 2026"},
		{d(2026, time.August, 1), "", "Saturday before the August month begins"},
		{d(2026, time.July, 26), "", "July: the summer Kings historia, no month file"},
		// Sep 1 2026 is a Tuesday: September begins Sunday Aug 30.
		{d(2026, time.August, 30), "september-1", "Sunday nearest Sep 1"},
		{d(2026, time.September, 6), "september-2", "second September Sunday"},
		// Oct 1 2026 is a Thursday: October defers to Sunday Oct 4
		// (2026 ordo: Oct 3 evening Magnificat "The Lord open your hearts").
		{d(2026, time.September, 27), "september-5", "Thursday first defers October"},
		{d(2026, time.October, 4), "october-1", "first Sunday of October 2026"},
		// November weeks anchor backwards from Advent (Nov 29 in 2026): the
		// last week before Advent reads week 5 (2026 ordo: Nov 21 evening
		// Magnificat "I have set watchmen").
		{d(2026, time.November, 1), "november-1", "first Sunday of November 2026"},
		{d(2026, time.November, 8), "november-3", "Advent-anchored: II week vanishes in 2026"},
		{d(2026, time.November, 15), "november-4", "Advent-anchored"},
		{d(2026, time.November, 22), "november-5", "last week before Advent"},
		{d(2026, time.November, 29), "", "Advent 1"},
		{d(2026, time.December, 6), "", "Advent"},
		// Weekdays resolve to their week's governing Sunday; a Saturday
		// still belongs to the outgoing week (I Vespers passes the
		// following Sunday's date instead).
		{d(2026, time.October, 7), "october-1", "weekday follows its Sunday"},
		{d(2026, time.January, 10), "", "winter"},
	}
	for _, c := range cases {
		if got := HistoriaWeekID(c.date); got != c.want {
			t.Errorf("HistoriaWeekID(%s) = %q, want %q (%s)",
				c.date.Format("2006-01-02"), got, c.want, c.why)
		}
	}
}
