package review

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
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
	if len(a.Resolutions) == 0 {
		t.Fatal("no resolution metadata")
	}
	for _, resolution := range a.Resolutions {
		if resolution.RequestedSlot == "" || resolution.SelectedRef == "" || resolution.SelectedTier == "" {
			t.Fatalf("incomplete resolution metadata: %#v", resolution)
		}
	}
	if len(a.Decisions) == 0 {
		t.Fatal("no composition decisions")
	}
	wantDependency := "proper/trinity-sunday/collect"
	foundDependency, foundOccurrence, foundSuffrage, foundMarian, foundPreces := false, false, false, false, false
	for _, d := range a.Dependencies {
		if d.Key == wantDependency {
			foundDependency = true
		}
	}
	for _, d := range a.Decisions {
		switch d.Rule {
		case "occurrence":
			if d.Outcome != "" {
				foundOccurrence = true
			}
		case "suffrage":
			foundSuffrage = true
		case "marian:selection":
			foundMarian = true
		case "preces":
			foundPreces = true
		}
	}
	if !foundDependency {
		t.Errorf("missing dependency %s", wantDependency)
	}
	if !foundOccurrence {
		t.Error("missing occurrence explanation")
	}
	if !foundSuffrage {
		t.Error("missing suffrage disposition on lauds")
	}
	if !foundMarian {
		t.Error("missing marian selection on lauds")
	}
	if foundPreces {
		t.Error("lauds must not record preces disposition")
	}
}

func TestExplainCompositionPriscaCommemorationUsesCommemorationOwner(t *testing.T) {
	a, err := ExplainComposition("../../data", "lauds", time.Date(2026, 1, 18, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	const prisca = "comm-01-18-st-prisca-of-rome-virgin-martyr"
	for _, resolution := range a.Resolutions {
		if resolution.RequestedSlot == "commemoration-collect" && strings.Contains(strings.Join(resolution.ProperIDs, " "), prisca) {
			if resolution.CanonicalOwner != prisca {
				t.Fatalf("Prisca commemoration owner = %q, want %q", resolution.CanonicalOwner, prisca)
			}
			return
		}
	}
	t.Fatal("missing Prisca commemoration collect resolution")
}

func TestShouldIncludeResolution(t *testing.T) {
	tests := []struct {
		name, source, tier string
		want               bool
	}{
		{"proper", "proper/feast/collect", "proper", true},
		{"common", "commons/martyr/collect", "common", true},
		{"seasonal", "seasonal/lent/collect", "seasonal", true},
		{"ordinary", "ordinary/lauds/collect", "ordinary", true},
		{"missing bare slot", "collect", "not-found", true},
		{"static source", "psalms/1", "not-found", true},
		{"static resolved source", "psalms/1", "ordinary", false},
		{"empty resolved source", "", "proper", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldIncludeResolution(tt.source, tt.tier); got != tt.want {
				t.Fatalf("include = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildReviewPlanSamplesObservedFeatures(t *testing.T) {
	p, err := BuildReviewPlan("../../data", 2026, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Selected) == 0 || len(p.Selected) >= p.CandidateCount {
		t.Fatalf("selected %d of %d candidates", len(p.Selected), p.CandidateCount)
	}
	if len(p.Features) == 0 {
		t.Fatal("empty feature inventory")
	}
	if !sort.StringsAreSorted(p.Features) {
		t.Fatal("feature inventory is not sorted")
	}
	if len(p.Selected) >= 2 {
		first := p.Selected[0].Exposure
		last := p.Selected[len(p.Selected)-1].Exposure
		if first < last {
			t.Fatalf("expected early page impact >= late page impact: first=%d last=%d", first, last)
		}
	}
	observed := map[string]bool{}
	for _, selected := range p.Selected {
		for _, feature := range selected.NewFeatures {
			observed[feature] = true
			if strings.HasPrefix(feature, "source:") {
				t.Fatalf("sample plan unexpectedly includes source feature %q", feature)
			}
			if strings.HasPrefix(feature, "decision:context:") || strings.HasPrefix(feature, "decision:office-context:") {
				t.Fatalf("sample plan includes descriptive context feature %q", feature)
			}
			if isWeekdayPsalmodyNoise(feature) {
				t.Fatalf("sample plan includes weekday psalmody noise %q", feature)
			}
		}
	}
	for _, f := range p.Features {
		if !observed[f] {
			t.Errorf("observed feature lacks a sample: %s", f)
		}
	}
}

func TestReviewPlanIgnoresHistoricalSignoffs(t *testing.T) {
	tmp := t.TempDir()
	linkData(t, tmp, "../../data")
	before, err := BuildReviewPlan(tmp, 2026, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	var ledger strings.Builder
	for _, sample := range before.Selected {
		c := sample.Candidate
		fmt.Fprintf(&ledger, "%s %s %s tester 2026-07-22 schema=3 checked\n", c.Hash, c.Hour, c.UnitKey)
	}
	if err := os.WriteFile(filepath.Join(tmp, "review", "signoffs.txt"), []byte(ledger.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	after, err := BuildReviewPlan(tmp, 2026, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatal("historical signoffs changed composition samples")
	}
}

func linkData(t *testing.T, tmp, dataRel string) {
	t.Helper()
	abs, err := filepath.Abs(dataRel)
	if err != nil {
		t.Fatal(err)
	}
	err = filepath.Walk(abs, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(abs, path)
		if err != nil {
			return err
		}
		if rel == "review" || strings.HasPrefix(rel, "review"+string(os.PathSeparator)) {
			return nil
		}
		dst := filepath.Join(tmp, rel)
		if info.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		return os.Symlink(path, dst)
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, "review"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestReviewPlanCSVIsReviewerFacing(t *testing.T) {
	p := &ReviewPlan{Selected: []PlannedReview{{
		Candidate: ReviewCandidate{
			Hash: "0123456789ab", Priority: "A", Hour: "lauds",
			Date: time.Date(2026, 6, 7, 0, 0, 0, 0, time.UTC), UnitKey: "trinity-sunday",
			Celebration: "Trinity Sunday",
		},
		NewFeatures: []string{"decision:preces=said", "resolution:collect=proper"},
		Exposure:    1200,
	}}}
	var out bytes.Buffer
	if err := WriteReviewPlanCSV(p, &out, "https://example.test"); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "sampled_features") || !strings.Contains(got, "feature_exposure") || !strings.Contains(got, "primary_year") {
		t.Fatalf("CSV missing expected columns:\n%s", got)
	}
	if strings.Contains(got, "remaining_impact") || strings.Contains(got, "signoff_status") {
		t.Fatalf("CSV still exposes retired review progress columns:\n%s", got)
	}
	if !strings.Contains(got, "decision:preces=said") {
		t.Fatalf("CSV missing sampled_features content:\n%s", got)
	}
	if strings.Contains(got, "0123456789ab") {
		t.Fatalf("review plan exposes internal composition identity:\n%s", got)
	}
	if strings.Contains(got, "dependencies") || strings.Contains(strings.Split(got, "\n")[0], "decisions") {
		t.Fatalf("CSV still has bulk columns:\n%s", got)
	}
}

func TestIsSampleFeature(t *testing.T) {
	cases := map[string]bool{
		"decision:preces=said": true,
		"decision:marian:boundary=purification-vespers-override":                  true,
		"decision:occurrence:higher-rank=challenger-wins":                         true,
		"resolution:collect=proper":                                               true,
		"decision:context:weekday=monday":                                         false,
		"decision:office-context:category=martyr":                                 false,
		"decision:occurrence=occurrence:general-precedence":                       false,
		"decision:condition:weekday-monday=included":                              false,
		"decision:condition:not-festal-vespers-psalmody,weekday-tuesday=included": false,
		"decision:condition:if-preces=included":                                   true,
		"source:ordinary/shared/kyrie":                                            false,
	}
	for feat, want := range cases {
		if got := isSampleFeature(feat); got != want {
			t.Errorf("isSampleFeature(%q)=%v want %v", feat, got, want)
		}
	}
}

func TestReviewPlanPrefersPrimaryYear(t *testing.T) {
	p, err := BuildReviewPlan("../../data", 2026, 28, false)
	if err != nil {
		t.Fatal(err)
	}
	if p.PrimaryYearPages == 0 {
		t.Fatal("expected some primary-year pages")
	}
	// Every primary_year=yes row must be dated 2026; future rows only after
	// primary-year sampling is finished (future rows may appear
	// later in the list).
	sawFuture := false
	for _, sel := range p.Selected {
		y := sel.Candidate.Date.Year()
		if sel.PrimaryYear {
			if y != 2026 {
				t.Fatalf("primary_year page dated %d", y)
			}
			if sawFuture {
				// Primary-phase pages are selected first; a primary page after a
				// future page would mean phase ordering broke.
				t.Fatalf("primary-year page after future-year page: %s", sel.Candidate.Date.Format("2006-01-02"))
			}
		} else {
			if y == 2026 {
				t.Fatalf("future-only flag on 2026 date %s", sel.Candidate.Date.Format("2006-01-02"))
			}
			sawFuture = true
		}
	}
	if p.FutureYearPages == 0 {
		t.Log("no future-only features in this sweep (ok if calendar covers all in 2026)")
	}
}
