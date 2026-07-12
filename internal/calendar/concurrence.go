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

func isEmberDay(f *models.Feast) bool {
	return f != nil && strings.Contains(f.ID, "ember-")
}

func isRogationDay(f *models.Feast) bool {
	return f != nil && f.ID == "rogation-monday"
}

func isVigil(f *models.Feast) bool {
	if f == nil {
		return false
	}
	id := strings.ToLower(f.ID)
	name := strings.ToLower(f.Name)
	return strings.HasPrefix(id, "vigil-") || strings.Contains(id, "-vigil-") ||
		strings.HasPrefix(name, "vigil of ") || strings.HasPrefix(name, "the vigil of ")
}

// occurrenceCommemoratedAtFirstVespers reports whether a following day's
// occurrence commemoration begins on the preceding evening. XIV.9 gives
// Ember days, Rogation Monday, and common vigils Lauds only; the remaining
// occurrence classes represented in CalendarDay.Commemorations include
// I Vespers in their hour coverage.
func occurrenceCommemoratedAtFirstVespers(comm *models.Feast) (bool, string) {
	if comm == nil {
		return false, "commemoration:first-vespers-nil"
	}
	if isEmberDay(comm) || isRogationDay(comm) || isVigil(comm) {
		return false, "commemoration:first-vespers-feria-or-vigil-lauds-only"
	}
	return true, "commemoration:first-vespers-occurrence-included"
}

// occurrenceCommemoratedAtSecondVespers reports whether an observance
// commemorated at Lauds remains at II Vespers of the day's winning office.
// XIV.9 gives Memorials and simple octave days only I Vespers and Lauds;
// Ember days, Rogation Monday, and common vigils are commemorated only at
// Lauds. Sundays, seasonal ferias, simplified Doubles, and common octaves
// retain both Vespers subject to the exclusions for I Class winners.
func occurrenceCommemoratedAtSecondVespers(winner, comm *models.Feast) (bool, string) {
	if comm == nil {
		return false, "commemoration:second-vespers-nil"
	}
	if isEmberDay(comm) || isRogationDay(comm) || isVigil(comm) {
		return false, "commemoration:second-vespers-feria-or-vigil-lauds-only"
	}
	if comm.Rank == models.Commemoration || comm.Rank == models.Simple {
		return false, "commemoration:second-vespers-memorial-or-simple"
	}
	if winner != nil && winner.Rank == models.Double1stClass && comm.Category != models.CategorySunday {
		return false, "commemoration:second-vespers-first-class-exclusion"
	}
	if isDayWithinOctave(comm) && winner != nil && winner.Rank.Weight() >= models.Double2ndClass.Weight() {
		return false, "commemoration:second-vespers-day-within-octave-exclusion"
	}
	return true, "commemoration:second-vespers-included"
}

// followingOfficeCommemoratedAtSecondVespers excludes next-day offices whose
// hour coverage does not extend to the preceding evening. Simples and
// Memorials have no II-Vespers boundary commemoration (XIV.7-9); common
// vigils are Simple offices and are covered by the same rule.
func followingOfficeCommemoratedAtSecondVespers(following *models.CalendarDay) (bool, string) {
	if following == nil || following.Celebration == nil {
		return false, "commemoration:following-office-at-second-vespers-nil"
	}
	feast := following.Celebration
	if feast.Rank == models.Simple || feast.Rank == models.Commemoration {
		return false, "commemoration:following-office-at-second-vespers-simple-or-memorial"
	}
	return true, "commemoration:following-office-at-second-vespers-included"
}

func octaveCelebrationParent(day *models.CalendarDay) string {
	if day == nil || day.Celebration == nil {
		return ""
	}
	if day.Celebration.HasOctave {
		return day.Celebration.ID
	}
	parent := day.WithinOctaveOf
	if parent == "" {
		return ""
	}
	if strings.HasPrefix(day.Celebration.ID, parent+"-octave-day") {
		return parent
	}
	// Easter Monday and Tuesday have explicit temporal entries rather than
	// generated octave-day IDs, but they continue the Easter octave office.
	if parent == "easter-sunday" &&
		(day.Celebration.ID == "easter-monday" || day.Celebration.ID == "easter-tuesday") {
		return parent
	}
	return ""
}

func sameOctaveOffice(preceding, following *models.CalendarDay) bool {
	precParent := octaveCelebrationParent(preceding)
	return precParent != "" && precParent == octaveCelebrationParent(following)
}

// outgoingCommemoratedAtFirstVespers applies XIV.7-8 to the office displaced
// by I Vespers of the following feast. Offices which ended at None are not
// automatically concurrence parties; higher-class incoming feasts also have
// explicit exclusions for Sundays, ferias, octaves, and I/II Class Doubles.
func outgoingCommemoratedAtFirstVespers(winner, loser *models.Feast) (bool, string) {
	if loser == nil {
		return false, "commemoration:first-vespers-no-outgoing-office"
	}

	// XIV.7 excludes seasonal ferias before a I Class Double, while XIV.8
	// expressly retains them before a II Class Double.
	if winner != nil && winner.Rank == models.Double1stClass &&
		loser.Category == models.CategoryFeria && loser.Rank == models.PrivilegedFeria {
		return false, "commemoration:first-vespers-first-class-seasonal-feria-exclusion"
	}

	if !hasSecondVespers(loser) {
		// A day within an octave may still be commemorated under XIII.13;
		// ferias in the penitential seasons are handled as privileged ferias.
		if isDayWithinOctave(loser) {
			if winner != nil && winner.Rank.Weight() >= models.Double2ndClass.Weight() {
				return false, "commemoration:first-vespers-day-within-octave-exclusion"
			}
			return true, "commemoration:first-vespers-day-within-octave"
		}
		if loser.Category == models.CategoryFeria && loser.Rank == models.PrivilegedFeria &&
			!isEmberDay(loser) && !isRogationDay(loser) && !isVigil(loser) {
			return true, "commemoration:first-vespers-seasonal-feria"
		}
		return false, "commemoration:first-vespers-office-ended-at-none"
	}

	if winner == nil {
		return true, "commemoration:first-vespers-concurrence"
	}
	if winner.Rank == models.Double1stClass {
		if loser.Category == models.CategorySunday {
			if winner.ID == "christmas" || winner.ID == "epiphany" {
				return true, "commemoration:first-vespers-nativity-epiphany-sunday"
			}
			return false, "commemoration:first-vespers-first-class-sunday-exclusion"
		}
		if loser.Rank.Weight() >= models.Double2ndClass.Weight() || loser.Category == models.CategoryFeria {
			return false, "commemoration:first-vespers-first-class-exclusion"
		}
	}
	if winner.Rank == models.Double2ndClass {
		if winner.ID == "circumcision" &&
			(loser.Category == models.CategorySunday || loser.Rank.Weight() >= models.GreaterDouble.Weight()) {
			return false, "commemoration:first-vespers-circumcision-exclusion"
		}
		if isDayWithinOctave(loser) {
			return false, "commemoration:first-vespers-second-class-octave-exclusion"
		}
		if loser.Category == models.CategoryFeria && loser.Rank != models.PrivilegedFeria {
			return false, "commemoration:first-vespers-second-class-feria-exclusion"
		}
	}
	return true, "commemoration:first-vespers-concurrence"
}

// concurrenceWinner determines which office wins vespers when II Vespers of
// prec concurs with I Vespers of fol. Returns VespersIIOfPreceding or
// VespersIOfFollowing.
//
// Implements Section XIII.2-17 of the monastic rubrics, with the parish
// ordo's resolutions where the rubrics leave room (equal-rank concurrence,
// possession against an incoming lesser Sunday).
func concurrenceWinner(prec, fol *models.Feast) models.VespersOwner {
	winner, _ := concurrenceWinnerWithRule(prec, fol)
	return winner
}

func concurrenceWinnerWithRule(prec, fol *models.Feast) (models.VespersOwner, string) {
	// 1. Greater Sundays of the I Class hold their Vespers against feasts
	// below II Class. A sanctoral I or II Class Double takes the concurrence
	// (XIII.6; 2026 ordo: St Benedict D2 retains II Vespers against Laetare,
	// and St Tikhon's D1 takes I Vespers from Low Sunday). Temporal octave
	// days do not displace them, and Easter and Pentecost yield to nothing.
	if sundaysFirstClass[prec.ID] {
		if fol.Rank.Weight() >= models.Double2ndClass.Weight() && !fol.IsTemporal() &&
			prec.ID != "easter-sunday" && prec.ID != "pentecost" {
			return models.VespersIOfFollowing, "concurrence:greater-sunday-vs-class-i-ii"
		}
		return models.VespersIIOfPreceding, "concurrence:greater-sunday"
	}
	if sundaysFirstClass[fol.ID] {
		if prec.Rank.Weight() >= models.Double2ndClass.Weight() && !prec.IsTemporal() &&
			fol.ID != "easter-sunday" && fol.ID != "pentecost" {
			return models.VespersIIOfPreceding, "concurrence:greater-sunday-vs-class-i-ii"
		}
		return models.VespersIOfFollowing, "concurrence:greater-sunday"
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
			return feastWins, "concurrence:class-i-ii-vs-sunday"
		}
		// 3. Below II Class: the Sunday takes the concurrence (XIII.2-5;
		// the ordo applies this even to Greater Double feasts — the XV
		// Sunday keeps its II Vespers against the Exaltation, and Chains
		// of St Peter, the Name of Mary and the Presentation all yield
		// their II Vespers to the incoming Sunday). An Octave Day in
		// possession is the exception: it keeps its II Vespers (2026
		// ordo: Octave Day of the Assumption before the XII Sunday).
		if feast == prec && isOctaveDay(feast) {
			return feastWins, "concurrence:octave-day-in-possession-vs-sunday"
		}
		return sundayWins, "concurrence:sunday-below-class-ii"
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
			return otherWins, "concurrence:class-i-ii-vs-octave-day" // XIII.10 excludes I/II Class Doubles
		}
		if isDoubleOrAbove(other) {
			return octaveWins, "concurrence:octave-day-vs-double"
		}
	}

	// 6. Octave Day vs Octave Day — worthier wins (XIII.11)
	if isOctaveDay(prec) && isOctaveDay(fol) {
		if compareFeastPrecedence(prec, fol) {
			return models.VespersIIOfPreceding, "concurrence:octave-day-vs-octave-day"
		}
		return models.VespersIOfFollowing, "concurrence:octave-day-vs-octave-day"
	}

	// 7. Double vs day-within-Octave / Saturday BVM — Double wins (XIII.12,13)
	if isDoubleOrAbove(prec) && (isDayWithinOctave(fol) || isSaturdayBVM(fol)) {
		return models.VespersIIOfPreceding, "concurrence:double-vs-octave-or-saturday-bvm"
	}
	if isDoubleOrAbove(fol) && (isDayWithinOctave(prec) || isSaturdayBVM(prec)) {
		return models.VespersIOfFollowing, "concurrence:double-vs-octave-or-saturday-bvm"
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
		return models.VespersIIOfPreceding, "concurrence:general-precedence"
	}
	if !compareFeastPrecedence(fol, prec) && prec.Rank.Weight() >= models.Double2ndClass.Weight() {
		return models.VespersIIOfPreceding, "concurrence:equal-second-class"
	}
	return models.VespersIOfFollowing, "concurrence:equal-following"
}

// boundaryCommemorations collects the commemorations proper to an evening's
// Vespers: the celebration that lost the concurrence (XIII.2-17), followed by
// the following day's own commemorated feasts, whose observance begins with a
// commemoration at this Vespers (2026 ordo: Marcellus at St Maurus' II
// Vespers, Sabina on the eve of the Beheading). A nil loser (e.g. a plain
// feria with no Celebration of its own) contributes nothing.
//
// Whether the concurrence loser is commemorated depends on the incoming
// office and the loser's class (XIV.7-8). The following day's memorial-rank
// commemorations belong to the day that is beginning: at I Vespers of the
// following they are always kept
// (Hadrian at the Nativity BVM's I Vespers, Barnabas at Corpus Christi's),
// but the II Vespers of an outgoing Double of the II Class or above (Sundays
// excepted) does not admit them — the ordo prints "No Comm." at the Vespers
// of the Circumcision, the Purification, St Lawrence and St Matthew, and
// through the Easter and Pentecost octaves, while a privileged feria
// (Lent, Embertide, Advent) is still commemorated ("Comm. Fer." at
// St Joseph's and the Conception's Vespers). The winner and duplicates are
// filtered by finalizeCommemorationsWithDecisions.
func boundaryCommemorationsWithDecisions(winner, loser *models.Feast, following *models.CalendarDay, secondVespers, sameOctave bool) ([]*models.Feast, []models.CompositionDecision) {
	suppressIncoming := secondVespers && winner != nil &&
		winner.Rank.Weight() >= models.Double2ndClass.Weight() &&
		winner.Category != models.CategorySunday

	suppressed := func(c *models.Feast) bool {
		return suppressIncoming &&
			c.Rank.Weight() < models.Double2ndClass.Weight() &&
			c.Category != models.CategorySunday && c.Category != models.CategoryFeria
	}

	var comms []*models.Feast
	var decisions []models.CompositionDecision
	if loser != nil {
		if sameOctave {
			decisions = append(decisions, models.CompositionDecision{Rule: "commemoration:same-octave-boundary", Outcome: "suppressed", Detail: loser.ID})
		} else if !secondVespers {
			if included, rule := outgoingCommemoratedAtFirstVespers(winner, loser); included {
				comms = append(comms, loser)
				decisions = append(decisions, models.CompositionDecision{Rule: rule, Outcome: "included", Detail: loser.ID})
			} else {
				decisions = append(decisions, models.CompositionDecision{Rule: rule, Outcome: "suppressed", Detail: loser.ID})
			}
		} else {
			if included, rule := followingOfficeCommemoratedAtSecondVespers(following); included {
				comms = append(comms, loser)
				decisions = append(decisions, models.CompositionDecision{Rule: rule, Outcome: "included", Detail: loser.ID})
			} else {
				decisions = append(decisions, models.CompositionDecision{Rule: rule, Outcome: "suppressed", Detail: loser.ID})
			}
		}
	}
	for _, c := range following.Commemorations {
		if included, rule := occurrenceCommemoratedAtFirstVespers(c); !included {
			decisions = append(decisions, models.CompositionDecision{Rule: rule, Outcome: "suppressed", Detail: c.ID})
		} else if !suppressed(c) {
			comms = append(comms, c)
			decisions = append(decisions, models.CompositionDecision{Rule: rule, Outcome: "included", Detail: c.ID})
		} else {
			decisions = append(decisions, models.CompositionDecision{Rule: "commemoration:incoming-at-second-vespers", Outcome: "suppressed", Detail: c.ID})
		}
	}
	finalized, finalDecisions := finalizeCommemorationsWithDecisions(winner, comms)
	return finalized, append(decisions, finalDecisions...)
}

func secondVespersCommemorationsWithDecisions(winner *models.Feast, day *models.CalendarDay, boundary []*models.Feast, boundaryDecisions []models.CompositionDecision) ([]*models.Feast, []models.CompositionDecision) {
	comms := make([]*models.Feast, 0, len(day.Commemorations)+len(boundary))
	decisions := append([]models.CompositionDecision{}, boundaryDecisions...)
	for _, comm := range day.Commemorations {
		if included, rule := occurrenceCommemoratedAtSecondVespers(winner, comm); included {
			comms = append(comms, comm)
			decisions = append(decisions, models.CompositionDecision{Rule: rule, Outcome: "included", Detail: comm.ID})
		} else {
			decisions = append(decisions, models.CompositionDecision{Rule: rule, Outcome: "suppressed", Detail: comm.ID})
		}
	}
	comms = append(comms, boundary...)
	finalized, finalDecisions := finalizeCommemorationsWithDecisions(winner, comms)
	return finalized, append(decisions, finalDecisions...)
}

// noOwnerCommemorationsWithDecisions composes an evening on which neither
// adjacent celebration owns I/II Vespers. Eligible occurrence commemorations
// from the current day remain at II Vespers, while eligible commemorations of
// the following day begin at I Vespers (XIV.9). The office itself remains the
// current ferial or simple office.
func noOwnerCommemorationsWithDecisions(preceding, following *models.CalendarDay) ([]*models.Feast, []models.CompositionDecision) {
	var current, incoming []*models.Feast
	var decisions []models.CompositionDecision
	for _, comm := range preceding.Commemorations {
		if included, rule := occurrenceCommemoratedAtSecondVespers(preceding.Celebration, comm); included {
			current = append(current, comm)
			decisions = append(decisions, models.CompositionDecision{Rule: rule, Outcome: "included", Detail: comm.ID})
		} else {
			decisions = append(decisions, models.CompositionDecision{Rule: rule, Outcome: "suppressed", Detail: comm.ID})
		}
	}
	for _, comm := range following.Commemorations {
		if included, rule := occurrenceCommemoratedAtFirstVespers(comm); included {
			incoming = append(incoming, comm)
			decisions = append(decisions, models.CompositionDecision{Rule: "commemoration:incoming-at-unowned-vespers", Outcome: "included", Detail: comm.ID})
		} else {
			decisions = append(decisions, models.CompositionDecision{Rule: rule, Outcome: "suppressed", Detail: comm.ID})
		}
	}
	current, currentDecisions := finalizeCommemorationsWithDecisions(preceding.Celebration, current)
	incoming, incomingDecisions := finalizeCommemorationsWithDecisions(following.Celebration, incoming)
	decisions = append(decisions, currentDecisions...)
	decisions = append(decisions, incomingDecisions...)
	combined, dedupeDecisions := dedupeCommemorationsWithDecisions(nil, append(current, incoming...))
	combined, capDecisions := capCommemorationsWithDecisions(combined)
	decisions = append(decisions, dedupeDecisions...)
	decisions = append(decisions, capDecisions...)
	return combined, decisions
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
	sameOctave := sameOctaveOffice(preceding, following)

	// Neither celebration owns I/II Vespers. Keep the current office, but use
	// the following day's occurrence commemorations (XIV.9).
	if !precHasII && !folHasI {
		comms, decisions := noOwnerCommemorationsWithDecisions(preceding, following)
		return models.VespersDesignation{
			Commemorations: comms,
			Rule:           "concurrence:neither-office-has-rights",
			Decisions:      decisions,
		}
	}

	// If preceding has no II Vespers, following wins by default
	if !precHasII {
		comms, decisions := boundaryCommemorationsWithDecisions(folFeast, precFeast, following, false, sameOctave)
		return models.VespersDesignation{
			Owner:          models.VespersIOfFollowing,
			Feast:          folFeast,
			Color:          following.Color,
			Season:         following.Season,
			Commemorations: comms,
			Rule:           "concurrence:following-only",
			Decisions:      decisions,
		}
	}

	// If following has no I Vespers, preceding wins by default
	if !folHasI {
		boundary, boundaryDecisions := boundaryCommemorationsWithDecisions(precFeast, folFeast, following, true, sameOctave)
		comms, decisions := secondVespersCommemorationsWithDecisions(precFeast, preceding, boundary, boundaryDecisions)
		return models.VespersDesignation{
			Owner:          models.VespersIIOfPreceding,
			Feast:          precFeast,
			Color:          preceding.Color,
			Season:         preceding.Season,
			Commemorations: comms,
			Rule:           "concurrence:preceding-only",
			Decisions:      decisions,
		}
	}

	// Both have vespers — resolve the concurrence
	winner, rule := concurrenceWinnerWithRule(precFeast, folFeast)
	if winner == models.VespersIIOfPreceding {
		boundary, boundaryDecisions := boundaryCommemorationsWithDecisions(precFeast, folFeast, following, true, sameOctave)
		comms, decisions := secondVespersCommemorationsWithDecisions(precFeast, preceding, boundary, boundaryDecisions)
		return models.VespersDesignation{
			Owner:          models.VespersIIOfPreceding,
			Feast:          precFeast,
			Color:          preceding.Color,
			Season:         preceding.Season,
			Commemorations: comms,
			Rule:           rule,
			Decisions:      decisions,
		}
	}
	comms, decisions := boundaryCommemorationsWithDecisions(folFeast, precFeast, following, false, sameOctave)
	return models.VespersDesignation{
		Owner:          models.VespersIOfFollowing,
		Feast:          folFeast,
		Color:          following.Color,
		Season:         following.Season,
		Commemorations: comms,
		Rule:           rule,
		Decisions:      decisions,
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
