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
	Form         models.PrayerForm            `json:"form"`
	Date         string                       `json:"date"`
	Hour         string                       `json:"hour"`
	UnitKey      string                       `json:"unit_key"`
	Celebration  string                       `json:"celebration"`
	Season       models.Season                `json:"season"`
	Color        models.Color                 `json:"color"`
	Decisions    []models.CompositionDecision `json:"decisions"`
	Dependencies []DependencyEvidence         `json:"dependencies"`
	Resolutions  []ResolutionEvidence         `json:"resolutions,omitempty"`
}

// ResolutionEvidence explains a rendered dynamic slot without carrying its
// text. It is derived after composition and deliberately remains outside the
// OfficeElement/hash model.
type ResolutionEvidence struct {
	RequestedSlot    string   `json:"requested_slot"`
	ResolverHour     string   `json:"resolver_hour"`
	ResolverSlot     string   `json:"resolver_slot"`
	CanonicalOwner   string   `json:"canonical_owner,omitempty"`
	ProperIDs        []string `json:"proper_ids,omitempty"`
	DirectCandidates []string `json:"direct_candidates,omitempty"`
	DirectExisting   []string `json:"direct_existing,omitempty"`
	SelectedRef      string   `json:"selected_ref"`
	SelectedTier     string   `json:"selected_tier"`
	Reason           string   `json:"reason"`
	FirstVespers     bool     `json:"first_vespers,omitempty"`
}

// ExplainComposition composes one hour and joins its complete dependency set
// to the generated provenance inventory.
func ExplainComposition(dataDir, hourName string, date time.Time, forms ...models.PrayerForm) (*CompositionAssurance, error) {
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
	hour, err := eng.ComposeHourWithOptions(hourName, &days[idx], calendar.ComputeMoveableDates(date.Year()), office.ComposeOptions{Form: requestedPrayerForm(forms)})
	if err != nil {
		return nil, err
	}
	inv, err := ScanProvenance(dataDir)
	if err != nil {
		return nil, err
	}
	byKey := inv.ByKey()

	a := &CompositionAssurance{
		Form:        hour.Form,
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
	for _, section := range hour.Sections {
		for _, elem := range section.Elements {
			if elem.SlotRef == "" {
				continue
			}
			trace := traceInventoryElement(eng, &days[idx], hourName, elem)
			if !shouldIncludeResolution(elem.SourceRef, trace.SelectedTier) {
				continue
			}
			a.Resolutions = append(a.Resolutions, ResolutionEvidence{
				RequestedSlot: trace.RequestedSlot, CanonicalOwner: trace.CanonicalOwner,
				ResolverHour: trace.ResolverHour, ResolverSlot: trace.ResolverSlot,
				ProperIDs: trace.ProperIDs, DirectCandidates: trace.DirectCandidates,
				DirectExisting: trace.DirectExisting, SelectedRef: trace.SelectedRef,
				SelectedTier: trace.SelectedTier, Reason: trace.Reason,
				FirstVespers: trace.FirstVespers,
			})
		}
	}
	return a, nil
}

func isResolutionSource(ref string) bool {
	return strings.HasPrefix(ref, "proper/") || strings.HasPrefix(ref, "commons/") ||
		strings.HasPrefix(ref, "seasonal/") || strings.HasPrefix(ref, "ordinary/")
}

func shouldIncludeResolution(sourceRef, selectedTier string) bool {
	return isResolutionSource(sourceRef) || selectedTier == "not-found"
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

// ReviewCandidate is one composition considered by the optional sample planner.
type ReviewCandidate struct {
	Form         models.PrayerForm
	Hash         string // internal composition identity; omitted from sample CSV
	Priority     string
	Hour         string
	Date         time.Time
	UnitKey      string
	Celebration  string
	Context      string
	Dependencies []string
	Decisions    []string
	Features     []string // observed engine decisions and source tiers, not rubric requirements
}

// PlannedReview records why one sample was selected.
type PlannedReview struct {
	Candidate   ReviewCandidate
	NewFeatures []string
	Exposure    int // sum of occurrences of NewFeatures; a date-hour may count more than once
	PrimaryYear bool
}

// ReviewPlan samples observed engine behavior. It cannot establish rubric
// completeness, correctness, or coverage of interactions between features.
type ReviewPlan struct {
	StartYear        int
	Years            int
	CandidateCount   int
	Features         []string
	Selected         []PlannedReview
	IncludeSources   bool
	PrimaryYearPages int
	FutureYearPages  int
}

// isSampleFeature reports whether a feature participates in the
// default sample. Context tags, pure weekday psalmody section gates, and the
// redundant occurrence= alias are excluded to keep the sample compact.
// These exclusions do not establish that the omitted appointments are correct.
func isSampleFeature(feature string) bool {
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
	return isSampleFeature(feature)
}

// BuildReviewPlan selects examples of observed engine decisions and source
// tiers, weighted by their frequency. It always starts from the whole observed
// inventory; historical page signoffs have no effect on sampling.
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
	// One representative per observed feature combination, preferring the
	// primary year. Different combinations may share the same rendered text.
	representatives := map[string]ReviewCandidate{}
	plan := &ReviewPlan{StartYear: startYear, Years: years, IncludeSources: includeSources}
	for year := startYear; year < startYear+years; year++ {
		days, err := calendar.BuildCalendar(year, dataDir)
		if err != nil {
			return nil, err
		}
		moveable := calendar.ComputeMoveableDates(year)
		for i := range days {
			day := &days[i]
			for _, hourName := range HourNames {
				forms, err := composeReviewForms(eng, hourName, day, moveable)
				if err != nil {
					return nil, fmt.Errorf("composing %s for %s: %w", hourName, day.Date.Format("2006-01-02"), err)
				}
				for _, hour := range forms {
					plan.CandidateCount++
					c := candidateFor(day, hourName, hour, includeSources)
					c.Dependencies = nil // source keys, when requested, are already in Features
					for _, f := range c.Features {
						allFeatures[f] = true
						featureFanOut[f]++
					}
					sig := strings.Join(c.Features, "\x1f")
					if old, ok := representatives[sig]; !ok || betterRepresentative(c, old, startYear) {
						representatives[sig] = c
					}
				}
			}
		}
	}
	candidates := make([]ReviewCandidate, 0, len(representatives))
	for _, c := range representatives {
		candidates = append(candidates, c)
	}
	sort.Slice(candidates, func(i, j int) bool { return betterRepresentative(candidates[i], candidates[j], startYear) })
	for f := range allFeatures {
		plan.Features = append(plan.Features, f)
	}
	sort.Strings(plan.Features)
	// Sample primary-year behavior first; later dates provide examples of
	// features absent from that year. Exposure weights use the entire sweep.
	unsampled := allFeatures
	used := make([]bool, len(candidates))
	selectPhase := func(primaryOnly bool) {
		for len(unsampled) > 0 {
			best, bestNew, bestImpact := -1, []string(nil), -1
			for i, c := range candidates {
				if used[i] || (primaryOnly && c.Date.Year() != startYear) {
					continue
				}
				var newly []string
				impact := 0
				for _, f := range c.Features {
					if unsampled[f] {
						newly = append(newly, f)
						impact += featureFanOut[f]
					}
				}
				if len(newly) == 0 {
					continue
				}
				if betterCoverPick(c, impact, len(newly), best < 0, candidates, best, bestImpact, len(bestNew), startYear) {
					best, bestNew, bestImpact = i, newly, impact
				}
			}
			if best < 0 {
				return
			}
			used[best] = true
			for _, f := range bestNew {
				delete(unsampled, f)
			}
			primary := candidates[best].Date.Year() == startYear
			if primary {
				plan.PrimaryYearPages++
			} else {
				plan.FutureYearPages++
			}
			plan.Selected = append(plan.Selected, PlannedReview{Candidate: candidates[best], NewFeatures: bestNew, Exposure: bestImpact, PrimaryYear: primary})
		}
	}
	selectPhase(true)
	selectPhase(false)
	if len(unsampled) > 0 {
		return nil, fmt.Errorf("sample planner could not represent %d observed features", len(unsampled))
	}
	return plan, nil
}

func betterRepresentative(a, b ReviewCandidate, primaryYear int) bool {
	aPrimary, bPrimary := a.Date.Year() == primaryYear, b.Date.Year() == primaryYear
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
		Form: hour.Form, Hash: HashHour(hour), Priority: u.Priority(), Hour: hourName, Date: day.Date,
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
	if hourOrder[a.Hour] != hourOrder[b.Hour] {
		return hourOrder[a.Hour] < hourOrder[b.Hour]
	}
	if a.Form != b.Form {
		return a.Form < b.Form
	}
	return a.Hash < b.Hash
}

// PrintReviewPlanSummary describes the sample without a review-completion score.
func PrintReviewPlanSummary(p *ReviewPlan, w io.Writer) {
	fmt.Fprintf(w, "=== Composition samples: %d-%d ===\n", p.StartYear, p.StartYear+p.Years-1)
	fmt.Fprintln(w, "Examples of observed engine behavior; not a rubric checklist or correctness measure.")
	fmt.Fprintf(w, "  composed date-hour forms: %d\n", p.CandidateCount)
	fmt.Fprintf(w, "  observed features:       %d\n", len(p.Features))
	fmt.Fprintf(w, "  sample pages:            %d\n", len(p.Selected))
	fmt.Fprintf(w, "  primary-year samples:    %d\n", p.PrimaryYearPages)
	fmt.Fprintf(w, "  later-year samples:      %d\n", p.FutureYearPages)
	fmt.Fprintf(w, "  source keys included:    %t\n", p.IncludeSources)
}

// WriteReviewPlanCSV writes sample pages with the features that selected them.
// Columns stay reviewer-facing: no bulk dependency dumps or internal hashes.
func WriteReviewPlanCSV(p *ReviewPlan, w io.Writer, baseURL string) error {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	baseURL = strings.TrimRight(baseURL, "/")
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{
		"order", "priority", "hour", "date", "unit_key", "celebration", "context",
		"primary_year", "feature_exposure", "sampled_features", "url",
	})
	for i, selected := range p.Selected {
		c := selected.Candidate
		primary := "no"
		if selected.PrimaryYear {
			primary = "yes"
		}
		_ = cw.Write([]string{
			fmt.Sprint(i + 1), c.Priority, c.Hour, c.Date.Format("2006-01-02"),
			c.UnitKey, c.Celebration, c.Context, primary,
			fmt.Sprint(selected.Exposure),
			strings.Join(selected.NewFeatures, "; "),
			baseURL + "/" + c.Hour + "/" + c.Date.Format("2006-01-02") + "?form=" + string(c.Form),
		})
	}
	cw.Flush()
	return cw.Error()
}
