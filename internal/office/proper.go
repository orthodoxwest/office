package office

import (
	"strconv"
	"strings"
	"time"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/texts"
)

func baseProperRef(ref string) string {
	i := len(ref) - 1
	for i >= 0 && ref[i] >= '0' && ref[i] <= '9' {
		i--
	}
	if i >= 0 && i < len(ref)-1 && ref[i] == '-' {
		return ref[:i]
	}
	return ref
}

func refCandidates(ref string) []string {
	base := baseProperRef(ref)
	if base == ref {
		return []string{ref}
	}
	return []string{ref, base}
}

func hourRefCandidates(hourName, ref string) []string {
	cands := refCandidates(ref)
	out := make([]string, 0, len(cands))
	for _, cand := range cands {
		out = append(out, cand+"-"+hourName)
	}
	return out
}

func feastProperIDs(feast *models.Feast) []string {
	if feast == nil || feast.ID == "" {
		return nil
	}
	if feast.ProperID != "" && feast.ProperID != feast.ID {
		// Days within an octave repeat the parent's office; their ProperID
		// points at an optional per-day antiphon set (…-octave-set-N), so
		// try day-specific texts, then the set, then the parent feast. For
		// every other ProperID redirect (borrowed Sunday offices), the
		// redirect wins.
		if i := strings.Index(feast.ID, "-octave-day"); i > 0 {
			return []string{feast.ID, feast.ProperID, feast.ID[:i]}
		}
		return []string{feast.ProperID, feast.ID}
	}
	return []string{feast.ID}
}

// isSynthesizedFeria reports whether feast is an engine-generated feria with
// no proper of its own. Named temporal ferias such as the Lent Ember days are
// also ranked PrivilegedFeria, but must remain eligible for their dedicated
// proper texts.
func isSynthesizedFeria(feast *models.Feast) bool {
	if feast == nil {
		return false
	}
	return feast.ID == "privileged-lenten-feria" || feast.ID == models.FeriaCommemorationID
}

// isPerAnnumSunday reports whether the feast is an ordinary numbered Sunday
// after Epiphany or Pentecost, including resumed and anticipated Sundays
// whose ProperID redirects to one. epiphany-sunday-1 and pentecost-sunday-2
// are excluded: they always fall within the Epiphany and Corpus Christi
// octaves respectively, whose offices own their eves.
func isPerAnnumSunday(f *models.Feast) bool {
	if f == nil || f.Category != models.CategorySunday {
		return false
	}
	for _, id := range feastProperIDs(f) {
		if id == "epiphany-sunday-1" || id == "pentecost-sunday-2" {
			return false
		}
		if strings.HasPrefix(id, "epiphany-sunday-") || strings.HasPrefix(id, "pentecost-sunday-") {
			return true
		}
	}
	return false
}

func firstText(corpus *texts.TextCorpus, prefix string, refs []string) (string, string) {
	for _, ref := range refs {
		key := prefix + ref
		if text := corpus.Get(key); text != "" {
			return text, key
		}
	}
	return "", ""
}

func seasonRefCandidates(refs []string, season models.Season) []string {
	if season == "" {
		return nil
	}
	out := make([]string, 0, len(refs))
	for _, ref := range refs {
		out = append(out, ref+"-"+string(season))
	}
	return out
}

// lookupSectionText resolves a section within one corpus tier. A
// season-qualified section (for example, versicle-lauds-easter) overrides the
// ordinary section at that same tier; a missing variant falls through to the
// existing hour-qualified and generic section candidates.
func lookupSectionText(prefix string, season models.Season, hourName, ref string, corpus *texts.TextCorpus) (string, string) {
	var hourCandidates []string
	if hourName != "" {
		hourCandidates = hourRefCandidates(hourName, ref)
	}
	refCands := refCandidates(ref)
	if text, resolved := firstText(corpus, prefix, seasonRefCandidates(hourCandidates, season)); text != "" {
		return text, resolved
	}
	if text, resolved := firstText(corpus, prefix, seasonRefCandidates(refCands, season)); text != "" {
		return text, resolved
	}
	if text, resolved := firstText(corpus, prefix, hourCandidates); text != "" {
		return text, resolved
	}
	return firstText(corpus, prefix, refCands)
}

// substituteProperName replaces the liturgical placeholder "N." with the
// saint's proper name in resolved text. Returns text unchanged if name is empty.
func substituteProperName(text, name string) string {
	if name == "" {
		return text
	}
	return strings.ReplaceAll(text, "N.", name)
}

// lookupCommonsText checks the commons tier for a text reference, trying
// paschal variant first (during Easter), then regular commons. For each,
// it tries hour-qualified ref before generic ref. Returns the text and
// resolved ref, or empty strings if not found.
func lookupCommonsText(category models.FeastCategory, season models.Season, hourName, ref string, corpus *texts.TextCorpus) (string, string) {
	if category == "" {
		return "", ""
	}
	cat := string(category)

	// Paschal variant (Easter season only)
	if season == models.Easter {
		prefix := "commons/" + cat + "-paschal/"
		if text, resolved := lookupSectionText(prefix, "", hourName, ref, corpus); text != "" {
			return text, resolved
		}
	}

	// Regular commons
	prefix := "commons/" + cat + "/"
	if text, resolved := lookupSectionText(prefix, season, hourName, ref, corpus); text != "" {
		return text, resolved
	}

	return "", ""
}

func resolveProperCollectText(day *models.CalendarDay, hourName string, corpus *texts.TextCorpus) (string, string) {
	switch hourName {
	case "terce", "sext", "none":
		return resolveProperText(day, "lauds", "collect", corpus)
	default:
		return resolveProperText(day, hourName, "collect", corpus)
	}
}

// resolveProperText looks up a proper text for a given reference, checking
// in order: feast-specific proper, common of saints (with paschal variant),
// seasonal default, weekday ordinary, ordinary fallback, shared fallback.
// Returns the text and the ref it was resolved from.
func resolveProperText(day *models.CalendarDay, hourName, ref string, corpus *texts.TextCorpus) (string, string) {
	// At I Vespers of a following feast, texts that differ from II Vespers
	// carry a "-first" ref variant (e.g. magnificat-antiphon-first); prefer
	// it across all tiers, falling back to the shared ref.
	if day.FirstVespers && hourName == "vespers" && !strings.HasSuffix(ref, "-first") {
		if text, resolved := resolveProperText(day, hourName, ref+"-first", corpus); !strings.HasPrefix(text, "[Proper text not found") {
			return text, resolved
		}
	}

	hourCandidates := hourRefCandidates(hourName, ref)
	refCands := refCandidates(ref)
	weekday := strings.ToLower(civilWeekday(day).String())

	var properName string
	if day.Celebration != nil {
		properName = day.Celebration.ProperName
	}
	ferialVespersAntiphon := hourName == "vespers" &&
		strings.HasPrefix(baseProperRef(ref), "psalm-antiphon") &&
		usesWeekdayVespersAntiphons(day, corpus)

	// 0. The Greater ("O") Antiphons: at Vespers of December 17-23 the
	// date-fixed O antiphon supersedes the Advent Sunday's or feria's own
	// Magnificat antiphon, and (per the ordo, e.g. the Expectation of the
	// B.V.M.) a feast that would take its antiphon from the commons — but
	// yields to a feast's own proper antiphon (e.g. St Thomas's Quia
	// vidisti). The antiphon follows the calendar day the Vespers is sung
	// on: at I Vespers the office day carries tomorrow's date, so step back.
	greaterAntiphon := func() (string, string) {
		if hourName != "vespers" || day.Season != models.Advent || day.Date.Month() != time.December {
			return "", ""
		}
		oDay := day.Date.Day()
		if day.FirstVespers {
			oDay--
		}
		if oDay < 17 || oDay > 23 {
			return "", ""
		}
		for _, cand := range refCands {
			dateRef := "seasonal/advent/" + cand + "-december-" + strconv.Itoa(oDay)
			if text := corpus.Get(dateRef); text != "" {
				return text, dateRef
			}
		}
		return "", ""
	}
	if day.Celebration == nil ||
		day.Celebration.Category == models.CategorySunday ||
		day.Celebration.Category == models.CategoryFeria {
		if text, dateRef := greaterAntiphon(); text != "" {
			return substituteProperName(text, properName), dateRef
		}
	}

	// 0.7. Saturday evening before a per-annum Sunday: the I Vespers
	// Magnificat antiphon follows the scripture cycle rather than the
	// Sunday's own gospel antiphon — the month historia from August until
	// Advent, before that the summer (Kings) antiphon seeded as the
	// Sunday's -first proper, and otherwise the ferial Saturday antiphon
	// from the psalter (2026 ordo: "God hath holpen" on the free Saturdays
	// of Epiphanytide, the historia antiphons through summer and autumn).
	if hourName == "vespers" && day.FirstVespers &&
		strings.HasPrefix(ref, "magnificat-antiphon") && strings.HasSuffix(ref, "-first") &&
		isPerAnnumSunday(day.Celebration) {
		if id := calendar.HistoriaWeekID(day.Date); id != "" {
			key := "proper/historia-" + id + "/magnificat-antiphon-first"
			if text := corpus.Get(key); text != "" {
				return text, key
			}
		}
		for _, feastID := range feastProperIDs(day.Celebration) {
			key := "proper/" + feastID + "/magnificat-antiphon-first"
			if text := corpus.Get(key); text != "" {
				return text, key
			}
		}
		const ferial = "ordinary/vespers/magnificat-antiphon-saturday"
		if text := corpus.Get(ferial); text != "" {
			return text, ferial
		}
	}

	// 0.8. I Vespers of a Sunday is recited on Saturday and uses the
	// Saturday psalter. An explicitly Vespers-qualified Sunday antiphon still
	// wins; otherwise do not let a generic Sunday Lauds antiphon displace the
	// Saturday psalter antiphon.
	if hourName == "vespers" && isSundayFirstVespers(day) &&
		day.Celebration != nil &&
		strings.HasPrefix(ref, "psalm-antiphon") && !strings.HasSuffix(ref, "-first") {
		for _, feastID := range feastProperIDs(day.Celebration) {
			if day.Season == models.Easter {
				prefix := "proper/" + feastID + "-paschal/"
				if text, resolved := firstText(corpus, prefix, hourCandidates); text != "" {
					return substituteProperName(text, properName), resolved
				}
			}
			prefix := "proper/" + feastID + "/"
			if text, resolved := firstText(corpus, prefix, hourCandidates); text != "" {
				return substituteProperName(text, properName), resolved
			}
		}
		// Paschaltide replaces the individual Saturday antiphons with the
		// single seasonal Alleluia antiphon while retaining the Saturday
		// psalms themselves.
		if day.Season == models.Easter {
			prefix := "seasonal/" + string(day.Season) + "/"
			if text, resolved := firstText(corpus, prefix, hourCandidates); text != "" {
				return text, resolved
			}
			if text, resolved := firstText(corpus, prefix, refCands); text != "" {
				return text, resolved
			}
		}
		for _, cand := range refCands {
			saturdayRef := "ordinary/vespers/" + cand + "-saturday"
			if text := corpus.Get(saturdayRef); text != "" {
				return text, saturdayRef
			}
		}
	}

	// 1. Feast-specific proper (hour-qualified, then generic)
	if !ferialVespersAntiphon &&
		day.Celebration != nil && day.Celebration.ID != "" && !isSynthesizedFeria(day.Celebration) {
		for _, feastID := range feastProperIDs(day.Celebration) {
			if day.Season == models.Easter {
				prefix := "proper/" + feastID + "-paschal/"
				if text, resolved := firstText(corpus, prefix, hourCandidates); text != "" {
					return substituteProperName(text, properName), resolved
				}
				if text, resolved := firstText(corpus, prefix, refCands); text != "" {
					return substituteProperName(text, properName), resolved
				}
			}
			prefix := "proper/" + feastID + "/"
			if text, resolved := lookupSectionText(prefix, day.Season, hourName, ref, corpus); text != "" {
				return substituteProperName(text, properName), resolved
			}
		}
	}

	// 1.5. The O antiphon outranks a feast's commons-sourced Magnificat
	// antiphon (see step 0); a proper antiphon has already returned above.
	if text, dateRef := greaterAntiphon(); text != "" {
		return substituteProperName(text, properName), dateRef
	}

	// 2. Common of Saints (paschal, then regular; hour-qualified, then generic)
	if !ferialVespersAntiphon &&
		day.Celebration != nil && !isSynthesizedFeria(day.Celebration) {
		if text, resolved := lookupCommonsText(day.Celebration.Category, day.Season, hourName, ref, corpus); text != "" {
			return substituteProperName(text, properName), resolved
		}
	}

	// 2.5. Weekly temporal texts: ferias take per-day texts distributed
	// through the week from the governing Sunday's proper file
	// (proper/<week>/<ref>-<weekday>), e.g. the gospel-canticle antiphons
	// drawn from the Sunday's Gospel. Sundays use the file's own sections
	// via tier 1; a celebrated feast's propers and commons stay ahead.
	if day.TemporalWeekID != "" && weekday != "sunday" {
		prefix := "proper/" + day.TemporalWeekID + "/"
		wdRefs := make([]string, 0, len(refCands))
		for _, cand := range refCands {
			wdRefs = append(wdRefs, cand+"-"+weekday)
		}
		if text, resolved := firstText(corpus, prefix, wdRefs); text != "" {
			return substituteProperName(text, properName), resolved
		}
	}

	// 3. Seasonal default (hour-qualified, then generic)
	// The seasonal "-first" entries model the Saturday books before Sundays;
	// they must not displace a weekday feast's own generic Vespers text merely
	// because that feast is also celebrating first Vespers.
	seasonalSundayFirst := strings.HasSuffix(ref, "-first") && hourName == "vespers"
	if day.Season != "" && (!seasonalSundayFirst || isSundayFirstVespers(day)) {
		prefix := "seasonal/" + string(day.Season) + "/"
		if text, resolved := firstText(corpus, prefix, hourCandidates); text != "" {
			return substituteProperName(text, properName), resolved
		}
		if text, resolved := firstText(corpus, prefix, refCands); text != "" {
			return substituteProperName(text, properName), resolved
		}
	}

	// 4. Weekday ordinary (e.g. ordinary/lauds/hymn-monday). The Sunday Lauds
	// hymn additionally varies by season: in the summer window it resolves to
	// hymn-sunday-summer rather than the (winter) hymn-sunday default.
	for _, cand := range refCands {
		wd := weekday
		if hourName == "lauds" && cand == "hymn" && weekday == "sunday" && sundayLaudsHymnIsSummer(day.Date) {
			wd = "sunday-summer"
		}
		weekdayRef := "ordinary/" + hourName + "/" + cand + "-" + wd
		if text := corpus.Get(weekdayRef); text != "" {
			return substituteProperName(text, properName), weekdayRef
		}
	}

	// 5. Ordinary fallback (hour-specific)
	for _, cand := range refCands {
		ordinaryRef := "ordinary/" + hourName + "/" + cand
		if text := corpus.Get(ordinaryRef); text != "" {
			return substituteProperName(text, properName), ordinaryRef
		}
	}

	// 6. Shared ordinary fallback
	for _, cand := range refCands {
		sharedRef := "ordinary/shared/" + cand
		if text := corpus.Get(sharedRef); text != "" {
			return substituteProperName(text, properName), sharedRef
		}
	}

	return "[Proper text not found: " + ref + "]", ref
}
