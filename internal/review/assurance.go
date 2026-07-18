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
	Hash         string // internal composition identity; omitted from reviewer output
	Priority     string
	Hour         string
	Date         time.Time
	UnitKey      string
	Celebration  string
	Context      string
	Dependencies []string
	Decisions    []string
	Features     []string
}

// PlannedReview records why one candidate was selected.
type PlannedReview struct {
	Candidate   ReviewCandidate
	NewCoverage []string
}

// ReviewPlan is a greedy coverage plan over all distinct source and decision
// features exercised in a year sweep.
type ReviewPlan struct {
	StartYear      int
	Years          int
	CandidateCount int
	FeatureCount   int
	Features       []string
	RenderedKeys   []string
	Selected       []PlannedReview
	IncludeSources bool
	Uncovered      []string
}

// BuildReviewPlan composes the requested years, collapses candidates with
// identical feature sets, then greedily selects pages covering the most still
// unseen dependencies and decision branches.
func BuildReviewPlan(dataDir string, startYear, years int, includeSources bool) (*ReviewPlan, error) {
	if years < 1 {
		return nil, fmt.Errorf("years must be at least 1")
	}
	eng, err := office.NewEngine(dataDir)
	if err != nil {
		return nil, err
	}

	bySignature := map[string]ReviewCandidate{}
	allFeatures := map[string]bool{}
	renderedKeys := map[string]bool{}
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
				for _, dependency := range c.Dependencies {
					renderedKeys[dependency] = true
				}
				for _, f := range c.Features {
					allFeatures[f] = true
				}
				sig := strings.Join(c.Features, "\x1f")
				if old, ok := bySignature[sig]; !ok || candidateLess(c, old) {
					bySignature[sig] = c
				}
			}
		}
	}

	candidates := make([]ReviewCandidate, 0, len(bySignature))
	for _, c := range bySignature {
		candidates = append(candidates, c)
	}
	sort.Slice(candidates, func(i, j int) bool { return candidateLess(candidates[i], candidates[j]) })

	uncovered := make(map[string]bool, len(allFeatures))
	for f := range allFeatures {
		uncovered[f] = true
	}
	plan := &ReviewPlan{StartYear: startYear, Years: years, CandidateCount: rawCount, IncludeSources: includeSources}
	for key := range renderedKeys {
		plan.RenderedKeys = append(plan.RenderedKeys, key)
	}
	sort.Strings(plan.RenderedKeys)
	for feature := range allFeatures {
		plan.Features = append(plan.Features, feature)
	}
	sort.Strings(plan.Features)
	plan.FeatureCount = len(plan.Features)
	used := make([]bool, len(candidates))
	for len(uncovered) > 0 {
		best, bestNew := -1, []string(nil)
		for i, c := range candidates {
			if used[i] {
				continue
			}
			var newly []string
			for _, f := range c.Features {
				if uncovered[f] {
					newly = append(newly, f)
				}
			}
			if len(newly) > len(bestNew) || (len(newly) == len(bestNew) && len(newly) > 0 && (best < 0 || candidateLess(c, candidates[best]))) {
				best, bestNew = i, newly
			}
		}
		if best < 0 || len(bestNew) == 0 {
			break
		}
		used[best] = true
		for _, f := range bestNew {
			delete(uncovered, f)
		}
		sort.Strings(bestNew)
		plan.Selected = append(plan.Selected, PlannedReview{Candidate: candidates[best], NewCoverage: bestNew})
	}
	for feature := range uncovered {
		plan.Uncovered = append(plan.Uncovered, feature)
	}
	sort.Strings(plan.Uncovered)
	return plan, nil
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
		featureSet["decision:"+decision] = true
	}
	for decision := range decisionSet {
		c.Decisions = append(c.Decisions, decision)
	}
	for _, section := range hour.Sections {
		for _, elem := range section.Elements {
			if elem.SlotRef == "" || elem.SourceRef == "" {
				continue
			}
			tier, _, _ := strings.Cut(elem.SourceRef, "/")
			featureSet["resolution:"+elem.SlotRef+"="+tier] = true
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
	fmt.Fprintf(w, "  composed candidates: %d\n", p.CandidateCount)
	fmt.Fprintf(w, "  coverage features:   %d\n", p.FeatureCount)
	fmt.Fprintf(w, "  selected pages:      %d\n", len(p.Selected))
	fmt.Fprintf(w, "  source-key coverage: %t\n", p.IncludeSources)
	fmt.Fprintf(w, "  uncovered features:  %d\n", len(p.Uncovered))
}

// WriteReviewPlanCSV writes the selected review pages in greedy order.
func WriteReviewPlanCSV(p *ReviewPlan, w io.Writer, baseURL string) error {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	baseURL = strings.TrimRight(baseURL, "/")
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"order", "priority", "hour", "date", "unit_key", "celebration", "context", "new_coverage", "dependencies", "decisions", "url"})
	for i, selected := range p.Selected {
		c := selected.Candidate
		_ = cw.Write([]string{
			fmt.Sprint(i + 1), c.Priority, c.Hour, c.Date.Format("2006-01-02"),
			c.UnitKey, c.Celebration, c.Context, fmt.Sprint(len(selected.NewCoverage)),
			strings.Join(c.Dependencies, "; "), strings.Join(c.Decisions, "; "),
			baseURL + "/" + c.Hour + "/" + c.Date.Format("2006-01-02"),
		})
	}
	cw.Flush()
	return cw.Error()
}
