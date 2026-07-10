package calendar

import (
	"regexp"
	"strings"
	"time"

	"github.com/orthodoxwest/office/internal/models"
)

// privilegedFeastIDs are days that always win regardless of conflicting feasts.
var privilegedFeastIDs = map[string]bool{
	"ash-wednesday":  true,
	"palm-sunday":    true,
	"holy-monday":    true,
	"holy-tuesday":   true,
	"holy-wednesday": true,
	"holy-thursday":  true,
	"good-friday":    true,
	"holy-saturday":  true,
	"easter-sunday":  true,
	"easter-monday":  true,
	"easter-tuesday": true,
	"low-sunday":     true,
	"christmas":      true,
	"vigil-nativity": true,
}

// sortKey computes the precedence key for a feast (higher = wins).
// Sundays below greater-double get boosted to greater-double level.
func sortKey(f *models.Feast) [3]int {
	weight := f.Rank.Weight()
	if f.Category == models.CategorySunday && weight < models.GreaterDouble.Weight() {
		weight = models.GreaterDouble.Weight()
	}
	temporal := 0
	if f.IsTemporal() {
		temporal = 1
	}
	lord := 0
	if f.Category == models.CategoryLord {
		lord = 1
	}
	return [3]int{weight, temporal, lord}
}

// compareSortKeys returns true if a > b (a wins over b).
func compareSortKeys(a, b [3]int) bool {
	for i := range 3 {
		if a[i] != b[i] {
			return a[i] > b[i]
		}
	}
	return false
}

func isCorpusOctaveDay(f *models.Feast) bool {
	return strings.HasPrefix(f.ID, "corpus-christi-octave-day")
}

func isPrivilegedFeria(f *models.Feast) bool {
	return f != nil && f.Category == models.CategoryFeria && f.Rank == models.PrivilegedFeria
}

// compareFeastPrecedence returns true if a should win over b.
func compareFeastPrecedence(a, b *models.Feast) bool {
	// Privileged ferias of the second class take the Office over every feast
	// below a Double of the second class. Their ordinary rank weight is shared
	// with a Semi-double, so this must precede the general rank comparison.
	aPrivilegedFeria := isPrivilegedFeria(a)
	bPrivilegedFeria := isPrivilegedFeria(b)
	if aPrivilegedFeria != bPrivilegedFeria {
		other := b
		if bPrivilegedFeria {
			other = a
		}
		privilegedWins := other.Rank.Weight() < models.Double2ndClass.Weight()
		if aPrivilegedFeria {
			return privilegedWins
		}
		return !privilegedWins
	}

	aCorpus := isCorpusOctaveDay(a)
	bCorpus := isCorpusOctaveDay(b)
	if aCorpus != bCorpus {
		// Sundays and first-class feasts outrank Corpus octave days.
		if aCorpus {
			if b.Category == models.CategorySunday || b.Rank == models.Double1stClass {
				return false
			}
			return true
		}
		if a.Category == models.CategorySunday || a.Rank == models.Double1stClass {
			return true
		}
		return false
	}

	return compareSortKeys(sortKey(a), sortKey(b))
}

func resolvedDayColor(winner *models.Feast, season models.Season, seasonColor models.Color) models.Color {
	if winner == nil {
		return seasonColor
	}

	// In Lent and Passiontide, lesser-rank sanctoral observances use the
	// seasonal color. Not in Septuagesimatide: the 2026 ordo celebrates
	// St Scholastica (Greater Double) in white during pre-Lent.
	if (season == models.Lent || season == models.Passiontide) &&
		winner.Rank.Weight() < models.Double2ndClass.Weight() {
		return seasonColor
	}

	return winner.Color
}

func allCommemorations(candidates []*models.Feast) bool {
	if len(candidates) == 0 {
		return false
	}
	for _, f := range candidates {
		if f.Rank != models.Commemoration {
			return false
		}
	}
	return true
}

var commNormalizationSpaceRE = regexp.MustCompile(`\s+`)

func normalizeCommemorationName(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	n = strings.ReplaceAll(n, "&", " and ")
	n = strings.ReplaceAll(n, "pope", "bishop")
	n = strings.ReplaceAll(n, ".", " ")
	n = strings.ReplaceAll(n, ",", " ")
	n = strings.ReplaceAll(n, ";", " ")
	n = strings.ReplaceAll(n, "(", " ")
	n = strings.ReplaceAll(n, ")", " ")
	n = strings.ReplaceAll(n, "'", " ")
	n = strings.ReplaceAll(n, "’", " ")
	n = strings.ReplaceAll(n, "-", " ")
	n = strings.ReplaceAll(n, "commemoration of", " ")
	n = strings.ReplaceAll(n, "apostles", " ")
	n = strings.ReplaceAll(n, "apostle", " ")
	n = strings.ReplaceAll(n, " the ", " ")
	n = strings.TrimSpace(n)
	return commNormalizationSpaceRE.ReplaceAllString(n, " ")
}

func dedupeCommemorations(winner *models.Feast, comms []*models.Feast) []*models.Feast {
	winnerKey := ""
	if winner != nil {
		winnerKey = normalizeCommemorationName(winner.Name)
	}

	sameOrContained := func(a, b string) bool {
		if a == "" || b == "" {
			return a == b
		}
		return a == b || strings.Contains(a, b) || strings.Contains(b, a)
	}

	seen := make([]string, 0, len(comms))
	deduped := make([]*models.Feast, 0, len(comms))
	for _, comm := range comms {
		key := normalizeCommemorationName(comm.Name)
		if key == "" {
			key = comm.ID
		}
		if sameOrContained(key, winnerKey) {
			continue
		}
		duplicate := false
		for _, prior := range seen {
			if sameOrContained(key, prior) {
				duplicate = true
				break
			}
		}
		if duplicate {
			continue
		}
		seen = append(seen, key)
		deduped = append(deduped, comm)
	}

	return deduped
}

const maxCommemorationsPerDay = 3

func suppressesStGeorgeOctave(winner *models.Feast) bool {
	if winner == nil {
		return false
	}

	if privilegedFeastIDs[winner.ID] {
		return true
	}

	return strings.HasPrefix(winner.ID, "easter-sunday-octave-day-")
}

func shouldSuppressCommemoration(winner, comm *models.Feast) bool {
	if comm == nil {
		return false
	}

	// Data-driven feast coupling (e.g., commemorations that belong only with
	// a specific winning celebration).
	if comm.OnlyWith != "" && (winner == nil || winner.ID != comm.OnlyWith) {
		return true
	}

	if winner == nil {
		return false
	}

	// Within the Pentecost octave, the current day takes precedence over the
	// coincident Whit Ember feria commemoration.
	if strings.HasPrefix(winner.ID, "pentecost-octave-day-") &&
		strings.HasPrefix(comm.ID, "whit-ember-") {
		return true
	}

	// The Sunday-within-octave office does not also commemorate Day IV of the
	// same Ascension/Corpus octave.
	if winner.Category == models.CategorySunday &&
		(strings.HasPrefix(comm.ID, "ascension-octave-day-4") ||
			strings.HasPrefix(comm.ID, "corpus-christi-octave-day-4")) {
		return true
	}

	// Privileged days suppress St George octave commemorations in the years
	// where the octave overlaps Holy Week or the Easter octave.
	if suppressesStGeorgeOctave(winner) && strings.HasPrefix(comm.ID, "st-george-octave-day") {
		return true
	}

	return false
}

func finalizeCommemorations(winner *models.Feast, comms []*models.Feast) []*models.Feast {
	filtered := make([]*models.Feast, 0, len(comms))
	for _, comm := range comms {
		if shouldSuppressCommemoration(winner, comm) {
			continue
		}
		filtered = append(filtered, comm)
	}

	deduped := dedupeCommemorations(winner, filtered)
	if len(deduped) <= maxCommemorationsPerDay {
		return deduped
	}
	return deduped[:maxCommemorationsPerDay]
}

// ResolveDay resolves conflicts for a single day.
// Returns the resolved CalendarDay and any feasts to transfer out.
func ResolveDay(
	date time.Time,
	candidates []*models.Feast,
	season models.Season,
	seasonColor models.Color,
	moveable *MoveableDates,
	transferredIn []*models.Feast,
) (*models.CalendarDay, []*models.Feast) {
	allCandidates := make([]*models.Feast, 0, len(candidates)+len(transferredIn))
	allCandidates = append(allCandidates, candidates...)
	allCandidates = append(allCandidates, transferredIn...)

	var transfersOut []*models.Feast

	if len(allCandidates) == 0 {
		return &models.CalendarDay{
			Date:           date,
			Season:         season,
			Color:          seasonColor,
			ResolutionRule: "occurrence:no-candidates",
		}, transfersOut
	}

	if allCommemorations(allCandidates) {
		return &models.CalendarDay{
			Date:           date,
			Season:         season,
			Commemorations: finalizeCommemorations(nil, allCandidates),
			Color:          seasonColor,
			ResolutionRule: "occurrence:commemorations-only",
		}, transfersOut
	}

	// Check for privileged days
	var privileged []*models.Feast
	for _, f := range allCandidates {
		if privilegedFeastIDs[f.ID] {
			privileged = append(privileged, f)
		}
	}

	if len(privileged) > 0 {
		winner := privileged[0]
		for _, f := range privileged[1:] {
			if compareFeastPrecedence(f, winner) {
				winner = f
			}
		}

		var comms []*models.Feast
		for _, f := range allCandidates {
			if f == winner {
				continue
			}
			if privilegedFeastIDs[f.ID] {
				continue
			}
			if f.Rank.Weight() >= models.Double2ndClass.Weight() && f.Category != models.CategorySunday {
				transfersOut = append(transfersOut, f)
			} else if f.Rank.Weight() >= models.Commemoration.Weight() {
				comms = append(comms, f)
			}
		}
		comms = finalizeCommemorations(winner, comms)

		return &models.CalendarDay{
			Date:           date,
			Season:         season,
			Celebration:    winner,
			Commemorations: comms,
			Color:          resolvedDayColor(winner, season, seasonColor),
			ResolutionRule: "occurrence:privileged-day",
		}, transfersOut
	}

	// Sort by precedence — find winner
	winner := allCandidates[0]
	for _, f := range allCandidates[1:] {
		if compareFeastPrecedence(f, winner) {
			winner = f
		}
	}

	var comms []*models.Feast
	for _, f := range allCandidates {
		if f == winner {
			continue
		}
		if f.Rank.Weight() >= models.Double2ndClass.Weight() && f.Category != models.CategorySunday {
			transfersOut = append(transfersOut, f)
		} else if f.Rank.Weight() >= models.Commemoration.Weight() {
			comms = append(comms, f)
		}
	}
	comms = finalizeCommemorations(winner, comms)

	return &models.CalendarDay{
		Date:           date,
		Season:         season,
		Celebration:    winner,
		Commemorations: comms,
		Color:          resolvedDayColor(winner, season, seasonColor),
		ResolutionRule: "occurrence:general-precedence",
	}, transfersOut
}
