package calendar

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/orthodoxwest/office/internal/models"
)

var easterOffsetRE = regexp.MustCompile(`^easter([+-]\d+)$`)

func romanNumeral(n int) string {
	if n <= 0 {
		return strconv.Itoa(n)
	}
	values := []int{1000, 900, 500, 400, 100, 90, 50, 40, 10, 9, 5, 4, 1}
	symbols := []string{"M", "CM", "D", "CD", "C", "XC", "L", "XL", "X", "IX", "V", "IV", "I"}
	var b strings.Builder
	for i, v := range values {
		for n >= v {
			b.WriteString(symbols[i])
			n -= v
		}
	}
	return b.String()
}

// shortNames for octave/vigil references.
var shortNames = map[string]string{
	"easter-sunday":         "Easter",
	"pentecost":             "Pentecost",
	"christmas":             "Christmas",
	"epiphany":              "the Epiphany",
	"ascension":             "the Ascension",
	"corpus-christi":        "Corpus Christi",
	"ss-peter-paul":         "Ss Peter & Paul",
	"assumption-bvm":        "the Assumption",
	"nativity-bvm":          "the Nativity of the B.V.M.",
	"conception-bvm":        "the Conception of the B.V.M.",
	"all-saints":            "All Saints",
	"nativity-john-baptist": "St John the Baptist",
}

var privilegedOctaves = map[string]bool{
	"easter-sunday": true,
	"pentecost":     true,
}

var simpleOctaves = map[string]bool{
	"nativity-bvm": true,
}

func getShortName(feast *models.Feast) string {
	if short, ok := shortNames[feast.ID]; ok {
		return short
	}
	return feast.Name
}

// OctaveDisplayName returns the human-readable name for an octave parent feast
// given its feast ID (e.g. "christmas" → "Christmas").
func OctaveDisplayName(feastID string) string {
	if short, ok := shortNames[feastID]; ok {
		return short
	}
	return titleCase(strings.ReplaceAll(feastID, "-", " "))
}

// epiphanySundayFeasts generates Feast objects for Sundays after Epiphany.
func epiphanySundayFeasts(year int, septuagesima time.Time) []*models.Feast {
	epiphany := time.Date(year, 1, 6, 0, 0, 0, 0, time.UTC)
	// Find the first Sunday after Epiphany
	daysUntilSunday := (7 - int(epiphany.Weekday())) % 7
	if daysUntilSunday == 0 {
		daysUntilSunday = 7 // Epiphany itself is Sunday; first Sunday after is next week
	}
	firstSunday := epiphany.AddDate(0, 0, daysUntilSunday)

	var feasts []*models.Feast
	current := firstSunday
	n := 1
	for current.Before(septuagesima) {
		var name string
		color := models.Green
		if n == 1 {
			name = "Sunday within the Octave of Epiphany"
			color = models.White
		} else {
			name = fmt.Sprintf("%s Sunday after Epiphany", romanNumeral(n))
		}

		feasts = append(feasts, &models.Feast{
			ID:       fmt.Sprintf("epiphany-sunday-%d", n),
			Name:     name,
			Rank:     models.SemiDouble,
			Color:    color,
			Category: models.CategorySunday,
			DateRule: fmt.Sprintf("epiphany-sunday-%d", n),
		})
		current = current.AddDate(0, 0, 7)
		n++
	}

	switch len(feasts) {
	case 7:
		// The office of the VII Sunday after Epiphany is that of the XXIII
		// Sunday after Pentecost in years with seven Sundays after Epiphany.
		feasts[6].ProperID = "pentecost-sunday-23"
	case 8:
		// In years with eight Sundays after Epiphany, VII and VIII use the
		// XXII and XXIII Pentecost offices respectively.
		feasts[6].ProperID = "pentecost-sunday-22"
		feasts[7].ProperID = "pentecost-sunday-23"
	}

	return feasts
}

// eastertideSundayFeasts generates Sundays after Easter through Trinity.
func eastertideSundayFeasts(easter time.Time) []*models.Feast {
	type easterSunday struct {
		id     string
		name   string
		offset int
		color  models.Color
	}

	specs := []easterSunday{
		{id: "easter-sunday-2", name: "II Sunday after Easter", offset: 14, color: models.White},
		{id: "easter-sunday-3", name: "III Sunday after Easter", offset: 21, color: models.White},
		{id: "easter-sunday-4", name: "IV Sunday after Easter", offset: 28, color: models.White},
		{id: "easter-sunday-5", name: "V Sunday after Easter", offset: 35, color: models.White},
		{id: "ascension-sunday-within-octave", name: "Sunday within the Octave of the Ascension", offset: 42, color: models.White},
		{id: "pentecost-sunday-1", name: "I Sunday after Pentecost", offset: 56, color: models.Green},
	}

	feasts := make([]*models.Feast, 0, len(specs))
	for _, s := range specs {
		feasts = append(feasts, &models.Feast{
			ID:       s.id,
			Name:     s.name,
			Rank:     models.SemiDouble,
			Color:    s.color,
			Category: models.CategorySunday,
			DateRule: fmt.Sprintf("easter+%d", s.offset),
		})
	}
	return feasts
}

// adventSundayFeasts generates Feast objects for the 4 Advent Sundays.
func adventSundayFeasts(moveable *MoveableDates) []*models.Feast {
	dates := []time.Time{moveable.Advent1, moveable.Advent2, moveable.Advent3, moveable.Advent4}
	names := []string{
		"I Sunday of Advent",
		"II Sunday of Advent",
		"III Sunday of Advent (Gaudete)",
		"IV Sunday of Advent",
	}
	colors := []models.Color{models.Violet, models.Violet, models.Rose, models.Violet}

	feasts := make([]*models.Feast, 4)
	for i := range 4 {
		_ = dates[i]
		feasts[i] = &models.Feast{
			ID:       fmt.Sprintf("advent-sunday-%d", i+1),
			Name:     names[i],
			Rank:     models.Double2ndClass,
			Color:    colors[i],
			Category: models.CategorySunday,
			DateRule: fmt.Sprintf("advent-sunday-%d", i+1),
		}
	}
	return feasts
}

// pentecostSundayFeasts generates Feast objects for Sundays 2-24 after Pentecost.
// When more than 24 Sundays fall before Advent, the surplus Sundays between the
// XXIII and the last are the resumed Sundays after Epiphany that had no place
// earlier in the year, and the last Sunday always takes the XXIV propers.
func pentecostSundayFeasts(easter time.Time, advent1 time.Time) []*models.Feast {
	firstSunday := easter.AddDate(0, 0, 56) // I Sunday after Pentecost (Trinity)
	// Both dates are Sundays, so the difference is a whole number of weeks and
	// equals the length of the I..last series (the last Sunday is advent1 - 7).
	total := int(advent1.Sub(firstSunday).Hours() / (24 * 7))
	surplus := total - 24

	var feasts []*models.Feast
	straight := 24
	if surplus > 0 {
		straight = 23
	}
	for n := 2; n <= straight; n++ {
		offset := 49 + (n * 7)
		sundayDate := easter.AddDate(0, 0, offset)
		if !sundayDate.Before(advent1) {
			break
		}
		feasts = append(feasts, &models.Feast{
			ID:       fmt.Sprintf("pentecost-sunday-%d", n),
			Name:     fmt.Sprintf("%s Sunday after Pentecost", romanNumeral(n)),
			Rank:     models.SemiDouble,
			Color:    models.Green,
			Category: models.CategorySunday,
			DateRule: fmt.Sprintf("easter+%d", offset),
		})
	}

	if surplus > 0 {
		// Resume the highest-numbered skipped Epiphany Sundays, in order,
		// on the Sundays between the XXIII and the last.
		for i := range surplus {
			n := 6 - surplus + 1 + i
			offset := 49 + ((24 + i) * 7)
			feasts = append(feasts, &models.Feast{
				ID:       fmt.Sprintf("epiphany-sunday-%d-resumed", n),
				Name:     fmt.Sprintf("%s Sunday after Epiphany (Resumed)", romanNumeral(n)),
				Rank:     models.SemiDouble,
				Color:    models.Green,
				Category: models.CategorySunday,
				ProperID: fmt.Sprintf("epiphany-sunday-%d", n),
				DateRule: fmt.Sprintf("easter+%d", offset),
			})
		}
		lastOffset := 49 + ((24 + surplus) * 7)
		feasts = append(feasts, &models.Feast{
			ID:       "pentecost-sunday-24",
			Name:     "XXIV & Last Sunday after Pentecost",
			Rank:     models.SemiDouble,
			Color:    models.Green,
			Category: models.CategorySunday,
			DateRule: fmt.Sprintf("easter+%d", lastOffset),
		})
	}

	// The last Sunday after Pentecost always uses 24th Sunday propers
	if len(feasts) > 0 && feasts[len(feasts)-1].ID != "pentecost-sunday-24" {
		last := feasts[len(feasts)-1]
		feasts[len(feasts)-1] = &models.Feast{
			ID:       last.ID,
			Name:     "XXIV & Last Sunday after Pentecost",
			Rank:     last.Rank,
			Color:    last.Color,
			Category: last.Category,
			ProperID: "pentecost-sunday-24",
			DateRule: last.DateRule,
			Notes:    "Uses 24th Sunday propers",
		}
	}

	return feasts
}

// nativityOctaveSundayFeast generates the observed Sunday-within-Nativity-Octave
// office. When the only Sunday would fall on St Stephen, St John, or Holy
// Innocents (Dec 26-28), or when Christmas itself is Sunday, the office is
// observed on Dec 29.
func nativityOctaveSundayFeast(year int) *models.Feast {
	observed := time.Date(year, 12, 29, 0, 0, 0, 0, time.UTC)
	for day := 26; day <= 31; day++ {
		candidate := time.Date(year, 12, day, 0, 0, 0, 0, time.UTC)
		if candidate.Weekday() != time.Sunday {
			continue
		}
		if day >= 29 {
			observed = candidate
		}
		break
	}

	return &models.Feast{
		ID:       "nativity-sunday-within-octave",
		Name:     "Sunday within the Octave of the Nativity",
		Rank:     models.SemiDouble,
		Color:    models.White,
		Category: models.CategorySunday,
		Month:    int(observed.Month()),
		Day:      observed.Day(),
	}
}

// vigilFeasts generates synthetic vigil Feast objects.
func vigilFeasts(feasts []*models.Feast, year int, easter time.Time, moveable *MoveableDates) []*models.Feast {
	var vigils []*models.Feast
	for _, feast := range feasts {
		if !feast.HasVigil {
			continue
		}
		parentDate := resolveFeastDate(feast, year, easter, moveable)
		if parentDate.IsZero() {
			continue
		}
		vigilDate := parentDate.AddDate(0, 0, -1)
		// Vigils are not observed on Sunday; anticipate to Saturday.
		if vigilDate.Weekday() == time.Sunday {
			vigilDate = vigilDate.AddDate(0, 0, -1)
		}
		short := getShortName(feast)
		vigils = append(vigils, &models.Feast{
			ID:       "vigil-of-" + feast.ID,
			Name:     "Vigil of " + short,
			Rank:     models.Simple,
			Color:    models.Violet,
			Category: models.CategoryFeria,
			Month:    int(vigilDate.Month()),
			Day:      vigilDate.Day(),
		})
	}
	return vigils
}

// octaveFeasts generates synthetic Feast objects for octave days (days 2-8).
func octaveFeasts(feasts []*models.Feast, year int, easter time.Time, moveable *MoveableDates) []*models.Feast {
	var generated []*models.Feast
	for _, feast := range feasts {
		if !feast.HasOctave {
			continue
		}
		parentDate := resolveFeastDate(feast, year, easter, moveable)
		if parentDate.IsZero() {
			continue
		}

		isPrivileged := privilegedOctaves[feast.ID]
		isSimple := simpleOctaves[feast.ID]
		isChristmas := feast.ID == "christmas"

		for dayNum := 2; dayNum <= 8; dayNum++ {
			octaveDate := parentDate.AddDate(0, 0, dayNum-1)

			if isChristmas && dayNum == 8 {
				// The civil octave day (Jan 1) has its own fixed feast.
				continue
			}

			if isPrivileged {
				// Easter has explicit entries for Monday, Tuesday, and Low Sunday.
				if feast.ID == "easter-sunday" && (dayNum == 2 || dayNum == 3 || dayNum == 8) {
					continue
				}
				// Pentecost octave has no separate octave day on Trinity Sunday.
				if feast.ID == "pentecost" && dayNum == 8 {
					continue
				}
			}

			isOctaveDay := dayNum == 8

			var rank models.Rank
			switch {
			case isChristmas:
				rank = models.SemiDouble
			case isPrivileged:
				rank = models.Double1stClass
			case isSimple:
				rank = models.Simple
			case isOctaveDay:
				rank = models.GreaterDouble
			default:
				rank = models.SemiDouble
			}

			short := getShortName(feast)
			var feastID, name string
			if isOctaveDay {
				feastID = feast.ID + "-octave-day"
				switch feast.ID {
				case "conception-bvm":
					name = "Octave of the Conception of the B.V.M"
				case "nativity-john-baptist":
					name = "Octave of St John Baptist"
				case "ss-peter-paul":
					name = "The Octave of Ss Peter & Paul"
				default:
					name = "Octave Day of " + short
				}
			} else {
				feastID = fmt.Sprintf("%s-octave-day-%d", feast.ID, dayNum)
				if isChristmas {
					name = fmt.Sprintf("Day %s within the Nativity Octave", romanNumeral(dayNum))
				} else if feast.ID == "conception-bvm" {
					name = fmt.Sprintf("Day %s within Conception Octave", romanNumeral(dayNum))
				} else if feast.ID == "nativity-john-baptist" {
					name = fmt.Sprintf("Day %s within the Octave of St John Baptist", romanNumeral(dayNum))
				} else if feast.ID == "ss-peter-paul" {
					name = fmt.Sprintf("Day %s within the Octave of Ss Peter & Paul", romanNumeral(dayNum))
				} else {
					name = fmt.Sprintf("Day %s within the Octave of %s", romanNumeral(dayNum), short)
				}
			}

			// Days within an octave repeat the office of the feast:
			// resolve proper texts from the parent feast. Ferial days within
			// the octave additionally take per-day antiphon sets in course,
			// skipping any Sunday (which has its own office), so the set
			// index counts non-Sunday days since the feast. Set files are
			// optional: resolution falls back to the parent's propers.
			properID := feast.ID
			if !isOctaveDay {
				setIdx := 0
				for n := 2; n <= dayNum; n++ {
					if parentDate.AddDate(0, 0, n-1).Weekday() != time.Sunday {
						setIdx++
					}
				}
				if octaveDate.Weekday() != time.Sunday {
					properID = fmt.Sprintf("%s-octave-set-%d", feast.ID, setIdx)
				}
			}
			if feast.IsFixed() {
				generated = append(generated, &models.Feast{
					ID:       feastID,
					Name:     name,
					Rank:     rank,
					Color:    feast.Color,
					Category: feast.Category,
					ProperID: properID,
					Month:    int(octaveDate.Month()),
					Day:      octaveDate.Day(),
				})
			} else {
				delta := int(octaveDate.Sub(easter).Hours() / 24)
				generated = append(generated, &models.Feast{
					ID:       feastID,
					Name:     name,
					Rank:     rank,
					Color:    feast.Color,
					Category: feast.Category,
					ProperID: properID,
					DateRule: fmt.Sprintf("easter+%d", delta),
				})
			}
		}
	}
	return generated
}

// remainingEmberFeasts generates Advent and September Ember Days.
func remainingEmberFeasts(moveable *MoveableDates, year int) []*models.Feast {
	var feasts []*models.Feast

	// Advent Ember Days: Wed, Fri, Sat after 3rd Sunday of Advent (Gaudete)
	adventEmberWed := moveable.Advent3.AddDate(0, 0, 3)
	adventEmberFri := moveable.Advent3.AddDate(0, 0, 5)
	adventEmberSat := moveable.Advent3.AddDate(0, 0, 6)
	for _, pair := range [][2]any{
		{"advent-ember-wednesday", adventEmberWed},
		{"advent-ember-friday", adventEmberFri},
		{"advent-ember-saturday", adventEmberSat},
	} {
		label := pair[0].(string)
		dt := pair[1].(time.Time)
		feasts = append(feasts, &models.Feast{
			ID:       label,
			Name:     titleCase(strings.ReplaceAll(label, "-", " ")),
			Rank:     models.SemiDouble,
			Color:    models.Violet,
			Category: models.CategoryFeria,
			Month:    int(dt.Month()),
			Day:      dt.Day(),
		})
	}

	// September Ember Days: Wed, Fri, Sat after Sep 14 (Holy Cross)
	sep15 := time.Date(year, 9, 15, 0, 0, 0, 0, time.UTC)
	daysToWed := (3 - int(sep15.Weekday()) + 7) % 7 // Wednesday = 3 in Go
	if daysToWed == 0 && sep15.Weekday() != time.Wednesday {
		daysToWed = 7
	}
	sepEmberWed := sep15.AddDate(0, 0, daysToWed)
	sepEmberFri := sepEmberWed.AddDate(0, 0, 2)
	sepEmberSat := sepEmberWed.AddDate(0, 0, 3)
	for _, pair := range [][2]any{
		{"september-ember-wednesday", sepEmberWed},
		{"september-ember-friday", sepEmberFri},
		{"september-ember-saturday", sepEmberSat},
	} {
		label := pair[0].(string)
		dt := pair[1].(time.Time)
		feasts = append(feasts, &models.Feast{
			ID:       label,
			Name:     titleCase(strings.ReplaceAll(label, "-", " ")),
			Rank:     models.SemiDouble,
			Color:    models.Violet,
			Category: models.CategoryFeria,
			Month:    int(dt.Month()),
			Day:      dt.Day(),
		})
	}

	return feasts
}

// titleCase converts "advent ember wednesday" to "Advent Ember Wednesday".
func titleCase(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

func isLeapYear(year int) bool {
	if year%400 == 0 {
		return true
	}
	if year%100 == 0 {
		return false
	}
	return year%4 == 0
}

// Roman bissextile handling: in leap years, fixed feasts from Feb 24-28
// are observed one civil day later.
func adjustFixedDateForLeapYear(year, month, day int, feastID string) (int, int) {
	if strings.HasPrefix(feastID, "vigil-") || strings.HasPrefix(feastID, "vigil-of-") {
		return month, day
	}
	if month == 2 && day >= 24 && day <= 28 && isLeapYear(year) {
		return month, day + 1
	}
	return month, day
}

// resolveFeastDate resolves a feast to a concrete date for the given year.
func resolveFeastDate(feast *models.Feast, year int, easter time.Time, moveable *MoveableDates) time.Time {
	if feast.IsFixed() {
		if feast.SkipRomanLeapShift {
			return time.Date(year, time.Month(feast.Month), feast.Day, 0, 0, 0, 0, time.UTC)
		}
		month, day := adjustFixedDateForLeapYear(year, feast.Month, feast.Day, feast.ID)
		return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	}

	if feast.DateRule != "" {
		if feast.DateRule == "holy-name" {
			jan2 := time.Date(year, 1, 2, 0, 0, 0, 0, time.UTC)
			for i := 0; i <= 3; i++ {
				d := jan2.AddDate(0, 0, i)
				if d.Weekday() == time.Sunday {
					return d
				}
			}
			return jan2
		}

		if feast.DateRule == "last-sunday-october" {
			oct31 := time.Date(year, 10, 31, 0, 0, 0, 0, time.UTC)
			return oct31.AddDate(0, 0, -int(oct31.Weekday()))
		}

		if m := easterOffsetRE.FindStringSubmatch(feast.DateRule); m != nil {
			offset, _ := strconv.Atoi(m[1])
			return easter.AddDate(0, 0, offset)
		}

		if strings.HasPrefix(feast.DateRule, "epiphany-sunday-") {
			parts := strings.Split(feast.DateRule, "-")
			idx, _ := strconv.Atoi(parts[len(parts)-1])
			epiphany := time.Date(year, 1, 6, 0, 0, 0, 0, time.UTC)
			daysUntilSunday := (7 - int(epiphany.Weekday())) % 7
			if daysUntilSunday == 0 {
				daysUntilSunday = 7
			}
			return epiphany.AddDate(0, 0, daysUntilSunday+(idx-1)*7)
		}

		if strings.HasPrefix(feast.DateRule, "advent-sunday-") {
			parts := strings.Split(feast.DateRule, "-")
			idx, _ := strconv.Atoi(parts[len(parts)-1])
			adventDates := []time.Time{moveable.Advent1, moveable.Advent2, moveable.Advent3, moveable.Advent4}
			if idx >= 1 && idx <= 4 {
				return adventDates[idx-1]
			}
		}

		if suffix, ok := strings.CutPrefix(feast.DateRule, "pentecost-sunday-"); ok {
			idx, _ := strconv.Atoi(suffix)
			return moveable.Pentecost.AddDate(0, 0, 7*idx)
		}
	}

	return time.Time{}
}

// temporalWeekID returns the ID of the temporal Sunday office governing the
// week that begins on the given Sunday, chosen from that day's feast
// candidates (the Sunday feast is present even when displaced by a higher
// occurrence). Resumed Sundays report their ProperID so weekday texts
// resolve to the original Epiphany Sunday's proper file. Returns "" when no
// temporal Sunday falls on the day (e.g. the Sundays of Christmastide before
// the computed Sunday-within-the-octave).
func temporalWeekID(dayCandidates []*models.Feast) string {
	// Temporal Sundays of Our Lord carry Category "lord" rather than
	// "sunday"; the ones that head a week with per-day weekly texts are
	// accepted as fallbacks when no Category=sunday candidate exists.
	lordSundays := map[string]bool{"easter-sunday": true, "low-sunday": true, "pentecost": true}
	fallback := ""
	for _, f := range dayCandidates {
		if f.Category == models.CategorySunday {
			if f.ProperID != "" {
				return f.ProperID
			}
			return f.ID
		}
		if lordSundays[f.ID] {
			fallback = f.ID
		}
	}
	return fallback
}

// lentenFeriaName returns a descriptive name like "Wednesday after Lent III"
// for unnamed weekday ferias during Lent and Passiontide, or "" for other days.
func lentenFeriaName(date time.Time, easter time.Time, season models.Season) string {
	if date.Weekday() == time.Sunday {
		return ""
	}
	weekday := date.Weekday().String()
	daysToEaster := int(easter.Sub(date) / (24 * time.Hour))
	switch {
	case season == models.Lent && daysToEaster >= 36 && daysToEaster <= 41:
		return fmt.Sprintf("%s after Lent I", weekday)
	case season == models.Lent && daysToEaster >= 29 && daysToEaster <= 34:
		return fmt.Sprintf("%s after Lent II", weekday)
	case season == models.Lent && daysToEaster >= 22 && daysToEaster <= 27:
		return fmt.Sprintf("%s after Lent III", weekday)
	case season == models.Lent && daysToEaster >= 15 && daysToEaster <= 20:
		return fmt.Sprintf("%s after Lent IV", weekday)
	case season == models.Passiontide && daysToEaster >= 8 && daysToEaster <= 13:
		return fmt.Sprintf("%s after Passion Sunday", weekday)
	}
	return ""
}

func saturdayOfficeBVMAllowed(season models.Season) bool {
	switch season {
	case models.Christmas, models.Epiphany, models.Easter, models.Pentecost:
		return true
	default:
		return false
	}
}

func saturdayOfficeBVMFeast(date time.Time) *models.Feast {
	return &models.Feast{
		ID:       "saturday-office-bvm",
		Name:     "Saturday Office of the B.V.M",
		Rank:     models.Simple,
		Color:    models.White,
		Category: models.CategoryBlessedVirgin,
		Month:    int(date.Month()),
		Day:      date.Day(),
	}
}

// BuildCalendar builds the complete liturgical calendar for a year.
func BuildCalendar(year int, dataDir string) ([]models.CalendarDay, error) {
	feasts, err := LoadFeasts(dataDir)
	if err != nil {
		return nil, fmt.Errorf("loading feasts: %w", err)
	}
	penitentialRules, err := loadPenitentialRules(dataDir)
	if err != nil {
		return nil, fmt.Errorf("loading penitential rules: %w", err)
	}

	_, err = LoadSeasons(dataDir)
	if err != nil {
		return nil, fmt.Errorf("loading seasons: %w", err)
	}

	moveable := ComputeMoveableDates(year)

	// Generate computed Sundays
	computedSundays := epiphanySundayFeasts(year, moveable.Septuagesima)
	computedSundays = append(computedSundays, adventSundayFeasts(moveable)...)
	computedSundays = append(computedSundays, eastertideSundayFeasts(moveable.Easter)...)
	computedSundays = append(computedSundays, pentecostSundayFeasts(moveable.Easter, moveable.Advent1)...)
	computedSundays = append(computedSundays, nativityOctaveSundayFeast(year))

	// Assign base feasts + computed Sundays to dates
	allBase := make([]*models.Feast, 0, len(feasts)+len(computedSundays))
	allBase = append(allBase, feasts...)
	allBase = append(allBase, computedSundays...)

	candidates := make(map[time.Time][]*models.Feast)
	for _, feast := range allBase {
		d := resolveFeastDate(feast, year, moveable.Easter, moveable)
		if !d.IsZero() {
			candidates[d] = append(candidates[d], feast)
		}
	}

	// Generate vigils, octave days, and remaining Ember Days
	vigils := vigilFeasts(allBase, year, moveable.Easter, moveable)
	octaves := octaveFeasts(allBase, year, moveable.Easter, moveable)
	embers := remainingEmberFeasts(moveable, year)

	synthetics := make([]*models.Feast, 0, len(vigils)+len(octaves)+len(embers))
	synthetics = append(synthetics, vigils...)
	synthetics = append(synthetics, octaves...)
	synthetics = append(synthetics, embers...)

	for _, feast := range synthetics {
		d := resolveFeastDate(feast, year, moveable.Easter, moveable)
		if !d.IsZero() {
			candidates[d] = append(candidates[d], feast)
		}
	}

	anchorDates := make(map[string]time.Time, len(allBase)+len(synthetics))
	for _, feast := range append(append([]*models.Feast{}, allBase...), synthetics...) {
		d := resolveFeastDate(feast, year, moveable.Easter, moveable)
		if !d.IsZero() {
			anchorDates[feast.ID] = d
		}
	}

	// Build octave range lookup: for each HasOctave feast, map days 1-8 to the parent ID.
	octaveRanges := make(map[time.Time]string)
	for _, feast := range allBase {
		if !feast.HasOctave {
			continue
		}
		parentDate := resolveFeastDate(feast, year, moveable.Easter, moveable)
		if parentDate.IsZero() {
			continue
		}
		for dayNum := 1; dayNum <= 8; dayNum++ {
			d := parentDate.AddDate(0, 0, dayNum-1)
			octaveRanges[d] = feast.ID
		}
	}

	// Walk each day of the year, resolving conflicts
	start := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(year, 12, 31, 0, 0, 0, 0, time.UTC)
	var calendarDays []models.CalendarDay
	var pendingTransfers []*models.Feast
	var weekID string

	for current := start; !current.After(end); current = current.AddDate(0, 0, 1) {
		season := DetermineSeason(current, moveable)
		seasonColor := SeasonColors[season]

		dayCandidates := candidates[current]
		if current.Weekday() == time.Sunday {
			weekID = temporalWeekID(dayCandidates)
		}
		transferredIn := pendingTransfers
		pendingTransfers = nil

		calDay, transfersOut := ResolveDay(current, dayCandidates, season, seasonColor, moveable, transferredIn)
		pendingTransfers = append(pendingTransfers, transfersOut...)
		calDay.TemporalWeekID = weekID

		if calDay.Celebration == nil {
			calDay.Tempora = lentenFeriaName(current, moveable.Easter, season)
		}

		if current.Weekday() == time.Saturday && calDay.Celebration == nil && saturdayOfficeBVMAllowed(season) {
			calDay.Celebration = saturdayOfficeBVMFeast(current)
			calDay.Color = calDay.Celebration.Color
		}

		if parentID, ok := octaveRanges[current]; ok {
			calDay.WithinOctaveOf = parentID
		}

		calDay.MarianAntiphon = DetermineMarianAntiphon(current, moveable)

		calendarDays = append(calendarDays, *calDay)
	}

	if err := applyPenitentialRules(calendarDays, penitentialRules, anchorDates); err != nil {
		return nil, fmt.Errorf("applying penitential rules: %w", err)
	}

	resolveVespersConcurrence(calendarDays)

	return calendarDays, nil
}
