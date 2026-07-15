package office

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/orthodoxwest/office/internal/texts"
)

// hourNames lists the canonical hour names (stems of the office/*.txt files).
var hourNames = []string{"lauds", "prime", "terce", "sext", "none", "vespers", "compline"}

// isValidCondition mirrors the pattern logic of evaluateCondition to validate
// a condition string without needing runtime calendar data.
// Comma-separated conditions are ANDed; "not-" prefix negates; "feast-*" and
// "weekday-*" prefixes are structural patterns; "if-preces" is a named condition.
func isValidCondition(condition string) bool {
	if strings.Contains(condition, ",") {
		for _, part := range strings.Split(condition, ",") {
			if !isValidCondition(strings.TrimSpace(part)) {
				return false
			}
		}
		return true
	}
	if strings.HasPrefix(condition, "not-") {
		return isValidCondition(condition[4:])
	}
	switch condition {
	case "if-preces", "is-feast", "is-ferial", "festal-lauds-psalmody", "if-suffrage", "if-cross-commemoration":
		return true
	}
	if strings.HasPrefix(condition, "feast-") {
		return len(condition) > 6 // feast ID must be non-empty
	}
	if strings.HasPrefix(condition, "weekday-") {
		day := condition[8:]
		switch day {
		case "sunday", "monday", "tuesday", "wednesday", "thursday", "friday", "saturday":
			return true
		}
		return false
	}
	if strings.HasPrefix(condition, "season-") {
		return len(condition) > 7 // season name must be non-empty
	}
	return false
}

// validElementTypes is the set of Type values recognised by mapElementType.
// Unknown values map silently to "rubric".
var validElementTypes = map[string]bool{
	"psalm":             true,
	"canticle":          true,
	"hymn":              true,
	"antiphon":          true,
	"versicle":          true,
	"response":          true,
	"prayer":            true,
	"preces":            true,
	"rubric":            true,
	"chapter":           true,
	"collect":           true,
	"blessing":          true,
	"marian":            true,
	"proper-antiphon":   true,
	"proper-collect":    true,
	"proper-hymn":       true,
	"proper-responsory": true,
	"proper-versicle":   true,
	"proper-chapter":    true,
	"commemorations":    true,
	"gloria-patri":      true,
}

func validationHours(hour, elemType, ref string) []string {
	if elemType == "proper-collect" && ref == "collect" {
		switch hour {
		case "terce", "sext", "none":
			return []string{"lauds"}
		}
	}
	return []string{hour}
}

func hasAnyKeySuffix(corpus *texts.TextCorpus, refs []string) bool {
	for _, ref := range refs {
		if corpus.HasKeySuffix(ref) {
			return true
		}
	}
	return false
}

// ValidateHourDefinitions parses all hour definition files in dataDir/office/,
// checks that every Ref resolves to a corpus key, and returns all errors found.
//
// Three kinds of refs are handled:
//   - Static refs (most element types): the Ref string must be a direct corpus key.
//   - proper-*: the Ref is a symbolic name; the engine checks hour-specific first
//     ("ordinary/<hour>/<ref>"), then shared ("ordinary/shared/<ref>").
//   - marian + ref "seasonal": resolved at runtime to one of five hardcoded
//     Marian antiphon keys, all of which must be corpus keys.
//   - commemorations: fully dynamic; nothing to validate.
func ValidateHourDefinitions(dataDir string) []string {
	corpus, err := texts.LoadTexts(dataDir)
	if err != nil {
		return []string{fmt.Sprintf("loading text corpus: %v", err)}
	}

	var parseErrors []string
	// required maps corpus key → first source location for error messages
	required := make(map[string]string)

	for _, hour := range hourNames {
		defPath := filepath.Join(dataDir, "office", hour+".txt")
		sections, err := ParseHourDefinition(defPath)
		if err != nil {
			parseErrors = append(parseErrors, fmt.Sprintf("office/%s.txt: %v", hour, err))
			continue
		}

		for _, section := range sections {
			src := fmt.Sprintf("office/%s.txt [%s]", hour, section.Name)

			if section.Condition != "" && !isValidCondition(section.Condition) {
				parseErrors = append(parseErrors, fmt.Sprintf(
					"%s: unknown Condition %q", src, section.Condition,
				))
			}

			for _, elem := range section.Elements {
				if !validElementTypes[elem.Type] {
					parseErrors = append(parseErrors, fmt.Sprintf(
						"%s: unknown element Type %q", src, elem.Type,
					))
				}
				switch elem.Type {
				case "proper-antiphon", "proper-collect", "proper-hymn", "proper-responsory", "proper-versicle", "proper-chapter":
					// Symbolic ref; validate that the ordinary fallback exists.
					// The engine tries (mirroring resolveProperText / resolveProperCollectText):
					//   1. hour-specific   ordinary/<hour>/<ref>
					//   2. shared          ordinary/shared/<ref>
					//   3. weekday per-day ordinary/<hour>/<ref>-<weekday>
					refCands := refCandidates(elem.Ref)
					foundOrdinary := false
					for _, validationHour := range validationHours(hour, elem.Type, elem.Ref) {
						for _, cand := range refCands {
							hourRef := "ordinary/" + validationHour + "/" + cand
							if corpus.Has(hourRef) {
								foundOrdinary = true
								if _, seen := required[hourRef]; !seen {
									required[hourRef] = src
								}
							}
						}

						// Check for weekday variants (e.g. ordinary/lauds/hymn-monday).
						weekdays := []string{"sunday", "monday", "tuesday", "wednesday", "thursday", "friday", "saturday"}
						for _, cand := range refCands {
							hourRef := "ordinary/" + validationHour + "/" + cand
							for _, wd := range weekdays {
								wdRef := hourRef + "-" + wd
								if corpus.Has(wdRef) {
									foundOrdinary = true
									if _, seen := required[wdRef]; !seen {
										required[wdRef] = src
									}
								}
							}
						}
					}

					for _, cand := range refCands {
						sharedRef := "ordinary/shared/" + cand
						if corpus.Has(sharedRef) {
							foundOrdinary = true
							if _, seen := required[sharedRef]; !seen {
								required[sharedRef] = src
							}
						}
					}

					// If no ordinary path exists at all, check feast/seasonal corpus paths.
					// These are resolved dynamically at runtime via resolveProperText.
					if !foundOrdinary && !hasAnyKeySuffix(corpus, refCands) {
						parseErrors = append(parseErrors, fmt.Sprintf(
							"%s: proper ref %q not found in corpus (checked ordinary, shared, weekday, and feast/seasonal paths)",
							src, elem.Ref,
						))
					}

				case "marian":
					if elem.Ref != "seasonal" {
						parseErrors = append(parseErrors, fmt.Sprintf(
							"%s: marian element has unsupported ref %q (expected \"seasonal\")", src, elem.Ref,
						))
						continue
					}
					// All five seasonal Marian antiphon keys must be in the corpus.
					for _, key := range marianCorpusKeys {
						if _, seen := required[key]; !seen {
							required[key] = src
						}
					}

				case "commemorations":
					// Resolved dynamically at runtime; nothing to validate.

				default:
					// Direct corpus lookup.
					if _, seen := required[elem.Ref]; !seen {
						required[elem.Ref] = src
					}
				}
			}
		}
	}

	var refErrors []string
	for ref, src := range required {
		if !corpus.Has(ref) {
			refErrors = append(refErrors, fmt.Sprintf("%s: ref not found in corpus: %s", src, ref))
		}
	}
	sort.Strings(refErrors)

	return append(parseErrors, refErrors...)
}

// marianCorpusKeys are the five corpus keys that a "marian seasonal" element
// may resolve to at runtime (one per liturgical period).
var marianCorpusKeys = []string{
	"ordinary/marian/alma-redemptoris-advent",
	"ordinary/marian/alma-redemptoris-christmas",
	"ordinary/marian/ave-regina-caelorum",
	"ordinary/marian/regina-caeli",
	"ordinary/marian/salve-regina",
}
