// Package audit reports placeholder texts and missing feast propers.
package audit

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/texts"
)

// properRefs lists every text ref looked up under proper/{feast-id}/ during
// office composition. The first three are used when the feast is the day's
// celebration; the last three when it appears as a commemoration.
var properRefs = []string{
	"psalm-antiphon",         // Lauds + Vespers (celebration)
	"benedictus-antiphon",    // Lauds (celebration)
	"magnificat-antiphon",    // Vespers (celebration)
	"collect",                // All hours (celebration)
	"commemoration-antiphon", // Lauds + Vespers (commemoration)
	"commemoration-versicle", // Lauds + Vespers (commemoration)
	// commemoration-collect is not audited separately: engine falls back to collect
}

// ordinaryFallbacks maps each proper ref to the ordinary corpus key(s) it
// ultimately falls back to when no feast-specific or seasonal text exists.
var ordinaryFallbacks = map[string][]string{
	"psalm-antiphon":         {"ordinary/lauds/psalm-antiphon", "ordinary/vespers/psalm-antiphon"},
	"benedictus-antiphon":    {"ordinary/lauds/benedictus-antiphon"},
	"magnificat-antiphon":    {"ordinary/vespers/magnificat-antiphon"},
	"collect":                {"ordinary/lauds/collect", "ordinary/vespers/collect", "ordinary/prime/collect", "ordinary/terce/collect", "ordinary/sext/collect", "ordinary/none/collect", "ordinary/compline/collect"},
	"commemoration-antiphon": {"ordinary/lauds/commemoration-antiphon"},
	"commemoration-versicle": {"ordinary/lauds/commemoration-versicle"},
}

// FeastGap describes which proper texts are missing or only commons-covered for a single feast.
type FeastGap struct {
	Feast               *models.Feast
	MissingRefs         []string // refs absent from both proper/{feast-id}/ and commons/{category}/
	CommonsFallbackRefs []string // refs absent from proper/{feast-id}/ but covered by commons/{category}/
	PhFallbackRefs      []string // subset of MissingRefs whose ordinary fallback is itself a placeholder
}

// FlatAntiphonSet describes a file whose indexed psalm antiphons are all identical.
type FlatAntiphonSet struct {
	ID     string
	Name   string
	Rank   models.Rank
	Source models.FeastSource
}

// Report is the full audit result.
type Report struct {
	Placeholders        []string          // corpus keys whose value is placeholder text
	Gaps                []FeastGap        // feasts with missing or commons-only refs (excluding suppressed)
	Suppressed          []string          // feast IDs suppressed via data/audit-ok.txt
	MissingPropName     []string          // feast IDs whose resolved texts contain "N." but have no ProperName
	FlatProperAntiphons []FlatAntiphonSet // proper files with psalm-antiphon-1..5 all identical
	FlatCommonAntiphons []string          // common files with psalm-antiphon-1..5 all identical
	MixedRegister       []string          // refs whose text mixes archaic and modern 2nd-person pronouns
	ModernCollects      []string          // collect refs using modern 2nd-person pronouns
}

var (
	archaicPronounRE = regexp.MustCompile(`(?i)\b(thou|thee|thy|thine)\b`)
	modernPronounRE  = regexp.MustCompile(`(?i)\b(you|your|yours|yourself|yourselves)\b`)
)

// Run loads data from dataDir and returns an audit Report.
func Run(dataDir string) (*Report, error) {
	corpus, err := texts.LoadTexts(dataDir)
	if err != nil {
		return nil, fmt.Errorf("loading texts: %w", err)
	}

	feasts, err := calendar.LoadFeasts(dataDir)
	if err != nil {
		return nil, fmt.Errorf("loading feasts: %w", err)
	}

	suppress, err := loadSuppressFile(dataDir)
	if err != nil {
		return nil, fmt.Errorf("loading audit-ok.txt: %w", err)
	}

	r := &Report{
		Placeholders: corpus.FindPlaceholders(),
	}

	// Build a set of placeholder keys for fast lookup.
	phSet := make(map[string]bool, len(r.Placeholders))
	for _, k := range r.Placeholders {
		phSet[k] = true
	}

	for _, feast := range feasts {
		if feast.Rank == models.Commemoration {
			continue
		}
		suppRefs := suppress[feast.ID]
		if suppRefs["*"] {
			r.Suppressed = append(r.Suppressed, feast.ID)
			continue
		}

		var missing, commonsFallback, phFallback []string
		for _, ref := range properRefs {
			if suppRefs[ref] {
				continue
			}
			if corpus.Has("proper/" + feast.ID + "/" + ref) {
				continue
			}
			if corpus.Has("commons/" + string(feast.Category) + "/" + ref) {
				commonsFallback = append(commonsFallback, ref)
			} else {
				missing = append(missing, ref)
				for _, fb := range ordinaryFallbacks[ref] {
					if phSet[fb] {
						phFallback = append(phFallback, ref)
						break
					}
				}
			}
		}

		if len(missing) > 0 || len(commonsFallback) > 0 {
			r.Gaps = append(r.Gaps, FeastGap{
				Feast:               feast,
				MissingRefs:         missing,
				CommonsFallbackRefs: commonsFallback,
				PhFallbackRefs:      phFallback,
			})
		}
	}

	// Check for feasts whose resolved texts contain "N." but lack ProperName.
	for _, feast := range feasts {
		if feast.Rank == models.Commemoration {
			continue
		}
		if feast.ProperName != "" {
			continue
		}
		for _, ref := range properRefs {
			// Check feast-specific proper
			text := corpus.Get("proper/" + feast.ID + "/" + ref)
			// Check commons fallback
			if text == "" && feast.Category != "" {
				text = corpus.Get("commons/" + string(feast.Category) + "/" + ref)
			}
			if strings.Contains(text, "N.") {
				r.MissingPropName = append(r.MissingPropName, feast.ID)
				break
			}
		}
	}

	r.FlatProperAntiphons = findFlatProperAntiphons(corpus, feasts)
	r.FlatCommonAntiphons = findFlatCommonAntiphons(corpus)
	r.ModernCollects, r.MixedRegister = findTranslationReviewEntries(corpus)

	return r, nil
}

func findFlatProperAntiphons(corpus *texts.TextCorpus, feasts []*models.Feast) []FlatAntiphonSet {
	var flat []FlatAntiphonSet
	for _, feast := range feasts {
		if !hasFlatIndexedPsalmAntiphons(corpus, "proper/"+feast.ID) {
			continue
		}
		flat = append(flat, FlatAntiphonSet{
			ID:     feast.ID,
			Name:   feast.Name,
			Rank:   feast.Rank,
			Source: feast.Source,
		})
	}

	sort.Slice(flat, func(i, j int) bool {
		if flat[i].Source != flat[j].Source {
			return flat[i].Source < flat[j].Source
		}
		wi, wj := flat[i].Rank.Weight(), flat[j].Rank.Weight()
		if wi != wj {
			return wi > wj
		}
		return flat[i].Name < flat[j].Name
	})

	return flat
}

func findFlatCommonAntiphons(corpus *texts.TextCorpus) []string {
	commonIDs := []string{
		"apostle",
		"bishop-martyr",
		"blessed-virgin",
		"confessor",
		"confessor-bishop",
		"confessor-doctor",
		"dedication",
		"evangelist",
		"holy-woman",
		"martyr",
		"virgin",
		"virgin-martyr",
	}

	var flat []string
	for _, id := range commonIDs {
		if hasFlatIndexedPsalmAntiphons(corpus, "commons/"+id) {
			flat = append(flat, id)
		}
	}
	sort.Strings(flat)
	return flat
}

// alleluiaAntiphon is the paschal antiphon under which all psalms of an hour
// are sung. A set of five identical copies of it is the intentional encoding
// of "psalms under one antiphon" (the renderer collapses the repetitions),
// not a flat-antiphon data gap.
const alleluiaAntiphon = "Alleluia, * alleluia, alleluia."

func hasFlatIndexedPsalmAntiphons(corpus *texts.TextCorpus, prefix string) bool {
	var antiphons []string
	for i := 1; i <= 5; i++ {
		ref := fmt.Sprintf("%s/psalm-antiphon-%d", prefix, i)
		text := corpus.Get(ref)
		if text == "" {
			return false
		}
		antiphons = append(antiphons, text)
	}

	first := antiphons[0]
	for _, text := range antiphons[1:] {
		if text != first {
			return false
		}
	}
	return first != alleluiaAntiphon
}

func findTranslationReviewEntries(corpus *texts.TextCorpus) ([]string, []string) {
	var modernCollects []string
	var mixedRegister []string

	for key, text := range corpus.Entries() {
		if !hasReviewablePrefix(key) {
			continue
		}

		hasModern := modernPronounRE.MatchString(text)
		hasArchaic := archaicPronounRE.MatchString(text)

		if hasModern && strings.HasSuffix(key, "/collect") {
			modernCollects = append(modernCollects, key)
		}
		if hasModern && hasArchaic && hasMixedRegisterSection(key) {
			mixedRegister = append(mixedRegister, key)
		}
	}

	sort.Strings(modernCollects)
	sort.Strings(mixedRegister)
	return modernCollects, mixedRegister
}

func hasReviewablePrefix(key string) bool {
	return strings.HasPrefix(key, "proper/") ||
		strings.HasPrefix(key, "commons/") ||
		strings.HasPrefix(key, "ordinary/") ||
		strings.HasPrefix(key, "seasonal/")
}

func hasMixedRegisterSection(key string) bool {
	section := key[strings.LastIndex(key, "/")+1:]
	switch {
	case section == "collect":
		return true
	case section == "psalm-antiphon":
		return true
	case strings.HasPrefix(section, "psalm-antiphon-"):
		return true
	case section == "benedictus-antiphon":
		return true
	case section == "magnificat-antiphon":
		return true
	case section == "commemoration-antiphon":
		return true
	case section == "commemoration-versicle":
		return true
	default:
		return false
	}
}

// printGaps writes a sorted, grouped list of feast gaps to w using the provided
// format function to render each gap line.
func printGaps(w io.Writer, gaps []FeastGap, format func(FeastGap) string) {
	bySource := map[models.FeastSource][]FeastGap{}
	for _, g := range gaps {
		bySource[g.Feast.Source] = append(bySource[g.Feast.Source], g)
	}
	for _, src := range []models.FeastSource{models.SourceBase, models.SourceAWRV} {
		gs := bySource[src]
		if len(gs) == 0 {
			continue
		}
		sort.Slice(gs, func(i, j int) bool {
			wi, wj := gs[i].Feast.Rank.Weight(), gs[j].Feast.Rank.Weight()
			if wi != wj {
				return wi > wj
			}
			return gs[i].Feast.Name < gs[j].Feast.Name
		})
		fmt.Fprintf(w, "\n  [%s]\n", src)
		for _, g := range gs {
			fmt.Fprint(w, format(g))
		}
	}
}

// Print writes a human-readable report to w.
func Print(r *Report, w io.Writer) {
	// ── Placeholders ─────────────────────────────────────────────────────────
	fmt.Fprintf(w, "=== Placeholders: %d corpus entries ===\n", len(r.Placeholders))
	if len(r.Placeholders) > 0 {
		fmt.Fprintln(w, "These are shared texts used as fallbacks — fill these in before adding propers.")
		for _, k := range r.Placeholders {
			fmt.Fprintf(w, "  %s\n", k)
		}
	}
	fmt.Fprintln(w)

	// ── Missing propers ───────────────────────────────────────────────────────
	var gapsMissing, gapsCommons []FeastGap
	for _, g := range r.Gaps {
		if len(g.MissingRefs) > 0 {
			gapsMissing = append(gapsMissing, g)
		} else {
			gapsCommons = append(gapsCommons, g)
		}
	}

	phFallbackCount := 0
	for _, g := range gapsMissing {
		if len(g.PhFallbackRefs) > 0 {
			phFallbackCount++
		}
	}

	fmt.Fprintf(w, "=== Missing propers: %d feast(s) ===\n", len(gapsMissing))
	if len(gapsMissing) > 0 {
		if phFallbackCount == len(gapsMissing) {
			fmt.Fprintln(w, "All feasts fall back to placeholder ordinary texts (marked with !).")
		} else if phFallbackCount > 0 {
			fmt.Fprintf(w, "%d feast(s) fall back to placeholder ordinary texts (marked with !).\n", phFallbackCount)
		}
		printGaps(w, gapsMissing, func(g FeastGap) string {
			note := ""
			if len(g.PhFallbackRefs) > 0 {
				note = " !"
			}
			return fmt.Sprintf("  [%s] %s (%s)%s\n    missing: %s\n",
				g.Feast.Rank.Abbrev(), g.Feast.Name, g.Feast.ID, note,
				strings.Join(g.MissingRefs, ", "))
		})
	}
	fmt.Fprintln(w)

	// ── Commons fallback ──────────────────────────────────────────────────────
	fmt.Fprintf(w, "=== Commons fallback: %d feast(s) ===\n", len(gapsCommons))
	if len(gapsCommons) > 0 {
		fmt.Fprintln(w, "These refs are covered by the category commons, not feast-specific propers.")
		fmt.Fprintln(w, "Add to data/audit-ok.txt to acknowledge and suppress.")
		printGaps(w, gapsCommons, func(g FeastGap) string {
			return fmt.Sprintf("  [%s] %s (%s)\n    commons: %s\n",
				g.Feast.Rank.Abbrev(), g.Feast.Name, g.Feast.ID,
				strings.Join(g.CommonsFallbackRefs, ", "))
		})
	}
	fmt.Fprintln(w)

	// ── Flat antiphon sets ────────────────────────────────────────────────────
	fmt.Fprintf(w, "=== Flat indexed psalm antiphons: %d proper, %d common ===\n", len(r.FlatProperAntiphons), len(r.FlatCommonAntiphons))
	if len(r.FlatProperAntiphons) > 0 || len(r.FlatCommonAntiphons) > 0 {
		fmt.Fprintln(w, "These files have psalm-antiphon-1..5 all set to the same text.")
		fmt.Fprintln(w, "They are likely candidates for Divinum Officium seeding cleanup under the newer antiphon model.")

		if len(r.FlatProperAntiphons) > 0 {
			fmt.Fprintln(w)
			fmt.Fprintln(w, "  [proper]")
			bySource := map[models.FeastSource][]FlatAntiphonSet{}
			for _, item := range r.FlatProperAntiphons {
				bySource[item.Source] = append(bySource[item.Source], item)
			}
			for _, src := range []models.FeastSource{models.SourceBase, models.SourceAWRV} {
				items := bySource[src]
				if len(items) == 0 {
					continue
				}
				fmt.Fprintf(w, "  [%s]\n", src)
				for _, item := range items {
					fmt.Fprintf(w, "    [%s] %s (%s)\n", item.Rank.Abbrev(), item.Name, item.ID)
				}
			}
		}

		if len(r.FlatCommonAntiphons) > 0 {
			fmt.Fprintln(w)
			fmt.Fprintln(w, "  [commons]")
			for _, id := range r.FlatCommonAntiphons {
				fmt.Fprintf(w, "    %s\n", id)
			}
		}
	}
	fmt.Fprintln(w)

	// ── Translation review ────────────────────────────────────────────────────
	fmt.Fprintf(w, "=== Translation review: %d mixed-register, %d modern-pronoun collect(s) ===\n", len(r.MixedRegister), len(r.ModernCollects))
	if len(r.MixedRegister) > 0 || len(r.ModernCollects) > 0 {
		fmt.Fprintln(w, "These entries are high-signal candidates for translation normalization.")
		fmt.Fprintln(w, "Mixed-register entries contain both thou/thee/thy and you/your language in the same text.")

		if len(r.MixedRegister) > 0 {
			fmt.Fprintln(w)
			fmt.Fprintln(w, "  [mixed-register]")
			for _, key := range r.MixedRegister {
				fmt.Fprintf(w, "    %s\n", key)
			}
		}

		if len(r.ModernCollects) > 0 {
			fmt.Fprintln(w)
			fmt.Fprintln(w, "  [modern-pronoun collects]")
			for _, key := range r.ModernCollects {
				fmt.Fprintf(w, "    %s\n", key)
			}
		}
	}
	fmt.Fprintln(w)

	// ── Suppressed ────────────────────────────────────────────────────────────
	if len(r.Suppressed) > 0 {
		fmt.Fprintf(w, "=== Suppressed: %d feast(s) via data/audit-ok.txt ===\n", len(r.Suppressed))
		for _, id := range r.Suppressed {
			fmt.Fprintf(w, "  %s\n", id)
		}
		fmt.Fprintln(w)
	}

	// ── Missing ProperName ──────────────────────────────────────────────────
	if len(r.MissingPropName) > 0 {
		fmt.Fprintf(w, "=== Missing ProperName: %d feast(s) ===\n", len(r.MissingPropName))
		fmt.Fprintln(w, "These feasts have texts containing \"N.\" but no ProperName set.")
		fmt.Fprintln(w, "Add ProperName = <name> to the feast definition to replace \"N.\" with the saint's name.")
		for _, id := range r.MissingPropName {
			fmt.Fprintf(w, "  %s\n", id)
		}
		fmt.Fprintln(w)
	}

	fmt.Fprintln(w, "To mark intentional fallbacks, add lines to data/audit-ok.txt:")
	fmt.Fprintln(w, "  feast-id *                   (suppress all warnings for this feast)")
	fmt.Fprintln(w, "  feast-id ref1 ref2 ...        (suppress specific refs only)")
}

// loadSuppressFile reads data/audit-ok.txt. Missing file is not an error.
// Returns map[feastID]map[ref]bool. A ref of "*" suppresses all warnings.
func loadSuppressFile(dataDir string) (map[string]map[string]bool, error) {
	path := filepath.Join(dataDir, "audit-ok.txt")
	result := make(map[string]map[string]bool)

	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return result, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Fields(line)
		feastID := parts[0]
		refs := parts[1:]
		if result[feastID] == nil {
			result[feastID] = make(map[string]bool)
		}
		if len(refs) == 0 || (len(refs) == 1 && refs[0] == "*") {
			result[feastID]["*"] = true
		} else {
			for _, ref := range refs {
				result[feastID][ref] = true
			}
		}
	}
	return result, scanner.Err()
}
