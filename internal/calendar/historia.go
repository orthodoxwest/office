package calendar

import (
	"strconv"
	"time"
)

var historiaMonthNames = map[int]string{
	8:  "august",
	9:  "september",
	10: "october",
	11: "november",
}

// HistoriaWeekID returns the scripture-cycle ("historia") month-week that
// governs the week containing date, as "august-1" through "november-5", or
// "" when no month historia applies: before the first liturgical Sunday of
// August (the summer Kings antiphons ride the numbered Pentecost weeks) or
// from the First Sunday of Advent onward.
//
// The rule follows the traditional breviary as implemented by Divinum
// Officium's monthday(): a liturgical month begins on the Sunday nearest
// the first of the calendar month (a first falling Thursday to Saturday
// defers to the following Sunday), and its weeks count from that Sunday —
// except November, whose weeks after the first are anchored backwards from
// Advent so that the last week before Advent always reads week 5.
func HistoriaWeekID(date time.Time) string {
	date = time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
	// The governing Sunday of date's week.
	sunday := date.AddDate(0, 0, -int(date.Weekday()))
	year := sunday.Year()

	advent1 := adventSunday(year)
	if !sunday.Before(advent1) {
		return ""
	}

	litMonth := 0
	var litStart time.Time
	for m := 8; m <= 11; m++ {
		first := time.Date(year, time.Month(m), 1, 0, 0, 0, 0, time.UTC)
		start := first.AddDate(0, 0, -int(first.Weekday()))
		if first.Weekday() >= time.Thursday {
			start = start.AddDate(0, 0, 7)
		}
		if sunday.Before(start) {
			break
		}
		litMonth, litStart = m, start
	}
	if litMonth == 0 {
		return ""
	}

	week := int(sunday.Sub(litStart).Hours()) / (24 * 7)
	if litMonth == 11 && week > 0 {
		// Count back from Advent: the week ending at Advent is the fifth.
		week = 4 - int(advent1.Sub(sunday).Hours()-24)/(24*7)
	}
	return historiaMonthNames[litMonth] + "-" + strconv.Itoa(week+1)
}

// adventSunday returns the First Sunday of Advent (the Sunday nearest the
// feast of St Andrew, November 30) for the given year.
func adventSunday(year int) time.Time {
	andrew := time.Date(year, time.November, 30, 0, 0, 0, 0, time.UTC)
	w := int(andrew.Weekday())
	if w <= 3 {
		return andrew.AddDate(0, 0, -w)
	}
	return andrew.AddDate(0, 0, 7-w)
}
