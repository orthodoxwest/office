package calendar

import (
	"cmp"
	"slices"

	"github.com/orthodoxwest/office/internal/models"
)

// LaudsCommemorations includes the displaced seasonal feria in the same
// XIV.14 order as the occurrence commemorations. It does not mutate the day.
func LaudsCommemorations(day *models.CalendarDay) []*models.Feast {
	comms := slices.Clone(day.Commemorations)
	if day.FeriaCommemoration != nil {
		comms = append(comms, day.FeriaCommemoration)
	}
	return orderCommemorations(comms, commemorationOrderContext{season: day.Season, winner: day.Celebration})
}

// Ordering is separate from eligibility, duplicate selection, and text context.
// In particular, replacing tomorrow's duplicate octave with today's identity
// can retain concurrence priority without selecting tomorrow's proper texts.
type commemorationOrderContext struct {
	season         models.Season
	winner         *models.Feast
	concurrent     *models.Feast
	incoming       []*models.Feast
	incomingSeason models.Season
}

func orderCommemorations(comms []*models.Feast, ctx commemorationOrderContext) []*models.Feast {
	// Sort complete office groups, not pairwise aliases of their anchors:
	// otherwise a stable tie could leave an unrelated office inside a pair.
	type group struct {
		office     *models.Feast
		companions []*models.Feast
	}
	var groups []group
	present := make(map[string]bool, len(comms))
	for _, f := range comms {
		if f != nil {
			present[f.ID] = true
		}
	}
	for _, f := range comms {
		if f != nil && (f.CompanionOf == "" || !present[f.CompanionOf]) {
			groups = append(groups, group{office: f})
		}
	}
	for i := range groups {
		for _, f := range comms {
			if f != nil && f.CompanionOf == groups[i].office.ID {
				groups[i].companions = append(groups[i].companions, f)
			}
		}
	}
	season := func(f *models.Feast) models.Season {
		if slices.Contains(ctx.incoming, f) && ctx.incomingSeason != "" {
			return ctx.incomingSeason
		}
		return ctx.season
	}
	// XIV.14(a) promotes a Feast of the Lord over a lesser Sunday or the
	// Epiphany vigil. This is a list-level promotion, not a pairwise exception
	// that would make the comparator non-transitive in a three-office list.
	preferLord := slices.ContainsFunc(comms, func(f *models.Feast) bool {
		return f != nil && (isLesserSunday(f, season(f)) || isEpiphanyVigil(f))
	})
	concurrent := func(g group) bool {
		return ctx.concurrent != nil && (g.office == ctx.concurrent || slices.Contains(g.companions, ctx.concurrent))
	}
	tier := func(f *models.Feast) int {
		// A companion whose parent owns the hour follows that office before
		// other occurrence prayers (Diurnal pp.463,577). An orphan retains
		// its ordinary tier; a present parent supplies the group's tier.
		if ctx.winner != nil && f.CompanionOf == ctx.winner.ID {
			return -1
		}
		return commemorationTier(f, season(f), preferLord)
	}
	slices.SortStableFunc(groups, func(left, right group) int {
		a, b := left.office, right.office
		if concurrent(left) != concurrent(right) {
			if concurrent(left) {
				return -1
			}
			return 1
		}
		if n := cmp.Compare(tier(a), tier(b)); n != 0 {
			return n
		}
		// Keep same-tier ties stable. Title-X dignity is not fully modeled;
		// XIV.14's I-before-II tie also conflicts with the 2026 Mar17 ordo
		// (#402); full same-tier dignity is tracked in #403.
		return cmp.Compare(b.Rank.Weight(), a.Rank.Weight())
	})
	ordered := make([]*models.Feast, 0, len(comms))
	for _, g := range groups {
		ordered = append(ordered, g.office)
		ordered = append(ordered, g.companions...)
	}
	return ordered
}

func isEpiphanyVigil(f *models.Feast) bool {
	return f.CommemorationClass == models.CommemorationEpiphanyVigil
}

func isLesserSunday(f *models.Feast, season models.Season) bool {
	if !isSunday(f) || sundaysFirstClass[f.ID] {
		return false
	}
	return season != models.Advent && season != models.Septuagesima && season != models.Lent && season != models.Passiontide
}

// Tiers follow General Rubrics XIV.14(a-n), pp.26-27. This is deliberately
// distinct from occurrence precedence: e.g. an Ascension octave weekday
// follows a Double, whereas a Corpus Christi octave weekday precedes it.
func commemorationTier(f *models.Feast, season models.Season, preferLord bool) int {
	if isSunday(f) {
		if isLesserSunday(f, season) {
			return 2
		}
		return 0
	}
	if isEpiphanyVigil(f) {
		return 2
	}
	if preferLord && f.Category == models.CategoryLord && isDoubleOrAbove(f) && !isOctaveDay(f) && !isDayWithinOctave(f) {
		return 1
	}
	if isDayWithinOctave(f) && f.OctaveClass == models.OctavePrivilegedSecond {
		return 3
	}
	if isEmberDay(f) || isRogationDay(f) || (f.Category == models.CategoryFeria && !isVigil(f) && (season == models.Lent || season == models.Passiontide)) {
		return 4
	}
	if isOctaveDay(f) && f.Rank == models.GreaterDouble {
		return 5
	}
	if f.Rank.Weight() >= models.GreaterDouble.Weight() {
		return 6
	}
	if f.Rank == models.Double {
		return 7
	}
	if isDayWithinOctave(f) && f.OctaveClass == models.OctavePrivilegedThird {
		return 8
	}
	if f.CommemorationClass == models.CommemorationPostAscensionFeria {
		return 9
	}
	if isDayWithinOctave(f) {
		return 10
	}
	if f.Category == models.CategoryFeria && !isVigil(f) && (season == models.Advent || season == models.Septuagesima) {
		return 11
	}
	if isVigil(f) {
		return 12
	}
	if f.Rank == models.Simple && (isOctaveDay(f) || f.OctaveClass == models.OctaveSimple) {
		return 13
	}
	if f.Rank != models.Commemoration {
		return 14
	}
	return 15
}
