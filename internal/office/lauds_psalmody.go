package office

import (
	"fmt"
	"strings"
	"time"

	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/texts"
)

// Lesser Doubles without proper psalm antiphons retain the weekday psalter
// (Diurnal pp.44,46). A Common's antiphons do not make them proper.
// Explicit proper @use aliases count as appointments; they must represent
// printed directions, not convenience copies of the Common. Simple
// offices also retain the weekday psalter; their octave days use the festal
// weekday canticle. Saturday BVM has its existing explicit hour sections.
func usesWeekdayLaudsPsalmody(day *models.CalendarDay, corpus *texts.TextCorpus) bool {
	if day == nil || day.Celebration == nil || corpus == nil || day.Celebration.ID == saturdayOfficeBVMID {
		return false
	}
	feast := day.Celebration
	if feast.Category == models.CategoryFeria {
		return false
	}
	eligible := feast.Rank == models.Double || feast.Rank == models.Simple ||
		(feast.Category == models.CategorySunday && civilWeekday(day) == time.Saturday)
	if !eligible {
		return false
	}
	for _, id := range feastProperIDs(feast) {
		prefixes := []string{"proper/" + id + "/"}
		if day.Season == models.Easter {
			prefixes = append([]string{"proper/" + id + "-paschal/"}, prefixes...)
		}
		for _, prefix := range prefixes {
			// Explicit data may retain the previous festal selection while a
			// conflicting source appointment awaits clergy review.
			if corpus.Get(prefix+"lauds-psalmody") == "festal" {
				return false
			}
			text, _ := lookupSectionText(prefix, day.Season, "lauds", "psalm-antiphon-1", corpus)
			if text != "" && !texts.IsOmitted(text) {
				return false
			}
		}
	}
	return true
}

func usesFestalWeekdayLaudsCanticle(day *models.CalendarDay, corpus *texts.TextCorpus) bool {
	if !usesWeekdayLaudsPsalmody(day, corpus) || civilWeekday(day) == time.Sunday {
		return false
	}
	f := day.Celebration
	return f.Rank == models.Double || strings.Contains(f.ID, "octave-day") || f.Category == models.CategorySunday
}

// This belongs to actual Lauds composition, not the shared proper lookup:
// Prime borrows the feast/Common's Lauds antiphon even when the weekday
// psalter is appointed at Lauds itself (e.g. Common of a Confessor, p.45*).
func weekdayLaudsAntiphon(day *models.CalendarDay, ref string, corpus *texts.TextCorpus) (string, string) {
	weekday := strings.ToLower(civilWeekday(day).String())
	canticleSlot := "psalm-antiphon-4"
	if civilWeekday(day) == time.Saturday {
		canticleSlot = "psalm-antiphon-3"
	}
	if ref == canticleSlot && usesFestalWeekdayLaudsCanticle(day, corpus) {
		key := "ordinary/lauds/festal-canticle-antiphon-" + weekday
		if day.Season == models.Easter {
			key += "-easter"
		}
		return corpus.Get(key), key
	}
	if day.Season == models.Easter {
		key := "seasonal/easter/" + ref
		return corpus.Get(key), key
	}
	key := "ordinary/lauds/" + ref + "-" + weekday
	return corpus.Get(key), key
}

func validateLaudsPsalmodyDeclarations(corpus *texts.TextCorpus) []string {
	var errs []string
	for _, key := range corpus.References() {
		if (strings.HasSuffix(key, "/lauds-psalmody") || strings.HasPrefix(key, "psalmody/lauds/")) && corpus.Get(key) != "festal" {
			errs = append(errs, fmt.Sprintf("%s: invalid Lauds psalmody declaration (expected festal)", key))
		}
	}
	return errs
}
