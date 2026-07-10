package review

import (
	"encoding/csv"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/office"
)

// ProvenanceQueueEntry is one atomic corpus review task, ranked by the
// rendered coverage that a verification would unlock.
type ProvenanceQueueEntry struct {
	Key                  string
	ContentHash          string
	Status               ProvenanceStatus
	Score                int
	Occurrences          int
	PriorityAOccurrences int
	PrincipalOccurrences int
	DistinctCompositions int
	Hours                []string
	RepresentativeHour   string
	RepresentativeDate   time.Time
	Sources              []SourceCitation
}

// ProvenanceQueue is a deterministic dependency-weighted corpus work queue.
type ProvenanceQueue struct {
	StartYear       int
	Years           int
	IncludeVerified bool
	Entries         []ProvenanceQueueEntry
}

type queueAccumulator struct {
	entry             ProvenanceQueueEntry
	hours             map[string]bool
	compositions      map[string]bool
	representative    ReviewCandidate
	hasRepresentative bool
}

// BuildProvenanceQueue sweeps composed hours and ranks each corpus entry.
// The score favors fan-out across distinct compositions, then priority-A and
// principal-hour use, while still counting raw occurrences:
//
//	20*distinct compositions + 5*priority-A uses + 3*principal-hour uses + uses
func BuildProvenanceQueue(dataDir string, startYear, years int, includeVerified bool) (*ProvenanceQueue, error) {
	if years < 1 {
		return nil, fmt.Errorf("years must be at least 1")
	}
	inventory, err := ScanProvenance(dataDir)
	if err != nil {
		return nil, err
	}
	acc := make(map[string]*queueAccumulator, len(inventory.Entries))
	for _, p := range inventory.Entries {
		if !shouldQueueProvenance(p.Status, includeVerified) {
			continue
		}
		acc[p.Key] = &queueAccumulator{
			entry: ProvenanceQueueEntry{
				Key: p.Key, ContentHash: p.ContentHash, Status: p.Status, Sources: p.Sources,
			},
			hours: make(map[string]bool), compositions: make(map[string]bool),
		}
	}

	eng, err := office.NewEngine(dataDir)
	if err != nil {
		return nil, err
	}
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
				candidate := candidateFor(day, hourName, hour, false)
				priorityA := candidate.Priority == "A"
				principal := hourTier[hourName] == 0
				for _, ref := range candidate.Dependencies {
					a := acc[ref]
					if a == nil {
						continue
					}
					a.entry.Occurrences++
					if priorityA {
						a.entry.PriorityAOccurrences++
					}
					if principal {
						a.entry.PrincipalOccurrences++
					}
					a.hours[hourName] = true
					a.compositions[candidate.Hash] = true
					if !a.hasRepresentative || representativeLess(candidate, a.representative) {
						a.representative = candidate
						a.hasRepresentative = true
					}
				}
			}
		}
	}

	queue := &ProvenanceQueue{StartYear: startYear, Years: years, IncludeVerified: includeVerified}
	for _, a := range acc {
		for hour := range a.hours {
			a.entry.Hours = append(a.entry.Hours, hour)
		}
		sort.Slice(a.entry.Hours, func(i, j int) bool { return hourOrder[a.entry.Hours[i]] < hourOrder[a.entry.Hours[j]] })
		a.entry.DistinctCompositions = len(a.compositions)
		a.entry.Score = provenanceQueueScore(a.entry)
		if a.hasRepresentative {
			a.entry.RepresentativeHour = a.representative.Hour
			a.entry.RepresentativeDate = a.representative.Date
		}
		queue.Entries = append(queue.Entries, a.entry)
	}
	sort.Slice(queue.Entries, func(i, j int) bool { return provenanceQueueLess(queue.Entries[i], queue.Entries[j]) })
	return queue, nil
}

func shouldQueueProvenance(status ProvenanceStatus, includeVerified bool) bool {
	return status != ProvenanceVerified || includeVerified
}

func provenanceQueueScore(e ProvenanceQueueEntry) int {
	return 20*e.DistinctCompositions + 5*e.PriorityAOccurrences + 3*e.PrincipalOccurrences + e.Occurrences
}

func provenanceQueueLess(a, b ProvenanceQueueEntry) bool {
	if a.Score != b.Score {
		return a.Score > b.Score
	}
	if a.PriorityAOccurrences != b.PriorityAOccurrences {
		return a.PriorityAOccurrences > b.PriorityAOccurrences
	}
	if a.Occurrences != b.Occurrences {
		return a.Occurrences > b.Occurrences
	}
	return a.Key < b.Key
}

func representativeLess(a, b ReviewCandidate) bool {
	if a.Priority != b.Priority {
		return a.Priority < b.Priority
	}
	aPrincipal, bPrincipal := hourTier[a.Hour] == 0, hourTier[b.Hour] == 0
	if aPrincipal != bPrincipal {
		return aPrincipal
	}
	if !a.Date.Equal(b.Date) {
		return a.Date.Before(b.Date)
	}
	return hourOrder[a.Hour] < hourOrder[b.Hour]
}

// PrintProvenanceQueueSummary writes compact queue and usage counts.
func PrintProvenanceQueueSummary(q *ProvenanceQueue, w io.Writer) {
	used := 0
	statuses := map[ProvenanceStatus]int{}
	for _, e := range q.Entries {
		statuses[e.Status]++
		if e.Occurrences > 0 {
			used++
		}
	}
	fmt.Fprintf(w, "=== Atomic provenance review queue: %d-%d ===\n", q.StartYear, q.StartYear+q.Years-1)
	fmt.Fprintf(w, "  queued entries:      %d\n", len(q.Entries))
	fmt.Fprintf(w, "  rendered entries:    %d\n", used)
	fmt.Fprintf(w, "  include verified:    %t\n", q.IncludeVerified)
	for _, status := range []ProvenanceStatus{ProvenanceVerified, ProvenanceNeedsReview, ProvenanceSourceUnknown} {
		fmt.Fprintf(w, "  %-19s %d\n", string(status)+":", statuses[status])
	}
}

// WriteProvenanceQueueCSV writes one row per atomic corpus entry.
func WriteProvenanceQueueCSV(q *ProvenanceQueue, w io.Writer, baseURL string) error {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	baseURL = strings.TrimRight(baseURL, "/")
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"rank", "score", "key", "status", "occurrences", "priority_a_occurrences", "principal_occurrences", "distinct_compositions", "hours", "source", "locator", "page", "representative_url"})
	for i, e := range q.Entries {
		var sources, locators, pages []string
		for _, source := range e.Sources {
			sources = appendUnique(sources, source.Source)
			locators = appendUnique(locators, source.Locator)
			pages = appendUnique(pages, source.Page)
		}
		url := ""
		if !e.RepresentativeDate.IsZero() {
			url = fmt.Sprintf("%s/%s/%s", baseURL, e.RepresentativeHour, e.RepresentativeDate.Format("2006-01-02"))
		}
		_ = cw.Write([]string{
			fmt.Sprint(i + 1), fmt.Sprint(e.Score), e.Key, string(e.Status),
			fmt.Sprint(e.Occurrences), fmt.Sprint(e.PriorityAOccurrences), fmt.Sprint(e.PrincipalOccurrences),
			fmt.Sprint(e.DistinctCompositions), strings.Join(e.Hours, "; "), strings.Join(sources, "; "),
			strings.Join(locators, "; "), strings.Join(pages, "; "), url,
		})
	}
	cw.Flush()
	return cw.Error()
}

func appendUnique(values []string, value string) []string {
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}
