package review

import (
	"bytes"
	"os"
	"path/filepath"
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

func TestBuildReviewPlanReducesStructuralChecklist(t *testing.T) {
	p, err := BuildReviewPlan("../../data", 2026, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Selected) == 0 || len(p.Selected) >= p.CandidateCount {
		t.Fatalf("selected %d of %d candidates", len(p.Selected), p.CandidateCount)
	}
	if p.FullCoverPages == 0 || p.FullCoverPages < len(p.Selected) {
		t.Fatalf("full-cover pages=%d residual=%d", p.FullCoverPages, len(p.Selected))
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
	if p.TotalImpact <= 0 || p.RemainingImpact <= 0 {
		t.Fatalf("impact totals missing: total=%d remaining=%d", p.TotalImpact, p.RemainingImpact)
	}
	if len(p.Selected) >= 2 {
		first := p.Selected[0].NewImpact
		last := p.Selected[len(p.Selected)-1].NewImpact
		if first < last {
			t.Fatalf("expected early page impact >= late page impact: first=%d last=%d", first, last)
		}
	}
	for _, selected := range p.Selected {
		for _, feature := range selected.NewCoverage {
			if strings.HasPrefix(feature, "source:") {
				t.Fatalf("structural plan unexpectedly includes source feature %q", feature)
			}
			if strings.HasPrefix(feature, "decision:context:") || strings.HasPrefix(feature, "decision:office-context:") {
				t.Fatalf("structural plan includes tier-B context feature %q", feature)
			}
			if isWeekdayPsalmodyNoise(feature) {
				t.Fatalf("structural plan includes weekday psalmody noise %q", feature)
			}
		}
		if selected.Candidate.SignoffState == "" {
			t.Fatal("selected candidate missing signoff_status")
		}
	}
}

func TestReviewPlanCreditsSchemaCurrentSignoffs(t *testing.T) {
	tmp := t.TempDir()
	linkData(t, tmp, "../../data")
	if err := os.WriteFile(filepath.Join(tmp, "review", "signoffs.txt"), []byte("# empty\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	empty, err := BuildReviewPlan(tmp, 2026, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if empty.CreditedCount != 0 {
		t.Fatalf("empty signoffs should credit 0 features, got %d", empty.CreditedCount)
	}
	if len(empty.Selected) == 0 {
		t.Fatal("expected pages with empty signoffs")
	}
	first := empty.Selected[0]
	// Prefer a page that is not the only coverer for its features: use first residual page.
	s := Signoff{
		Hash: first.Candidate.Hash, Hour: first.Candidate.Hour, UnitKey: first.Candidate.UnitKey,
		Reviewer: "tester", Date: "2026-07-22", Schema: StructuralFeatureSchema,
	}
	if err := AppendSignoff(tmp, s); err != nil {
		t.Fatal(err)
	}
	after, err := BuildReviewPlan(tmp, 2026, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if after.CreditedCount == 0 {
		t.Fatal("expected credited features after schema-current sign-off")
	}
	if after.RemainingImpact >= empty.RemainingImpact {
		t.Fatalf("remaining impact should shrink: before=%d after=%d", empty.RemainingImpact, after.RemainingImpact)
	}
	if len(after.Selected) >= len(empty.Selected) {
		t.Fatalf("residual pages should shrink after credit: before=%d after=%d", len(empty.Selected), len(after.Selected))
	}
	credited := map[string]bool{}
	for _, f := range after.CreditedFeatures {
		credited[f] = true
	}
	for _, f := range first.NewCoverage {
		if !credited[f] {
			t.Fatalf("feature %q from signed page not credited", f)
		}
	}
	for _, sel := range after.Selected {
		if sel.Candidate.Hash == first.Candidate.Hash && sel.Candidate.SignoffState == Current.String() {
			t.Fatalf("signed page re-selected while residual unsigned cover should exist: %s %s",
				sel.Candidate.Hour, sel.Candidate.Date.Format("2006-01-02"))
		}
	}
}

func TestReviewPlanLegacySignoffDoesNotCredit(t *testing.T) {
	tmp := t.TempDir()
	linkData(t, tmp, "../../data")
	if err := os.WriteFile(filepath.Join(tmp, "review", "signoffs.txt"), []byte("# empty\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	empty, err := BuildReviewPlan(tmp, 2026, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	first := empty.Selected[0]
	// Write a legacy line without schema= (schema 0).
	line := first.Candidate.Hash + " " + first.Candidate.Hour + " " + first.Candidate.UnitKey + " tester 2026-07-22 legacy note\n"
	if err := os.WriteFile(filepath.Join(tmp, "review", "signoffs.txt"), []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}
	after, err := BuildReviewPlan(tmp, 2026, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if after.CreditedCount != 0 {
		t.Fatalf("legacy sign-off must not credit structural features, got %d", after.CreditedCount)
	}
	if after.CurrentSignoffs == 0 {
		t.Fatal("legacy sign-off should still count as current content status")
	}
	if after.CreditingSignoffs != 0 {
		t.Fatalf("crediting sign-offs = %d, want 0", after.CreditingSignoffs)
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
			Celebration: "Trinity Sunday", SignoffState: "unreviewed",
		},
		NewCoverage: []string{"decision:preces=said", "resolution:collect=proper"},
		NewImpact:   1200,
	}}}
	var out bytes.Buffer
	if err := WriteReviewPlanCSV(p, &out, "https://example.test"); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "new_features") || !strings.Contains(got, "new_impact") || !strings.Contains(got, "signoff_status") || !strings.Contains(got, "primary_year") {
		t.Fatalf("CSV missing expected columns:\n%s", got)
	}
	if strings.Contains(got, "remaining_impact") {
		t.Fatalf("CSV still uses old remaining_impact column:\n%s", got)
	}
	if !strings.Contains(got, "decision:preces=said") {
		t.Fatalf("CSV missing new_features content:\n%s", got)
	}
	if strings.Contains(got, "0123456789ab") {
		t.Fatalf("review plan exposes internal composition identity:\n%s", got)
	}
	if strings.Contains(got, "dependencies") || strings.Contains(strings.Split(got, "\n")[0], "decisions") {
		t.Fatalf("CSV still has bulk columns:\n%s", got)
	}
}

func TestIsTierAStructuralFeature(t *testing.T) {
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
		if got := isTierAStructuralFeature(feat); got != want {
			t.Errorf("isTierAStructuralFeature(%q)=%v want %v", feat, got, want)
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
	// primary-year residual for that phase is exhausted (future rows may appear
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

func TestParseSignoffSchema(t *testing.T) {
	input := `aaa lauds trinity-sunday mary.k 2026-06-08 schema=2 checked
bbb vespers all-saints john.d 2026-06-09 legacy note
ccc prime feria-lent jane.d 2026-06-10 schema=1 old universe
`
	signoffs, err := ParseSignoffs(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(signoffs) != 3 {
		t.Fatalf("got %d", len(signoffs))
	}
	if signoffs[0].Schema != 2 || signoffs[0].Note != "checked" {
		t.Fatalf("signoffs[0]=%#v", signoffs[0])
	}
	if signoffs[1].Schema != 0 || signoffs[1].Note != "legacy note" {
		t.Fatalf("signoffs[1]=%#v", signoffs[1])
	}
	if signoffs[2].Schema != 1 || signoffs[2].CreditsStructuralFeatures() {
		t.Fatalf("signoffs[2]=%#v should not credit", signoffs[2])
	}
	if !signoffs[0].CreditsStructuralFeatures() {
		t.Fatal("schema=2 should credit at current StructuralFeatureSchema")
	}
}
