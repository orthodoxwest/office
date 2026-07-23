package review

import (
	"encoding/csv"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/office"
)

// DependencyEvidence joins a rendered corpus dependency to its current
// provenance record.
type DependencyEvidence struct {
	Key     string           `json:"key"`
	Status  ProvenanceStatus `json:"status"`
	File    string           `json:"file,omitempty"`
	Section string           `json:"section,omitempty"`
	Sources []SourceCitation `json:"sources,omitempty"`
}

// CompositionAssurance is the machine-readable explanation for one rendered
// hour. It contains no reference-book contents.
type CompositionAssurance struct {
	Date         string                       `json:"date"`
	Hour         string                       `json:"hour"`
	UnitKey      string                       `json:"unit_key"`
	Celebration  string                       `json:"celebration"`
	Season       models.Season                `json:"season"`
	Color        models.Color                 `json:"color"`
	Decisions    []models.CompositionDecision `json:"decisions"`
	Dependencies []DependencyEvidence         `json:"dependencies"`
}

// ExplainComposition composes one hour and joins its complete dependency set
// to the generated provenance inventory.
func ExplainComposition(dataDir, hourName string, date time.Time) (*CompositionAssurance, error) {
	days, err := calendar.BuildCalendar(date.Year(), dataDir)
	if err != nil {
		return nil, err
	}
	idx := date.YearDay() - 1
	if idx < 0 || idx >= len(days) {
		return nil, fmt.Errorf("date out of range: %s", date.Format("2006-01-02"))
	}
	eng, err := office.NewEngine(dataDir)
	if err != nil {
		return nil, err
	}
	hour, err := eng.ComposeHour(hourName, &days[idx], calendar.ComputeMoveableDates(date.Year()))
	if err != nil {
		return nil, err
	}
	inv, err := ScanProvenance(dataDir)
	if err != nil {
		return nil, err
	}
	byKey := inv.ByKey()

	a := &CompositionAssurance{
		Date:        date.Format("2006-01-02"),
		Hour:        hourName,
		UnitKey:     unitKey(&days[idx], hourName),
		Celebration: celebrationName(&days[idx]),
		Season:      hour.Season,
		Color:       hour.Color,
		Decisions:   dedupeDecisions(hour.Decisions),
	}
	for _, ref := range hourDependencies(hour) {
		e, ok := byKey[ref]
		if !ok {
			a.Dependencies = append(a.Dependencies, DependencyEvidence{Key: ref, Status: ProvenanceSourceUnknown})
			continue
		}
		a.Dependencies = append(a.Dependencies, DependencyEvidence{
			Key: ref, Status: e.Status,
			File: e.File, Section: e.Section, Sources: e.Sources,
		})
	}
	return a, nil
}

func hourDependencies(hour *models.OfficeHour) []string {
	seen := map[string]bool{}
	var refs []string
	for _, section := range hour.Sections {
		for _, elem := range section.Elements {
			elemRefs := elem.SourceRefs
			if len(elemRefs) == 0 && elem.SourceRef != "" {
				elemRefs = []string{elem.SourceRef}
			}
			for _, ref := range elemRefs {
				if ref == "" || seen[ref] {
					continue
				}
				seen[ref] = true
				refs = append(refs, ref)
			}
		}
	}
	sort.Strings(refs)
	return refs
}

// HourDependencies returns the sorted, unique corpus keys behind a rendered
// hour. It is safe to expose in reviewer interfaces: it contains no source
// contents or local paths.
func HourDependencies(hour *models.OfficeHour) []string {
	return hourDependencies(hour)
}

func dedupeDecisions(in []models.CompositionDecision) []models.CompositionDecision {
	seen := map[string]bool{}
	out := make([]models.CompositionDecision, 0, len(in))
	for _, d := range in {
		key := d.Rule + "\x1f" + d.Outcome + "\x1f" + d.Detail
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, d)
	}
	return out
}

// UniqueCompositionDecisions removes duplicate trace events while preserving
// their first-seen order.
func UniqueCompositionDecisions(in []models.CompositionDecision) []models.CompositionDecision {
	return dedupeDecisions(in)
}

// ReviewCandidate is one representative composition considered by the set
// cover planner.
type ReviewCandidate struct {
	Hash         string // internal composition identity; omitted from reviewer CSV
	Priority     string
	Hour         string
	Date         time.Time
	UnitKey      string
	Celebration  string
	Context      string
	Dependencies []string
	Decisions    []string
	Features     []string // tier-A (or source) features used for cover
	SignoffState string   // current / stale / unreviewed
}

// PlannedReview records why one candidate was selected.
type PlannedReview struct {
	Candidate   ReviewCandidate
	NewCoverage []string
	NewImpact   int  // fan-out impact of NewCoverage at selection time
	PrimaryYear bool // true when the checklist date is in StartYear
}

// ReviewPlan is a fan-out-weighted coverage plan over tier-A structural
// features (optionally including source keys), reduced by current sign-offs
// that carry the current StructuralFeatureSchema.
type ReviewPlan struct {
	StartYear         int
	Years             int
	CandidateCount    int
	FeatureCount      int // tier-A features observed in the sweep
	Features          []string
	FeatureFanOut     map[string]int
	RenderedKeys      []string
	Selected          []PlannedReview
	IncludeSources    bool
	Uncovered         []string // residual after cover; should be empty without bugs
	CreditedCount     int      // features already covered by schema-current sign-offs
	CreditedFeatures  []string
	TotalImpact       int // sum of fan-out over all tier-A features
	RemainingImpact   int // impact still uncovered after sign-off credit, before selection
	CoveredImpact     int // impact newly covered by selected pages
	FullCoverPages    int // greedy cover size ignoring sign-off credit
	PrimaryYearPages  int // selected pages dated in StartYear
	FutureYearPages   int // selected pages for features absent from StartYear
	CurrentSignoffs   int // live content hashes with a matching sign-off
	CreditingSignoffs int // subset that also carry StructuralFeatureSchema
	StaleSignoffs     int
}

// isTierAStructuralFeature reports whether a feature participates in the
// default structural set-cover. Tier B (context tags, pure weekday psalmody
// section gates, and the redundant occurrence= alias) is excluded so fan-out
// weighting does not burn reviewer time on descriptive state.
func isTierAStructuralFeature(feature string) bool {
	switch {
	case strings.HasPrefix(feature, "source:"):
		return false
	case strings.HasPrefix(feature, "decision:context:"):
		return false
	case strings.HasPrefix(feature, "decision:office-context:"):
		return false
	case strings.HasPrefix(feature, "decision:occurrence="):
		// Redundant with decision:occurrence:resolution-mode=...
		return false
	case isWeekdayPsalmodyNoise(feature):
		return false
	case strings.HasPrefix(feature, "decision:"):
		return true
	case strings.HasPrefix(feature, "resolution:"):
		return true
	default:
		return false
	}
}

// isWeekdayPsalmodyNoise matches section conditions that exist only to pick
// which weekday psalmody block runs. Covering all seven weekdays multiplies
// the plan without adding rubric branches.
func isWeekdayPsalmodyNoise(feature string) bool {
	if !strings.HasPrefix(feature, "decision:condition:") {
		return false
	}
	body := strings.TrimPrefix(feature, "decision:condition:")
	cond, _, _ := strings.Cut(body, "=")
	if strings.Contains(cond, "not-festal-vespers-psalmody,weekday-") {
		return true
	}
	if strings.Contains(cond, "not-is-feast,weekday-") && strings.Contains(cond, "not-festal-lauds-psalmody") {
		return true
	}
	if strings.HasPrefix(cond, "weekday-") && !strings.Contains(cond, ",") {
		return true
	}
	return false
}

// coverFeature reports whether the feature is in the cover universe for this plan.
func coverFeature(feature string, includeSources bool) bool {
	if strings.HasPrefix(feature, "source:") {
		return includeSources
	}
	return isTierAStructuralFeature(feature)
}

// BuildReviewPlan composes the requested years, extracts tier-A structural
// features (and optionally every rendered source key), credits features from
// schema-current sign-offs, then greedily selects unreviewed pages that cover
// the most remaining date-hour fan-out.
func BuildReviewPlan(dataDir string, startYear, years int, includeSources bool) (*ReviewPlan, error) {
	if years < 1 {
		return nil, fmt.Errorf("years must be at least 1")
	}
	eng, err := office.NewEngine(dataDir)
	if err != nil {
		return nil, err
	}

	allFeatures := map[string]bool{}
	featureFanOut := map[string]int{}
	featureInPrimaryYear := map[string]bool{} // features observed in startYear
	renderedKeys := map[string]bool{}
	// One representative candidate per content hash (same composition).
	hashCandidate := map[string]ReviewCandidate{}
	// Feature signature -> content hashes that carry it.
	sigHashes := map[string][]string{}
	rawCount := 0

	for year := startYear; year < startYear+years; year++ {
		days, err := calendar.BuildCalendar(year, dataDir)
		if err != nil {
			return nil, err
		}
		moveable := calendar.ComputeMoveableDates(year)
		for i := range days {
			day := &days[i]
			for _, hourName := range HourNames {
				hour, err := eng.ComposeHour(hourName, day, moveable)
				if err != nil {
					return nil, fmt.Errorf("composing %s for %s: %w", hourName, day.Date.Format("2006-01-02"), err)
				}
				rawCount++
				c := candidateFor(day, hourName, hour, includeSources)
				// Drop bulk dependency text after features are extracted unless
				// source coverage is in play (already folded into Features).
				if !includeSources {
					c.Dependencies = nil
				}
				for _, dependency := range hourDependencies(hour) {
					renderedKeys[dependency] = true
				}
				inPrimary := year == startYear
				for _, f := range c.Features {
					allFeatures[f] = true
					featureFanOut[f]++
					if inPrimary {
						featureInPrimaryYear[f] = true
					}
				}
				if old, ok := hashCandidate[c.Hash]; !ok || betterHashRepresentative(c, old, startYear) {
					hashCandidate[c.Hash] = c
				}
				sig := strings.Join(c.Features, "\x1f")
				seen := false
				for _, h := range sigHashes[sig] {
					if h == c.Hash {
						seen = true
						break
					}
				}
				if !seen {
					sigHashes[sig] = append(sigHashes[sig], c.Hash)
				}
			}
		}
	}

	signoffs, err := LoadSignoffs(dataDir)
	if err != nil {
		return nil, err
	}
	liveHashes := make(map[string]bool, len(hashCandidate))
	for h := range hashCandidate {
		liveHashes[h] = true
	}
	currentHashes := map[string]bool{}
	creditingHashes := map[string]bool{}
	staleSignoffs := 0
	currentSignoffs := 0
	creditingSignoffs := 0
	orphanedByKey := map[string]bool{}
	for i := range signoffs {
		s := &signoffs[i]
		if liveHashes[s.Hash] {
			currentHashes[s.Hash] = true
			currentSignoffs++
			if s.CreditsStructuralFeatures() {
				creditingHashes[s.Hash] = true
				creditingSignoffs++
			}
			continue
		}
		staleSignoffs++
		orphanedByKey[s.Hour+"\x1f"+s.UnitKey] = true
	}

	// Structural credit only from schema-current sign-offs (not every live hash).
	credited := map[string]bool{}
	for hash := range creditingHashes {
		c := hashCandidate[hash]
		for _, f := range c.Features {
			credited[f] = true
		}
	}

	// Collapse each feature signature to one representative, preferring
	// unreviewed pages in the primary (start) year as the checklist link.
	candidates := make([]ReviewCandidate, 0, len(sigHashes))
	for _, hashes := range sigHashes {
		var best ReviewCandidate
		have := false
		for _, h := range hashes {
			c := hashCandidate[h]
			c.SignoffState = signoffStateFor(c, currentHashes, orphanedByKey)
			if !have || betterRepresentative(c, best, startYear) {
				best = c
				have = true
			}
		}
		if have {
			candidates = append(candidates, best)
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		return betterRepresentative(candidates[i], candidates[j], startYear)
	})

	uncovered := make(map[string]bool, len(allFeatures))
	totalImpact := 0
	remainingImpact := 0
	for f := range allFeatures {
		totalImpact += featureFanOut[f]
		if credited[f] {
			continue
		}
		uncovered[f] = true
		remainingImpact += featureFanOut[f]
	}

	// Full-cover size (ignore sign-off credit) for assurance stability.
	fullCoverPages := greedyCoverCount(candidates, allFeatures, featureFanOut, featureInPrimaryYear, startYear)

	plan := &ReviewPlan{
		StartYear: startYear, Years: years, CandidateCount: rawCount,
		IncludeSources: includeSources, FeatureFanOut: featureFanOut,
		TotalImpact: totalImpact, RemainingImpact: remainingImpact,
		FullCoverPages:  fullCoverPages,
		CurrentSignoffs: currentSignoffs, CreditingSignoffs: creditingSignoffs,
		StaleSignoffs: staleSignoffs, CreditedCount: len(credited),
	}
	for f := range credited {
		plan.CreditedFeatures = append(plan.CreditedFeatures, f)
	}
	sort.Strings(plan.CreditedFeatures)
	for key := range renderedKeys {
		plan.RenderedKeys = append(plan.RenderedKeys, key)
	}
	sort.Strings(plan.RenderedKeys)
	for feature := range allFeatures {
		plan.Features = append(plan.Features, feature)
	}
	sort.Strings(plan.Features)
	plan.FeatureCount = len(plan.Features)

	// Two-phase cover: (1) features observed in the primary year, using only
	// primary-year pages; (2) remaining features that never occur in the
	// primary year, allowing later years. Fan-out still weights the full sweep.
	used := make([]bool, len(candidates))
	selectPhase := func(restrictPrimary bool) {
		for len(uncovered) > 0 {
			restrictToUnsigned := residualHasUnsignedCover(candidates, used, uncovered, restrictPrimary, startYear)
			best, bestNew, bestImpact := -1, []string(nil), -1
			for i, c := range candidates {
				if used[i] {
					continue
				}
				if restrictPrimary && c.Date.Year() != startYear {
					continue
				}
				if restrictToUnsigned && c.SignoffState == Current.String() {
					continue
				}
				var newly []string
				impact := 0
				for _, f := range c.Features {
					if !uncovered[f] {
						continue
					}
					// In the primary phase, only spend picks on features that
					// actually appear in the primary year (future-only features
					// wait for phase 2 even if this page also has them).
					if restrictPrimary && !featureInPrimaryYear[f] {
						continue
					}
					newly = append(newly, f)
					impact += featureFanOut[f]
				}
				if len(newly) == 0 {
					continue
				}
				if betterCoverPick(c, impact, len(newly), best < 0, candidates, best, bestImpact, len(bestNew), startYear) {
					best, bestNew, bestImpact = i, newly, impact
				}
			}
			if best < 0 || len(bestNew) == 0 {
				if restrictToUnsigned {
					// Retry allowing currently signed pages in this phase.
					restrictToUnsigned = false
					// Manual retry: re-run loop body once without unsigned filter
					// by clearing the flag and continuing the outer for.
					// Simpler: fall through to a second scan without the flag.
					best, bestNew, bestImpact = -1, nil, -1
					for i, c := range candidates {
						if used[i] {
							continue
						}
						if restrictPrimary && c.Date.Year() != startYear {
							continue
						}
						var newly []string
						impact := 0
						for _, f := range c.Features {
							if !uncovered[f] {
								continue
							}
							if restrictPrimary && !featureInPrimaryYear[f] {
								continue
							}
							newly = append(newly, f)
							impact += featureFanOut[f]
						}
						if len(newly) == 0 {
							continue
						}
						if betterCoverPick(c, impact, len(newly), best < 0, candidates, best, bestImpact, len(bestNew), startYear) {
							best, bestNew, bestImpact = i, newly, impact
						}
					}
					if best < 0 || len(bestNew) == 0 {
						return
					}
				} else {
					return
				}
			}
			used[best] = true
			for _, f := range bestNew {
				delete(uncovered, f)
			}
			sort.Strings(bestNew)
			plan.CoveredImpact += bestImpact
			primary := candidates[best].Date.Year() == startYear
			if primary {
				plan.PrimaryYearPages++
			} else {
				plan.FutureYearPages++
			}
			plan.Selected = append(plan.Selected, PlannedReview{
				Candidate: candidates[best], NewCoverage: bestNew, NewImpact: bestImpact,
				PrimaryYear: primary,
			})
		}
	}
	selectPhase(true)  // primary year first
	selectPhase(false) // future-only residual

	for feature := range uncovered {
		plan.Uncovered = append(plan.Uncovered, feature)
	}
	sort.Strings(plan.Uncovered)
	return plan, nil
}

func residualHasUnsignedCover(candidates []ReviewCandidate, used []bool, uncovered map[string]bool, restrictPrimary bool, primaryYear int) bool {
	for i, c := range candidates {
		if used[i] || c.SignoffState == Current.String() {
			continue
		}
		if restrictPrimary && c.Date.Year() != primaryYear {
			continue
		}
		for _, f := range c.Features {
			if uncovered[f] {
				return true
			}
		}
	}
	return false
}

// greedyCoverCount returns how many pages a full (no-credit) two-phase cover needs.
func greedyCoverCount(candidates []ReviewCandidate, allFeatures map[string]bool, fanOut map[string]int, featureInPrimaryYear map[string]bool, primaryYear int) int {
	uncovered := make(map[string]bool, len(allFeatures))
	for f := range allFeatures {
		uncovered[f] = true
	}
	used := make([]bool, len(candidates))
	pages := 0
	run := func(restrictPrimary bool) {
		for len(uncovered) > 0 {
			best, bestNew, bestImpact := -1, []string(nil), -1
			for i, c := range candidates {
				if used[i] {
					continue
				}
				if restrictPrimary && c.Date.Year() != primaryYear {
					continue
				}
				var newly []string
				impact := 0
				for _, f := range c.Features {
					if !uncovered[f] {
						continue
					}
					if restrictPrimary && !featureInPrimaryYear[f] {
						continue
					}
					newly = append(newly, f)
					impact += fanOut[f]
				}
				if len(newly) == 0 {
					continue
				}
				if betterCoverPick(c, impact, len(newly), best < 0, candidates, best, bestImpact, len(bestNew), primaryYear) {
					best, bestNew, bestImpact = i, newly, impact
				}
			}
			if best < 0 || len(bestNew) == 0 {
				return
			}
			used[best] = true
			pages++
			for _, f := range bestNew {
				delete(uncovered, f)
			}
		}
	}
	run(true)
	run(false)
	return pages
}

func signoffStateFor(c ReviewCandidate, currentHashes, orphanedByKey map[string]bool) string {
	if currentHashes[c.Hash] {
		return Current.String()
	}
	if orphanedByKey[c.Hour+"\x1f"+c.UnitKey] {
		return Stale.String()
	}
	return Unreviewed.String()
}

// betterHashRepresentative picks which date to keep for a content hash.
// Prefer the primary year so checklist links are ordo-verifiable when possible.
func betterHashRepresentative(a, b ReviewCandidate, primaryYear int) bool {
	aPrimary := a.Date.Year() == primaryYear
	bPrimary := b.Date.Year() == primaryYear
	if aPrimary != bPrimary {
		return aPrimary
	}
	return candidateLess(a, b)
}

func betterRepresentative(a, b ReviewCandidate, primaryYear int) bool {
	aDone := a.SignoffState == Current.String()
	bDone := b.SignoffState == Current.String()
	if aDone != bDone {
		return !aDone
	}
	aPrimary := a.Date.Year() == primaryYear
	bPrimary := b.Date.Year() == primaryYear
	if aPrimary != bPrimary {
		return aPrimary
	}
	return candidateLess(a, b)
}

func betterCoverPick(c ReviewCandidate, impact, newCount int, first bool, candidates []ReviewCandidate, best, bestImpact, bestNewCount, primaryYear int) bool {
	if first || best < 0 {
		return true
	}
	if impact != bestImpact {
		return impact > bestImpact
	}
	cDone := c.SignoffState == Current.String()
	bDone := candidates[best].SignoffState == Current.String()
	if cDone != bDone {
		return !cDone
	}
	cPrimary := c.Date.Year() == primaryYear
	bPrimary := candidates[best].Date.Year() == primaryYear
	if cPrimary != bPrimary {
		return cPrimary
	}
	if newCount != bestNewCount {
		return newCount > bestNewCount
	}
	return candidateLess(c, candidates[best])
}

func candidateFor(day *models.CalendarDay, hourName string, hour *models.OfficeHour, includeSources bool) ReviewCandidate {
	u := Unit{Hour: hourName, Rank: celebrationRank(day, hourName), Date: day.Date}
	c := ReviewCandidate{
		Hash: HashHour(hour), Priority: u.Priority(), Hour: hourName, Date: day.Date,
		UnitKey: unitKey(day, hourName), Celebration: celebrationName(day),
		Context: contextNote(day, hourName), Dependencies: hourDependencies(hour),
	}
	featureSet := map[string]bool{}
	if includeSources {
		for _, ref := range c.Dependencies {
			featureSet["source:"+ref] = true
		}
	}
	decisionSet := map[string]bool{}
	for _, d := range dedupeDecisions(hour.Decisions) {
		decision := d.Rule + "=" + d.Outcome
		decisionSet[decision] = true
		feat := "decision:" + decision
		if coverFeature(feat, includeSources) {
			featureSet[feat] = true
		}
	}
	for decision := range decisionSet {
		feat := "decision:" + decision
		if coverFeature(feat, includeSources) {
			c.Decisions = append(c.Decisions, decision)
		}
	}
	for _, section := range hour.Sections {
		for _, elem := range section.Elements {
			if elem.SlotRef == "" || elem.SourceRef == "" {
				continue
			}
			tier, _, _ := strings.Cut(elem.SourceRef, "/")
			feat := "resolution:" + elem.SlotRef + "=" + tier
			if coverFeature(feat, includeSources) {
				featureSet[feat] = true
			}
		}
	}
	sort.Strings(c.Decisions)
	for f := range featureSet {
		c.Features = append(c.Features, f)
	}
	sort.Strings(c.Features)
	return c
}

func candidateLess(a, b ReviewCandidate) bool {
	if a.Priority != b.Priority {
		return a.Priority < b.Priority
	}
	if hourTier[a.Hour] != hourTier[b.Hour] {
		return hourTier[a.Hour] < hourTier[b.Hour]
	}
	if !a.Date.Equal(b.Date) {
		return a.Date.Before(b.Date)
	}
	return hourOrder[a.Hour] < hourOrder[b.Hour]
}

// PrintReviewPlanSummary writes compact generated coverage counts.
func PrintReviewPlanSummary(p *ReviewPlan, w io.Writer) {
	fmt.Fprintf(w, "=== Coverage-oriented review plan: %d-%d ===\n", p.StartYear, p.StartYear+p.Years-1)
	fmt.Fprintf(w, "  composed candidates:   %d\n", p.CandidateCount)
	fmt.Fprintf(w, "  structural features:   %d\n", p.FeatureCount)
	fmt.Fprintf(w, "  credited by sign-off:  %d\n", p.CreditedCount)
	fmt.Fprintf(w, "  total feature impact:  %d\n", p.TotalImpact)
	fmt.Fprintf(w, "  residual impact:       %d\n", p.RemainingImpact)
	fmt.Fprintf(w, "  full-cover pages:      %d\n", p.FullCoverPages)
	fmt.Fprintf(w, "  residual pages:        %d\n", len(p.Selected))
	fmt.Fprintf(w, "  primary-year pages:    %d\n", p.PrimaryYearPages)
	fmt.Fprintf(w, "  future-year pages:     %d\n", p.FutureYearPages)
	fmt.Fprintf(w, "  impact covered:        %d\n", p.CoveredImpact)
	fmt.Fprintf(w, "  current sign-offs:     %d\n", p.CurrentSignoffs)
	fmt.Fprintf(w, "  crediting sign-offs:   %d\n", p.CreditingSignoffs)
	fmt.Fprintf(w, "  stale sign-offs:       %d\n", p.StaleSignoffs)
	fmt.Fprintf(w, "  source-key coverage:   %t\n", p.IncludeSources)
	fmt.Fprintf(w, "  uncovered features:    %d\n", len(p.Uncovered))
}

// WriteReviewPlanCSV writes the selected review pages in impact order.
// Columns stay reviewer-facing: no bulk dependency dumps or internal hashes.
func WriteReviewPlanCSV(p *ReviewPlan, w io.Writer, baseURL string) error {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	baseURL = strings.TrimRight(baseURL, "/")
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{
		"order", "priority", "hour", "date", "unit_key", "celebration", "context",
		"signoff_status", "primary_year", "new_impact", "new_features", "url",
	})
	for i, selected := range p.Selected {
		c := selected.Candidate
		primary := "no"
		if selected.PrimaryYear {
			primary = "yes"
		}
		_ = cw.Write([]string{
			fmt.Sprint(i + 1), c.Priority, c.Hour, c.Date.Format("2006-01-02"),
			c.UnitKey, c.Celebration, c.Context, c.SignoffState, primary,
			fmt.Sprint(selected.NewImpact),
			strings.Join(selected.NewCoverage, "; "),
			baseURL + "/" + c.Hour + "/" + c.Date.Format("2006-01-02"),
		})
	}
	cw.Flush()
	return cw.Error()
}
