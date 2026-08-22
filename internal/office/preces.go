package office

import (
	"slices"
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

// officeOfTheDeadIDs are the celebrations composed with the Office of the
// Dead. Its psalms are not concluded with the Gloria Patri: "At the end of all
// Psalms is always said: Rest eternal grant unto them, O Lord. And let light
// perpetual shine upon them, even if the Office be said for one person only"
// (Monastic Diurnal p. 72*). The Commemoration of All the Departed O.S.B.
// (Nov 14) belongs here too once it is in the kalendar.
var officeOfTheDeadIDs = map[string]bool{
	"all-souls": true,
}

// isOfficeOfTheDead reports whether the day's office is the Office of the
// Dead, which differs from every other office in more than its texts: five
// psalms at Vespers rather than four, no Gloria Patri, and no chapter, hymn or
// short responsory at either hour.
func isOfficeOfTheDead(day *models.CalendarDay) bool {
	return day != nil && day.Celebration != nil && officeOfTheDeadIDs[day.Celebration.ID]
}

// psalmDoxologyKeys are every corpus key psalmDoxologyRef can return, so the
// hour-definition validator can require them all.
var psalmDoxologyKeys = []string{
	"ordinary/shared/gloria-patri",
	"shared/formulas/rest-eternal",
}

// psalmDoxologyRef returns the corpus key for what concludes each psalm and
// gospel canticle of the day's office — the Gloria Patri, or "Rest eternal" in
// the Office of the Dead.
func psalmDoxologyRef(day *models.CalendarDay) string {
	if isOfficeOfTheDead(day) {
		return "shared/formulas/rest-eternal"
	}
	return "ordinary/shared/gloria-patri"
}

// usesFestalLaudsPsalmody reports whether Lauds takes the festal psalms. Most
// Sunday offices retain the Sunday psalter, but the printed offices for the
// Sundays within the Nativity and Epiphany octaves explicitly share the
// corresponding feast's psalmody.
func usesFestalLaudsPsalmody(day *models.CalendarDay) bool {
	if day == nil || day.Celebration == nil {
		return false
	}
	for _, id := range feastProperIDs(day.Celebration) {
		if id == "nativity-sunday-within-octave" ||
			id == "epiphany-sunday-within-octave" {
			return true
		}
	}
	return day.Celebration.Category != models.CategoryFeria &&
		day.Celebration.Category != models.CategorySunday
}

// PrecesDisposition is the structural reason preces are said or omitted.
// Values are stable review-plan feature outcomes (rule "preces").
const (
	PrecesSaid                           = "said"
	PrecesSuppressedDoubleOffice         = "suppressed:double-office"
	PrecesSuppressedWithinOctave         = "suppressed:within-octave"
	PrecesSuppressedOctaveDay            = "suppressed:octave-day"
	PrecesSuppressedDoubleCommemoration  = "suppressed:double-commemoration"
	PrecesSuppressedOctaveCommemoration  = "suppressed:octave-commemoration"
	PrecesSuppressedFridayAfterAscension = "suppressed:friday-after-ascension-octave"
	PrecesSuppressedVigilEpiphany        = "suppressed:vigil-epiphany"
	PrecesSuppressedEasterSunday         = "suppressed:easter-sunday"
)

// precesDisposition determines whether preces should be said and why.
//
// Preces are NOT said when:
//   - The celebration has a Double office
//   - Day is within an octave, or is itself an octave-day office
//   - Any commemoration is a Double or octave-related
//   - The Friday after the Ascension octave (Easter+47)
//   - Vigil of Epiphany (Jan 5)
//   - Any Sunday in Eastertide (season Easter: Low Sunday through the
//     Saturday/Sunday before Pentecost). The 2026 archdiocesan ordo prints
//     No Preces on every such Sunday while retaining Preces on the ferias
//     of the same weeks; this is parish practice beyond the bare diurnal
//     §XXXVII list (see #15).
//
// The suppression on a Sunday that commemorates a Double follows §XXXVII.2 and
// is corroborated by the Diurnal's own General Rubrics §IV: "When a Double
// Feast or an Octave is commemorated on a Sunday, the Preces and Suffrage are
// omitted on Sunday. On other Sundays, except within Privileged or Common
// Octaves, they are always said." The 2026 ordo prints Preces on those Sundays
// anyway; #15 holds that contradiction open pending a ruling, and we follow the
// two printed witnesses until it lands.
func precesDisposition(day *models.CalendarDay, moveable *calendar.MoveableDates) (said bool, reason string) {
	if day == nil {
		// Match historical shouldSayPreces(nil): no celebration → preces said.
		return true, PrecesSaid
	}
	if celebrationHasDoubleOffice(day.Celebration) {
		return false, PrecesSuppressedDoubleOffice
	}

	// Within an octave
	if day.WithinOctaveOf != "" {
		return false, PrecesSuppressedWithinOctave
	}

	// Octave-day offices. The Monastic Diurnal's General Rubrics §VII settles
	// both halves: "Within Octaves the usual Suffrage of All Saints and the
	// Preces at Prime and Compline are not said, even though the Office be of a
	// Sunday or of a Greater Feria… On the Octave Day the Office is said as on
	// the day of the Feast, unless otherwise noted; but on a Simple Octave Day
	// the Office is Simple, the Preces and Suffrage being omitted." A festal
	// octave day therefore takes the feast's Double office (suppressed above),
	// and a simple one is suppressed by name. Cf. §XXXVII.2 and #15.
	if day.Celebration != nil && strings.Contains(day.Celebration.ID, "octave-day") {
		return false, PrecesSuppressedOctaveDay
	}

	// Check commemorations
	for _, comm := range day.Commemorations {
		if comm.Rank.Weight() >= models.Double.Weight() {
			return false, PrecesSuppressedDoubleCommemoration
		}
		if strings.Contains(comm.ID, "-octave-") {
			return false, PrecesSuppressedOctaveCommemoration
		}
	}

	// Friday after Ascension octave (Easter+47)
	if moveable != nil {
		fridayAfterAscensionOctave := moveable.Easter.AddDate(0, 0, 47)
		if day.Date.Equal(fridayAfterAscensionOctave) {
			return false, PrecesSuppressedFridayAfterAscension
		}
	}

	// Vigil of Epiphany (Jan 5)
	if day.Date.Month() == 1 && day.Date.Day() == 5 {
		return false, PrecesSuppressedVigilEpiphany
	}

	// Eastertide Sundays (2026 ordo: No Preces on every Sunday of season
	// Easter, including II–V after Easter and the Sunday in the Ascension
	// octave, while ferias of those weeks keep Preces).
	if day.Season == models.Easter && day.Date.Weekday() == time.Sunday {
		return false, PrecesSuppressedEasterSunday
	}

	return true, PrecesSaid
}

// shouldSayPreces reports whether preces should be said at the Little Hours and Compline.
func shouldSayPreces(day *models.CalendarDay, moveable *calendar.MoveableDates) bool {
	said, _ := precesDisposition(day, moveable)
	return said
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

// SuffrageDisposition is the structural reason the Suffrage of All Saints is
// said or omitted. Values are stable review-plan feature outcomes (rule "suffrage").
const (
	SuffrageSaid                    = "said"
	SuffrageSuppressedNonCustomary  = "suppressed:non-customary-office"
	SuffrageSuppressedWithinOctave  = "suppressed:within-octave"
	SuffrageSuppressedOutOfSeason   = "suppressed:out-of-season"
	SuffrageSuppressedCommemoration = "suppressed:commemoration"
)

// suffrageDisposition determines whether the Suffrage of All Saints is said and why.
//
// The Suffrage is said in its customary seasons on Sundays, ferias, vigils,
// and the Saturday Office of the B.V.M., unless an octave or a
// simplified-double commemoration suppresses it.
func suffrageDisposition(day *models.CalendarDay, moveable *calendar.MoveableDates) (said bool, reason string) {
	_ = moveable
	if day == nil {
		// Preserve prior shouldSaySuffrage(nil) → false (out of season).
		return false, SuffrageSuppressedOutOfSeason
	}

	if !officeAllowsCustomarySuffrage(day) {
		return false, SuffrageSuppressedNonCustomary
	}

	if day.WithinOctaveOf != "" {
		return false, SuffrageSuppressedWithinOctave
	}

	if !withinSuffrageSeason(day) {
		return false, SuffrageSuppressedOutOfSeason
	}

	if slices.ContainsFunc(day.Commemorations, commemorationSuppressesSuffrage) {
		return false, SuffrageSuppressedCommemoration
	}

	return true, SuffrageSaid
}

// shouldSaySuffrage reports whether the Suffrage of All Saints should be said.
func shouldSaySuffrage(day *models.CalendarDay, moveable *calendar.MoveableDates) bool {
	said, _ := suffrageDisposition(day, moveable)
	return said
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
