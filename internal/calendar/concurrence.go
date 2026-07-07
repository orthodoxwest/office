package calendar

import (
	"strings"

	"github.com/orthodoxwest/office/internal/models"
)

// sundaysFirstClass are the Greater Sundays of the I Class, which hold their
// Vespers in concurrence against everything below a I Class Double feast
// (Rubrics, "Greater Sundays"; the 2026 ordo gives St Tikhon's I Vespers
// precedence over Low Sunday, so a sanctoral I Class Double supersedes).
var sundaysFirstClass = map[string]bool{
	"advent-sunday-1": true,
	"lent-sunday-1":   true,
	"lent-sunday-2":   true,
	"lent-sunday-3":   true,
	"laetare-sunday":  true,
	"passion-sunday":  true,
	"palm-sunday":     true,
	"easter-sunday":   true,
	"low-sunday":      true,
	"pentecost":       true,
}

// hasFirstVespers returns true if the office begins at I Vespers.
// Ferias never do (XIII.18) — including the privileged ones modelled as
// feasts (Ash Wednesday, the Holy Week ferias, vigils; all Category feria)
// and the Triduum, whose Vespers follow the day's liturgy. The Saturday
// Office of the BVM does begin at I Vespers on a free Friday evening
// (2026 ordo: Friday "Vespers W / I of fol." throughout the year).
func hasFirstVespers(f *models.Feast) bool {
	if f == nil {
		return false
	}
	if f.Category == models.CategoryFeria {
		return false // XIII.18
	}
	switch f.ID {
	case "holy-thursday", "good-friday", "holy-saturday":
		return false
	case "saturday-office-bvm":
		return true
	}
	if f.Category == models.CategorySunday {
		return true // IV.7: Sundays always have both I and II Vespers
	}
	return f.Rank.Weight() >= models.Double.Weight()
}

// hasSecondVespers returns true if the feast has II Vespers.
// Simples have NO II Vespers (XIII.17). Ferias cannot concur (XIII.18).
func hasSecondVespers(f *models.Feast) bool {
	if f == nil {
		return false
	}
	if f.Category == models.CategorySunday {
		return true // IV.7
	}
	if f.Rank == models.Simple || f.Rank == models.Commemoration {
		return false // XIII.17
	}
	if f.Category == models.CategoryFeria {
		return false // XIII.18
	}
	return f.Rank.Weight() >= models.Double.Weight()
}

// isOctaveDay returns true if the feast ID ends with "-octave-day" (terminal octave day).
func isOctaveDay(f *models.Feast) bool {
	return strings.HasSuffix(f.ID, "-octave-day")
}

// isDayWithinOctave returns true if the feast is a non-terminal day within an octave.
func isDayWithinOctave(f *models.Feast) bool {
	// IDs like "epiphany-octave-day-2" through "epiphany-octave-day-7"
	idx := strings.LastIndex(f.ID, "-octave-day-")
	if idx < 0 {
		return false
	}
	suffix := f.ID[idx+len("-octave-day-"):]
	for _, c := range suffix {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(suffix) > 0
}

// isDoubleOrAbove returns true if the feast rank is Double or higher.
func isDoubleOrAbove(f *models.Feast) bool {
	return f.Rank.Weight() >= models.Double.Weight()
}

// isSaturdayBVM returns true if the feast is the Saturday Office of the BVM.
func isSaturdayBVM(f *models.Feast) bool {
	return f.ID == "saturday-office-bvm"
}

// isSunday returns true if the feast is a Sunday.
func isSunday(f *models.Feast) bool {
	return f.Category == models.CategorySunday
}

// concurrenceWinner determines which office wins vespers when II Vespers of
// prec concurs with I Vespers of fol. Returns VespersIIOfPreceding or
// VespersIOfFollowing.
//
// Implements Section XIII.2-17 of the monastic rubrics, with the parish
// ordo's resolutions where the rubrics leave room (equal-rank concurrence,
// possession against an incoming lesser Sunday).
func concurrenceWinner(prec, fol *models.Feast) models.VespersOwner {
	// 1. Greater Sundays of the I Class hold their Vespers against
	// everything except a sanctoral I Class Double feast (2026 ordo:
	// St Tikhon's I Vespers supersedes Low Sunday; temporal I Class days
	// like Easter Monday never displace them, and Easter and Pentecost
	// themselves yield to nothing).
	if sundaysFirstClass[prec.ID] {
		if fol.Rank == models.Double1stClass && !fol.IsTemporal() &&
			prec.ID != "easter-sunday" && prec.ID != "pentecost" {
			return models.VespersIOfFollowing
		}
		return models.VespersIIOfPreceding
	}
	if sundaysFirstClass[fol.ID] {
		if prec.Rank == models.Double1stClass && !prec.IsTemporal() &&
			fol.ID != "easter-sunday" && fol.ID != "pentecost" {
			return models.VespersIIOfPreceding
		}
		return models.VespersIOfFollowing
	}

	// 2. I or II Class Double feast vs any other Sunday — feast wins
	// (XIII.6; ordo: Chair of Peter over Quinquagesima, St Thomas over
	// IV Advent).
	if isSunday(prec) != isSunday(fol) {
		feast := prec
		feastWins := models.VespersIIOfPreceding
		sundayWins := models.VespersIOfFollowing
		if isSunday(prec) {
			feast = fol
			feastWins, sundayWins = sundayWins, feastWins
		}

		if feast.Rank.Weight() >= models.Double2ndClass.Weight() {
			return feastWins
		}
		// 3. Below II Class: the Sunday takes the concurrence (XIII.2-5;
		// the ordo applies this even to Greater Double feasts — the XV
		// Sunday keeps its II Vespers against the Exaltation, and Chains
		// of St Peter, the Name of Mary and the Presentation all yield
		// their II Vespers to the incoming Sunday). An Octave Day in
		// possession is the exception: it keeps its II Vespers (2026
		// ordo: Octave Day of the Assumption before the XII Sunday).
		if feast == prec && isOctaveDay(feast) {
			return feastWins
		}
		return sundayWins
	}

	// 5. Octave Day vs a Double below II Class — the Octave Day wins, in
	// both directions (XIII.10; ordo: Octave Day of the Epiphany over
	// St Hilary, Octave of John Baptist over the Commemoration of St Paul).
	if isOctaveDay(prec) != isOctaveDay(fol) {
		other := fol
		octaveWins := models.VespersIIOfPreceding
		otherWins := models.VespersIOfFollowing
		if isOctaveDay(fol) {
			other = prec
			octaveWins, otherWins = otherWins, octaveWins
		}
		if other.Rank.Weight() >= models.Double2ndClass.Weight() {
			return otherWins // XIII.10 excludes I/II Class Doubles
		}
		if isDoubleOrAbove(other) {
			return octaveWins
		}
	}

	// 6. Octave Day vs Octave Day — worthier wins (XIII.11)
	if isOctaveDay(prec) && isOctaveDay(fol) {
		if compareFeastPrecedence(prec, fol) {
			return models.VespersIIOfPreceding
		}
		return models.VespersIOfFollowing
	}

	// 7. Double vs day-within-Octave / Saturday BVM — Double wins (XIII.12,13)
	if isDoubleOrAbove(prec) && (isDayWithinOctave(fol) || isSaturdayBVM(fol)) {
		return models.VespersIIOfPreceding
	}
	if isDoubleOrAbove(fol) && (isDayWithinOctave(prec) || isSaturdayBVM(prec)) {
		return models.VespersIOfFollowing
	}

	// 8. Fallback: the worthier office takes the concurrence. When they are
	// equally worthy, II Class feasts keep the entire Vespers of the
	// preceding with a commemoration of the following (2026 ordo:
	// St Stephen/St John/Holy Innocents, Chair of Peter/St Matthias,
	// St James/St Anne), while lesser equals pass Vespers to the following
	// from the chapter (XIII.9), which the ordo marks "I of fol." and whose
	// Magnificat antiphon it quotes (St Chrysostom/St Cyril, St Augustine
	// of Canterbury/St Bede) — approximated here as I of the following.
	if compareFeastPrecedence(prec, fol) {
		return models.VespersIIOfPreceding
	}
	if !compareFeastPrecedence(fol, prec) && prec.Rank.Weight() >= models.Double2ndClass.Weight() {
		return models.VespersIIOfPreceding
	}
	return models.VespersIOfFollowing
}

// boundaryCommemorations collects the commemorations proper to an evening's
// Vespers: the celebration that lost the concurrence (XIII.2-17), followed by
// the following day's own commemorated feasts, whose observance begins with a
// commemoration at this Vespers (2026 ordo: Marcellus at St Maurus' II
// Vespers, Sabina on the eve of the Beheading). A nil loser (e.g. a plain
// feria with no Celebration of its own) contributes nothing.
//
// The concurrence loser is always commemorated (XIII.2-17: Jerome at
// St Michael's Vespers, the Octave Day of St John at the Holy Name's
// I Vespers). The following day's memorial-rank commemorations belong to the
// day that is beginning: at I Vespers of the following they are always kept
// (Hadrian at the Nativity BVM's I Vespers, Barnabas at Corpus Christi's),
// but the II Vespers of an outgoing Double of the II Class or above (Sundays
// excepted) does not admit them — the ordo prints "No Comm." at the Vespers
// of the Circumcision, the Purification, St Lawrence and St Matthew, and
// through the Easter and Pentecost octaves, while a privileged feria
// (Lent, Embertide, Advent) is still commemorated ("Comm. Fer." at
// St Joseph's and the Conception's Vespers). The winner and duplicates are
// filtered by finalizeCommemorations.
func boundaryCommemorations(winner, loser *models.Feast, following *models.CalendarDay, secondVespers bool) []*models.Feast {
	suppressIncoming := secondVespers && winner != nil &&
		winner.Rank.Weight() >= models.Double2ndClass.Weight() &&
		winner.Category != models.CategorySunday

	suppressed := func(c *models.Feast) bool {
		return suppressIncoming &&
			c.Rank.Weight() < models.Double2ndClass.Weight() &&
			c.Category != models.CategorySunday && c.Category != models.CategoryFeria
	}

	var comms []*models.Feast
	if loser != nil {
		// A loser that never had I Vespers rights (a simple octave day, a
		// day within an octave) is not a true concurrence party: it counts
		// as an incoming office and is subject to the same suppression
		// (the ordo's "No Comm." at the Circumcision's and the Epiphany's
		// II Vespers). A genuine concurrence loser is always commemorated.
		if hasFirstVespers(loser) || !suppressed(loser) {
			comms = append(comms, loser)
		}
	}
	for _, c := range following.Commemorations {
		if !suppressed(c) {
			comms = append(comms, c)
		}
	}
	return finalizeCommemorations(winner, comms)
}

// resolveConcurrence determines the vespers designation for an evening given
// the preceding day's celebration and the following day's celebration.
//
// A day's Celebration is nil for a plain feria, which has no vespers rights
// of its own (XIII.18) — that must not be confused with "no concurrence to
// resolve": the following day's I Vespers still applies in that case.
func resolveConcurrence(preceding, following *models.CalendarDay) models.VespersDesignation {
	precFeast := preceding.Celebration
	folFeast := following.Celebration

	precHasII := precFeast != nil && hasSecondVespers(precFeast)
	folHasI := folFeast != nil && hasFirstVespers(folFeast)

	// Neither has vespers — not applicable
	if !precHasII && !folHasI {
		return models.VespersDesignation{}
	}

	// If preceding has no II Vespers, following wins by default
	if !precHasII {
		return models.VespersDesignation{
			Owner:          models.VespersIOfFollowing,
			Feast:          folFeast,
			Color:          following.Color,
			Season:         following.Season,
			Commemorations: boundaryCommemorations(folFeast, precFeast, following, false),
		}
	}

	// If following has no I Vespers, preceding wins by default
	if !folHasI {
		return models.VespersDesignation{
			Owner:          models.VespersIIOfPreceding,
			Feast:          precFeast,
			Color:          preceding.Color,
			Season:         preceding.Season,
			Commemorations: boundaryCommemorations(precFeast, folFeast, following, true),
		}
	}

	// Both have vespers — resolve the concurrence
	winner := concurrenceWinner(precFeast, folFeast)
	if winner == models.VespersIIOfPreceding {
		return models.VespersDesignation{
			Owner:          models.VespersIIOfPreceding,
			Feast:          precFeast,
			Color:          preceding.Color,
			Season:         preceding.Season,
			Commemorations: boundaryCommemorations(precFeast, folFeast, following, true),
		}
	}
	return models.VespersDesignation{
		Owner:          models.VespersIOfFollowing,
		Feast:          folFeast,
		Color:          following.Color,
		Season:         following.Season,
		Commemorations: boundaryCommemorations(folFeast, precFeast, following, false),
	}
}

// resolveVespersConcurrence iterates through the calendar days and resolves
// vespers concurrence for each evening.
func resolveVespersConcurrence(days []models.CalendarDay) {
	for i := 0; i < len(days)-1; i++ {
		days[i].Vespers = resolveConcurrence(&days[i], &days[i+1])
	}

	// Dec 31 edge case: resolve against Jan 1 Circumcision, which is always
	// the same feast regardless of year.
	if len(days) > 0 {
		last := &days[len(days)-1]
		jan1 := &models.CalendarDay{
			Season: models.Christmas,
			Celebration: &models.Feast{
				ID:       "circumcision",
				Name:     "Circumcision of Our Lord",
				Rank:     models.Double2ndClass,
				Color:    models.White,
				Category: models.CategoryLord,
				Month:    1,
				Day:      1,
			},
			Color: models.White,
		}
		last.Vespers = resolveConcurrence(last, jan1)
	}
}
