package review

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/office"
)

const assuranceBaselineFile = "assurance-baseline.json"

// AssuranceBaseline makes reductions in verified text coverage explicit in a
// reviewable data-file diff.
type AssuranceBaseline struct {
	StartYear       int `json:"start_year"`
	Years           int `json:"years"`
	VerifiedMinimum int `json:"verified_minimum"`
}

// AssuranceReport is a source-content-free release assurance summary.
type AssuranceReport struct {
	StartYear          int
	Years              int
	CandidateCount     int
	Verified           int
	NeedsReview        int
	SourceUnknown      int
	ClassifiedZeroes   int
	UnclassifiedZeroes int
	StaleZeroClasses   int
	StaleAttestations  int
}

// BuildAssuranceReport generates text-provenance release facts. Composition
// sampling and historical page signoffs do not participate in this gate.
func BuildAssuranceReport(dataDir string, startYear, years int) (*AssuranceReport, error) {
	rendered, count, err := renderedDependencies(dataDir, startYear, years)
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
	report := &AssuranceReport{StartYear: startYear, Years: years, CandidateCount: count}
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

// renderedDependencies inventories every composed date-hour form directly.
// A sample of feature representatives cannot determine all rendered text usage.
func renderedDependencies(dataDir string, startYear, years int) (map[string]bool, int, error) {
	if years < 1 {
		return nil, 0, fmt.Errorf("years must be at least 1")
	}
	eng, err := office.NewEngine(dataDir)
	if err != nil {
		return nil, 0, err
	}
	rendered := map[string]bool{}
	count := 0
	for year := startYear; year < startYear+years; year++ {
		days, err := calendar.BuildCalendar(year, dataDir)
		if err != nil {
			return nil, 0, err
		}
		moveable := calendar.ComputeMoveableDates(year)
		for i := range days {
			day := &days[i]
			for _, hourName := range HourNames {
				forms, err := composeReviewForms(eng, hourName, day, moveable)
				if err != nil {
					return nil, 0, fmt.Errorf("composing %s for %s: %w", hourName, day.Date.Format("2006-01-02"), err)
				}
				for _, hour := range forms {
					count++
					for _, key := range hourDependencies(hour) {
						rendered[key] = true
					}
				}
			}
		}
	}
	return rendered, count, nil
}

// WriteAssuranceSnapshot records provenance counts without source contents.
// Calendar, composition, source selections and decisions have their own parity snapshot.
func WriteAssuranceSnapshot(report *AssuranceReport, w io.Writer) {
	WriteAssuranceSummary(report, nil, w, true)
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
	if report.Verified < baseline.VerifiedMinimum {
		failures = append(failures, fmt.Sprintf("verified provenance decreased: got %d, baseline requires %d", report.Verified, baseline.VerifiedMinimum))
	}
	return failures
}

// WriteAssuranceSummary writes plain text or Markdown suitable for a CI job
// summary. It reports source verification, not composition correctness.
func WriteAssuranceSummary(report *AssuranceReport, failures []string, w io.Writer, markdown bool) {
	if markdown {
		fmt.Fprintln(w, "## Text provenance assurance")
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Source verification does not establish correct appointments or complete office structure.")
		fmt.Fprintln(w)
		fmt.Fprintln(w, "| Measure | Count |")
		fmt.Fprintln(w, "|---|---:|")
		fmt.Fprintf(w, "| Distinct date-hour forms (%d–%d) | %d |\n", report.StartYear, report.StartYear+report.Years-1, report.CandidateCount)
		fmt.Fprintf(w, "| Verified text entries | %d |\n", report.Verified)
		fmt.Fprintf(w, "| Rendered text entries needing review | %d |\n", report.NeedsReview)
		fmt.Fprintf(w, "| Rendered text entries with unknown source | %d |\n", report.SourceUnknown)
		fmt.Fprintf(w, "| Classified zero-occurrence entries | %d |\n", report.ClassifiedZeroes)
		fmt.Fprintf(w, "| Zeroes needing classification | %d |\n", report.UnclassifiedZeroes)
		fmt.Fprintf(w, "| Stale zero-occurrence classifications | %d |\n", report.StaleZeroClasses)
		fmt.Fprintf(w, "| Stale attestations | %d |\n", report.StaleAttestations)
	} else {
		fmt.Fprintf(w, "=== Text provenance assurance: %d-%d ===\n", report.StartYear, report.StartYear+report.Years-1)
		fmt.Fprintln(w, "Source verification does not establish correct appointments or complete office structure.")
		fmt.Fprintf(w, "  distinct date-hour forms: %d\n", report.CandidateCount)
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
		VerifiedMinimum: report.Verified,
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
