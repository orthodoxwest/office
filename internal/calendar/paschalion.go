package calendar

import "time"

// JulianEaster computes the date of Pascha (Easter) using the Julian paschalion.
// Uses the Meeus Julian algorithm to find Easter in the Julian calendar,
// then converts to the proleptic Gregorian calendar.
func JulianEaster(year int) time.Time {
	a := year % 4
	b := year % 7
	c := year % 19
	d := (19*c + 15) % 30
	e := (2*a + 4*b - d + 34) % 7
	month := (d + e + 114) / 31 // 3 = March, 4 = April (Julian)
	day := ((d + e + 114) % 31) + 1

	// Julian date, then convert to Gregorian
	julian := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	offset := julianToGregorianOffset(year)
	return julian.AddDate(0, 0, offset)
}

// julianToGregorianOffset returns the number of days to add to convert
// a Julian date to Gregorian. For 1901-2099: +13 days.
func julianToGregorianOffset(year int) int {
	century := year / 100
	return century - century/4 - 2
}
