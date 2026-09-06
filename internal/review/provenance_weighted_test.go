package review

import (
	"bytes"
	"strings"
	"testing"
)

func TestUsageWeightedProvenancePercentages(t *testing.T) {
	u := UsageWeightedProvenance{
		RenderedEntries: 4, VerifiedEntries: 1,
		TotalOccurrences: 100, VerifiedOccurrences: 40,
	}
	if got, want := u.EntryPercent(), 25.0; got != want {
		t.Fatalf("EntryPercent() = %v, want %v", got, want)
	}
	if got, want := u.OccurrencePercent(), 40.0; got != want {
		t.Fatalf("OccurrencePercent() = %v, want %v", got, want)
	}
}

func TestUsageWeightedProvenanceZeroDenominators(t *testing.T) {
	var u UsageWeightedProvenance
	if got := u.EntryPercent(); got != 0 {
		t.Fatalf("EntryPercent() with no rendered entries = %v, want 0", got)
	}
	if got := u.OccurrencePercent(); got != 0 {
		t.Fatalf("OccurrencePercent() with no occurrences = %v, want 0", got)
	}
}

func TestBuildUsageWeightedProvenance(t *testing.T) {
	u, err := BuildUsageWeightedProvenance("../../data", 2026, 1)
	if err != nil {
		t.Fatal(err)
	}
	if u.RenderedEntries == 0 || u.TotalOccurrences == 0 {
		t.Fatalf("expected a non-empty sweep, got RenderedEntries=%d TotalOccurrences=%d", u.RenderedEntries, u.TotalOccurrences)
	}
	if u.VerifiedEntries > u.RenderedEntries {
		t.Fatalf("verified entries (%d) exceed rendered entries (%d)", u.VerifiedEntries, u.RenderedEntries)
	}
	if u.VerifiedOccurrences > u.TotalOccurrences {
		t.Fatalf("verified occurrences (%d) exceed total occurrences (%d)", u.VerifiedOccurrences, u.TotalOccurrences)
	}
	sum := u.VerifiedOccurrences + u.NeedsReviewOccurrences + u.SourceUnknownOccurrences
	if sum != u.TotalOccurrences {
		t.Fatalf("status occurrence buckets sum to %d, want %d (TotalOccurrences)", sum, u.TotalOccurrences)
	}
	if p := u.OccurrencePercent(); p < 0 || p > 100 {
		t.Fatalf("OccurrencePercent() = %v out of range", p)
	}
}

func TestPrintUsageWeightedProvenance(t *testing.T) {
	u := &UsageWeightedProvenance{
		StartYear: 2026, Years: 1,
		RenderedEntries: 10, VerifiedEntries: 7,
		TotalOccurrences: 1000, VerifiedOccurrences: 900,
		NeedsReviewOccurrences: 80, SourceUnknownOccurrences: 20,
	}
	var out bytes.Buffer
	PrintUsageWeightedProvenance(u, &out)
	got := out.String()
	for _, want := range []string{"2026-2026", "90.0%", "70.0%"} {
		if !strings.Contains(got, want) {
			t.Fatalf("summary missing %q:\n%s", want, got)
		}
	}
}
