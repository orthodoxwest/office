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
			if corpus.Get(prefix+laudsPsalmodyRef) != "" {
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
		return lookupSeasonalText(day, "lauds", ref, corpus)
	}
	key := "ordinary/lauds/" + ref + "-" + weekday
	return corpus.Get(key), key
}

const (
	laudsPsalmodyRef        = "lauds-psalmody"
	laudsLaudatePsalmodyRef = "lauds-laudate-psalmody"
)

// A proper may retain the standard festal form or declare the main psalmody
// and Laudate separately. Keeping the latter separate preserves the hour's
// section boundaries and allows Psalm150 alone in the Office of the Dead.
func lookupLaudsPsalmody(day *models.CalendarDay, ref string, corpus *texts.TextCorpus) (string, string) {
	if day == nil || day.Celebration == nil || corpus == nil {
		return "", ""
	}
	for _, id := range feastProperIDs(day.Celebration) {
		if day.Season == models.Easter {
			if body, key := firstText(corpus, "proper/"+id+"-paschal/", []string{ref}); body != "" {
				return body, key
			}
		}
		if body, key := firstText(corpus, "proper/"+id+"/", []string{ref}); body != "" {
			return body, key
		}
	}
	return "", ""
}

func usesDeclaredLaudsPsalmody(day *models.CalendarDay, corpus *texts.TextCorpus) bool {
	body, _ := lookupLaudsPsalmody(day, laudsPsalmodyRef, corpus)
	return body != "" && body != "festal"
}

func validHourPsalmodyRef(hour, ref string) bool {
	return (hour == "vespers" && ref == vespersPsalmodyRef) ||
		(hour == "lauds" && (ref == laudsPsalmodyRef || ref == laudsLaudatePsalmodyRef))
}

func resolveHourPsalmody(day *models.CalendarDay, hour, ref string, corpus *texts.TextCorpus) ([]psalmodyItem, string, error) {
	if !validHourPsalmodyRef(hour, ref) {
		return nil, "", fmt.Errorf("unsupported %s psalmody ref %q", hour, ref)
	}
	if hour == "vespers" {
		return resolveVespersPsalmody(day, corpus)
	}
	body, source := lookupLaudsPsalmody(day, ref, corpus)
	items, ferial, err := parsePsalmodyDeclaration(body)
	if err != nil {
		return nil, source, fmt.Errorf("invalid Lauds psalmody %q: %w", ref, err)
	}
	if ferial {
		return nil, source, fmt.Errorf("lauds psalmody %q does not support ferial markers", source)
	}
	items, err = selectPsalmodyItems(items, day.Date)
	return items, source, err
}

func validateLaudsPsalmodyDeclarations(corpus *texts.TextCorpus) []string {
	var errs []string
	for _, key := range corpus.References() {
		if !strings.HasSuffix(key, "/"+laudsPsalmodyRef) && !strings.HasSuffix(key, "/"+laudsLaudatePsalmodyRef) && !strings.HasPrefix(key, "psalmody/lauds/") {
			continue
		}
		body := corpus.Get(key)
		if body == "festal" && !strings.HasSuffix(key, "/"+laudsLaudatePsalmodyRef) {
			continue
		}
		items, ferial, err := parsePsalmodyDeclaration(body)
		if err != nil || ferial {
			errs = append(errs, fmt.Sprintf("%s: invalid Lauds psalmody declaration (expected festal or antiphon/psalm rows)", key))
			continue
		}
		for _, item := range items {
			if (!strings.HasPrefix(item.psalm, "psalms/") && !strings.HasPrefix(item.psalm, "canticles/")) || !corpus.Has(item.psalm) {
				errs = append(errs, fmt.Sprintf("%s: psalm or canticle ref not found: %s", key, item.psalm))
			}
			if !corpus.HasKeySuffix(item.antiphon) {
				errs = append(errs, fmt.Sprintf("%s: antiphon ref not found: %s", key, item.antiphon))
			}
		}
		if strings.HasSuffix(key, "/"+laudsPsalmodyRef) && !corpus.Has(strings.TrimSuffix(key, laudsPsalmodyRef)+laudsLaudatePsalmodyRef) {
			errs = append(errs, fmt.Sprintf("%s: declared Lauds psalmody requires a Laudate declaration", key))
		}
	}
	return errs
}
