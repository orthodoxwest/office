package calendar

import (
	"time"

	"github.com/orthodoxwest/office/internal/models"
)

// DetermineSeason returns the liturgical season for a given date.
func DetermineSeason(date time.Time, moveable *MoveableDates) models.Season {
	year := date.Year()

	// Christmas: Dec 25-31
	if date.Month() == 12 && date.Day() >= 25 {
		return models.Christmas
	}

	// Christmas: Jan 1-5
	if date.Month() == 1 && date.Day() <= 5 {
		return models.Christmas
	}

	// Advent: Advent 1 to Dec 24
	dec24 := time.Date(year, 12, 24, 0, 0, 0, 0, time.UTC)
	if !date.Before(moveable.Advent1) && !date.After(dec24) {
		return models.Advent
	}

	// Passiontide: Passion Sunday to Holy Saturday
	if !date.Before(moveable.PassionSunday) && !date.After(moveable.HolySaturday) {
		return models.Passiontide
	}

	// Lent: Ash Wednesday to day before Passion Sunday
	if !date.Before(moveable.AshWednesday) && date.Before(moveable.PassionSunday) {
		return models.Lent
	}

	// Septuagesima: Septuagesima Sunday to day before Ash Wednesday
	if !date.Before(moveable.Septuagesima) && date.Before(moveable.AshWednesday) {
		return models.Septuagesima
	}

	// Easter: Easter Sunday to day before Pentecost
	if !date.Before(moveable.Easter) && date.Before(moveable.Pentecost) {
		return models.Easter
	}

	// Pentecost: Pentecost to day before Advent 1
	if !date.Before(moveable.Pentecost) && date.Before(moveable.Advent1) {
		return models.Pentecost
	}

	// Epiphany: Jan 6 to day before Septuagesima
	jan6 := time.Date(year, 1, 6, 0, 0, 0, 0, time.UTC)
	if !date.Before(jan6) && date.Before(moveable.Septuagesima) {
		return models.Epiphany
	}

	// Default (should not reach for valid dates)
	return models.Christmas
}

// DetermineMarianAntiphon returns the corpus subkey (under ordinary/marian/)
// for the Marian antiphon sung at Compline on the given date.
//
// Periods:
//
//	alma-redemptoris-advent:    day before Advent 1 through Dec 24
//	alma-redemptoris-christmas: Dec 25 through Feb 1
//	ave-regina-caelorum:        Feb 2 through Holy Wednesday
//	regina-caeli:               Holy Saturday through Saturday of Pentecost octave
//	salve-regina:               Trinity Sunday through 2 days before Advent 1
func DetermineMarianAntiphon(date time.Time, moveable *MoveableDates) string {
	year := date.Year()

	advent1 := moveable.Advent1
	dayBeforeAdvent1 := advent1.AddDate(0, 0, -1)
	dec24 := time.Date(year, 12, 24, 0, 0, 0, 0, time.UTC)
	dec25 := time.Date(year, 12, 25, 0, 0, 0, 0, time.UTC)
	feb2 := time.Date(year, 2, 2, 0, 0, 0, 0, time.UTC)
	holyWednesday := moveable.HolyWednesday
	holySaturday := moveable.HolySaturday
	pentecostOctaveSaturday := moveable.Pentecost.AddDate(0, 0, 6)
	trinitySunday := moveable.TrinitySunday
	twoDaysBeforeAdvent1 := advent1.AddDate(0, 0, -2)

	switch {
	case !date.Before(dayBeforeAdvent1) && !date.After(dec24):
		return "alma-redemptoris-advent"
	case !date.Before(dec25): // Dec 25–31
		return "alma-redemptoris-christmas"
	case date.Before(feb2): // Jan 1–Feb 1
		return "alma-redemptoris-christmas"
	case !date.Before(feb2) && !date.After(holyWednesday):
		return "ave-regina-caelorum"
	case !date.Before(holySaturday) && !date.After(pentecostOctaveSaturday):
		return "regina-caeli"
	case !date.Before(trinitySunday) && !date.After(twoDaysBeforeAdvent1):
		return "salve-regina"
	}

	return "salve-regina"
}

// SeasonColors maps each season to its default liturgical color.
var SeasonColors = map[models.Season]models.Color{
	models.Advent:       models.Violet,
	models.Christmas:    models.White,
	models.Epiphany:     models.Green,
	models.Septuagesima: models.Violet,
	models.Lent:         models.Violet,
	models.Passiontide:  models.Violet,
	models.Easter:       models.White,
	models.Pentecost:    models.Green,
}
