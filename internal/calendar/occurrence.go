package calendar

import (
	"fmt"
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

func isCorpusOctaveDay(f *models.Feast) bool {
	return strings.HasPrefix(f.ID, "corpus-christi-octave-day")
}

func isPrivilegedFeria(f *models.Feast) bool {
	return f != nil && f.Category == models.CategoryFeria && f.Rank == models.PrivilegedFeria
}

// compareFeastPrecedence returns true if a should win over b.
func compareFeastPrecedence(a, b *models.Feast) bool {
	wins, _ := compareFeastPrecedenceWithDecision(a, b)
	return wins
}

func compareFeastPrecedenceWithDecision(a, b *models.Feast) (bool, models.CompositionDecision) {
	detail := "challenger=" + a.ID + "; incumbent=" + b.ID
	decision := func(rule string, wins bool) (bool, models.CompositionDecision) {
		outcome := "incumbent-holds"
		if wins {
			outcome = "challenger-wins"
		}
		return wins, models.CompositionDecision{Rule: rule, Outcome: outcome, Detail: detail}
	}

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
			if privilegedWins {
				return decision("occurrence:privileged-feria-below-second-class", true)
			}
			return decision("occurrence:second-class-over-privileged-feria", false)
		}
		if privilegedWins {
			return decision("occurrence:privileged-feria-below-second-class", false)
		}
		return decision("occurrence:second-class-over-privileged-feria", true)
	}

	aCorpus := isCorpusOctaveDay(a)
	bCorpus := isCorpusOctaveDay(b)
	if aCorpus != bCorpus {
		// Sundays and first-class feasts outrank Corpus octave days.
		if aCorpus {
			if b.Category == models.CategorySunday || b.Rank == models.Double1stClass {
				return decision("occurrence:sunday-or-first-class-over-corpus-octave", false)
			}
			return decision("occurrence:corpus-octave-precedence", true)
		}
		if a.Category == models.CategorySunday || a.Rank == models.Double1stClass {
			return decision("occurrence:sunday-or-first-class-over-corpus-octave", true)
		}
		return decision("occurrence:corpus-octave-precedence", false)
	}

	aKey, bKey := sortKey(a), sortKey(b)
	if aKey[0] != bKey[0] {
		rule := "occurrence:higher-rank"
		aBoosted := a.Category == models.CategorySunday && a.Rank.Weight() < models.GreaterDouble.Weight()
		bBoosted := b.Category == models.CategorySunday && b.Rank.Weight() < models.GreaterDouble.Weight()
		if aBoosted || bBoosted {
			rule = "occurrence:sunday-rank-boost"
		}
		return decision(rule, aKey[0] > bKey[0])
	}
	if aKey[1] != bKey[1] {
		return decision("occurrence:temporal-tiebreak", aKey[1] > bKey[1])
	}
	if aKey[2] != bKey[2] {
		return decision("occurrence:lord-tiebreak", aKey[2] > bKey[2])
	}
	return decision("occurrence:equal-precedence-possession", false)
}

func resolvedDayColorWithDecision(winner *models.Feast, season models.Season, seasonColor models.Color) (models.Color, models.CompositionDecision) {
	if winner == nil {
		return seasonColor, models.CompositionDecision{Rule: "color:resolution", Outcome: "seasonal-feria", Detail: string(seasonColor)}
	}

	// In Lent and Passiontide, lesser-rank sanctoral observances use the
	// seasonal color. Not in Septuagesimatide: the 2026 ordo celebrates
	// St Scholastica (Greater Double) in white during pre-Lent.
	if (season == models.Lent || season == models.Passiontide) &&
		winner.Rank.Weight() < models.Double2ndClass.Weight() {
		return seasonColor, models.CompositionDecision{Rule: "color:resolution", Outcome: "penitential-season-over-lesser-feast", Detail: winner.ID + "=" + string(seasonColor)}
	}

	return winner.Color, models.CompositionDecision{Rule: "color:resolution", Outcome: "celebration-color", Detail: winner.ID + "=" + string(winner.Color)}
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

func dedupeCommemorationsWithDecisions(winner *models.Feast, comms []*models.Feast) ([]*models.Feast, []models.CompositionDecision) {
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
	var decisions []models.CompositionDecision
	for _, comm := range comms {
		key := normalizeCommemorationName(comm.Name)
		if key == "" {
			key = comm.ID
		}
		if sameOrContained(key, winnerKey) {
			decisions = append(decisions, models.CompositionDecision{Rule: "commemoration:matches-winner", Outcome: "suppressed", Detail: comm.ID})
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
			decisions = append(decisions, models.CompositionDecision{Rule: "commemoration:duplicate-name", Outcome: "suppressed", Detail: comm.ID})
			continue
		}
		seen = append(seen, key)
		deduped = append(deduped, comm)
	}

	return deduped, decisions
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

func commemorationSuppressionDecision(winner, comm *models.Feast) (bool, models.CompositionDecision) {
	if comm == nil {
		return false, models.CompositionDecision{}
	}

	// Data-driven feast coupling (e.g., commemorations that belong only with
	// a specific winning celebration).
	if comm.OnlyWith != "" && (winner == nil || winner.ID != comm.OnlyWith) {
		return true, models.CompositionDecision{Rule: "commemoration:only-with", Outcome: "suppressed", Detail: comm.ID + " requires " + comm.OnlyWith}
	}

	if winner == nil {
		return false, models.CompositionDecision{}
	}

	// Within the Pentecost octave, the current day takes precedence over the
	// coincident Whit Ember feria commemoration.
	if strings.HasPrefix(winner.ID, "pentecost-octave-day-") &&
		strings.HasPrefix(comm.ID, "whit-ember-") {
		return true, models.CompositionDecision{Rule: "commemoration:pentecost-ember", Outcome: "suppressed", Detail: comm.ID}
	}

	// The Sunday-within-octave office does not also commemorate Day IV of the
	// same Ascension/Corpus octave.
	if winner.Category == models.CategorySunday &&
		(strings.HasPrefix(comm.ID, "ascension-octave-day-4") ||
			strings.HasPrefix(comm.ID, "corpus-christi-octave-day-4")) {
		return true, models.CompositionDecision{Rule: "commemoration:same-octave-sunday", Outcome: "suppressed", Detail: comm.ID}
	}

	// Privileged days suppress St George octave commemorations in the years
	// where the octave overlaps Holy Week or the Easter octave.
	if suppressesStGeorgeOctave(winner) && strings.HasPrefix(comm.ID, "st-george-octave-day") {
		return true, models.CompositionDecision{Rule: "commemoration:st-george-octave", Outcome: "suppressed", Detail: comm.ID}
	}

	return false, models.CompositionDecision{}
}

func finalizeCommemorationsWithDecisions(winner *models.Feast, comms []*models.Feast) ([]*models.Feast, []models.CompositionDecision) {
	filtered := make([]*models.Feast, 0, len(comms))
	var decisions []models.CompositionDecision
	for _, comm := range comms {
		if suppressed, decision := commemorationSuppressionDecision(winner, comm); suppressed {
			decisions = append(decisions, decision)
			continue
		}
		filtered = append(filtered, comm)
	}

	deduped, dedupeDecisions := dedupeCommemorationsWithDecisions(winner, filtered)
	decisions = append(decisions, dedupeDecisions...)
	capped, capDecisions := capCommemorationsWithDecisions(deduped)
	return capped, append(decisions, capDecisions...)
}

func capCommemorationsWithDecisions(comms []*models.Feast) ([]*models.Feast, []models.CompositionDecision) {
	if len(comms) <= maxCommemorationsPerDay {
		return comms, nil
	}
	dropped := make([]string, 0, len(comms)-maxCommemorationsPerDay)
	for _, comm := range comms[maxCommemorationsPerDay:] {
		dropped = append(dropped, comm.ID)
	}
	return comms[:maxCommemorationsPerDay], []models.CompositionDecision{{Rule: "commemoration:cap", Outcome: "truncated", Detail: strings.Join(dropped, ",")}}
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
	decisions := []models.CompositionDecision{{Rule: "occurrence:resolution-mode", Outcome: "start", Detail: fmt.Sprintf("candidates=%d; transferred-in=%d", len(candidates), len(transferredIn))}}
	if len(transferredIn) > 0 {
		decisions = append(decisions, models.CompositionDecision{Rule: "occurrence:transfer-in", Outcome: "considered", Detail: strings.Join(feastIDs(transferredIn), ",")})
	}

	if len(allCandidates) == 0 {
		_, colorDecision := resolvedDayColorWithDecision(nil, season, seasonColor)
		decisions = append(decisions, models.CompositionDecision{Rule: "occurrence:resolution-mode", Outcome: "no-candidates"}, colorDecision)
		return &models.CalendarDay{
			Date:                date,
			Season:              season,
			Color:               seasonColor,
			ResolutionRule:      "occurrence:no-candidates",
			OccurrenceDecisions: decisions,
		}, transfersOut
	}

	if allCommemorations(allCandidates) {
		comms, commDecisions := finalizeCommemorationsWithDecisions(nil, allCandidates)
		_, colorDecision := resolvedDayColorWithDecision(nil, season, seasonColor)
		decisions = append(decisions, models.CompositionDecision{Rule: "occurrence:resolution-mode", Outcome: "commemorations-only"})
		decisions = append(decisions, commDecisions...)
		decisions = append(decisions, colorDecision)
		return &models.CalendarDay{
			Date:                date,
			Season:              season,
			Commemorations:      comms,
			Color:               seasonColor,
			ResolutionRule:      "occurrence:commemorations-only",
			OccurrenceDecisions: decisions,
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
		decisions = append(decisions, models.CompositionDecision{Rule: "occurrence:resolution-mode", Outcome: "privileged-fixed-day", Detail: strings.Join(feastIDs(privileged), ",")})
		winner := privileged[0]
		for _, f := range privileged[1:] {
			wins, decision := compareFeastPrecedenceWithDecision(f, winner)
			decisions = append(decisions, decision)
			if wins {
				winner = f
			}
		}

		var comms []*models.Feast
		for _, f := range allCandidates {
			if f == winner {
				continue
			}
			if privilegedFeastIDs[f.ID] {
				decisions = append(decisions, models.CompositionDecision{Rule: "occurrence:other-privileged-day", Outcome: "suppressed", Detail: f.ID})
				continue
			}
			if f.Rank.Weight() >= models.Double2ndClass.Weight() && f.Category != models.CategorySunday {
				transfersOut = append(transfersOut, f)
				decisions = append(decisions, models.CompositionDecision{Rule: "occurrence:transfer-out", Outcome: "second-class-or-higher", Detail: f.ID})
			} else if f.Rank.Weight() >= models.Commemoration.Weight() {
				comms = append(comms, f)
				decisions = append(decisions, models.CompositionDecision{Rule: "occurrence:loser-disposition", Outcome: "commemorated", Detail: f.ID})
			}
		}
		comms, commDecisions := finalizeCommemorationsWithDecisions(winner, comms)
		decisions = append(decisions, commDecisions...)
		color, colorDecision := resolvedDayColorWithDecision(winner, season, seasonColor)
		decisions = append(decisions, colorDecision)

		return &models.CalendarDay{
			Date:                date,
			Season:              season,
			Celebration:         winner,
			Commemorations:      comms,
			Color:               color,
			ResolutionRule:      "occurrence:privileged-day",
			OccurrenceDecisions: decisions,
		}, transfersOut
	}

	// Sort by precedence — find winner
	decisions = append(decisions, models.CompositionDecision{Rule: "occurrence:resolution-mode", Outcome: "general-precedence"})
	winner := allCandidates[0]
	for _, f := range allCandidates[1:] {
		wins, decision := compareFeastPrecedenceWithDecision(f, winner)
		decisions = append(decisions, decision)
		if wins {
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
			decisions = append(decisions, models.CompositionDecision{Rule: "occurrence:transfer-out", Outcome: "second-class-or-higher", Detail: f.ID})
		} else if f.Rank.Weight() >= models.Commemoration.Weight() {
			comms = append(comms, f)
			decisions = append(decisions, models.CompositionDecision{Rule: "occurrence:loser-disposition", Outcome: "commemorated", Detail: f.ID})
		}
	}
	comms, commDecisions := finalizeCommemorationsWithDecisions(winner, comms)
	decisions = append(decisions, commDecisions...)
	color, colorDecision := resolvedDayColorWithDecision(winner, season, seasonColor)
	decisions = append(decisions, colorDecision)

	return &models.CalendarDay{
		Date:                date,
		Season:              season,
		Celebration:         winner,
		Commemorations:      comms,
		Color:               color,
		ResolutionRule:      "occurrence:general-precedence",
		OccurrenceDecisions: decisions,
	}, transfersOut
}

func feastIDs(feasts []*models.Feast) []string {
	ids := make([]string, 0, len(feasts))
	for _, feast := range feasts {
		if feast != nil {
			ids = append(ids, feast.ID)
		}
	}
	return ids
}
