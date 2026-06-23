package office

import (
	"time"

	"github.com/orthodoxwest/office/internal/calendar"
)

// sundayNearestOctober1 returns the Sunday nearest the first day of October in
// the given civil year. When October 1 is within three days after a Sunday the
// preceding Sunday is nearer; otherwise the following Sunday is taken.
func sundayNearestOctober1(year int) time.Time {
	oct1 := time.Date(year, time.October, 1, 0, 0, 0, 0, time.UTC)
	wd := int(oct1.Weekday()) // Sunday = 0
	if wd <= 3 {
		return oct1.AddDate(0, 0, -wd)
	}
	return oct1.AddDate(0, 0, 7-wd)
}

// sundayLaudsHymnIsSummer reports whether the summer hymn is the appointed
// default for Sunday Lauds on date.
//
// Per the diurnal rubric, the winter hymn ("Aeterne rerum Conditor") is said
// from the octave of Epiphany through Quinquagesima Sunday, and from the Sunday
// nearest the first day of October until Advent; the summer hymn ("Ecce iam
// noctis") is said from the II Sunday after Trinity until the Sunday nearest
// October 1. Between Quinquagesima and the II Sunday after Trinity (Lent through
// the I Sunday after Trinity), and between Advent and the Epiphany octave, a
// seasonal or proper hymn governs, so the winter default is retained and this
// reports false.
func sundayLaudsHymnIsSummer(date time.Time) bool {
	md := calendar.ComputeMoveableDates(date.Year())
	secondAfterTrinity := md.TrinitySunday.AddDate(0, 0, 14)
	autumnSunday := sundayNearestOctober1(date.Year())
	return !date.Before(secondAfterTrinity) && date.Before(autumnSunday)
}
