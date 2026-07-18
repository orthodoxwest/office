package office

import (
	"bufio"
	"fmt"
	"sort"
	"strings"

	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/texts"
)

const (
	vespersPsalmodyRef        = "vespers-psalmody"
	defaultVespersPsalmodyKey = "ordinary/vespers/festal-psalmody"
	ferialPsalmodyDeclaration = "ferial"
)

type psalmodyItem struct {
	antiphon string
	psalm    string
}

// parsePsalmodyDeclaration parses one corpus declaration. Each non-empty line
// pairs the symbolic antiphon slot on the left with a concrete psalm key on
// the right, for example "psalm-antiphon-1 = psalms/110". The single word
// "ferial" is a stop marker: it preserves the weekday psalter instead of
// falling through to the shared festal default.
func parsePsalmodyDeclaration(body string) ([]psalmodyItem, bool, error) {
	body = strings.TrimSpace(body)
	if body == ferialPsalmodyDeclaration {
		return nil, true, nil
	}
	if body == "" {
		return nil, false, fmt.Errorf("declaration is empty")
	}

	var items []psalmodyItem
	seenAntiphons := make(map[string]bool)
	scanner := bufio.NewScanner(strings.NewReader(body))
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		antiphon, psalm, found := strings.Cut(line, "=")
		antiphon = strings.TrimSpace(antiphon)
		psalm = strings.TrimSpace(psalm)
		if !found || antiphon == "" || psalm == "" || strings.Contains(psalm, "=") {
			return nil, false, fmt.Errorf("line %d: expected <antiphon-key> = <psalm-key>", lineNumber)
		}
		if strings.ContainsAny(antiphon, " \t") || strings.ContainsAny(psalm, " \t") {
			return nil, false, fmt.Errorf("line %d: corpus keys may not contain whitespace", lineNumber)
		}
		if seenAntiphons[antiphon] {
			return nil, false, fmt.Errorf("line %d: duplicate antiphon key %q", lineNumber, antiphon)
		}
		seenAntiphons[antiphon] = true
		items = append(items, psalmodyItem{antiphon: antiphon, psalm: psalm})
	}
	if err := scanner.Err(); err != nil {
		return nil, false, err
	}
	if len(items) == 0 {
		return nil, false, fmt.Errorf("declaration has no psalms")
	}
	return items, false, nil
}

func vespersPsalmodyCandidates(day *models.CalendarDay) []string {
	if day != nil && day.FirstVespers {
		return []string{vespersPsalmodyRef + "-first", vespersPsalmodyRef}
	}
	return []string{vespersPsalmodyRef}
}

func lookupVespersPsalmody(day *models.CalendarDay, corpus *texts.TextCorpus) (string, string) {
	if day == nil || day.Celebration == nil || corpus == nil || isSynthesizedFeria(day.Celebration) {
		return "", ""
	}
	refs := vespersPsalmodyCandidates(day)

	for _, feastID := range feastProperIDs(day.Celebration) {
		if day.Season == models.Easter {
			if body, key := firstText(corpus, "proper/"+feastID+"-paschal/", refs); body != "" {
				return body, key
			}
		}
		if body, key := firstText(corpus, "proper/"+feastID+"/", refs); body != "" {
			return body, key
		}
	}

	category := day.Celebration.Category
	if category != "" {
		if day.Season == models.Easter {
			if body, key := firstText(corpus, "commons/"+string(category)+"-paschal/", refs); body != "" {
				return body, key
			}
		}
		if body, key := firstText(corpus, "commons/"+string(category)+"/", refs); body != "" {
			return body, key
		}
	}

	if category == "" || category == models.CategoryFeria || category == models.CategorySunday {
		return "", ""
	}
	return corpus.Get(defaultVespersPsalmodyKey), defaultVespersPsalmodyKey
}

// resolveVespersPsalmody follows the same feast-proper then common lookup used
// for other proper texts. A shared standard festal declaration is the final
// data fallback; an explicit "ferial" declaration stops that fallback.
func resolveVespersPsalmody(day *models.CalendarDay, corpus *texts.TextCorpus) ([]psalmodyItem, string, error) {
	body, source := lookupVespersPsalmody(day, corpus)
	if body == "" {
		if source != "" {
			return nil, source, fmt.Errorf("vespers psalmody declaration %q not found", source)
		}
		return nil, source, nil
	}
	items, ferial, err := parsePsalmodyDeclaration(body)
	if err != nil {
		return nil, source, fmt.Errorf("invalid vespers psalmody declaration %q: %w", source, err)
	}
	if ferial {
		return nil, source, nil
	}
	return items, source, nil
}

func usesFestalVespersPsalmody(day *models.CalendarDay, corpus *texts.TextCorpus) bool {
	items, _, err := resolveVespersPsalmody(day, corpus)
	return err == nil && len(items) != 0
}

func composeResolvedPsalmody(day *models.CalendarDay, hourName string, items []psalmodyItem, corpus *texts.TextCorpus) []models.OfficeElement {
	elems := make([]models.OfficeElement, 0, len(items)*4)
	for _, item := range items {
		antiphon := resolveHourElement(day, hourName, HourElement{Type: "proper-antiphon", Ref: item.antiphon}, corpus)
		elems = append(elems,
			antiphon,
			resolveElement(HourElement{Type: "psalm", Ref: item.psalm}, corpus),
			resolveElement(HourElement{Type: "gloria-patri", Ref: "ordinary/shared/gloria-patri"}, corpus),
			antiphon,
		)
	}
	return elems
}

func validateVespersPsalmodyDeclarations(corpus *texts.TextCorpus) []string {
	var errs []string
	for _, key := range corpus.References() {
		if key != defaultVespersPsalmodyKey &&
			!strings.HasPrefix(key, "psalmody/vespers/") &&
			!strings.HasSuffix(key, "/"+vespersPsalmodyRef) &&
			!strings.HasSuffix(key, "/"+vespersPsalmodyRef+"-first") {
			continue
		}
		body := corpus.Get(key)
		items, ferial, err := parsePsalmodyDeclaration(body)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: invalid Vespers psalmody declaration: %v", key, err))
			continue
		}
		if ferial {
			continue
		}
		for _, item := range items {
			if !corpus.Has(item.psalm) {
				errs = append(errs, fmt.Sprintf("%s: psalm ref not found in corpus: %s", key, item.psalm))
			}
			if !corpus.HasKeySuffix(item.antiphon) {
				errs = append(errs, fmt.Sprintf("%s: antiphon ref not found in corpus: %s", key, item.antiphon))
			}
		}
	}
	if !corpus.Has(defaultVespersPsalmodyKey) {
		errs = append(errs, fmt.Sprintf("default Vespers psalmody declaration not found: %s", defaultVespersPsalmodyKey))
	}
	sort.Strings(errs)
	return errs
}
