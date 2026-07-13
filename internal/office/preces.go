package office

import (
	"strings"
	"time"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
)

const saturdayOfficeBVMID = "saturday-office-bvm"

var doubleFeriaOfficeIDs = map[string]bool{
	"all-souls":       true,
	"vigil-nativity":  true,
	"vigil-pentecost": true,
}

// evaluateCondition reports whether a section condition is satisfied for the given day.
// Comma-separated conditions are ANDed together: "not-feast-easter-sunday,weekday-sunday"
// means "Sunday AND not Easter Sunday".
func evaluateCondition(condition string, day *models.CalendarDay, moveable *calendar.MoveableDates) bool {
	ok, known := evaluateConditionKnown(condition, day, moveable)
	return known && ok
}

func evaluateConditionKnown(condition string, day *models.CalendarDay, moveable *calendar.MoveableDates) (bool, bool) {
	// Handle comma-separated AND conditions
	if strings.Contains(condition, ",") {
		for _, part := range strings.Split(condition, ",") {
			ok, known := evaluateConditionKnown(strings.TrimSpace(part), day, moveable)
			if !known || !ok {
				return false, known
			}
		}
		return true, true
	}

	// Handle negated conditions: "not-feast-easter-sunday" → negate "feast-easter-sunday"
	if strings.HasPrefix(condition, "not-") {
		ok, known := evaluateConditionKnown(condition[4:], day, moveable)
		if !known {
			return false, false
		}
		return !ok, true
	}

	switch condition {
	case "if-preces":
		return shouldSayPreces(day, moveable), true
	case "if-suffrage":
		return shouldSaySuffrage(day, moveable), true
	case "if-cross-commemoration":
		return shouldSayCrossCommemoration(day, moveable), true
	case "is-feast":
		return day.Celebration != nil &&
			day.Celebration.Category != models.CategoryFeria &&
			day.Celebration.Category != models.CategorySunday, true
	case "is-ferial":
		if day.Celebration == nil {
			return true, true
		}
		return day.Celebration.Category == models.CategoryFeria, true
	default:
		if strings.HasPrefix(condition, "weekday-") {
			return isWeekdayMatch(condition, day), true
		}
		if strings.HasPrefix(condition, "feast-") {
			return day.Celebration != nil && day.Celebration.ID == condition[6:], true
		}
		if strings.HasPrefix(condition, "season-") {
			return string(day.Season) == condition[7:], true
		}
		return false, false
	}
}

// shouldSayPreces determines whether preces should be said at the Little Hours and Compline.
//
// Preces are NOT said when:
//   - The celebration has a Double office
//   - Day is within an octave, or is itself an octave-day office
//   - Any commemoration is a Double or octave-related
//   - The Friday after the Ascension octave (Easter+47)
//   - Vigil of Epiphany (Jan 5)
func shouldSayPreces(day *models.CalendarDay, moveable *calendar.MoveableDates) bool {
	if celebrationHasDoubleOffice(day.Celebration) {
		return false
	}

	// Within an octave
	if day.WithinOctaveOf != "" {
		return false
	}

	// Octave-day offices: preces are not said within octaves (General Rubrics
	// §XXXVII.2), and the parish treats the octave day itself as covered (#15).
	if day.Celebration != nil && strings.Contains(day.Celebration.ID, "octave-day") {
		return false
	}

	// Check commemorations
	for _, comm := range day.Commemorations {
		if comm.Rank.Weight() >= models.Double.Weight() {
			return false
		}
		if strings.Contains(comm.ID, "-octave-") {
			return false
		}
	}

	// Friday after Ascension octave (Easter+47)
	if moveable != nil {
		fridayAfterAscensionOctave := moveable.Easter.AddDate(0, 0, 47)
		if day.Date.Equal(fridayAfterAscensionOctave) {
			return false
		}
	}

	// Vigil of Epiphany (Jan 5)
	if day.Date.Month() == 1 && day.Date.Day() == 5 {
		return false
	}

	return true
}

// celebrationHasDoubleOffice distinguishes office form from occurrence
// precedence. Penitential Sundays and privileged ferias carry elevated ranks
// so they win the calendar day, but their offices remain Sunday or ferial and
// therefore do not suppress the preces (General Rubrics §XXXVII.2).
func celebrationHasDoubleOffice(feast *models.Feast) bool {
	if feast == nil || feast.Rank.Weight() < models.Double.Weight() {
		return false
	}

	switch feast.Category {
	case models.CategorySunday:
		return false
	case models.CategoryFeria:
		// These are actual Double offices whose calendar category is ferial.
		// Other elevated ferias use their rank for precedence rather than
		// office form.
		return doubleFeriaOfficeIDs[feast.ID]
	default:
		return true
	}
}

func officeAllowsCustomarySuffrage(day *models.CalendarDay) bool {
	if day == nil || day.Celebration == nil {
		return true
	}

	if day.Celebration.ID == saturdayOfficeBVMID {
		return true
	}

	switch day.Celebration.Category {
	case models.CategorySunday, models.CategoryFeria:
		return true
	default:
		return false
	}
}

func withinSuffrageSeason(day *models.CalendarDay) bool {
	if day == nil {
		return false
	}

	switch day.Season {
	case models.Epiphany:
		// The suffrage begins after the Epiphany octave.
		return !(day.Date.Month() == time.January && day.Date.Day() >= 7 && day.Date.Day() <= 13)
	case models.Septuagesima, models.Lent, models.Pentecost:
		return true
	default:
		return false
	}
}

func commemorationSuppressesSuffrage(comm *models.Feast) bool {
	if comm == nil {
		return false
	}

	if comm.Rank.Weight() >= models.Double.Weight() {
		return true
	}

	return strings.Contains(comm.ID, "octave")
}

// shouldSaySuffrage determines whether the Suffrage of All Saints should be said.
//
// The Suffrage is said in its customary seasons on Sundays, ferias, vigils,
// and the Saturday Office of the B.V.M., unless an octave or a
// simplified-double commemoration suppresses it.
func shouldSaySuffrage(day *models.CalendarDay, moveable *calendar.MoveableDates) bool {
	_ = moveable

	if !officeAllowsCustomarySuffrage(day) {
		return false
	}

	if day.WithinOctaveOf != "" {
		return false
	}

	if !withinSuffrageSeason(day) {
		return false
	}

	for _, comm := range day.Commemorations {
		if commemorationSuppressesSuffrage(comm) {
			return false
		}
	}

	return true
}

// shouldSayCrossCommemoration determines whether the Commemoration of the Cross
// should be said at Lauds/Vespers during Paschaltide.
//
// It applies during Easter weeks II–V (Easter+7 through Easter+35) when the
// celebration rank is below Double.
func shouldSayCrossCommemoration(day *models.CalendarDay, moveable *calendar.MoveableDates) bool {
	if moveable == nil {
		return false
	}

	if !officeAllowsCustomarySuffrage(day) {
		return false
	}

	if day.WithinOctaveOf != "" {
		return false
	}

	// Must be Easter season
	if day.Season != models.Easter {
		return false
	}

	// Easter weeks II–V: Easter+7 through Easter+35
	since := int(day.Date.Sub(moveable.Easter).Hours() / 24)
	return since >= 7 && since <= 35
}
