package review

import (
	"bytes"
	"strings"
	"testing"
)

func TestEvaluateAssuranceRetainsVerifiedFloor(t *testing.T) {
	baseline := &AssuranceBaseline{VerifiedMinimum: 2}
	for _, count := range []int{1, 2, 3} {
		failures := EvaluateAssurance(&AssuranceReport{Verified: count}, baseline)
		if (len(failures) > 0) != (count < 2) {
			t.Fatalf("verified=%d, failures=%v", count, failures)
		}
	}
}

func TestAssuranceSummaryContainsNoSourceText(t *testing.T) {
	report := &AssuranceReport{
		StartYear: 2026, Years: 1, CandidateCount: 2555,
		Verified: 1, NeedsReview: 3, SourceUnknown: 4,
	}
	var out bytes.Buffer
	WriteAssuranceSummary(report, nil, &out, true)
	for _, want := range []string{
		"Text provenance assurance",
		"Verified text entries", "Classified zero-occurrence entries",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("summary missing %q:\n%s", want, out.String())
		}
	}
}

func TestAssuranceOutputsContainNoStructuralCompletionScore(t *testing.T) {
	report := &AssuranceReport{StartYear: 2026, Years: 1, Verified: 2}
	for _, markdown := range []bool{false, true} {
		var out bytes.Buffer
		WriteAssuranceSummary(report, nil, &out, markdown)
		for _, retired := range []string{"Modeled structural features", "Full structural-cover pages", "Residual structural-review pages", "Uncovered features", "credited", "full-cover", "residual"} {
			if strings.Contains(out.String(), retired) {
				t.Errorf("retired metric %q in report", retired)
			}
		}
	}
}

func TestUpdateAssuranceBaselineRoundTrip(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir+"/review/.keep", "")
	report := &AssuranceReport{StartYear: 2026, Years: 28, Verified: 4}
	if err := UpdateAssuranceBaseline(dir, report); err != nil {
		t.Fatal(err)
	}
	got, err := LoadAssuranceBaseline(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.VerifiedMinimum != 4 {
		t.Fatalf("baseline = %#v", got)
	}
}
