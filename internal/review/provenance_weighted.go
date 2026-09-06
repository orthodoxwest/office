package review

import (
	"fmt"
	"io"
)

// UsageWeightedProvenance reports what share of a year's worth of prayed
// text is already verified, weighting each corpus entry by how many times it
// is actually rendered across every hour of every day in the sweep — so a
// psalm repeated daily counts far more than a once-a-year collect. This
// differs from the flat "N of M corpus entries verified" count reported by
// ProvenanceInventory, which weights every entry equally regardless of use
// (and also counts entries the sweep never rendered at all).
type UsageWeightedProvenance struct {
	StartYear int
	Years     int

	RenderedEntries int // distinct corpus keys actually rendered in the sweep
	VerifiedEntries int // subset of RenderedEntries with a verified attestation

	TotalOccurrences         int // sum of per-day-per-hour renders across RenderedEntries
	VerifiedOccurrences      int
	NeedsReviewOccurrences   int
	SourceUnknownOccurrences int
}

// EntryPercent is the flat, unweighted verified share among entries actually
// rendered in the sweep — comparable to the raw corpus verified count, but
// scoped to what a year of prayer actually touches.
func (u UsageWeightedProvenance) EntryPercent() float64 {
	if u.RenderedEntries == 0 {
		return 0
	}
	return 100 * float64(u.VerifiedEntries) / float64(u.RenderedEntries)
}

// OccurrencePercent is the usage-weighted verified share: verified renders
// over total renders, across every hour of every day in the sweep.
func (u UsageWeightedProvenance) OccurrencePercent() float64 {
	if u.TotalOccurrences == 0 {
		return 0
	}
	return 100 * float64(u.VerifiedOccurrences) / float64(u.TotalOccurrences)
}

// BuildUsageWeightedProvenance composes every hour of every day across
// [startYear, startYear+years) and weights each rendered corpus entry's
// provenance status by how many times it was actually prayed in the sweep.
func BuildUsageWeightedProvenance(dataDir string, startYear, years int) (*UsageWeightedProvenance, error) {
	// includeVerified=true: BuildProvenanceQueue otherwise drops verified
	// entries entirely, which would silently exclude them from both totals.
	queue, err := BuildProvenanceQueue(dataDir, startYear, years, true)
	if err != nil {
		return nil, err
	}
	u := &UsageWeightedProvenance{StartYear: startYear, Years: years}
	for _, e := range queue.Entries {
		if e.Occurrences == 0 {
			continue // never rendered in this sweep; out of scope for this metric
		}
		u.RenderedEntries++
		u.TotalOccurrences += e.Occurrences
		switch e.Status {
		case ProvenanceVerified:
			u.VerifiedEntries++
			u.VerifiedOccurrences += e.Occurrences
		case ProvenanceNeedsReview:
			u.NeedsReviewOccurrences += e.Occurrences
		default:
			u.SourceUnknownOccurrences += e.Occurrences
		}
	}
	return u, nil
}

// PrintUsageWeightedProvenance writes a compact human-readable summary
// suitable for a status update: counts only, no source contents.
func PrintUsageWeightedProvenance(u *UsageWeightedProvenance, w io.Writer) {
	fmt.Fprintf(w, "=== Usage-weighted provenance: %d-%d ===\n", u.StartYear, u.StartYear+u.Years-1)
	fmt.Fprintf(w, "  rendered entries:       %5d\n", u.RenderedEntries)
	fmt.Fprintf(w, "  verified entries:       %5d (%.1f%% of rendered entries)\n", u.VerifiedEntries, u.EntryPercent())
	fmt.Fprintf(w, "  total occurrences:      %5d\n", u.TotalOccurrences)
	fmt.Fprintf(w, "  verified occurrences:   %5d (%.1f%% of prayed text, usage-weighted)\n", u.VerifiedOccurrences, u.OccurrencePercent())
	fmt.Fprintf(w, "  needs-review occ.:      %5d\n", u.NeedsReviewOccurrences)
	fmt.Fprintf(w, "  source-unknown occ.:    %5d\n", u.SourceUnknownOccurrences)
}
