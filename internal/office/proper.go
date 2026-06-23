package office

import (
	"strings"

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
		return []string{feast.ProperID, feast.ID}
	}
	return []string{feast.ID}
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
		if text, resolved := firstText(corpus, prefix, hourRefCandidates(hourName, ref)); text != "" {
			return text, resolved
		}
		if text, resolved := firstText(corpus, prefix, refCandidates(ref)); text != "" {
			return text, resolved
		}
	}

	// Regular commons
	prefix := "commons/" + cat + "/"
	if text, resolved := firstText(corpus, prefix, hourRefCandidates(hourName, ref)); text != "" {
		return text, resolved
	}
	if text, resolved := firstText(corpus, prefix, refCandidates(ref)); text != "" {
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
	hourCandidates := hourRefCandidates(hourName, ref)
	refCands := refCandidates(ref)

	var properName string
	if day.Celebration != nil {
		properName = day.Celebration.ProperName
	}

	// 1. Feast-specific proper (hour-qualified, then generic)
	if day.Celebration != nil && day.Celebration.ID != "" {
		for _, feastID := range feastProperIDs(day.Celebration) {
			prefix := "proper/" + feastID + "/"
			if text, resolved := firstText(corpus, prefix, hourCandidates); text != "" {
				return substituteProperName(text, properName), resolved
			}
			if text, resolved := firstText(corpus, prefix, refCands); text != "" {
				return substituteProperName(text, properName), resolved
			}
		}
	}

	// 2. Common of Saints (paschal, then regular; hour-qualified, then generic)
	if day.Celebration != nil {
		if text, resolved := lookupCommonsText(day.Celebration.Category, day.Season, hourName, ref, corpus); text != "" {
			return substituteProperName(text, properName), resolved
		}
	}

	// 3. Seasonal default (hour-qualified, then generic)
	if day.Season != "" {
		prefix := "seasonal/" + string(day.Season) + "/"
		if text, resolved := firstText(corpus, prefix, hourCandidates); text != "" {
			return substituteProperName(text, properName), resolved
		}
		if text, resolved := firstText(corpus, prefix, refCands); text != "" {
			return substituteProperName(text, properName), resolved
		}
	}

	// 4. Weekday ordinary (e.g. ordinary/lauds/hymn-monday)
	weekday := strings.ToLower(day.Date.Weekday().String())
	for _, cand := range refCands {
		weekdayRef := "ordinary/" + hourName + "/" + cand + "-" + weekday
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
