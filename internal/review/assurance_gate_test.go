package review

import (
	"bytes"
	"strings"
	"testing"
)

func TestEvaluateAssurance(t *testing.T) {
	report := &AssuranceReport{
		ModeledFeatures: 9, Verified: 1, UncoveredFeatures: []string{"decision:missing"},
	}
	baseline := &AssuranceBaseline{VerifiedMinimum: 2, ModeledFeaturesMinimum: 10}
	failures := EvaluateAssurance(report, baseline)
	if len(failures) != 3 {
		t.Fatalf("failures = %v", failures)
	}
}

func TestAssuranceSummaryContainsNoSourceText(t *testing.T) {
	report := &AssuranceReport{
		StartYear: 2026, Years: 1, CandidateCount: 2555, ModeledFeatures: 10,
		SelectedPages: 3, Verified: 1, Documented: 2, NeedsReview: 3, Undocumented: 4,
	}
	var out bytes.Buffer
	WriteAssuranceSummary(report, nil, &out, true)
	for _, want := range []string{"Office assurance summary", "Modeled structural features", "Verified text entries"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("summary missing %q:\n%s", want, out.String())
		}
	}
}

func TestUpdateAssuranceBaselineRoundTrip(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir+"/review/.keep", "")
	report := &AssuranceReport{StartYear: 2026, Years: 28, Verified: 4, ModeledFeatures: 218}
	if err := UpdateAssuranceBaseline(dir, report); err != nil {
		t.Fatal(err)
	}
	got, err := LoadAssuranceBaseline(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.VerifiedMinimum != 4 || got.ModeledFeaturesMinimum != 218 {
		t.Fatalf("baseline = %#v", got)
	}
}
