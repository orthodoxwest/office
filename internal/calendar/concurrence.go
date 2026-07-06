package calendar

import (
	"strings"

	"github.com/orthodoxwest/office/internal/models"
)

// hasFirstVespers returns true if the feast has I Vespers (Double+ or Sunday).
func hasFirstVespers(f *models.Feast) bool {
	if f == nil {
		return false
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

// isGreaterSunday returns true if f is a Sunday with rank >= Double2ndClass.
func isGreaterSunday(f *models.Feast) bool {
	return f.Category == models.CategorySunday && f.Rank.Weight() >= models.Double2ndClass.Weight()
}

// isLesserSunday returns true if f is a Sunday with rank < Double2ndClass.
func isLesserSunday(f *models.Feast) bool {
	return f.Category == models.CategorySunday && f.Rank.Weight() < models.Double2ndClass.Weight()
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

// isLordFeast returns true if the feast has category "lord".
func isLordFeast(f *models.Feast) bool {
	return f.Category == models.CategoryLord
}

// concurrenceWinner determines which office wins vespers when II Vespers of
// prec concurs with I Vespers of fol. Returns VespersIIOfPreceding or
// VespersIOfFollowing.
//
// Implements Section XIII.2-17 of the monastic rubrics.
func concurrenceWinner(prec, fol *models.Feast) models.VespersOwner {
	// 1. Privileged days always win
	if privilegedFeastIDs[prec.ID] {
		return models.VespersIIOfPreceding
	}
	if privilegedFeastIDs[fol.ID] {
		return models.VespersIOfFollowing
	}

	// 2-3. I Class Double vs any Sunday — feast wins (XIII.6, IV.1)
	if prec.Rank == models.Double1stClass && !isSunday(prec) && isSunday(fol) {
		return models.VespersIIOfPreceding
	}
	if fol.Rank == models.Double1stClass && !isSunday(fol) && isSunday(prec) {
		return models.VespersIOfFollowing
	}

	// 4. II Class Double vs Greater Sunday — Sunday wins (XIII.6)
	if prec.Rank == models.Double2ndClass && !isSunday(prec) && isGreaterSunday(fol) {
		return models.VespersIOfFollowing
	}
	if fol.Rank == models.Double2ndClass && !isSunday(fol) && isGreaterSunday(prec) {
		return models.VespersIIOfPreceding
	}

	// 5. II Class Double vs Lesser Sunday — feast wins (XIII.6)
	if prec.Rank == models.Double2ndClass && !isSunday(prec) && isLesserSunday(fol) {
		return models.VespersIIOfPreceding
	}
	if fol.Rank == models.Double2ndClass && !isSunday(fol) && isLesserSunday(prec) {
		return models.VespersIOfFollowing
	}

	// 6. XII-Lesson Feast of Lord (Double+ Lord) vs Lesser Sunday — feast wins (XIII.7)
	if isDoubleOrAbove(prec) && isLordFeast(prec) && isLesserSunday(fol) {
		return models.VespersIIOfPreceding
	}
	if isDoubleOrAbove(fol) && isLordFeast(fol) && isLesserSunday(prec) {
		return models.VespersIOfFollowing
	}

	// 7. Non-I/II-Class Double vs any Sunday — Sunday wins (XIII.3,4)
	if isDoubleOrAbove(prec) && !isSunday(prec) &&
		prec.Rank.Weight() < models.Double2ndClass.Weight() && isSunday(fol) {
		return models.VespersIOfFollowing
	}
	if isDoubleOrAbove(fol) && !isSunday(fol) &&
		fol.Rank.Weight() < models.Double2ndClass.Weight() && isSunday(prec) {
		return models.VespersIIOfPreceding
	}

	// 8. Sunday vs non-Lord feast — Sunday wins; Feast of Lord vs Sunday — Lord wins (XIII.8)
	if isSunday(prec) && !isLordFeast(fol) && !isSunday(fol) {
		return models.VespersIIOfPreceding
	}
	if isSunday(fol) && !isLordFeast(prec) && !isSunday(prec) {
		return models.VespersIOfFollowing
	}
	if isLordFeast(prec) && isSunday(fol) {
		return models.VespersIIOfPreceding
	}
	if isLordFeast(fol) && isSunday(prec) {
		return models.VespersIOfFollowing
	}

	// 9. Double vs Octave Day — Double wins (XIII.10)
	if isDoubleOrAbove(prec) && !isOctaveDay(prec) && isOctaveDay(fol) {
		return models.VespersIIOfPreceding
	}
	if isDoubleOrAbove(fol) && !isOctaveDay(fol) && isOctaveDay(prec) {
		return models.VespersIOfFollowing
	}

	// 10. Octave Day vs Octave Day — worthier wins (XIII.11)
	if isOctaveDay(prec) && isOctaveDay(fol) {
		if compareFeastPrecedence(prec, fol) {
			return models.VespersIIOfPreceding
		}
		return models.VespersIOfFollowing
	}

	// 11. Double vs day-within-Octave / Saturday BVM — Double wins (XIII.12)
	if isDoubleOrAbove(prec) && (isDayWithinOctave(fol) || isSaturdayBVM(fol)) {
		return models.VespersIIOfPreceding
	}
	if isDoubleOrAbove(fol) && (isDayWithinOctave(prec) || isSaturdayBVM(prec)) {
		return models.VespersIOfFollowing
	}

	// 12. Day-within-Octave vs Double — Double wins (XIII.13)
	// (already covered by rule 11 above)

	// 13. Equal worth: default to II of preceding (TODO Phase 3: split vespers)
	// 14. Fallback: use compareFeastPrecedence
	if compareFeastPrecedence(prec, fol) {
		return models.VespersIIOfPreceding
	}
	// Default: following wins when equal or following is greater
	return models.VespersIOfFollowing
}

// boundaryCommemoration wraps the celebration that lost a vespers concurrence
// so it can be commemorated at the evening's Vespers (XIII.2-17). A nil loser
// (e.g. a plain feria with no Celebration of its own) yields no commemoration.
func boundaryCommemoration(loser *models.Feast) []*models.Feast {
	if loser == nil {
		return nil
	}
	return []*models.Feast{loser}
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
			Commemorations: boundaryCommemoration(precFeast),
		}
	}

	// If following has no I Vespers, preceding wins by default
	if !folHasI {
		return models.VespersDesignation{
			Owner:          models.VespersIIOfPreceding,
			Feast:          precFeast,
			Color:          preceding.Color,
			Season:         preceding.Season,
			Commemorations: boundaryCommemoration(folFeast),
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
			Commemorations: boundaryCommemoration(folFeast),
		}
	}
	return models.VespersDesignation{
		Owner:          models.VespersIOfFollowing,
		Feast:          folFeast,
		Color:          following.Color,
		Season:         following.Season,
		Commemorations: boundaryCommemoration(precFeast),
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
