package review

import (
	"bytes"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestExplainCompositionIncludesDependenciesAndDecisions(t *testing.T) {
	a, err := ExplainComposition("../../data", "lauds", time.Date(2026, 6, 7, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Dependencies) == 0 {
		t.Fatal("no composition dependencies")
	}
	if len(a.Decisions) == 0 {
		t.Fatal("no composition decisions")
	}
	wantDependency := "proper/trinity-sunday/collect"
	foundDependency, foundOccurrence := false, false
	for _, d := range a.Dependencies {
		if d.Key == wantDependency {
			foundDependency = true
		}
	}
	for _, d := range a.Decisions {
		if d.Rule == "occurrence" && d.Outcome != "" {
			foundOccurrence = true
		}
	}
	if !foundDependency {
		t.Errorf("missing dependency %s", wantDependency)
	}
	if !foundOccurrence {
		t.Error("missing occurrence explanation")
	}
}

func TestBuildReviewPlanReducesStructuralChecklist(t *testing.T) {
	p, err := BuildReviewPlan("../../data", 2026, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Selected) == 0 || len(p.Selected) >= p.CandidateCount {
		t.Fatalf("selected %d of %d candidates", len(p.Selected), p.CandidateCount)
	}
	if len(p.Uncovered) != 0 {
		t.Fatalf("uncovered features: %v", p.Uncovered)
	}
	if p.FeatureCount == 0 || p.FeatureCount != len(p.Features) {
		t.Fatalf("feature inventory has %d entries, count reports %d", len(p.Features), p.FeatureCount)
	}
	if !sort.StringsAreSorted(p.Features) {
		t.Fatal("feature inventory is not sorted")
	}
	if len(p.RenderedKeys) == 0 || !sort.StringsAreSorted(p.RenderedKeys) {
		t.Fatal("rendered dependency inventory is empty or unsorted")
	}
	for _, selected := range p.Selected {
		for _, feature := range selected.NewCoverage {
			if len(feature) >= 7 && feature[:7] == "source:" {
				t.Fatalf("structural plan unexpectedly includes source feature %q", feature)
			}
		}
	}
}

func TestReviewPlanCSVHidesCompositionHashes(t *testing.T) {
	p := &ReviewPlan{Selected: []PlannedReview{{Candidate: ReviewCandidate{
		Hash: "0123456789ab", Priority: "A", Hour: "lauds",
		Date: time.Date(2026, 6, 7, 0, 0, 0, 0, time.UTC), UnitKey: "trinity-sunday",
		Celebration: "Trinity Sunday",
	}}}}
	var out bytes.Buffer
	if err := WriteReviewPlanCSV(p, &out, "https://example.test"); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "hash") || strings.Contains(out.String(), "0123456789ab") {
		t.Fatalf("review plan exposes internal composition identity:\n%s", out.String())
	}
}
