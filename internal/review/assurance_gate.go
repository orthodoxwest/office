package review

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const assuranceBaselineFile = "assurance-baseline.json"

// AssuranceBaseline makes reductions in verified text coverage or modeled
// structural coverage explicit in a reviewable data-file diff.
type AssuranceBaseline struct {
	StartYear              int `json:"start_year"`
	Years                  int `json:"years"`
	VerifiedMinimum        int `json:"verified_minimum"`
	ModeledFeaturesMinimum int `json:"modeled_features_minimum"`
}

// AssuranceReport is a source-content-free release assurance summary.
type AssuranceReport struct {
	StartYear          int
	Years              int
	CandidateCount     int
	ModeledFeatures    int
	ModeledFeatureIDs  []string
	SelectedPages      int
	UncoveredFeatures  []string
	Verified           int
	NeedsReview        int
	SourceUnknown      int
	ClassifiedZeroes   int
	UnclassifiedZeroes int
	StaleZeroClasses   int
	StaleAttestations  int
}

// BuildAssuranceReport generates the structural and provenance release facts.
func BuildAssuranceReport(dataDir string, startYear, years int) (*AssuranceReport, error) {
	plan, err := BuildReviewPlan(dataDir, startYear, years, false)
	if err != nil {
		return nil, err
	}
	provenance, err := ScanProvenance(dataDir)
	if err != nil {
		return nil, err
	}
	zeroClassifications, err := LoadZeroClassifications(dataDir, provenance)
	if err != nil {
		return nil, err
	}
	rendered := make(map[string]bool, len(plan.RenderedKeys))
	for _, key := range plan.RenderedKeys {
		rendered[key] = true
	}
	report := &AssuranceReport{
		StartYear: startYear, Years: years, CandidateCount: plan.CandidateCount,
		ModeledFeatures: plan.FeatureCount, SelectedPages: len(plan.Selected),
		ModeledFeatureIDs: append([]string(nil), plan.Features...),
		UncoveredFeatures: append([]string(nil), plan.Uncovered...),
	}
	for _, entry := range provenance.Entries {
		if entry.Status == ProvenanceVerified {
			report.Verified++
		} else if rendered[entry.Key] {
			if entry.Status == ProvenanceNeedsReview {
				report.NeedsReview++
			} else {
				report.SourceUnknown++
			}
		} else if classification, ok := zeroClassifications[entry.Key]; ok && classification.Classified() {
			report.ClassifiedZeroes++
		} else {
			report.UnclassifiedZeroes++
			if classification.Stale {
				report.StaleZeroClasses++
			}
		}
		if entry.Stale {
			report.StaleAttestations++
		}
	}
	return report, nil
}

// WriteAssuranceSnapshot writes the deterministic, source-content-free
// review artifact checked in as a golden file. The summary makes count changes
// obvious while the sorted feature inventory catches one-for-one structural
// substitutions that a count alone would miss.
func WriteAssuranceSnapshot(report *AssuranceReport, w io.Writer) {
	WriteAssuranceSummary(report, nil, w, true)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "### Modeled structural features")
	if len(report.ModeledFeatureIDs) == 0 {
		fmt.Fprintln(w, "_None._")
	} else {
		for _, feature := range report.ModeledFeatureIDs {
			fmt.Fprintf(w, "- `%s`\n", feature)
		}
	}
	if len(report.UncoveredFeatures) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "### Uncovered structural features")
		for _, feature := range report.UncoveredFeatures {
			fmt.Fprintf(w, "- `%s`\n", feature)
		}
	}
}

// LoadAssuranceBaseline reads the intentional release floor.
func LoadAssuranceBaseline(dataDir string) (*AssuranceBaseline, error) {
	path := filepath.Join(dataDir, "review", assuranceBaselineFile)
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var baseline AssuranceBaseline
	if err := json.Unmarshal(body, &baseline); err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	if baseline.Years < 1 {
		return nil, fmt.Errorf("%s: years must be at least 1", path)
	}
	return &baseline, nil
}

// EvaluateAssurance returns gate failures. Stale attestations are reported but
// become gate failures only when they reduce verified coverage below baseline.
func EvaluateAssurance(report *AssuranceReport, baseline *AssuranceBaseline) []string {
	var failures []string
	if len(report.UncoveredFeatures) > 0 {
		failures = append(failures, fmt.Sprintf("%d structural feature(s) are uncovered", len(report.UncoveredFeatures)))
	}
	if report.Verified < baseline.VerifiedMinimum {
		failures = append(failures, fmt.Sprintf("verified provenance decreased: got %d, baseline requires %d", report.Verified, baseline.VerifiedMinimum))
	}
	if report.ModeledFeatures < baseline.ModeledFeaturesMinimum {
		failures = append(failures, fmt.Sprintf("modeled structural features decreased: got %d, baseline requires %d", report.ModeledFeatures, baseline.ModeledFeaturesMinimum))
	}
	return failures
}

// WriteAssuranceSummary writes plain text or Markdown suitable for a CI job
// summary. It includes counts and rule coverage only, never source contents.
func WriteAssuranceSummary(report *AssuranceReport, failures []string, w io.Writer, markdown bool) {
	if markdown {
		fmt.Fprintln(w, "## Office assurance summary")
		fmt.Fprintln(w)
		fmt.Fprintln(w, "| Measure | Count |")
		fmt.Fprintln(w, "|---|---:|")
		fmt.Fprintf(w, "| Candidate date-hours (%d–%d) | %d |\n", report.StartYear, report.StartYear+report.Years-1, report.CandidateCount)
		fmt.Fprintf(w, "| Modeled structural features | %d |\n", report.ModeledFeatures)
		fmt.Fprintf(w, "| Selected structural-review pages | %d |\n", report.SelectedPages)
		fmt.Fprintf(w, "| Uncovered features | %d |\n", len(report.UncoveredFeatures))
		fmt.Fprintf(w, "| Verified text entries | %d |\n", report.Verified)
		fmt.Fprintf(w, "| Rendered text entries needing review | %d |\n", report.NeedsReview)
		fmt.Fprintf(w, "| Rendered text entries with unknown source | %d |\n", report.SourceUnknown)
		fmt.Fprintf(w, "| Classified zero-occurrence entries | %d |\n", report.ClassifiedZeroes)
		fmt.Fprintf(w, "| Zeroes needing classification | %d |\n", report.UnclassifiedZeroes)
		fmt.Fprintf(w, "| Stale zero-occurrence classifications | %d |\n", report.StaleZeroClasses)
		fmt.Fprintf(w, "| Stale attestations | %d |\n", report.StaleAttestations)
	} else {
		fmt.Fprintf(w, "=== Office assurance: %d-%d ===\n", report.StartYear, report.StartYear+report.Years-1)
		fmt.Fprintf(w, "  candidate date-hours: %d\n", report.CandidateCount)
		fmt.Fprintf(w, "  modeled features:     %d\n", report.ModeledFeatures)
		fmt.Fprintf(w, "  selected pages:       %d\n", report.SelectedPages)
		fmt.Fprintf(w, "  uncovered features:   %d\n", len(report.UncoveredFeatures))
		fmt.Fprintf(w, "  verified:             %d\n", report.Verified)
		fmt.Fprintf(w, "  rendered needs review:%5d\n", report.NeedsReview)
		fmt.Fprintf(w, "  rendered unknown:     %d\n", report.SourceUnknown)
		fmt.Fprintf(w, "  classified zeroes:    %d\n", report.ClassifiedZeroes)
		fmt.Fprintf(w, "  unclassified zeroes:  %d\n", report.UnclassifiedZeroes)
		fmt.Fprintf(w, "  stale zero classes:   %d\n", report.StaleZeroClasses)
		fmt.Fprintf(w, "  stale attestations:   %d\n", report.StaleAttestations)
	}
	if len(failures) > 0 {
		if markdown {
			fmt.Fprintln(w, "\n### Gate failures")
		}
		for _, failure := range failures {
			fmt.Fprintf(w, "- %s\n", failure)
		}
	}
}

// UpdateAssuranceBaseline atomically raises or intentionally resets the
// reviewable coverage floor to the current report.
func UpdateAssuranceBaseline(dataDir string, report *AssuranceReport) error {
	baseline := AssuranceBaseline{
		StartYear: report.StartYear, Years: report.Years,
		VerifiedMinimum: report.Verified, ModeledFeaturesMinimum: report.ModeledFeatures,
	}
	body, err := json.MarshalIndent(baseline, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	dir := filepath.Join(dataDir, "review")
	tmp, err := os.CreateTemp(dir, ".assurance-baseline-*.json")
	if err != nil {
		return err
	}
	name := tmp.Name()
	ok := false
	defer func() {
		_ = tmp.Close()
		if !ok {
			_ = os.Remove(name)
		}
	}()
	if _, err := tmp.Write(body); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(name, 0o644); err != nil {
		return err
	}
	if err := os.Rename(name, filepath.Join(dir, assuranceBaselineFile)); err != nil {
		return err
	}
	ok = true
	return nil
}
