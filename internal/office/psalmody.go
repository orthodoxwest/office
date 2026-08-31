package office

import (
	"bufio"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/texts"
)

const (
	vespersPsalmodyRef                            = "vespers-psalmody"
	defaultVespersPsalmodyKey                     = "ordinary/vespers/festal-psalmody"
	ferialPsalmodyDeclaration                     = "ferial"
	ferialPsalmodyWithWeekdayAntiphonsDeclaration = "ferial weekday-antiphons"
)

type psalmodyItem struct {
	slot     string
	antiphon string
	psalm    string
	dates    map[string]bool
}

// parsePsalmodyDeclaration parses one corpus declaration. Each non-empty line
// pairs the symbolic antiphon slot on the left with a concrete psalm key on
// the right, for example "psalm-antiphon-1 = psalms/110". A row may carry a
// fixed-date selector, for example "... = psalms/130 dates=12-25,12-27";
// disjoint alternatives for the same antiphon slot are allowed. An alternative
// whose antiphon text also differs may add "antiphon=<corpus-key>". The single
// word "ferial" is a stop marker: it preserves the weekday psalms instead of
// falling through to the shared festal default, while still allowing proper
// antiphons. "ferial weekday-antiphons" also preserves the weekday psalter's
// antiphons.
func parsePsalmodyDeclaration(body string) ([]psalmodyItem, bool, error) {
	body = strings.TrimSpace(body)
	if body == ferialPsalmodyDeclaration ||
		body == ferialPsalmodyWithWeekdayAntiphonsDeclaration {
		return nil, true, nil
	}
	if body == "" {
		return nil, false, fmt.Errorf("declaration is empty")
	}

	var items []psalmodyItem
	type antiphonAlternatives struct {
		unconditional bool
		dates         map[string]bool
	}
	seenAntiphons := make(map[string]antiphonAlternatives)
	scanner := bufio.NewScanner(strings.NewReader(body))
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		antiphon, rhs, found := strings.Cut(line, "=")
		antiphon = strings.TrimSpace(antiphon)
		rhs = strings.TrimSpace(rhs)
		if !found || antiphon == "" || rhs == "" {
			return nil, false, fmt.Errorf("line %d: expected <antiphon-key> = <psalm-key>", lineNumber)
		}

		fields := strings.Fields(rhs)
		if len(fields) == 0 || len(fields) > 3 || strings.Contains(fields[0], "=") {
			return nil, false, fmt.Errorf("line %d: expected <antiphon-key> = <psalm-key> [dates=MM-DD,...] [antiphon=<corpus-key>]", lineNumber)
		}
		psalm := fields[0]
		antiphonRef := antiphon
		var dates map[string]bool
		for _, option := range fields[1:] {
			switch {
			case strings.HasPrefix(option, "dates="):
				if dates != nil {
					return nil, false, fmt.Errorf("line %d: duplicate dates option", lineNumber)
				}
				value := strings.TrimPrefix(option, "dates=")
				if value == "" {
					return nil, false, fmt.Errorf("line %d: expected a non-empty dates=MM-DD list", lineNumber)
				}
				dates = make(map[string]bool)
				for date := range strings.SplitSeq(value, ",") {
					parsed, err := time.Parse("01-02", date)
					if err != nil || parsed.Format("01-02") != date {
						return nil, false, fmt.Errorf("line %d: invalid declaration date %q (expected MM-DD)", lineNumber, date)
					}
					if dates[date] {
						return nil, false, fmt.Errorf("line %d: duplicate declaration date %q", lineNumber, date)
					}
					dates[date] = true
				}
			case strings.HasPrefix(option, "antiphon="):
				if antiphonRef != antiphon {
					return nil, false, fmt.Errorf("line %d: duplicate antiphon option", lineNumber)
				}
				antiphonRef = strings.TrimPrefix(option, "antiphon=")
				if antiphonRef == "" {
					return nil, false, fmt.Errorf("line %d: antiphon option requires a corpus key", lineNumber)
				}
			default:
				return nil, false, fmt.Errorf("line %d: unknown declaration option %q", lineNumber, option)
			}
		}
		if strings.ContainsAny(antiphon, " \t") || strings.ContainsAny(psalm, " \t") || strings.ContainsAny(antiphonRef, " \t") {
			return nil, false, fmt.Errorf("line %d: corpus keys may not contain whitespace", lineNumber)
		}

		seen, exists := seenAntiphons[antiphon]
		if len(dates) == 0 {
			if exists {
				return nil, false, fmt.Errorf("line %d: duplicate antiphon key %q", lineNumber, antiphon)
			}
			seen.unconditional = true
		} else {
			if seen.unconditional {
				return nil, false, fmt.Errorf("line %d: conditional alternative overlaps unconditional antiphon key %q", lineNumber, antiphon)
			}
			if seen.dates == nil {
				seen.dates = make(map[string]bool)
			}
			for date := range dates {
				if seen.dates[date] {
					return nil, false, fmt.Errorf("line %d: antiphon key %q repeats date %s", lineNumber, antiphon, date)
				}
				seen.dates[date] = true
			}
		}
		seenAntiphons[antiphon] = seen
		items = append(items, psalmodyItem{slot: antiphon, antiphon: antiphonRef, psalm: psalm, dates: dates})
	}
	if err := scanner.Err(); err != nil {
		return nil, false, err
	}
	if len(items) == 0 {
		return nil, false, fmt.Errorf("declaration has no psalms")
	}
	return items, false, nil
}

func selectPsalmodyItems(items []psalmodyItem, date time.Time) ([]psalmodyItem, error) {
	monthDay := date.Format("01-02")
	selected := make([]psalmodyItem, 0, len(items))
	expected := make(map[string]bool)
	counts := make(map[string]int)
	for _, item := range items {
		expected[item.slot] = true
		if len(item.dates) != 0 && !item.dates[monthDay] {
			continue
		}
		selected = append(selected, item)
		counts[item.slot]++
	}
	for antiphon := range expected {
		if counts[antiphon] == 0 {
			return nil, fmt.Errorf("antiphon key %q has no alternative for date %s", antiphon, monthDay)
		}
		if counts[antiphon] > 1 {
			return nil, fmt.Errorf("antiphon key %q has multiple alternatives for date %s", antiphon, monthDay)
		}
	}
	return selected, nil
}

func vespersPsalmodyCandidates(day *models.CalendarDay) []string {
	if day != nil && day.FirstVespers {
		return []string{vespersPsalmodyRef + "-first", vespersPsalmodyRef}
	}
	return []string{vespersPsalmodyRef}
}

func hasFeastProperVespersPsalmAntiphons(day *models.CalendarDay, corpus *texts.TextCorpus) bool {
	if day == nil || day.Celebration == nil || corpus == nil {
		return false
	}
	for _, feastID := range feastProperIDs(day.Celebration) {
		if day.Season == models.Easter {
			if text, _ := lookupSectionText("proper/"+feastID+"-paschal/", "", "vespers", "psalm-antiphon-1", corpus); text != "" {
				return true
			}
		}
		if text, _ := lookupSectionText("proper/"+feastID+"/", day.Season, "vespers", "psalm-antiphon-1", corpus); text != "" {
			return true
		}
	}
	return false
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

	// A plain Double with neither a feast-proper psalmody declaration nor
	// feast-proper Vespers psalm antiphons takes the civil weekday psalter
	// (General Rubrics XX.1, XXV.4). This gate belongs before the Common so a
	// Common cannot manufacture festal psalmody. An office within an octave
	// retains that octave's psalmody context.
	//
	// A Simple takes the weekday psalter on the same footing: General Rubrics
	// III.1 defines the Simple Office as the office of the Feria, and the ordos
	// print the ferial psalms on these evenings ("Fri. Ps." on the eve of a
	// Simple octave day, 2022-2026).
	if (day.Celebration.Rank == models.Double || day.Celebration.Rank == models.Simple) &&
		day.WithinOctaveOf == "" &&
		!hasFeastProperVespersPsalmAntiphons(day, corpus) {
		return ferialPsalmodyWithWeekdayAntiphonsDeclaration, "rubric/plain-double-ferial-vespers"
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
	date := day.Date
	if day.FirstVespers {
		// vespersOfficeDay advances Date to the following feast so ordinary
		// proper resolution uses its liturgical date. Fixed-date psalmody
		// appointments, however, are printed for the civil evening on which
		// Vespers is sung (notably Dec. 30 I Vespers of an octave Sunday).
		date = date.AddDate(0, 0, -1)
	}
	items, err = selectPsalmodyItems(items, date)
	if err != nil {
		return nil, source, fmt.Errorf("invalid vespers psalmody declaration %q: %w", source, err)
	}
	return items, source, nil
}

// officeOfTheDeadIDs are the celebrations composed with the Office of the
// Dead, which differs from every other office in more than its texts: five
// psalms at Vespers rather than four, no Gloria Patri, no opening versicles,
// and no chapter, hymn or short responsory at either hour. The Commemoration
// of All the Departed O.S.B. (Nov 14) belongs here too once it is in the
// kalendar. Hour definitions ask for it by the "office-of-the-dead" condition
// rather than by feast ID, so a second entry here reaches the data too.
var officeOfTheDeadIDs = map[string]bool{
	"all-souls": true,
}

// isOfficeOfTheDead reports whether the day's office is the Office of the Dead.
func isOfficeOfTheDead(day *models.CalendarDay) bool {
	return day != nil && day.Celebration != nil && officeOfTheDeadIDs[day.Celebration.ID]
}

// vespersOfTheDeadLabel is the section heading for Vespers of the Dead when it
// is appended after the day's Vespers on the evening before All Souls.
const vespersOfTheDeadLabel = "Vespers of the Dead"

// deadOfficeDay returns the synthetic office used to compose Vespers of the
// Dead and Compline of the Dead on the evening before All Souls. Colour is
// black; commemorations of the civil day do not belong to the Dead office.
func deadOfficeDay(day *models.CalendarDay) *models.CalendarDay {
	if day == nil || day.Vespers.AppendedFeast == nil {
		return day
	}
	feast := day.Vespers.AppendedFeast
	out := *day
	out.Celebration = feast
	out.Color = models.Black
	out.Commemorations = nil
	out.FeriaCommemoration = nil
	out.FirstVespers = false
	out.FollowingOfficeCommemorationID = ""
	out.Vespers = models.VespersDesignation{
		Owner:                   models.VespersIIOfPreceding,
		Feast:                   feast,
		Color:                   models.Black,
		Season:                  day.Season,
		AppendedOfficeOfTheDead: true,
		AppendedFeast:           feast,
		Rule:                    "vespers:appended-office-of-the-dead",
	}
	return &out
}

// What concludes each psalm and gospel canticle. The Office of the Dead says
// no Gloria Patri: "At the end of all Psalms is always said: Rest eternal
// grant unto them, O Lord. And let light perpetual shine upon them, even if
// the Office be said for one person only" (Monastic Diurnal p. 72*).
const (
	doxologyGloriaPatri = "ordinary/shared/gloria-patri"
	doxologyRestEternal = "shared/formulas/rest-eternal"

	// doxologyRefPerOffice is the Ref an hour definition uses to defer the
	// choice to the office of the day, as "marian" defers to "seasonal".
	doxologyRefPerOffice = "office"
)

// psalmDoxologyKeys is every key psalmDoxologyRef can return, so the
// hour-definition validator can require them all.
var psalmDoxologyKeys = []string{doxologyGloriaPatri, doxologyRestEternal}

// psalmDoxologyRef returns the corpus key for what concludes each psalm and
// gospel canticle of the day's office. Callers must first ask
// saysPsalmDoxology, since some offices conclude them with nothing at all.
func psalmDoxologyRef(day *models.CalendarDay) string {
	if isOfficeOfTheDead(day) {
		return doxologyRestEternal
	}
	return doxologyGloriaPatri
}

// saysPsalmDoxology reports whether the office concludes its psalms and gospel
// canticles with anything at all. Nothing is said during the Triduum: the
// Monastic Diurnal prints those psalms with the antiphon following straight on
// from the last verse (p. 311 at Lauds, and the same form at the Little Hours
// on p. 314). The choice belongs to the office rather than to the hour
// definition, so definitions that name the Gloria Patri outright still defer
// here and one rubric governs every hour.
//
// The rubric bounds itself: "Thus are ended all the Hours during this Triduum
// through None of Holy Sabbath" (p. 313). Holy Saturday's evening belongs to
// Easter, which says the Gloria Patri again — its Vespers already resolves
// against Easter, but Compline keeps the civil day for the sections proper to
// Holy Saturday, so the bound has to be stated rather than inferred.
func saysPsalmDoxology(day *models.CalendarDay, hourName string) bool {
	if !isTriduum(day) {
		return true
	}
	if day.Celebration.ID == "holy-saturday" && (hourName == "vespers" || hourName == "compline") {
		return true
	}
	return false
}

func usesFestalVespersPsalmody(day *models.CalendarDay, corpus *texts.TextCorpus) bool {
	items, _, err := resolveVespersPsalmody(day, corpus)
	return err == nil && len(items) != 0
}

func usesWeekdayVespersAntiphons(day *models.CalendarDay, corpus *texts.TextCorpus) bool {
	body, _ := lookupVespersPsalmody(day, corpus)
	return strings.TrimSpace(body) == ferialPsalmodyWithWeekdayAntiphonsDeclaration
}

func composeResolvedPsalmody(day *models.CalendarDay, hourName string, items []psalmodyItem, corpus *texts.TextCorpus) []models.OfficeElement {
	elems := make([]models.OfficeElement, 0, len(items)*4)
	for _, item := range items {
		antiphon := resolveHourElement(day, hourName, HourElement{Type: "proper-antiphon", Ref: item.antiphon}, corpus)
		elems = append(elems,
			antiphon,
			resolveElement(HourElement{Type: "psalm", Ref: item.psalm}, corpus),
		)
		if saysPsalmDoxology(day, hourName) {
			elems = append(elems, resolveElement(HourElement{Type: "gloria-patri", Ref: psalmDoxologyRef(day)}, corpus))
		}
		elems = append(elems, antiphon)
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
